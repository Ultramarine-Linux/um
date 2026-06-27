package env

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/Ultramarine-Linux/um/pkg/util"
	"go.yaml.in/yaml/v4"
)

// minimal struct to parse `bootc edit` output
type bootcEditSpec struct {
	Spec struct {
		Image struct {
			Image string `yaml:"image"`
		} `yaml:"image"`
	} `yaml:"spec"`
}

func GetBootcImage() (string, error) {
	util.SudoIfNeeded([]string{})

	// run `bootc edit` and pipe output
	// which then gets parsed by YAML

	bootcEditCmd := exec.Command("bootc", "edit")
	bootcEditCmd.Env = append(os.Environ(),
		"EDITOR=cat",
	)

	bootcEditOutput, err := bootcEditCmd.Output()

	if err != nil {
		return "", err
	}

	bootcEditOutput = bytes.TrimSpace(bootcEditOutput)
	bootcEditOutput = []byte(strings.TrimSuffix(string(bootcEditOutput), "\nEdit cancelled, no changes made."))

	var bootcEditParsed bootcEditSpec
	if err := yaml.Unmarshal(bootcEditOutput, &bootcEditParsed); err != nil {
		return "", err
	}

	if bootcEditParsed.Spec.Image.Image == "" {
		return "", errors.New("bootc edit output did not contain spec.image.image")
	}

	return bootcEditParsed.Spec.Image.Image, nil
}
