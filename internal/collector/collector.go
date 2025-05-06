package collector

import "time"

// Metrics represents the data collected by a collector
type Metrics struct {
	Timestamp time.Time `json:"timestamp"`
	CPUUsage  float64   `json:"cpu_usage"`
	RAMUsage  float64   `json:"ram_usage"`
	DiskUsage float64   `json:"disk_usage"`
}

// Collector defines the interface for metric collection
type Collector interface {
	Collect() (Metrics, error)
}
