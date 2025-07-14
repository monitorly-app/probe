package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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
	configPath            string        // Path to the config file
	restartChan           chan struct{} // Channel to signal restart
}

// NewAPISender creates a new APISender instance
func NewAPISender(baseURL, organizationID, serverID, applicationToken, machineName, encryptionKey, configPath string, restartChan chan struct{}) *APISender {
	return &APISender{
		baseURL:               baseURL,
		organizationID:        organizationID,
		serverID:              serverID,
		applicationToken:      applicationToken,
		machineName:           machineName,
		encryptionKey:         encryptionKey,
		client:                &http.Client{Timeout: 30 * time.Second},
		encryptionWarningOnce: sync.Once{},
		configPath:            configPath,
		restartChan:           restartChan,
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
	
	// DEBUG: Log what we're sending
	fmt.Printf("DEBUG: Sending to URL: %s\n", url)
	fmt.Printf("DEBUG: Organization ID: %s\n", s.organizationID)
	fmt.Printf("DEBUG: Server ID: %s\n", s.serverID)
	fmt.Printf("DEBUG: Application Token: %s\n", s.applicationToken)
	fmt.Printf("DEBUG: Is System Info: %v\n", isSystemInfo)

	// Prepare request body
	requestBody := map[string]interface{}{
		"machine_name": s.machineName,
		"metrics":      metrics,
	}
	
	// DEBUG: Log the request body
	requestBodyJSON, _ := json.MarshalIndent(requestBody, "", "  ")
	fmt.Printf("DEBUG: Request body: %s\n", string(requestBodyJSON))

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
	req.Header.Set("User-Agent", "Monitorly-Probe/v1.0.0")
	if isCompressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	s.checkConfigUpdate(resp)

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
		fallbackReq.Header.Set("User-Agent", "Monitorly-Probe/v1.0.0")
		if fallbackIsCompressed {
			fallbackReq.Header.Set("Content-Encoding", "gzip")
		}

		// Send fallback request
		resp, err = s.client.Do(fallbackReq)
		if err != nil {
			return fmt.Errorf("failed to send fallback request: %w", err)
		}
		defer resp.Body.Close()

		s.checkConfigUpdate(resp)
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// DEBUG: Log the response
		responseBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("DEBUG: Response status: %d\n", resp.StatusCode)
		fmt.Printf("DEBUG: Response body: %s\n", string(responseBody))
		
		switch resp.StatusCode {
		case http.StatusNotFound: // 404
			return fmt.Errorf("FATAL: API request failed with status 404 - Organization or server not found")
		case http.StatusUnauthorized: // 401
			return fmt.Errorf("FATAL: API request failed with status 401 - Invalid application token")
		case http.StatusRequestEntityTooLarge: // 413
			return fmt.Errorf("WARNING: API request failed with status 413 - Too many metrics for your plan, some metrics were ignored")
		case http.StatusTooManyRequests: // 429
			// Check for rate limit header
			if rateLimitHeader := resp.Header.Get("X-Rate-Limit"); rateLimitHeader != "" {
				// Try to parse the rate limit as seconds
				if s.configPath != "" && s.restartChan != nil {
					if err := s.updateSendIntervalInConfig(rateLimitHeader); err != nil {
						logger.GetDefaultLogger().Printf("Failed to update send interval in config: %v", err)
					}
				}
				return fmt.Errorf("WARNING: API request failed with status 429 - Rate limit exceeded, recommended interval: %s seconds", rateLimitHeader)
			}
			return fmt.Errorf("WARNING: API request failed with status 429 - Rate limit exceeded")
		case http.StatusServiceUnavailable: // 503
			return fmt.Errorf("WARNING: API request failed with status 503 - Server is undergoing maintenance, metrics will be buffered")
		default:
			return fmt.Errorf("API request failed with status %d", resp.StatusCode)
		}
	}

	return nil
}

// checkConfigUpdate checks the X-Configuration-Last-Update header and updates config if needed
func (s *APISender) checkConfigUpdate(resp *http.Response) {
	header := resp.Header.Get("X-Configuration-Last-Update")
	if header == "" || s.configPath == "" || s.restartChan == nil {
		return
	}
	serverTime, err := parseConfigTimestamp(header)
	if err != nil {
		logger.GetDefaultLogger().Printf("Invalid X-Configuration-Last-Update header: %v", err)
		return
	}
	fileInfo, err := os.Stat(s.configPath)
	if err != nil {
		logger.GetDefaultLogger().Printf("Could not stat config file: %v", err)
		return
	}
	if !serverTime.After(fileInfo.ModTime()) {
		return
	}
	// Fetch new config
	url := strings.TrimRight(s.baseURL, "/") + "/api/" + s.organizationID + "/servers/" + s.serverID + "/config"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.GetDefaultLogger().Printf("Failed to create config fetch request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.applicationToken)
	req.Header.Set("User-Agent", "Monitorly-Probe/v1.0.0")
	resp2, err := s.client.Do(req)
	if err != nil {
		logger.GetDefaultLogger().Printf("Failed to fetch latest config: %v", err)
		return
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		logger.GetDefaultLogger().Printf("Failed to fetch config: status %d", resp2.StatusCode)
		return
	}
	data, err := io.ReadAll(resp2.Body)
	if err != nil {
		logger.GetDefaultLogger().Printf("Failed to read config body: %v", err)
		return
	}
	err = os.WriteFile(s.configPath, data, 0644)
	if err != nil {
		logger.GetDefaultLogger().Printf("Failed to write new config: %v", err)
		return
	}
	logger.GetDefaultLogger().Printf("Config updated from server, triggering restart...")
	select {
	case s.restartChan <- struct{}{}:
	default:
	}
}

// parseConfigTimestamp parses the config update timestamp from header
func parseConfigTimestamp(ts string) (time.Time, error) {
	// Try RFC3339 and fallback to Unix
	t, err := time.Parse(time.RFC3339, ts)
	if err == nil {
		return t, nil
	}
	// Try as Unix timestamp
	if unix, err2 := time.ParseInLocation("2006-01-02 15:04:05", ts, time.UTC); err2 == nil {
		return unix, nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp format: %s", ts)
}

// updateSendIntervalInConfig updates the send_interval in the config file based on the rate limit
func (s *APISender) updateSendIntervalInConfig(rateLimitStr string) error {
	// Parse the rate limit as seconds to validate the format
	_, err := time.ParseDuration(rateLimitStr + "s")
	if err != nil {
		return fmt.Errorf("invalid rate limit format: %w", err)
	}

	// Read the current config file
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Convert to string for easier manipulation
	configStr := string(data)

	// Look for the send_interval line in the sender section
	senderSectionFound := false
	sendIntervalFound := false
	lines := strings.Split(configStr, "\n")

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "sender:" {
			senderSectionFound = true
			continue
		}

		if senderSectionFound && strings.Contains(trimmedLine, "send_interval:") {
			// Preserve the original indentation and replace only the value
			indentMatch := strings.Index(line, "send_interval:")
			if indentMatch >= 0 {
				indent := line[:indentMatch]
				lines[i] = indent + "send_interval: " + rateLimitStr + "s"
				sendIntervalFound = true
				break
			}
		}

		// If we've moved past the sender section (line with no indentation and not empty), stop looking
		if senderSectionFound && len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmedLine != "" {
			break
		}
	}

	// If we didn't find the send_interval line, log a warning
	if !sendIntervalFound {
		return fmt.Errorf("could not find send_interval in config file")
	}

	// Write the updated config back to the file
	err = os.WriteFile(s.configPath, []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated config: %w", err)
	}

	// Signal a restart to apply the new config
	logger.GetDefaultLogger().Printf("Updated send_interval in config to %s, triggering restart...", rateLimitStr+"s")
	select {
	case s.restartChan <- struct{}{}:
	default:
	}

	return nil
}

// SendConfigValidation sends configuration to API for validation
func (s *APISender) SendConfigValidation(configPath string) error {
	// Read configuration file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Build API URL for config validation
	url := fmt.Sprintf("%s/api/%s/servers/%s/config", s.baseURL, s.organizationID, s.serverID)

	// Create request with YAML config in body
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(configData))
	if err != nil {
		return fmt.Errorf("failed to create config validation request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-yaml")
	req.Header.Set("Authorization", "Bearer "+s.applicationToken)
	req.Header.Set("User-Agent", "Monitorly-Probe/v1.0.0")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send config validation request: %w", err)
	}
	defer resp.Body.Close()

	// Handle different response codes
	switch resp.StatusCode {
	case 200:
		// Configuration is valid - log and return success for restart
		logger.GetDefaultLogger().Printf("Configuration validated successfully by API")
		return nil

	case 205:
		// API made changes - update local config and restart
		logger.GetDefaultLogger().Printf("Warning: API has made changes to the configuration")

		// Read the updated configuration from response
		updatedConfig, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read updated config from API: %w", err)
		}

		// Write updated configuration to file
		err = os.WriteFile(configPath, updatedConfig, 0644)
		if err != nil {
			return fmt.Errorf("failed to write updated config: %w", err)
		}

		logger.GetDefaultLogger().Printf("Configuration updated with API changes, restarting...")
		return nil

	case 422:
		// Configuration is invalid - read error details and return fatal error
		bodyData, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.GetDefaultLogger().Fatalf("FATAL: Configuration is invalid (status 422) - unable to read error details")
		}

		var errorResponse struct {
			Error   string      `json:"error"`
			Details interface{} `json:"details"`
		}

		if err := json.Unmarshal(bodyData, &errorResponse); err != nil {
			logger.GetDefaultLogger().Fatalf("FATAL: Configuration is invalid (status 422) - %s", string(bodyData))
		} else {
			logger.GetDefaultLogger().Fatalf("FATAL: Configuration is invalid - %s: %v", errorResponse.Error, errorResponse.Details)
		}
		return fmt.Errorf("configuration validation failed")

	case 401:
		return fmt.Errorf("FATAL: Invalid authentication for config validation (status 401)")

	case 404:
		return fmt.Errorf("FATAL: Organization or server not found for config validation (status 404)")

	default:
		bodyData, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("config validation failed with status %d: %s", resp.StatusCode, string(bodyData))
	}
}
