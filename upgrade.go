package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Ultramarine-Linux/um/pkg/util"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

var UpgradeEnvars = []string{
	"UM_DATA",
}

// getCurrentReleaseVersion reads /etc/os-release to safely discover what version the user is running
func getCurrentReleaseVersion() (string, error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VERSION_ID=") {
			// Strips VERSION_ID="43" down to just 43
			version := strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			return version, nil
		}
	}
	return "", fmt.Errorf("failed to locate VERSION_ID inside /etc/os-release")
}

// getNextReleaseVersion calculates the next upgrade jump.
// (Owen mentioned moving from 43 to 44 is actively being prepared right now)
func getNextReleaseVersion(currentVersion string) (string, error) {
	// TODO: When Fyra infrastructure APIs are ready, substitute this logic with an active infrastructure lookup.
	if currentVersion == "43" {
		return "44", nil
	}

	// Dynamic fallback: attempt to increment numerically if not explicit string matches
	var currentNum int
	_, err := fmt.Sscanf(currentVersion, "%d", &currentNum)
	if err != nil {
		return "", fmt.Errorf("invalid current version structure format: %v", err)
	}

	return fmt.Sprintf("%d", currentNum+1), nil
}

// runCommand routes standard I/O so DNF download progress animations are visible to the user
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
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
			fmt.Printf("Your system is completely up-to-date on Ultramarine %s.\n", currentVer)
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

	// 1. Fully sync and prepare current local system tracking branch packages
	fmt.Println("\n[*] Syncing and fully updating packages on your current release version...")
	if err := runCommand("dnf", "upgrade", "--refresh", "-y"); err != nil {
		return cli.Exit(fmt.Sprintf("DNF failed during pre-upgrade packages synchronization: %v", err), 1)
	}

	// 2. Double check dnf-plugin-system-upgrade is local
	fmt.Println("\n[*] Assuring system upgrade plugins are active...")
	if err := runCommand("dnf", "install", "dnf-plugin-system-upgrade", "-y"); err != nil {
		return cli.Exit(fmt.Sprintf("Failed asserting infrastructure tracking dependencies: %v", err), 1)
	}

	// 3. Initiate major repository release downloads
	fmt.Printf("\n[*] Downloading upgrade tree metadata targets for Release %s...\n", targetVer)
	downloadArgs := []string{"system-upgrade", "download", fmt.Sprintf("--releasever=%s", targetVer), "--allowerasing", "-y"}
	if err := runCommand("dnf", downloadArgs...); err != nil {
		return cli.Exit(fmt.Sprintf("DNF execution pipeline failed to pull systemic upgrade branches: %v", err), 1)
	}

	// 4. Verification and target reboot sequence configuration
	var rebootNow bool
	if yesFlag {
		rebootNow = true
	} else {
		fmt.Println()
		err := huh.NewConfirm().
			Title("Upgrade ecosystem prepared successfully! Reboot and apply upgrades now?").
			Affirmative("Reboot and Upgrade").
			Negative("Reboot Later").
			Value(&rebootNow).
			Run()
		if err != nil {
			return err
		}
	}

	if rebootNow {
        fmt.Println("\n[*] Activating system-upgrade trigger and rebooting system...")
        
        // 1. Try the polite DNF system-upgrade reboot sequence first
        if err := runCommand("dnf", "system-upgrade", "reboot"); err != nil {
            fmt.Printf("\n[!] Polite reboot blocked by inhibitor locks (%v). Forcing bypass...\n", err)
            
            // 2. Fallback Override: Tell systemctl to force the reboot past the locks
            forceReboot := exec.Command("systemctl", "reboot", "--force")
            forceReboot.Stdout = os.Stdout
            forceReboot.Stderr = os.Stderr
            
            if err := forceReboot.Run(); err != nil {
                // 3. Ultimate Fallback: Direct hardware reset via standard reboot flags
                fmt.Println("[!] Forceful systemctl failed. Triggering hardware reset...")
                hardReset := exec.Command("reboot", "-f")
                _ = hardReset.Run()
                
                return cli.Exit("Failed to automatically cycle the machine power.", 1)
            }
        }
	} else {
		fmt.Println("\nUpgrade path staged successfully! To run the final deployment cycle later at any time, execute:")
		fmt.Println("sudo dnf system-upgrade reboot")
	}

	return nil
}
