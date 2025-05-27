package system

import (
	"testing"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
)

func TestNewServiceCollector(t *testing.T) {
	services := []config.Service{
		{
			Name:  "test-service",
			Label: "Test Service",
		},
	}

	c := NewServiceCollector(services)

	if c == nil {
		t.Errorf("NewServiceCollector() returned nil")
		return
	}

	// Test that it implements the Collector interface
	var _ collector.Collector = c

	// Test that it's the correct type
	if _, ok := c.(*ServiceCollector); !ok {
		t.Errorf("NewServiceCollector() returned wrong type: %T", c)
	}
}

func TestServiceCollector_Collect(t *testing.T) {
	services := []config.Service{
		{
			Name:  "test-service",
			Label: "Test Service",
		},
	}

	c := &ServiceCollector{
		Services: services,
	}

	metrics, err := c.Collect()
	if err != nil {
		t.Logf("ServiceCollector.Collect() error (might be expected in test environment): %v", err)
	}

	if len(metrics) != len(services) {
		t.Errorf("ServiceCollector.Collect() returned %d metrics, want %d", len(metrics), len(services))
		return
	}

	for i, metric := range metrics {
		if metric.Category != collector.CategorySystem {
			t.Errorf("ServiceCollector.Collect() metric %d category = %v, want %v", i, metric.Category, collector.CategorySystem)
		}
		if metric.Name != collector.NameService {
			t.Errorf("ServiceCollector.Collect() metric %d name = %v, want %v", i, metric.Name, collector.NameService)
		}
		if metric.Timestamp.IsZero() {
			t.Errorf("ServiceCollector.Collect() metric %d timestamp is zero", i)
		}
		if metric.Value == nil {
			t.Errorf("ServiceCollector.Collect() metric %d value is nil", i)
		}

		// Check metadata
		if name, exists := metric.Metadata["name"]; !exists {
			t.Errorf("ServiceCollector.Collect() metric %d missing name metadata", i)
		} else if name != services[i].Name {
			t.Errorf("ServiceCollector.Collect() metric %d name metadata = %v, want %v", i, name, services[i].Name)
		}

		if label, exists := metric.Metadata["label"]; !exists {
			t.Errorf("ServiceCollector.Collect() metric %d missing label metadata", i)
		} else if label != services[i].Label {
			t.Errorf("ServiceCollector.Collect() metric %d label metadata = %v, want %v", i, label, services[i].Label)
		}

		// Check that the value is a float64 (0.0 for active, 1.0 for inactive)
		if value, ok := metric.Value.(float64); !ok {
			t.Errorf("ServiceCollector.Collect() metric %d value is not float64: %T", i, metric.Value)
		} else if value != 0.0 && value != 1.0 {
			t.Errorf("ServiceCollector.Collect() metric %d value = %v, want 0.0 or 1.0", i, value)
		}
	}
}

// BenchmarkServiceCollector_Collect benchmarks service collection
func BenchmarkServiceCollector_Collect(b *testing.B) {
	services := []config.Service{
		{
			Name:  "test-service",
			Label: "Test Service",
		},
	}

	c := &ServiceCollector{
		Services: services,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Collect()
		if err != nil {
			b.Fatalf("ServiceCollector.Collect() failed: %v", err)
		}
	}
}
