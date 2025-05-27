package system

import (
	"os/exec"
	"testing"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
)

// mockCmd implements a mock exec.Cmd
type mockCmd struct {
	err error
}

func (m *mockCmd) Run() error {
	return m.err
}

// mockExecCommand replaces exec.Command for testing
type mockExecCommand struct {
	command string
	args    []string
	err     error
}

func (m *mockExecCommand) Command(command string, args ...string) *exec.Cmd {
	m.command = command
	m.args = args
	// Create a real command that will succeed or fail based on our mock error
	if m.err != nil {
		// Use a command that will fail
		return exec.Command("false")
	}
	// Use a command that will succeed
	return exec.Command("true")
}

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

func TestCheckServiceStatusSystemd(t *testing.T) {
	// Save original execCommand
	originalExecCommand := execCommand
	defer func() {
		execCommand = originalExecCommand
	}()

	tests := []struct {
		name    string
		service string
		err     error
		want    bool
	}{
		{
			name:    "service is running",
			service: "test-service",
			err:     nil,
			want:    true,
		},
		{
			name:    "service is not running",
			service: "test-service",
			err:     exec.ErrNotFound,
			want:    false,
		},
		{
			name:    "service check error",
			service: "test-service",
			err:     exec.ErrNotFound,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecCommand{
				err: tt.err,
			}
			execCommand = mock.Command

			got := checkServiceStatusSystemd(tt.service)
			if got != tt.want {
				t.Errorf("checkServiceStatusSystemd() = %v, want %v", got, tt.want)
			}

			// Verify the command was called correctly
			if mock.command != "systemctl" {
				t.Errorf("Expected command 'systemctl', got %s", mock.command)
			}
			if len(mock.args) != 3 || mock.args[0] != "is-active" || mock.args[1] != "--quiet" || mock.args[2] != tt.service {
				t.Errorf("Expected args [is-active --quiet %s], got %v", tt.service, mock.args)
			}
		})
	}
}

func TestNewServiceCollectorFunc(t *testing.T) {
	tests := []struct {
		name     string
		services []config.Service
		want     bool
	}{
		{
			name: "with services",
			services: []config.Service{
				{Name: "service1"},
				{Name: "service2"},
			},
			want: true,
		},
		{
			name:     "no services",
			services: nil,
			want:     false,
		},
		{
			name:     "empty services",
			services: []config.Service{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewServiceCollectorFunc(tt.services)
			got := collector != nil
			if got != tt.want {
				t.Errorf("NewServiceCollectorFunc() returned %v, want %v", got, tt.want)
			}

			// Additional check for nil services
			if tt.services == nil && got {
				t.Error("NewServiceCollectorFunc() returned non-nil for nil services")
			}
		})
	}
}
