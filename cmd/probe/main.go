package main

import (
	"log"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
	"github.com/monitorly-app/probe/internal/logger"
	"github.com/monitorly-app/probe/internal/sender"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		// Use standard log here since logger isn't initialized yet
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	if err := logger.Initialize(cfg.Logging.FilePath); err != nil {
		logger.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	// Initialize collectors
	systemCollector := collector.NewSystemCollector()

	// Initialize sender based on configuration
	var metricSender sender.Sender

	switch cfg.Sender.Target {
	case "api":
		metricSender = sender.NewAPISender(cfg.API.URL, cfg.API.Key)
		logger.Printf("Metrics will be sent to API: %s", cfg.API.URL)
	case "log_file":
		metricSender = sender.NewFileLogger(cfg.LogFile.Path)
		logger.Printf("Metrics will be logged to file: %s", cfg.LogFile.Path)
	default:
		logger.Fatalf("Unknown sender target: %s", cfg.Sender.Target)
	}

	// Channel for collected metrics
	metricsChan := make(chan collector.Metrics, 100)

	// Start collection routine
	go collectRoutine(systemCollector, metricsChan, cfg.Collection.Interval)

	// Start sender routine
	go sendRoutine(metricSender, metricsChan, cfg.Sender.SendInterval)

	// Keep the program running
	select {}
}

func collectRoutine(collector *collector.SystemCollector, metricsChan chan collector.Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		metrics, err := collector.Collect()
		if err != nil {
			logger.Printf("Error collecting metrics: %v", err)
			continue
		}

		metricsChan <- metrics
		logger.Printf("Collected metrics: CPU=%v%%, RAM=%v%%, Disk=%v%%",
			metrics.CPUUsage, metrics.RAMUsage, metrics.DiskUsage)
	}
}

func sendRoutine(sender sender.Sender, metricsChan chan collector.Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var metrics []collector.Metrics

	// Collect metrics until it's time to send
	for {
		select {
		case m := <-metricsChan:
			metrics = append(metrics, m)
		case <-ticker.C:
			if len(metrics) > 0 {
				err := sender.Send(metrics)
				if err != nil {
					logger.Printf("Error sending metrics: %v", err)
				} else {
					logger.Printf("Sent %d metrics", len(metrics))
					// Clear metrics after successful send
					metrics = []collector.Metrics{}
				}
			}
		}
	}
}
