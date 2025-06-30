package system

import (
	"testing"

	"github.com/monitorly-app/probe/internal/collector"
)

func TestNewSystemInfoCollector(t *testing.T) {
	c := NewSystemInfoCollector()

	if c == nil {
		t.Errorf("NewSystemInfoCollector() returned nil")
		return
	}

	// Test that it implements the Collector interface
	var _ collector.Collector = c

	// Test that it's the correct type
	if _, ok := c.(*SystemInfoCollector); !ok {
		t.Errorf("NewSystemInfoCollector() returned wrong type: %T", c)
	}
}

func TestSystemInfoCollector_Collect(t *testing.T) {
	c := &SystemInfoCollector{}

	metrics, err := c.Collect()
	if err != nil {
		t.Logf("SystemInfoCollector.Collect() error (might be expected in test environment): %v", err)
	}

	// Should return exactly one metric
	if len(metrics) != 1 {
		t.Errorf("SystemInfoCollector.Collect() returned %d metrics, want 1", len(metrics))
		return
	}

	metric := metrics[0]

	// Verify metric properties
	if metric.Category != collector.CategorySystem {
		t.Errorf("SystemInfoCollector.Collect() metric category = %v, want %v", metric.Category, collector.CategorySystem)
	}
	if metric.Name != collector.NameSystemInfo {
		t.Errorf("SystemInfoCollector.Collect() metric name = %v, want %v", metric.Name, collector.NameSystemInfo)
	}
	if metric.Timestamp.IsZero() {
		t.Errorf("SystemInfoCollector.Collect() metric timestamp is zero")
	}
	if metric.Value == nil {
		t.Errorf("SystemInfoCollector.Collect() metric value is nil")
	}

	// Check that the value is a SystemInfo struct
	if info, ok := metric.Value.(*SystemInfo); ok {
		// Verify required fields are not empty
		if info.Hostname == "" {
			t.Error("SystemInfoCollector.Collect() hostname is empty")
		}
		if info.OS == "" {
			t.Error("SystemInfoCollector.Collect() OS is empty")
		}
		if info.OSVersion == "" {
			t.Error("SystemInfoCollector.Collect() OS version is empty")
		}
		if info.KernelVersion == "" {
			t.Error("SystemInfoCollector.Collect() kernel version is empty")
		}
		if info.CPU.Name == "" {
			t.Error("SystemInfoCollector.Collect() CPU name is empty")
		}
		if info.CPU.Cores <= 0 {
			t.Error("SystemInfoCollector.Collect() CPU cores is not positive")
		}
		if info.CPU.Frequency <= 0 {
			t.Error("SystemInfoCollector.Collect() CPU frequency is not positive")
		}
		if info.RAM.Total <= 0 {
			t.Error("SystemInfoCollector.Collect() RAM total is not positive")
		}
		if info.LastBootTime <= 0 {
			t.Error("SystemInfoCollector.Collect() last boot time is not positive")
		}

		// Note: PublicIP, Disks, and Services might be empty in test environment
		t.Logf("SystemInfo: %+v", info)
	} else {
		t.Errorf("SystemInfoCollector.Collect() metric value is not *SystemInfo, got %T", metric.Value)
	}
}
