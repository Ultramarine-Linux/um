package tweaks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnabledStateRoundTrip(t *testing.T) {
	t.Setenv("UM_STATE", t.TempDir())

	toggle := Tweak{TweakType: TweakTypeToggle, Path: "/usr/share/um/tweaks/example"}
	if toggle.Enabled() {
		t.Fatal("tweak should start disabled")
	}
	if err := toggle.setEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !toggle.Enabled() {
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
	if toggle.Enabled() {
		t.Fatal("tweak should be disabled")
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
	if entries[1].Type != "script" || entries[1].Stability != "" {
		t.Fatalf("unexpected entry: %+v", entries[1])
	}
}
