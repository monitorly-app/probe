package sender

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestAPISender_SendWithContext(t *testing.T) {
	// Setup mock logger
	ml := &mockLogger{}
	originalLogger := logger.GetDefaultLogger()
	logger.SetDefaultLogger(ml)
	defer logger.SetDefaultLogger(originalLogger)

	// Sample metrics for testing
	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  "system",
			Name:      "cpu",
			Value:     45.67,
		},
	}

	tests := []struct {
		name            string
		encryptionKey   string
		responses       []int // Status codes to return in sequence
		expectEncrypted bool  // Whether we expect the first request to be encrypted
		wantErr         bool
		wantLogMessage  string // Expected log message for encryption failure
	}{
		{
			name:            "successful unencrypted request",
			encryptionKey:   "",
			responses:       []int{http.StatusOK},
			expectEncrypted: false,
			wantErr:         false,
		},
		{
			name:            "successful encrypted request",
			encryptionKey:   "12345678901234567890123456789012", // 32 bytes
			responses:       []int{http.StatusOK},
			expectEncrypted: true,
			wantErr:         false,
		},
		{
			name:            "encryption not available fallback",
			encryptionKey:   "12345678901234567890123456789012", // 32 bytes
			responses:       []int{http.StatusPreconditionFailed, http.StatusOK},
			expectEncrypted: true,
			wantErr:         false,
			wantLogMessage:  "Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission.",
		},
		{
			name:            "encryption not available fallback failure",
			encryptionKey:   "12345678901234567890123456789012", // 32 bytes
			responses:       []int{http.StatusPreconditionFailed, http.StatusInternalServerError},
			expectEncrypted: true,
			wantErr:         true,
			wantLogMessage:  "Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			ml.buffer.Reset()

			responseIndex := 0
			var requests []*http.Request
			var requestBodies []string

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
				requestBodies = append(requestBodies, string(body))

				// Provide a new body for future reads
				r.Body = io.NopCloser(bytes.NewBuffer(body))

				w.WriteHeader(tt.responses[responseIndex])
				if responseIndex < len(tt.responses)-1 {
					responseIndex++
				}
			}))
			defer server.Close()

			// Create sender
			sender := NewAPISender(
				server.URL,
				"test-project",
				"test-token",
				"test-machine",
				tt.encryptionKey,
			)

			// Send metrics
			err := sender.SendWithContext(context.Background(), metrics)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("SendWithContext() error = %v, wantErr %v", err, tt.wantErr)
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

			// Decode the first request body
			var payload map[string]interface{}
			if err := json.NewDecoder(strings.NewReader(firstBody)).Decode(&payload); err != nil {
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

				// Decode the second request body
				var secondPayload map[string]interface{}
				if err := json.NewDecoder(strings.NewReader(requestBodies[1])).Decode(&secondPayload); err != nil {
					t.Fatalf("Failed to decode second request body: %v", err)
				}

				// Verify it's not encrypted
				if encrypted, ok := secondPayload["encrypted"].(bool); ok && encrypted {
					t.Error("Second request should not be encrypted")
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

	// Create test server that returns 412 for encrypted requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
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

	// Sample metrics
	metrics := []collector.Metrics{
		{
			Timestamp: time.Now(),
			Category:  "system",
			Name:      "cpu",
			Value:     45.67,
		},
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
	for i := 0; i < numGoroutines; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent SendWithContext() error = %v", err)
		}
	}

	// Verify that we got the expected number of requests
	if requestCount <= numGoroutines {
		t.Errorf("Expected more than %d requests due to retries, got %d", numGoroutines, requestCount)
	}

	// Verify that we got some encrypted requests before falling back
	if encryptedCount == 0 {
		t.Error("Expected some encrypted requests before fallback")
	}

	// Verify that the warning log appears exactly once
	logContent := ml.buffer.String()
	expectedLog := "Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission."
	count := strings.Count(logContent, expectedLog)
	if count != 1 {
		t.Errorf("Expected exactly one encryption warning log, got %d", count)
	}
}
