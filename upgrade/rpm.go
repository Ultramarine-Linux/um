package upgrade

import (
	"strconv"
	"strings"

	"github.com/Ultramarine-Linux/um/pkg/util"
)

// rpmUpdate is a single pending update parsed from `dnf check-update`.
type rpmUpdate struct {
	nameArch, version, repo string
}

// parseDnfCheckUpdate parses `dnf check-update` output (dnf4 and dnf5).
// Only the list of upgrades is returned; the obsoletes section, headers and
// metadata/status lines are ignored.
func parseDnfCheckUpdate(output string) []rpmUpdate {
	var updates []rpmUpdate
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Obsoleting") {
			// Everything after this header lists obsoleted packages, not upgrades.
			break
		}
		// Continuation lines (e.g. wrapped or obsoleted entries) are indented.
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || !strings.Contains(fields[0], ".") || strings.HasSuffix(fields[0], ":") {
			continue
		}
		updates = append(updates, rpmUpdate{nameArch: fields[0], version: fields[1], repo: fields[2]})
	}
	return updates
}

// installedRpmVersions maps "name.arch" to the installed EVR. When several
// versions are installed (e.g. kernels), the most recently installed wins.
func installedRpmVersions() map[string]string {
	output, err := util.CommandOutput("rpm", "-qa", "--qf", "%{NAME}.%{ARCH}\t%{EVR}\t%{INSTALLTIME}\n")
	if err != nil {
		return map[string]string{}
	}
	return parseRpmInstalled(output)
}

func parseRpmInstalled(output string) map[string]string {
	versions := map[string]string{}
	installTimes := map[string]int64{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			continue
		}
		installTime, _ := strconv.ParseInt(fields[2], 10, 64)
		if prev, seen := installTimes[fields[0]]; seen && prev > installTime {
			continue
		}
		versions[fields[0]] = fields[1]
		installTimes[fields[0]] = installTime
	}
	return versions
}

func rpmPackageName(nameArch string) string {
	if i := strings.LastIndex(nameArch, "."); i > 0 {
		return nameArch[:i]
	}
	return nameArch
}

func rpmPackageUpdates() ([]PackageUpdate, error) {
	// dnf check-update exits with 100 when updates are available. No
	// --refresh: let dnf decide when metadata has expired, otherwise every
	// (unprivileged, separately cached) listing re-downloads all metadata.
	output, err := util.CommandOutputWithExitCodes("dnf", []int{100}, "check-update")
	if err != nil {
		return nil, err
	}
	updates := parseDnfCheckUpdate(output)
	if len(updates) == 0 {
		return nil, nil
	}
	installed := installedRpmVersions()
	result := make([]PackageUpdate, 0, len(updates))
	for _, u := range updates {
		result = append(result, PackageUpdate{
			ID: u.nameArch, Name: rpmPackageName(u.nameArch), Source: "RPM (" + u.repo + ")",
			Backend: "rpm", Kind: KindPackage,
			OldVersion: installed[u.nameArch], NewVersion: u.version, Icon: "package-x-generic",
		})
	}
	return result, nil
}
