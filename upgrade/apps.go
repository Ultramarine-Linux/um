package upgrade

import (
	"bufio"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const applicationsDir = "/usr/share/applications"

// metainfoDirs are where packages install AppStream metainfo.
// /usr/share/appdata is the legacy location.
var metainfoDirs = []string{"/usr/share/metainfo", "/usr/share/appdata"}

// appInfo is what an update needs to be presented as an app.
type appInfo struct {
	// ID is the AppStream component ID, if the app has metainfo.
	ID   string
	Name string
	Icon string
}

type metainfoText struct {
	Lang  string `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	Value string `xml:",chardata"`
}

type metainfoIcon struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type metainfoLaunchable struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type metainfoComponent struct {
	Type        string               `xml:"type,attr"`
	ID          string               `xml:"id"`
	Names       []metainfoText       `xml:"name"`
	Icons       []metainfoIcon       `xml:"icon"`
	Launchables []metainfoLaunchable `xml:"launchable"`
}

// metainfo is the subset of an AppStream metainfo file um uses.
type metainfo struct {
	ID string
	// Name is the untranslated name; um's output is not localized.
	Name string
	// Icon is a stock icon name or an absolute path, if the metainfo has one.
	// Local metainfo usually doesn't; the launcher's icon is used instead.
	Icon string
	// DesktopIDs are the launchers of the app ("org.kde.dolphin.desktop").
	DesktopIDs []string
}

// parseMetainfo parses a metainfo file. ok is false unless it describes a
// desktop application (type "desktop-application", or the legacy "desktop").
func parseMetainfo(content []byte) (info metainfo, ok bool) {
	var c metainfoComponent
	if err := xml.Unmarshal(content, &c); err != nil {
		return metainfo{}, false
	}
	if c.Type != "desktop-application" && c.Type != "desktop" {
		return metainfo{}, false
	}
	info.ID = strings.TrimSpace(c.ID)
	for _, name := range c.Names {
		if name.Lang == "" {
			info.Name = strings.TrimSpace(name.Value)
			break
		}
	}
	for _, icon := range c.Icons {
		value := strings.TrimSpace(icon.Value)
		// "cached" and "remote" icons refer to an AppStream catalog or a URL.
		if (icon.Type == "stock" || (icon.Type == "local" && filepath.IsAbs(value))) && value != "" {
			info.Icon = value
			break
		}
	}
	for _, l := range c.Launchables {
		if l.Type == "desktop-id" && strings.TrimSpace(l.Value) != "" {
			info.DesktopIDs = append(info.DesktopIDs, strings.TrimSpace(l.Value))
		}
	}
	// Legacy metainfo has no <launchable>; its ID is (or names) the desktop ID.
	if len(info.DesktopIDs) == 0 && info.ID != "" {
		id := info.ID
		if !strings.HasSuffix(id, ".desktop") {
			id += ".desktop"
		}
		info.DesktopIDs = append(info.DesktopIDs, id)
	}
	return info, info.ID != "" && info.Name != ""
}

// desktopEntry holds the fields of a .desktop file needed to present an app.
type desktopEntry struct {
	Name string
	Icon string
}

// parseDesktopEntry reads the [Desktop Entry] group of a .desktop file. ok is
// false for entries that aren't visible applications (NoDisplay, Hidden,
// Type other than Application, or no Name).
func parseDesktopEntry(content string) (entry desktopEntry, ok bool) {
	inGroup := false
	visible := true
	isApplication := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inGroup = line == "[Desktop Entry]"
			continue
		}
		if !inGroup {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		// Localized keys (Name[de]=...) are skipped: um's output is not localized.
		switch strings.TrimSpace(key) {
		case "Name":
			entry.Name = strings.TrimSpace(value)
		case "Icon":
			entry.Icon = strings.TrimSpace(value)
		case "Type":
			isApplication = strings.TrimSpace(value) == "Application"
		case "NoDisplay", "Hidden":
			if strings.TrimSpace(value) == "true" {
				visible = false
			}
		}
	}
	return entry, visible && isApplication && entry.Name != ""
}

// rpmAppFiles are the files of one package that describe an app.
type rpmAppFiles struct {
	Metainfo []string
	Desktop  []string
	// ApplicationsDir is where the package's launchers live. Empty means
	// /usr/share/applications; Nix packages use <store path>/share/applications.
	ApplicationsDir string
}

// parseRpmAppFiles parses `rpm -q --qf '[%{=NAME}.%{=ARCH}\t%{FILENAMES}\n]'`
// output into "name.arch" -> its metainfo files and launchers, sorted.
func parseRpmAppFiles(output string) map[string]*rpmAppFiles {
	files := map[string]*rpmAppFiles{}
	get := func(nameArch string) *rpmAppFiles {
		if files[nameArch] == nil {
			files[nameArch] = &rpmAppFiles{}
		}
		return files[nameArch]
	}
	for _, line := range strings.Split(output, "\n") {
		nameArch, path, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		dir := filepath.Dir(path)
		switch {
		case dir == applicationsDir && strings.HasSuffix(path, ".desktop"):
			get(nameArch).Desktop = append(get(nameArch).Desktop, path)
		case isMetainfoDir(dir) && strings.HasSuffix(path, ".xml"):
			get(nameArch).Metainfo = append(get(nameArch).Metainfo, path)
		}
	}
	for _, f := range files {
		sort.Strings(f.Metainfo)
		sort.Strings(f.Desktop)
	}
	return files
}

func isMetainfoDir(dir string) bool {
	for _, d := range metainfoDirs {
		if dir == d {
			return true
		}
	}
	return false
}

// rpmAppFilesFor looks up the app files of the given packages ("name.arch")
// with a single rpm query.
func rpmAppFilesFor(nameArchs []string) map[string]*rpmAppFiles {
	if len(nameArchs) == 0 {
		return nil
	}
	args := append([]string{"-q", "--qf", "[%{=NAME}.%{=ARCH}\t%{FILENAMES}\n]"}, nameArchs...)
	// rpm exits with the number of packages that aren't installed (e.g. new
	// dependencies). The output for the others is still valid, so use it anyway.
	output, _ := exec.Command("rpm", args...).Output()
	return parseRpmAppFiles(string(output))
}

// matchesPackage reports whether a file named after an app ID belongs to the
// package's "main" app: "firefox" matches "firefox.desktop" and
// "org.mozilla.firefox.appdata.xml".
func matchesPackage(packageName, path string) bool {
	base := filepath.Base(path)
	for _, suffix := range []string{".desktop", ".metainfo.xml", ".appdata.xml", ".xml"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	return base == packageName || strings.HasSuffix(base, "."+packageName)
}

// preferPackageMatch moves paths named after the package to the front.
func preferPackageMatch(packageName string, paths []string) []string {
	sorted := append([]string(nil), paths...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return matchesPackage(packageName, sorted[i]) && !matchesPackage(packageName, sorted[j])
	})
	return sorted
}

// fileReader reads a file; replaced in tests.
type fileReader func(path string) ([]byte, error)

// resolveApp decides whether a package is a desktop app.
//
// AppStream metainfo is authoritative: a package is an app if it ships a
// desktop-application component, with the name and ID from the metainfo and
// the icon from the metainfo or its launcher. Packages that ship metainfo but
// no desktop application (CLI tools, addons, ...) are not apps. Only packages
// without any metainfo, common for third-party RPMs, fall back to their
// visible launchers.
func resolveApp(packageName string, files *rpmAppFiles, read fileReader) (appInfo, bool) {
	if files == nil {
		return appInfo{}, false
	}

	appsDir := files.ApplicationsDir
	if appsDir == "" {
		appsDir = applicationsDir
	}
	launcher := func(desktopIDs []string) (desktopEntry, bool) {
		for _, id := range desktopIDs {
			path := filepath.Join(appsDir, id)
			if content, err := read(path); err == nil {
				if entry, ok := parseDesktopEntry(string(content)); ok {
					return entry, true
				}
			}
		}
		return desktopEntry{}, false
	}

	for _, path := range preferPackageMatch(packageName, files.Metainfo) {
		content, err := read(path)
		if err != nil {
			continue
		}
		info, ok := parseMetainfo(content)
		if !ok {
			continue
		}
		app := appInfo{ID: info.ID, Name: info.Name, Icon: info.Icon}
		if app.Icon == "" {
			if entry, ok := launcher(info.DesktopIDs); ok {
				app.Icon = entry.Icon
			}
		}
		return app, true
	}
	if len(files.Metainfo) > 0 {
		return appInfo{}, false
	}

	for _, path := range preferPackageMatch(packageName, files.Desktop) {
		if entry, ok := launcher([]string{filepath.Base(path)}); ok {
			return appInfo{Name: entry.Name, Icon: entry.Icon}, true
		}
	}
	return appInfo{}, false
}

// annotateRpmApps marks RPM updates that are desktop apps, using the app's
// name, AppStream ID and icon so frontends can show them like Flatpaks.
func annotateRpmApps(updates []PackageUpdate) {
	nameArchs := make([]string, 0, len(updates))
	for _, u := range updates {
		nameArchs = append(nameArchs, u.ID)
	}
	appFiles := rpmAppFilesFor(nameArchs)
	for i := range updates {
		app, ok := resolveApp(rpmPackageName(updates[i].ID), appFiles[updates[i].ID], os.ReadFile)
		if !ok {
			continue
		}
		updates[i].Kind = KindApp
		updates[i].AppID = app.ID
		updates[i].Name = app.Name
		if app.Icon != "" {
			updates[i].Icon = app.Icon
		}
	}
}
