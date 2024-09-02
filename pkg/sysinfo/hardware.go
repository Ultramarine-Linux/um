package sysinfo

import (
	"runtime"

	"github.com/jaypipes/ghw"
	mem "github.com/mackerelio/go-osstat/memory"
)

type GPU struct {
	Name   string
	Driver string
}

func GatherGPUs() ([]GPU, error) {
	gpu, err := ghw.GPU()
	if err != nil {
		return nil, err
	}

	var gpus []GPU

	for _, card := range gpu.GraphicsCards {
		gpus = append(gpus, GPU{
			Name:   card.DeviceInfo.Vendor.Name,
			Driver: card.DeviceInfo.Driver,
		})
	}

	return gpus, nil
}

type CPU struct {
	Model string
	Arch  string
}

func GatherCPUs() ([]CPU, error) {
	cpu, err := ghw.CPU()
	if err != nil {
		return nil, err
	}

	var cpus []CPU

	for _, processor := range cpu.Processors {
		cpus = append(cpus, CPU{
			Model: processor.Model,
			Arch:  runtime.GOARCH,
		})
	}

	return cpus, nil
}

type Hardware struct {
	Vendor         string
	Product        string
	CPUs           []CPU
	GPUs           []GPU
	PhysicalMemory uint64
	UsableMemory   uint64
	Swap           uint64
	// RootFree       diskFree
	// RootFS
}

func GatherHardware() (*Hardware, error) {
	baseboard, err := ghw.Baseboard(ghw.WithDisableWarnings())
	if err != nil {
		return nil, err
	}

	gpus, err := GatherGPUs()
	if err != nil {
		return nil, err
	}

	cpus, err := GatherCPUs()
	if err != nil {
		return nil, err
	}

	memory, err := ghw.Memory()
	if err != nil {
		return nil, err
	}

	memoryStats, err := mem.Get()
	if err != nil {
		return nil, err
	}

	return &Hardware{
		GPUs:           gpus,
		CPUs:           cpus,
		Vendor:         baseboard.Vendor,
		Product:        baseboard.Product,
		PhysicalMemory: uint64(memory.TotalPhysicalBytes),
		UsableMemory:   uint64(memory.TotalPhysicalBytes),
		Swap:           memoryStats.SwapTotal,
	}, nil
}
