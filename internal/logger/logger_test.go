package logger

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// MockLogger implements LoggerInterface for testing
type MockLogger struct {
	messages []string
	fatals   []string
	closed   bool
	mu       sync.Mutex
}

func (m *MockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
}

func (m *MockLogger) Fatalf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fatals = append(m.fatals, fmt.Sprintf(format, v...))
}

func (m *MockLogger) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockLogger) GetMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.messages...)
}

func (m *MockLogger) GetFatals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.fatals...)
}

func (m *MockLogger) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name        string
		logFilePath string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid log file path",
			logFilePath: filepath.Join(t.TempDir(), "test.log"),
			wantErr:     false,
		},
		{
			name:        "nested directory path",
			logFilePath: filepath.Join(t.TempDir(), "logs", "nested", "test.log"),
			wantErr:     false,
		},
		{
			name:        "invalid path with null character",
			logFilePath: "invalid\x00path",
			wantErr:     true,
			errContains: "invalid argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.logFilePath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewLogger() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewLogger() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NewLogger() unexpected error = %v", err)
				return
			}

			if logger == nil {
				t.Errorf("NewLogger() returned nil logger")
				return
			}

			// Verify the log file was created
			if _, err := os.Stat(tt.logFilePath); os.IsNotExist(err) {
				t.Errorf("NewLogger() did not create log file at %s", tt.logFilePath)
			}

			// Clean up
			logger.Close()
		})
	}
}

func TestLogger_Printf(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	logger, err := NewLogger(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	tests := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{
			name:   "simple message",
			format: "test message",
			args:   nil,
			want:   "test message",
		},
		{
			name:   "formatted message",
			format: "user %s has %d items",
			args:   []interface{}{"john", 5},
			want:   "user john has 5 items",
		},
		{
			name:   "empty message",
			format: "",
			args:   nil,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture output
			var buf bytes.Buffer
			multiWriter := io.MultiWriter(&buf, logger.logFile)
			logger.stdLog = log.New(multiWriter, "", log.Ldate|log.Ltime)

			// Call Printf
			logger.Printf(tt.format, tt.args...)

			// Get the output
			output := buf.String()

			// Check if the output contains the expected message
			if !strings.Contains(output, tt.want) {
				t.Errorf("Logger.Printf() output = %q, want to contain %q", output, tt.want)
			}

			// Verify log file contains the message
			content, err := os.ReadFile(logFile)
			if err != nil {
				t.Errorf("Failed to read log file: %v", err)
			}

			if !strings.Contains(string(content), tt.want) {
				t.Errorf("Log file content = %q, want to contain %q", string(content), tt.want)
			}
		})
	}
}

func TestLogger_Close(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	logger, err := NewLogger(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test closing
	err = logger.Close()
	if err != nil {
		t.Errorf("Logger.Close() error = %v, want nil", err)
	}

	// Test closing again (should not error)
	err = logger.Close()
	if err != nil {
		t.Errorf("Logger.Close() second call error = %v, want nil", err)
	}
}

func TestInitialize(t *testing.T) {
	// Reset global state
	defaultLogger = nil
	once = sync.Once{}

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name        string
		logFilePath string
		wantErr     bool
	}{
		{
			name:        "valid initialization",
			logFilePath: logFile,
			wantErr:     false,
		},
		{
			name:        "invalid path",
			logFilePath: "invalid\x00path",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset for each test
			defaultLogger = nil
			once = sync.Once{}

			err := Initialize(tt.logFilePath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Initialize() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Initialize() unexpected error = %v", err)
				return
			}

			if defaultLogger == nil {
				t.Errorf("Initialize() did not set defaultLogger")
			}

			// Test that second call doesn't reinitialize
			oldLogger := defaultLogger
			err = Initialize(tt.logFilePath)
			if err != nil {
				t.Errorf("Initialize() second call error = %v", err)
			}
			if defaultLogger != oldLogger {
				t.Errorf("Initialize() second call changed defaultLogger")
			}

			// Clean up
			Close()
		})
	}
}

func TestGetDefaultLogger(t *testing.T) {
	// Reset global state
	defaultLogger = nil
	once = sync.Once{}

	// Test when no logger is set
	logger := GetDefaultLogger()
	if logger != nil {
		t.Errorf("GetDefaultLogger() = %v, want nil when not initialized", logger)
	}

	// Test after setting a logger
	mockLogger := &MockLogger{}
	SetDefaultLogger(mockLogger)

	logger = GetDefaultLogger()
	if logger != mockLogger {
		t.Errorf("GetDefaultLogger() = %v, want %v", logger, mockLogger)
	}
}

func TestSetDefaultLogger(t *testing.T) {
	// Reset global state
	defaultLogger = nil

	mockLogger := &MockLogger{}
	SetDefaultLogger(mockLogger)

	if defaultLogger != mockLogger {
		t.Errorf("SetDefaultLogger() did not set defaultLogger correctly")
	}

	// Test setting to nil
	SetDefaultLogger(nil)
	if defaultLogger != nil {
		t.Errorf("SetDefaultLogger(nil) did not set defaultLogger to nil")
	}
}

func TestPrintf(t *testing.T) {
	tests := []struct {
		name           string
		setupLogger    func() LoggerInterface
		format         string
		args           []interface{}
		expectedMsg    string
		shouldFallback bool
	}{
		{
			name: "with default logger",
			setupLogger: func() LoggerInterface {
				mock := &MockLogger{}
				return mock
			},
			format:      "test message %s",
			args:        []interface{}{"arg"},
			expectedMsg: "test message arg",
		},
		{
			name: "without default logger (fallback)",
			setupLogger: func() LoggerInterface {
				return nil
			},
			format:         "fallback message",
			args:           nil,
			shouldFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			originalLogger := defaultLogger
			defer func() { defaultLogger = originalLogger }()

			mockLogger := tt.setupLogger()
			defaultLogger = mockLogger

			if tt.shouldFallback {
				// For fallback test, we can't easily test the standard log output
				// but we can verify it doesn't panic
				Printf(tt.format, tt.args...)
				return
			}

			// Test with mock logger
			Printf(tt.format, tt.args...)

			mock := mockLogger.(*MockLogger)
			messages := mock.GetMessages()
			if len(messages) != 1 {
				t.Errorf("Printf() logged %d messages, want 1", len(messages))
				return
			}

			if messages[0] != tt.expectedMsg {
				t.Errorf("Printf() logged %v, want %v", messages[0], tt.expectedMsg)
			}
		})
	}
}

func TestFatalf(t *testing.T) {
	tests := []struct {
		name           string
		setupLogger    func() LoggerInterface
		format         string
		args           []interface{}
		expectedMsg    string
		shouldFallback bool
	}{
		{
			name: "with default logger",
			setupLogger: func() LoggerInterface {
				mock := &MockLogger{}
				return mock
			},
			format:      "fatal error %s",
			args:        []interface{}{"occurred"},
			expectedMsg: "fatal error occurred",
		},
		{
			name: "without default logger (fallback)",
			setupLogger: func() LoggerInterface {
				return nil
			},
			format:         "fallback fatal",
			args:           nil,
			shouldFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			originalLogger := defaultLogger
			defer func() { defaultLogger = originalLogger }()

			mockLogger := tt.setupLogger()
			defaultLogger = mockLogger

			if tt.shouldFallback {
				// For fallback test, we expect it to call log.Fatalf which would exit
				// We can't test this easily without subprocess, so we skip
				t.Skip("Cannot test fallback Fatalf without subprocess")
				return
			}

			// Test with mock logger
			Fatalf(tt.format, tt.args...)

			mock := mockLogger.(*MockLogger)
			fatals := mock.GetFatals()
			if len(fatals) != 1 {
				t.Errorf("Fatalf() logged %d fatal messages, want 1", len(fatals))
				return
			}

			if fatals[0] != tt.expectedMsg {
				t.Errorf("Fatalf() logged %v, want %v", fatals[0], tt.expectedMsg)
			}
		})
	}
}

func TestClose(t *testing.T) {
	tests := []struct {
		name        string
		setupLogger func() LoggerInterface
		wantErr     bool
	}{
		{
			name: "with default logger",
			setupLogger: func() LoggerInterface {
				return &MockLogger{}
			},
			wantErr: false,
		},
		{
			name: "without default logger",
			setupLogger: func() LoggerInterface {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			originalLogger := defaultLogger
			defer func() { defaultLogger = originalLogger }()

			defaultLogger = tt.setupLogger()

			err := Close()

			if tt.wantErr && err == nil {
				t.Errorf("Close() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Close() unexpected error = %v", err)
			}

			// If we had a mock logger, verify it was closed
			if mock, ok := defaultLogger.(*MockLogger); ok {
				if !mock.IsClosed() {
					t.Errorf("Close() did not close the mock logger")
				}
			}
		})
	}
}
