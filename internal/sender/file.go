package sender

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

type FileLogger struct {
	filePath string
}

func NewFileLogger(filePath string) *FileLogger {
	return &FileLogger{
		filePath: filePath,
	}
}

func (f *FileLogger) Send(metrics []collector.Metrics) error {
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
