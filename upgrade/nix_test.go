package upgrade

import (
	"reflect"
	"testing"
)

func TestParseNixProfileListV3(t *testing.T) {
	got, err := parseNixProfileList([]byte(`{"elements":{
		"flox":{"active":true,"attrPath":"packages.x86_64-linux.default","originalUrl":"github:flox/flox/latest","outputs":null,"priority":5,"storePaths":["/nix/store/rdyh48k2s5919hpc45ifkczws3668k2s-flox-1.17.0-g486737b"],"url":"github:flox/flox/486737b"},
		"local":{"active":true,"attrPath":"","originalUrl":"","storePaths":["/nix/store/x-local-1"]},
		"off":{"active":false,"attrPath":"legacyPackages.x86_64-linux.hello","originalUrl":"flake:nixpkgs","storePaths":["/nix/store/y-hello-2.12"]},
		"btop":{"attrPath":"legacyPackages.x86_64-linux.btop","originalUrl":"flake:nixpkgs","outputs":["out"],"storePaths":["/nix/store/z-btop-1.4.0"]}
	},"version":3}`))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range got {
		names = append(names, e.Name)
	}
	// Store-path installs and inactive entries can't be upgraded.
	if !reflect.DeepEqual(names, []string{"btop", "flox"}) {
		t.Fatalf("got %v", names)
	}
	if got[0].Outputs[0] != "out" || got[1].Outputs != nil {
		t.Fatalf("unexpected outputs: %+v", got)
	}
}

func TestParseNixProfileListV2(t *testing.T) {
	got, err := parseNixProfileList([]byte(`{"elements":[{"active":true,"attrPath":"legacyPackages.x86_64-linux.hello","originalUrl":"flake:nixpkgs","storePaths":["/nix/store/y-hello-2.12"]}],"version":2}`))
	if err != nil || len(got) != 1 || got[0].Name != "hello" {
		t.Fatalf("got %+v %v", got, err)
	}
	if got, err := parseNixProfileList([]byte(`{"elements":{},"version":3}`)); err != nil || len(got) != 0 {
		t.Fatalf("empty profile: %+v %v", got, err)
	}
	if _, err := parseNixProfileList([]byte(`not json`)); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDrvVersion(t *testing.T) {
	for name, want := range map[string]string{
		"flox-1.17.0-g486737b": "1.17.0-g486737b",
		"hello-2.12.1":         "2.12.1",
		"nix-index-unstable":   "",
		"python3.12-foo-1.0":   "1.0",
		"noversion":            "",
	} {
		if got := drvVersion(name); got != want {
			t.Errorf("drvVersion(%q) = %q, want %q", name, got, want)
		}
	}
	if got := storePathName("/nix/store/rdyh48k2s5919hpc45ifkczws3668k2s-flox-1.17.0"); got != "flox-1.17.0" {
		t.Fatalf("got %q", got)
	}
}

func TestNixUpdateFor(t *testing.T) {
	e := nixProfileElement{Name: "flox", OriginalURL: "github:flox/flox/latest",
		StorePaths: []string{"/nix/store/a-flox-1.17.0-g486737b"}}

	u, ok := nixUpdateFor(e, nixEvalResult{Name: "flox-1.19.0-ga6a3147", Version: "1.19.0-ga6a3147",
		OutPaths: []string{"/nix/store/b-flox-1.19.0-ga6a3147"}})
	want := PackageUpdate{ID: "nix:flox", Name: "flox", Source: "Nix (github:flox/flox/latest)", Backend: "nix",
		Kind: KindPackage, Icon: "package-x-generic", OldVersion: "1.17.0-g486737b", NewVersion: "1.19.0-ga6a3147"}
	if !ok || u != want {
		t.Fatalf("got %+v %v", u, ok)
	}

	// Same store paths: nothing to upgrade.
	if _, ok := nixUpdateFor(e, nixEvalResult{OutPaths: []string{"/nix/store/a-flox-1.17.0-g486737b"}}); ok {
		t.Fatal("unchanged package reported as an update")
	}
	// Rebuild without a version bump still counts; version falls back to the name.
	u, ok = nixUpdateFor(e, nixEvalResult{Name: "flox-1.17.0-g486737b", OutPaths: []string{"/nix/store/c-flox-1.17.0-g486737b"}})
	if !ok || u.NewVersion != "1.17.0-g486737b" {
		t.Fatalf("got %+v %v", u, ok)
	}
	if _, ok := nixUpdateFor(e, nixEvalResult{Error: true}); ok {
		t.Fatal("failed evaluation reported as an update")
	}
}

func TestResolveNixApp(t *testing.T) {
	store := "/nix/store/h-warp-7.2"
	read := fakeFiles(map[string]string{
		store + "/share/metainfo/dev.warp.Warp.metainfo.xml": `<component type="desktop-application"><id>dev.warp.Warp</id><name>Warp</name><launchable type="desktop-id">dev.warp.Warp.desktop</launchable></component>`,
		store + "/share/applications/dev.warp.Warp.desktop":  "[Desktop Entry]\nType=Application\nName=Warp\nIcon=dev.warp.Warp",
	})
	files := &rpmAppFiles{
		Metainfo:        []string{store + "/share/metainfo/dev.warp.Warp.metainfo.xml"},
		Desktop:         []string{store + "/share/applications/dev.warp.Warp.desktop"},
		ApplicationsDir: store + "/share/applications",
	}
	if app, ok := resolveApp("warp", files, read); !ok || app != (appInfo{ID: "dev.warp.Warp", Name: "Warp", Icon: "dev.warp.Warp"}) {
		t.Fatalf("got %+v %v", app, ok)
	}
}

func TestDeterminateNixVersions(t *testing.T) {
	installed, latest := determinateNixVersions("Determinate Nixd daemon version: 3.22.5\n" +
		"Determinate Nixd client version: 3.22.5\nLatest version: 3.23.3\n\nA new version of Determinate Nix is available.\n")
	if installed != "3.22.5" || latest != "3.23.3" {
		t.Fatalf("got %q %q", installed, latest)
	}
	if installed, latest := determinateNixVersions("Determinate Nixd client version: 3.22.5\n"); installed != "3.22.5" || latest != "" {
		t.Fatalf("offline: got %q %q", installed, latest)
	}
}

func TestFindExecutablePrefersKnownPaths(t *testing.T) {
	dir := t.TempDir()
	if got := findExecutable("definitely-not-on-path-um", dir+"/missing"); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := findExecutable("sh", dir); got == dir || got == "" {
		t.Fatalf("directory should be skipped and PATH used, got %q", got)
	}
}
