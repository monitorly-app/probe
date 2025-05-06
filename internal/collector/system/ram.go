package system

import (
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/shirou/gopsutil/v4/mem"
)

// RAMCollector implements the collector.Collector interface for RAM metrics
type RAMCollector struct{}

// NewRAMCollector creates a new instance of RAMCollector
func NewRAMCollector() collector.Collector {
	return &RAMCollector{}
}

// Collect gathers RAM metrics
func (c *RAMCollector) Collect() ([]collector.Metrics, error) {
	metrics := make([]collector.Metrics, 0, 1)
	now := time.Now()

	// Collect memory usage
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return metrics, err
	}

	value := collector.RoundToTwoDecimalPlaces(memInfo.UsedPercent)
	metrics = append(metrics, collector.Metrics{
		Timestamp: now,
		Category:  collector.CategorySystem,
		Name:      collector.NameRAM,
		Value:     value,
	})

	return metrics, nil
}
