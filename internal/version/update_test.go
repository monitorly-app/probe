package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

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
					time.Sleep(DefaultTimeout + time.Second)
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

func TestFindAppropriateAsset(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("This test only runs on Linux as we only build Linux binaries")
		return
	}

	tests := []struct {
		name    string
		release *GitHubRelease
		want    string
		wantErr bool
	}{
		{
			name: "matching asset found",
			release: &GitHubRelease{
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               fmt.Sprintf("monitorly-probe-1.0.0-linux-%s", runtime.GOARCH),
						BrowserDownloadURL: "https://example.com/download",
					},
				},
			},
			want: "https://example.com/download",
		},
		{
			name: "no matching asset",
			release: &GitHubRelease{
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               "monitorly-probe-1.0.0-windows-amd64",
						BrowserDownloadURL: "https://example.com/download",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty assets",
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
		timeout    time.Duration
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
			timeout:    time.Second,
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
			timeout: time.Second,
			wantErr: true,
		},
		{
			name: "timeout",
			setupMock: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Sleep for a short duration to trigger context timeout
					time.Sleep(100 * time.Millisecond)
				}))
				return server, ""
			},
			timeout: 50 * time.Millisecond,
			wantErr: true,
		},
		{
			name: "invalid URL",
			setupMock: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				server.Close() // Close the server to make the URL invalid
				return server, ""
			},
			timeout: time.Second,
			wantErr: true,
		},
		{
			name: "empty response",
			setupMock: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				return server, ""
			},
			timeout: time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, expectedContent := tt.setupMock()
			if !strings.Contains(tt.name, "invalid URL") {
				defer server.Close()
			}

			got, err := downloadBinaryWithTimeout(server.URL, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("downloadBinary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.verifyFile {
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

func TestStartUpdateChecker(t *testing.T) {
	tests := []struct {
		name       string
		nextCheck  time.Time
		retryDelay time.Duration
		setupMock  func() *httptest.Server
	}{
		{
			name:       "update available",
			nextCheck:  time.Now().Add(-time.Hour), // Check immediately
			retryDelay: time.Minute,
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					json.NewEncoder(w).Encode(GitHubRelease{
						TagName: "v2.0.0",
					})
				}))
			},
		},
		{
			name:       "no update available",
			nextCheck:  time.Now().Add(-time.Hour), // Check immediately
			retryDelay: time.Minute,
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					json.NewEncoder(w).Encode(GitHubRelease{
						TagName: "v1.0.0",
					})
				}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupMock()
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
			Version = "v1.0.0"

			// Create a context that cancels after a short time
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Start the update checker
			StartUpdateChecker(ctx, tt.nextCheck, tt.retryDelay)

			// Wait for context to be done
			<-ctx.Done()
		})
	}
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
	if runtime.GOOS != "linux" {
		t.Skip("This test only runs on Linux")
		return
	}

	tests := []struct {
		name    string
		setup   func() string
		wantErr bool
	}{
		{
			name: "successful replace",
			setup: func() string {
				// Create a temporary file with some content
				f, err := os.CreateTemp("", "test-binary-*")
				if err != nil {
					t.Fatal(err)
				}
				f.Write([]byte("test content"))
				f.Close()
				return f.Name()
			},
			wantErr: false,
		},
		{
			name: "source file does not exist",
			setup: func() string {
				return "nonexistent-file"
			},
			wantErr: true,
		},
		{
			name: "permission denied",
			setup: func() string {
				// Create a temporary file with read-only permissions
				f, err := os.CreateTemp("", "test-binary-*")
				if err != nil {
					t.Fatal(err)
				}
				f.Write([]byte("test content"))
				f.Close()
				os.Chmod(f.Name(), 0400) // Read-only
				return f.Name()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newBinaryPath := tt.setup()
			if newBinaryPath != "nonexistent-file" {
				defer os.Remove(newBinaryPath)
			}

			err := replaceBinary(newBinaryPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("replaceBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
