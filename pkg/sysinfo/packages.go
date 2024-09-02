package sysinfo

// #cgo pkg-config: flatpak rpm
// #include "packages.h"
import "C"

func getInstalledSystemFlatpakCount() int {
	return int(C.get_installed_system_flatpak_count())
}

func getInstalledUserFlatpakCount() int {
	return int(C.get_installed_user_flatpak_count())
}

func getInstalledRpmCount() int {
	return int(C.get_installed_rpm_count())
}

type Packages struct {
	RPMCount           int `json:"rpm_count"`
	SystemFlatpakCount int `json:"system_flatpak_count"`
	UserFlatpakCount   int `json:"user_flatpak_count"`
}

func GatherPackages() (*Packages, error) {
	return &Packages{
		RPMCount:           getInstalledRpmCount(),
		SystemFlatpakCount: getInstalledSystemFlatpakCount(),
		UserFlatpakCount:   getInstalledUserFlatpakCount(),
	}, nil
}
