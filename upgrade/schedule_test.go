package upgrade

import (
	"testing"
	"time"
)

func TestParseTimerShow(t *testing.T) {
	if got := parseTimerShow("ActiveState=active\nNextElapseUSecRealtime=@1761181200\n"); got != (ScheduleStatus{true, 1761181200}) {
		t.Fatalf("got %+v", got)
	}
	if got := parseTimerShow("ActiveState=inactive\nNextElapseUSecRealtime=\n"); got != (ScheduleStatus{}) {
		t.Fatalf("got %+v", got)
	}
}

func TestReadyToInstall(t *testing.T) {
	noon := time.Date(2026, 10, 22, 12, 0, 0, 0, time.Local)
	night := time.Date(2026, 10, 22, 23, 0, 0, 0, time.Local)
	auto := DefaultSettings
	idle := 45 * time.Minute

	cases := []struct {
		name      string
		settings  Settings
		tonight   bool
		inUse     bool
		inactive  time.Duration
		installed bool
		now       time.Time
		want      bool
	}{
		{"idle long enough", auto, false, false, idle, false, noon, true},
		{"in use", auto, false, true, 0, false, noon, false},
		{"in use with no wait", Settings{Automatic: true}, false, true, 0, false, noon, false},
		{"no wait", Settings{Automatic: true}, false, false, 0, false, noon, true},
		{"used recently", auto, false, false, 10 * time.Minute, false, noon, false},
		{"already installed", auto, false, false, idle, true, noon, false},
		{"automatic off", Settings{InactiveMinutes: 30}, false, false, idle, false, noon, false},
		{"outside window", Settings{Automatic: true, InactiveMinutes: 30, WindowStart: "01:00", WindowEnd: "05:00"}, false, false, idle, false, noon, false},
		{"tonight ignores automatic and recent installs", Settings{InactiveMinutes: 30}, true, false, idle, true, night, true},
		{"tonight waits for the night", auto, true, false, idle, false, noon, false},
		{"tonight uses the window", Settings{InactiveMinutes: 30, WindowStart: "11:00", WindowEnd: "13:00"}, true, false, idle, false, noon, true},
	}
	for _, c := range cases {
		if got, why := readyToInstall(c.settings, c.tonight, c.inUse, c.inactive, c.installed, c.now); got != c.want {
			t.Errorf("%s: got %v (%s), want %v", c.name, got, why, c.want)
		}
	}
}
