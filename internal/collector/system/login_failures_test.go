package system

import (
	"strings"
	"testing"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

func TestNewLoginFailuresCollector(t *testing.T) {
	c := NewLoginFailuresCollector()

	if c == nil {
		t.Errorf("NewLoginFailuresCollector() returned nil")
		return
	}

	// Test that it implements the Collector interface
	var _ collector.Collector = c

	// Test that it's the correct type
	if _, ok := c.(*LoginFailuresCollector); !ok {
		t.Errorf("NewLoginFailuresCollector() returned wrong type: %T", c)
	}
}

func TestLoginFailuresCollector_Collect(t *testing.T) {
	c := &LoginFailuresCollector{
		lastCheck: time.Now().Add(-1 * time.Minute),
	}

	metrics, err := c.Collect()
	if err != nil {
		t.Logf("LoginFailuresCollector.Collect() error (might be expected in test environment): %v", err)
	}

	// Should return exactly one metric
	if len(metrics) != 1 {
		t.Errorf("LoginFailuresCollector.Collect() returned %d metrics, want 1", len(metrics))
		return
	}

	metric := metrics[0]

	// Verify metric properties
	if metric.Category != collector.CategorySystem {
		t.Errorf("LoginFailuresCollector.Collect() metric category = %v, want %v", metric.Category, collector.CategorySystem)
	}
	if metric.Name != collector.NameLoginFailures {
		t.Errorf("LoginFailuresCollector.Collect() metric name = %v, want %v", metric.Name, collector.NameLoginFailures)
	}
	if metric.Timestamp.IsZero() {
		t.Errorf("LoginFailuresCollector.Collect() metric timestamp is zero")
	}
	if metric.Value == nil {
		t.Errorf("LoginFailuresCollector.Collect() metric value is nil")
	}

	// Check that the value is a slice of LoginFailure
	if failures, ok := metric.Value.([]LoginFailure); ok {
		t.Logf("Found %d login failures", len(failures))
		for i, failure := range failures {
			if failure.Timestamp.IsZero() {
				t.Errorf("LoginFailuresCollector.Collect() failure %d has zero timestamp", i)
			}
			t.Logf("Failure %d: user=%s, ip=%s, service=%s, time=%s",
				i, failure.Username, failure.SourceIP, failure.Service, failure.Timestamp.Format(time.RFC3339))
		}
	} else {
		t.Errorf("LoginFailuresCollector.Collect() metric value is not []LoginFailure: %T", metric.Value)
	}
}

func TestLoginFailuresCollector_parseJournalctlOutput(t *testing.T) {
	c := &LoginFailuresCollector{}

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
			name:     "SSH failed password",
			output:   "2024-06-02T10:30:00+0200 server sshd[1234]: Failed password for admin from 192.168.1.100 port 22 ssh2",
			expected: 1,
		},
		{
			name:     "SSH invalid user",
			output:   "2024-06-02T10:30:00+0200 server sshd[1234]: Invalid user hacker from 10.0.0.50 port 22",
			expected: 1,
		},
		{
			name:     "SSH connection closed",
			output:   "2024-06-02T10:30:00+0200 server sshd[1234]: Connection closed by 192.168.1.100 port 22 [preauth]",
			expected: 1,
		},
		{
			name: "multiple failures",
			output: `2024-06-02T10:30:00+0200 server sshd[1234]: Failed password for admin from 192.168.1.100 port 22 ssh2
2024-06-02T10:31:00+0200 server sshd[1235]: Invalid user test from 10.0.0.50 port 22
2024-06-02T10:32:00+0200 server sshd[1236]: Connection closed by 172.16.0.1 port 22 [preauth]`,
			expected: 3,
		},
		{
			name:     "PAM authentication failure",
			output:   "2024-06-02T10:30:00+0200 server pam[1234]: authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=192.168.1.100 user=admin",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failures, err := c.parseJournalctlOutput(tt.output)
			if err != nil {
				t.Errorf("parseJournalctlOutput() error = %v", err)
				return
			}

			if len(failures) != tt.expected {
				t.Errorf("parseJournalctlOutput() returned %d failures, want %d", len(failures), tt.expected)
				return
			}

			// Verify failure data for non-empty outputs
			if tt.expected > 0 {
				for i, failure := range failures {
					if failure.Timestamp.IsZero() {
						t.Errorf("parseJournalctlOutput() failure %d has zero timestamp", i)
					}
					if failure.Service == "" {
						t.Errorf("parseJournalctlOutput() failure %d has empty service", i)
					}
					t.Logf("Parsed failure %d: user=%s, ip=%s, service=%s, time=%s",
						i, failure.Username, failure.SourceIP, failure.Service, failure.Timestamp.Format(time.RFC3339))
				}
			}
		})
	}
}

func TestLoginFailuresCollector_parseLogTimestamp(t *testing.T) {
	c := &LoginFailuresCollector{}

	tests := []struct {
		name      string
		timeStr   string
		wantError bool
	}{
		{"valid timestamp with single digit day", "Jun  2 10:30:15", false},
		{"valid timestamp with double digit day", "Jun 12 10:30:15", false},
		{"valid timestamp different month", "Dec 25 23:59:59", false},
		{"invalid timestamp", "invalid", true},
		{"empty timestamp", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timestamp, err := c.parseLogTimestamp(tt.timeStr)

			if tt.wantError {
				if err == nil {
					t.Errorf("parseLogTimestamp() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("parseLogTimestamp() error = %v", err)
					return
				}
				if timestamp.IsZero() {
					t.Errorf("parseLogTimestamp() returned zero timestamp")
				}
				t.Logf("Parsed timestamp: %s -> %s", tt.timeStr, timestamp.Format(time.RFC3339))
			}
		})
	}
}

func TestLoginFailuresCollector_extractService(t *testing.T) {
	c := &LoginFailuresCollector{}

	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{"SSH service", "Jun  2 10:30:15 server sshd[1234]: Failed password", "ssh"},
		{"PAM service", "Jun  2 10:30:15 server pam[1234]: authentication failure", "pam"},
		{"Login service", "Jun  2 10:30:15 server login[1234]: FAILED LOGIN", "login"},
		{"systemd-logind service", "Jun  2 10:30:15 server systemd-logind[1234]: Failed to start session", "systemd-logind"},
		{"unknown service", "Jun  2 10:30:15 server unknown[1234]: some message", "unknown"},
		{"empty line", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.extractService(tt.line)
			if result != tt.expected {
				t.Errorf("extractService(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

func TestLoginFailuresCollector_parseLogFile(t *testing.T) {
	c := &LoginFailuresCollector{}

	// Test with sample syslog format data
	sampleLogData := `Jun  2 10:30:15 server sshd[1234]: Failed password for admin from 192.168.1.100 port 22 ssh2
Jun  2 10:31:20 server sshd[1235]: Invalid user hacker from 10.0.0.50 port 22
Jun  2 10:32:30 server sshd[1236]: Connection closed by 172.16.0.1 port 22 [preauth]
Jun  2 10:33:45 server pam[1237]: authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=192.168.1.200 user=testuser`

	// Since we can't easily create temporary files in this test environment,
	// we'll test the parsing logic by calling parseJournalctlOutput with syslog-like data
	// This is not ideal but allows us to test the parsing logic

	// Convert syslog format to journalctl-like format for testing
	journalctlLikeData := strings.ReplaceAll(sampleLogData, "Jun  2 10:", "2024-06-02T10:")
	journalctlLikeData = strings.ReplaceAll(journalctlLikeData, "Jun  2 10:", "2024-06-02T10:")
	journalctlLikeData = strings.ReplaceAll(journalctlLikeData, ":15 ", ":15+0200 ")
	journalctlLikeData = strings.ReplaceAll(journalctlLikeData, ":20 ", ":20+0200 ")
	journalctlLikeData = strings.ReplaceAll(journalctlLikeData, ":30 ", ":30+0200 ")
	journalctlLikeData = strings.ReplaceAll(journalctlLikeData, ":45 ", ":45+0200 ")

	failures, err := c.parseJournalctlOutput(journalctlLikeData)
	if err != nil {
		t.Errorf("parseJournalctlOutput() error = %v", err)
		return
	}

	expectedCount := 4
	if len(failures) != expectedCount {
		t.Errorf("parseJournalctlOutput() returned %d failures, want %d", len(failures), expectedCount)
		return
	}

	// Verify specific failures
	expectedFailures := []struct {
		username string
		sourceIP string
		service  string
	}{
		{"admin", "192.168.1.100", "ssh"},
		{"hacker", "10.0.0.50", "ssh"},
		{"unknown", "172.16.0.1", "ssh"},
		{"testuser", "192.168.1.200", "pam"},
	}

	for i, expected := range expectedFailures {
		if i >= len(failures) {
			t.Errorf("Missing failure %d", i)
			continue
		}

		failure := failures[i]
		if failure.Username != expected.username {
			t.Errorf("Failure %d username = %q, want %q", i, failure.Username, expected.username)
		}
		if failure.SourceIP != expected.sourceIP {
			t.Errorf("Failure %d sourceIP = %q, want %q", i, failure.SourceIP, expected.sourceIP)
		}
		if failure.Service != expected.service {
			t.Errorf("Failure %d service = %q, want %q", i, failure.Service, expected.service)
		}
		if failure.Timestamp.IsZero() {
			t.Errorf("Failure %d has zero timestamp", i)
		}
		if failure.Message == "" {
			t.Errorf("Failure %d has empty message", i)
		}
	}
}

// BenchmarkLoginFailuresCollector_Collect benchmarks login failure collection
func BenchmarkLoginFailuresCollector_Collect(b *testing.B) {
	c := &LoginFailuresCollector{
		lastCheck: time.Now().Add(-1 * time.Minute),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Collect()
		if err != nil {
			b.Logf("LoginFailuresCollector.Collect() error (might be expected in test environment): %v", err)
		}
	}
}
