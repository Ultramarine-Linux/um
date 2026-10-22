package upgrade

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Ultramarine-Linux/um/pkg/util"
)

type loginSession struct {
	ID, Name, Class, Type string
	Active, Locked, Idle  bool
}

// parseShowSessions parses `loginctl show-session ID... -p ...` output, one
// block of key=value lines per session, separated by blank lines.
func parseShowSessions(output string) []loginSession {
	var sessions []loginSession
	var current *loginSession
	flush := func() {
		if current != nil && current.ID != "" {
			sessions = append(sessions, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if current == nil {
			current = &loginSession{}
		}
		switch key {
		case "Id":
			current.ID = value
		case "Name":
			current.Name = value
		case "Class":
			current.Class = value
		case "Type":
			current.Type = value
		case "Active":
			current.Active = value == "yes"
		case "LockedHint":
			current.Locked = value == "yes"
		case "IdleHint":
			current.Idle = value == "yes"
		}
	}
	flush()
	return sessions
}

// inUse reports whether someone is at this session: an active, unlocked,
// non-idle login session on a seat (graphical or text console).
func (s loginSession) inUse() bool {
	if s.Class != "user" || !s.Active || s.Locked || s.Idle {
		return false
	}
	switch s.Type {
	case "wayland", "x11", "mir", "tty":
		return true
	}
	return false
}

// sessionInUse returns the first session someone is using, optionally only
// sessions of userName.
func sessionInUse(sessions []loginSession, userName string) (loginSession, bool) {
	for _, s := range sessions {
		if userName != "" && s.Name != userName {
			continue
		}
		if s.inUse() {
			return s, true
		}
	}
	return loginSession{}, false
}

func loginSessions() ([]loginSession, error) {
	list, err := exec.Command("loginctl", "list-sessions", "--no-legend").Output()
	if err != nil {
		return nil, fmt.Errorf("loginctl list-sessions failed: %w", err)
	}
	var ids []string
	for _, line := range strings.Split(string(list), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			ids = append(ids, fields[0])
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	args := append([]string{"show-session"}, ids...)
	args = append(args, "-p", "Id", "-p", "Name", "-p", "Class", "-p", "Type", "-p", "Active", "-p", "LockedHint", "-p", "IdleHint")
	output, err := exec.Command("loginctl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("loginctl show-session failed: %w", err)
	}
	return parseShowSessions(string(output)), nil
}

// onACPower reports whether the device runs on AC power (also true for
// devices without a battery).
func onACPower() bool {
	err := exec.Command("systemd-ac-power").Run()
	var exitErr *exec.ExitError
	// Treat a missing systemd-ac-power as AC power rather than never updating.
	return err == nil || !errors.As(err, &exitErr)
}

// installInterval is how often updates install at most.
const installInterval = 20 * time.Hour

// notifyInterval is how often "Ask me" checks for updates and notifies.
const notifyInterval = 24 * time.Hour

func readTimestamp(path string) (time.Time, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(n, 0), true
}

func writeTimestamp(path string, t time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.FormatInt(t.Unix(), 10)+"\n"), 0644)
}

// within reports whether the timestamp in path is less than d before now.
func within(path string, d time.Duration, now time.Time) bool {
	t, ok := readTimestamp(path)
	return ok && now.Sub(t) < d && !t.After(now)
}

// activityTracker remembers since when no one has used the device. The timers
// poll every few minutes, so "inactive for 30 minutes" means 30 minutes of
// consecutive polls seeing no session in use.
type activityTracker struct {
	path string
}

// inactiveFor records the current observation and returns how long the device
// has been inactive (0 if it's in use now).
func (a activityTracker) inactiveFor(inUse bool, now time.Time) time.Duration {
	if inUse {
		_ = os.Remove(a.path)
		return 0
	}
	since, ok := readTimestamp(a.path)
	if !ok || since.After(now) {
		_ = writeTimestamp(a.path, now)
		return 0
	}
	return now.Sub(since)
}

func systemStateFile(name string) string { return filepath.Join(util.GetStateDir(), name) }

// userStateDir is $XDG_STATE_HOME/um (~/.local/state/um).
func userStateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "um")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "um")
}
