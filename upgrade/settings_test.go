package upgrade

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpgradeSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "um", "upgrade.json")
	t.Setenv("UM_UPGRADE_CONFIG", path)

	got, err := LoadSettings()
	if err != nil || got != DefaultSettings {
		t.Fatalf("missing file should give defaults: %+v %v", got, err)
	}
	want := Settings{Automatic: false, Offline: true, InactiveMinutes: 10, WindowStart: "01:00", WindowEnd: "05:00"}
	if err := SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err = LoadSettings()
	if err != nil || got != want {
		t.Fatalf("got %+v %v", got, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm()&0004 == 0 {
		t.Fatal("settings must be world-readable")
	}

	// Missing keys keep their defaults.
	os.WriteFile(path, []byte(`{"offline": true}`), 0644)
	if got, _ := LoadSettings(); got != (Settings{Automatic: true, Offline: true, InactiveMinutes: 30}) {
		t.Fatalf("got %+v", got)
	}
	os.WriteFile(path, []byte(`nope`), 0644)
	if _, err := LoadSettings(); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestInWindow(t *testing.T) {
	at := func(hm string) time.Time {
		tm, _ := time.Parse("15:04", hm)
		return time.Date(2026, 10, 22, tm.Hour(), tm.Minute(), 0, 0, time.Local)
	}
	cases := []struct {
		now, start, end string
		want            bool
	}{
		{"03:00", "", "", true},
		{"03:00", "01:00", "05:00", true},
		{"05:00", "01:00", "05:00", false},
		{"00:59", "01:00", "05:00", false},
		{"23:30", "22:00", "06:00", true},
		{"05:59", "22:00", "06:00", true},
		{"12:00", "22:00", "06:00", false},
	}
	for _, c := range cases {
		if got := inWindow(at(c.now), c.start, c.end); got != c.want {
			t.Errorf("inWindow(%s, %s-%s) = %v, want %v", c.now, c.start, c.end, got, c.want)
		}
	}
}

func TestValidateSettings(t *testing.T) {
	for _, bad := range []Settings{
		{InactiveMinutes: -1},
		{WindowStart: "01:00"},
		{WindowStart: "25:00", WindowEnd: "05:00"},
	} {
		if bad.Validate() == nil {
			t.Errorf("expected %+v to be invalid", bad)
		}
	}
	if err := (Settings{InactiveMinutes: 30, WindowStart: "22:00", WindowEnd: "06:00"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
