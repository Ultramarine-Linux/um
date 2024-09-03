package sysinfo

import (
	"runtime"

	"github.com/jaypipes/ghw"
	mem "github.com/mackerelio/go-osstat/memory"
)

type GPU struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

func GatherGPUs() ([]GPU, error) {
	gpu, err := ghw.GPU()
	if err != nil {
		return nil, err
	}

	var gpus []GPU

	for _, card := range gpu.GraphicsCards {
		gpus = append(gpus, GPU{
			Name:   card.DeviceInfo.Product.Name,
			Driver: card.DeviceInfo.Driver,
		})
	}

	return gpus, nil
}

type CPU struct {
	Model string `json:"model"`
	Arch  string `json:"arch"`
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
	Vendor         string `json:"vendor"`
	Product        string `json:"product"`
	CPUs           []CPU  `json:"cpus"`
	GPUs           []GPU  `json:"gpus"`
	PhysicalMemory uint64 `json:"physical_memory"`
	UsableMemory   uint64 `json:"usable_memory"`
	Swap           uint64 `json:"swap"`
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
