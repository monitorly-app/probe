package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

type APISender struct {
	apiURL string
	apiKey string
	client *http.Client
}

func NewAPISender(apiURL, apiKey string) *APISender {
	return &APISender{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *APISender) Send(metrics []collector.Metrics) error {
	payload, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("error marshalling metrics: %w", err)
	}

	req, err := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(payload))
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
