package sender

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
