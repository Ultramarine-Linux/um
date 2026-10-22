package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Ultramarine-Linux/um/pkg/util"
	"github.com/acobaugh/osrelease"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type collectionsResponse struct {
	Collections []collection `json:"collections"`
}

type collection struct {
	AllowRetire bool   `json:"allow_retire"`
	Branchname  string `json:"branchname"`
	DateCreated string `json:"date_created"`
	DateUpdated string `json:"date_updated"`
	DistTag     string `json:"dist_tag"`
	KojiName    string `json:"koji_name"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Version     string `json:"version"`
}

var UpgradeEnvars = []string{
	"UM_DATA",
}

// getCurrentReleaseVersion reads /etc/os-release to safely discover what version the user is running
func getCurrentReleaseVersion() (string, error) {
	release, err := osrelease.Read()
	if err != nil {
		return "", err
	}
	return release["VERSION_ID"], nil
}

// getNextReleaseVersion calculates the next upgrade jump.
func getNextReleaseVersion(currentVersion string) (string, error) {
	const url = "https://ultramarine-linux.org/pkgdb/collections.json"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("fetch collections.json: http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var cr collectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", err
	}

	// Build a sorted list of active numeric versions.
	type vitem struct {
		raw string
		n   int
	}
	var active []vitem
	for _, c := range cr.Collections {
		if strings.TrimSpace(c.Status) != "Active" {
			continue
		}
		n, ok := parseMajorVersionInt(c.Version)
		if !ok {
			continue // skip "devel" or other non-numeric versions
		}
		active = append(active, vitem{raw: c.Version, n: n})
	}
	if len(active) == 0 {
		return "", errors.New("no Active numeric versions found in collections.json")
	}

	sort.Slice(active, func(i, j int) bool { return active[i].n < active[j].n })

	curN, curIsNum := parseMajorVersionInt(currentVersion)

	if curN == 0 {
		return "", fmt.Errorf("Cannot detect current version from /etc/os-release (VERSION_ID=%q)", currentVersion)
	}

	// If currentVersion is not numeric (e.g., "devel"), treat it as "before everything".
	if !curIsNum {
		return strconv.Itoa(active[0].n), nil
	}

	// Find the first Active version with n > current.
	for i := 0; i < len(active); i++ {
		if active[i].n > curN {
			return strconv.Itoa(active[i].n), nil
		}
	}

	return "", fmt.Errorf("no next Active version found greater than %q", currentVersion)
}

func parseMajorVersionInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// Versions in the JSON are like "42", "43", "44". Keep it strict.
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// runCommand routes standard I/O so DNF download progress animations are visible to the user
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

type PackageUpdate struct {
	// ID uniquely identifies the update: "name.arch" for RPMs, the
	// application ID for Flatpaks.
	ID         string `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source"`
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`
	Icon       string `json:"icon"`
}

type PackageUpdatesResponse struct {
	Updates []PackageUpdate `json:"updates"`
	// Errors lists sources that could not be queried. Updates from the
	// remaining sources are still reported.
	Errors []string `json:"errors"`
}

func commandOutput(name string, args ...string) (string, error) {
	return commandOutputWithExitCodes(name, nil, args...)
}

func commandOutputWithExitCodes(name string, allowedExitCodes []int, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			for _, allowedExitCode := range allowedExitCodes {
				if exitErr.ExitCode() == allowedExitCode {
					return strings.TrimSpace(string(output)), nil
				}
			}
			return "", fmt.Errorf("%s failed: %s", name, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

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
	output, err := commandOutput("rpm", "-qa", "--qf", "%{NAME}.%{ARCH}\t%{EVR}\t%{INSTALLTIME}\n")
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

// flatpakUpdate is a single pending update parsed from `flatpak remote-ls --updates`.
type flatpakUpdate struct {
	appID, name, version, branch, origin string
}

func parseFlatpakRemoteLs(output string) []flatpakUpdate {
	var updates []flatpakUpdate
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 5 || fields[0] == "" || fields[0] == "Application ID" {
			continue
		}
		updates = append(updates, flatpakUpdate{
			appID: fields[0], name: fields[1], version: fields[2], branch: fields[3], origin: fields[4],
		})
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
	output, err := commandOutput("flatpak", "list", "--"+installation, "--columns=application,branch,version")
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

func rpmPackageUpdates() ([]PackageUpdate, error) {
	// dnf check-update exits with 100 when updates are available. No
	// --refresh: let dnf decide when metadata has expired, otherwise every
	// (unprivileged, separately cached) listing re-downloads all metadata.
	output, err := commandOutputWithExitCodes("dnf", []int{100}, "check-update")
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
			OldVersion: installed[u.nameArch], NewVersion: u.version, Icon: "package-x-generic",
		})
	}
	return result, nil
}

func flatpakPackageUpdates(installation string) ([]PackageUpdate, error) {
	output, err := commandOutput("flatpak", "remote-ls", "--"+installation, "--updates",
		"--columns=application,name,version,branch,origin")
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
	return result, nil
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
	// Flatpak apps export their icon under their application ID.
	return PackageUpdate{
		ID: id, Name: name, Source: source,
		OldVersion: installed[u.appID+"/"+u.branch], NewVersion: u.version, Icon: u.appID,
	}
}

// packageUpdates collects updates from every available source. A failing
// source is recorded in Errors instead of hiding the other sources' updates.
func packageUpdates() PackageUpdatesResponse {
	response := PackageUpdatesResponse{Updates: []PackageUpdate{}, Errors: []string{}}
	collect := func(source string, updates []PackageUpdate, err error) {
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("%s: %v", source, err))
			return
		}
		response.Updates = append(response.Updates, updates...)
	}

	if _, err := exec.LookPath("dnf"); err == nil {
		updates, err := rpmPackageUpdates()
		collect("rpm", updates, err)
	}
	if _, err := exec.LookPath("flatpak"); err == nil {
		for _, installation := range flatpakInstallations {
			updates, err := flatpakPackageUpdates(installation)
			collect("flatpak "+installation, updates, err)
		}
	}
	return response
}

func upgradeList(c *cli.Context) error {
	updates := packageUpdates()
	if c.Bool("json") {
		if err := json.NewEncoder(os.Stdout).Encode(updates); err != nil {
			return err
		}
	} else {
		for _, update := range updates.Updates {
			fmt.Printf("%s: %s -> %s (%s)\n", update.Name, update.OldVersion, update.NewVersion, update.Source)
		}
		for _, e := range updates.Errors {
			fmt.Fprintf(os.Stderr, "warning: failed to list updates from %s\n", e)
		}
	}
	// Only fail when nothing could be queried at all.
	if len(updates.Errors) > 0 && len(updates.Updates) == 0 {
		return cli.Exit("failed to list updates", 1)
	}
	return nil
}

func upgradeApply(c *cli.Context) error {
	assumeYes := c.Bool("yes")
	_, flatpakErr := exec.LookPath("flatpak")
	hasFlatpak := flatpakErr == nil

	// User-installation Flatpaks must be updated as the invoking user: after
	// escalating, flatpak would only see root's (empty) user installation.
	if hasFlatpak && os.Getuid() != 0 {
		args := []string{"update", "--user"}
		if assumeYes {
			args = append(args, "--noninteractive")
		}
		if err := runCommand("flatpak", args...); err != nil {
			return cli.Exit(fmt.Sprintf("Flatpak user update failed: %v", err), 1)
		}
	}

	util.SudoIfNeeded(UpgradeEnvars)
	updated := false
	if _, err := exec.LookPath("dnf"); err == nil {
		args := []string{"upgrade", "--refresh"}
		if assumeYes {
			args = append(args, "-y")
		}
		if err := runCommand("dnf", args...); err != nil {
			return cli.Exit(fmt.Sprintf("RPM update failed: %v", err), 1)
		}
		updated = true
	}
	if hasFlatpak {
		args := []string{"update", "--system"}
		if assumeYes {
			args = append(args, "--noninteractive")
		}
		if err := runCommand("flatpak", args...); err != nil {
			return cli.Exit(fmt.Sprintf("Flatpak update failed: %v", err), 1)
		}
		updated = true
	}
	if !updated {
		return cli.Exit("no supported package manager found", 1)
	}
	return nil
}

func systemVersionUpgrade(c *cli.Context) error {
	// Elevate to root execution via your utility helper block early on
	util.SudoIfNeeded(UpgradeEnvars)

	currentVer, err := getCurrentReleaseVersion()
	if err != nil {
		return cli.Exit(fmt.Sprintf("Failed evaluating current local OS variant version: %v", err), 1)
	}

	targetVer, err := getNextReleaseVersion(currentVer)
	if err != nil {
		return cli.Exit(fmt.Sprintf("Failed resolving target update pathways: %v", err), 1)
	}

	// Hook handling for the --check dry-run flag execution
	if c.Bool("check") {
		if currentVer == targetVer {
			fmt.Printf("Your system is already up to date! Running Ultramarine %s.\n", currentVer)
		} else {
			fmt.Printf("A major system upgrade is available! Version %s -> %s\n", currentVer, targetVer)
		}
		return nil
	}

	if currentVer == targetVer {
		fmt.Println("No newer release branches detected. Your system is up to date!")
		return nil
	}

	yesFlag := c.Bool("yes")
	var confirm bool

	if yesFlag {
		confirm = true
	} else {
		description := fmt.Sprintf("This will transition your system from Ultramarine Linux %s to release version %s.\n"+
			"This action downloads substantial amounts of packages and requires a target reboot to execute fully.", currentVer, targetVer)

		err := huh.NewConfirm().
			Title("Do you want to initiate the system version upgrade?").
			Affirmative("Begin Upgrade").
			Negative("Abort").
			Description(description).
			Value(&confirm).
			Run()
		if err != nil {
			return err
		}
	}

	if !confirm {
		fmt.Println("Aborting version upgrade process...")
		return nil
	}

	fmt.Println("\n[*] Syncing and fully updating packages on your current release version...")
	if err := runCommand("dnf", "upgrade", "--refresh", "-y"); err != nil {
		return cli.Exit(fmt.Sprintf("DNF failed during pre-upgrade packages synchronization: %v", err), 1)
	}

	fmt.Printf("\n[*] Downloading upgrade tree metadata targets for Release %s...\n", targetVer)
	downloadArgs := []string{"system-upgrade", "download", fmt.Sprintf("--releasever=%s", targetVer), "-y"}
	if c.Bool("allowerasing") {
		downloadArgs = append(downloadArgs, "--allowerasing")
	}
	if err := runCommand("dnf", downloadArgs...); err != nil {
		return cli.Exit(fmt.Sprintf("DNF execution pipeline failed to pull systemic upgrade branches: %v", err), 1)
	}

	var rebootNow bool
	if yesFlag {
		rebootNow = true
	} else {
		fmt.Println()
		err := huh.NewConfirm().
			Title("All Done! Ready to Restart?").
			Affirmative("Restart and Upgrade").
			Negative("Restart Later").
			Value(&rebootNow).
			Run()
		if err != nil {
			return err
		}
	}

	if rebootNow {
		fmt.Println("\n[*] Activating system-upgrade trigger and rebooting system...")

		if err := runCommand("dnf", "system-upgrade", "reboot"); err != nil {
			return cli.Exit("Failed to reboot the machine for system upgrade. You may try running `sudo dnf system-upgrade reboot` manually.", 1)
		}
	} else {
		fmt.Println("\nUpgrade path staged successfully! To run the final deployment, execute: (expores in 12hrs or after reboot)")
		fmt.Println("sudo dnf system-upgrade reboot")
	}

	return nil
}
