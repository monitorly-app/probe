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
	// NameService is the name for service metrics
	NameService MetricName = "service"
	// NameUserActivity is the name for user activity metrics
	NameUserActivity MetricName = "user_activity"
	// NameLoginFailures is the name for login failure metrics
	NameLoginFailures MetricName = "login_failures"
	// NamePort is the name for port monitoring metrics
	NamePort MetricName = "port"
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
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	// Use math.Round to handle edge cases like 1.005 correctly
	// We add a small epsilon to handle floating point precision issues
	epsilon := 1e-10
	return math.Round((value+epsilon)*100) / 100
}
