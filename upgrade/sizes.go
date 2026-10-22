package upgrade

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type upgradeSize struct {
	download, installed int64
}

// localeIndependentOutput runs a command with the C locale, so sizes and
// numbers aren't localized (e.g. "1,1 GB").
func localeIndependentOutput(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	// Partial output is still useful (e.g. rpm -q with some packages missing).
	output, _ := cmd.Output()
	return string(output)
}

// parseTabSizes parses "key\tsize[\tsize]" lines with sizes in bytes.
func parseTabSizes(output string) map[string]upgradeSize {
	sizes := map[string]upgradeSize{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		var s upgradeSize
		s.download, _ = strconv.ParseInt(fields[1], 10, 64)
		if len(fields) > 2 {
			s.installed, _ = strconv.ParseInt(fields[2], 10, 64)
		}
		sizes[fields[0]] = s
	}
	return sizes
}
