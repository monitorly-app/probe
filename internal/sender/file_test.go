package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

func TestNewFileLogger(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     *FileLogger
	}{
		{
			name:     "valid file path",
			filePath: "test.log",
			want:     &FileLogger{filePath: "test.log"},
		},
		{
			name:     "nested path",
			filePath: "logs/nested/test.log",
			want:     &FileLogger{filePath: "logs/nested/test.log"},
		},
		{
			name:     "empty path",
			filePath: "",
			want:     &FileLogger{filePath: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewFileLogger(tt.filePath)
			if got.filePath != tt.want.filePath {
				t.Errorf("NewFileLogger() filePath = %v, want %v", got.filePath, tt.want.filePath)
			}
		})
	}
}

func TestFileLogger_Send(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	logger := NewFileLogger(logFile)

	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	err := logger.Send(metrics)
	if err != nil {
		t.Errorf("FileLogger.Send() error = %v", err)
		return
	}

	// Verify the file was created and contains valid JSON
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
		return
	}

	// Verify file contains valid JSON array
	var decodedMetrics []collector.Metrics
	if err := json.Unmarshal(content, &decodedMetrics); err != nil {
		t.Errorf("FileLogger.Send() wrote invalid JSON: %v", err)
		return
	}

	// Check if the number of metrics matches
	if len(decodedMetrics) != 1 {
		t.Errorf("FileLogger.Send() wrote %d metrics, want 1", len(decodedMetrics))
		return
	}
}

func TestFileLogger_SendWithContext(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		ctx         context.Context
		metrics     []collector.Metrics
		filePath    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid context and metrics",
			ctx:      context.Background(),
			metrics:  []collector.Metrics{{Timestamp: time.Now(), Category: collector.CategorySystem, Name: collector.NameCPU, Value: 75.5}},
			filePath: filepath.Join(tempDir, "valid.log"),
			wantErr:  false,
		},
		{
			name:        "cancelled context",
			ctx:         func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			metrics:     []collector.Metrics{{Timestamp: time.Now(), Category: collector.CategorySystem, Name: collector.NameCPU, Value: 75.5}},
			filePath:    filepath.Join(tempDir, "cancelled.log"),
			wantErr:     true,
			errContains: "context cancelled",
		},
		{
			name:        "invalid file path",
			ctx:         context.Background(),
			metrics:     []collector.Metrics{{Timestamp: time.Now(), Category: collector.CategorySystem, Name: collector.NameCPU, Value: 75.5}},
			filePath:    "\x00invalid",
			wantErr:     true,
			errContains: "invalid argument",
		},
		{
			name:     "empty metrics",
			ctx:      context.Background(),
			metrics:  []collector.Metrics{},
			filePath: filepath.Join(tempDir, "empty.log"),
			wantErr:  false,
		},
		{
			name:     "nil metrics",
			ctx:      context.Background(),
			metrics:  nil,
			filePath: filepath.Join(tempDir, "nil.log"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewFileLogger(tt.filePath)
			err := logger.SendWithContext(tt.ctx, tt.metrics)

			if (err != nil) != tt.wantErr {
				t.Errorf("FileLogger.SendWithContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("FileLogger.SendWithContext() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if !tt.wantErr {
				// Verify the file was created
				if _, err := os.Stat(tt.filePath); os.IsNotExist(err) {
					t.Errorf("FileLogger.SendWithContext() did not create file at %s", tt.filePath)
					return
				}

				// Verify file contains valid JSON
				content, err := os.ReadFile(tt.filePath)
				if err != nil {
					t.Errorf("Failed to read log file: %v", err)
					return
				}

				// Verify file contains valid JSON array
				var decodedMetrics []collector.Metrics
				if err := json.Unmarshal(content, &decodedMetrics); err != nil {
					t.Errorf("FileLogger.SendWithContext() wrote invalid JSON: %v", err)
					return
				}

				// Check if the number of metrics matches
				if len(decodedMetrics) != len(tt.metrics) {
					t.Errorf("FileLogger.SendWithContext() wrote %d metrics, want %d", len(decodedMetrics), len(tt.metrics))
					return
				}

				// For empty metrics, verify it's an empty array with a newline
				if len(tt.metrics) == 0 && string(content) != "[]\n" {
					t.Errorf("FileLogger.SendWithContext() wrote %q for empty metrics, want %q", string(content), "[]\n")
					return
				}
			}
		})
	}
}

// BenchmarkFileLogger_Send benchmarks the Send function
func BenchmarkFileLogger_Send(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench.log")
	logger := NewFileLogger(logFile)

	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := logger.Send(metrics); err != nil {
			b.Fatalf("FileLogger.Send() failed: %v", err)
		}
	}
}

// BenchmarkFileLogger_SendWithContext benchmarks the SendWithContext function
func BenchmarkFileLogger_SendWithContext(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench.log")
	logger := NewFileLogger(logFile)
	ctx := context.Background()

	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := logger.SendWithContext(ctx, metrics); err != nil {
			b.Fatalf("FileLogger.SendWithContext() failed: %v", err)
		}
	}
}

// TestFileLogger_ConcurrentAccess tests concurrent access to the same log file
func TestFileLogger_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "concurrent.log")
	logger := NewFileLogger(logFile)

	// Create multiple metrics sets
	metricsSets := make([][]collector.Metrics, 10)
	for i := range metricsSets {
		metricsSets[i] = []collector.Metrics{
			{
				Timestamp: time.Now(),
				Category:  collector.CategorySystem,
				Name:      collector.NameCPU,
				Value:     float64(i),
			},
		}
	}

	// Send metrics concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, len(metricsSets))

	for _, metrics := range metricsSets {
		wg.Add(1)
		go func(m []collector.Metrics) {
			defer wg.Done()
			if err := logger.Send(m); err != nil {
				errChan <- err
			}
		}(metrics)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("concurrent Send() error: %v", err)
	}

	// Verify file contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Split content into lines and decode each line
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != len(metricsSets) {
		t.Errorf("Expected %d lines in log file, got %d", len(metricsSets), len(lines))
	}

	for _, line := range lines {
		var metrics []collector.Metrics
		if err := json.Unmarshal([]byte(line), &metrics); err != nil {
			t.Errorf("Failed to decode line %q: %v", line, err)
		}
	}
}

// TestFileLogger_Permissions tests file and directory permissions
func TestFileLogger_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permissions test on Windows")
	}

	tempDir := t.TempDir()
	tests := []struct {
		name        string
		setup       func(string) string
		wantErr     bool
		errContains string
	}{
		{
			name: "read-only directory",
			setup: func(dir string) string {
				readOnlyDir := filepath.Join(dir, "readonly")
				if err := os.Mkdir(readOnlyDir, 0500); err != nil {
					t.Fatalf("Failed to create read-only directory: %v", err)
				}
				return filepath.Join(readOnlyDir, "test.log")
			},
			wantErr:     true,
			errContains: "permission denied",
		},
		{
			name: "read-only file",
			setup: func(dir string) string {
				filePath := filepath.Join(dir, "readonly.log")
				if err := os.WriteFile(filePath, []byte(""), 0400); err != nil {
					t.Fatalf("Failed to create read-only file: %v", err)
				}
				return filePath
			},
			wantErr:     true,
			errContains: "permission denied",
		},
		{
			name: "nested directory creation",
			setup: func(dir string) string {
				return filepath.Join(dir, "nested", "dirs", "test.log")
			},
			wantErr: false,
		},
	}

	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setup(tempDir)
			logger := NewFileLogger(filePath)
			err := logger.Send(metrics)

			if (err != nil) != tt.wantErr {
				t.Errorf("FileLogger.Send() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("FileLogger.Send() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if !tt.wantErr {
				// Verify file was created with correct permissions
				info, err := os.Stat(filePath)
				if err != nil {
					t.Errorf("Failed to stat file: %v", err)
					return
				}

				// Check file permissions (0644)
				if info.Mode().Perm() != 0644 {
					t.Errorf("File permissions = %v, want %v", info.Mode().Perm(), 0644)
				}
			}
		})
	}
}

// TestFileLogger_AppendBehavior tests the file append behavior
func TestFileLogger_AppendBehavior(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "append.log")
	logger := NewFileLogger(logFile)

	// Initial metrics
	metrics1 := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	// Second set of metrics
	metrics2 := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameRAM,
			Value:     80.0,
		},
	}

	// Write first set of metrics
	if err := logger.Send(metrics1); err != nil {
		t.Fatalf("First Send() failed: %v", err)
	}

	// Get file size after first write
	info1, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Failed to stat file after first write: %v", err)
	}
	size1 := info1.Size()

	// Write second set of metrics
	if err := logger.Send(metrics2); err != nil {
		t.Fatalf("Second Send() failed: %v", err)
	}

	// Get file size after second write
	info2, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Failed to stat file after second write: %v", err)
	}
	size2 := info2.Size()

	// Verify file grew in size
	if size2 <= size1 {
		t.Errorf("File size did not increase after append (before: %d, after: %d)", size1, size2)
	}

	// Read file contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Split content into lines
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines in log file, got %d", len(lines))
	}

	// Verify each line contains valid JSON
	for i, line := range lines {
		var metrics []collector.Metrics
		if err := json.Unmarshal([]byte(line), &metrics); err != nil {
			t.Errorf("Line %d: failed to decode JSON: %v", i+1, err)
			continue
		}

		if len(metrics) != 1 {
			t.Errorf("Line %d: expected 1 metric, got %d", i+1, len(metrics))
			continue
		}

		// Verify metric values
		metric := metrics[0]
		if i == 0 {
			if metric.Name != collector.NameCPU || metric.Value != 75.5 {
				t.Errorf("Line 1: unexpected metric: got %+v", metric)
			}
		} else {
			if metric.Name != collector.NameRAM || metric.Value != 80.0 {
				t.Errorf("Line 2: unexpected metric: got %+v", metric)
			}
		}
	}
}

// TestFileLogger_ConcurrentAccessWithModes tests concurrent access to the same log file with different file modes
func TestFileLogger_ConcurrentAccessWithModes(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "concurrent_modes.log")

	// Create a single logger instance to be used by all goroutines
	logger := NewFileLogger(logFile)

	// Create different metrics sets
	metricsSets := [][]collector.Metrics{
		{
			{
				Timestamp: time.Now(),
				Category:  collector.CategorySystem,
				Name:      collector.NameCPU,
				Value:     75.5,
			},
		},
		{
			{
				Timestamp: time.Now(),
				Category:  collector.CategorySystem,
				Name:      collector.NameRAM,
				Value:     80.0,
			},
		},
		{
			{
				Timestamp: time.Now(),
				Category:  collector.CategorySystem,
				Name:      collector.NameDisk,
				Value:     90.0,
			},
		},
	}

	// Send metrics concurrently with different contexts
	var wg sync.WaitGroup
	errChan := make(chan error, len(metricsSets)*3) // 3 goroutines per metric set

	for i, metrics := range metricsSets {
		for j := 0; j < 3; j++ { // Create 3 goroutines per metric set
			wg.Add(1)
			go func(m []collector.Metrics, idx int) {
				defer wg.Done()

				// Create different contexts for each goroutine
				var ctx context.Context
				var cancel context.CancelFunc

				switch idx % 3 {
				case 0:
					// Normal context
					ctx = context.Background()
					cancel = func() {}
				case 1:
					// Context with timeout
					ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
				case 2:
					// Context with deadline
					ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
				}
				defer cancel()

				if err := logger.SendWithContext(ctx, m); err != nil {
					errChan <- fmt.Errorf("goroutine %d: %w", idx, err)
				}
			}(metrics, i*3+j)
		}
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}
	if len(errors) > 0 {
		t.Errorf("Got %d errors during concurrent access: %v", len(errors), errors)
	}

	// Verify file contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Split content into lines and decode each line
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	expectedLines := 3 * len(metricsSets) // 3 goroutines per metric set
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines in log file, got %d", expectedLines, len(lines))
	}

	// Count occurrences of each metric type
	counts := make(map[collector.MetricName]int)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue // Skip empty lines
		}
		var metrics []collector.Metrics
		if err := json.Unmarshal([]byte(line), &metrics); err != nil {
			t.Errorf("Failed to decode line %q: %v", line, err)
			continue
		}
		// Each line should contain exactly one metric (as an array with one element)
		if len(metrics) != 1 {
			t.Errorf("Expected 1 metric per line, got %d", len(metrics))
			continue
		}
		counts[metrics[0].Name]++
	}

	// Verify we got the expected number of each metric type
	expectedCount := 3 // 3 goroutines per metric type
	for _, name := range []collector.MetricName{collector.NameCPU, collector.NameRAM, collector.NameDisk} {
		if count := counts[name]; count != expectedCount {
			t.Errorf("Expected %d occurrences of metric %s, got %d", expectedCount, name, count)
		}
	}
}

// TestFileLogger_FileRotation tests file rotation scenarios
func TestFileLogger_FileRotation(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "rotating.log")
	logger := NewFileLogger(logFile)

	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	// Write initial metrics
	if err := logger.Send(metrics); err != nil {
		t.Fatalf("Initial Send() failed: %v", err)
	}

	// Simulate file rotation: rename the current file
	rotatedFile := logFile + ".1"
	if err := os.Rename(logFile, rotatedFile); err != nil {
		t.Fatalf("Failed to rotate file: %v", err)
	}

	// Write more metrics after rotation
	metrics[0].Value = 80.0
	if err := logger.Send(metrics); err != nil {
		t.Fatalf("Send() after rotation failed: %v", err)
	}

	// Verify both files exist
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("New log file was not created after rotation")
	}
	if _, err := os.Stat(rotatedFile); os.IsNotExist(err) {
		t.Error("Rotated log file does not exist")
	}

	// Verify contents of both files
	checkFileContents := func(path string, expectedValue float64) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", path, err)
			return
		}

		var metrics []collector.Metrics
		if err := json.Unmarshal(content, &metrics); err != nil {
			t.Errorf("Failed to decode metrics from %s: %v", path, err)
			return
		}

		if len(metrics) != 1 {
			t.Errorf("Expected 1 metric in %s, got %d", path, len(metrics))
			return
		}

		if metrics[0].Value != expectedValue {
			t.Errorf("Expected value %.1f in %s, got %.1f", expectedValue, path, metrics[0].Value)
		}
	}

	checkFileContents(rotatedFile, 75.5)
	checkFileContents(logFile, 80.0)

	// Test writing to a deleted file
	if err := os.Remove(logFile); err != nil {
		t.Fatalf("Failed to remove log file: %v", err)
	}

	// Write should recreate the file
	metrics[0].Value = 85.0
	if err := logger.Send(metrics); err != nil {
		t.Fatalf("Send() after file deletion failed: %v", err)
	}

	checkFileContents(logFile, 85.0)
}
