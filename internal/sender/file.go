package sender

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/serialization"
)

// FileLogger implements the Sender interface for logging metrics to a file
type FileLogger struct {
	filePath string
	mu       sync.Mutex
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

	f.mu.Lock()
	defer f.mu.Unlock()

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

	// Use the serialization package to write metrics (without indentation)
	if err := serialization.WriteMetricsTo(file, metrics, false); err != nil {
		return fmt.Errorf("failed to write metrics to log file: %w", err)
	}

	return nil
}
