package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// mockRuntime allows mocking runtime.GOOS and runtime.GOARCH for testing
type mockRuntime struct {
	goos   string
	goarch string
}

func (m *mockRuntime) GOOS() string {
	if m.goos != "" {
		return m.goos
	}
	return runtime.GOOS
}

func (m *mockRuntime) GOARCH() string {
	if m.goarch != "" {
		return m.goarch
	}
	return runtime.GOARCH
}

func TestCheckForUpdates(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		mockResponse   GitHubRelease
		mockStatus     int
		wantUpdate     bool
		wantVersion    string
		wantErr        bool
	}{
		{
			name:           "newer version available",
			currentVersion: "1.0.0",
			mockResponse: GitHubRelease{
				TagName: "v2.0.0",
			},
			mockStatus:  http.StatusOK,
			wantUpdate:  true,
			wantVersion: "v2.0.0",
		},
		{
			name:           "current version up to date",
			currentVersion: "2.0.0",
			mockResponse: GitHubRelease{
				TagName: "v2.0.0",
			},
			mockStatus:  http.StatusOK,
			wantUpdate:  false,
			wantVersion: "v2.0.0",
		},
		{
			name:           "server error",
			currentVersion: "1.0.0",
			mockStatus:     http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "invalid version format",
			currentVersion: "1.0.0",
			mockResponse: GitHubRelease{
				TagName: "invalid",
			},
			mockStatus:  http.StatusOK,
			wantUpdate:  false,
			wantVersion: "invalid",
		},
		{
			name:           "empty version response",
			currentVersion: "1.0.0",
			mockResponse: GitHubRelease{
				TagName: "",
			},
			mockStatus:  http.StatusOK,
			wantUpdate:  false,
			wantVersion: "",
		},
		{
			name:           "pre-release version comparison",
			currentVersion: "1.0.0-beta.1",
			mockResponse: GitHubRelease{
				TagName: "v1.0.0",
			},
			mockStatus:  http.StatusOK,
			wantUpdate:  true,
			wantVersion: "v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request headers
				if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
					t.Errorf("missing or invalid Accept header")
				}
				if !contains(r.Header.Get("User-Agent"), "Monitorly-Probe/") {
					t.Errorf("missing or invalid User-Agent header")
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Save original values
			origVersion := Version
			origURL := GitHubAPIReleaseURL
			// Restore after test
			defer func() {
				Version = origVersion
				GitHubAPIReleaseURL = origURL
			}()

			// Set test values
			Version = tt.currentVersion
			GitHubAPIReleaseURL = server.URL

			// Run test
			gotUpdate, gotVersion, err := CheckForUpdates()
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckForUpdates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotUpdate != tt.wantUpdate {
					t.Errorf("CheckForUpdates() update = %v, want %v", gotUpdate, tt.wantUpdate)
				}
				if gotVersion != tt.wantVersion {
					t.Errorf("CheckForUpdates() version = %v, want %v", gotVersion, tt.wantVersion)
				}
			}
		})
	}
}

func TestGetLatestVersionFromGitHub(t *testing.T) {
	// Override DefaultTimeout for this test
	origTimeout := DefaultTimeout
	DefaultTimeout = 50 * time.Millisecond
	defer func() { DefaultTimeout = origTimeout }()

	tests := []struct {
		name         string
		mockResponse GitHubRelease
		mockStatus   int
		want         string
		wantErr      bool
	}{
		{
			name: "successful response",
			mockResponse: GitHubRelease{
				TagName: "v1.0.0",
			},
			mockStatus: http.StatusOK,
			want:       "v1.0.0",
		},
		{
			name:       "server error",
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "timeout",
			mockStatus: -1, // Special value to trigger timeout
			wantErr:    true,
		},
		{
			name: "invalid json response",
			mockResponse: GitHubRelease{
				TagName: "", // Empty tag name
			},
			mockStatus: http.StatusOK,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.mockStatus == -1 {
				// Create a server that times out
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(100 * time.Millisecond) // Double the timeout to ensure it triggers
				}))
			} else {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.mockStatus)
					if tt.mockStatus == http.StatusOK {
						json.NewEncoder(w).Encode(tt.mockResponse)
					}
				}))
			}
			defer server.Close()

			// Save and restore original URL
			origURL := GitHubAPIReleaseURL
			defer func() { GitHubAPIReleaseURL = origURL }()
			GitHubAPIReleaseURL = server.URL

			got, err := GetLatestVersionFromGitHub()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestVersionFromGitHub() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetLatestVersionFromGitHub() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Save original function and restore after test
var originalGetOS = getOS
var originalGetArch = getArch

func TestFindAppropriateAsset(t *testing.T) {
	// Create mock runtime
	mock := &mockRuntime{
		goos:   runtime.GOOS,
		goarch: runtime.GOARCH,
	}

	// Save original function and restore after test
	defer func() {
		getOS = originalGetOS
		getArch = originalGetArch
	}()

	// Set mock functions
	getOS = mock.GOOS
	getArch = mock.GOARCH

	tests := []struct {
		name    string
		goos    string
		goarch  string
		release *GitHubRelease
		want    string
		wantErr bool
	}{
		{
			name:   "matching linux/amd64 asset",
			goos:   "linux",
			goarch: "amd64",
			release: &GitHubRelease{
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               "monitorly-probe-1.0.0-linux-amd64",
						BrowserDownloadURL: "https://example.com/linux-amd64",
					},
				},
			},
			want:    "https://example.com/linux-amd64",
			wantErr: false,
		},
		{
			name:   "matching linux/arm64 asset",
			goos:   "linux",
			goarch: "arm64",
			release: &GitHubRelease{
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               "monitorly-probe-1.0.0-linux-arm64",
						BrowserDownloadURL: "https://example.com/linux-arm64",
					},
				},
			},
			want:    "https://example.com/linux-arm64",
			wantErr: false,
		},
		{
			name:   "no matching asset",
			goos:   "linux",
			goarch: "amd64",
			release: &GitHubRelease{
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               "monitorly-probe-1.0.0-windows-amd64",
						BrowserDownloadURL: "https://example.com/windows",
					},
				},
			},
			wantErr: true,
		},
		{
			name:   "unsupported OS",
			goos:   "darwin",
			goarch: "amd64",
			release: &GitHubRelease{
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               "monitorly-probe-1.0.0-linux-amd64",
						BrowserDownloadURL: "https://example.com/linux",
					},
				},
			},
			wantErr: true,
		},
		{
			name:   "empty assets",
			goos:   "linux",
			goarch: "amd64",
			release: &GitHubRelease{
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.goos = tt.goos
			mock.goarch = tt.goarch

			got, err := findAppropriateAsset(tt.release)
			if (err != nil) != tt.wantErr {
				t.Errorf("findAppropriateAsset() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("findAppropriateAsset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDownloadBinary(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func() (*httptest.Server, string)
		wantErr    bool
		verifyFile bool
	}{
		{
			name: "successful download",
			setupMock: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("test binary content"))
				}))
				return server, "test binary content"
			},
			verifyFile: true,
		},
		{
			name: "server error",
			setupMock: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				return server, ""
			},
			wantErr: true,
		},
		{
			name: "invalid URL",
			setupMock: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				server.Close() // Close the server to make the URL invalid
				return server, ""
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, expectedContent := tt.setupMock()
			if !strings.Contains(tt.name, "invalid URL") {
				defer server.Close()
			}

			got, err := downloadBinary(server.URL)
			if (err != nil) != tt.wantErr {
				t.Errorf("downloadBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.verifyFile && err == nil {
				// Verify the downloaded file
				content, err := os.ReadFile(got)
				if err != nil {
					t.Errorf("failed to read downloaded file: %v", err)
					return
				}
				if string(content) != expectedContent {
					t.Errorf("downloadBinary() content = %v, want %v", string(content), expectedContent)
				}
				// Clean up
				os.Remove(got)
			}
		})
	}
}

// mockDownloadBinary is a variable to allow mocking downloadBinary in tests
var mockDownloadBinary = downloadBinary

// mockReplaceBinary is a variable to allow mocking replaceBinary in tests
var mockReplaceBinary = replaceBinary

// TestStartUpdateChecker tests the update checker routine
func TestStartUpdateChecker(t *testing.T) {
	// Save original values
	originalCheckInterval := updateCheckInterval
	originalRetryDelay := updateRetryDelay
	originalVersion := Version
	originalGetOS := getOS
	originalDownloadBinary := downloadBinaryFunc
	originalReplaceBinary := replaceBinaryFunc
	defer func() {
		updateCheckInterval = originalCheckInterval
		updateRetryDelay = originalRetryDelay
		Version = originalVersion
		getOS = originalGetOS
		downloadBinaryFunc = originalDownloadBinary
		replaceBinaryFunc = originalReplaceBinary
	}()

	// Set shorter intervals for testing
	updateCheckInterval = 10 * time.Millisecond
	updateRetryDelay = 1 * time.Millisecond

	// Set current version to an older version
	Version = "v1.0.0"

	// Mock os.Exit with proper synchronization
	exitCalled := make(chan bool, 1)
	osExitLock.Lock()
	originalExit := osExit
	osExit = func(code int) {
		select {
		case exitCalled <- true:
		default:
		}
	}
	osExitLock.Unlock()
	defer func() {
		osExitLock.Lock()
		osExit = originalExit
		osExitLock.Unlock()
	}()

	// Mock OS check to return Linux
	getOS = func() string {
		return "linux"
	}

	// Mock download and replace functions
	downloadBinaryFunc = func(url string) (string, error) {
		return "/tmp/mock-binary", nil
	}
	replaceBinaryFunc = func(newBinaryPath string) error {
		return nil
	}

	// Create a test server that returns a newer version
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
			t.Errorf("missing or invalid Accept header")
		}
		if !contains(r.Header.Get("User-Agent"), "Monitorly-Probe/") {
			t.Errorf("missing or invalid User-Agent header")
		}

		response := GitHubRelease{
			TagName: "v2.0.0",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{
					Name:               fmt.Sprintf("monitorly-probe-2.0.0-linux-%s", runtime.GOARCH),
					BrowserDownloadURL: "https://example.com/download",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Set the API URL to our test server
	originalURL := GitHubAPIReleaseURL
	GitHubAPIReleaseURL = server.URL
	defer func() { GitHubAPIReleaseURL = originalURL }()

	// Create a context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start the update checker with next check in 10ms
	nextCheck := time.Now().Add(10 * time.Millisecond)
	StartUpdateChecker(ctx, nextCheck, 1*time.Millisecond)

	// Wait for the exit call or timeout
	select {
	case <-exitCalled:
		// Success: update checker attempted to restart
	case <-time.After(100 * time.Millisecond):
		t.Error("Update checker did not attempt to restart after update within timeout")
	}

	// Cancel the context to stop the checker
	cancel()

	// Give a short time for the goroutine to respond to cancellation
	time.Sleep(10 * time.Millisecond)
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return s != "" && substr != "" && strings.Contains(s, substr)
}

func TestSelfUpdate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("This test only runs on Linux")
		return
	}

	tests := []struct {
		name           string
		currentVersion string
		mockResponse   GitHubRelease
		mockStatus     int
		wantErr        bool
	}{
		{
			name:           "successful update",
			currentVersion: "1.0.0",
			mockResponse: GitHubRelease{
				TagName: "v2.0.0",
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               fmt.Sprintf("monitorly-probe-2.0.0-linux-%s", runtime.GOARCH),
						BrowserDownloadURL: "https://example.com/download",
					},
				},
			},
			mockStatus: http.StatusOK,
		},
		{
			name:           "no update needed",
			currentVersion: "2.0.0",
			mockResponse: GitHubRelease{
				TagName: "v2.0.0",
			},
			mockStatus: http.StatusOK,
		},
		{
			name:           "server error",
			currentVersion: "1.0.0",
			mockStatus:     http.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "no matching asset",
			currentVersion: "1.0.0",
			mockResponse: GitHubRelease{
				TagName: "v2.0.0",
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               "monitorly-probe-2.0.0-windows-amd64",
						BrowserDownloadURL: "https://example.com/download",
					},
				},
			},
			mockStatus: http.StatusOK,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatus)
				if tt.mockStatus == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Save original values
			origURL := GitHubAPIReleaseURL
			origVersion := Version
			defer func() {
				GitHubAPIReleaseURL = origURL
				Version = origVersion
			}()

			// Set test values
			GitHubAPIReleaseURL = server.URL
			Version = tt.currentVersion

			err := SelfUpdate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SelfUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReplaceBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows as it requires special handling")
	}

	tests := []struct {
		name    string
		setup   func() (string, string, error)
		wantErr bool
	}{
		{
			name: "successful replace",
			setup: func() (string, string, error) {
				// Create a temporary directory
				tmpDir, err := os.MkdirTemp("", "test-binary-*")
				if err != nil {
					return "", "", err
				}

				// Create a fake current binary
				currentBin := filepath.Join(tmpDir, "current")
				if err := os.WriteFile(currentBin, []byte("old content"), 0755); err != nil {
					os.RemoveAll(tmpDir)
					return "", "", err
				}

				// Create a new binary
				newBin := filepath.Join(tmpDir, "new")
				if err := os.WriteFile(newBin, []byte("new content"), 0755); err != nil {
					os.RemoveAll(tmpDir)
					return "", "", err
				}

				return currentBin, newBin, nil
			},
			wantErr: false,
		},
		{
			name: "source file does not exist",
			setup: func() (string, string, error) {
				tmpDir, err := os.MkdirTemp("", "test-binary-*")
				if err != nil {
					return "", "", err
				}
				return filepath.Join(tmpDir, "current"), filepath.Join(tmpDir, "nonexistent"), nil
			},
			wantErr: true,
		},
		{
			name: "destination directory not writable",
			setup: func() (string, string, error) {
				// Create a temporary directory
				tmpDir, err := os.MkdirTemp("", "test-binary-*")
				if err != nil {
					return "", "", err
				}

				// Create a read-only subdirectory
				roDir := filepath.Join(tmpDir, "readonly")
				if err := os.Mkdir(roDir, 0500); err != nil {
					os.RemoveAll(tmpDir)
					return "", "", err
				}

				currentBin := filepath.Join(roDir, "current")
				newBin := filepath.Join(tmpDir, "new")

				// Create the new binary
				if err := os.WriteFile(newBin, []byte("new content"), 0755); err != nil {
					os.RemoveAll(tmpDir)
					return "", "", err
				}

				return currentBin, newBin, nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentBin, newBin, err := tt.setup()
			if err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
			defer os.RemoveAll(filepath.Dir(currentBin))

			// Mock os.Executable to return our test binary path
			oldExec := osExecutable
			osExecutable = func() (string, error) {
				return currentBin, nil
			}
			defer func() { osExecutable = oldExec }()

			err = replaceBinary(newBin)
			if (err != nil) != tt.wantErr {
				t.Errorf("replaceBinary() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify the content was replaced
				content, err := os.ReadFile(currentBin)
				if err != nil {
					t.Errorf("Failed to read replaced binary: %v", err)
					return
				}
				if string(content) != "new content" {
					t.Errorf("replaceBinary() content = %v, want %v", string(content), "new content")
				}
			}
		})
	}
}

// TestSelfUpdateErrorCases tests error scenarios for SelfUpdate
func TestSelfUpdateErrorCases(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalURL := GitHubAPIReleaseURL
	originalDownloadBinary := downloadBinaryFunc
	originalReplaceBinary := replaceBinaryFunc
	originalGetOS := getOS
	originalGetArch := getArch
	defer func() {
		Version = originalVersion
		GitHubAPIReleaseURL = originalURL
		downloadBinaryFunc = originalDownloadBinary
		replaceBinaryFunc = originalReplaceBinary
		getOS = originalGetOS
		getArch = originalGetArch
	}()

	tests := []struct {
		name           string
		currentVersion string
		setupMocks     func()
		wantErr        bool
		errContains    string
	}{
		{
			name:           "no update needed",
			currentVersion: "v2.0.0",
			setupMocks: func() {
				// Create a test server that returns the same version
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := GitHubRelease{TagName: "v2.0.0"}
					json.NewEncoder(w).Encode(response)
				}))
				GitHubAPIReleaseURL = server.URL
			},
			wantErr: false,
		},
		{
			name:           "check for updates fails",
			currentVersion: "v1.0.0",
			setupMocks: func() {
				// Create a test server that returns an error
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				GitHubAPIReleaseURL = server.URL
			},
			wantErr:     true,
			errContains: "failed to check for updates",
		},
		{
			name:           "get release info fails",
			currentVersion: "v1.0.0",
			setupMocks: func() {
				// First call succeeds for CheckForUpdates, second call fails for getLatestReleaseInfo
				callCount := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					callCount++
					if callCount == 1 {
						// First call for CheckForUpdates
						response := GitHubRelease{TagName: "v2.0.0"}
						json.NewEncoder(w).Encode(response)
					} else {
						// Second call for getLatestReleaseInfo
						w.WriteHeader(http.StatusInternalServerError)
					}
				}))
				GitHubAPIReleaseURL = server.URL
			},
			wantErr:     true,
			errContains: "failed to get release info",
		},
		{
			name:           "no appropriate asset found",
			currentVersion: "v1.0.0",
			setupMocks: func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := GitHubRelease{
						TagName: "v2.0.0",
						Assets: []struct {
							Name               string `json:"name"`
							BrowserDownloadURL string `json:"browser_download_url"`
						}{
							{
								Name:               "monitorly-probe-2.0.0-windows-amd64",
								BrowserDownloadURL: "https://example.com/windows",
							},
						},
					}
					json.NewEncoder(w).Encode(response)
				}))
				GitHubAPIReleaseURL = server.URL
				getOS = func() string { return "linux" }
			},
			wantErr:     true,
			errContains: "failed to find appropriate asset",
		},
		{
			name:           "download fails",
			currentVersion: "v1.0.0",
			setupMocks: func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := GitHubRelease{
						TagName: "v2.0.0",
						Assets: []struct {
							Name               string `json:"name"`
							BrowserDownloadURL string `json:"browser_download_url"`
						}{
							{
								Name:               fmt.Sprintf("monitorly-probe-2.0.0-linux-%s", runtime.GOARCH),
								BrowserDownloadURL: "https://example.com/download",
							},
						},
					}
					json.NewEncoder(w).Encode(response)
				}))
				GitHubAPIReleaseURL = server.URL
				getOS = func() string { return "linux" }
				getArch = func() string { return runtime.GOARCH }
				downloadBinaryFunc = func(url string) (string, error) {
					return "", fmt.Errorf("download failed")
				}
			},
			wantErr:     true,
			errContains: "failed to download binary",
		},
		{
			name:           "replace binary fails",
			currentVersion: "v1.0.0",
			setupMocks: func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := GitHubRelease{
						TagName: "v2.0.0",
						Assets: []struct {
							Name               string `json:"name"`
							BrowserDownloadURL string `json:"browser_download_url"`
						}{
							{
								Name:               fmt.Sprintf("monitorly-probe-2.0.0-linux-%s", runtime.GOARCH),
								BrowserDownloadURL: "https://example.com/download",
							},
						},
					}
					json.NewEncoder(w).Encode(response)
				}))
				GitHubAPIReleaseURL = server.URL
				getOS = func() string { return "linux" }
				getArch = func() string { return runtime.GOARCH }
				downloadBinaryFunc = func(url string) (string, error) {
					return "/tmp/mock-binary", nil
				}
				replaceBinaryFunc = func(newBinaryPath string) error {
					return fmt.Errorf("replace failed")
				}
			},
			wantErr:     true,
			errContains: "failed to replace binary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.currentVersion
			tt.setupMocks()

			err := SelfUpdate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SelfUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("SelfUpdate() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestDownloadBinaryWithTimeoutErrorCases tests error scenarios for downloadBinaryWithTimeout
func TestDownloadBinaryWithTimeoutErrorCases(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() *httptest.Server
		timeout   time.Duration
		wantErr   bool
	}{
		{
			name: "timeout",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(100 * time.Millisecond) // Longer than timeout
					w.Write([]byte("test"))
				}))
			},
			timeout: 10 * time.Millisecond,
			wantErr: true,
		},
		{
			name: "empty response body",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Length", "0")
					w.WriteHeader(http.StatusOK)
				}))
			},
			timeout: time.Second,
			wantErr: true,
		},
		{
			name: "invalid status code",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			timeout: time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupMock()
			defer server.Close()

			_, err := downloadBinaryWithTimeout(server.URL, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("downloadBinaryWithTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetLatestReleaseInfoErrorCases tests error scenarios for getLatestReleaseInfo
func TestGetLatestReleaseInfoErrorCases(t *testing.T) {
	// Save original URL
	originalURL := GitHubAPIReleaseURL
	defer func() { GitHubAPIReleaseURL = originalURL }()

	tests := []struct {
		name      string
		setupMock func() *httptest.Server
		wantErr   bool
	}{
		{
			name: "invalid JSON response",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("invalid json"))
				}))
			},
			wantErr: true,
		},
		{
			name: "server error",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupMock()
			defer server.Close()

			GitHubAPIReleaseURL = server.URL

			_, err := getLatestReleaseInfo()
			if (err != nil) != tt.wantErr {
				t.Errorf("getLatestReleaseInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestStartUpdateCheckerErrorCases tests error scenarios for StartUpdateChecker
func TestStartUpdateCheckerErrorCases(t *testing.T) {
	// Save original values
	originalCheckInterval := updateCheckInterval
	originalRetryDelay := updateRetryDelay
	originalVersion := Version
	originalURL := GitHubAPIReleaseURL
	originalDownloadBinary := downloadBinaryFunc
	originalReplaceBinary := replaceBinaryFunc
	defer func() {
		updateCheckInterval = originalCheckInterval
		updateRetryDelay = originalRetryDelay
		Version = originalVersion
		GitHubAPIReleaseURL = originalURL
		downloadBinaryFunc = originalDownloadBinary
		replaceBinaryFunc = originalReplaceBinary
	}()

	// Set shorter intervals for testing
	updateCheckInterval = 10 * time.Millisecond
	updateRetryDelay = 5 * time.Millisecond

	tests := []struct {
		name       string
		setupMocks func() *httptest.Server
		wantRetry  bool
	}{
		{
			name: "update check fails with retry",
			setupMocks: func() *httptest.Server {
				Version = "v1.0.0"
				// Create a server that always returns an error
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				return server
			},
			wantRetry: true,
		},
		{
			name: "update fails with retry",
			setupMocks: func() *httptest.Server {
				Version = "v1.0.0"
				// Create a server that returns a newer version
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := GitHubRelease{
						TagName: "v2.0.0",
						Assets: []struct {
							Name               string `json:"name"`
							BrowserDownloadURL string `json:"browser_download_url"`
						}{
							{
								Name:               "monitorly-probe-2.0.0-linux-amd64",
								BrowserDownloadURL: "https://example.com/download",
							},
						},
					}
					json.NewEncoder(w).Encode(response)
				}))
				// Mock download to fail
				downloadBinaryFunc = func(url string) (string, error) {
					return "", fmt.Errorf("download failed")
				}
				return server
			},
			wantRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupMocks()
			defer server.Close()

			GitHubAPIReleaseURL = server.URL

			// Create a context with cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Start the update checker
			nextCheck := time.Now().Add(10 * time.Millisecond)
			StartUpdateChecker(ctx, nextCheck, 5*time.Millisecond)

			// Wait for a short period to allow the checker to run
			time.Sleep(30 * time.Millisecond)

			// Cancel the context to stop the checker
			cancel()

			// Give time for the goroutine to stop
			time.Sleep(10 * time.Millisecond)

			// The test passes if it doesn't hang or crash
		})
	}
}
