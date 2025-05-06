package collector

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

type Metrics struct {
	Timestamp time.Time `json:"timestamp"`
	CPUUsage  float64   `json:"cpu_usage"`
	RAMUsage  float64   `json:"ram_usage"`
	DiskUsage float64   `json:"disk_usage"`
}

type SystemCollector struct{}

func NewSystemCollector() *SystemCollector {
	return &SystemCollector{}
}

func (c *SystemCollector) Collect() (Metrics, error) {
	metrics := Metrics{
		Timestamp: time.Now(),
	}

	// Collect CPU usage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return metrics, err
	}
	if len(cpuPercent) > 0 {
		metrics.CPUUsage = cpuPercent[0]
	}

	// Collect memory usage
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return metrics, err
	}
	metrics.RAMUsage = memInfo.UsedPercent

	// Collect disk usage (root partition)
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return metrics, err
	}
	metrics.DiskUsage = diskInfo.UsedPercent

	return metrics, nil
}
