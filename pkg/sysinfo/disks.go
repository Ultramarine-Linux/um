package sysinfo

import (
	"github.com/jaypipes/ghw"
)

type Disk struct {
	Model      string
	Name       string
	Type       string
	Controller string
}

func GatherDisks() ([]Disk, error) {
	var disks []Disk

	block, err := ghw.Block()
	if err != nil {
		return nil, err
	}

	for _, disk := range block.Disks {
		if disk.BusPath == "unknown" {
			continue
		}

		disks = append(disks, Disk{
			Model:      disk.Model,
			Name:       disk.Name,
			Type:       disk.DriveType.String(),
			Controller: disk.StorageController.String(),
		})
	}

	return disks, nil
}

type RootDisk struct {
}

func GatherRootDiskInfo() {}
