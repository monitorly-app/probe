package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/config"
	"github.com/monitorly-app/probe/internal/logger"
	"github.com/monitorly-app/probe/internal/sender"
)

func main() {
	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		logger.Printf("Received signal: %v. Shutting down...", sig)
		cancel()
	}()

	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		// Use standard log here since logger isn't initialized yet
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	if err := logger.Initialize(cfg.Logging.FilePath); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		}
	}()

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

	// Use WaitGroup to track goroutines
	var wg sync.WaitGroup
	wg.Add(2) // One for collector, one for sender

	// Start collection routine
	go func() {
		defer wg.Done()
		collectRoutine(ctx, systemCollector, metricsChan, cfg.Collection.Interval)
	}()

	// Start sender routine
	go func() {
		defer wg.Done()
		sendRoutine(ctx, metricSender, metricsChan, cfg.Sender.SendInterval)
	}()

	// Wait for context cancellation (from signal handler)
	<-ctx.Done()
	logger.Printf("Context canceled, shutting down gracefully...")

	// Wait for goroutines to finish with a timeout
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	// Add a timeout for shutdown
	select {
	case <-waitChan:
		logger.Printf("All goroutines finished")
	case <-time.After(5 * time.Second):
		logger.Printf("Timed out waiting for goroutines to finish")
	}

	logger.Printf("Monitorly probe shutdown complete")
}

func collectRoutine(ctx context.Context, collector collector.Collector, metricsChan chan collector.Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Printf("Collection routine shutting down")
			return
		case <-ticker.C:
			metrics, err := collector.Collect()
			if err != nil {
				logger.Printf("Error collecting metrics: %v", err)
				continue
			}

			select {
			case metricsChan <- metrics:
				logger.Printf("Collected metrics: CPU=%v%%, RAM=%v%%, Disk=%v%%",
					metrics.CPUUsage, metrics.RAMUsage, metrics.DiskUsage)
			case <-ctx.Done():
				return
			}
		}
	}
}

func sendRoutine(ctx context.Context, sender sender.Sender, metricsChan chan collector.Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var metrics []collector.Metrics

	// Collect metrics until it's time to send
	for {
		select {
		case <-ctx.Done():
			// Try to send any remaining metrics before shutting down
			if len(metrics) > 0 {
				if err := sender.Send(metrics); err != nil {
					logger.Printf("Error sending final metrics: %v", err)
				} else {
					logger.Printf("Sent %d final metrics", len(metrics))
				}
			}
			logger.Printf("Sender routine shutting down")
			return
		case m := <-metricsChan:
			metrics = append(metrics, m)
		case <-ticker.C:
			if len(metrics) > 0 {
				if err := sender.Send(metrics); err != nil {
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
