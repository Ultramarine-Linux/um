package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// notifyPendingUpdates shows a desktop notification if updates are available,
// checking at most once a day. "Open Updates" opens System Settings > Updates.
func notifyPendingUpdates(now time.Time) error {
	checked := filepath.Join(userStateDir(), "last-update-check")
	if within(checked, notifyInterval, now) {
		return nil
	}
	if err := writeTimestamp(checked, now); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	updates := List()
	count := len(updates.Updates)
	if count == 0 {
		return nil
	}
	if _, err := exec.LookPath("notify-send"); err != nil {
		fmt.Printf("%d updates are available.\n", count)
		return nil
	}
	body := fmt.Sprintf("%d updates are ready to install.", count)
	if count == 1 {
		body = "1 update is ready to install."
	}
	// notify-send waits for an action or for the notification to close.
	action, err := exec.Command("notify-send", "--app-name=Updates", "--icon=system-software-update",
		"--action=open=Open Updates", "Updates are ready", body).Output()
	if err != nil {
		return nil
	}
	if strings.TrimSpace(string(action)) == "open" {
		return exec.Command("systemsettings", "kcm_ultramarine_updates").Start()
	}
	return nil
}
