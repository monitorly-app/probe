package system

import (
	"testing"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
)

func TestNewCollectors(t *testing.T) {
	collectors := NewCollectors()

	// Test that the expected collectors are returned
	expectedCollectors := []string{"cpu", "ram"}

	if len(collectors) != len(expectedCollectors) {
		t.Errorf("NewCollectors() returned %d collectors, want %d", len(collectors), len(expectedCollectors))
	}

	for _, name := range expectedCollectors {
		if _, exists := collectors[name]; !exists {
			t.Errorf("NewCollectors() missing collector: %s", name)
		}
	}

	// Test that each collector constructor returns a valid collector
	for name, constructor := range collectors {
		t.Run(name, func(t *testing.T) {
			c := constructor()
			if c == nil {
				t.Errorf("Collector constructor for %s returned nil", name)
			}

			// Test that the collector implements the interface
			var _ collector.Collector = c
		})
	}
}

func TestNewCPUCollector(t *testing.T) {
	c := NewCPUCollector()

	if c == nil {
		t.Errorf("NewCPUCollector() returned nil")
		return
	}

	// Test that it implements the Collector interface
	var _ collector.Collector = c

	// Test that it's the correct type
	if _, ok := c.(*CPUCollector); !ok {
		t.Errorf("NewCPUCollector() returned wrong type: %T", c)
	}
}

func TestNewRAMCollector(t *testing.T) {
	c := NewRAMCollector()

	if c == nil {
		t.Errorf("NewRAMCollector() returned nil")
		return
	}

	// Test that it implements the Collector interface
	var _ collector.Collector = c

	// Test that it's the correct type
	if _, ok := c.(*RAMCollector); !ok {
		t.Errorf("NewRAMCollector() returned wrong type: %T", c)
	}
}

func TestNewDiskCollector(t *testing.T) {
	mountPoints := []config.MountPoint{
		{
			Path:           "/",
			Label:          "root",
			CollectPercent: true,
			CollectUsage:   true,
		},
		{
			Path:           "/home",
			Label:          "home",
			CollectPercent: true,
			CollectUsage:   false,
		},
	}

	c := NewDiskCollector(mountPoints)

	if c == nil {
		t.Errorf("NewDiskCollector() returned nil")
		return
	}

	// Test that it implements the Collector interface
	var _ collector.Collector = c

	// Test that it's the correct type
	diskCollector, ok := c.(*DiskCollector)
	if !ok {
		t.Errorf("NewDiskCollector() returned wrong type: %T", c)
		return
	}

	// Test that mount points are set correctly
	if len(diskCollector.MountPoints) != len(mountPoints) {
		t.Errorf("NewDiskCollector() mount points length = %d, want %d", len(diskCollector.MountPoints), len(mountPoints))
	}

	for i, mp := range diskCollector.MountPoints {
		if mp.Path != mountPoints[i].Path {
			t.Errorf("NewDiskCollector() mount point %d path = %s, want %s", i, mp.Path, mountPoints[i].Path)
		}
		if mp.Label != mountPoints[i].Label {
			t.Errorf("NewDiskCollector() mount point %d label = %s, want %s", i, mp.Label, mountPoints[i].Label)
		}
		if mp.CollectPercent != mountPoints[i].CollectPercent {
			t.Errorf("NewDiskCollector() mount point %d CollectPercent = %v, want %v", i, mp.CollectPercent, mountPoints[i].CollectPercent)
		}
		if mp.CollectUsage != mountPoints[i].CollectUsage {
			t.Errorf("NewDiskCollector() mount point %d CollectUsage = %v, want %v", i, mp.CollectUsage, mountPoints[i].CollectUsage)
		}
	}
}

func TestNewDiskCollectorFunc(t *testing.T) {
	mountPoints := []config.MountPoint{
		{
			Path:           "/",
			Label:          "root",
			CollectPercent: true,
			CollectUsage:   true,
		},
	}

	constructorFunc := NewDiskCollectorFunc(mountPoints)

	if constructorFunc == nil {
		t.Errorf("NewDiskCollectorFunc() returned nil")
		return
	}

	// Test that the constructor function works
	c := constructorFunc()

	if c == nil {
		t.Errorf("NewDiskCollectorFunc()() returned nil")
		return
	}

	// Test that it implements the Collector interface
	var _ collector.Collector = c

	// Test that it's the correct type
	diskCollector, ok := c.(*DiskCollector)
	if !ok {
		t.Errorf("NewDiskCollectorFunc()() returned wrong type: %T", c)
		return
	}

	// Test that mount points are set correctly
	if len(diskCollector.MountPoints) != len(mountPoints) {
		t.Errorf("NewDiskCollectorFunc()() mount points length = %d, want %d", len(diskCollector.MountPoints), len(mountPoints))
	}
}

func TestCPUCollector_Collect(t *testing.T) {
	c := &CPUCollector{}

	metrics, err := c.Collect()

	// Note: This test might fail in some environments where CPU metrics can't be collected
	// In a real test environment, you might want to mock the gopsutil calls
	if err != nil {
		t.Logf("CPUCollector.Collect() error (might be expected in test environment): %v", err)
		return
	}

	// If successful, verify the metrics
	if len(metrics) == 0 {
		t.Errorf("CPUCollector.Collect() returned no metrics")
		return
	}

	for _, metric := range metrics {
		if metric.Category != collector.CategorySystem {
			t.Errorf("CPUCollector.Collect() metric category = %v, want %v", metric.Category, collector.CategorySystem)
		}
		if metric.Name != collector.NameCPU {
			t.Errorf("CPUCollector.Collect() metric name = %v, want %v", metric.Name, collector.NameCPU)
		}
		if metric.Timestamp.IsZero() {
			t.Errorf("CPUCollector.Collect() metric timestamp is zero")
		}
		if metric.Value == nil {
			t.Errorf("CPUCollector.Collect() metric value is nil")
		}

		// Check that the value is a reasonable CPU percentage
		if value, ok := metric.Value.(float64); ok {
			if value < 0 || value > 100 {
				t.Errorf("CPUCollector.Collect() metric value = %v, want between 0 and 100", value)
			}
		} else {
			t.Errorf("CPUCollector.Collect() metric value is not float64: %T", metric.Value)
		}
	}
}

func TestRAMCollector_Collect(t *testing.T) {
	c := &RAMCollector{}

	metrics, err := c.Collect()

	// Note: This test might fail in some environments where RAM metrics can't be collected
	if err != nil {
		t.Logf("RAMCollector.Collect() error (might be expected in test environment): %v", err)
		return
	}

	// If successful, verify the metrics
	if len(metrics) == 0 {
		t.Errorf("RAMCollector.Collect() returned no metrics")
		return
	}

	for _, metric := range metrics {
		if metric.Category != collector.CategorySystem {
			t.Errorf("RAMCollector.Collect() metric category = %v, want %v", metric.Category, collector.CategorySystem)
		}
		if metric.Name != collector.NameRAM {
			t.Errorf("RAMCollector.Collect() metric name = %v, want %v", metric.Name, collector.NameRAM)
		}
		if metric.Timestamp.IsZero() {
			t.Errorf("RAMCollector.Collect() metric timestamp is zero")
		}
		if metric.Value == nil {
			t.Errorf("RAMCollector.Collect() metric value is nil")
		}

		// Check that the value is a reasonable RAM percentage
		if value, ok := metric.Value.(float64); ok {
			if value < 0 || value > 100 {
				t.Errorf("RAMCollector.Collect() metric value = %v, want between 0 and 100", value)
			}
		} else {
			t.Errorf("RAMCollector.Collect() metric value is not float64: %T", metric.Value)
		}
	}
}

func TestDiskCollector_Collect(t *testing.T) {
	tests := []struct {
		name        string
		mountPoints []config.MountPoint
		wantMetrics int
	}{
		{
			name: "single mount point with both percent and usage",
			mountPoints: []config.MountPoint{
				{
					Path:           "/",
					Label:          "root",
					CollectPercent: true,
					CollectUsage:   true,
				},
			},
			wantMetrics: 1,
		},
		{
			name: "single mount point with only percent",
			mountPoints: []config.MountPoint{
				{
					Path:           "/",
					Label:          "root",
					CollectPercent: true,
					CollectUsage:   false,
				},
			},
			wantMetrics: 1,
		},
		{
			name: "single mount point with only usage",
			mountPoints: []config.MountPoint{
				{
					Path:           "/",
					Label:          "root",
					CollectPercent: false,
					CollectUsage:   true,
				},
			},
			wantMetrics: 1,
		},
		{
			name: "multiple mount points",
			mountPoints: []config.MountPoint{
				{
					Path:           "/",
					Label:          "root",
					CollectPercent: true,
					CollectUsage:   true,
				},
				{
					Path:           "/tmp",
					Label:          "temp",
					CollectPercent: true,
					CollectUsage:   false,
				},
			},
			wantMetrics: 2,
		},
		{
			name:        "no mount points",
			mountPoints: []config.MountPoint{},
			wantMetrics: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &DiskCollector{
				MountPoints: tt.mountPoints,
			}

			metrics, err := c.Collect()

			// Note: This test might fail for invalid mount points
			if err != nil {
				t.Logf("DiskCollector.Collect() error (might be expected for invalid paths): %v", err)
			}

			// The number of metrics might be less than expected if some mount points fail
			if len(metrics) > tt.wantMetrics {
				t.Errorf("DiskCollector.Collect() returned %d metrics, want at most %d", len(metrics), tt.wantMetrics)
			}

			for i, metric := range metrics {
				if metric.Category != collector.CategorySystem {
					t.Errorf("DiskCollector.Collect() metric %d category = %v, want %v", i, metric.Category, collector.CategorySystem)
				}
				if metric.Name != collector.NameDisk {
					t.Errorf("DiskCollector.Collect() metric %d name = %v, want %v", i, metric.Name, collector.NameDisk)
				}
				if metric.Timestamp.IsZero() {
					t.Errorf("DiskCollector.Collect() metric %d timestamp is zero", i)
				}
				if metric.Value == nil {
					t.Errorf("DiskCollector.Collect() metric %d value is nil", i)
				}
				if metric.Metadata == nil {
					t.Errorf("DiskCollector.Collect() metric %d metadata is nil", i)
				}

				// Check metadata
				if mountpoint, exists := metric.Metadata["mountpoint"]; !exists {
					t.Errorf("DiskCollector.Collect() metric %d missing mountpoint metadata", i)
				} else if mountpoint == "" {
					t.Errorf("DiskCollector.Collect() metric %d mountpoint metadata is empty", i)
				}

				if label, exists := metric.Metadata["label"]; !exists {
					t.Errorf("DiskCollector.Collect() metric %d missing label metadata", i)
				} else if label == "" {
					t.Errorf("DiskCollector.Collect() metric %d label metadata is empty", i)
				}

				// Check value structure
				if valueMap, ok := metric.Value.(map[string]interface{}); ok {
					// Check that the value contains expected fields based on configuration
					// This is a basic check since we can't predict exact mount point configurations
					if len(valueMap) == 0 {
						t.Errorf("DiskCollector.Collect() metric %d value map is empty", i)
					}
				} else {
					t.Errorf("DiskCollector.Collect() metric %d value is not map[string]interface{}: %T", i, metric.Value)
				}
			}
		})
	}
}

func TestDiskCollector_CollectWithInvalidPath(t *testing.T) {
	c := &DiskCollector{
		MountPoints: []config.MountPoint{
			{
				Path:           "/nonexistent/path/that/should/not/exist",
				Label:          "invalid",
				CollectPercent: true,
				CollectUsage:   true,
			},
			{
				Path:           "/",
				Label:          "root",
				CollectPercent: true,
				CollectUsage:   true,
			},
		},
	}

	metrics, err := c.Collect()

	// Should not return an error, but should skip invalid paths
	if err != nil {
		t.Errorf("DiskCollector.Collect() with invalid path returned error: %v", err)
	}

	// Should have at most 1 metric (for the valid "/" path)
	if len(metrics) > 1 {
		t.Errorf("DiskCollector.Collect() with invalid path returned %d metrics, want at most 1", len(metrics))
	}

	// If we got a metric, it should be for the valid path
	if len(metrics) == 1 {
		if mountpoint, exists := metrics[0].Metadata["mountpoint"]; !exists || mountpoint != "/" {
			t.Errorf("DiskCollector.Collect() with invalid path returned metric for wrong mountpoint: %v", mountpoint)
		}
	}
}

// BenchmarkCPUCollector_Collect benchmarks CPU collection
func BenchmarkCPUCollector_Collect(b *testing.B) {
	c := &CPUCollector{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Collect()
		if err != nil {
			b.Fatalf("CPUCollector.Collect() failed: %v", err)
		}
	}
}

// BenchmarkRAMCollector_Collect benchmarks RAM collection
func BenchmarkRAMCollector_Collect(b *testing.B) {
	c := &RAMCollector{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Collect()
		if err != nil {
			b.Fatalf("RAMCollector.Collect() failed: %v", err)
		}
	}
}

// BenchmarkDiskCollector_Collect benchmarks disk collection
func BenchmarkDiskCollector_Collect(b *testing.B) {
	c := &DiskCollector{
		MountPoints: []config.MountPoint{
			{
				Path:           "/",
				Label:          "root",
				CollectPercent: true,
				CollectUsage:   true,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Collect()
		if err != nil {
			b.Fatalf("DiskCollector.Collect() failed: %v", err)
		}
	}
}
