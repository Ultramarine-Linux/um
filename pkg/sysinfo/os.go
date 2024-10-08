package sysinfo

import (
	"strings"

	"github.com/acobaugh/osrelease"
)

type OS struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Variant string `json:"variant"`
	Atomic  bool   `json:"atomic"`
}

func GatherOS() (*OS, error) {
	release, err := osrelease.Read()
	if err != nil {
		return nil, err
	}

	return &OS{
		Name:    release["NAME"],
		Version: release["VERSION"],
		Variant: release["VARIANT"],
		Atomic:  strings.HasPrefix(release["VARIANT"], "Atomic"),
	}, nil
}
