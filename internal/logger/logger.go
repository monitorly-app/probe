package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// LoggerInterface defines the interface for logging operations
type LoggerInterface interface {
	Printf(format string, v ...interface{})
	Fatalf(format string, v ...interface{})
	Close() error
}

var (
	// Default logger instance
	defaultLogger LoggerInterface
	once          sync.Once
)

// Logger represents a logger that writes to both stdout and a file
type Logger struct {
	stdLog  *log.Logger
	fileLog *log.Logger
	logFile *os.File
}

// Initialize sets up the default logger with the specified log file path
func Initialize(logFilePath string) error {
	var err error
	once.Do(func() {
		logger, initErr := NewLogger(logFilePath)
		if initErr != nil {
			err = initErr
			return
		}
		defaultLogger = logger
	})
	return err
}

// NewLogger creates a new instance of Logger with the specified log file path
func NewLogger(logFilePath string) (*Logger, error) {
	// Create directory for log file if it doesn't exist
	dir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for log file: %w", err)
	}

	// Open log file
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer to write to both stdout and log file
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Create logger with timestamp, file, and line number
	stdLogger := log.New(multiWriter, "", log.Ldate|log.Ltime)

	// Create file-only logger for the full log format
	fileLogger := log.New(logFile, "", log.Ldate|log.Ltime)

	logger := &Logger{
		stdLog:  stdLogger,
		fileLog: fileLogger,
		logFile: logFile,
	}

	// Log initialization
	logger.Printf("Logging initialized to file: %s", logFilePath)

	return logger, nil
}

// Close closes the log file
func Close() error {
	if defaultLogger != nil {
		return defaultLogger.Close()
	}
	return nil
}

// Close closes the log file for a specific logger instance
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// Printf logs a formatted message to both stdout and the log file
func Printf(format string, v ...interface{}) {
	if defaultLogger == nil {
		// Fall back to standard logger if not initialized
		log.Printf(format, v...)
		return
	}

	defaultLogger.Printf(format, v...)
}

// Printf logs a formatted message for a specific logger instance
func (l *Logger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.stdLog.Print(msg)
}

// Fatalf logs a formatted message and exits the program
func Fatalf(format string, v ...interface{}) {
	if defaultLogger == nil {
		// Fall back to standard logger if not initialized
		log.Fatalf(format, v...)
		return
	}

	defaultLogger.Fatalf(format, v...)
}

// Fatalf logs a formatted message and exits the program for a specific logger instance
func (l *Logger) Fatalf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.stdLog.Print(msg)
	os.Exit(1)
}
