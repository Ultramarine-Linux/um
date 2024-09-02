package main

import (
	"encoding/json"
	"fmt"

	"github.com/Ultramarine-Linux/um/pkg/sysinfo"
	"github.com/Ultramarine-Linux/um/util"
	"github.com/charmbracelet/lipgloss"

	"github.com/urfave/cli/v2"
)

var listHeader = lipgloss.NewStyle().
	Foreground(purple).
	MarginRight(2).
	MarginTop(1).
	Bold(true).
	Render

var listItem = lipgloss.NewStyle().PaddingLeft(2).Render

func networkSection() ([]string, error) {
	devices, err := sysinfo.GatherNetworkDevices()
	if err != nil {
		return nil, err
	}

	devicesInfo := []string{
		listHeader("Network"),
	}

	for _, device := range devices {
		statusString := "Unknown"

		if device.Connected {
			statusString = "Connected"
		}

		devicesInfo = append(devicesInfo, listItem(fmt.Sprintf("%s (%s): %s", device.Interface, device.Type, statusString)))
	}

	return devicesInfo, nil
}

func statusSection() ([]string, error) {
	status, err := sysinfo.GatherStatus()
	if err != nil {
		return nil, err
	}

	packages, err := sysinfo.GatherPackages()
	if err != nil {
		return nil, err
	}

	return []string{
		listHeader("Status"),
		listItem("Uptime: " + status.Uptime.String()),
		listItem("Kernel: " + status.Kernel),
		listItem("Disk Free: " + util.FormatBytes(int64(status.RootDiskFree))),
		listItem("Filesystem: " + status.RootFilesystem),
		listItem(fmt.Sprintf("Packages: %d rpms, %d system flatpaks, %d user flatpaks", packages.RPMCount, packages.SystemFlatpakCount, packages.UserFlatpakCount)),
	}, nil
}

func osSection() (result []string, err error) {
	os, err := sysinfo.GatherOS()
	if err != nil {
		return nil, err
	}

	var atomicValue string

	if os.Atomic {
		atomicValue = "True"
	} else {
		atomicValue = "False"
	}

	return []string{
		listHeader("System"),
		listItem("Name: " + os.Name),
		listItem("Version: " + os.Version),
		listItem("Variant: " + os.Variant),
		listItem("Atomic: " + atomicValue),
	}, nil
}

func hwSection() (result []string, err error) {
	hardware, err := sysinfo.GatherHardware()
	if err != nil {
		return nil, err
	}

	result = []string{
		listHeader("Hardware"),
	}

	result = append(result, listItem(fmt.Sprintf("Vendor: %s", hardware.Vendor)))
	result = append(result, listItem(fmt.Sprintf("Product: %s", hardware.Product)))

	result = append(result, listItem(fmt.Sprintf("Memory: %s (physical), %s (usuable)",
		util.FormatBytes(int64(hardware.PhysicalMemory)),
		util.FormatBytes(int64(hardware.UsableMemory)))))

	result = append(result, listItem(fmt.Sprintf("Swap: %s", util.FormatBytes(int64(hardware.Swap)))))

	for i, cpu := range hardware.CPUs {
		title := "CPU"
		if len(hardware.CPUs) > 1 {
			title = title + string(i)
		}

		result = append(result, listItem(fmt.Sprintf("%s: %s (%s)", title, cpu.Model, cpu.Arch)))
	}

	for i, gpu := range hardware.GPUs {
		title := "GPU"
		if len(hardware.GPUs) > 1 {
			title = title + string(i)
		}

		result = append(result, listItem(fmt.Sprintf("%s: %s", title, gpu.Name)))
		result = append(result, listItem(fmt.Sprintf("%s Driver: %s", title, gpu.Driver))) //?
	}

	return
}

func disksSection() (result []string, err error) {
	disks, err := sysinfo.GatherDisks()
	if err != nil {
		return nil, err
	}

	result = []string{
		listHeader("Disk"),
	}

	for i, disk := range disks {
		title := "Disk"
		if len(disks) > 1 {
			title = title + string(i)
		}

		result = append(result, listItem(fmt.Sprintf("%s: %s (%s)", title, disk.Model, disk.Name)))
		result = append(result, listItem(fmt.Sprintf("%s Type: %s", title, disk.Type)))
		result = append(result, listItem(fmt.Sprintf("%s Controler: %s", title, disk.Controller)))
	}

	return
}

func desktopSection() (result []string, err error) {
	desktop, err := sysinfo.GatherDesktop()
	if err != nil {
		return nil, err
	}

	result = []string{
		listHeader("Desktop"),
		listItem("Name: " + desktop.Name),
		listItem("Protocol: " + desktop.Protocol.String()),
	}
	return
}

func status(c *cli.Context) error {
	if c.Bool("json") {
		all, err := sysinfo.GatherAll()
		if err != nil {
			return err
		}

		bytes, err := json.Marshal(all)
		if err != nil {
			return err
		}

		println(string(bytes))

		return nil
	}

	os, err := osSection()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, os...))

	hw, err := hwSection()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, hw...))

	disk, err := disksSection()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, disk...))

	desktop, err := desktopSection()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, desktop...))

	status, err := statusSection()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, status...))

	network, err := networkSection()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, network...))

	return nil
}
