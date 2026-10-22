package upgrade

import (
	"reflect"
	"testing"
)

func TestParseFlatpakRemoteLs(t *testing.T) {
	output := "org.kde.haruna\tHaruna\t1.8.1\tstable\tflathub\n" +
		"org.freedesktop.Platform.GL.default\tMesa\t26.2.2\t25.08\tflathub\n" +
		"short\tline"
	got := parseFlatpakRemoteLs(output)
	want := []flatpakUpdate{
		{"org.kde.haruna", "Haruna", "1.8.1", "stable", "flathub", ""},
		{"org.freedesktop.Platform.GL.default", "Mesa", "26.2.2", "25.08", "flathub", ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseFlatpakRemoteLsEmptyVersion(t *testing.T) {
	got := parseFlatpakRemoteLs("org.freedesktop.Platform.codecs-extra\tCodecs Extra Extension\t\t25.08-extra\tflathub")
	if len(got) != 1 || got[0].version != "" || got[0].branch != "25.08-extra" {
		t.Fatalf("unexpected parse: %+v", got)
	}
}

func TestFlatpakPackageUpdateIDsAreUnique(t *testing.T) {
	installed := map[string]string{"org.freedesktop.Platform.GL.default/25.08": "26.2.1"}
	a := flatpakPackageUpdate(flatpakUpdate{"org.freedesktop.Platform.GL.default", "Mesa", "26.2.2", "25.08", "flathub", "runtime/org.freedesktop.Platform.GL.default/x86_64/25.08"}, "system", installed)
	b := flatpakPackageUpdate(flatpakUpdate{"org.freedesktop.Platform.GL.default", "Mesa", "26.2.2", "24.08", "flathub", ""}, "system", installed)
	c := flatpakPackageUpdate(flatpakUpdate{"org.freedesktop.Platform.GL.default", "Mesa", "26.2.2", "25.08", "flathub", ""}, "user", nil)
	if a.ID == b.ID || a.ID == c.ID {
		t.Fatalf("IDs not unique: %q %q %q", a.ID, b.ID, c.ID)
	}
	if a.OldVersion != "26.2.1" || a.Icon != "org.freedesktop.Platform.GL.default" || a.Source != "Flatpak (flathub)" {
		t.Fatalf("unexpected update: %+v", a)
	}
	if c.Source != "Flatpak (flathub) [user]" {
		t.Fatalf("unexpected user source: %q", c.Source)
	}
}

func TestFlatpakKindFromRef(t *testing.T) {
	got := parseFlatpakRemoteLs("org.kde.haruna\tHaruna\t1.8.1\tstable\tflathub\tapp/org.kde.haruna/x86_64/stable\n" +
		"org.gnome.Platform\tGNOME\t\t51\tflathub\truntime/org.gnome.Platform/x86_64/51")
	if len(got) != 2 {
		t.Fatalf("unexpected parse: %+v", got)
	}
	app := flatpakPackageUpdate(got[0], "system", nil)
	runtime := flatpakPackageUpdate(got[1], "system", nil)
	if app.Kind != KindApp || runtime.Kind != KindRuntime || app.Backend != "flatpak" {
		t.Fatalf("unexpected kinds: %+v %+v", app, runtime)
	}
}
