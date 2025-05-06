package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

// FileLogger implements the Sender interface for logging metrics to a file
type FileLogger struct {
	filePath string
}

// NewFileLogger creates a new instance of FileLogger
func NewFileLogger(filePath string) *FileLogger {
	return &FileLogger{
		filePath: filePath,
	}
}

// Send logs metrics to a file
func (f *FileLogger) Send(metrics []collector.Metrics) error {
	return f.SendWithContext(context.Background(), metrics)
}

// SendWithContext logs metrics to a file with context support
func (f *FileLogger) SendWithContext(ctx context.Context, metrics []collector.Metrics) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
		// Continue processing
	}

	// Ensure directory exists
	dir := filepath.Dir(f.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for log file: %w", err)
	}

	// Open file in append mode or create if it doesn't exist
	file, err := os.OpenFile(f.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// For each metric, format and write to the file
	for _, metric := range metrics {
		// Check for context cancellation periodically
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while writing metrics: %w", ctx.Err())
		default:
			// Continue processing
		}

		// Add a timestamp for when the log entry was written
		entry := struct {
			LogTime time.Time         `json:"log_time"`
			Metric  collector.Metrics `json:"metric"`
		}{
			LogTime: time.Now(),
			Metric:  metric,
		}

		// Marshal to JSON
		jsonData, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal metric: %w", err)
		}

		// Write to file with a newline
		if _, err := file.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write to log file: %w", err)
		}
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline to log file: %w", err)
		}
	}

	return nil
}
