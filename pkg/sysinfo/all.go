package sysinfo

type All struct {
	Desktop        Desktop         `json:"desktop"`
	Disks          []Disk          `json:"disks"`
	Hardware       Hardware        `json:"hardware"`
	NetworkDevices []NetworkDevice `json:"network_devices"`
	OS             OS              `json:"os"`
	Packages       Packages        `json:"packages"`
	Status         Status          `json:"status"`
}

func GatherAll() (*All, error) {
	desktop, err := GatherDesktop()
	if err != nil {
		return nil, err
	}

	disks, err := GatherDisks()
	if err != nil {
		return nil, err
	}

	hardware, err := GatherHardware()
	if err != nil {
		return nil, err
	}

	networkDevices, err := GatherNetworkDevices()
	if err != nil {
		return nil, err
	}

	os, err := GatherOS()
	if err != nil {
		return nil, err
	}

	packages, err := GatherPackages()
	if err != nil {
		return nil, err
	}

	status, err := GatherStatus()
	if err != nil {
		return nil, err
	}

	return &All{
		Desktop:        *desktop,
		Disks:          disks,
		Hardware:       *hardware,
		NetworkDevices: networkDevices,
		OS:             *os,
		Packages:       *packages,
		Status:         *status,
	}, nil
}
