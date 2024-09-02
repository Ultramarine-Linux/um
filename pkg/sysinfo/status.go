package sysinfo

import (
	"os"
	"strings"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/mackerelio/go-osstat/uptime"
	"golang.org/x/sys/unix"
)

type Status struct {
	Uptime         time.Duration `json:"uptime"`
	Kernel         string        `json:"kernel"`
	RootDiskFree   uint64        `json:"root_disk_free"`
	RootFilesystem string        `json:"root_filesystem"`
}

func GatherStatus() (*Status, error) {
	dur, err := uptime.Get()
	if err != nil {
		return nil, err
	}

	u := unix.Utsname{}
	err = unix.Uname(&u)
	if err != nil {
		return nil, err
	}

	var stat unix.Statfs_t
	wd, err := os.Getwd()
	unix.Statfs(wd, &stat)
	diskFree := stat.Bavail * uint64(stat.Bsize)

	block, err := ghw.Block()
	if err != nil {
		return nil, err
	}

	rootFilesystem := "Unknown"

	for _, disk := range block.Disks {
		for _, part := range disk.Partitions {
			if part.MountPoint == "/" {
				rootFilesystem = part.Type
			}
		}
	}

	return &Status{
		Uptime:         dur,
		Kernel:         strings.Trim(string(u.Release[:]), "\u0000"),
		RootDiskFree:   diskFree,
		RootFilesystem: rootFilesystem,
	}, err
}
