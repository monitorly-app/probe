package config

import (
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

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

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
		cfg.LogFile.Path = "metrics.log"
	}
	if cfg.Logging.FilePath == "" {
		cfg.Logging.FilePath = "monitorly.log"
	}

	return &cfg, nil
}
