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

// CommandLineFlags holds all command-line flag values
type CommandLineFlags struct {
	ConfigPath      string
	ShowVersion     bool
	CheckUpdate     bool
	SkipUpdateCheck bool
	ForceUpdate     bool
}

// parseCommandLineFlags parses command-line arguments and returns flag values
func parseCommandLineFlags() *CommandLineFlags {
	flags := &CommandLineFlags{}
	flag.StringVar(&flags.ConfigPath, "config", "config.yaml", "Path to the configuration file")
	flag.BoolVar(&flags.ShowVersion, "version", false, "Show version information and exit")
	flag.BoolVar(&flags.CheckUpdate, "check-update", false, "Check for updates and exit")
	flag.BoolVar(&flags.SkipUpdateCheck, "skip-update-check", false, "Skip update check at startup")
	flag.BoolVar(&flags.ForceUpdate, "update", false, "Check for updates and update if available")
	flag.Parse()
	return flags
}

// handleVersionFlag handles the --version flag
func handleVersionFlag() {
	fmt.Println(version.Info())
}

// handleCheckUpdateFlag handles the --check-update flag
func handleCheckUpdateFlag() error {
	updateAvailable, latestVersion, err := version.CheckForUpdates()
	if err != nil {
		return fmt.Errorf("error checking for updates: %w", err)
	}

	if updateAvailable {
		fmt.Printf("Update available: %s (current: %s)\n", latestVersion, version.GetVersion())
		fmt.Println("Run with --update to automatically update")
	} else {
		fmt.Println("No updates available, you are running the latest version")
	}
	return nil
}

// handleForceUpdateFlag handles the --update flag
func handleForceUpdateFlag() error {
	fmt.Println("Checking for updates...")
	updateAvailable, latestVersion, err := version.CheckForUpdates()
	if err != nil {
		return fmt.Errorf("error checking for updates: %w", err)
	}

	if updateAvailable {
		fmt.Printf("Update available: %s (current: %s). Updating...\n", latestVersion, version.GetVersion())
		if err := version.SelfUpdate(); err != nil {
			return fmt.Errorf("error updating: %w", err)
		}
		fmt.Println("Update successful. Please restart the application.")
		os.Exit(0)
	} else {
		fmt.Println("No updates available, you are running the latest version")
	}
	return nil
}

// performStartupUpdateCheck performs the automatic update check at startup
func performStartupUpdateCheck() {
	log.Println("Checking for updates...")
	updateAvailable, latestVersion, err := version.CheckForUpdates()
	if err != nil {
		log.Printf("Error checking for updates: %v", err)
	} else if updateAvailable {
		log.Printf("Update available: %s (current: %s). Updating...", latestVersion, version.GetVersion())
		if err := version.SelfUpdate(); err != nil {
			log.Printf("Error updating: %v", err)
		} else {
			log.Println("Update successful. Restarting...")
			// Restart the application - just exit with success code
			// and let the service manager restart the application
			os.Exit(0)
		}
	} else {
		log.Println("No updates available")
	}
}

// setupSignalHandling sets up graceful shutdown signal handling
func setupSignalHandling(ctx context.Context, cancel context.CancelFunc) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		log.Printf("Received signal: %v. Shutting down...", sig)
		cancel()
	}()
}

// setupConfigWatcher creates and configures a file system watcher for the config file
func setupConfigWatcher(configPath string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch config directory: %w", err)
	}

	return watcher, nil
}

// startUpdateChecker starts the automatic update checker if enabled in config
func startUpdateChecker(ctx context.Context, cfg *config.Config) {
	if !cfg.Updates.Enabled {
		return
	}

	nextCheck, err := cfg.GetUpdateCheckTime()
	if err != nil {
		log.Printf("Error parsing update check time: %v, using default (midnight)", err)
		nextCheck = time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour) // Next midnight
	}
	retryDelay := cfg.GetUpdateRetryDelay()
	log.Printf("Automatic updates enabled, next check at %s", nextCheck.Format("2006-01-02 15:04:05"))
	version.StartUpdateChecker(ctx, nextCheck, retryDelay)
}

// runMainLoop runs the main application loop with config reloading
func runMainLoop(ctx context.Context, configPath string, initialConfig *config.Config, restartChan <-chan struct{}) {
	cfg := initialConfig

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
			newCfg, err := loadConfig(configPath)
			if err != nil {
				log.Printf("Error loading new configuration: %v, continuing with old config", err)
				continue
			}
			cfg = newCfg
		}
	}
}

// runApplication is the main application logic, extracted from main() for testability
func runApplication(flags *CommandLineFlags) error {
	// Handle version flag
	if flags.ShowVersion {
		handleVersionFlag()
		return nil
	}

	// Handle check-update flag
	if flags.CheckUpdate {
		return handleCheckUpdateFlag()
	}

	// Handle force update flag
	if flags.ForceUpdate {
		return handleForceUpdateFlag()
	}

	log.Printf("Starting %s", version.Info())

	// Check for updates at startup, unless skipped
	if !flags.SkipUpdateCheck {
		performStartupUpdateCheck()
	}

	// Find the config file
	absConfigPath, err := findConfigFile(flags.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to find config file: %w", err)
	}

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	setupSignalHandling(ctx, cancel)

	// Create configuration watcher
	watcher, err := setupConfigWatcher(absConfigPath)
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Initialize configuration
	cfg, err := loadConfig(absConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set up a channel to restart the application on config changes
	restartChan := make(chan struct{})

	// Start config watcher goroutine
	go watchConfigFile(ctx, watcher, absConfigPath, restartChan)

	// Start update checker if enabled
	startUpdateChecker(ctx, cfg)

	// Run the main application loop
	runMainLoop(ctx, absConfigPath, cfg, restartChan)

	return nil
}

func main() {
	flags := parseCommandLineFlags()

	if err := runApplication(flags); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

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

	// Initialize sender based on configuration
	var metricSender sender.Sender

	switch cfg.Sender.Target {
	case "api":
		metricSender = sender.NewAPISender(
			cfg.API.URL,
			cfg.API.OrganizationID,
			cfg.API.ServerID,
			cfg.API.ApplicationToken,
			machineName,
			cfg.API.EncryptionKey,
		)
		logger.Printf("Metrics will be sent to API: %s for organization: %s", cfg.API.URL, cfg.API.OrganizationID)
		if cfg.API.EncryptionKey != "" {
			logger.Printf("Encryption enabled for API communication")
		}
	case "log_file":
		metricSender = sender.NewFileLogger(cfg.LogFile.Path)
		logger.Printf("Metrics will be logged to file: %s", cfg.LogFile.Path)
	default:
		logger.Fatalf("Unknown sender target: %s", cfg.Sender.Target)
	}

	// Send initial system information
	systemInfoCollector := system.NewSystemInfoCollector()
	systemInfo, err := systemInfoCollector.Collect()
	if err != nil {
		logger.Printf("Warning: Failed to collect system information: %v", err)
	} else {
		if err := metricSender.Send(systemInfo); err != nil {
			logger.Printf("Warning: Failed to send system information: %v", err)
		} else {
			logger.Printf("Initial system information sent successfully")
		}
	}

	// Channel for collected metrics
	metricsChan := make(chan []collector.Metrics, 100)

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

	if cfg.Collection.Service.Enabled {
		wg.Add(1)
		serviceCollector := system.NewServiceCollector(cfg.Collection.Service.Services)
		go func() {
			defer wg.Done()
			collectRoutine(ctx, "Service", serviceCollector, metricsChan, cfg.Collection.Service.Interval)
		}()
		logger.Printf("Service collector started with interval: %v", cfg.Collection.Service.Interval)
	}

	if cfg.Collection.UserActivity.Enabled {
		wg.Add(1)
		userActivityCollector := system.NewUserActivityCollector()
		go func() {
			defer wg.Done()
			collectRoutine(ctx, "UserActivity", userActivityCollector, metricsChan, cfg.Collection.UserActivity.Interval)
		}()
		logger.Printf("User activity collector started with interval: %v", cfg.Collection.UserActivity.Interval)
	}

	if cfg.Collection.LoginFailures.Enabled {
		wg.Add(1)
		loginFailuresCollector := system.NewLoginFailuresCollector()
		go func() {
			defer wg.Done()
			collectRoutine(ctx, "LoginFailures", loginFailuresCollector, metricsChan, cfg.Collection.LoginFailures.Interval)
		}()
		logger.Printf("Login failures collector started with interval: %v", cfg.Collection.LoginFailures.Interval)
	}

	if cfg.Collection.Port.Enabled {
		wg.Add(1)
		portCollector := system.NewPortCollector()
		go func() {
			defer wg.Done()
			collectRoutine(ctx, "Port", portCollector, metricsChan, cfg.Collection.Port.Interval)
		}()
		logger.Printf("Port monitoring collector started with interval: %v", cfg.Collection.Port.Interval)
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
