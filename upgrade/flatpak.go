package upgrade

import (
	"strconv"
	"strings"

	"github.com/Ultramarine-Linux/um/pkg/util"
)

// flatpakUpdate is a single pending update parsed from `flatpak remote-ls --updates`.
type flatpakUpdate struct {
	appID, name, version, branch, origin string
	// ref is e.g. "app/org.kde.haruna/x86_64/stable" or "runtime/...".
	ref string
}

func parseFlatpakRemoteLs(output string) []flatpakUpdate {
	var updates []flatpakUpdate
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 5 || fields[0] == "" || fields[0] == "Application ID" {
			continue
		}
		u := flatpakUpdate{appID: fields[0], name: fields[1], version: fields[2], branch: fields[3], origin: fields[4]}
		if len(fields) > 5 {
			u.ref = fields[5]
		}
		updates = append(updates, u)
	}
	return updates
}

// flatpakInstallations are queried separately so updates can be attributed
// to (and applied in) the right installation.
var flatpakInstallations = []string{"system", "user"}

// installedFlatpakVersions maps "appID/branch" to the installed version for
// one installation ("system" or "user").
func installedFlatpakVersions(installation string) map[string]string {
	versions := map[string]string{}
	output, err := util.CommandOutput("flatpak", "list", "--"+installation, "--columns=application,branch,version")
	if err != nil {
		return versions
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 || fields[0] == "" {
			continue
		}
		versions[fields[0]+"/"+fields[1]] = fields[2]
	}
	return versions
}

func flatpakPackageUpdates(installation string) ([]PackageUpdate, error) {
	output, err := util.CommandOutput("flatpak", "remote-ls", "--"+installation, "--updates",
		"--columns=application,name,version,branch,origin,ref")
	if err != nil {
		return nil, err
	}
	updates := parseFlatpakRemoteLs(output)
	if len(updates) == 0 {
		return nil, nil
	}
	installed := installedFlatpakVersions(installation)
	result := make([]PackageUpdate, 0, len(updates))
	for _, u := range updates {
		result = append(result, flatpakPackageUpdate(u, installation, installed))
	}
	annotateFlatpakSizes(result, updates, installation)
	return result, nil
}

func appIDIfApp(kind, appID string) string {
	if kind == KindApp {
		return appID
	}
	return ""
}

func flatpakPackageUpdate(u flatpakUpdate, installation string, installed map[string]string) PackageUpdate {
	name := u.name
	if name == "" {
		name = u.appID
	}
	source := "Flatpak"
	if u.origin != "" {
		source += " (" + u.origin + ")"
	}
	if installation != "system" {
		source += " [" + installation + "]"
	}
	// The same app ID can appear for several branches (e.g. runtime
	// extensions) and in both installations, so qualify the ID.
	id := u.appID + "//" + u.branch
	if installation != "system" {
		id += "@" + installation
	}
	kind := KindRuntime
	if strings.HasPrefix(u.ref, "app/") {
		kind = KindApp
	}
	// Flatpak apps export their icon under their application ID.
	return PackageUpdate{
		ID: id, Name: name, Source: source, Backend: "flatpak", Kind: kind,
		AppID:      appIDIfApp(kind, u.appID),
		OldVersion: installed[u.appID+"/"+u.branch], NewVersion: u.version, Icon: u.appID,
	}
}

// parseSISize parses GLib's formatted sizes as printed by flatpak with the C
// locale: "512 bytes", "12.0 kB", "418.7 MB", "1.1 GB" (powers of 1000).
func parseSISize(s string) int64 {
	// GLib separates the number and unit with a non-breaking space.
	s = strings.ReplaceAll(s, "\u00a0", " ")
	value, unit, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok {
		return 0
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	multipliers := map[string]float64{
		"byte": 1, "bytes": 1, "kB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12,
	}
	m, ok := multipliers[unit]
	if !ok {
		return 0
	}
	return int64(n * m)
}

// parseFlatpakSizes parses "application\tbranch\tsize[\tsize]" lines into
// "application/branch" -> sizes.
func parseFlatpakSizes(output string) map[string]upgradeSize {
	sizes := map[string]upgradeSize{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 || fields[0] == "" {
			continue
		}
		s := upgradeSize{download: parseSISize(fields[2])}
		if len(fields) > 3 {
			s.installed = parseSISize(fields[3])
		}
		sizes[fields[0]+"/"+fields[1]] = s
	}
	return sizes
}

// annotateFlatpakSizes fills in sizes for Flatpak updates. Flatpak reports the
// full download size; with static deltas the actual download can be smaller.
func annotateFlatpakSizes(updates []PackageUpdate, refs []flatpakUpdate, installation string) {
	if len(updates) == 0 {
		return
	}
	remote := parseFlatpakSizes(localeIndependentOutput("flatpak", "remote-ls", "--"+installation, "--updates",
		"--columns=application,branch,download-size,installed-size"))
	local := parseFlatpakSizes(localeIndependentOutput("flatpak", "list", "--"+installation,
		"--columns=application,branch,size"))
	for i := range updates {
		key := refs[i].appID + "/" + refs[i].branch
		s, ok := remote[key]
		if !ok {
			continue
		}
		updates[i].DownloadSize = s.download
		if s.installed > 0 {
			updates[i].InstalledSizeChange = s.installed - local[key].download
		}
	}
}
