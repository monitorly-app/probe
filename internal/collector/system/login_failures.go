package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
)

// LoginFailuresCollector implements the collector.Collector interface for login failure metrics
type LoginFailuresCollector struct {
	lastCheck time.Time
}

// NewLoginFailuresCollector creates a new instance of LoginFailuresCollector
func NewLoginFailuresCollector() collector.Collector {
	return &LoginFailuresCollector{
		lastCheck: time.Now().Add(-1 * time.Minute), // Start from 1 minute ago
	}
}

// LoginFailure represents a failed login attempt
type LoginFailure struct {
	Timestamp time.Time `json:"timestamp"`
	Username  string    `json:"username"`
	SourceIP  string    `json:"source_ip"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
}

// Collect gathers login failure metrics by checking system logs since the last collection
func (c *LoginFailuresCollector) Collect() ([]collector.Metrics, error) {
	metrics := make([]collector.Metrics, 0, 1)
	now := time.Now()

	failures, err := c.getLoginFailuresSince(c.lastCheck)
	if err != nil {
		return metrics, fmt.Errorf("failed to get login failures: %w", err)
	}

	// Update last check time
	c.lastCheck = now

	// Create a single metric with all login failures since last check
	metrics = append(metrics, collector.Metrics{
		Timestamp: now,
		Category:  collector.CategorySystem,
		Name:      collector.NameLoginFailures,
		Value:     failures,
	})

	return metrics, nil
}

// getLoginFailuresSince retrieves login failures from system logs since the specified time
func (c *LoginFailuresCollector) getLoginFailuresSince(since time.Time) ([]LoginFailure, error) {
	var failures []LoginFailure

	// Try different log sources in order of preference
	logSources := []func(time.Time) ([]LoginFailure, error){
		c.getFailuresFromJournalctl,
		c.getFailuresFromAuthLog,
		c.getFailuresFromSecureLog,
	}

	for _, source := range logSources {
		sourceFailures, err := source(since)
		if err == nil && len(sourceFailures) >= 0 {
			// Successfully got data from this source, use it
			failures = sourceFailures
			break
		}
	}

	return failures, nil
}

// getFailuresFromJournalctl gets login failures from systemd journal (modern systems)
func (c *LoginFailuresCollector) getFailuresFromJournalctl(since time.Time) ([]LoginFailure, error) {
	// Format time for journalctl
	sinceStr := since.Format("2006-01-02 15:04:05")

	// Use journalctl to get authentication failures
	cmd := exec.Command("journalctl", "--since", sinceStr, "-u", "ssh", "-u", "sshd", "-u", "systemd-logind", "--no-pager", "-o", "short-iso")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute journalctl: %w", err)
	}

	return c.parseJournalctlOutput(string(output))
}

// getFailuresFromAuthLog gets login failures from /var/log/auth.log (Debian/Ubuntu)
func (c *LoginFailuresCollector) getFailuresFromAuthLog(since time.Time) ([]LoginFailure, error) {
	return c.parseLogFile("/var/log/auth.log", since)
}

// getFailuresFromSecureLog gets login failures from /var/log/secure (RHEL/CentOS)
func (c *LoginFailuresCollector) getFailuresFromSecureLog(since time.Time) ([]LoginFailure, error) {
	return c.parseLogFile("/var/log/secure", since)
}

// parseLogFile parses a traditional syslog file for login failures
func (c *LoginFailuresCollector) parseLogFile(logPath string, since time.Time) ([]LoginFailure, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}
	defer file.Close()

	var failures []LoginFailure
	scanner := bufio.NewScanner(file)

	// Patterns for different types of authentication failures
	patterns := []*regexp.Regexp{
		// SSH authentication failures
		regexp.MustCompile(`(\w+\s+\d+\s+\d+:\d+:\d+).*sshd.*Failed password for (?:invalid user )?(\w+) from ([\d\.]+)`),
		regexp.MustCompile(`(\w+\s+\d+\s+\d+:\d+:\d+).*sshd.*Invalid user (\w+) from ([\d\.]+)`),
		regexp.MustCompile(`(\w+\s+\d+\s+\d+:\d+:\d+).*sshd.*Connection closed by ([\d\.]+) port \d+ \[preauth\]`),
		// PAM authentication failures - specific patterns
		regexp.MustCompile(`(\w+\s+\d+\s+\d+:\d+:\d+).*pam.*authentication failure.*rhost=([\d\.]+).*user=(\w+)`),
		regexp.MustCompile(`(\w+\s+\d+\s+\d+:\d+:\d+).*pam.*authentication failure.*user=(\w+).*rhost=([\d\.]+)`),
		// Login failures
		regexp.MustCompile(`(\w+\s+\d+\s+\d+:\d+:\d+).*login.*FAILED LOGIN.*FROM ([\d\.]+).*FOR (\w+)`),
	}

	for scanner.Scan() {
		line := scanner.Text()

		for _, pattern := range patterns {
			matches := pattern.FindStringSubmatch(line)
			if len(matches) >= 3 {
				// Parse timestamp
				timestamp, err := c.parseLogTimestamp(matches[1])
				if err != nil || timestamp.Before(since) {
					continue
				}

				failure := LoginFailure{
					Timestamp: timestamp,
					Message:   line,
					Service:   c.extractService(line),
				}

				// Extract username and IP based on pattern
				if len(matches) >= 4 {
					if strings.Contains(line, "sshd") {
						failure.Username = matches[2]
						failure.SourceIP = matches[3]
					} else if strings.Contains(line, "pam") {
						// First pattern: rhost=IP user=USERNAME (matches[2]=IP, matches[3]=USERNAME)
						// Second pattern: user=USERNAME rhost=IP (matches[2]=USERNAME, matches[3]=IP)
						// Check which pattern matched by looking at the actual content
						if strings.Contains(matches[2], ".") && !strings.Contains(matches[3], ".") {
							// matches[2] looks like an IP, matches[3] looks like a username
							failure.SourceIP = matches[2]
							failure.Username = matches[3]
						} else {
							// matches[2] looks like a username, matches[3] looks like an IP
							failure.Username = matches[2]
							failure.SourceIP = matches[3]
						}
					} else if strings.Contains(line, "login") {
						failure.Username = matches[3]
						failure.SourceIP = matches[2]
					}
				} else if len(matches) == 3 {
					// Connection closed pattern
					failure.SourceIP = matches[2]
					failure.Username = "unknown"
				}

				failures = append(failures, failure)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return failures, nil
}

// parseJournalctlOutput parses journalctl output for login failures
func (c *LoginFailuresCollector) parseJournalctlOutput(output string) ([]LoginFailure, error) {
	var failures []LoginFailure
	scanner := bufio.NewScanner(strings.NewReader(output))

	// Patterns for journalctl output (ISO format timestamps)
	patterns := []*regexp.Regexp{
		// SSH authentication failures with ISO timestamp
		regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}).*sshd.*Failed password for (?:invalid user )?(\w+) from ([\d\.]+)`),
		regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}).*sshd.*Invalid user (\w+) from ([\d\.]+)`),
		regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}).*sshd.*Connection closed by ([\d\.]+) port \d+ \[preauth\]`),
		// PAM authentication failures - more specific patterns
		regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}).*pam.*authentication failure.*rhost=([\d\.]+).*user=(\w+)`),
		regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}).*pam.*authentication failure.*user=(\w+).*rhost=([\d\.]+)`),
	}

	for scanner.Scan() {
		line := scanner.Text()

		for _, pattern := range patterns {
			matches := pattern.FindStringSubmatch(line)
			if len(matches) >= 3 {
				// Parse ISO timestamp
				timestamp, err := time.Parse("2006-01-02T15:04:05-0700", matches[1])
				if err != nil {
					continue
				}

				failure := LoginFailure{
					Timestamp: timestamp,
					Message:   line,
					Service:   c.extractService(line),
				}

				// Extract username and IP based on pattern
				if len(matches) >= 4 {
					if strings.Contains(line, "sshd") {
						failure.Username = matches[2]
						failure.SourceIP = matches[3]
					} else if strings.Contains(line, "pam") {
						// First pattern: rhost=IP user=USERNAME (matches[2]=IP, matches[3]=USERNAME)
						// Second pattern: user=USERNAME rhost=IP (matches[2]=USERNAME, matches[3]=IP)
						// Check which pattern matched by looking at the actual content
						if strings.Contains(matches[2], ".") && !strings.Contains(matches[3], ".") {
							// matches[2] looks like an IP, matches[3] looks like a username
							failure.SourceIP = matches[2]
							failure.Username = matches[3]
						} else {
							// matches[2] looks like a username, matches[3] looks like an IP
							failure.Username = matches[2]
							failure.SourceIP = matches[3]
						}
					} else if strings.Contains(line, "login") {
						failure.Username = matches[3]
						failure.SourceIP = matches[2]
					}
				} else if len(matches) == 3 {
					// Connection closed pattern
					failure.SourceIP = matches[2]
					failure.Username = "unknown"
				}

				failures = append(failures, failure)
				break // Found a match, no need to try other patterns
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading journalctl output: %w", err)
	}

	return failures, nil
}

// parseLogTimestamp parses traditional syslog timestamp format
func (c *LoginFailuresCollector) parseLogTimestamp(timeStr string) (time.Time, error) {
	// Current year for syslog format (which doesn't include year)
	currentYear := time.Now().Year()

	// Try different timestamp formats
	formats := []string{
		"Jan 2 15:04:05",
		"Jan _2 15:04:05",
	}

	for _, format := range formats {
		fullTimeStr := fmt.Sprintf("%d %s", currentYear, timeStr)
		if t, err := time.Parse(fmt.Sprintf("2006 %s", format), fullTimeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timeStr)
}

// extractService extracts the service name from a log line
func (c *LoginFailuresCollector) extractService(line string) string {
	if strings.Contains(line, "sshd") {
		return "ssh"
	} else if strings.Contains(line, "systemd-logind") {
		return "systemd-logind"
	} else if strings.Contains(line, "pam") {
		return "pam"
	} else if strings.Contains(line, "login") {
		return "login"
	}
	return "unknown"
}
