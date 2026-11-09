package tweaks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

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

// ValidateID checks that id names a single entry in the tweaks directory, so it
// can't load a tweak from elsewhere (e.g. `../../tmp/evil`) and run it as root
func ValidateID(id string) error {
	if id == "" || id == "." || id == ".." || id != filepath.Base(id) || strings.ContainsRune(id, os.PathSeparator) {
		return fmt.Errorf("invalid tweak ID: %q", id)
	}
	return nil
}

func LoadTweakId(tweakid string) (*Tweak, error) {
	if err := ValidateID(tweakid); err != nil {
		return nil, err
	}
	return LoadTweakFromPath(filepath.Join(TweaksPath(), tweakid))
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

// StatePath returns the directory holding tweak state markers. Markers are
// world-readable so unprivileged frontends (e.g. the Plasma Tweaks KCM) can show
// which tweaks are enabled without escalating.
func StatePath() string {
	return filepath.Join(util.GetStateDir(), "tweaks")
}

// overridden by tests
var ansiblePlaybook = "/usr/bin/ansible-playbook"

// checkUnknownExitCode is what check.sh exits with when it can't tell, so the
// state marker is used instead. It's not 2 because shells and test(1) use that
// for errors, and a broken check.sh should be reported rather than ignored.
const checkUnknownExitCode = 100

var errCheckUnknown = errors.New("check script could not determine the state")

const checkTimeout = 10 * time.Second

func (t Tweak) ID() string {
	return filepath.Base(t.Path)
}

func (t Tweak) stateFile() string {
	return filepath.Join(StatePath(), t.ID()+".enabled")
}

func (t Tweak) checkScript() string {
	return filepath.Join(t.Path, "check.sh")
}

// Enabled reports whether a toggle tweak is enabled or a oneshot tweak has been
// applied. Script tweaks never have state.
//
// A check.sh, if present, is the source of truth, which also covers tweaks
// enabled before um recorded state. Without one, or when it exits
// checkUnknownExitCode, the marker written by Enable/Disable is used. An error
// means the state is unknown, not that the tweak is disabled.
func (t Tweak) Enabled() (bool, error) {
	if t.TweakType == TweakTypeScript {
		return false, nil
	}

	if info, err := os.Stat(t.checkScript()); err == nil && info.Mode().IsRegular() {
		enabled, err := t.runCheck()
		if !errors.Is(err, errCheckUnknown) {
			return enabled, err
		}
	}

	_, err := os.Stat(t.stateFile())
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}

func (t Tweak) runCheck() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	// run through sh so a check.sh that lost its exec bit in packaging still works
	cmd := exec.CommandContext(ctx, "/bin/sh", t.checkScript())
	cmd.Dir = t.Path
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		return false, nil
	case errors.As(err, &exitErr) && exitErr.ExitCode() == checkUnknownExitCode:
		return false, errCheckUnknown
	case ctx.Err() != nil:
		return false, fmt.Errorf("check script for %s timed out", t.ID())
	default:
		return false, fmt.Errorf("check script for %s could not determine its state: %w", t.ID(), err)
	}
}

// ensureStateDir creates path if needed and checks it's safe for markers. It
// must be a real directory, root-owned and not group/world-writable when
// running as root (so UM_STATE can't point root at a caller-controlled
// directory), and traversable by everyone. Modes from a restrictive umask are
// fixed up.
func ensureStateDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	// check and chmod through one O_NOFOLLOW fd so a swapped-in symlink can't race us
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("state path %s must be a directory, not a symlink: %w", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()
	if os.Geteuid() == 0 {
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Uid != 0 {
			return fmt.Errorf("state directory %s is not owned by root, refusing to use it", path)
		}
		if perm&0022 != 0 {
			return fmt.Errorf("state directory %s is writable by other users, refusing to use it", path)
		}
	}
	if perm&0055 != 0055 {
		return f.Chmod(perm | 0755)
	}
	return nil
}

func ensureStateDirs() error {
	if err := ensureStateDir(util.GetStateDir()); err != nil {
		return err
	}
	return ensureStateDir(StatePath())
}

func (t Tweak) setEnabled(enabled bool) error {
	if err := ensureStateDirs(); err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(t.stateFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	// never write through a symlink planted in the state dir
	f, err := os.OpenFile(t.stateFile(), os.O_WRONLY|os.O_CREATE|syscall.O_NOFOLLOW, 0644)
	if err != nil {
		return err
	}
	// the umask (which sudo may carry over) filters the mode above, so set it
	// explicitly for unprivileged frontends
	if err := f.Chmod(0644); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// lock keeps concurrent um processes (e.g. the KCM and a terminal) from running
// playbooks at the same time
func lock() (func(), error) {
	if err := ensureStateDirs(); err != nil {
		return nil, err
	}
	// 0600 because flock works on read-only fds, so a world-readable lock file
	// would let any user block um forever
	f, err := os.OpenFile(filepath.Join(StatePath(), ".lock"), os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); errors.Is(err, unix.EWOULDBLOCK) {
		fmt.Fprintln(os.Stderr, "Waiting for another um process to finish changing tweaks...")
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		unix.Flock(int(f.Fd()), unix.LOCK_UN)
		f.Close()
	}, nil
}

// runPlaybook runs a playbook from the tweak directory. A failing playbook
// returns an *exec.ExitError carrying ansible's exit code.
//
// um outlives ansible so the lock is held for the whole run. SIGTERM/SIGHUP
// (e.g. the KCM cancelling through pkexec) are forwarded to ansible instead of
// killing um. SIGINT is ignored since the terminal already sends it to ansible.
func (t Tweak) runPlaybook(playbook string) error {
	cmd := exec.Command(ansiblePlaybook, "--connection=local", filepath.Join(t.Path, playbook))
	cmd.Dir = t.Path
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// if um is killed outright, take ansible down too rather than leave it
	// running without the lock. Pdeathsig fires when the forking OS thread
	// exits, so pin this goroutine to its thread until ansible is done.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)

	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case sig := <-signals:
				if sig != syscall.SIGINT {
					cmd.Process.Signal(sig)
				}
			case <-done:
				return
			}
		}
	}()
	return cmd.Wait()
}

func (t Tweak) Enable() error {
	switch t.TweakType {
	case TweakTypeOneshot:
	case TweakTypeToggle:
	default:
		return TweakTypeNotSupportedError(t.TweakType)
	}

	unlock, err := lock()
	if err != nil {
		return err
	}
	defer unlock()

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

	unlock, err := lock()
	if err != nil {
		return err
	}
	defer unlock()

	if err := t.runPlaybook("disable.yml"); err != nil {
		return err
	}
	if err := t.setEnabled(false); err != nil {
		return fmt.Errorf("tweak was disabled, but recording its state failed: %w", err)
	}
	return nil
}

func (t Tweak) Run() error {
	if t.TweakType != TweakTypeScript {
		return TweakTypeNotSupportedError(t.TweakType)
	}

	if err := os.Chdir(t.Path); err != nil {
		return err
	}
	script := filepath.Join(t.Path, "script.sh")
	return unix.Exec(script, []string{script}, os.Environ())
}

// ListEntry is a tweak as printed by `um tweaks list --json`
type ListEntry struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Stability   string `json:"stability,omitempty"`
	Warning     string `json:"warning,omitempty"`
	// always false for script tweaks, and means "applied" for oneshot tweaks
	Enabled bool `json:"enabled"`
	// set when the state couldn't be determined, in which case Enabled is meaningless
	StateError string `json:"state_error,omitempty"`
}

// SortedEntries returns all tweaks as list entries, sorted by ID
func SortedEntries(tweaks map[string]Tweak) []ListEntry {
	entries := make([]ListEntry, 0, len(tweaks))
	for id, t := range tweaks {
		entry := ListEntry{ID: id, Type: t.TweakType.String()}
		if enabled, err := t.Enabled(); err != nil {
			entry.StateError = err.Error()
		} else {
			entry.Enabled = enabled
		}
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
