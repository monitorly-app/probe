package collector

import (
	"math"
	"testing"
	"time"
)

func TestRoundToTwoDecimalPlaces(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{
			name:  "positive value with more than 2 decimals",
			value: 75.12345,
			want:  75.12,
		},
		{
			name:  "positive value with exactly 2 decimals",
			value: 75.12,
			want:  75.12,
		},
		{
			name:  "positive value with 1 decimal",
			value: 75.1,
			want:  75.1,
		},
		{
			name:  "positive integer",
			value: 75.0,
			want:  75.0,
		},
		{
			name:  "negative value with more than 2 decimals",
			value: -75.12345,
			want:  -75.12,
		},
		{
			name:  "negative value with exactly 2 decimals",
			value: -75.12,
			want:  -75.12,
		},
		{
			name:  "zero",
			value: 0.0,
			want:  0.0,
		},
		{
			name:  "very small positive value",
			value: 0.001,
			want:  0.0,
		},
		{
			name:  "very small negative value",
			value: -0.001,
			want:  -0.0,
		},
		{
			name:  "value requiring rounding up",
			value: 75.126,
			want:  75.13,
		},
		{
			name:  "value requiring rounding down",
			value: 75.124,
			want:  75.12,
		},
		{
			name:  "large value",
			value: 999999.999,
			want:  1000000.0,
		},
		{
			name:  "value with many decimal places",
			value: 12.3456789,
			want:  12.35,
		},
		{
			name:  "edge case - 0.005",
			value: 0.005,
			want:  0.01,
		},
		{
			name:  "edge case - 0.004",
			value: 0.004,
			want:  0.0,
		},
		{
			name:  "infinity",
			value: math.Inf(1),
			want:  math.Inf(1),
		},
		{
			name:  "negative infinity",
			value: math.Inf(-1),
			want:  math.Inf(-1),
		},
		{
			name:  "NaN",
			value: math.NaN(),
			want:  math.NaN(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RoundToTwoDecimalPlaces(tt.value)

			// Special handling for NaN
			if math.IsNaN(tt.want) {
				if !math.IsNaN(got) {
					t.Errorf("RoundToTwoDecimalPlaces() = %v, want NaN", got)
				}
				return
			}

			// Special handling for infinity
			if math.IsInf(tt.want, 0) {
				if !math.IsInf(got, 0) || math.Signbit(got) != math.Signbit(tt.want) {
					t.Errorf("RoundToTwoDecimalPlaces() = %v, want %v", got, tt.want)
				}
				return
			}

			if got != tt.want {
				t.Errorf("RoundToTwoDecimalPlaces() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetricConstants(t *testing.T) {
	// Test that constants have expected values
	tests := []struct {
		name     string
		constant interface{}
		expected interface{}
	}{
		{
			name:     "CategorySystem",
			constant: CategorySystem,
			expected: MetricCategory("system"),
		},
		{
			name:     "NameCPU",
			constant: NameCPU,
			expected: MetricName("cpu"),
		},
		{
			name:     "NameRAM",
			constant: NameRAM,
			expected: MetricName("ram"),
		},
		{
			name:     "NameDisk",
			constant: NameDisk,
			expected: MetricName("disk"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Constant %s = %v, want %v", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestMetricsStruct(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		metric Metrics
	}{
		{
			name: "basic metric",
			metric: Metrics{
				Timestamp: now,
				Category:  CategorySystem,
				Name:      NameCPU,
				Value:     75.5,
			},
		},
		{
			name: "metric with metadata",
			metric: Metrics{
				Timestamp: now,
				Category:  CategorySystem,
				Name:      NameDisk,
				Metadata: MetricMetadata{
					"mountpoint": "/",
					"label":      "root",
				},
				Value: map[string]interface{}{
					"percent": 45.2,
					"used":    1024000,
				},
			},
		},
		{
			name: "metric with nil metadata",
			metric: Metrics{
				Timestamp: now,
				Category:  CategorySystem,
				Name:      NameRAM,
				Metadata:  nil,
				Value:     60.2,
			},
		},
		{
			name: "metric with empty metadata",
			metric: Metrics{
				Timestamp: now,
				Category:  CategorySystem,
				Name:      NameCPU,
				Metadata:  MetricMetadata{},
				Value:     80.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the struct can be created and accessed
			metric := tt.metric

			if metric.Timestamp.IsZero() {
				t.Errorf("Metrics.Timestamp should not be zero")
			}

			if metric.Category == "" {
				t.Errorf("Metrics.Category should not be empty")
			}

			if metric.Name == "" {
				t.Errorf("Metrics.Name should not be empty")
			}

			if metric.Value == nil {
				t.Errorf("Metrics.Value should not be nil")
			}
		})
	}
}

func TestMetricMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata MetricMetadata
		key      string
		expected string
		exists   bool
	}{
		{
			name: "existing key",
			metadata: MetricMetadata{
				"mountpoint": "/",
				"label":      "root",
			},
			key:      "mountpoint",
			expected: "/",
			exists:   true,
		},
		{
			name: "non-existing key",
			metadata: MetricMetadata{
				"mountpoint": "/",
				"label":      "root",
			},
			key:      "nonexistent",
			expected: "",
			exists:   false,
		},
		{
			name:     "empty metadata",
			metadata: MetricMetadata{},
			key:      "any",
			expected: "",
			exists:   false,
		},
		{
			name:     "nil metadata",
			metadata: nil,
			key:      "any",
			expected: "",
			exists:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, exists := tt.metadata[tt.key]

			if exists != tt.exists {
				t.Errorf("MetricMetadata[%s] exists = %v, want %v", tt.key, exists, tt.exists)
			}

			if exists && value != tt.expected {
				t.Errorf("MetricMetadata[%s] = %v, want %v", tt.key, value, tt.expected)
			}
		})
	}
}

func TestMetricTypes(t *testing.T) {
	// Test that the types can be used as expected
	var category MetricCategory = "test"
	var name MetricName = "test"
	var metadata MetricMetadata = make(map[string]string)
	var value MetricValue = "test"

	if category != "test" {
		t.Errorf("MetricCategory assignment failed")
	}

	if name != "test" {
		t.Errorf("MetricName assignment failed")
	}

	if metadata == nil {
		t.Errorf("MetricMetadata assignment failed")
	}

	if value != "test" {
		t.Errorf("MetricValue assignment failed")
	}

	// Test that MetricValue can hold different types
	testValues := []struct {
		name  string
		value MetricValue
	}{
		{"string", "string"},
		{"int", 42},
		{"float", 42.5},
		{"bool", true},
		{"map", map[string]interface{}{"key": "value"}},
		{"slice", []int{1, 2, 3}},
	}

	for _, tt := range testValues {
		t.Run(tt.name, func(t *testing.T) {
			var mv MetricValue = tt.value
			// For comparable types, test equality
			switch tt.value.(type) {
			case string, int, float64, bool:
				if mv != tt.value {
					t.Errorf("MetricValue assignment failed for type %T", tt.value)
				}
			default:
				// For non-comparable types like maps and slices, just verify assignment worked
				if mv == nil {
					t.Errorf("MetricValue assignment failed for type %T", tt.value)
				}
			}
		})
	}
}

func TestCollectorInterface(t *testing.T) {
	// Test that we can define a mock collector that implements the interface
	mockCollector := &MockCollector{
		metrics: []Metrics{
			{
				Timestamp: time.Now(),
				Category:  CategorySystem,
				Name:      NameCPU,
				Value:     75.5,
			},
		},
	}

	// Test that it implements the interface
	var collector Collector = mockCollector

	metrics, err := collector.Collect()
	if err != nil {
		t.Errorf("MockCollector.Collect() error = %v", err)
	}

	if len(metrics) != 1 {
		t.Errorf("MockCollector.Collect() returned %d metrics, want 1", len(metrics))
	}
}

// MockCollector is a test implementation of the Collector interface
type MockCollector struct {
	metrics []Metrics
	err     error
}

func (m *MockCollector) Collect() ([]Metrics, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.metrics, nil
}

func TestRoundToTwoDecimalPlacesPrecision(t *testing.T) {
	// Test specific precision cases that might be problematic
	precisionTests := []struct {
		input    float64
		expected float64
	}{
		{0.125, 0.13}, // Should round up
		{0.124, 0.12}, // Should round down
		{0.115, 0.12}, // Should round up (banker's rounding might differ)
		{0.114, 0.11}, // Should round down
		{1.005, 1.01}, // Should round up
		{1.004, 1.0},  // Should round down
		{2.675, 2.68}, // Should round up
		{2.674, 2.67}, // Should round down
	}

	for _, tt := range precisionTests {
		t.Run("precision", func(t *testing.T) {
			got := RoundToTwoDecimalPlaces(tt.input)
			if got != tt.expected {
				t.Errorf("RoundToTwoDecimalPlaces(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// BenchmarkRoundToTwoDecimalPlaces benchmarks the rounding function
func BenchmarkRoundToTwoDecimalPlaces(b *testing.B) {
	testValue := 75.12345

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RoundToTwoDecimalPlaces(testValue)
	}
}

// BenchmarkRoundToTwoDecimalPlacesVaried benchmarks with varied inputs
func BenchmarkRoundToTwoDecimalPlacesVaried(b *testing.B) {
	testValues := []float64{
		0.0,
		1.23456,
		-1.23456,
		999999.999,
		0.001,
		75.126,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, val := range testValues {
			_ = RoundToTwoDecimalPlaces(val)
		}
	}
}
