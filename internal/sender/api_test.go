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
	"strings"
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
	m.Printf(format, v...)
	panic("logger.Fatalf called during test")
}

func (m *mockLogger) Close() error {
	return nil
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewAPISender(
		server.URL,
		"test-project",
		"test-token",
		"test-machine",
		"",
	)

	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  "system",
			Name:      "cpu",
			Value:     45.67,
		},
	}

	if err := sender.Send(metrics); err != nil {
		t.Errorf("Send() error = %v", err)
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

	// Create a large metric payload that will trigger compression
	largeMetrics := make([]collector.Metrics, 100)
	now := time.Now()
	for i := range largeMetrics {
		largeMetrics[i] = collector.Metrics{
			Timestamp: now,
			Category:  "system",
			Name:      "cpu",
			Value:     float64(i),
			Metadata: collector.MetricMetadata{
				"host":     fmt.Sprintf("host-%d", i),
				"instance": fmt.Sprintf("instance-%d", i),
				"region":   fmt.Sprintf("region-%d", i),
			},
		}
	}

	// Sample metrics for testing (small payload)
	smallMetrics := []collector.Metrics{
		{
			Timestamp: now,
			Category:  "system",
			Name:      "cpu",
			Value:     45.67,
		},
	}

	tests := []struct {
		name             string
		setupSender      func(server *httptest.Server) *APISender
		setupContext     func() (context.Context, context.CancelFunc)
		metrics          []collector.Metrics
		responses        []int // Status codes to return in sequence
		expectEncrypted  bool  // Whether we expect the first request to be encrypted
		expectCompressed bool  // Whether we expect the request to be compressed
		wantErr          bool
		wantLogMessage   string // Expected log message for encryption failure
		skipRequestCheck bool   // Skip request validation for error cases
	}{
		{
			name: "successful unencrypted small request",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "")
			},
			setupContext:     func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:          smallMetrics,
			responses:        []int{http.StatusOK},
			expectEncrypted:  false,
			expectCompressed: false,
			wantErr:          false,
		},
		{
			name: "successful unencrypted large request with compression",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "")
			},
			setupContext:     func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:          largeMetrics,
			responses:        []int{http.StatusOK},
			expectEncrypted:  false,
			expectCompressed: true,
			wantErr:          false,
		},
		{
			name: "successful encrypted small request",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "12345678901234567890123456789012")
			},
			setupContext:     func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:          smallMetrics,
			responses:        []int{http.StatusOK},
			expectEncrypted:  true,
			expectCompressed: false,
			wantErr:          false,
		},
		{
			name: "successful encrypted large request with compression",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "12345678901234567890123456789012")
			},
			setupContext:     func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:          largeMetrics,
			responses:        []int{http.StatusOK},
			expectEncrypted:  true,
			expectCompressed: true,
			wantErr:          false,
		},
		{
			name: "encryption not available fallback with compression",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "12345678901234567890123456789012")
			},
			setupContext:     func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:          largeMetrics,
			responses:        []int{http.StatusPreconditionFailed, http.StatusOK},
			expectEncrypted:  true,
			expectCompressed: true,
			wantErr:          false,
			wantLogMessage:   "Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission.",
		},
		{
			name: "encryption not available fallback failure",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "12345678901234567890123456789012")
			},
			setupContext:     func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:          smallMetrics,
			responses:        []int{http.StatusPreconditionFailed, http.StatusInternalServerError},
			expectEncrypted:  true,
			expectCompressed: false,
			wantErr:          true,
			wantLogMessage:   "Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission.",
		},
		{
			name: "invalid URL",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender("invalid://url", "test-project", "test-token", "test-machine", "")
			},
			setupContext:     func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:          smallMetrics,
			wantErr:          true,
			skipRequestCheck: true,
		},
		{
			name: "context cancelled",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "")
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx, cancel
			},
			metrics:          smallMetrics,
			wantErr:          true,
			skipRequestCheck: true,
		},
		{
			name: "server error",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "")
			},
			setupContext: func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:      smallMetrics,
			responses:    []int{http.StatusInternalServerError},
			wantErr:      true,
		},
		{
			name: "invalid encryption key length",
			setupSender: func(server *httptest.Server) *APISender {
				return NewAPISender(server.URL, "test-project", "test-token", "test-machine", "invalid-key")
			},
			setupContext:     func() (context.Context, context.CancelFunc) { return context.Background(), func() {} },
			metrics:          smallMetrics,
			wantErr:          true,
			skipRequestCheck: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			ml.buffer.Reset()

			responseIndex := 0
			var requests []*http.Request
			var requestBodies [][]byte

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Read and store the request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("Failed to read request body: %v", err)
				}
				r.Body.Close()

				// Store the request and body
				requests = append(requests, r)
				requestBodies = append(requestBodies, body)

				// Provide a new body for future reads
				r.Body = io.NopCloser(bytes.NewBuffer(body))

				if tt.responses != nil && len(tt.responses) > 0 {
					w.WriteHeader(tt.responses[responseIndex])
					if responseIndex < len(tt.responses)-1 {
						responseIndex++
					}
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer server.Close()

			// Create sender
			sender := tt.setupSender(server)

			// Get context
			ctx, cancel := tt.setupContext()
			defer cancel()

			// Send metrics
			err := sender.SendWithContext(ctx, tt.metrics)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("SendWithContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Skip request validation for error cases where no request is expected
			if tt.skipRequestCheck {
				return
			}

			// Verify the first request
			if len(requests) == 0 {
				t.Fatal("no request was made")
			}

			firstRequest := requests[0]
			firstBody := requestBodies[0]

			// Check Content-Type and Authorization headers
			if got := firstRequest.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %v, want application/json", got)
			}
			if got := firstRequest.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf("Authorization = %v, want Bearer test-token", got)
			}

			// Check compression header and decompress if needed
			var decodedBody []byte
			if tt.expectCompressed {
				if got := firstRequest.Header.Get("Content-Encoding"); got != "gzip" {
					t.Errorf("Content-Encoding = %v, want gzip", got)
				}
				var err error
				decodedBody, err = decompressGzip(firstBody)
				if err != nil {
					t.Fatalf("Failed to decompress request body: %v", err)
				}
			} else {
				if got := firstRequest.Header.Get("Content-Encoding"); got != "" {
					t.Errorf("Content-Encoding header should not be present for uncompressed data, got %v", got)
				}
				decodedBody = firstBody
			}

			// Decode the first request body
			var payload map[string]interface{}
			if err := json.Unmarshal(decodedBody, &payload); err != nil {
				t.Fatalf("Failed to decode request body: %v", err)
			}

			// Check if the request was encrypted as expected
			encrypted, ok := payload["encrypted"].(bool)
			if !ok && tt.expectEncrypted {
				t.Error("Expected encrypted field in payload")
			}
			if ok && encrypted != tt.expectEncrypted {
				t.Errorf("encrypted = %v, want %v", encrypted, tt.expectEncrypted)
			}

			// Check compression flag in payload
			if tt.expectCompressed {
				compressed, ok := payload["compressed"].(bool)
				if !ok || !compressed {
					t.Error("Expected compressed field to be true in payload")
				}
			}

			// If encryption was expected, verify encrypted data is present
			if tt.expectEncrypted && encrypted {
				if _, ok := payload["data"].(string); !ok {
					t.Error("Expected encrypted data field in payload")
				}
			}

			// Check for expected log message
			if tt.wantLogMessage != "" {
				if !strings.Contains(ml.buffer.String(), tt.wantLogMessage) {
					t.Errorf("Expected log message not found.\nWant: %s\nGot: %s", tt.wantLogMessage, ml.buffer.String())
				}
			}

			// For fallback case, verify the second request is unencrypted
			if len(tt.responses) > 1 && tt.responses[0] == http.StatusPreconditionFailed {
				if len(requests) < 2 {
					t.Fatal("Expected a second request for fallback case")
				}

				// Get the second request body
				secondBody := requestBodies[1]

				// Decompress if needed
				var decodedSecondBody []byte
				if tt.expectCompressed {
					if got := requests[1].Header.Get("Content-Encoding"); got != "gzip" {
						t.Errorf("Second request Content-Encoding = %v, want gzip", got)
					}
					var err error
					decodedSecondBody, err = decompressGzip(secondBody)
					if err != nil {
						t.Fatalf("Failed to decompress second request body: %v", err)
					}
				} else {
					decodedSecondBody = secondBody
				}

				// Decode the second request body
				var secondPayload map[string]interface{}
				if err := json.Unmarshal(decodedSecondBody, &secondPayload); err != nil {
					t.Fatalf("Failed to decode second request body: %v", err)
				}

				// Verify it's not encrypted
				if encrypted, ok := secondPayload["encrypted"].(bool); ok && encrypted {
					t.Error("Second request should not be encrypted")
				}

				// Check compression status remains consistent
				if tt.expectCompressed {
					compressed, ok := secondPayload["compressed"].(bool)
					if !ok || !compressed {
						t.Error("Expected compressed field to be true in second payload")
					}
				}
			}
		})
	}
}

func TestAPISender_SendWithContext_Concurrent(t *testing.T) {
	// Setup mock logger
	ml := &mockLogger{}
	originalLogger := logger.GetDefaultLogger()
	logger.SetDefaultLogger(ml)
	defer logger.SetDefaultLogger(originalLogger)

	requestCount := 0
	encryptedCount := 0
	compressedCount := 0

	// Create test server that returns 412 for encrypted requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Decompress if needed
		var decodedBody []byte
		if r.Header.Get("Content-Encoding") == "gzip" {
			compressedCount++
			decodedBody, err = decompressGzip(body)
			if err != nil {
				t.Errorf("Failed to decompress request body: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		} else {
			decodedBody = body
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(decodedBody, &payload); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		requestCount++
		if encrypted, ok := payload["encrypted"].(bool); ok && encrypted {
			encryptedCount++
			w.WriteHeader(http.StatusPreconditionFailed)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Create sender with encryption enabled
	sender := NewAPISender(
		server.URL,
		"test-project",
		"test-token",
		"test-machine",
		"12345678901234567890123456789012", // 32 bytes
	)

	// Create a large metric payload that will trigger compression
	now := time.Now()
	metrics := make([]collector.Metrics, 100)
	for i := range metrics {
		metrics[i] = collector.Metrics{
			Timestamp: now,
			Category:  "system",
			Name:      "cpu",
			Value:     float64(i),
			Metadata: collector.MetricMetadata{
				"host":     fmt.Sprintf("host-%d", i),
				"instance": fmt.Sprintf("instance-%d", i),
				"region":   fmt.Sprintf("region-%d", i),
			},
		}
	}

	// Send metrics concurrently
	const numGoroutines = 10
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			errChan <- sender.SendWithContext(context.Background(), metrics)
		}()
	}

	// Collect results
	var errors []error
	for i := 0; i < numGoroutines; i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	// We expect some errors due to encryption fallback
	if len(errors) > numGoroutines/2 {
		t.Errorf("Too many errors in concurrent execution: %v", errors)
	}

	// Verify that we got the expected number of requests
	if requestCount <= numGoroutines {
		t.Errorf("Expected more than %d requests due to retries, got %d", numGoroutines, requestCount)
	}

	// Verify that we got some encrypted requests before falling back
	if encryptedCount == 0 {
		t.Error("Expected some encrypted requests before fallback")
	}

	// Verify that compression was used
	if compressedCount == 0 {
		t.Error("Expected some compressed requests")
	}

	// Verify that the warning log appears exactly once
	logContent := ml.buffer.String()
	expectedLog := "Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission."
	count := strings.Count(logContent, expectedLog)
	if count != 1 {
		t.Errorf("Expected exactly one encryption warning log, got %d", count)
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
		setupContext  func() (context.Context, context.CancelFunc)
		metrics       []collector.Metrics
		encryptionKey string
		wantErr       bool
	}{
		{
			name: "server timeout",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(200 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}))
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 100*time.Millisecond)
			},
			metrics: []collector.Metrics{
				{
					Timestamp: time.Now(),
					Category:  "system",
					Name:      "cpu",
					Value:     45.67,
				},
			},
			wantErr: true,
		},
		{
			name: "server closes connection",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hj, ok := w.(http.Hijacker)
					if !ok {
						t.Fatal("webserver doesn't support hijacking")
					}
					conn, _, err := hj.Hijack()
					if err != nil {
						t.Fatal(err)
					}
					conn.Close()
				}))
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			metrics: []collector.Metrics{
				{
					Timestamp: time.Now(),
					Category:  "system",
					Name:      "cpu",
					Value:     45.67,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid encryption key length",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			metrics: []collector.Metrics{
				{
					Timestamp: time.Now(),
					Category:  "system",
					Name:      "cpu",
					Value:     45.67,
				},
			},
			encryptionKey: "invalid-key-length",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			ctx, cancel := tt.setupContext()
			defer cancel()

			sender := NewAPISender(
				server.URL,
				"test-project",
				"test-token",
				"test-machine",
				tt.encryptionKey,
			)

			err := sender.SendWithContext(ctx, tt.metrics)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendWithContext() error = %v, wantErr %v", err, tt.wantErr)
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

func TestAPISender_BootTimeInPayload(t *testing.T) {
	// Create a test server to capture the request
	var capturedPayload APIPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		err = json.Unmarshal(body, &capturedPayload)
		if err != nil {
			t.Fatalf("Failed to unmarshal request body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create sender
	sender := NewAPISender(server.URL, "test-project", "test-token", "test-machine", "")

	// Create test metrics
	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	// Send metrics
	err := sender.Send(metrics)
	if err != nil {
		t.Fatalf("Failed to send metrics: %v", err)
	}

	// Verify boot time is included in payload
	if capturedPayload.BootTime == nil {
		t.Error("BootTime should be included in the payload")
	} else {
		// Boot time should be a reasonable Unix timestamp (not zero and not in the future)
		now := time.Now().Unix()
		bootTime := *capturedPayload.BootTime

		if bootTime <= 0 {
			t.Errorf("BootTime should be positive, got: %d", bootTime)
		}

		if bootTime > now {
			t.Errorf("BootTime should not be in the future, got: %d, now: %d", bootTime, now)
		}

		// Boot time should be reasonable (not more than a year ago for testing purposes)
		oneYearAgo := now - (365 * 24 * 60 * 60)
		if bootTime < oneYearAgo {
			t.Errorf("BootTime seems too old, got: %d, one year ago: %d", bootTime, oneYearAgo)
		}
	}

	// Verify other fields are still present
	if capturedPayload.MachineName != "test-machine" {
		t.Errorf("Expected machine name 'test-machine', got: %s", capturedPayload.MachineName)
	}

	if len(capturedPayload.Metrics) != 1 {
		t.Errorf("Expected 1 metric, got: %d", len(capturedPayload.Metrics))
	}
}

func TestAPISender_BootTimeInEncryptedPayload(t *testing.T) {
	// Create a test server to capture the request
	var capturedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		err = json.Unmarshal(body, &capturedPayload)
		if err != nil {
			t.Fatalf("Failed to unmarshal request body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create sender with encryption key
	encryptionKey := "12345678901234567890123456789012" // 32 bytes
	sender := NewAPISender(server.URL, "test-project", "test-token", "test-machine", encryptionKey)

	// Create test metrics
	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  collector.CategorySystem,
			Name:      collector.NameCPU,
			Value:     75.5,
		},
	}

	// Send metrics
	err := sender.Send(metrics)
	if err != nil {
		t.Fatalf("Failed to send metrics: %v", err)
	}

	// Verify boot time is included in encrypted payload
	bootTimeInterface, exists := capturedPayload["boot_time"]
	if !exists {
		t.Error("boot_time should be included in the encrypted payload")
	} else {
		// Convert to int64 for validation
		var bootTime int64
		switch v := bootTimeInterface.(type) {
		case float64:
			bootTime = int64(v)
		case int64:
			bootTime = v
		default:
			t.Fatalf("boot_time should be a number, got: %T", v)
		}

		// Boot time should be a reasonable Unix timestamp
		now := time.Now().Unix()

		if bootTime <= 0 {
			t.Errorf("BootTime should be positive, got: %d", bootTime)
		}

		if bootTime > now {
			t.Errorf("BootTime should not be in the future, got: %d, now: %d", bootTime, now)
		}
	}

	// Verify other encrypted payload fields
	if capturedPayload["machine_name"] != "test-machine" {
		t.Errorf("Expected machine name 'test-machine', got: %v", capturedPayload["machine_name"])
	}

	if capturedPayload["encrypted"] != true {
		t.Errorf("Expected encrypted to be true, got: %v", capturedPayload["encrypted"])
	}

	if _, exists := capturedPayload["data"]; !exists {
		t.Error("Expected encrypted data field to be present")
	}
}
