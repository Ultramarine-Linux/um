package tweaks

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func mustEnabled(t *testing.T, tw Tweak) bool {
	t.Helper()
	enabled, err := tw.Enabled()
	if err != nil {
		t.Fatal(err)
	}
	return enabled
}

func TestEnabledStateRoundTrip(t *testing.T) {
	t.Setenv("UM_STATE", t.TempDir())

	toggle := Tweak{TweakType: TweakTypeToggle, Path: "/usr/share/um/tweaks/example"}
	if mustEnabled(t, toggle) {
		t.Fatal("tweak should start disabled")
	}
	if err := toggle.setEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !mustEnabled(t, toggle) {
		t.Fatal("tweak should be enabled")
	}
	info, err := os.Stat(filepath.Join(StatePath(), "example.enabled"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0004 == 0 {
		t.Fatal("state marker must be world-readable for unprivileged frontends")
	}
	if err := toggle.setEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := toggle.setEnabled(false); err != nil {
		t.Fatalf("disabling twice should be a no-op: %v", err)
	}
	if mustEnabled(t, toggle) {
		t.Fatal("tweak should be disabled")
	}
}

func TestScriptTweaksHaveNoState(t *testing.T) {
	t.Setenv("UM_STATE", t.TempDir())
	script := Tweak{TweakType: TweakTypeScript, Path: "/x/script"}
	if err := (Tweak{TweakType: TweakTypeToggle, Path: "/x/script"}).setEnabled(true); err != nil {
		t.Fatal(err)
	}
	if mustEnabled(t, script) {
		t.Fatal("script tweaks should never report enabled")
	}
}

func TestStateDirPermissionsAreFixed(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	t.Setenv("UM_STATE", state)
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	if err := (Tweak{TweakType: TweakTypeToggle, Path: "/x/a"}).setEnabled(true); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{state, StatePath()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0055 != 0055 {
			t.Fatalf("%s must be readable and traversable by everyone, got %v", dir, info.Mode().Perm())
		}
	}
}

func TestMarkerRefusesSymlinks(t *testing.T) {
	t.Setenv("UM_STATE", t.TempDir())
	target := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(target, []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(StatePath(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(StatePath(), "a.enabled")); err != nil {
		t.Fatal(err)
	}
	if err := (Tweak{TweakType: TweakTypeToggle, Path: "/x/a"}).setEnabled(true); err == nil {
		t.Fatal("writing a marker through a symlink should fail")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(target)
	if string(data) != "keep me" || info.Mode().Perm() != 0600 {
		t.Fatal("symlink target was modified")
	}
}

func TestUnreadableStateIsUnknown(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read everything")
	}
	state := t.TempDir()
	t.Setenv("UM_STATE", state)
	if err := os.Chmod(state, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(state, 0755) })
	if _, err := (Tweak{TweakType: TweakTypeToggle, Path: "/x/a"}).Enabled(); err == nil {
		t.Fatal("an unreadable state dir should be reported as an error, not as disabled")
	}
}

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestCheckScript(t *testing.T) {
	t.Setenv("UM_STATE", t.TempDir())
	dir := t.TempDir()
	tw := Tweak{TweakType: TweakTypeToggle, Path: dir}

	for _, tc := range []struct {
		body    string
		enabled bool
		wantErr bool
	}{
		{"exit 0", true, false},
		{"exit 1", false, false},
		{"exit 100", false, false}, // unknown: falls back to the (absent) marker
		{"exit 2", false, true},    // shell/test(1) errors are reported
	} {
		writeScript(t, filepath.Join(dir, "check.sh"), tc.body)
		enabled, err := tw.Enabled()
		if (err != nil) != tc.wantErr || enabled != tc.enabled {
			t.Errorf("%q: got enabled=%v err=%v", tc.body, enabled, err)
		}
	}

	// When check.sh can't tell, the marker is used.
	writeScript(t, filepath.Join(dir, "check.sh"), "exit 100")
	if err := tw.setEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !mustEnabled(t, tw) {
		t.Fatal("an unknown check should fall back to the marker")
	}

	// check.sh wins over a stale marker.
	writeScript(t, filepath.Join(dir, "check.sh"), "exit 1")
	if err := tw.setEnabled(true); err != nil {
		t.Fatal(err)
	}
	if mustEnabled(t, tw) {
		t.Fatal("check.sh should be the source of truth")
	}
}

func TestValidateID(t *testing.T) {
	for _, id := range []string{"nix", "sudo-pwfeedback", "a.b"} {
		if err := ValidateID(id); err != nil {
			t.Errorf("%q should be valid: %v", id, err)
		}
	}
	for _, id := range []string{"", ".", "..", "../x", "../../../../tmp/owned", "a/b", "foo/.", "/etc"} {
		if err := ValidateID(id); err == nil {
			t.Errorf("%q should be rejected", id)
		}
		if _, err := LoadTweakId(id); err == nil {
			t.Errorf("LoadTweakId(%q) should fail", id)
		}
	}
}

func TestEnableDisable(t *testing.T) {
	t.Setenv("UM_STATE", t.TempDir())
	bin := t.TempDir()
	fake := filepath.Join(bin, "ansible-playbook")
	old := ansiblePlaybook
	ansiblePlaybook = fake
	t.Cleanup(func() { ansiblePlaybook = old })

	tw := Tweak{TweakType: TweakTypeToggle, Path: t.TempDir()}

	writeScript(t, fake, "exit 4")
	err := tw.Enable()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 4 {
		t.Fatalf("expected ansible's exit code 4, got %v", err)
	}
	if mustEnabled(t, tw) {
		t.Fatal("a failed enable must not record the tweak as enabled")
	}

	writeScript(t, fake, "exit 0")
	if err := tw.Enable(); err != nil {
		t.Fatal(err)
	}
	if !mustEnabled(t, tw) {
		t.Fatal("tweak should be enabled")
	}

	writeScript(t, fake, "exit 2")
	if err := tw.Disable(); err == nil {
		t.Fatal("expected disable to fail")
	}
	if !mustEnabled(t, tw) {
		t.Fatal("a failed disable must keep the tweak enabled")
	}

	writeScript(t, fake, "exit 0")
	if err := tw.Disable(); err != nil {
		t.Fatal(err)
	}
	if mustEnabled(t, tw) {
		t.Fatal("tweak should be disabled")
	}

	oneshot := Tweak{TweakType: TweakTypeOneshot, Path: tw.Path}
	if err := oneshot.Disable(); err == nil {
		t.Fatal("oneshot tweaks can't be disabled")
	}
}

func TestSortedEntries(t *testing.T) {
	t.Setenv("UM_STATE", t.TempDir())
	desc := "desc"
	stable := Stable
	entries := SortedEntries(map[string]Tweak{
		"zeta":  {TweakType: TweakTypeScript, Path: "/x/zeta"},
		"alpha": {TweakType: TweakTypeToggle, Description: &desc, Stability: &stable, Path: "/x/alpha"},
	})
	if len(entries) != 2 || entries[0].ID != "alpha" || entries[1].ID != "zeta" {
		t.Fatalf("unexpected order: %+v", entries)
	}
	if entries[0].Description != "desc" || entries[0].Stability != "stable" || entries[0].Type != "toggle" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	if entries[1].Type != "script" || entries[1].Stability != "" || entries[1].StateError != "" {
		t.Fatalf("unexpected entry: %+v", entries[1])
	}
}
