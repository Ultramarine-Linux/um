package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jaypipes/ghw"
	"github.com/mackerelio/go-osstat/uptime"
	"github.com/urfave/cli/v2"
)

func statusInfo() ([][]string, error) {
	dur, err := uptime.Get()
	if err != nil {
		return nil, err

	}
	//u := syscall.Utsname{}
	//err = syscall.Uname(&u)
	//if err != nil {
	//	return nil, err
	//}

	return [][]string{
		{"Uptime", dur.String()},
		//{"Kernel", string(u.Sysname)},
		{"Packages", "1000 (dnf)"},
	}, nil
}

func get_from_os_release(content string, id string) string {
	// I have no idea how to parse this properly
	var regex = regexp.MustCompile(id + "=" + `[^\n]+`)
	// apparently regexp does not support `^` to check if it starts on a new line using mode /m (multiline)
	// so now we need to painfully loop through each line
	var line string
	for _, l := range strings.Split(content, "\n") {
		line = regex.FindString(l)
		// make sure the line starts with what we've found
		if line != "" && strings.HasPrefix(l, line) {
			break
		}
	}
	// get rid of `KEY=` and quotes
	out, _ := strings.CutPrefix(line, id+"=")
	if strings.HasPrefix(out, `"`) && strings.HasSuffix(out, `"`) {
		return out[1 : len(out)-1]
	}
	return out
}

func gather_os_info(listHeader, listItem func(strs ...string) string) (result []string, err error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return
	}
	cont := string(content)
	return []string{
		listHeader("System"),
		listItem("Name: " + get_from_os_release(cont, "NAME")),
		listItem("Version: " + get_from_os_release(cont, "VERSION")),
		listItem("Variant: " + get_from_os_release(cont, "VARIANT")),
		listItem("Atomic: " + is_atomic()),
	}, nil
}

func is_atomic() string {
	_, err := os.Stat("/usr/bin/rpm-ostree")
	if os.IsNotExist(err) {
		return "False"
	}
	return "True"
}

func gather_hw_info(listHeader, listItem func(strs ...string) string) (result []string, err error) {
	cpu, err := ghw.CPU()
	if err != nil {
		return
	}
	gpu, err := ghw.GPU()
	if err != nil {
		return
	}

	result = []string{
		listHeader("Hardware"),
		listItem("CPU: " + cpu.Processors[0].Model),
	}
	for i, card := range gpu.GraphicsCards {
		result = append(result, listItem(fmt.Sprintf("GPU%d: %s", i, card.DeviceInfo.Product.Name)))
		result = append(result, listItem(fmt.Sprintf("GPU%d Driver: %s", i, card.DeviceInfo.Driver))) //?
	}
	return
}

func gather_desktop(listHeader, listItem func(strs ...string) string) (result []string, err error) {
	result = []string{
		listHeader("Desktop"),
		listItem("XDG_CURRENT_DESKTOP: " + os.Getenv("XDG_CURRENT_DESKTOP")),
	}
	return
}

func status(c *cli.Context) error {
	//release, err := osrelease.Read()
	//if err != nil {
	//	return err
	//}

	//fmt.Println(release["PRETTY_NAME"])

	subtle := lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}

	listHeader := lipgloss.NewStyle().
		Foreground(purple).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(subtle).
		MarginRight(2).
		MarginTop(1).
		Bold(true).
		Render

	listItem := lipgloss.NewStyle().PaddingLeft(2).Render

	osinfo, err := gather_os_info(listHeader, listItem)
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, osinfo...))

	hwinfo, err := gather_hw_info(listHeader, listItem)
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, hwinfo...))

	desktopinfo, err := gather_desktop(listHeader, listItem)
	if err != nil {
		return err
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, desktopinfo...))
	return nil
}
