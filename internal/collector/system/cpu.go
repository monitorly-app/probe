package system

import (
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/shirou/gopsutil/v4/cpu"
)

// CPUCollector implements the collector.Collector interface for CPU metrics
type CPUCollector struct{}

// NewCPUCollector creates a new instance of CPUCollector
func NewCPUCollector() collector.Collector {
	return &CPUCollector{}
}

// Collect gathers CPU metrics
func (c *CPUCollector) Collect() ([]collector.Metrics, error) {
	metrics := make([]collector.Metrics, 0, 1)
	now := time.Now()

	// Collect CPU usage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return metrics, err
	}

	if len(cpuPercent) > 0 {
		value := collector.RoundToTwoDecimalPlaces(cpuPercent[0])
		metrics = append(metrics, collector.Metrics{
			Timestamp: now,
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     value,
		})
	}

	return metrics, nil
}
