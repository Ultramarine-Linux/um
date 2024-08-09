package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/Ultramarine-Linux/um/util"
	"github.com/acobaugh/osrelease"
	"github.com/charmbracelet/lipgloss"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/unitutil"

	"github.com/mackerelio/go-osstat/uptime"
	"github.com/urfave/cli/v2"

	"github.com/Wifx/gonetworkmanager/v2"
)

var listHeader = lipgloss.NewStyle().
	Foreground(purple).
	MarginRight(2).
	MarginTop(1).
	Bold(true).
	Render

var listItem = lipgloss.NewStyle().PaddingLeft(2).Render

func networkInfo() ([]string, error) {
	nm, err := gonetworkmanager.NewNetworkManager()
	if err != nil {
		return nil, err
	}

	devices, err := nm.GetPropertyAllDevices()
	if err != nil {
		return nil, err
	}

	devicesInfo := []string{
		listHeader("Network"),
	}

	for _, device := range devices {
		deviceInterface, err := device.GetPropertyInterface()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		connection, err := device.GetPropertyActiveConnection()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		if connection == nil {
			continue
		}

		status, err := connection.GetPropertyState()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		proptype, err := connection.GetPropertyType()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		statusString := ""
		switch status {
		case gonetworkmanager.NmActiveConnectionStateActivated:
			statusString = "Connected"
		default:
			statusString = "Unknown"
		}
		devicesInfo = append(devicesInfo, listItem(fmt.Sprintf("%s (%s): %s", deviceInterface, proptype, statusString)))
	}

	return devicesInfo, nil
}

func statusInfo() ([]string, error) {
	dur, err := uptime.Get()
	if err != nil {
		return nil, err

	}

	u := unix.Utsname{}
	err = unix.Uname(&u)
	if err != nil {
		return nil, err
	}

	rpmCount := util.GetInstalledRpmCount()
	systemFlatpakCount := util.GetInstalledSystemFlatpakCount()
	userFlatpakCount := util.GetInstalledUserFlatpakCount()

	return []string{
		listHeader("Status"),
		listItem("Uptime: " + dur.String()),
		listItem("Kernel: " + string(u.Release[:])),
		listItem(fmt.Sprintf("Packages: %d rpms, %d system flatpaks, %d user flatpaks", rpmCount, systemFlatpakCount, userFlatpakCount)),
	}, nil
}

func gatherOsInfo() (result []string, err error) {
	release, err := osrelease.Read()
	if err != nil {
		return nil, err
	}

	var atomicValue string

	if strings.HasPrefix(release["VARIANT"], "Atomic") {
		atomicValue = "True"
	} else {
		atomicValue = "False"
	}

	return []string{
		listHeader("System"),
		listItem("Name: " + release["NAME"]),
		listItem("Version: " + release["VERSION"]),
		listItem("Variant: " + release["VARIANT"]),
		listItem("Atomic: " + atomicValue),
	}, nil
}

func gatherHwInfo() (result []string, err error) {
	cpu, err := ghw.CPU()
	if err != nil {
		return nil, err
	}

	gpu, err := ghw.GPU()
	if err != nil {
		return nil, err
	}

	result = []string{
		listHeader("Hardware"),
	}

	baseboard, err := ghw.Baseboard(ghw.WithDisableWarnings())
	if err != nil {
		fmt.Printf("Error getting baseboard info: %v", err)
	}
	result = append(result, listItem(fmt.Sprintf("Vendor: %s", baseboard.Vendor)))
	result = append(result, listItem(fmt.Sprintf("Product: %s", baseboard.Product)))

	memory, err := ghw.Memory()
	if err != nil {
		fmt.Printf("Error getting baseboard info: %v", err)
	}

	// from https://github.com/jaypipes/ghw/blob/main/pkg/memory/memory.go#L47
	// there is probably a better way to do this
	tpb := memory.TotalPhysicalBytes
	unit, unitStr := unitutil.AmountString(tpb)
	tpb = int64(math.Ceil(float64(memory.TotalPhysicalBytes) / float64(unit)))
	tpbs := fmt.Sprintf("%d%s", tpb, unitStr)

	tub := memory.TotalUsableBytes
	unit, unitStr = unitutil.AmountString(tub)
	tub = int64(math.Ceil(float64(memory.TotalUsableBytes) / float64(unit)))
	tubs := fmt.Sprintf("%d%s", tub, unitStr)

	result = append(result, listItem(fmt.Sprintf("Memory: %s (physical), %s (usuable)", tpbs, tubs)))

	for i, processor := range cpu.Processors {
		result = append(result, listItem(fmt.Sprintf("CPU%d: %s (%s)", i, processor.Model, runtime.GOARCH)))
	}

	for i, card := range gpu.GraphicsCards {
		result = append(result, listItem(fmt.Sprintf("GPU%d: %s", i, card.DeviceInfo.Product.Name)))
		result = append(result, listItem(fmt.Sprintf("GPU%d Driver: %s", i, card.DeviceInfo.Driver))) //?
	}

	return
}

func gatherDesktop() (result []string, err error) {
	var protocol string

	if s := os.Getenv("WAYLAND_DISPLAY"); s != "" {
		protocol = "Wayland"
	} else if s := os.Getenv("DISPLAY"); s != "" {
		protocol = "X11"
	} else {
		protocol = "Unknown"
	}

	result = []string{
		listHeader("Desktop"),
		listItem("Name: " + os.Getenv("XDG_CURRENT_DESKTOP")),
		listItem("Protocol: " + protocol),
	}
	return
}

func status(c *cli.Context) error {
	osinfo, err := gatherOsInfo()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, osinfo...))

	hwinfo, err := gatherHwInfo()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, hwinfo...))

	desktopinfo, err := gatherDesktop()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, desktopinfo...))

	statusinfo, err := statusInfo()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, statusinfo...))

	networkinfo, err := networkInfo()
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, networkinfo...))

	return nil
}
