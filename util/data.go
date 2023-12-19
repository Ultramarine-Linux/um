package util

import "os"

func GetDataDir() string {
	v, found := os.LookupEnv("UM_DATA")
	if !found {
		return "/usr/share/um"
	}

	return v
}

func GetStateDir() string {
	v, found := os.LookupEnv("UM_STATE")
	if !found {
		return "/var/lib/um"
	}

	return v
}
