package upgrade

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseDesktopEntry(t *testing.T) {
	entry, ok := parseDesktopEntry(`[Desktop Entry]
Type=Application
Name=LibreOffice Writer
Name[de]=LibreOffice Writer DE
Icon=libreoffice-writer

[Desktop Action new]
Name=New Document
Icon=other`)
	if !ok || entry != (desktopEntry{Name: "LibreOffice Writer", Icon: "libreoffice-writer"}) {
		t.Fatalf("got %+v %v", entry, ok)
	}

	for _, hidden := range []string{
		"[Desktop Entry]\nType=Application\nName=X\nNoDisplay=true",
		"[Desktop Entry]\nType=Application\nName=X\nHidden=true",
		"[Desktop Entry]\nType=Link\nName=X",
		"[Desktop Entry]\nType=Application\nIcon=x",
	} {
		if _, ok := parseDesktopEntry(hidden); ok {
			t.Fatalf("expected %q to be rejected", hidden)
		}
	}
}

func TestParseMetainfo(t *testing.T) {
	info, ok := parseMetainfo([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<component type="desktop-application">
  <id>org.mozilla.firefox</id>
  <name xml:lang="de">Feuerfuchs</name>
  <name>Firefox</name>
  <icon type="remote">https://example.org/firefox.png</icon>
  <icon type="stock">firefox</icon>
  <launchable type="desktop-id">org.mozilla.firefox.desktop</launchable>
  <provides><id>firefox.desktop</id></provides>
</component>`))
	want := metainfo{ID: "org.mozilla.firefox", Name: "Firefox", Icon: "firefox", DesktopIDs: []string{"org.mozilla.firefox.desktop"}}
	if !ok || !reflect.DeepEqual(info, want) {
		t.Fatalf("got %+v %v", info, ok)
	}

	// Legacy: type="desktop", no <launchable>, ID already ends in .desktop.
	info, ok = parseMetainfo([]byte(`<component type="desktop"><id>org.kde.dolphin.desktop</id><name>Dolphin</name></component>`))
	if !ok || info.Name != "Dolphin" || !reflect.DeepEqual(info.DesktopIDs, []string{"org.kde.dolphin.desktop"}) {
		t.Fatalf("got %+v %v", info, ok)
	}

	for _, notApp := range []string{
		`<component type="console-application"><id>dev.linderud.sbctl</id><name>sbctl</name></component>`,
		`<component type="addon"><id>org.kde.dolphin.plugin</id><name>Plugin</name></component>`,
		`<component type="desktop-application"><id>x</id></component>`,
		`not xml`,
	} {
		if _, ok := parseMetainfo([]byte(notApp)); ok {
			t.Fatalf("expected %q to be rejected", notApp)
		}
	}
}

func TestParseRpmAppFiles(t *testing.T) {
	got := parseRpmAppFiles("firefox.x86_64\t/usr/bin/firefox\n" +
		"firefox.x86_64\t/usr/share/metainfo/org.mozilla.firefox.appdata.xml\n" +
		"firefox.x86_64\t/usr/share/applications/org.mozilla.firefox.desktop\n" +
		"old.x86_64\t/usr/share/appdata/old.appdata.xml\n" +
		"foo.x86_64\t/usr/share/applications/sub/nested.desktop\n" +
		"foo.x86_64\t/usr/share/applications/foo.png")
	want := map[string]*rpmAppFiles{
		"firefox.x86_64": {
			Metainfo: []string{"/usr/share/metainfo/org.mozilla.firefox.appdata.xml"},
			Desktop:  []string{"/usr/share/applications/org.mozilla.firefox.desktop"},
		},
		"old.x86_64": {Metainfo: []string{"/usr/share/appdata/old.appdata.xml"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func fakeFiles(files map[string]string) fileReader {
	return func(path string) ([]byte, error) {
		if c, ok := files[path]; ok {
			return []byte(c), nil
		}
		return nil, errors.New("missing")
	}
}

func TestResolveAppFromMetainfo(t *testing.T) {
	read := fakeFiles(map[string]string{
		"/usr/share/metainfo/org.kde.dolphin.appdata.xml":     `<component type="desktop"><id>org.kde.dolphin.desktop</id><name>Dolphin</name></component>`,
		"/usr/share/metainfo/org.kde.dolphin.plugin.xml":      `<component type="addon"><id>org.kde.dolphin.plugin</id><name>Plugin</name></component>`,
		"/usr/share/applications/org.kde.dolphin.desktop":     "[Desktop Entry]\nType=Application\nName=Dolphin (launcher)\nIcon=system-file-manager",
		"/usr/share/metainfo/dev.linderud.sbctl.metainfo.xml": `<component type="console-application"><id>dev.linderud.sbctl</id><name>sbctl</name></component>`,
		"/usr/share/applications/sbctl-helper.desktop":        "[Desktop Entry]\nType=Application\nName=Helper",
	})

	app, ok := resolveApp("dolphin", &rpmAppFiles{
		Metainfo: []string{"/usr/share/metainfo/org.kde.dolphin.plugin.xml", "/usr/share/metainfo/org.kde.dolphin.appdata.xml"},
	}, read)
	// Name and ID from metainfo, icon from the launchable desktop file.
	if !ok || app != (appInfo{ID: "org.kde.dolphin.desktop", Name: "Dolphin", Icon: "system-file-manager"}) {
		t.Fatalf("got %+v %v", app, ok)
	}

	// Metainfo that isn't a desktop application wins over a stray launcher.
	if app, ok := resolveApp("sbctl", &rpmAppFiles{
		Metainfo: []string{"/usr/share/metainfo/dev.linderud.sbctl.metainfo.xml"},
		Desktop:  []string{"/usr/share/applications/sbctl-helper.desktop"},
	}, read); ok {
		t.Fatalf("console application should not be an app: %+v", app)
	}
}

func TestResolveAppFallsBackToLaunchers(t *testing.T) {
	read := fakeFiles(map[string]string{
		"/usr/share/applications/a-helper.desktop":  "[Desktop Entry]\nType=Application\nName=Helper\nIcon=h",
		"/usr/share/applications/hidden.desktop":    "[Desktop Entry]\nType=Application\nName=Hidden\nNoDisplay=true",
		"/usr/share/applications/1password.desktop": "[Desktop Entry]\nType=Application\nName=1Password\nIcon=1password",
	})
	files := &rpmAppFiles{Desktop: []string{
		"/usr/share/applications/a-helper.desktop", "/usr/share/applications/hidden.desktop",
		"/usr/share/applications/1password.desktop", "/usr/share/applications/gone.desktop",
	}}

	if app, ok := resolveApp("1password", files, read); !ok || app != (appInfo{Name: "1Password", Icon: "1password"}) {
		t.Fatalf("got %+v %v", app, ok)
	}
	if app, ok := resolveApp("other", files, read); !ok || app.Name != "Helper" {
		t.Fatalf("got %+v %v", app, ok)
	}
	if _, ok := resolveApp("x", &rpmAppFiles{Desktop: []string{"/usr/share/applications/hidden.desktop"}}, read); ok {
		t.Fatal("hidden launcher should not make a package an app")
	}
	if _, ok := resolveApp("x", nil, read); ok {
		t.Fatal("package without app files should not be an app")
	}
}
