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
	"github.com/monitorly-app/probe/internal/collector/system"
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

	// Channel for collected metrics
	metricsChan := make(chan []collector.Metrics, 100)

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

	// Use WaitGroup to track goroutines
	var wg sync.WaitGroup

	// Start collectors based on configuration
	if cfg.Collection.CPU.Enabled {
		wg.Add(1)
		cpuCollector := system.NewCPUCollector()
		go func() {
			defer wg.Done()
			collectRoutine(ctx, "CPU", cpuCollector, metricsChan, cfg.Collection.CPU.Interval)
		}()
		logger.Printf("CPU collector started with interval: %v", cfg.Collection.CPU.Interval)
	}

	if cfg.Collection.RAM.Enabled {
		wg.Add(1)
		ramCollector := system.NewRAMCollector()
		go func() {
			defer wg.Done()
			collectRoutine(ctx, "RAM", ramCollector, metricsChan, cfg.Collection.RAM.Interval)
		}()
		logger.Printf("RAM collector started with interval: %v", cfg.Collection.RAM.Interval)
	}

	if cfg.Collection.Disk.Enabled {
		wg.Add(1)
		diskCollector := system.NewDiskCollector(cfg.Collection.Disk.MountPoints)
		go func() {
			defer wg.Done()
			collectRoutine(ctx, "Disk", diskCollector, metricsChan, cfg.Collection.Disk.Interval)
		}()
		logger.Printf("Disk collector started with interval: %v", cfg.Collection.Disk.Interval)
	}

	// Start sender routine
	wg.Add(1)
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

func collectRoutine(ctx context.Context, name string, collector collector.Collector, metricsChan chan []collector.Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Printf("%s collection routine shutting down", name)
			return
		case <-ticker.C:
			metrics, err := collector.Collect()
			if err != nil {
				logger.Printf("Error collecting %s metrics: %v", name, err)
				continue
			}

			if len(metrics) == 0 {
				continue
			}

			select {
			case metricsChan <- metrics:
				for _, m := range metrics {
					logMetric(name, m)
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

func logMetric(collectorName string, metric collector.Metrics) {
	var metadataStr string
	if len(metric.Metadata) > 0 {
		metadataStr = " metadata="
		for k, v := range metric.Metadata {
			metadataStr += k + "=" + v + " "
		}
	}

	logger.Printf("Collected %s metric: category=%s name=%s%s value=%v",
		collectorName, metric.Category, metric.Name, metadataStr, metric.Value)
}

func sendRoutine(ctx context.Context, metricSender sender.Sender, metricsChan chan []collector.Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var allMetrics []collector.Metrics

	// Collect metrics until it's time to send
	for {
		select {
		case <-ctx.Done():
			// Try to send any remaining metrics before shutting down
			if len(allMetrics) > 0 {
				if err := metricSender.Send(allMetrics); err != nil {
					logger.Printf("Error sending final metrics: %v", err)
				} else {
					logger.Printf("Sent %d final metrics", len(allMetrics))
				}
			}
			logger.Printf("Sender routine shutting down")
			return
		case metrics := <-metricsChan:
			allMetrics = append(allMetrics, metrics...)
		case <-ticker.C:
			if len(allMetrics) > 0 {
				if err := metricSender.Send(allMetrics); err != nil {
					logger.Printf("Error sending metrics: %v", err)
				} else {
					logger.Printf("Sent %d metrics", len(allMetrics))
					// Clear metrics after successful send
					allMetrics = []collector.Metrics{}
				}
			}
		}
	}
}
