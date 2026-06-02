// um env library stuff

package env

import (
	_ "embed"
	"log"
	"os"
	"os/exec"
	"text/template"

	"slices"

	"github.com/BurntSushi/toml"
)

const UmEnvContext = "/var/um/env"
const UmEnvManifest = UmEnvContext + "/environment.toml"
const UmEnvContainerfile = UmEnvContext + "/Containerfile"
const UmEnvManagedImage = "localhost/um-env"

//go:embed Containerfile.gotmpl
var containerfileTemplateSource string

var containerfileTemplate = template.Must(template.New("Containerfile").Parse(containerfileTemplateSource))

// stuff to do in the environment
type EnvManifest struct {
	// [packages]
	Packages Packages `toml:"packages"`
}

type Packages struct {
	Install   []string `toml:"install"`
	Remove    []string `toml:"remove"`
	Reinstall []string `toml:"reinstall"`
}

type containerfileTemplateData struct {
	BaseImage string
}

func containsPackage(packages []string, packageName string) bool {
	return slices.Contains(packages, packageName)
}

func InitEnvironment(baseImage string) error {
	if err := os.MkdirAll(UmEnvContext, 0o755); err != nil {
		return err
	}

	manifest := EnvManifest{
		Packages: Packages{
			Install:   []string{},
			Remove:    []string{},
			Reinstall: []string{},
		},
	}

	manifestFile, err := os.OpenFile(UmEnvManifest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer manifestFile.Close()

	if err := toml.NewEncoder(manifestFile).Encode(manifest); err != nil {
		return err
	}

	containerfile, err := os.OpenFile(UmEnvContainerfile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer containerfile.Close()

	if err := containerfileTemplate.Execute(containerfile, containerfileTemplateData{
		BaseImage: baseImage,
	}); err != nil {
		return err
	}

	return nil
}

func LoadEnvManifest() (*EnvManifest, error) {
	// load from file
	data, err := os.ReadFile(UmEnvManifest)
	if err != nil {
		return nil, err
	}
	// set CWD
	if err := os.Chdir(UmEnvContext); err != nil {
		return nil, err
	}

	var manifest EnvManifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// saves the manifest to the specified path
func (e *EnvManifest) Save() error {
	data, err := toml.Marshal(e)
	if err != nil {
		return err
	}
	return os.WriteFile(UmEnvManifest, data, 0o644)
}

func (e *EnvManifest) AddPackage(packageName string) bool {
	if containsPackage(e.Packages.Install, packageName) {
		return false
	}

	e.Packages.Install = append(e.Packages.Install, packageName)

	return true
}

func appendPackageAction(args []string, action string, packages []string) []string {
	if len(packages) == 0 {
		return args
	}

	args = append(args, "--action="+action)
	args = append(args, packages...)

	return args
}

func (p *Packages) CommitTransaction() error {
	args := []string{"do", "-y"}
	args = appendPackageAction(args, "install", p.Install)
	args = appendPackageAction(args, "remove", p.Remove)
	args = appendPackageAction(args, "reinstall", p.Reinstall)

	if len(args) == 2 {
		log.Println("no package changes to apply")
		return nil
	}

	cmd := exec.Command("dnf", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()

}

// Apply changes to the environment
func (e *EnvManifest) ApplyChanges() error {
	return e.Packages.CommitTransaction()
}

func (e *EnvManifest) BuildContainerfile() error {
	// run podman build -t
	podmanArgs := []string{
		"build",
		"--pull=newer",
		"-f",
		UmEnvContainerfile,
		"-t",
		UmEnvManagedImage,
		UmEnvContext,
	}
	cmd := exec.Command("podman", podmanArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func EnvBootcSwitch() error {
	args := []string{"switch", "--transport=containers-storage", UmEnvManagedImage}
	cmd := exec.Command("bootc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
