package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/encryption"
	"github.com/monitorly-app/probe/internal/logger"
)

// APISender sends metrics to a remote API endpoint
type APISender struct {
	baseURL               string
	organizationID        string
	serverID              string
	applicationToken      string
	machineName           string
	encryptionKey         string
	client                *http.Client
	encryptionWarningOnce sync.Once
}

// NewAPISender creates a new APISender instance
func NewAPISender(baseURL, organizationID, serverID, applicationToken, machineName, encryptionKey string) *APISender {
	return &APISender{
		baseURL:          baseURL,
		organizationID:   organizationID,
		serverID:         serverID,
		applicationToken: applicationToken,
		machineName:      machineName,
		encryptionKey:    encryptionKey,
		client:           &http.Client{Timeout: 30 * time.Second},
	}
}

// Send sends metrics to the API endpoint
func (s *APISender) Send(metrics []collector.Metrics) error {
	return s.SendWithContext(context.Background(), metrics)
}

// SendWithContext sends metrics to the API endpoint with the provided context
func (s *APISender) SendWithContext(ctx context.Context, metrics []collector.Metrics) error {
	// Determine if this is system info or regular metrics
	isSystemInfo := false
	if len(metrics) == 1 && metrics[0].Name == collector.NameSystemInfo {
		isSystemInfo = true
	}

	// Build the appropriate URL
	var url string
	if isSystemInfo {
		url = fmt.Sprintf("%s/api/%s/servers/%s/info", s.baseURL, s.organizationID, s.serverID)
	} else {
		url = fmt.Sprintf("%s/api/%s/servers/%s/metrics", s.baseURL, s.organizationID, s.serverID)
	}

	// Prepare request body
	requestBody := map[string]interface{}{
		"machine_name": s.machineName,
		"metrics":      metrics,
	}

	// First try with encryption if a key is provided
	var requestData []byte
	var isEncrypted bool
	var isCompressed bool

	if s.encryptionKey != "" {
		if err := encryption.ValidateKey(s.encryptionKey); err != nil {
			return fmt.Errorf("invalid encryption key: %w", err)
		}

		// Marshal the original request body for encryption
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}

		// Encrypt the data
		encryptedData, err := encryption.Encrypt(jsonData, s.encryptionKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt data: %w", err)
		}

		// Prepare encrypted request body
		encryptedBody := map[string]interface{}{
			"machine_name": s.machineName,
			"encrypted":    true,
			"data":         encryptedData,
		}

		requestData, err = json.Marshal(encryptedBody)
		if err != nil {
			return fmt.Errorf("failed to marshal encrypted request body: %w", err)
		}
		isEncrypted = true
	} else {
		// No encryption, marshal the request body
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		requestData = jsonData
	}

	// Always compress the request data
	compressedData, err := compressData(requestData)
	if err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	requestData = compressedData
	isCompressed = true

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.applicationToken)
	if isCompressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle encryption not available (premium feature)
	if resp.StatusCode == http.StatusPreconditionFailed && isEncrypted {
		// Log warning only once per sender instance
		s.encryptionWarningOnce.Do(func() {
			logger.GetDefaultLogger().Printf("Warning: Encryption not available (requires premium subscription). Falling back to unencrypted transmission.")
		})

		// Retry without encryption - use the original request body
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal fallback request body: %w", err)
		}

		// Always compress the fallback request
		compressedData, err := compressData(jsonData)
		if err != nil {
			return fmt.Errorf("failed to compress fallback data: %w", err)
		}
		fallbackRequestData := compressedData
		fallbackIsCompressed := true

		// Create new request
		fallbackReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(fallbackRequestData))
		if err != nil {
			return fmt.Errorf("failed to create fallback request: %w", err)
		}

		// Set headers
		fallbackReq.Header.Set("Content-Type", "application/json")
		fallbackReq.Header.Set("Authorization", "Bearer "+s.applicationToken)
		if fallbackIsCompressed {
			fallbackReq.Header.Set("Content-Encoding", "gzip")
		}

		// Send fallback request
		resp, err = s.client.Do(fallbackReq)
		if err != nil {
			return fmt.Errorf("failed to send fallback request: %w", err)
		}
		defer resp.Body.Close()
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return nil
}
