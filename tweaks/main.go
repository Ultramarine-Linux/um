package tweaks

import (
	"fmt"
	"os"

	"github.com/Ultramarine-Linux/um/pkg/util"
	"go.yaml.in/yaml/v4"
	"golang.org/x/sys/unix"
)

type StabilityLevel int

const (
	GFL StabilityLevel = iota
	Devel
	Alpha
	Beta
	Stable
)

type Tweak struct {
	Description *string         `yaml:"description"`
	TweakType   TweakType       `yaml:"type"`
	Stability   *StabilityLevel `yaml:"stability"`
	Warning     *string         `yaml:"warning,omitempty"`
	Path        string          `yaml:"-"`
}

var stabilityLevelStrings = map[StabilityLevel]string{
	GFL:    "gfl",
	Devel:  "devel",
	Alpha:  "alpha",
	Beta:   "beta",
	Stable: "stable",
}

func (t *StabilityLevel) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	for k, v := range stabilityLevelStrings {
		if v == s {
			*t = k
			return nil
		}
	}
	return fmt.Errorf("unknown stability level: %s", s)
}

func (s StabilityLevel) String() string {
	if str, ok := stabilityLevelStrings[s]; ok {
		return str
	}
	return "unknown"
}

type TweakType int

const (
	// Tweaks of type "toggle" have an Ansible playbook called "enable.yml" and "disable.yml"
	TweakTypeToggle TweakType = iota
	// Tweaks of type "oneshot" have an Ansible playbook called "enable.yml" only
	TweakTypeOneshot
	// Tweaks of type "script" runs arbitrary code, defined in script.sh
	TweakTypeScript
)

func (t *TweakType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	switch s {
	case "toggle":
		*t = TweakType(TweakTypeToggle)
	case "oneshot":
		*t = TweakType(TweakTypeOneshot)
	case "script":
		*t = TweakType(TweakTypeScript)
	default:
		return fmt.Errorf("unknown tweak type: %s", s)
	}
	return nil
}

func TweakTypeNotSupportedError(t TweakType) error {
	return fmt.Errorf("tweak type %s is not supported in this operation", t.String())
}

func (t TweakType) String() string {
	switch t {
	case TweakTypeToggle:
		return "toggle"
	case TweakTypeOneshot:
		return "oneshot"
	case TweakTypeScript:
		return "script"
	default:
		return "unknown"
	}
}

func TweaksPath() string {
	// Optionally check envar `UM_TWEAKS_PATH` for an override location
	if os.Getenv("UM_TWEAKS_PATH") != "" {
		return os.Getenv("UM_TWEAKS_PATH")
	}
	return util.GetDataDir() + "/tweaks"
}

// Load a tweak
// path: path to tweak directory, e.g. /usr/share/um/tweaks/tweak-id
func LoadTweakFromPath(path string) (*Tweak, error) {
	tweakManifest := fmt.Sprintf("%s/metadata.yml", path)
	file, err := os.ReadFile(tweakManifest)
	if err != nil {
		return nil, err
	}

	var tweak Tweak
	err = yaml.Unmarshal(file, &tweak)
	if err != nil {
		return nil, err
	}

	tweak.Path = path

	return &tweak, nil
}

func LoadTweakId(tweakid string) (*Tweak, error) {
	tweaksManifest := TweaksPath()
	return LoadTweakFromPath(fmt.Sprintf("%s/%s", tweaksManifest, tweakid))
}

func List() (map[string]Tweak, error) {
	// Optionally check envar `UM_TWEAKS_PATH` for an override location
	tweaksManifest := TweaksPath()

	// now we just list the directories in this path

	files, err := os.ReadDir(tweaksManifest)
	if err != nil {
		return nil, err
	}

	tweaks := make(map[string]Tweak)
	for _, file := range files {
		if file.IsDir() {
			tweak, err := LoadTweakFromPath(fmt.Sprintf("%s/%s", tweaksManifest, file.Name()))
			if err != nil {
				return nil, err
			}
			tweaks[file.Name()] = *tweak
		}
	}
	return tweaks, nil
}

func (t Tweak) Enable() error {
	// check if tweak type is oneshot or toggle

	switch t.TweakType {
	case TweakTypeOneshot:
	case TweakTypeToggle:

	default:
		return TweakTypeNotSupportedError(t.TweakType)
	}

	// popd in path

	os.Chdir(t.Path)
	// run ansible-playbook enable.yml -i localhost, --connection=local
	cmd := []string{"--connection=local", t.Path + "/enable.yml"}

	if err := unix.Exec("/usr/bin/ansible-playbook", append([]string{"ansible-playbook"}, cmd...), os.Environ()); err != nil {
		return err
	}

	return nil
}

func (t Tweak) Disable() error {
	// check if tweak type is toggle
	if t.TweakType != TweakTypeToggle {
		return TweakTypeNotSupportedError(t.TweakType)
	}

	// popd in path

	os.Chdir(t.Path)
	// run ansible-playbook disable.yml -i localhost, --connection=local
	cmd := []string{"--connection=local", t.Path + "/disable.yml"}

	if err := unix.Exec("/usr/bin/ansible-playbook", append([]string{"ansible-playbook"}, cmd...), os.Environ()); err != nil {
		return err
	}

	return nil
}

func (t Tweak) Run() error {
	// check if tweak type is script
	if t.TweakType != TweakTypeScript {
		return TweakTypeNotSupportedError(t.TweakType)
	}

	// popd in path

	os.Chdir(t.Path)
	// run script.sh
	if err := unix.Exec(t.Path+"/script.sh", []string{}, os.Environ()); err != nil {
		return err
	}
	return nil
}
