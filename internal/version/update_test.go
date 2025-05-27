package version

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestCheckForUpdates_NewerVersionAvailable(t *testing.T) {
	// Setup a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a mock GitHub release response with a newer version
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"tag_name": "v2.0.0",
			"assets": [
				{
					"name": "monitorly-probe-2.0.0-linux-amd64",
					"browser_download_url": "https://example.com/download/monitorly-probe-2.0.0-linux-amd64"
				},
				{
					"name": "monitorly-probe-2.0.0-linux-arm64",
					"browser_download_url": "https://example.com/download/monitorly-probe-2.0.0-linux-arm64"
				}
			]
		}`))
	}))
	defer server.Close()

	// Save original URL and restore it after the test
	originalURL := GitHubAPIReleaseURL
	GitHubAPIReleaseURL = server.URL
	defer func() { GitHubAPIReleaseURL = originalURL }()

	// Set local version to an older version
	originalVersion := Version
	Version = "1.0.0"
	defer func() { Version = originalVersion }()

	// Test if update is available
	updateAvailable, latestVersion, err := CheckForUpdates()
	if err != nil {
		t.Fatalf("CheckForUpdates failed: %v", err)
	}

	if !updateAvailable {
		t.Errorf("Expected update to be available, but it wasn't")
	}

	if latestVersion != "v2.0.0" {
		t.Errorf("Expected latest version to be v2.0.0, got %s", latestVersion)
	}
}

func TestCheckForUpdates_NoNewerVersion(t *testing.T) {
	// Setup a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a mock GitHub release response with the same version
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"tag_name": "v1.0.0",
			"assets": [
				{
					"name": "monitorly-probe-1.0.0-linux-amd64",
					"browser_download_url": "https://example.com/download/monitorly-probe-1.0.0-linux-amd64"
				},
				{
					"name": "monitorly-probe-1.0.0-linux-arm64",
					"browser_download_url": "https://example.com/download/monitorly-probe-1.0.0-linux-arm64"
				}
			]
		}`))
	}))
	defer server.Close()

	// Save original URL and restore it after the test
	originalURL := GitHubAPIReleaseURL
	GitHubAPIReleaseURL = server.URL
	defer func() { GitHubAPIReleaseURL = originalURL }()

	// Set local version to the same version
	originalVersion := Version
	Version = "1.0.0"
	defer func() { Version = originalVersion }()

	// Test if update is available
	updateAvailable, latestVersion, err := CheckForUpdates()
	if err != nil {
		t.Fatalf("CheckForUpdates failed: %v", err)
	}

	if updateAvailable {
		t.Errorf("Expected no update to be available, but one was")
	}

	if latestVersion != "v1.0.0" {
		t.Errorf("Expected latest version to be v1.0.0, got %s", latestVersion)
	}
}

func TestFindAppropriateAsset(t *testing.T) {
	// Skip test if not running on Linux
	if runtime.GOOS != "linux" {
		t.Skip("This test only runs on Linux as we only build Linux binaries")
	}

	// Get current architecture
	arch := runtime.GOARCH

	// Create a mock release with assets for different platforms
	release := &GitHubRelease{
		TagName: "v2.0.0",
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{
				Name:               "monitorly-probe-2.0.0-linux-amd64",
				BrowserDownloadURL: "https://example.com/download/monitorly-probe-2.0.0-linux-amd64",
			},
			{
				Name:               "monitorly-probe-2.0.0-linux-arm64",
				BrowserDownloadURL: "https://example.com/download/monitorly-probe-2.0.0-linux-arm64",
			},
		},
	}

	// Test finding the asset for the current platform
	assetURL, err := findAppropriateAsset(release)
	if err != nil {
		t.Fatalf("findAppropriateAsset failed: %v", err)
	}

	// The URL should contain linux and the current architecture
	if !strings.Contains(assetURL, "linux") || !strings.Contains(assetURL, arch) {
		t.Errorf("Expected URL to contain linux and %s, but got %s", arch, assetURL)
	}
}
