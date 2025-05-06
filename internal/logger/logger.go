package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	// Default logger instance
	defaultLogger *Logger
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
		err = initializeLogger(logFilePath)
	})
	return err
}

func initializeLogger(logFilePath string) error {
	// Create directory for log file if it doesn't exist
	dir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for log file: %w", err)
	}

	// Open log file
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer to write to both stdout and log file
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Create logger with timestamp, file, and line number
	stdLogger := log.New(multiWriter, "", log.Ldate|log.Ltime)

	// Create file-only logger for the full log format
	fileLogger := log.New(logFile, "", log.Ldate|log.Ltime)

	defaultLogger = &Logger{
		stdLog:  stdLogger,
		fileLog: fileLogger,
		logFile: logFile,
	}

	// Log initialization
	Printf("Logging initialized to file: %s", logFilePath)

	return nil
}

// Close closes the log file
func Close() error {
	if defaultLogger != nil && defaultLogger.logFile != nil {
		return defaultLogger.logFile.Close()
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

	msg := fmt.Sprintf(format, v...)
	defaultLogger.stdLog.Print(msg)
}

// Fatalf logs a formatted message and exits the program
func Fatalf(format string, v ...interface{}) {
	if defaultLogger == nil {
		// Fall back to standard logger if not initialized
		log.Fatalf(format, v...)
		return
	}

	msg := fmt.Sprintf(format, v...)
	defaultLogger.stdLog.Print(msg)
	os.Exit(1)
}
