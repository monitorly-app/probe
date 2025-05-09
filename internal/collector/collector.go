package collector

import (
	"math"
	"time"
)

// MetricCategory represents the category of a metric
type MetricCategory string

// MetricName represents the name of a metric
type MetricName string

const (
	// CategorySystem is the category for system metrics
	CategorySystem MetricCategory = "system"

	// NameCPU is the name for CPU metrics
	NameCPU MetricName = "cpu"
	// NameRAM is the name for RAM metrics
	NameRAM MetricName = "ram"
	// NameDisk is the name for disk metrics
	NameDisk MetricName = "disk"
)

// MetricMetadata contains additional information about a metric
type MetricMetadata map[string]string

// MetricValue represents the value of a metric
type MetricValue interface{}

// Metrics represents a single metric data point
type Metrics struct {
	Timestamp time.Time      `json:"timestamp"`
	Category  MetricCategory `json:"category"`
	Name      MetricName     `json:"name"`
	Metadata  MetricMetadata `json:"metadata,omitempty"`
	Value     MetricValue    `json:"value"`
}

// Collector defines the interface for metric collection
type Collector interface {
	Collect() ([]Metrics, error)
}

// RoundToTwoDecimalPlaces rounds a float64 to two decimal places
func RoundToTwoDecimalPlaces(value float64) float64 {
	return math.Round(value*100) / 100
}
