package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/encryption"
)

// APISender sends metrics to a remote API endpoint
type APISender struct {
	baseURL          string
	organizationID   string
	serverID         string
	applicationToken string
	machineName      string
	encryptionKey    string
	client           *http.Client
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
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send sends metrics to the API endpoint using a background context
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

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Encrypt data if encryption key is provided
	var requestData []byte
	if s.encryptionKey != "" {
		encryptedData, err := encryption.Encrypt(jsonData, s.encryptionKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt data: %w", err)
		}
		requestData = []byte(encryptedData)
	} else {
		requestData = jsonData
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.applicationToken))
	if s.encryptionKey != "" {
		req.Header.Set("X-Encrypted", "true")
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	return nil
}
