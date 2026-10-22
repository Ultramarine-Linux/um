package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Ultramarine-Linux/um/pkg/util"
	"github.com/Ultramarine-Linux/um/upgrade"
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

func upgradeList(c *cli.Context) error {
	updates := upgrade.List()
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

	// User-level updates must run as the invoking user, before escalating.
	if os.Getuid() != 0 {
		if err := upgrade.ApplyUser(assumeYes); err != nil {
			return cli.Exit(err.Error(), 1)
		}
	}

	util.SudoIfNeeded(UpgradeEnvars)
	updated, err := upgrade.ApplySystem(assumeYes, false)
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}
	if !updated {
		return cli.Exit("no supported package manager found", 1)
	}
	return nil
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
	defer resp.Body.Close()

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
	if err := util.RunCommand("dnf", "upgrade", "--refresh", "-y"); err != nil {
		return cli.Exit(fmt.Sprintf("DNF failed during pre-upgrade packages synchronization: %v", err), 1)
	}

	fmt.Printf("\n[*] Downloading upgrade tree metadata targets for Release %s...\n", targetVer)
	downloadArgs := []string{"system-upgrade", "download", fmt.Sprintf("--releasever=%s", targetVer), "-y"}
	if c.Bool("allowerasing") {
		downloadArgs = append(downloadArgs, "--allowerasing")
	}
	if err := util.RunCommand("dnf", downloadArgs...); err != nil {
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

		if err := util.RunCommand("dnf", "system-upgrade", "reboot"); err != nil {
			return cli.Exit("Failed to reboot the machine for system upgrade. You may try running `sudo dnf system-upgrade reboot` manually.", 1)
		}
	} else {
		fmt.Println("\nUpgrade path staged successfully! To run the final deployment, execute: (expores in 12hrs or after reboot)")
		fmt.Println("sudo dnf system-upgrade reboot")
	}

	return nil
}

func upgradeSettingsCommand(c *cli.Context) error {
	changing := false
	for _, flag := range []string{"automatic", "offline", "inactive-minutes", "window"} {
		changing = changing || c.IsSet(flag)
	}
	if changing {
		// UM_UPGRADE_CONFIG points at a test/development file; escalating would
		// drop it (pkexec clears the environment) and change the real settings.
		if os.Getenv("UM_UPGRADE_CONFIG") == "" {
			util.SudoIfNeeded(UpgradeEnvars)
		}
		settings, err := upgrade.LoadSettings()
		if err != nil {
			// Replace broken settings instead of getting stuck on them.
			settings = upgrade.DefaultSettings
		}
		if c.IsSet("automatic") {
			settings.Automatic = c.Bool("automatic")
		}
		if c.IsSet("offline") {
			settings.Offline = c.Bool("offline")
		}
		if c.IsSet("inactive-minutes") {
			settings.InactiveMinutes = c.Int("inactive-minutes")
		}
		if c.IsSet("window") {
			start, end, err := upgrade.ParseWindow(c.String("window"))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			settings.WindowStart, settings.WindowEnd = start, end
		}
		if err := settings.Validate(); err != nil {
			return cli.Exit(err.Error(), 1)
		}
		if err := upgrade.SaveSettings(settings); err != nil {
			return cli.Exit(fmt.Sprintf("failed to save settings: %v", err), 1)
		}
	}

	settings, err := upgrade.LoadSettings()
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}
	if c.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(settings)
	}
	window := "any time"
	if settings.WindowStart != "" {
		window = settings.WindowStart + "-" + settings.WindowEnd
	}
	fmt.Printf("Automatic updates: %t\nInstall when the device hasn't been used for: %d minutes\nOnly install during: %s\nInstall system updates while restarting: %t\n",
		settings.Automatic, settings.InactiveMinutes, window, settings.Offline)
	return nil
}

func upgradeScheduleCommand(c *cli.Context) error {
	if c.Bool("status") {
		status := upgrade.TonightStatus()
		if c.Bool("json") {
			return json.NewEncoder(os.Stdout).Encode(status)
		}
		if status.Scheduled {
			fmt.Println("Updates are scheduled to install tonight.")
		} else {
			fmt.Println("No updates are scheduled.")
		}
		return nil
	}

	cancel := c.Bool("cancel")
	// The user's own updates are scheduled in their user manager first;
	// SudoIfNeeded then re-runs this command as root for the system part.
	if os.Getuid() != 0 {
		if cancel {
			upgrade.CancelTonight(true)
		} else if err := upgrade.ScheduleTonight(true); err != nil {
			return cli.Exit(fmt.Sprintf("failed to schedule your updates: %v", err), 1)
		}
	}

	util.SudoIfNeeded(UpgradeEnvars)
	if cancel {
		upgrade.CancelTonight(false)
		fmt.Println("Tonight's updates were cancelled.")
		return nil
	}
	if err := upgrade.ScheduleTonight(false); err != nil {
		return cli.Exit(fmt.Sprintf("failed to schedule updates: %v", err), 1)
	}
	fmt.Println("Updates will be installed tonight, when the device isn't in use.")
	return nil
}

// upgradeRunScheduled is run by um's system timers (see upgrade.RunScheduled).
func upgradeRunScheduled(c *cli.Context) error {
	if err := upgrade.RunScheduled(c.Bool("tonight")); err != nil {
		return cli.Exit(err.Error(), 1)
	}
	return nil
}

// upgradeRunUser is run by um's user timers (see upgrade.RunUser).
func upgradeRunUser(c *cli.Context) error {
	if err := upgrade.RunUser(c.Bool("tonight")); err != nil {
		return cli.Exit(err.Error(), 1)
	}
	return nil
}
