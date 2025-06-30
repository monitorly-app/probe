package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		wantErr     bool
		errContains string
		validate    func(*testing.T, *Config)
	}{
		{
			name: "valid minimal config",
			configYAML: `
api:
  url: "https://api.monitorly.io"
  organization_id: "123e4567-e89b-12d3-a456-426614174000"
  server_id: "123e4567-e89b-12d3-a456-426614174001"
  application_token: "test-token"
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.API.URL != "https://api.monitorly.io" {
					t.Errorf("expected API URL %q, got %q", "https://api.monitorly.io", cfg.API.URL)
				}
				// Check defaults were applied
				if !cfg.Collection.CPU.Enabled {
					t.Error("expected CPU collection to be enabled by default")
				}
				if cfg.Collection.CPU.Interval != 30*time.Second {
					t.Errorf("expected CPU interval %v, got %v", 30*time.Second, cfg.Collection.CPU.Interval)
				}
			},
		},
		{
			name: "valid full config",
			configYAML: `
machine_name: "test-server"
collection:
  cpu:
    enabled: true
    interval: 15s
  ram:
    enabled: true
    interval: 20s
  disk:
    enabled: true
    interval: 30s
    mount_points:
      - path: "/"
        label: "root"
        collect_usage: true
        collect_percent: true
      - path: "/home"
        label: "home"
        collect_usage: true
        collect_percent: false
sender:
  target: "api"
  send_interval: 1m
api:
  url: "https://api.monitorly.io"
  organization_id: "123e4567-e89b-12d3-a456-426614174000"
  server_id: "123e4567-e89b-12d3-a456-426614174001"
  application_token: "test-token"
  encryption_key: "12345678901234567890123456789012"
log_file:
  path: "/var/log/monitorly/metrics.log"
logging:
  file_path: "/var/log/monitorly/app.log"
updates:
  enabled: true
  check_time: "03:00"
  retry_delay: 2h
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.MachineName != "test-server" {
					t.Errorf("expected machine name %q, got %q", "test-server", cfg.MachineName)
				}
				if cfg.Collection.CPU.Interval != 15*time.Second {
					t.Errorf("expected CPU interval %v, got %v", 15*time.Second, cfg.Collection.CPU.Interval)
				}
				if len(cfg.Collection.Disk.MountPoints) != 2 {
					t.Errorf("expected 2 mount points, got %d", len(cfg.Collection.Disk.MountPoints))
				}
				if cfg.Updates.CheckTime != "03:00" {
					t.Errorf("expected check time %q, got %q", "03:00", cfg.Updates.CheckTime)
				}
			},
		},
		{
			name: "invalid yaml",
			configYAML: `
invalid:
  - yaml:
    syntax
`,
			wantErr:     true,
			errContains: "failed to parse config file",
		},
		{
			name: "missing required API fields",
			configYAML: `
sender:
  target: "api"
api:
  url: ""
`,
			wantErr:     true,
			errContains: "API URL is required",
		},
		{
			name: "invalid encryption key length",
			configYAML: `
api:
  url: "https://api.monitorly.io"
  organization_id: "123"
  server_id: "123e4567-e89b-12d3-a456-426614174000"
  application_token: "token"
  encryption_key: "too-short"
`,
			wantErr:     true,
			errContains: "encryption key must be exactly 32 bytes",
		},
		{
			name: "invalid collection intervals",
			configYAML: `
api:
  url: "https://api.monitorly.io"
  organization_id: "123"
  server_id: "123e4567-e89b-12d3-a456-426614174000"
  application_token: "token"
collection:
  cpu:
    enabled: true
    interval: 100ms
`,
			wantErr:     true,
			errContains: "CPU collection interval must be at least 1 second",
		},
		{
			name: "invalid mount point config",
			configYAML: `
api:
  url: "https://api.monitorly.io"
  organization_id: "123"
  server_id: "123e4567-e89b-12d3-a456-426614174000"
  application_token: "token"
collection:
  disk:
    enabled: true
    mount_points:
      - path: ""
        label: "root"
`,
			wantErr:     true,
			errContains: "mount point #1 is missing a path",
		},
		{
			name: "invalid sender target",
			configYAML: `
api:
  url: "https://api.monitorly.io"
  organization_id: "123"
  server_id: "123e4567-e89b-12d3-a456-426614174000"
  application_token: "token"
sender:
  target: "invalid"
`,
			wantErr:     true,
			errContains: "invalid sender target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("failed to write test config file: %v", err)
			}

			// Load the config
			cfg, err := Load(configPath)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestGetMachineName(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		wantHostname   bool
		wantErr        bool
		expectedResult string
	}{
		{
			name:           "configured machine name",
			config:         Config{MachineName: "test-server"},
			expectedResult: "test-server",
		},
		{
			name:         "fallback to hostname",
			config:       Config{},
			wantHostname: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, err := tt.config.GetMachineName()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantHostname {
				hostname, _ := os.Hostname()
				if name != hostname {
					t.Errorf("expected hostname %q, got %q", hostname, name)
				}
			} else if name != tt.expectedResult {
				t.Errorf("expected %q, got %q", tt.expectedResult, name)
			}
		})
	}
}

func TestGetUpdateCheckTime(t *testing.T) {
	tests := []struct {
		name        string
		checkTime   string
		wantErr     bool
		errContains string
		validate    func(*testing.T, time.Time)
	}{
		{
			name:      "default midnight",
			checkTime: "",
			validate: func(t *testing.T, tm time.Time) {
				if tm.Hour() != 0 || tm.Minute() != 0 {
					t.Errorf("expected midnight (00:00), got %02d:%02d", tm.Hour(), tm.Minute())
				}
			},
		},
		{
			name:      "custom time",
			checkTime: "15:30",
			validate: func(t *testing.T, tm time.Time) {
				if tm.Hour() != 15 || tm.Minute() != 30 {
					t.Errorf("expected 15:30, got %02d:%02d", tm.Hour(), tm.Minute())
				}
			},
		},
		{
			name:        "invalid format",
			checkTime:   "25:00",
			wantErr:     true,
			errContains: "invalid check_time format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Updates: struct {
					Enabled    bool          `yaml:"enabled"`
					CheckTime  string        `yaml:"check_time"`
					RetryDelay time.Duration `yaml:"retry_delay"`
				}{
					CheckTime: tt.checkTime,
				},
			}

			tm, err := cfg.GetUpdateCheckTime()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, tm)
			}
		})
	}
}

func TestGetUpdateRetryDelay(t *testing.T) {
	tests := []struct {
		name          string
		retryDelay    time.Duration
		expectedDelay time.Duration
	}{
		{
			name:          "default 1 hour",
			retryDelay:    0,
			expectedDelay: time.Hour,
		},
		{
			name:          "custom delay",
			retryDelay:    2 * time.Hour,
			expectedDelay: 2 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Updates: struct {
					Enabled    bool          `yaml:"enabled"`
					CheckTime  string        `yaml:"check_time"`
					RetryDelay time.Duration `yaml:"retry_delay"`
				}{
					RetryDelay: tt.retryDelay,
				},
			}

			delay := cfg.GetUpdateRetryDelay()
			if delay != tt.expectedDelay {
				t.Errorf("expected delay %v, got %v", tt.expectedDelay, delay)
			}
		})
	}
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
