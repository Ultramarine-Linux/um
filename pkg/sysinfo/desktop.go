package sysinfo

import "os"

type DisplayProtocol int

const (
	Unknown DisplayProtocol = iota
	Wayland
	X11
)

func (dp DisplayProtocol) String() string {
	switch dp {
	case Wayland:
		return "Wayland"
	case X11:
		return "X11"
	default:
		return "Unknown"
	}
}

type Desktop struct {
	Name     string          `json:"name"`
	Protocol DisplayProtocol `json:"protocol"`
}

func GatherDesktop() (*Desktop, error) {
	var protocol DisplayProtocol

	if s := os.Getenv("WAYLAND_DISPLAY"); s != "" {
		protocol = Wayland
	} else if s := os.Getenv("DISPLAY"); s != "" {
		protocol = X11
	} else {
		protocol = Unknown
	}

	return &Desktop{
		Name:     os.Getenv("XDG_CURRENT_DESKTOP"),
		Protocol: protocol,
	}, nil
}
