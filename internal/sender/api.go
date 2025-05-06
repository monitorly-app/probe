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
	apiURL string
	apiKey string
	client *http.Client
}

// NewAPISender creates a new instance of APISender
func NewAPISender(apiURL, apiKey string) *APISender {
	return &APISender{
		apiURL: apiURL,
		apiKey: apiKey,
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
	payload, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("error marshalling metrics: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, bytes.NewBuffer(payload))
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
