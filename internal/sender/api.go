package sender

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/encryption"
	"github.com/monitorly-app/probe/internal/logger"
)

// APISender implements the Sender interface for API-based metric sending
type APISender struct {
	apiURL           string
	projectID        string
	applicationToken string
	machineName      string
	encryptionKey    string
	client           *http.Client
	mu               sync.RWMutex
	encryptionFailed atomic.Bool
	warningLogged    atomic.Bool
}

// APIPayload represents the structure of the data sent to the API
type APIPayload struct {
	MachineName string              `json:"machine_name"`
	Metrics     []collector.Metrics `json:"metrics"`
	Encrypted   bool                `json:"encrypted"`
	Compressed  bool                `json:"compressed"`
}

// compressWriter is an interface that wraps the basic Write, Close, and Bytes methods
type compressWriter interface {
	io.WriteCloser
	Bytes() []byte
}

// writerFactory is a function type that creates a new compressWriter
type writerFactory func(io.Writer) compressWriter

// gzipWriter implements compressWriter using gzip compression
type gzipWriter struct {
	buf io.Writer
	gw  *gzip.Writer
}

// Write writes data to the gzip writer
func (w *gzipWriter) Write(p []byte) (n int, err error) {
	return w.gw.Write(p)
}

// Close closes the gzip writer
func (w *gzipWriter) Close() error {
	return w.gw.Close()
}

// Bytes returns the compressed data
func (w *gzipWriter) Bytes() []byte {
	if buf, ok := w.buf.(*bytes.Buffer); ok {
		return buf.Bytes()
	}
	return nil
}

// defaultGzipWriterFactory creates a new gzip writer
func defaultGzipWriterFactory(w io.Writer) compressWriter {
	return &gzipWriter{
		buf: w,
		gw:  gzip.NewWriter(w),
	}
}

// compressData compresses the input data using gzip
func compressData(data []byte) ([]byte, error) {
	return compressDataWithFactory(data, defaultGzipWriterFactory)
}

// compressDataWithFactory compresses the input data using the provided writer factory
func compressDataWithFactory(data []byte, newWriter writerFactory) ([]byte, error) {
	var buf bytes.Buffer
	gw := newWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write to gzip writer: %w", err)
	}

	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// compressDataWithWriter compresses the input data using the provided writer
func compressDataWithWriter(data []byte, gw compressWriter) ([]byte, error) {
	if data == nil {
		data = []byte{}
	}

	if _, err := gw.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write to gzip writer: %w", err)
	}

	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	result := gw.Bytes()
	if result == nil {
		return []byte{}, nil
	}
	return result, nil
}

// NewAPISender creates a new instance of APISender
func NewAPISender(apiURL, projectID, applicationToken, machineName, encryptionKey string) *APISender {
	return &APISender{
		apiURL:           apiURL,
		projectID:        projectID,
		applicationToken: applicationToken,
		machineName:      machineName,
		encryptionKey:    encryptionKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send sends metrics to the configured API endpoint
func (s *APISender) Send(metrics []collector.Metrics) error {
	return s.SendWithContext(context.Background(), metrics)
}

// SendWithContext sends metrics to the configured API endpoint with context support
func (s *APISender) SendWithContext(ctx context.Context, metrics []collector.Metrics) error {
	// Create initial payload
	payload := APIPayload{
		MachineName: s.machineName,
		Metrics:     metrics,
		Encrypted:   false,
		Compressed:  false,
	}

	// Check if we should attempt encryption
	shouldEncrypt := s.encryptionKey != "" && !s.encryptionFailed.Load()

	var jsonData []byte
	var err error
	var isCompressed bool

	if shouldEncrypt {
		// Marshal the payload first
		jsonData, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("error marshalling metrics: %w", err)
		}

		// Encrypt the JSON data
		encryptedData, err := encryption.Encrypt(jsonData, s.encryptionKey)
		if err != nil {
			return fmt.Errorf("error encrypting metrics: %w", err)
		}

		// Create a new payload with the encrypted data
		jsonData, err = json.Marshal(map[string]interface{}{
			"machine_name": s.machineName,
			"encrypted":    true,
			"compressed":   false,
			"data":         encryptedData,
		})
		if err != nil {
			return fmt.Errorf("error marshalling encrypted payload: %w", err)
		}
	} else {
		// Send unencrypted data
		jsonData, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("error marshalling metrics: %w", err)
		}
	}

	// Compress the data if it's larger than 1KB
	useCompression := len(jsonData) > 1024
	var finalData []byte

	if useCompression {
		// First update the compression flag in the payload
		if shouldEncrypt {
			var encryptedPayload map[string]interface{}
			if err := json.Unmarshal(jsonData, &encryptedPayload); err != nil {
				return fmt.Errorf("error unmarshalling encrypted payload: %w", err)
			}
			encryptedPayload["compressed"] = true
			jsonData, err = json.Marshal(encryptedPayload)
			if err != nil {
				return fmt.Errorf("error marshalling updated encrypted payload: %w", err)
			}
		} else {
			payload.Compressed = true
			jsonData, err = json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("error marshalling updated payload: %w", err)
			}
		}

		// Then compress the updated JSON data
		compressedData, err := compressData(jsonData)
		if err != nil {
			logger.Printf("Warning: Failed to compress data: %v. Sending uncompressed.", err)
			finalData = jsonData
			isCompressed = false
			// Reset compression flags since compression failed
			if shouldEncrypt {
				var encryptedPayload map[string]interface{}
				if err := json.Unmarshal(jsonData, &encryptedPayload); err == nil {
					encryptedPayload["compressed"] = false
					if newJSON, err := json.Marshal(encryptedPayload); err == nil {
						jsonData = newJSON
						finalData = newJSON
					}
				}
			} else {
				payload.Compressed = false
				if newJSON, err := json.Marshal(payload); err == nil {
					jsonData = newJSON
					finalData = newJSON
				}
			}
		} else {
			finalData = compressedData
			isCompressed = true
		}
	} else {
		finalData = jsonData
		isCompressed = false
	}

	// Ensure the URL ends with a trailing slash for consistent path joining
	baseURL := s.apiURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// Include project ID in the URL
	url := fmt.Sprintf("%s%s", baseURL, s.projectID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(finalData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.applicationToken)
	if useCompression && isCompressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPreconditionFailed {
		// Only log the warning once
		if !s.warningLogged.Swap(true) {
			logger.Printf("Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission.")
		}
		s.encryptionFailed.Store(true)
		// Retry without encryption
		return s.SendWithContext(ctx, metrics)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API returned non-success status: %s", resp.Status)
	}

	return nil
}
