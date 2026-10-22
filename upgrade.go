package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Ultramarine-Linux/um/pkg/util"
	"github.com/Ultramarine-Linux/um/upgrade"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

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

func systemVersionUpgrade(c *cli.Context) error {
	// Elevate to root execution via your utility helper block early on
	util.SudoIfNeeded(UpgradeEnvars)

	currentVer, err := upgrade.CurrentReleaseVersion()
	if err != nil {
		return cli.Exit(fmt.Sprintf("Failed evaluating current local OS variant version: %v", err), 1)
	}

	targetVer, err := upgrade.NextReleaseVersion(currentVer)
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
