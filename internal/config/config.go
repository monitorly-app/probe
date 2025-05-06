package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Collection struct {
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
	} `yaml:"collection"`
	Sender struct {
		Target       string        `yaml:"target"`
		SendInterval time.Duration `yaml:"send_interval"`
	} `yaml:"sender"`
	API struct {
		URL string `yaml:"url"`
		Key string `yaml:"key"`
	} `yaml:"api"`
	LogFile struct {
		Path string `yaml:"path"`
	} `yaml:"log_file"`
	Logging struct {
		FilePath string `yaml:"file_path"`
	} `yaml:"logging"`
}

// MountPoint represents a disk mount point configuration
type MountPoint struct {
	Path           string `yaml:"path"`
	Label          string `yaml:"label"`
	CollectUsage   bool   `yaml:"collect_usage"`
	CollectPercent bool   `yaml:"collect_percent"`
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
		if cfg.API.Key == "" {
			return fmt.Errorf("API key is required when sender target is set to 'api'")
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

	// Validate send interval
	if cfg.Sender.SendInterval < time.Second {
		return fmt.Errorf("send interval must be at least 1 second")
	}

	// Validate disk mount points
	for i, mp := range cfg.Collection.Disk.MountPoints {
		if mp.Path == "" {
			return fmt.Errorf("mount point #%d is missing a path", i+1)
		}
		if mp.Label == "" {
			return fmt.Errorf("mount point #%d is missing a label", i+1)
		}
		// Ensure at least one collection method is enabled
		if !mp.CollectUsage && !mp.CollectPercent {
			return fmt.Errorf("mount point %s must have at least one collection method enabled", mp.Path)
		}
	}

	return nil
}
