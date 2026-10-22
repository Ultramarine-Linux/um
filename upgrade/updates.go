package upgrade

import (
	"fmt"
	"os/exec"
)

type PackageUpdate struct {
	// ID uniquely identifies the update: "name.arch" for RPMs, the
	// application ID for Flatpaks.
	ID string `json:"id"`
	// Name is the display name: the app name for desktop apps, otherwise the
	// package name or Flatpak name.
	Name   string `json:"name"`
	Source string `json:"source"`
	// Backend is the package manager handling the update: "rpm" or "flatpak".
	Backend string `json:"backend"`
	// Kind is KindApp, KindRuntime or KindPackage. Frontends can use it to
	// list desktop apps first.
	Kind string `json:"kind"`
	// AppID is the AppStream component ID for apps, when known: the Flatpak
	// application ID, or the ID from the package's metainfo.
	AppID string `json:"app_id,omitempty"`
	// DownloadSize is the download size in bytes, 0 if unknown. For Flatpaks
	// it's the full size; static deltas can make the actual download smaller.
	DownloadSize int64 `json:"download_size"`
	// InstalledSizeChange is how much more (or, if negative, less) disk space
	// the update uses once installed, in bytes. 0 if unknown.
	InstalledSizeChange int64  `json:"installed_size_change"`
	OldVersion          string `json:"old_version"`
	NewVersion          string `json:"new_version"`
	Icon                string `json:"icon"`
}

type ListResponse struct {
	Updates []PackageUpdate `json:"updates"`
	// Errors lists sources that could not be queried. Updates from the
	// remaining sources are still reported.
	Errors []string `json:"errors"`
}

// Update kinds reported in `um upgrade list --json`, so frontends can show
// desktop apps before libraries and runtimes.
const (
	// KindApp is a desktop application.
	KindApp = "app"
	// KindRuntime is a Flatpak runtime or extension.
	KindRuntime = "runtime"
	// KindPackage is any other package (libraries, CLI tools, addons, ...).
	KindPackage = "package"
)

// List collects updates from every available source. A failing
// source is recorded in Errors instead of hiding the other sources' updates.
func List() ListResponse {
	response := ListResponse{Updates: []PackageUpdate{}, Errors: []string{}}
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
