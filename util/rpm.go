package util

// #cgo LDFLAGS: -lrpm
// #include "rpm.h"
import "C"

func GetInstalledRpmCount() int {
	return int(C.get_installed_rpm_count())
}
