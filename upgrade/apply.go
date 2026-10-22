package upgrade

import (
	"fmt"
	"os/exec"

	"github.com/Ultramarine-Linux/um/pkg/util"
)

// ApplyUser updates what belongs to the invoking user: user-installation
// Flatpaks and the Nix profile. After escalating, flatpak and nix would only see
// root's (empty) installation and profile.
func ApplyUser(assumeYes bool) error {
	if _, err := exec.LookPath("flatpak"); err == nil {
		args := []string{"update", "--user"}
		if assumeYes {
			args = append(args, "--noninteractive")
		}
		if err := util.RunCommand("flatpak", args...); err != nil {
			return fmt.Errorf("Flatpak user update failed: %w", err)
		}
	}
	// `nix profile upgrade` doesn't prompt.
	if canManageNixProfile() {
		if err := upgradeNixProfile(); err != nil {
			return fmt.Errorf("Nix profile upgrade failed: %w", err)
		}
	}
	return nil
}

// ApplySystem updates system packages, system Flatpaks and Nix itself.
// Needs root. Reports whether any package manager was found.
func ApplySystem(assumeYes bool) (bool, error) {
	updated := false
	if _, err := exec.LookPath("dnf"); err == nil {
		args := []string{"upgrade", "--refresh"}
		if assumeYes {
			args = append(args, "-y")
		}
		if err := util.RunCommand("dnf", args...); err != nil {
			return updated, fmt.Errorf("RPM update failed: %w", err)
		}
		updated = true
	}
	if determinateNixdBinary() != "" {
		// Nix itself, installed system-wide by the nix tweak. Only upgrades
		// when Determinate Systems advises a newer version.
		if err := upgradeDeterminateNix(); err != nil {
			return updated, fmt.Errorf("Determinate Nix upgrade failed: %w", err)
		}
		updated = true
	}
	if _, err := exec.LookPath("flatpak"); err == nil {
		args := []string{"update", "--system"}
		if assumeYes {
			args = append(args, "--noninteractive")
		}
		if err := util.RunCommand("flatpak", args...); err != nil {
			return updated, fmt.Errorf("Flatpak update failed: %w", err)
		}
		updated = true
	}
	return updated, nil
}
