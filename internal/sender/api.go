package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

// APISender implements the Sender interface for API-based metric sending
type APISender struct {
	apiURL      string
	apiKey      string
	machineName string
	client      *http.Client
}

// APIPayload represents the structure of the data sent to the API
type APIPayload struct {
	MachineName string              `json:"machine_name"`
	Metrics     []collector.Metrics `json:"metrics"`
}

// NewAPISender creates a new instance of APISender
func NewAPISender(apiURL, apiKey, machineName string) *APISender {
	return &APISender{
		apiURL:      apiURL,
		apiKey:      apiKey,
		machineName: machineName,
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
	// Create payload with machine name at the top level
	payload := APIPayload{
		MachineName: s.machineName,
		Metrics:     metrics,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling metrics: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API returned non-success status: %s", resp.Status)
	}

	return nil
}
