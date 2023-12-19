package main

import (
	"fmt"

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

func status(c *cli.Context) error {
	//release, err := osrelease.Read()
	//if err != nil {
	//	return err
	//}

	cpu, err := ghw.CPU()
	if err != nil {
		return err
	}

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

	fmt.Println(lipgloss.JoinVertical(lipgloss.Left,
		listHeader("Ultramarine"), listItem("Variant: Flagship MEOWY"), listItem("Atomic: False MEOWY")))

	fmt.Println(lipgloss.JoinVertical(lipgloss.Left,
		listHeader("Hardware"), listItem(fmt.Sprintf("CPU: %s", cpu.Processors[0].Model)), listItem("GPU: NVIDIA MEOWY")))

	fmt.Println(lipgloss.JoinVertical(lipgloss.Left,
		listHeader("Desktop"), listItem("Compositor: GNOME MEOWY")))

	return nil
}
