package system

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

// UserActivityCollector implements the collector.Collector interface for user activity metrics
type UserActivityCollector struct{}

// NewUserActivityCollector creates a new instance of UserActivityCollector
func NewUserActivityCollector() collector.Collector {
	return &UserActivityCollector{}
}

// UserSession represents an active user session
type UserSession struct {
	Username  string `json:"username"`
	Terminal  string `json:"terminal"`
	LoginIP   string `json:"login_ip"`
	LoginTime string `json:"login_time"`
}

// Collect gathers user activity metrics by listing active sessions
func (c *UserActivityCollector) Collect() ([]collector.Metrics, error) {
	metrics := make([]collector.Metrics, 0, 1)
	now := time.Now()

	sessions, err := c.getActiveSessions()
	if err != nil {
		return metrics, fmt.Errorf("failed to get active sessions: %w", err)
	}

	// Create a single metric with all active sessions
	metrics = append(metrics, collector.Metrics{
		Timestamp: now,
		Category:  collector.CategorySystem,
		Name:      collector.NameUserActivity,
		Value:     sessions,
	})

	return metrics, nil
}

// getActiveSessions retrieves active user sessions using the 'who' command
func (c *UserActivityCollector) getActiveSessions() ([]UserSession, error) {
	// Use 'who' command to get active sessions
	cmd := exec.Command("who")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute 'who' command: %w", err)
	}

	return c.parseWhoOutput(string(output))
}

// parseWhoOutput parses the output of the 'who' command
func (c *UserActivityCollector) parseWhoOutput(output string) ([]UserSession, error) {
	var sessions []UserSession
	scanner := bufio.NewScanner(strings.NewReader(output))

	// Regular expression to parse 'who' output
	// Format: username terminal login_time (ip_address)
	// Example: user1    pts/0        2024-01-15 10:30 (192.168.1.100)
	whoRegex := regexp.MustCompile(`^(\S+)\s+(\S+)\s+(.+?)\s*(?:\(([^)]+)\))?$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := whoRegex.FindStringSubmatch(line)
		if len(matches) >= 4 {
			username := matches[1]
			terminal := matches[2]
			loginTime := strings.TrimSpace(matches[3])
			loginIP := ""

			// Extract IP address if present (in parentheses)
			if len(matches) > 4 && matches[4] != "" {
				loginIP = matches[4]
			} else {
				// If no IP in parentheses, check if it's at the end of login time
				parts := strings.Fields(loginTime)
				if len(parts) > 0 {
					lastPart := parts[len(parts)-1]
					// Check if the last part looks like an IP address
					if c.isIPAddress(lastPart) {
						loginIP = lastPart
						// Remove IP from login time
						loginTime = strings.Join(parts[:len(parts)-1], " ")
					}
				}
			}

			// If still no IP found, try to get it from 'w' command for this user
			if loginIP == "" {
				loginIP = c.getIPFromWCommand(username, terminal)
			}

			sessions = append(sessions, UserSession{
				Username:  username,
				Terminal:  terminal,
				LoginIP:   loginIP,
				LoginTime: loginTime,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading who output: %w", err)
	}

	return sessions, nil
}

// getIPFromWCommand tries to get IP address from 'w' command output
func (c *UserActivityCollector) getIPFromWCommand(username, terminal string) string {
	cmd := exec.Command("w", username)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, terminal) {
			// Look for IP address pattern in the line
			fields := strings.Fields(line)
			for _, field := range fields {
				if c.isIPAddress(field) {
					return field
				}
			}
		}
	}

	return ""
}

// isIPAddress checks if a string looks like an IP address
func (c *UserActivityCollector) isIPAddress(s string) bool {
	// Simple regex for IPv4 addresses
	ipv4Regex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if ipv4Regex.MatchString(s) {
		return true
	}

	// Simple check for IPv6 addresses (contains colons)
	if strings.Contains(s, ":") && len(s) > 2 {
		return true
	}

	return false
}
