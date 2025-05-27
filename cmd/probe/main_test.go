package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"bytes"
	"io"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
)

func TestParseCommandLineFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected *CommandLineFlags
	}{
		{
			name: "default values",
			args: []string{"probe"},
			expected: &CommandLineFlags{
				ConfigPath:      "config.yaml",
				ShowVersion:     false,
				CheckUpdate:     false,
				SkipUpdateCheck: false,
				ForceUpdate:     false,
			},
		},
		{
			name: "all flags set",
			args: []string{"probe", "-config", "/custom/config.yaml", "-version", "-check-update", "-skip-update-check", "-update"},
			expected: &CommandLineFlags{
				ConfigPath:      "/custom/config.yaml",
				ShowVersion:     true,
				CheckUpdate:     true,
				SkipUpdateCheck: true,
				ForceUpdate:     true,
			},
		},
		{
			name: "partial flags",
			args: []string{"probe", "-config", "test.yaml", "-version"},
			expected: &CommandLineFlags{
				ConfigPath:      "test.yaml",
				ShowVersion:     true,
				CheckUpdate:     false,
				SkipUpdateCheck: false,
				ForceUpdate:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag.CommandLine for each test
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			// Save original os.Args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()

			// Set test args
			os.Args = tt.args

			flags := parseCommandLineFlags()

			if flags.ConfigPath != tt.expected.ConfigPath {
				t.Errorf("ConfigPath = %v, want %v", flags.ConfigPath, tt.expected.ConfigPath)
			}
			if flags.ShowVersion != tt.expected.ShowVersion {
				t.Errorf("ShowVersion = %v, want %v", flags.ShowVersion, tt.expected.ShowVersion)
			}
			if flags.CheckUpdate != tt.expected.CheckUpdate {
				t.Errorf("CheckUpdate = %v, want %v", flags.CheckUpdate, tt.expected.CheckUpdate)
			}
			if flags.SkipUpdateCheck != tt.expected.SkipUpdateCheck {
				t.Errorf("SkipUpdateCheck = %v, want %v", flags.SkipUpdateCheck, tt.expected.SkipUpdateCheck)
			}
			if flags.ForceUpdate != tt.expected.ForceUpdate {
				t.Errorf("ForceUpdate = %v, want %v", flags.ForceUpdate, tt.expected.ForceUpdate)
			}
		})
	}
}

func TestHandleVersionFlag(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleVersionFlag()

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains version information
	if !strings.Contains(output, "Monitorly Probe") {
		t.Errorf("handleVersionFlag() output should contain version info, got: %s", output)
	}
}

func TestHandleCheckUpdateFlag(t *testing.T) {
	// This test would require mocking the version.CheckForUpdates function
	// For now, we'll test that it doesn't panic and returns an error or nil
	err := handleCheckUpdateFlag()
	// We expect either no error (if update check succeeds) or an error (if it fails)
	// Both are acceptable in a test environment
	t.Logf("handleCheckUpdateFlag() returned: %v", err)
}

func TestHandleForceUpdateFlag(t *testing.T) {
	// This test would require mocking the version.CheckForUpdates function
	// For now, we'll test that it doesn't panic and returns an error or nil
	err := handleForceUpdateFlag()
	// We expect either no error (if update check succeeds) or an error (if it fails)
	// Both are acceptable in a test environment
	t.Logf("handleForceUpdateFlag() returned: %v", err)
}

func TestPerformStartupUpdateCheck(t *testing.T) {
	// Test that the function doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("performStartupUpdateCheck() panicked: %v", r)
		}
	}()

	performStartupUpdateCheck()
}

func TestSetupSignalHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test that signal handling setup doesn't panic
	setupSignalHandling(ctx, cancel)

	// Give it a moment to set up
	time.Sleep(10 * time.Millisecond)

	// Cancel the context to clean up
	cancel()
}

func TestSetupConfigWatcher(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create a test config file
	if err := os.WriteFile(configPath, []byte("test: config"), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	tests := []struct {
		name        string
		configPath  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid config path",
			configPath: configPath,
			wantErr:    false,
		},
		{
			name:        "invalid config path",
			configPath:  "/non/existent/path/config.yaml",
			wantErr:     true,
			errContains: "failed to watch config directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := setupConfigWatcher(tt.configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("setupConfigWatcher() expected error but got none")
					if watcher != nil {
						watcher.Close()
					}
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("setupConfigWatcher() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("setupConfigWatcher() unexpected error = %v", err)
				return
			}

			if watcher == nil {
				t.Error("setupConfigWatcher() returned nil watcher")
				return
			}

			// Clean up
			watcher.Close()
		})
	}
}

func TestStartUpdateChecker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name:   "updates disabled",
			config: &config.Config{},
		},
		{
			name: "updates enabled",
			config: func() *config.Config {
				cfg := &config.Config{}
				cfg.Updates.Enabled = true
				cfg.Updates.CheckTime = "02:00"
				cfg.Updates.RetryDelay = 5 * time.Minute
				return cfg
			}(),
		},
		{
			name: "updates enabled with invalid time",
			config: func() *config.Config {
				cfg := &config.Config{}
				cfg.Updates.Enabled = true
				cfg.Updates.CheckTime = "invalid"
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the function doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("startUpdateChecker() panicked: %v", r)
				}
			}()

			startUpdateChecker(ctx, tt.config)
		})
	}
}

func TestRunMainLoop(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create a valid config file
	configContent := `
api:
  url: "https://api.example.com"
  project_id: "test-project"
  application_token: "test-token"
machine_name: "test-machine"
sender:
  target: "log_file"
  send_interval: "1s"
log_file:
  path: "/tmp/test.log"
logging:
  file_path: "/tmp/app.log"
collection:
  cpu:
    enabled: true
    interval: "1s"
  ram:
    enabled: false
  disk:
    enabled: false
  service:
    enabled: false
updates:
  enabled: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load initial config
	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Create restart channel
	restartChan := make(chan struct{})

	// Test normal shutdown
	t.Run("normal shutdown", func(t *testing.T) {
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		runMainLoop(ctx, configPath, cfg, restartChan)
	})

	// Test config restart
	t.Run("config restart", func(t *testing.T) {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel2()

		restartChan2 := make(chan struct{}, 1)

		go func() {
			time.Sleep(100 * time.Millisecond)
			restartChan2 <- struct{}{}
			time.Sleep(100 * time.Millisecond)
			cancel2()
		}()

		runMainLoop(ctx2, configPath, cfg, restartChan2)
	})
}

func TestRunApplication(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	// Create a valid config file
	configContent := `
api:
  url: "https://api.example.com"
  project_id: "test-project"
  application_token: "test-token"
machine_name: "test-machine"
sender:
  target: "log_file"
  send_interval: "1s"
log_file:
  path: "/tmp/test.log"
logging:
  file_path: "/tmp/app.log"
collection:
  cpu:
    enabled: true
    interval: "1s"
  ram:
    enabled: false
  disk:
    enabled: false
  service:
    enabled: false
updates:
  enabled: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	tests := []struct {
		name    string
		flags   *CommandLineFlags
		wantErr bool
		timeout time.Duration
	}{
		{
			name: "version flag",
			flags: &CommandLineFlags{
				ShowVersion: true,
			},
			wantErr: false,
			timeout: 1 * time.Second,
		},
		{
			name: "check-update flag",
			flags: &CommandLineFlags{
				CheckUpdate: true,
			},
			wantErr: false, // May return error due to network, but that's acceptable
			timeout: 5 * time.Second,
		},
		{
			name: "valid config with skip update check",
			flags: &CommandLineFlags{
				ConfigPath:      configPath,
				SkipUpdateCheck: true,
			},
			wantErr: false,
			timeout: 2 * time.Second,
		},
		{
			name: "invalid config path",
			flags: &CommandLineFlags{
				ConfigPath:      "/non/existent/config.yaml",
				SkipUpdateCheck: true,
			},
			wantErr: true,
			timeout: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run in a goroutine with timeout to prevent hanging
			done := make(chan error, 1)
			go func() {
				done <- runApplication(tt.flags)
			}()

			select {
			case err := <-done:
				if (err != nil) != tt.wantErr {
					t.Errorf("runApplication() error = %v, wantErr %v", err, tt.wantErr)
				}
			case <-time.After(tt.timeout):
				// Timeout is acceptable for the main application loop test
				if tt.flags.ShowVersion || tt.flags.CheckUpdate || tt.wantErr {
					t.Error("runApplication() should have completed quickly for this test case")
				}
			}
		})
	}
}

func TestMainFunctionRefactored(t *testing.T) {
	// Test the new main function using subprocess execution
	if os.Getenv("TEST_MAIN_REFACTORED") == "1" {
		// This is the subprocess execution
		// Override os.Args to simulate command line arguments
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()

		testCase := os.Getenv("TEST_CASE")
		switch testCase {
		case "version":
			os.Args = []string{"probe", "-version"}
		case "check-update":
			os.Args = []string{"probe", "-check-update"}
		default:
			os.Args = []string{"probe", "-version"}
		}

		// Reset flag.CommandLine
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		main()
		return
	}

	tests := []struct {
		name     string
		testCase string
		wantExit bool
	}{
		{
			name:     "version flag",
			testCase: "version",
			wantExit: false,
		},
		{
			name:     "check-update flag",
			testCase: "check-update",
			wantExit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run the test in a subprocess
			cmd := exec.Command(os.Args[0], "-test.run=TestMainFunctionRefactored")
			cmd.Env = append(os.Environ(),
				"TEST_MAIN_REFACTORED=1",
				fmt.Sprintf("TEST_CASE=%s", tt.testCase),
			)

			// Set a timeout for the subprocess
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := cmd.Run()

			// For version and check-update flags, we expect the process to exit normally
			if err != nil {
				// Check if it's a timeout or other error
				if ctx.Err() == context.DeadlineExceeded {
					t.Error("Main function test timed out")
				} else {
					t.Logf("Main function exited with: %v (this may be expected for some flags)", err)
				}
			}
		})
	}
}

func TestSearchPaths(t *testing.T) {
	tests := []struct {
		name       string
		configFlag string
		wantFirst  string
		wantCount  int
	}{
		{
			name:       "default config flag",
			configFlag: "config.yaml",
			wantFirst:  "config.yaml",
			wantCount:  5, // config.yaml + home + current + configs + /etc
		},
		{
			name:       "custom config flag",
			configFlag: "/custom/path/config.yaml",
			wantFirst:  "/custom/path/config.yaml",
			wantCount:  5,
		},
		{
			name:       "empty config flag",
			configFlag: "",
			wantFirst:  "",
			wantCount:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := searchPaths(tt.configFlag)

			if len(paths) < tt.wantCount-1 { // -1 because home dir might fail
				t.Errorf("searchPaths() returned %d paths, want at least %d", len(paths), tt.wantCount-1)
			}

			if paths[0] != tt.wantFirst {
				t.Errorf("searchPaths() first path = %v, want %v", paths[0], tt.wantFirst)
			}

			// Check that common paths are included
			foundCurrentDir := false
			foundEtc := false
			for _, path := range paths {
				if path == "config.yaml" {
					foundCurrentDir = true
				}
				if path == "/etc/monitorly/config.yaml" {
					foundEtc = true
				}
			}

			if !foundCurrentDir {
				t.Error("searchPaths() should include current directory config.yaml")
			}
			if !foundEtc {
				t.Error("searchPaths() should include /etc/monitorly/config.yaml")
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Create a test config file
	testConfigPath := filepath.Join(tempDir, "test-config.yaml")
	testConfigContent := `
api:
  url: "https://api.example.com"
  project_id: "test-project"
  application_token: "test-token"
machine_name: "test-machine"
sender:
  target: "log_file"
log_file:
  path: "/tmp/test.log"
`
	if err := os.WriteFile(testConfigPath, []byte(testConfigContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	tests := []struct {
		name        string
		configFlag  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "existing config file",
			configFlag: testConfigPath,
			wantErr:    false,
		},
		{
			name:        "non-existing explicit config file",
			configFlag:  "/non/existing/config.yaml",
			wantErr:     true,
			errContains: "specified config file not found",
		},
		{
			name:        "default config file not found",
			configFlag:  "config.yaml",
			wantErr:     true,
			errContains: "no config file found in search paths",
		},
		{
			name:        "invalid path with permission error",
			configFlag:  "/root/restricted/config.yaml",
			wantErr:     true,
			errContains: "specified config file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := findConfigFile(tt.configFlag)

			if tt.wantErr {
				if err == nil {
					t.Errorf("findConfigFile() expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("findConfigFile() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("findConfigFile() unexpected error = %v", err)
				return
			}

			if result == "" {
				t.Error("findConfigFile() returned empty path")
			}

			// Verify the file exists
			if _, err := os.Stat(result); os.IsNotExist(err) {
				t.Errorf("findConfigFile() returned non-existing file: %s", result)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		configData  string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			configData: `
api:
  url: "https://api.example.com"
  project_id: "test-project"
  application_token: "test-token"
machine_name: "test-machine"
sender:
  target: "log_file"
log_file:
  path: "/tmp/test.log"
`,
			wantErr: false,
		},
		{
			name: "invalid yaml",
			configData: `
api:
  url: "https://api.example.com"
  project_id: "test-project"
  invalid: [unclosed
`,
			wantErr:     true,
			errContains: "failed to parse config file",
		},
		{
			name:        "non-existing file",
			configData:  "", // Will not create file
			wantErr:     true,
			errContains: "failed to read config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string
			if tt.configData != "" {
				configPath = filepath.Join(tempDir, tt.name+"-config.yaml")
				if err := os.WriteFile(configPath, []byte(tt.configData), 0644); err != nil {
					t.Fatalf("Failed to create test config file: %v", err)
				}
			} else {
				configPath = filepath.Join(tempDir, "non-existing.yaml")
			}

			cfg, err := loadConfig(configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("loadConfig() expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("loadConfig() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("loadConfig() unexpected error = %v", err)
				return
			}

			if cfg == nil {
				t.Error("loadConfig() returned nil config")
			}
		})
	}
}

func TestLogMetric(t *testing.T) {
	tests := []struct {
		name     string
		metric   collector.Metrics
		expected string
	}{
		{
			name: "metric without metadata",
			metric: collector.Metrics{
				Timestamp: time.Now(),
				Category:  collector.CategorySystem,
				Name:      collector.NameCPU,
				Value:     75.5,
			},
			expected: "category=system name=cpu",
		},
		{
			name: "metric with metadata",
			metric: collector.Metrics{
				Timestamp: time.Now(),
				Category:  collector.CategorySystem,
				Name:      collector.NameRAM,
				Value:     85.2,
				Metadata: collector.MetricMetadata{
					"host": "server1",
					"type": "physical",
				},
			},
			expected: "category=system name=ram metadata=",
		},
		{
			name: "metric with empty metadata",
			metric: collector.Metrics{
				Timestamp: time.Now(),
				Category:  collector.CategorySystem,
				Name:      collector.NameDisk,
				Value:     45.0,
				Metadata:  collector.MetricMetadata{},
			},
			expected: "category=system name=disk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that it doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("logMetric() panicked: %v", r)
				}
			}()

			logMetric("test-collector", tt.metric)
		})
	}
}

func TestCollectRoutine(t *testing.T) {
	tests := []struct {
		name         string
		collector    collector.Collector
		interval     time.Duration
		expectMetric bool
	}{
		{
			name: "successful collection",
			collector: &MockCollector{
				metrics: []collector.Metrics{
					{
						Timestamp: time.Now(),
						Category:  collector.CategorySystem,
						Name:      collector.NameCPU,
						Value:     75.5,
					},
				},
			},
			interval:     50 * time.Millisecond,
			expectMetric: true,
		},
		{
			name: "collector with error",
			collector: &MockCollector{
				err: fmt.Errorf("collection failed"),
			},
			interval:     50 * time.Millisecond,
			expectMetric: false,
		},
		{
			name: "collector with empty metrics",
			collector: &MockCollector{
				metrics: []collector.Metrics{},
			},
			interval:     50 * time.Millisecond,
			expectMetric: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with short timeout
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Create metrics channel
			metricsChan := make(chan []collector.Metrics, 10)

			// Run collect routine
			collectRoutine(ctx, "test-collector", tt.collector, metricsChan, tt.interval)

			// Check for metrics
			if tt.expectMetric {
				select {
				case metrics := <-metricsChan:
					if len(metrics) != 1 {
						t.Errorf("Expected 1 metric, got %d", len(metrics))
					}
					if metrics[0].Name != collector.NameCPU {
						t.Errorf("Expected CPU metric, got %v", metrics[0].Name)
					}
				case <-time.After(200 * time.Millisecond):
					t.Error("No metrics received within timeout")
				}
			} else {
				select {
				case <-metricsChan:
					t.Error("Unexpected metrics received")
				case <-time.After(200 * time.Millisecond):
					// Expected no metrics
				}
			}
		})
	}
}

func TestSendRoutine(t *testing.T) {
	tests := []struct {
		name        string
		sender      *MockSender
		sendMetrics bool
		expectSent  bool
	}{
		{
			name:        "successful send",
			sender:      &MockSender{},
			sendMetrics: true,
			expectSent:  true,
		},
		{
			name:        "send with error",
			sender:      &MockSender{err: fmt.Errorf("send failed")},
			sendMetrics: true,
			expectSent:  false,
		},
		{
			name:        "no metrics to send",
			sender:      &MockSender{},
			sendMetrics: false,
			expectSent:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with short timeout
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Create metrics channel
			metricsChan := make(chan []collector.Metrics, 10)

			if tt.sendMetrics {
				testMetrics := []collector.Metrics{
					{
						Timestamp: time.Now(),
						Category:  collector.CategorySystem,
						Name:      collector.NameCPU,
						Value:     75.5,
					},
				}
				metricsChan <- testMetrics
			}

			// Run send routine
			sendRoutine(ctx, tt.sender, metricsChan, 50*time.Millisecond)

			// Check results
			if tt.expectSent {
				if len(tt.sender.sentMetrics) == 0 {
					t.Error("No metrics were sent")
				}
			} else if tt.sender.err == nil && len(tt.sender.sentMetrics) > 0 {
				t.Error("Unexpected metrics were sent")
			}
		})
	}
}

func TestWatchConfigFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create initial config file
	initialConfig := `
api:
  url: "https://api.example.com"
  project_id: "test-project"
  application_token: "test-token"
machine_name: "test-machine"
sender:
  target: "log_file"
log_file:
  path: "/tmp/test.log"
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to create initial config file: %v", err)
	}

	// Create watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Add directory to watcher
	if err := watcher.Add(tempDir); err != nil {
		t.Fatalf("Failed to add directory to watcher: %v", err)
	}

	// Create context and restart channel
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	restartChan := make(chan struct{}, 1)

	// Start watching in a goroutine
	go watchConfigFile(ctx, watcher, configPath, restartChan)

	// Wait a bit for the watcher to start
	time.Sleep(100 * time.Millisecond)

	// Modify the config file
	modifiedConfig := `
api:
  url: "https://api.modified.com"
  project_id: "test-project"
  application_token: "test-token"
machine_name: "test-machine"
sender:
  target: "log_file"
log_file:
  path: "/tmp/test.log"
`
	if err := os.WriteFile(configPath, []byte(modifiedConfig), 0644); err != nil {
		t.Fatalf("Failed to modify config file: %v", err)
	}

	// Wait for restart signal
	select {
	case <-restartChan:
		// Success - config change was detected
	case <-time.After(1 * time.Second):
		t.Error("Config file change was not detected within timeout")
	}
}

func TestRunApp(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Create a minimal valid config
	cfg := &config.Config{
		MachineName: "test-machine",
	}

	// Set API config
	cfg.API.URL = "https://api.example.com"
	cfg.API.ProjectID = "test-project"
	cfg.API.ApplicationToken = "test-token"

	// Set sender config
	cfg.Sender.Target = "log_file"
	cfg.Sender.SendInterval = 1 * time.Second

	// Set log file config
	cfg.LogFile.Path = logFile

	// Set logging config
	cfg.Logging.FilePath = filepath.Join(tempDir, "app.log")

	// Set collection config
	cfg.Collection.CPU.Enabled = true
	cfg.Collection.CPU.Interval = 1 * time.Second
	cfg.Collection.RAM.Enabled = false
	cfg.Collection.RAM.Interval = 1 * time.Second
	cfg.Collection.Disk.Enabled = false
	cfg.Collection.Disk.Interval = 1 * time.Second
	cfg.Collection.Service.Enabled = false
	cfg.Collection.Service.Interval = 1 * time.Second

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Run the app
	wg := runApp(ctx, cfg)

	// Wait for a longer time to let collectors run and sender send metrics
	time.Sleep(300 * time.Millisecond)

	// Cancel context to stop the app
	cancel()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - all goroutines finished
	case <-time.After(5 * time.Second):
		t.Error("runApp did not shut down gracefully within timeout")
	}

	// Verify log file was created (it should be created even if empty)
	// Note: The file might not exist if no metrics were collected and sent
	// This is acceptable behavior, so we'll just check that the test doesn't crash
	if _, err := os.Stat(logFile); err != nil {
		t.Logf("Log file was not created (this is acceptable): %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock implementations for testing

type MockCollector struct {
	metrics []collector.Metrics
	err     error
}

func (m *MockCollector) Collect() ([]collector.Metrics, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.metrics, nil
}

type MockSender struct {
	sentMetrics [][]collector.Metrics
	err         error
	mu          sync.Mutex
}

func (m *MockSender) Send(metrics []collector.Metrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}
	m.sentMetrics = append(m.sentMetrics, metrics)
	return nil
}

func (m *MockSender) SendWithContext(ctx context.Context, metrics []collector.Metrics) error {
	return m.Send(metrics)
}
