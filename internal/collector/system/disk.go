package system

import (
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
	"github.com/shirou/gopsutil/v4/disk"
)

// DiskCollector implements the collector.Collector interface for disk metrics
type DiskCollector struct {
	MountPoints []config.MountPoint
}

// NewDiskCollector creates a new instance of DiskCollector
func NewDiskCollector(mountPoints []config.MountPoint) collector.Collector {
	return &DiskCollector{
		MountPoints: mountPoints,
	}
}

// Collect gathers disk metrics for specified mount points
func (c *DiskCollector) Collect() ([]collector.Metrics, error) {
	metrics := make([]collector.Metrics, 0, len(c.MountPoints)*2) // Potentially 2 metrics per mount point
	now := time.Now()

	for _, mp := range c.MountPoints {
		// Collect disk usage for the specified path
		diskInfo, err := disk.Usage(mp.Path)
		if err != nil {
			continue // Skip this mount point if there's an error, but continue with others
		}

		metadata := collector.MetricMetadata{
			"mountpoint": mp.Path,
			"label":      mp.Label,
		}

		// Add percentage metric if configured
		if mp.CollectPercent {
			percentValue := collector.RoundToTwoDecimalPlaces(diskInfo.UsedPercent)
			metrics = append(metrics, collector.Metrics{
				Timestamp: now,
				Category:  collector.CategorySystem,
				Name:      collector.NameDisk,
				Metadata:  metadata,
				Value: map[string]interface{}{
					"type":    "percent",
					"percent": percentValue,
				},
			})
		}

		// Add usage amount metric if configured
		if mp.CollectUsage {
			metrics = append(metrics, collector.Metrics{
				Timestamp: now,
				Category:  collector.CategorySystem,
				Name:      collector.NameDisk,
				Metadata:  metadata,
				Value: map[string]interface{}{
					"type":      "usage",
					"used":      diskInfo.Used,
					"total":     diskInfo.Total,
					"available": diskInfo.Free,
				},
			})
		}
	}

	return metrics, nil
}
