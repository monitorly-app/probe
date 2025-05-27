package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	goversion "github.com/hashicorp/go-version"
)

var (
	// DefaultTimeout is the default timeout for HTTP requests
	DefaultTimeout = 10 * time.Second

	// GitHubAPIReleaseURL is the GitHub API URL for releases
	GitHubAPIReleaseURL = "https://api.github.com/repos/monitorly-app/probe/releases/latest"

	// osExecutable is a variable to allow mocking os.Executable in tests
	osExecutable = os.Executable

	// getOS and getArch are variables to allow mocking runtime.GOOS and runtime.GOARCH in tests
	getOS   = func() string { return runtime.GOOS }
	getArch = func() string { return runtime.GOARCH }

	// osExit is a variable to allow mocking os.Exit in tests
	osExit = os.Exit

	// updateCheckInterval is the interval between update checks
	updateCheckInterval = 24 * time.Hour

	// updateRetryDelay is the delay between retry attempts
	updateRetryDelay = time.Minute

	// downloadBinaryFunc is a variable to allow mocking downloadBinary in tests
	downloadBinaryFunc = downloadBinary

	// replaceBinaryFunc is a variable to allow mocking replaceBinary in tests
	replaceBinaryFunc = replaceBinary
)

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckForUpdates checks if a new version is available on GitHub
// Returns true if an update is available, false otherwise
func CheckForUpdates() (bool, string, error) {
	latestVersion, err := GetLatestVersionFromGitHub()
	if err != nil {
		return false, "", fmt.Errorf("failed to get latest version: %w", err)
	}

	// Parse versions using hashicorp/go-version
	localVer, err := goversion.NewVersion(strings.TrimPrefix(Version, "v"))
	if err != nil {
		return false, latestVersion, nil // Invalid local version, assume no update needed
	}

	remoteVer, err := goversion.NewVersion(strings.TrimPrefix(latestVersion, "v"))
	if err != nil {
		return false, latestVersion, nil // Invalid remote version, assume no update needed
	}

	return remoteVer.GreaterThan(localVer), latestVersion, nil
}

// GetLatestVersionFromGitHub fetches the latest version from GitHub
func GetLatestVersionFromGitHub() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubAPIReleaseURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Monitorly-Probe/"+Version)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Return the tag name (version)
	return release.TagName, nil
}

// SelfUpdate updates the application to the latest version
func SelfUpdate() error {
	updateAvailable, _, err := CheckForUpdates()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !updateAvailable {
		return nil // No update needed
	}

	// Get the latest release information
	release, err := getLatestReleaseInfo()
	if err != nil {
		return fmt.Errorf("failed to get release info: %w", err)
	}

	// Find the appropriate asset for the current OS and architecture
	assetURL, err := findAppropriateAsset(release)
	if err != nil {
		return fmt.Errorf("failed to find appropriate asset: %w", err)
	}

	// Download the new binary
	newBinaryPath, err := downloadBinaryFunc(assetURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Replace the current binary
	if err := replaceBinaryFunc(newBinaryPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

// getLatestReleaseInfo fetches detailed information about the latest release
func getLatestReleaseInfo() (*GitHubRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubAPIReleaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Monitorly-Probe/"+Version)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &release, nil
}

// findAppropriateAsset finds the asset for the current OS and architecture
func findAppropriateAsset(release *GitHubRelease) (string, error) {
	// We only build for Linux
	if getOS() != "linux" {
		return "", fmt.Errorf("this application only runs on Linux, current OS: %s", getOS())
	}

	// Expected naming pattern: monitorly-probe-{version}-linux-{arch}
	// Example: monitorly-probe-1.0.0-linux-amd64
	expectedPattern := fmt.Sprintf("linux-%s", getArch())

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, expectedPattern) {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("no matching asset found for linux/%s", getArch())
}

// downloadBinary downloads the binary from the given URL
func downloadBinary(url string) (string, error) {
	return downloadBinaryWithTimeout(url, 5*time.Minute)
}

// downloadBinaryWithTimeout downloads the binary from the given URL with a specified timeout
func downloadBinaryWithTimeout(url string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Monitorly-Probe/"+Version)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Check for empty response
	if resp.ContentLength == 0 {
		return "", fmt.Errorf("empty response body")
	}

	// Create a temporary file to store the binary
	tmpFile, err := os.CreateTemp("", "monitorly-probe-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tmpFile.Close()

	// Copy the response body to the temporary file
	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name()) // Clean up on error
		return "", fmt.Errorf("failed to write binary: %w", err)
	}

	// Double-check that we actually wrote some data
	if written == 0 {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("no data written to file")
	}

	return tmpFile.Name(), nil
}

// replaceBinary replaces the current binary with the new one
func replaceBinary(newBinaryPath string) error {
	// Get the path to the current executable
	execPath, err := osExecutable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve any symlinks to get the real path
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// On Windows, can't replace a running executable directly
	// so we rename the current binary and copy the new one
	if runtime.GOOS == "windows" {
		oldPath := execPath + ".old"
		// Remove old backup if it exists
		_ = os.Remove(oldPath)

		// Rename current executable to .old
		if err := os.Rename(execPath, oldPath); err != nil {
			return fmt.Errorf("failed to rename current executable: %w", err)
		}
	}

	// Copy the new binary to the executable path
	sourceFile, err := os.Open(newBinaryPath)
	if err != nil {
		return fmt.Errorf("failed to open new binary: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(execPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy new binary: %w", err)
	}

	// Make sure the new binary is executable
	if err := os.Chmod(execPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Clean up temporary file
	_ = os.Remove(newBinaryPath)

	return nil
}

// StartUpdateChecker starts a goroutine that checks for updates at the specified time each day
func StartUpdateChecker(ctx context.Context, nextCheck time.Time, retryDelay time.Duration) {
	go func() {
		for {
			// Wait until the next check time
			waitDuration := time.Until(nextCheck)
			select {
			case <-ctx.Done():
				return
			case <-time.After(waitDuration):
				// Check for updates
				updateAvailable, latestVersion, err := CheckForUpdates()
				if err != nil {
					log.Printf("Error checking for updates: %v, will retry in %v", err, retryDelay)
					// Schedule retry
					nextCheck = time.Now().Add(retryDelay)
					continue
				}

				if updateAvailable {
					log.Printf("Update available: %s (current: %s). Updating...", latestVersion, GetVersion())
					if err := SelfUpdate(); err != nil {
						log.Printf("Error updating: %v, will retry in %v", err, retryDelay)
						// Schedule retry
						nextCheck = time.Now().Add(retryDelay)
						continue
					}
					log.Println("Update successful. Restarting...")
					// Exit with success code to allow service manager to restart
					osExit(0)
				} else {
					log.Println("No updates available")
				}

				// Schedule next check for tomorrow at the same time
				nextCheck = nextCheck.Add(24 * time.Hour)
			}
		}
	}()
}
