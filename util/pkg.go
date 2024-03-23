package util

// #cgo pkg-config: flatpak rpm
// #include "pkg.h"
import "C"

func GetInstalledSystemFlatpakCount() int {
	return int(C.get_installed_system_flatpak_count())
}

func GetInstalledUserFlatpakCount() int {
	return int(C.get_installed_user_flatpak_count())
}

func GetInstalledRpmCount() int {
	return int(C.get_installed_rpm_count())
}
