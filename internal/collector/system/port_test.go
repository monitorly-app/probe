package system

import (
	"testing"

	"github.com/monitorly-app/probe/internal/collector"
)

func TestPortCollector_Collect(t *testing.T) {
	c := &PortCollector{}

	metrics, err := c.Collect()
	if err != nil {
		t.Logf("PortCollector.Collect() error (might be expected in test environment): %v", err)
	}

	// Should return exactly one metric
	if len(metrics) != 1 {
		t.Errorf("PortCollector.Collect() returned %d metrics, want 1", len(metrics))
		return
	}

	metric := metrics[0]

	// Verify metric properties
	if metric.Category != collector.CategorySystem {
		t.Errorf("PortCollector.Collect() metric category = %v, want %v", metric.Category, collector.CategorySystem)
	}
	if metric.Name != collector.NamePort {
		t.Errorf("PortCollector.Collect() metric name = %v, want %v", metric.Name, collector.NamePort)
	}
	if metric.Timestamp.IsZero() {
		t.Errorf("PortCollector.Collect() metric timestamp is zero")
	}
	if metric.Value == nil {
		t.Errorf("PortCollector.Collect() metric value is nil")
	}

	// Check that the value is a slice of PortInfo
	if portInfos, ok := metric.Value.([]PortInfo); ok {
		// Validate PortInfo structure if any ports are returned
		for i, portInfo := range portInfos {
			if portInfo.Protocol != "tcp" && portInfo.Protocol != "udp" {
				t.Errorf("PortCollector.Collect() metric value[%d] has invalid protocol: %s", i, portInfo.Protocol)
			}

			// Local port should be valid
			if portInfo.LocalPort < 0 || portInfo.LocalPort > 65535 {
				t.Errorf("PortCollector.Collect() metric value[%d] has invalid local port: %d", i, portInfo.LocalPort)
			}

			// If remote port is set, it should be valid
			if portInfo.RemotePort > 65535 {
				t.Errorf("PortCollector.Collect() metric value[%d] has invalid remote port: %d", i, portInfo.RemotePort)
			}
		}
		t.Logf("PortCollector.Collect() returned %d port entries", len(portInfos))
	} else {
		t.Errorf("PortCollector.Collect() metric value is not []PortInfo, got %T", metric.Value)
	}
}

func TestPortCollector_getOpenPorts(t *testing.T) {
	c := &PortCollector{}

	ports, err := c.getOpenPorts()
	if err != nil {
		t.Logf("PortCollector.getOpenPorts() error (might be expected in test environment): %v", err)
		return
	}

	// Validate that returned ports have proper structure
	for i, port := range ports {
		if port.Protocol != "tcp" && port.Protocol != "udp" {
			t.Errorf("getOpenPorts() port[%d] has invalid protocol: %s", i, port.Protocol)
		}

		if port.LocalPort < 0 || port.LocalPort > 65535 {
			t.Errorf("getOpenPorts() port[%d] has invalid local port: %d", i, port.LocalPort)
		}

		// If process ID is set, it should be positive
		if port.ProcessID < 0 {
			t.Errorf("getOpenPorts() port[%d] has invalid process ID: %d", i, port.ProcessID)
		}
	}

	t.Logf("getOpenPorts() returned %d port entries", len(ports))
}

func TestPortCollector_getProcessName(t *testing.T) {
	c := &PortCollector{}

	// Test with an invalid PID (should return an error)
	_, err := c.getProcessName(-1)
	if err == nil {
		t.Errorf("getProcessName(-1) should return an error for invalid PID")
	}

	// Test with PID 1 (init process, should exist on Unix systems)
	if name, err := c.getProcessName(1); err == nil {
		if name == "" {
			t.Errorf("getProcessName(1) returned empty name")
		} else {
			t.Logf("getProcessName(1) returned: %s", name)
		}
	} else {
		t.Logf("getProcessName(1) error (might be expected in test environment): %v", err)
	}
}

func TestPortInfo_Structure(t *testing.T) {
	// Test that PortInfo struct can be properly instantiated
	portInfo := PortInfo{
		Protocol:    "tcp",
		LocalAddr:   "127.0.0.1",
		LocalPort:   8080,
		RemoteAddr:  "0.0.0.0",
		RemotePort:  0,
		Status:      "LISTEN",
		ProcessID:   1234,
		ProcessName: "test-process",
	}

	if portInfo.Protocol != "tcp" {
		t.Errorf("PortInfo protocol mismatch")
	}
	if portInfo.LocalPort != 8080 {
		t.Errorf("PortInfo local port mismatch")
	}
	if portInfo.ProcessID != 1234 {
		t.Errorf("PortInfo process ID mismatch")
	}
}

// BenchmarkPortCollector_Collect benchmarks port collection
func BenchmarkPortCollector_Collect(b *testing.B) {
	c := &PortCollector{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Collect()
		if err != nil {
			b.Fatalf("PortCollector.Collect() failed: %v", err)
		}
	}
}
