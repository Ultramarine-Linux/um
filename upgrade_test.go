package main

import (
	"reflect"
	"testing"
)

func TestParseDnfCheckUpdateDnf5(t *testing.T) {
	output := `Updating and loading repositories:
Repositories loaded.
Upgrades
1password-cli.x86_64 2.41.1-1 1password
kernel-core.x86_64   6.11.4-300.fc41 updates`
	got := parseDnfCheckUpdate(output)
	want := []rpmUpdate{
		{"1password-cli.x86_64", "2.41.1-1", "1password"},
		{"kernel-core.x86_64", "6.11.4-300.fc41", "updates"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseDnfCheckUpdateIgnoresObsoletes(t *testing.T) {
	output := `Last metadata expiration check: 0:01:00 ago on Mon 01 Jan 2024.

foo.noarch                 1.2-3.fc41                 updates
Obsoleting packages
bar.x86_64                 2.0-1.fc41                 updates
    baz.x86_64             1.0-1.fc40                 @System`
	got := parseDnfCheckUpdate(output)
	want := []rpmUpdate{{"foo.noarch", "1.2-3.fc41", "updates"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseFlatpakRemoteLs(t *testing.T) {
	output := "org.kde.haruna\tHaruna\t1.8.1\tstable\tflathub\n" +
		"org.freedesktop.Platform.GL.default\tMesa\t26.2.2\t25.08\tflathub\n" +
		"short\tline"
	got := parseFlatpakRemoteLs(output)
	want := []flatpakUpdate{
		{"org.kde.haruna", "Haruna", "1.8.1", "stable", "flathub"},
		{"org.freedesktop.Platform.GL.default", "Mesa", "26.2.2", "25.08", "flathub"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRpmPackageName(t *testing.T) {
	if got := rpmPackageName("python3.12-libs.x86_64"); got != "python3.12-libs" {
		t.Fatalf("got %q", got)
	}
}

func TestParseDnfCheckUpdateNoUpdates(t *testing.T) {
	if got := parseDnfCheckUpdate(""); len(got) != 0 {
		t.Fatalf("expected no updates, got %+v", got)
	}
}

func TestParseFlatpakRemoteLsEmptyVersion(t *testing.T) {
	got := parseFlatpakRemoteLs("org.freedesktop.Platform.codecs-extra\tCodecs Extra Extension\t\t25.08-extra\tflathub")
	if len(got) != 1 || got[0].version != "" || got[0].branch != "25.08-extra" {
		t.Fatalf("unexpected parse: %+v", got)
	}
}

func TestParseRpmInstalledPrefersNewestInstall(t *testing.T) {
	got := parseRpmInstalled("kernel-core.x86_64\t6.11.4-300.fc41\t200\n" +
		"kernel-core.x86_64\t6.11.5-300.fc41\t300\n" +
		"kernel-core.x86_64\t6.11.3-300.fc41\t100\n" +
		"bash.x86_64\t5.2.32-1.fc41\t50")
	if got["kernel-core.x86_64"] != "6.11.5-300.fc41" || got["bash.x86_64"] != "5.2.32-1.fc41" {
		t.Fatalf("unexpected versions: %+v", got)
	}
}

func TestFlatpakPackageUpdateIDsAreUnique(t *testing.T) {
	installed := map[string]string{"org.freedesktop.Platform.GL.default/25.08": "26.2.1"}
	a := flatpakPackageUpdate(flatpakUpdate{"org.freedesktop.Platform.GL.default", "Mesa", "26.2.2", "25.08", "flathub"}, "system", installed)
	b := flatpakPackageUpdate(flatpakUpdate{"org.freedesktop.Platform.GL.default", "Mesa", "26.2.2", "24.08", "flathub"}, "system", installed)
	c := flatpakPackageUpdate(flatpakUpdate{"org.freedesktop.Platform.GL.default", "Mesa", "26.2.2", "25.08", "flathub"}, "user", nil)
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
