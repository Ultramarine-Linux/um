package upgrade

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Settings is the system-wide automatic update configuration.
type Settings struct {
	Automatic       bool   `json:"automatic"`
	Offline         bool   `json:"offline"`
	InactiveMinutes int    `json:"inactive_minutes"`
	WindowStart     string `json:"window_start"`
	WindowEnd       string `json:"window_end"`
}

// DefaultSettings are used until the settings are changed.
var DefaultSettings = Settings{Automatic: true, InactiveMinutes: 30}

// Validate checks values that can't be expressed in the JSON types.
func (s Settings) Validate() error {
	if s.InactiveMinutes < 0 || s.InactiveMinutes > 24*60 {
		return fmt.Errorf("inactive minutes must be between 0 and %d", 24*60)
	}
	if (s.WindowStart == "") != (s.WindowEnd == "") {
		return errors.New("a time window needs both a start and an end")
	}
	for _, t := range []string{s.WindowStart, s.WindowEnd} {
		if _, err := parseClock(t); t != "" && err != nil {
			return err
		}
	}
	return nil
}

// parseClock parses "HH:MM" into minutes after midnight.
func parseClock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q, expected HH:MM", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}

// inWindow reports whether now falls between start and end ("HH:MM"). The
// window may wrap past midnight (22:00-06:00). An empty window is always open.
func inWindow(now time.Time, start, end string) bool {
	if start == "" || end == "" {
		return true
	}
	from, err1 := parseClock(start)
	to, err2 := parseClock(end)
	if err1 != nil || err2 != nil {
		return true
	}
	minute := now.Hour()*60 + now.Minute()
	if from <= to {
		return minute >= from && minute < to
	}
	return minute >= from || minute < to
}

func settingsPath() string {
	if p := os.Getenv("UM_UPGRADE_CONFIG"); p != "" {
		return p
	}
	return "/etc/um/upgrade.json"
}

// LoadSettings reads the settings, falling back to the defaults for a
// missing file or missing keys.
func LoadSettings() (Settings, error) {
	settings := DefaultSettings
	content, err := os.ReadFile(settingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(content, &settings); err != nil {
		return DefaultSettings, fmt.Errorf("invalid %s: %w", settingsPath(), err)
	}
	if err := settings.Validate(); err != nil {
		return DefaultSettings, fmt.Errorf("invalid %s: %w", settingsPath(), err)
	}
	return settings, nil
}

// SaveSettings writes the settings world-readable, so unprivileged
// frontends and user timers can read them.
func SaveSettings(settings Settings) error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(content, '\n'), 0644); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ParseWindow parses a time window like "01:00-05:00". An empty string means
// any time and returns empty start and end.
func ParseWindow(window string) (start, end string, err error) {
	window = strings.TrimSpace(window)
	if window == "" {
		return "", "", nil
	}
	start, end, ok := strings.Cut(window, "-")
	if !ok {
		return "", "", errors.New("the window must be HH:MM-HH:MM, or empty for any time")
	}
	start, end = strings.TrimSpace(start), strings.TrimSpace(end)
	for _, t := range []string{start, end} {
		if _, err := parseClock(t); err != nil {
			return "", "", err
		}
	}
	return start, end, nil
}
