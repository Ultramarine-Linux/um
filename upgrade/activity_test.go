package upgrade

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSessionInUse(t *testing.T) {
	sessions := parseShowSessions(`Id=2
Name=cypress
Class=user
Type=wayland
Active=yes
IdleHint=no
LockedHint=yes

Id=3
Name=cypress
Class=manager
Type=unspecified
Active=yes
IdleHint=no
LockedHint=no

Id=5
Name=guest
Class=user
Type=x11
Active=no
IdleHint=no
LockedHint=no

Id=7
Name=admin
Class=user
Type=tty
Active=yes
IdleHint=no
LockedHint=no
`)
	if len(sessions) != 4 {
		t.Fatalf("got %+v", sessions)
	}
	// Locked graphical session, user manager and inactive session don't count.
	if _, ok := sessionInUse(sessions, "cypress"); ok {
		t.Fatal("cypress is not using the device")
	}
	if s, ok := sessionInUse(sessions, ""); !ok || s.ID != "7" {
		t.Fatalf("admin's console session is in use: %+v %v", s, ok)
	}
	idle := loginSession{Class: "user", Type: "wayland", Active: true, Idle: true}
	if idle.inUse() {
		t.Fatal("idle session is not in use")
	}
}

func TestTimestamps(t *testing.T) {
	stamp := filepath.Join(t.TempDir(), "x", "stamp")
	now := time.Unix(1_800_000_000, 0)
	if within(stamp, time.Hour, now) {
		t.Fatal("missing stamp")
	}
	if err := writeTimestamp(stamp, now.Add(-30*time.Minute)); err != nil || !within(stamp, time.Hour, now) {
		t.Fatalf("stamp not written: %v", err)
	}
	if within(stamp, 10*time.Minute, now) {
		t.Fatal("old stamp counts as recent")
	}
	// A stamp from the future (clock changes) doesn't block forever.
	writeTimestamp(stamp, now.Add(time.Hour))
	if within(stamp, 24*time.Hour, now) {
		t.Fatal("future stamp counts as recent")
	}
}

func TestActivityTracker(t *testing.T) {
	a := activityTracker{filepath.Join(t.TempDir(), "inactive-since")}
	start := time.Unix(1_800_000_000, 0)
	if got := a.inactiveFor(false, start); got != 0 {
		t.Fatalf("first inactive poll: %v", got)
	}
	if got := a.inactiveFor(false, start.Add(25*time.Minute)); got != 25*time.Minute {
		t.Fatalf("got %v", got)
	}
	// Using the device resets the count.
	if got := a.inactiveFor(true, start.Add(30*time.Minute)); got != 0 {
		t.Fatalf("in use: %v", got)
	}
	if got := a.inactiveFor(false, start.Add(35*time.Minute)); got != 0 {
		t.Fatalf("after use: %v", got)
	}
	if got := a.inactiveFor(false, start.Add(40*time.Minute)); got != 5*time.Minute {
		t.Fatalf("got %v", got)
	}
}
