package upgrade

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

func TestParseDnfCheckUpdateNoUpdates(t *testing.T) {
	if got := parseDnfCheckUpdate(""); len(got) != 0 {
		t.Fatalf("expected no updates, got %+v", got)
	}
}

func TestRpmPackageName(t *testing.T) {
	if got := rpmPackageName("python3.12-libs.x86_64"); got != "python3.12-libs" {
		t.Fatalf("got %q", got)
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
