package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/collector/system"
	"github.com/monitorly-app/probe/internal/config"
	"github.com/monitorly-app/probe/internal/logger"
	"github.com/monitorly-app/probe/internal/sender"
	"github.com/monitorly-app/probe/internal/version"
)

// searchPaths returns a list of locations to search for the config file
func searchPaths(configFlag string) []string {
	// If config flag is set, that's the primary location
	paths := []string{configFlag}

	// Common locations for the config file
	homeDir, err := os.UserHomeDir()
	if err == nil {
		homePath := filepath.Join(homeDir, ".monitorly", "config.yaml")
		paths = append(paths, homePath)
	}

	// Add other common locations
	paths = append(paths,
		"config.yaml",                // Current directory
		"configs/config.yaml",        // Common configs directory
		"/etc/monitorly/config.yaml", // System-wide config location
	)

	return paths
}

// findConfigFile tries to find a config file in common locations
func findConfigFile(configFlag string) (string, error) {
	paths := searchPaths(configFlag)

	// Try each path
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue // Skip this path if we can't get the absolute path
		}

		if _, err := os.Stat(absPath); err == nil {
			log.Printf("Using config file: %s", absPath)
			return absPath, nil
		}
	}

	// If an explicit config path was provided but not found, that's an error
	if configFlag != "config.yaml" {
		return "", fmt.Errorf("specified config file not found: %s", configFlag)
	}

	return "", fmt.Errorf("no config file found in search paths")
}

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	showVersion := flag.Bool("version", false, "Show version information and exit")
	flag.Parse()

	// If version flag is provided, print version and exit
	if *showVersion {
		fmt.Println(version.Info())
		return
	}

	log.Printf("Starting %s", version.Info())

	// Find the config file
	absConfigPath, err := findConfigFile(*configPath)
	if err != nil {
		log.Fatalf("Failed to find config file: %v", err)
	}

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		log.Printf("Received signal: %v. Shutting down...", sig)
		cancel()
	}()

	// Create configuration watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create file watcher: %v", err)
	}
	defer watcher.Close()

	// Add the config file directory to the watcher
	configDir := filepath.Dir(absConfigPath)
	if err := watcher.Add(configDir); err != nil {
		log.Fatalf("Failed to watch config directory: %v", err)
	}

	// Initialize configuration
	cfg, err := loadConfig(absConfigPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set up a channel to restart the application on config changes
	restartChan := make(chan struct{})

	// Start config watcher goroutine
	go watchConfigFile(ctx, watcher, absConfigPath, restartChan)

	// Main application loop
	for {
		// Start the application with the current config
		appCtx, appCancel := context.WithCancel(ctx)
		appWg := runApp(appCtx, cfg)

		// Wait for either a config change or application shutdown
		select {
		case <-ctx.Done():
			// Global shutdown requested
			appCancel()
			appWg.Wait()
			return
		case <-restartChan:
			// Config changed, reload and restart
			log.Println("Configuration changed, restarting...")
			appCancel()
			appWg.Wait()

			// Load the new configuration
			newCfg, err := loadConfig(absConfigPath)
			if err != nil {
				log.Printf("Error loading new configuration: %v, continuing with old config", err)
				continue
			}
			cfg = newCfg
		}
	}
}

// loadConfig loads the configuration from the specified path
func loadConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// watchConfigFile monitors the config file for changes
func watchConfigFile(ctx context.Context, watcher *fsnotify.Watcher, configPath string, restartChan chan struct{}) {
	configFileName := filepath.Base(configPath)
	configDir := filepath.Dir(configPath)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Check if this event is for our config file
			if filepath.Base(event.Name) == configFileName && filepath.Dir(event.Name) == configDir {
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					// Wait a short time to ensure the file is fully written
					time.Sleep(100 * time.Millisecond)

					// Signal a restart
					select {
					case restartChan <- struct{}{}:
						log.Printf("Detected change to config file: %s", configPath)
					default:
						// A restart is already pending, no need to send again
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Error watching config file: %v", err)
		}
	}
}

// runApp starts the application with the given configuration
func runApp(ctx context.Context, cfg *config.Config) *sync.WaitGroup {
	// Initialize logger
	if err := logger.Initialize(cfg.Logging.FilePath); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		}
	}()

	// Get the machine name for metrics
	machineName, err := cfg.GetMachineName()
	if err != nil {
		logger.Printf("Warning: Failed to get machine name: %v. Using 'unknown'", err)
		machineName = "unknown"
	}
	logger.Printf("Using machine name: %s", machineName)

	// Channel for collected metrics
	metricsChan := make(chan []collector.Metrics, 100)

	// Initialize sender based on configuration
	var metricSender sender.Sender

	switch cfg.Sender.Target {
	case "api":
		metricSender = sender.NewAPISender(
			cfg.API.URL,
			cfg.API.ProjectID,
			cfg.API.ApplicationToken,
			machineName,
		)
		logger.Printf("Metrics will be sent to API: %s for project: %s", cfg.API.URL, cfg.API.ProjectID)
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

	// Setup a goroutine to wait for the context to be done
	go func() {
		<-ctx.Done()
		logger.Printf("Context canceled, shutting down collectors and sender...")
	}()

	return &wg
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
