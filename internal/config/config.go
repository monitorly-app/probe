package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	MachineName string `yaml:"machine_name"` // Machine name used to differentiate metrics from different servers
	Collection  struct {
		CPU struct {
			Enabled  bool          `yaml:"enabled"`
			Interval time.Duration `yaml:"interval"`
		} `yaml:"cpu"`
		RAM struct {
			Enabled  bool          `yaml:"enabled"`
			Interval time.Duration `yaml:"interval"`
		} `yaml:"ram"`
		Disk struct {
			Enabled     bool          `yaml:"enabled"`
			Interval    time.Duration `yaml:"interval"`
			MountPoints []MountPoint  `yaml:"mount_points"`
		} `yaml:"disk"`
		Service struct {
			Enabled  bool          `yaml:"enabled"`
			Interval time.Duration `yaml:"interval"`
			Services []Service     `yaml:"services"`
		} `yaml:"service"`
		UserActivity struct {
			Enabled  bool          `yaml:"enabled"`
			Interval time.Duration `yaml:"interval"`
		} `yaml:"user_activity"`
		LoginFailures struct {
			Enabled  bool          `yaml:"enabled"`
			Interval time.Duration `yaml:"interval"`
		} `yaml:"login_failures"`
		Port struct {
			Enabled  bool          `yaml:"enabled"`
			Interval time.Duration `yaml:"interval"`
		} `yaml:"port"`
	} `yaml:"collection"`
	Sender struct {
		Target       string        `yaml:"target"`
		SendInterval time.Duration `yaml:"send_interval"`
	} `yaml:"sender"`
	API struct {
		URL              string `yaml:"url"`
		OrganizationID   string `yaml:"organization_id"`   // Organization ID (UUID) for API requests
		ServerID         string `yaml:"server_id"`         // Server ID (UUID) for API requests
		ApplicationToken string `yaml:"application_token"` // Application token for API authentication
		EncryptionKey    string `yaml:"encryption_key"`    // Optional: If set, encrypts the request body. Requires premium subscription.
	} `yaml:"api"`
	LogFile struct {
		Path string `yaml:"path"`
	} `yaml:"log_file"`
	Logging struct {
		FilePath string `yaml:"file_path"`
	} `yaml:"logging"`
	Updates struct {
		Enabled    bool          `yaml:"enabled"`
		CheckTime  string        `yaml:"check_time"`  // Time of day to check for updates (HH:MM format)
		RetryDelay time.Duration `yaml:"retry_delay"` // How long to wait before retrying after a failed update
	} `yaml:"updates"`
}

// MountPoint represents a disk mount point configuration
type MountPoint struct {
	Path           string `yaml:"path"`
	Label          string `yaml:"label"`
	CollectUsage   bool   `yaml:"collect_usage"`
	CollectPercent bool   `yaml:"collect_percent"`
}

// Service represents a system service to monitor
type Service struct {
	Name  string `yaml:"name"`  // Service name (e.g., "nginx", "postgresql")
	Label string `yaml:"label"` // User-friendly label for the service
}

// Collection holds the configuration for metric collection
type Collection struct {
	CPU struct {
		Enabled  bool          `yaml:"enabled"`
		Interval time.Duration `yaml:"interval"`
	} `yaml:"cpu"`
	RAM struct {
		Enabled  bool          `yaml:"enabled"`
		Interval time.Duration `yaml:"interval"`
	} `yaml:"ram"`
	Disk struct {
		Enabled     bool          `yaml:"enabled"`
		Interval    time.Duration `yaml:"interval"`
		MountPoints []MountPoint  `yaml:"mount_points"`
	} `yaml:"disk"`
	Service struct {
		Enabled  bool          `yaml:"enabled"`
		Interval time.Duration `yaml:"interval"`
		Services []Service     `yaml:"services"`
	} `yaml:"service"`
	UserActivity struct {
		Enabled  bool          `yaml:"enabled"`
		Interval time.Duration `yaml:"interval"`
	} `yaml:"user_activity"`
	LoginFailures struct {
		Enabled  bool          `yaml:"enabled"`
		Interval time.Duration `yaml:"interval"`
	} `yaml:"login_failures"`
	Port struct {
		Enabled  bool          `yaml:"enabled"`
		Interval time.Duration `yaml:"interval"`
	} `yaml:"port"`
}

// GetMachineName returns the configured machine name or the system hostname if not specified
func (c *Config) GetMachineName() (string, error) {
	if c.MachineName != "" {
		return c.MachineName, nil
	}

	// Fallback to system hostname if not specified
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown", fmt.Errorf("failed to get system hostname: %w", err)
	}

	return hostname, nil
}

// Load reads the configuration file from the given path and returns a Config
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyDefaults sets default values for configuration options that are not specified
func applyDefaults(cfg *Config) {
	// Set defaults for CPU collection
	cfg.Collection.CPU.Enabled = true
	if cfg.Collection.CPU.Interval == 0 {
		cfg.Collection.CPU.Interval = 30 * time.Second
	}

	// Set defaults for RAM collection
	cfg.Collection.RAM.Enabled = true
	if cfg.Collection.RAM.Interval == 0 {
		cfg.Collection.RAM.Interval = 30 * time.Second
	}

	// Set defaults for Disk collection
	cfg.Collection.Disk.Enabled = true
	if cfg.Collection.Disk.Interval == 0 {
		cfg.Collection.Disk.Interval = 60 * time.Second
	}

	// If no mount points are specified, add the root mount point
	if len(cfg.Collection.Disk.MountPoints) == 0 {
		cfg.Collection.Disk.MountPoints = []MountPoint{
			{
				Path:           "/",
				Label:          "root",
				CollectUsage:   true,
				CollectPercent: true,
			},
		}
	}

	// Set defaults for user activity collection
	cfg.Collection.UserActivity.Enabled = true
	if cfg.Collection.UserActivity.Interval == 0 {
		cfg.Collection.UserActivity.Interval = 1 * time.Minute
	}

	// Set defaults for login failures collection
	cfg.Collection.LoginFailures.Enabled = true
	if cfg.Collection.LoginFailures.Interval == 0 {
		cfg.Collection.LoginFailures.Interval = 1 * time.Minute
	}

	// Set defaults for port monitoring collection
	cfg.Collection.Port.Enabled = true
	if cfg.Collection.Port.Interval == 0 {
		cfg.Collection.Port.Interval = 1 * time.Minute
	}

	// Set defaults for sender
	if cfg.Sender.SendInterval == 0 {
		cfg.Sender.SendInterval = 5 * time.Minute
	}
	if cfg.Sender.Target == "" {
		cfg.Sender.Target = "api"
	}

	// Set defaults for log paths
	if cfg.LogFile.Path == "" {
		cfg.LogFile.Path = "logs/metrics.log"
	}
	if cfg.Logging.FilePath == "" {
		cfg.Logging.FilePath = "logs/monitorly.log"
	}
}

// validate performs validation on the configuration
func validate(cfg *Config) error {
	// Validate sender target
	switch cfg.Sender.Target {
	case "api":
		if cfg.API.URL == "" {
			return fmt.Errorf("API URL is required when sender target is set to 'api'")
		}
		if cfg.API.OrganizationID == "" {
			return fmt.Errorf("organization ID is required when sender target is set to 'api'")
		}
		if cfg.API.ServerID == "" {
			return fmt.Errorf("server ID is required when sender target is set to 'api'")
		}
		if cfg.API.ApplicationToken == "" {
			return fmt.Errorf("application token is required when sender target is set to 'api'")
		}
		if cfg.API.EncryptionKey != "" {
			if len(cfg.API.EncryptionKey) != 32 {
				return fmt.Errorf("encryption key must be exactly 32 bytes long")
			}
		}
	case "log_file":
		// No validation needed for log_file target
	default:
		return fmt.Errorf("invalid sender target: %s (must be 'api' or 'log_file')", cfg.Sender.Target)
	}

	// Validate collection intervals if enabled
	if cfg.Collection.CPU.Enabled && cfg.Collection.CPU.Interval < time.Second {
		return fmt.Errorf("CPU collection interval must be at least 1 second")
	}
	if cfg.Collection.RAM.Enabled && cfg.Collection.RAM.Interval < time.Second {
		return fmt.Errorf("RAM collection interval must be at least 1 second")
	}
	if cfg.Collection.Disk.Enabled && cfg.Collection.Disk.Interval < time.Second {
		return fmt.Errorf("Disk collection interval must be at least 1 second")
	}
	if cfg.Collection.UserActivity.Enabled && cfg.Collection.UserActivity.Interval < time.Second {
		return fmt.Errorf("User activity collection interval must be at least 1 second")
	}
	if cfg.Collection.LoginFailures.Enabled && cfg.Collection.LoginFailures.Interval < time.Second {
		return fmt.Errorf("Login failures collection interval must be at least 1 second")
	}
	if cfg.Collection.Port.Enabled && cfg.Collection.Port.Interval < time.Second {
		return fmt.Errorf("Port monitoring collection interval must be at least 1 second")
	}

	// Validate send interval
	if cfg.Sender.SendInterval < time.Second {
		return fmt.Errorf("send interval must be at least 1 second")
	}

	return nil
}

// GetUpdateCheckTime returns the time to check for updates, defaulting to midnight if not specified
func (c *Config) GetUpdateCheckTime() (time.Time, error) {
	if c.Updates.CheckTime == "" {
		c.Updates.CheckTime = "00:00" // Default to midnight
	}

	// Parse the time string
	now := time.Now()
	checkTime, err := time.Parse("15:04", c.Updates.CheckTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid check_time format (use HH:MM): %w", err)
	}

	// Create a time.Time for today at the specified time
	scheduledTime := time.Date(
		now.Year(), now.Month(), now.Day(),
		checkTime.Hour(), checkTime.Minute(), 0, 0,
		now.Location(),
	)

	// If the scheduled time has already passed today, schedule for tomorrow
	if scheduledTime.Before(now) {
		scheduledTime = scheduledTime.Add(24 * time.Hour)
	}

	return scheduledTime, nil
}

// GetUpdateRetryDelay returns the delay between update retries, defaulting to 1 hour
func (c *Config) GetUpdateRetryDelay() time.Duration {
	if c.Updates.RetryDelay == 0 {
		return time.Hour // Default to 1 hour
	}
	return c.Updates.RetryDelay
}
