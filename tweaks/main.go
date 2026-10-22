package tweaks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

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

// StatePath returns the directory where tweak state markers are stored.
// Markers are world-readable so unprivileged frontends (e.g. the Plasma
// Tweaks KCM) can show which tweaks are enabled without escalating.
func StatePath() string {
	return util.GetStateDir() + "/tweaks"
}

func (t Tweak) ID() string {
	return filepath.Base(t.Path)
}

func (t Tweak) stateFile() string {
	return filepath.Join(StatePath(), t.ID()+".enabled")
}

// Enabled reports whether a toggle tweak is enabled, or whether a oneshot
// tweak has been applied. Script tweaks never have state.
func (t Tweak) Enabled() bool {
	if t.TweakType == TweakTypeScript {
		return false
	}
	_, err := os.Stat(t.stateFile())
	return err == nil
}

func (t Tweak) setEnabled(enabled bool) error {
	if !enabled {
		if err := os.Remove(t.stateFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(StatePath(), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(t.stateFile(), nil, 0644); err != nil {
		return err
	}
	// Modes passed above are filtered by the umask (which sudo may carry
	// over from the user), so set them explicitly: unprivileged frontends
	// must be able to read the state.
	if err := os.Chmod(StatePath(), 0755); err != nil {
		return err
	}
	return os.Chmod(t.stateFile(), 0644)
}

func (t Tweak) runPlaybook(playbook string) error {
	// run ansible-playbook <playbook> --connection=local from the tweak directory
	cmd := exec.Command("/usr/bin/ansible-playbook", "--connection=local", filepath.Join(t.Path, playbook))
	cmd.Dir = t.Path
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t Tweak) Enable() error {
	switch t.TweakType {
	case TweakTypeOneshot:
	case TweakTypeToggle:
	default:
		return TweakTypeNotSupportedError(t.TweakType)
	}

	if err := t.runPlaybook("enable.yml"); err != nil {
		return err
	}
	if err := t.setEnabled(true); err != nil {
		return fmt.Errorf("tweak was applied, but recording its enabled state failed: %w", err)
	}
	return nil
}

func (t Tweak) Disable() error {
	if t.TweakType != TweakTypeToggle {
		return TweakTypeNotSupportedError(t.TweakType)
	}

	if err := t.runPlaybook("disable.yml"); err != nil {
		return err
	}
	if err := t.setEnabled(false); err != nil {
		return fmt.Errorf("tweak was disabled, but recording its state failed: %w", err)
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
	if err := unix.Exec(t.Path+"/script.sh", []string{t.Path + "/script.sh"}, os.Environ()); err != nil {
		return err
	}
	return nil
}

// ListEntry is the machine-readable representation of a tweak, used by
// `um tweaks list --json`.
type ListEntry struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Stability   string `json:"stability,omitempty"`
	Warning     string `json:"warning,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// SortedEntries returns all tweaks as list entries, sorted by ID.
func SortedEntries(tweaks map[string]Tweak) []ListEntry {
	entries := make([]ListEntry, 0, len(tweaks))
	for id, t := range tweaks {
		entry := ListEntry{ID: id, Type: t.TweakType.String(), Enabled: t.Enabled()}
		if t.Description != nil {
			entry.Description = *t.Description
		}
		if t.Stability != nil {
			entry.Stability = t.Stability.String()
		}
		if t.Warning != nil {
			entry.Warning = *t.Warning
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries
}
