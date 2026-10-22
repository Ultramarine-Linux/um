package upgrade

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	tonightTimer     = "um-upgrade-tonight"
	tonightUserTimer = "um-upgrade-tonight-user"
	// Without an update window, "tonight" means 22:00-06:00.
	tonightDefaultStart = "22:00"
	tonightDefaultEnd   = "06:00"
	// Tonight's timers check every 5 minutes, like the automatic ones, and stop
	// once the updates are installed. If the device is in use all night, they
	// try again the next night.
	pollCalendar = "*:0/5"
)

// ScheduleStatus is the state of Apply Tonight.
type ScheduleStatus struct {
	Scheduled bool `json:"scheduled"`
	// Next is when tonight's run is next attempted, as a Unix timestamp.
	Next int64 `json:"next,omitempty"`
}

// parseTimerShow parses `systemctl show --timestamp=unix -p ActiveState -p
// NextElapseUSecRealtime` output.
func parseTimerShow(output string) ScheduleStatus {
	var status ScheduleStatus
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "ActiveState":
			status.Scheduled = value == "active"
		case "NextElapseUSecRealtime":
			if n, err := strconv.ParseInt(strings.TrimPrefix(value, "@"), 10, 64); err == nil {
				status.Next = n
			}
		}
	}
	if !status.Scheduled {
		status.Next = 0
	}
	return status
}

func TonightStatus() ScheduleStatus {
	output, _ := exec.Command("systemctl", "show", "--timestamp=unix", "-p", "ActiveState",
		"-p", "NextElapseUSecRealtime", tonightTimer+".timer").Output()
	return parseTimerShow(string(output))
}

func systemctl(userManager bool, args ...string) error {
	if userManager {
		args = append([]string{"--user"}, args...)
	}
	return exec.Command("systemctl", args...).Run()
}

// scheduleTransientTimer (re)creates a transient timer running um with args.
func scheduleTransientTimer(userManager bool, unit string, args ...string) error {
	// Replace an existing schedule; it's fine if there is none.
	_ = systemctl(userManager, "stop", unit+".timer")
	_ = systemctl(userManager, "reset-failed", unit+".service")

	um, err := os.Executable()
	if err != nil {
		return err
	}
	runArgs := []string{}
	if userManager {
		runArgs = append(runArgs, "--user")
	}
	runArgs = append(runArgs, "--unit="+unit, "--on-calendar="+pollCalendar,
		"--timer-property=AccuracySec=1min", "--description=Install updates tonight",
		"--property=Nice=19", "--property=IOSchedulingClass=idle", "--", um)
	runArgs = append(runArgs, args...)
	if output, err := exec.Command("systemd-run", runArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("systemd-run failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// readyToInstall decides whether a timer run should install updates now.
// It's separate from the side effects so it can be tested.
func readyToInstall(settings Settings, tonight, inUse bool, inactive time.Duration, installedRecently bool, now time.Time) (bool, string) {
	start, end := settings.WindowStart, settings.WindowEnd
	if tonight {
		if start == "" {
			start, end = tonightDefaultStart, tonightDefaultEnd
		}
	} else {
		if !settings.Automatic {
			return false, "automatic updates are off"
		}
		if installedRecently {
			return false, "updates were installed recently"
		}
	}
	if !inWindow(now, start, end) {
		return false, "outside the update window"
	}
	if inUse {
		return false, "the device is in use"
	}
	if inactive < time.Duration(settings.InactiveMinutes)*time.Minute {
		return false, "the device was used recently"
	}
	return true, ""
}

// RunScheduled installs system updates from the automatic or tonight
// timer. Runs as root every few minutes, so skipping is silent.
func RunScheduled(tonight bool) error {
	if os.Getuid() != 0 {
		return errors.New("run-scheduled is run by um's system timers as root")
	}
	settings, err := LoadSettings()
	if err != nil {
		return err
	}
	if !tonight && !settings.Automatic {
		return nil
	}
	sessions, err := loginSessions()
	if err != nil {
		return err
	}
	_, inUse := sessionInUse(sessions, "")
	now := time.Now()
	inactive := activityTracker{systemStateFile("inactive-since")}.inactiveFor(inUse, now)
	installed := systemStateFile("last-automatic-upgrade")
	if ok, _ := readyToInstall(settings, tonight, inUse, inactive, within(installed, installInterval, now), now); !ok {
		return nil
	}
	if !onACPower() {
		return nil
	}

	fmt.Println("No one is using the device; installing updates.")
	if _, err := ApplySystem(true, settings.Offline); err != nil {
		return err
	}
	if settings.Offline {
		fmt.Println("System updates will be installed during the next restart.")
	}
	if err := writeTimestamp(installed, now); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if tonight {
		_ = systemctl(false, "stop", tonightTimer+".timer")
	}
	return nil
}

// RunUser handles the user's part from the user timers: installing the
// user's own updates when automatic (or tonight's) updates apply, otherwise
// notifying about pending updates once a day. Runs as the user.
func RunUser(tonight bool) error {
	if os.Getuid() == 0 {
		return errors.New("run-user is run by um's user timers")
	}
	settings, err := LoadSettings()
	if err != nil {
		return err
	}
	now := time.Now()
	if !tonight && !settings.Automatic {
		return notifyPendingUpdates(now)
	}

	current, err := user.Current()
	if err != nil {
		return err
	}
	sessions, err := loginSessions()
	if err != nil {
		return err
	}
	_, inUse := sessionInUse(sessions, current.Username)
	inactive := activityTracker{filepath.Join(userStateDir(), "inactive-since")}.inactiveFor(inUse, now)
	installed := filepath.Join(userStateDir(), "last-automatic-upgrade")
	if ok, _ := readyToInstall(settings, tonight, inUse, inactive, within(installed, installInterval, now), now); !ok {
		return nil
	}

	fmt.Println("You're not using the device; installing your updates.")
	if err := ApplyUser(true); err != nil {
		return err
	}
	if err := writeTimestamp(installed, now); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if tonight {
		_ = systemctl(true, "stop", tonightUserTimer+".timer")
	}
	return nil
}

// ScheduleTonight sets up Apply Tonight in the user's manager (userManager)
// or the system manager (as root). Scheduling again replaces the old timer.
func ScheduleTonight(userManager bool) error {
	if userManager {
		return scheduleTransientTimer(true, tonightUserTimer, "upgrade", "run-user", "--tonight")
	}
	return scheduleTransientTimer(false, tonightTimer, "upgrade", "run-scheduled", "--tonight")
}

// CancelTonight removes Apply Tonight from the user's or the system manager.
// It's not an error if nothing was scheduled.
func CancelTonight(userManager bool) {
	unit := tonightTimer
	if userManager {
		unit = tonightUserTimer
	}
	_ = systemctl(userManager, "stop", unit+".timer")
}
