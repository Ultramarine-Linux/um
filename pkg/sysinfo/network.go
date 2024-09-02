package sysinfo

import (
	"github.com/Wifx/gonetworkmanager"
)

type NetworkDevice struct {
	Interface string `json:"interface"`
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
}

func GatherNetworkDevices() ([]NetworkDevice, error) {
	var networkDevices []NetworkDevice

	nm, err := gonetworkmanager.NewNetworkManager()
	if err != nil {
		return nil, err
	}

	devices, err := nm.GetPropertyAllDevices()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		deviceInterface, err := device.GetPropertyInterface()
		if err != nil {
			return nil, err
		}

		connection, err := device.GetPropertyActiveConnection()
		if err != nil {
			return nil, err
		}
		if connection == nil {
			continue
		}

		status, err := connection.GetPropertyState()
		if err != nil {
			return nil, err
		}

		proptype, err := connection.GetPropertyType()
		if err != nil {
			return nil, err
		}

		networkDevices = append(networkDevices, NetworkDevice{
			Interface: deviceInterface,
			Type:      proptype,
			Connected: status == gonetworkmanager.NmActiveConnectionStateActivated,
		})
	}

	return networkDevices, nil
}
