package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// TODO : Add secret to encrypt the data packages we send to the API
type Config struct {
	Collection struct {
		Interval time.Duration `yaml:"interval"`
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
	// Set defaults if not specified
	if cfg.Collection.Interval == 0 {
		cfg.Collection.Interval = 30 * time.Second
	}
	if cfg.Sender.SendInterval == 0 {
		cfg.Sender.SendInterval = 5 * time.Minute
	}
	if cfg.Sender.Target == "" {
		cfg.Sender.Target = "api"
	}
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

	// Validate intervals
	if cfg.Collection.Interval < time.Second {
		return fmt.Errorf("collection interval must be at least 1 second")
	}
	if cfg.Sender.SendInterval < time.Second {
		return fmt.Errorf("send interval must be at least 1 second")
	}

	return nil
}
