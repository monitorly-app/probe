package system

import (
	"testing"

	"github.com/monitorly-app/probe/internal/collector"
)

func TestNewUserActivityCollector(t *testing.T) {
	c := NewUserActivityCollector()

	if c == nil {
		t.Errorf("NewUserActivityCollector() returned nil")
		return
	}

	// Test that it implements the Collector interface
	var _ collector.Collector = c

	// Test that it's the correct type
	if _, ok := c.(*UserActivityCollector); !ok {
		t.Errorf("NewUserActivityCollector() returned wrong type: %T", c)
	}
}

func TestUserActivityCollector_Collect(t *testing.T) {
	c := &UserActivityCollector{}

	metrics, err := c.Collect()
	if err != nil {
		t.Logf("UserActivityCollector.Collect() error (might be expected in test environment): %v", err)
	}

	// Should return exactly one metric
	if len(metrics) != 1 {
		t.Errorf("UserActivityCollector.Collect() returned %d metrics, want 1", len(metrics))
		return
	}

	metric := metrics[0]

	// Verify metric properties
	if metric.Category != collector.CategorySystem {
		t.Errorf("UserActivityCollector.Collect() metric category = %v, want %v", metric.Category, collector.CategorySystem)
	}
	if metric.Name != collector.NameUserActivity {
		t.Errorf("UserActivityCollector.Collect() metric name = %v, want %v", metric.Name, collector.NameUserActivity)
	}
	if metric.Timestamp.IsZero() {
		t.Errorf("UserActivityCollector.Collect() metric timestamp is zero")
	}
	if metric.Value == nil {
		t.Errorf("UserActivityCollector.Collect() metric value is nil")
	}

	// Check that the value is a slice of UserSession
	if sessions, ok := metric.Value.([]UserSession); ok {
		t.Logf("Found %d active sessions", len(sessions))
		for i, session := range sessions {
			if session.Username == "" {
				t.Errorf("UserActivityCollector.Collect() session %d has empty username", i)
			}
			if session.Terminal == "" {
				t.Errorf("UserActivityCollector.Collect() session %d has empty terminal", i)
			}
			// LoginIP and LoginTime can be empty in some cases, so we don't test for them
			t.Logf("Session %d: user=%s, terminal=%s, ip=%s, time=%s",
				i, session.Username, session.Terminal, session.LoginIP, session.LoginTime)
		}
	} else {
		t.Errorf("UserActivityCollector.Collect() metric value is not []UserSession: %T", metric.Value)
	}
}

func TestUserActivityCollector_parseWhoOutput(t *testing.T) {
	c := &UserActivityCollector{}

	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "single user with IP",
			output:   "user1    pts/0        2024-01-15 10:30 (192.168.1.100)",
			expected: 1,
		},
		{
			name:     "single user without IP",
			output:   "user1    console      2024-01-15 10:30",
			expected: 1,
		},
		{
			name: "multiple users",
			output: `user1    pts/0        2024-01-15 10:30 (192.168.1.100)
user2    pts/1        2024-01-15 11:00 (10.0.0.50)
root     console      2024-01-15 09:00`,
			expected: 3,
		},
		{
			name: "users with IPv6",
			output: `user1    pts/0        2024-01-15 10:30 (2001:db8::1)
user2    pts/1        2024-01-15 11:00 (192.168.1.100)`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions, err := c.parseWhoOutput(tt.output)
			if err != nil {
				t.Errorf("parseWhoOutput() error = %v", err)
				return
			}

			if len(sessions) != tt.expected {
				t.Errorf("parseWhoOutput() returned %d sessions, want %d", len(sessions), tt.expected)
				return
			}

			// Verify session data for non-empty outputs
			if tt.expected > 0 {
				for i, session := range sessions {
					if session.Username == "" {
						t.Errorf("parseWhoOutput() session %d has empty username", i)
					}
					if session.Terminal == "" {
						t.Errorf("parseWhoOutput() session %d has empty terminal", i)
					}
					t.Logf("Parsed session %d: user=%s, terminal=%s, ip=%s, time=%s",
						i, session.Username, session.Terminal, session.LoginIP, session.LoginTime)
				}
			}
		})
	}
}

func TestUserActivityCollector_isIPAddress(t *testing.T) {
	c := &UserActivityCollector{}

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"valid IPv4", "192.168.1.100", true},
		{"valid IPv4 localhost", "127.0.0.1", true},
		{"valid IPv4 zero", "0.0.0.0", true},
		{"valid IPv6", "2001:db8::1", true},
		{"valid IPv6 localhost", "::1", true},
		{"invalid - not IP", "hostname", false},
		{"invalid - empty", "", false},
		{"invalid - partial IP", "192.168.1", false},
		{"invalid - too many octets", "192.168.1.1.1", false},
		{"invalid - letters in IP", "192.168.a.1", false},
		{"invalid - single colon", ":", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.isIPAddress(tt.ip)
			if got != tt.want {
				t.Errorf("isIPAddress(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// BenchmarkUserActivityCollector_Collect benchmarks user activity collection
func BenchmarkUserActivityCollector_Collect(b *testing.B) {
	c := &UserActivityCollector{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Collect()
		if err != nil {
			b.Fatalf("UserActivityCollector.Collect() failed: %v", err)
		}
	}
}
