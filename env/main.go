// um env library stuff

package env

import (
	"log"
	"os"
	"os/exec"

	"github.com/BurntSushi/toml"
)

const umEnvContext = "/var/um/env"
const umEnvManifest = umEnvContext + "/environment.toml"

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

func LoadEnvManifest() (*EnvManifest, error) {
	// load from file
	data, err := os.ReadFile(umEnvManifest)
	if err != nil {
		return nil, err
	}
	// set CWD
	if err := os.Chdir(umEnvContext); err != nil {
		return nil, err
	}

	var manifest EnvManifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
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
	args := []string{"do"}
	args = appendPackageAction(args, "install", p.Install)
	args = appendPackageAction(args, "remove", p.Remove)
	args = appendPackageAction(args, "reinstall", p.Reinstall)

	if len(args) == 1 {
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
