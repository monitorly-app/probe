package sender

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/logger"
)

// mockLogger implements logger.LoggerInterface for testing
type mockLogger struct {
	buffer bytes.Buffer
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.buffer.WriteString(fmt.Sprintf(format+"\n", v...))
}

func (m *mockLogger) Fatalf(format string, v ...interface{}) {
	m.buffer.WriteString(fmt.Sprintf("FATAL: "+format+"\n", v...))
	// Note: Don't panic during tests, just log the fatal message for verification
}

func (m *mockLogger) Close() error {
	return nil
}

// APIPayload represents the structure of the API payload
type APIPayload struct {
	MachineName *string             `json:"machine_name,omitempty"`
	Metrics     []collector.Metrics `json:"metrics,omitempty"`
	Encrypted   *bool               `json:"encrypted,omitempty"`
	Compressed  *bool               `json:"compressed,omitempty"`
	Data        *string             `json:"data,omitempty"`
}

// decompressGzip decompresses gzipped data
func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzipped data: %w", err)
	}

	return decompressed, nil
}

// mockCompressWriter implements the compressWriter interface for testing
type mockCompressWriter struct {
	writeErr error
	closeErr error
	buf      *bytes.Buffer
}

func (m *mockCompressWriter) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.buf.Write(p)
}

func (m *mockCompressWriter) Close() error {
	return m.closeErr
}

func (m *mockCompressWriter) Bytes() []byte {
	return m.buf.Bytes()
}

// Test the Send method directly
func TestAPISender_Send(t *testing.T) {
	// Setup mock logger
	ml := &mockLogger{}
	originalLogger := logger.GetDefaultLogger()
	logger.SetDefaultLogger(ml)
	defer logger.SetDefaultLogger(originalLogger)

	tests := []struct {
		name          string
		setupServer   func() (*httptest.Server, *int)
		metrics       []collector.Metrics
		encryptionKey string
		expectedError string
		expectedCalls int
		expectedLog   string
	}{
		{
			name: "successful unencrypted request",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusOK)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "",
			expectedCalls: 1,
		},
		{
			name: "successful unencrypted large request with compression",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusOK)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "",
			expectedCalls: 1,
		},
		{
			name: "successful encrypted small request",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusOK)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "12345678901234567890123456789012",
			expectedError: "",
			expectedCalls: 1,
		},
		{
			name: "successful encrypted large request with compression",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusOK)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "12345678901234567890123456789012",
			expectedError: "",
			expectedCalls: 1,
		},
		{
			name: "encryption not available fallback with compression",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if calls == 1 {
						w.WriteHeader(http.StatusPreconditionFailed)
					} else {
						w.WriteHeader(http.StatusOK)
					}
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "12345678901234567890123456789012",
			expectedError: "",
			expectedCalls: 2,
			expectedLog:   "Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission.",
		},
		{
			name: "server error",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusInternalServerError)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "API request failed with status 500",
			expectedCalls: 1,
		},
		{
			name: "not found error (404)",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusNotFound)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "FATAL: API request failed with status 404 - Organization or server not found",
			expectedCalls: 1,
		},
		{
			name: "unauthorized error (401)",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusUnauthorized)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "FATAL: API request failed with status 401 - Invalid application token",
			expectedCalls: 1,
		},
		{
			name: "invalid encryption key",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusBadRequest)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "invalid-key-length",
			expectedError: "invalid encryption key: encryption key must be exactly 32 bytes long",
			expectedCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			ml.buffer.Reset()

			server, calls := tt.setupServer()
			defer server.Close()

			restartChan := make(chan struct{}, 1)
			sender := NewAPISender(
				server.URL,
				"test-project",
				"test-server",
				"test-token",
				"test-machine",
				tt.encryptionKey,
				"", // no config path needed
				restartChan,
			)

			err := sender.Send(tt.metrics)
			if (err != nil) != (tt.expectedError != "") {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.expectedError != "")
			}

			if err != nil && !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Error())
			}

			if *calls != tt.expectedCalls {
				t.Errorf("expected %d API calls, got %d", tt.expectedCalls, *calls)
			}

			// Check for expected log message
			if tt.expectedLog != "" && !strings.Contains(ml.buffer.String(), tt.expectedLog) {
				t.Errorf("expected log message %q, got %q", tt.expectedLog, ml.buffer.String())
			}
		})
	}
}

// Test compression function directly
func Test_compressData(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid data",
			data:    []byte("test data for compression"),
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: false,
		},
		{
			name:    "nil data",
			data:    nil,
			wantErr: false,
		},
		{
			name:    "large data",
			data:    bytes.Repeat([]byte("a"), 1000),
			wantErr: false,
		},
		{
			name:    "unicode data",
			data:    []byte("测试数据压缩"),
			wantErr: false,
		},
		{
			name:    "binary data",
			data:    []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
			wantErr: false,
		},
		{
			name:    "json data",
			data:    []byte(`{"test": "data", "array": [1,2,3], "nested": {"key": "value"}}`),
			wantErr: false,
		},
		{
			name:    "mixed content",
			data:    []byte("text with numbers 12345 and special chars !@#$%^&*()"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := compressData(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("compressData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify compression actually happened
				if len(compressed) > 0 {
					// Check gzip header magic numbers
					if len(compressed) < 2 || compressed[0] != 0x1f || compressed[1] != 0x8b {
						t.Error("Output is not in gzip format")
					}
				}

				// Verify we can decompress the data
				decompressed, err := decompressGzip(compressed)
				if err != nil {
					t.Errorf("Failed to decompress data: %v", err)
					return
				}

				// Verify the decompressed data matches the original
				if !bytes.Equal(decompressed, tt.data) {
					t.Errorf("Decompressed data doesn't match original.\nGot: %v\nWant: %v", decompressed, tt.data)
				}

				// For non-empty input, verify some compression happened
				if len(tt.data) > 20 && len(compressed) >= len(tt.data) {
					t.Logf("Warning: No compression achieved for %s (original: %d bytes, compressed: %d bytes)",
						tt.name, len(tt.data), len(compressed))
				}
			}
		})
	}
}

// Test error cases for compressData
func Test_compressData_errors(t *testing.T) {
	tests := []struct {
		name          string
		writerFactory writerFactory
		data          []byte
		expectedError string
	}{
		{
			name: "write error",
			writerFactory: func(w io.Writer) compressWriter {
				return &mockCompressWriter{
					writeErr: errors.New("mock write error"),
					buf:      &bytes.Buffer{},
				}
			},
			data:          []byte("test data"),
			expectedError: "failed to write to gzip writer",
		},
		{
			name: "close error",
			writerFactory: func(w io.Writer) compressWriter {
				return &mockCompressWriter{
					closeErr: errors.New("mock close error"),
					buf:      &bytes.Buffer{},
				}
			},
			data:          []byte("test data"),
			expectedError: "failed to close gzip writer",
		},
		{
			name: "write and close error",
			writerFactory: func(w io.Writer) compressWriter {
				return &mockCompressWriter{
					writeErr: errors.New("mock write error"),
					closeErr: errors.New("mock close error"),
					buf:      &bytes.Buffer{},
				}
			},
			data:          []byte("test data"),
			expectedError: "failed to write to gzip writer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compressDataWithFactory(tt.data, tt.writerFactory)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Error())
			}
		})
	}
}

func TestAPISender_SendWithContext(t *testing.T) {
	// Setup mock logger
	ml := &mockLogger{}
	originalLogger := logger.GetDefaultLogger()
	logger.SetDefaultLogger(ml)
	defer logger.SetDefaultLogger(originalLogger)

	tests := []struct {
		name           string
		setupServer    func() (*httptest.Server, *int)
		metrics        []collector.Metrics
		encryptionKey  string
		expectedError  string
		expectedCalls  int
		expectedStatus int
		expectedLog    string
	}{
		{
			name: "successful unencrypted small request",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusOK)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey:  "",
			expectedError:  "",
			expectedCalls:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name: "successful unencrypted large request with compression",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusOK)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey:  "",
			expectedError:  "",
			expectedCalls:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name: "successful encrypted small request",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusOK)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey:  "12345678901234567890123456789012",
			expectedError:  "",
			expectedCalls:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name: "successful encrypted large request with compression",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusOK)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey:  "12345678901234567890123456789012",
			expectedError:  "",
			expectedCalls:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name: "encryption not available fallback with compression",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if calls == 1 {
						w.WriteHeader(http.StatusPreconditionFailed)
					} else {
						w.WriteHeader(http.StatusOK)
					}
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey:  "12345678901234567890123456789012",
			expectedError:  "",
			expectedCalls:  2,
			expectedStatus: http.StatusOK,
			expectedLog:    "Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission.",
		},
		{
			name: "server error",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusInternalServerError)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey:  "",
			expectedError:  "API request failed with status 500",
			expectedCalls:  1,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "invalid encryption key",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusBadRequest)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey:  "invalid-key-length",
			expectedError:  "invalid encryption key: encryption key must be exactly 32 bytes long",
			expectedCalls:  0,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "too many metrics error (413)",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusRequestEntityTooLarge)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "WARNING: API request failed with status 413 - Too many metrics for your plan",
			expectedCalls: 1,
		},
		{
			name: "rate limit error (429) without header",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusTooManyRequests)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "WARNING: API request failed with status 429 - Rate limit exceeded",
			expectedCalls: 1,
		},
		{
			name: "rate limit error (429) with header",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.Header().Set("X-Rate-Limit", "60")
					w.WriteHeader(http.StatusTooManyRequests)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "WARNING: API request failed with status 429 - Rate limit exceeded, recommended interval: 60 seconds",
			expectedCalls: 1,
		},
		{
			name: "service unavailable error (503)",
			setupServer: func() (*httptest.Server, *int) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.WriteHeader(http.StatusServiceUnavailable)
				}))
				return server, &calls
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "WARNING: API request failed with status 503 - Server is undergoing maintenance",
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			ml.buffer.Reset()

			server, calls := tt.setupServer()
			defer server.Close()

			restartChan := make(chan struct{}, 1)
			sender := NewAPISender(
				server.URL,
				"test-project",
				"test-server",
				"test-token",
				"test-machine",
				tt.encryptionKey,
				"", // no config path needed
				restartChan,
			)

			err := sender.SendWithContext(context.Background(), tt.metrics)
			if (err != nil) != (tt.expectedError != "") {
				t.Errorf("SendWithContext() error = %v, wantErr %v", err, tt.expectedError != "")
			}

			if err != nil && !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Error())
			}

			if *calls != tt.expectedCalls {
				t.Errorf("expected %d API calls, got %d", tt.expectedCalls, *calls)
			}

			// Check for expected log message
			if tt.expectedLog != "" && !strings.Contains(ml.buffer.String(), tt.expectedLog) {
				t.Errorf("expected log message %q, got %q", tt.expectedLog, ml.buffer.String())
			}
		})
	}
}

func TestAPISender_SendWithContext_Concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		var decodedBody []byte
		if r.Header.Get("Content-Encoding") == "gzip" {
			decodedBody, err = decompressGzip(body)
			if err != nil {
				t.Errorf("Failed to decompress request body: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			decodedBody = body
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(decodedBody, &payload); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if encrypted, ok := payload["encrypted"].(bool); ok && encrypted {
			w.WriteHeader(http.StatusPreconditionFailed)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	restartChan := make(chan struct{}, 1)
	sender := NewAPISender(
		server.URL,
		"test-project",
		"test-server",
		"test-token",
		"test-machine",
		"", // no encryption key
		"", // no config path needed
		restartChan,
	)

	// Run concurrent requests
	numGoroutines := 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			metrics := []collector.Metrics{
				{
					Timestamp: time.Now(),
					Category:  "system",
					Name:      "cpu",
					Value:     float64(i),
				},
			}
			if err := sender.Send(metrics); err != nil {
				errors <- fmt.Errorf("goroutine %d error: %v", i, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		t.Errorf("Got %d errors during concurrent execution: %v", len(errs), errs)
	}
}

// Test error cases for SendWithContext
func TestAPISender_SendWithContext_Errors(t *testing.T) {
	// Setup mock logger
	ml := &mockLogger{}
	originalLogger := logger.GetDefaultLogger()
	logger.SetDefaultLogger(ml)
	defer logger.SetDefaultLogger(originalLogger)

	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		metrics       []collector.Metrics
		encryptionKey string
		expectedError string
	}{
		{
			name: "server timeout",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(200 * time.Millisecond)
				}))
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "context deadline exceeded",
		},
		{
			name: "server closes connection",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hj, ok := w.(http.Hijacker)
					if !ok {
						http.Error(w, "hijacking not supported", http.StatusInternalServerError)
						return
					}
					conn, _, err := hj.Hijack()
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					conn.Close()
				}))
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "",
			expectedError: "EOF",
		},
		{
			name: "invalid encryption key",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
				}))
			},
			metrics: []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "system",
				Name:      "cpu",
				Value:     45.67,
			}},
			encryptionKey: "invalid-key-length",
			expectedError: "invalid encryption key: encryption key must be exactly 32 bytes long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			ml.buffer.Reset()

			server := tt.setupServer()
			defer server.Close()

			restartChan := make(chan struct{}, 1)
			sender := NewAPISender(
				server.URL,
				"test-project",
				"test-server",
				"test-token",
				"test-machine",
				tt.encryptionKey,
				"", // no config path needed
				restartChan,
			)

			// Set a short timeout for the client
			sender.client.Timeout = 100 * time.Millisecond

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err := sender.SendWithContext(ctx, tt.metrics)
			if err == nil {
				t.Error("SendWithContext() error = nil, wantErr true")
				return
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Error())
			}
		})
	}
}

// Test the default gzip writer factory
func Test_defaultGzipWriterFactory(t *testing.T) {
	var buf bytes.Buffer
	writer := defaultGzipWriterFactory(&buf)

	testData := []byte("test data for compression")
	if _, err := writer.Write(testData); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify the output is valid gzip data
	reader, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Errorf("NewReader() error = %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(decompressed, testData) {
		t.Errorf("Decompressed data = %v, want %v", decompressed, testData)
	}
}

// Test compressDataWithWriter function
func Test_compressDataWithWriter(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		writer  compressWriter
		wantErr bool
	}{
		{
			name: "successful compression",
			data: []byte("test data"),
			writer: &mockCompressWriter{
				buf: &bytes.Buffer{},
			},
			wantErr: false,
		},
		{
			name: "write error",
			data: []byte("test data"),
			writer: &mockCompressWriter{
				writeErr: errors.New("mock write error"),
				buf:      &bytes.Buffer{},
			},
			wantErr: true,
		},
		{
			name: "close error",
			data: []byte("test data"),
			writer: &mockCompressWriter{
				closeErr: errors.New("mock close error"),
				buf:      &bytes.Buffer{},
			},
			wantErr: true,
		},
		{
			name: "write and close error",
			data: []byte("test data"),
			writer: &mockCompressWriter{
				writeErr: errors.New("mock write error"),
				closeErr: errors.New("mock close error"),
				buf:      &bytes.Buffer{},
			},
			wantErr: true,
		},
		{
			name: "empty data",
			data: []byte{},
			writer: &mockCompressWriter{
				buf: &bytes.Buffer{},
			},
			wantErr: false,
		},
		{
			name: "nil data",
			data: nil,
			writer: &mockCompressWriter{
				buf: &bytes.Buffer{},
			},
			wantErr: false,
		},
		{
			name: "large data",
			data: bytes.Repeat([]byte("a"), 1000),
			writer: &mockCompressWriter{
				buf: &bytes.Buffer{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := compressDataWithWriter(tt.data, tt.writer)
			if (err != nil) != tt.wantErr {
				t.Errorf("compressDataWithWriter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the result is not nil and has some data
				if result == nil {
					t.Error("compressDataWithWriter() returned nil result")
					return
				}

				// For non-empty input, verify some data was written
				if len(tt.data) > 0 && len(result) == 0 {
					t.Error("compressDataWithWriter() returned empty result for non-empty input")
				}

				// For empty input, verify minimal output
				if len(tt.data) == 0 && len(result) > 10 {
					t.Errorf("compressDataWithWriter() returned too much data for empty input: %d bytes", len(result))
				}
			}
		})
	}
}

// Test gzipWriter Bytes method specifically
func Test_gzipWriter_Bytes(t *testing.T) {
	tests := []struct {
		name     string
		setupBuf func() io.Writer
		wantNil  bool
	}{
		{
			name: "with bytes.Buffer",
			setupBuf: func() io.Writer {
				return &bytes.Buffer{}
			},
			wantNil: false,
		},
		{
			name: "with non-bytes.Buffer writer",
			setupBuf: func() io.Writer {
				// Use a writer that's not a *bytes.Buffer
				return &mockWriter{}
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := tt.setupBuf()
			writer := &gzipWriter{
				buf: buf,
				gw:  gzip.NewWriter(buf),
			}

			// Write some test data
			testData := []byte("test data for bytes method")
			if _, err := writer.Write(testData); err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}

			// Test the Bytes method
			result := writer.Bytes()

			if tt.wantNil {
				if result != nil {
					t.Errorf("Bytes() = %v, want nil for non-bytes.Buffer writer", result)
				}
			} else {
				if result == nil {
					t.Error("Bytes() = nil, want non-nil for bytes.Buffer writer")
				} else if len(result) == 0 {
					t.Error("Bytes() returned empty slice, expected compressed data")
				}
			}
		})
	}
}

// mockWriter implements io.Writer but is not a *bytes.Buffer
type mockWriter struct {
	data []byte
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func TestAPISender_ConfigAutoUpdate(t *testing.T) {
	// Setup mock logger
	ml := &mockLogger{}
	originalLogger := logger.GetDefaultLogger()
	logger.SetDefaultLogger(ml)
	defer logger.SetDefaultLogger(originalLogger)

	tempConfigFile, err := os.CreateTemp("", "config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tempConfigFile.Name())

	// Write initial config content
	initialConfig := []byte("initial: config")
	if err := os.WriteFile(tempConfigFile.Name(), initialConfig, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	tests := []struct {
		name              string
		configLastUpdate  string // X-Configuration-Last-Update header value
		configResponse    []byte // Response from /config endpoint
		expectConfigFetch bool   // Whether we expect a config fetch request
		expectRestart     bool   // Whether we expect a restart signal
		serverStatus      int    // HTTP status for /config endpoint
		shouldReturnError bool
		expectedLog       string
	}{
		{
			name:              "no header - no update",
			configLastUpdate:  "",
			expectConfigFetch: false,
			expectRestart:     false,
			serverStatus:      http.StatusOK,
		},
		{
			name:              "older timestamp - no update",
			configLastUpdate:  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			expectConfigFetch: false,
			expectRestart:     false,
			serverStatus:      http.StatusOK,
		},
		{
			name:              "newer timestamp - update config",
			configLastUpdate:  time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			configResponse:    []byte("new: config"),
			expectConfigFetch: true,
			expectRestart:     true,
			serverStatus:      http.StatusOK,
			expectedLog:       "Config updated from server, triggering restart...",
		},
		{
			name:              "config fetch fails",
			configLastUpdate:  time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			expectConfigFetch: true,
			expectRestart:     false,
			serverStatus:      http.StatusInternalServerError,
			expectedLog:       "Failed to fetch config: status 500",
		},
		{
			name:              "invalid timestamp",
			configLastUpdate:  "invalid-timestamp",
			expectConfigFetch: false,
			expectRestart:     false,
			serverStatus:      http.StatusOK,
			expectedLog:       "Invalid X-Configuration-Last-Update header: invalid timestamp format: invalid-timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			ml.buffer.Reset()

			// Create a channel to receive restart signals
			restartChan := make(chan struct{}, 1)
			configFetchCount := 0

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					configFetchCount++
					w.WriteHeader(tt.serverStatus)
					if tt.configResponse != nil {
						w.Write(tt.configResponse)
					}
					return
				}

				// Regular metrics endpoint
				if tt.configLastUpdate != "" {
					w.Header().Set("X-Configuration-Last-Update", tt.configLastUpdate)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create sender with temp config file
			sender := NewAPISender(
				server.URL,
				"test-org",
				"test-server",
				"test-token",
				"test-machine",
				"",
				tempConfigFile.Name(),
				restartChan,
			)

			// Send test metrics
			metrics := []collector.Metrics{{
				Timestamp: time.Now(),
				Category:  "test",
				Name:      "metric",
				Value:     1.0,
			}}

			err := sender.Send(metrics)
			if (err != nil) != tt.shouldReturnError {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.shouldReturnError)
			}

			// Check if config was fetched as expected
			if tt.expectConfigFetch && configFetchCount == 0 {
				t.Error("Expected config fetch but none occurred")
			}
			if !tt.expectConfigFetch && configFetchCount > 0 {
				t.Error("Unexpected config fetch occurred")
			}

			// Check if restart was triggered as expected
			select {
			case <-restartChan:
				if !tt.expectRestart {
					t.Error("Unexpected restart signal")
				}
			default:
				if tt.expectRestart {
					t.Error("Expected restart signal but none received")
				}
			}

			// Check for expected log message
			if tt.expectedLog != "" && !strings.Contains(ml.buffer.String(), tt.expectedLog) {
				t.Errorf("expected log message %q, got %q", tt.expectedLog, ml.buffer.String())
			}
		})
	}
}

func TestAPISender_UpdateSendIntervalInConfig(t *testing.T) {
	// Setup mock logger
	ml := &mockLogger{}
	originalLogger := logger.GetDefaultLogger()
	logger.SetDefaultLogger(ml)
	defer logger.SetDefaultLogger(originalLogger)

	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `api:
  url: "https://api.example.com"
  organization_id: "test-org"
  server_id: "test-server"
  application_token: "test-token"
machine_name: "test-machine"
sender:
  target: "api"
  send_interval: "10s"
logging:
  file_path: "/tmp/app.log"
collection:
  cpu:
    enabled: true
    interval: "1s"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Create a channel to receive restart signals
	restartChan := make(chan struct{}, 1)

	// Create the API sender
	sender := NewAPISender(
		"https://api.example.com",
		"test-org",
		"test-server",
		"test-token",
		"test-machine",
		"",
		configPath,
		restartChan,
	)

	// Test updating the send interval
	err := sender.updateSendIntervalInConfig("60")
	if err != nil {
		t.Errorf("updateSendIntervalInConfig() error = %v", err)
	}

	// Read the updated config file
	updatedConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read updated config file: %v", err)
	}

	// Check if the send_interval was updated
	if !strings.Contains(string(updatedConfig), "send_interval: 60s") {
		t.Errorf("send_interval was not updated correctly, got: %s", string(updatedConfig))
	}

	// Check if restart was signaled
	select {
	case <-restartChan:
		// Expected
	default:
		t.Error("Restart signal was not sent")
	}

	// Test with invalid rate limit
	err = sender.updateSendIntervalInConfig("invalid")
	if err == nil {
		t.Error("updateSendIntervalInConfig() expected error for invalid rate limit")
	}

	// Test with missing send_interval in config
	invalidConfigPath := filepath.Join(tempDir, "invalid_config.yaml")
	invalidConfig := `api:
  url: "https://api.example.com"
  organization_id: "test-org"
  server_id: "test-server"
  application_token: "test-token"
machine_name: "test-machine"
sender:
  target: "api"
logging:
  file_path: "/tmp/app.log"
`
	if err := os.WriteFile(invalidConfigPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to create invalid test config file: %v", err)
	}

	invalidSender := NewAPISender(
		"https://api.example.com",
		"test-org",
		"test-server",
		"test-token",
		"test-machine",
		"",
		invalidConfigPath,
		restartChan,
	)

	err = invalidSender.updateSendIntervalInConfig("60")
	if err == nil {
		t.Error("updateSendIntervalInConfig() expected error for missing send_interval")
	}
}

func TestAPISender_SendConfigValidation(t *testing.T) {
	// Setup mock logger
	ml := &mockLogger{}
	originalLogger := logger.GetDefaultLogger()
	logger.SetDefaultLogger(ml)
	defer logger.SetDefaultLogger(originalLogger)

	// Create a temporary config file
	tempConfigFile, err := os.CreateTemp("", "config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tempConfigFile.Name())

	// Write test configuration
	testConfig := `machine_name: "test-server"
sender:
  target: "api"
  send_interval: 5m
api:
  url: "https://api.test.com"
  organization_id: "test-org"
  server_id: "test-server"
  application_token: "test-token"
collection:
  cpu:
    enabled: true
    interval: 30s
`
	if err := os.WriteFile(tempConfigFile.Name(), []byte(testConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	tests := []struct {
		name               string
		serverResponse     int
		responseBody       string
		responseHeaders    map[string]string
		shouldReturnError  bool
		shouldUpdateConfig bool
		expectedLogRegex   string
	}{
		{
			name:              "valid config - 200 OK",
			serverResponse:    200,
			responseBody:      `{"status": "valid", "message": "Configuration is valid"}`,
			shouldReturnError: false,
			expectedLogRegex:  "Configuration validated successfully by API",
		},
		{
			name:           "config updated by API - 205",
			serverResponse: 205,
			responseBody: `machine_name: "test-server"
sender:
  target: "api"
  send_interval: 10m  # Updated by API
api:
  url: "https://api.test.com"
  organization_id: "test-org"
  server_id: "test-server"
  application_token: "test-token"
collection:
  cpu:
    enabled: true
    interval: 60s  # Updated by API
`,
			responseHeaders: map[string]string{
				"Content-Type": "application/x-yaml",
			},
			shouldReturnError:  false,
			shouldUpdateConfig: true,
			expectedLogRegex:   "Warning: API has made changes to the configuration",
		},
		{
			name:           "invalid config - 422",
			serverResponse: 422,
			responseBody:   `{"error": "Invalid configuration", "details": ["Missing required field: sender.target"]}`,
			responseHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			shouldReturnError: true,
			expectedLogRegex:  "FATAL: Configuration is invalid",
		},
		{
			name:              "authentication error - 401",
			serverResponse:    401,
			responseBody:      `{"error": "Invalid authentication"}`,
			shouldReturnError: true,
		},
		{
			name:              "server not found - 404",
			serverResponse:    404,
			responseBody:      `{"error": "Server not found"}`,
			shouldReturnError: true,
		},
		{
			name:              "server error - 500",
			serverResponse:    500,
			responseBody:      `{"error": "Internal server error"}`,
			shouldReturnError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			ml.buffer.Reset()

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Set response headers
				for k, v := range tt.responseHeaders {
					w.Header().Set(k, v)
				}

				// Set status code and write response
				w.WriteHeader(tt.serverResponse)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Create APISender
			sender := NewAPISender(
				server.URL,
				"test-org",
				"test-server",
				"test-token",
				"test-machine",
				"",
				tempConfigFile.Name(),
				make(chan struct{}, 1),
			)

			// Note: Fatal calls are captured by the mock logger's buffer

			// Call SendConfigValidation
			err := sender.SendConfigValidation(tempConfigFile.Name())

			// Check error expectation
			if (err != nil) != tt.shouldReturnError {
				t.Errorf("SendConfigValidation() error = %v, wantErr %v", err, tt.shouldReturnError)
			}

			// Check if config was updated
			if tt.shouldUpdateConfig {
				updatedConfig, err := os.ReadFile(tempConfigFile.Name())
				if err != nil {
					t.Errorf("Failed to read updated config: %v", err)
				} else {
					if !strings.Contains(string(updatedConfig), "send_interval: 10m") {
						t.Error("Config was not updated with API changes")
					}
				}
			}

			// Check log output
			if tt.expectedLogRegex != "" {
				logOutput := ml.buffer.String()
				if !strings.Contains(logOutput, tt.expectedLogRegex) {
					t.Errorf("Expected log to contain %q, got %q", tt.expectedLogRegex, logOutput)
				}
			}

			// Fatal calls are logged to the mock logger buffer and checked via expectedLogRegex
		})
	}
}
