package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		buildDate string
		commit    string
		validate  func(*testing.T, string)
	}{
		{
			name:      "default values",
			version:   "dev",
			buildDate: "unknown",
			commit:    "unknown",
			validate: func(t *testing.T, info string) {
				expected := "Monitorly Probe vdev (commit: unknown, built: unknown"
				if !strings.HasPrefix(info, expected) {
					t.Errorf("Info() = %v, want prefix %v", info, expected)
				}
				if !strings.Contains(info, runtime.GOOS) {
					t.Errorf("Info() = %v, missing OS %v", info, runtime.GOOS)
				}
				if !strings.Contains(info, runtime.GOARCH) {
					t.Errorf("Info() = %v, missing arch %v", info, runtime.GOARCH)
				}
			},
		},
		{
			name:      "production values",
			version:   "1.2.3",
			buildDate: "2024-03-27T12:00:00Z",
			commit:    "abc123def456",
			validate: func(t *testing.T, info string) {
				expected := "Monitorly Probe v1.2.3 (commit: abc123def456, built: 2024-03-27T12:00:00Z"
				if !strings.HasPrefix(info, expected) {
					t.Errorf("Info() = %v, want prefix %v", info, expected)
				}
			},
		},
		{
			name:      "empty values",
			version:   "",
			buildDate: "",
			commit:    "",
			validate: func(t *testing.T, info string) {
				if !strings.Contains(info, "v (commit: , built: ") {
					t.Errorf("Info() = %v, not handling empty values correctly", info)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origVersion := Version
			origBuildDate := BuildDate
			origCommit := Commit

			// Set test values
			Version = tt.version
			BuildDate = tt.buildDate
			Commit = tt.commit

			// Run test
			info := Info()
			tt.validate(t, info)

			// Restore original values
			Version = origVersion
			BuildDate = origBuildDate
			Commit = origCommit
		})
	}
}

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "default version",
			version: "dev",
			want:    "dev",
		},
		{
			name:    "semantic version",
			version: "1.2.3",
			want:    "1.2.3",
		},
		{
			name:    "pre-release version",
			version: "1.0.0-beta.1",
			want:    "1.0.0-beta.1",
		},
		{
			name:    "empty version",
			version: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			origVersion := Version
			// Set test value
			Version = tt.version

			if got := GetVersion(); got != tt.want {
				t.Errorf("GetVersion() = %v, want %v", got, tt.want)
			}

			// Restore original value
			Version = origVersion
		})
	}
}

func TestGetBuildDate(t *testing.T) {
	tests := []struct {
		name      string
		buildDate string
		want      string
	}{
		{
			name:      "default build date",
			buildDate: "unknown",
			want:      "unknown",
		},
		{
			name:      "ISO 8601 format",
			buildDate: "2024-03-27T12:00:00Z",
			want:      "2024-03-27T12:00:00Z",
		},
		{
			name:      "custom format",
			buildDate: "Mar 27 2024 12:00:00",
			want:      "Mar 27 2024 12:00:00",
		},
		{
			name:      "empty build date",
			buildDate: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			origBuildDate := BuildDate
			// Set test value
			BuildDate = tt.buildDate

			if got := GetBuildDate(); got != tt.want {
				t.Errorf("GetBuildDate() = %v, want %v", got, tt.want)
			}

			// Restore original value
			BuildDate = origBuildDate
		})
	}
}

func TestGetCommit(t *testing.T) {
	tests := []struct {
		name   string
		commit string
		want   string
	}{
		{
			name:   "default commit",
			commit: "unknown",
			want:   "unknown",
		},
		{
			name:   "full commit hash",
			commit: "abcdef1234567890",
			want:   "abcdef1234567890",
		},
		{
			name:   "short commit hash",
			commit: "abcdef12",
			want:   "abcdef12",
		},
		{
			name:   "empty commit",
			commit: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			origCommit := Commit
			// Set test value
			Commit = tt.commit

			if got := GetCommit(); got != tt.want {
				t.Errorf("GetCommit() = %v, want %v", got, tt.want)
			}

			// Restore original value
			Commit = origCommit
		})
	}
}

func TestVersionVariablesIntegration(t *testing.T) {
	// Test that all version-related functions work together correctly
	testVersion := "1.2.3"
	testBuildDate := "2024-03-27T12:00:00Z"
	testCommit := "abcdef123456"

	// Save original values
	origVersion := Version
	origBuildDate := BuildDate
	origCommit := Commit

	// Set test values
	Version = testVersion
	BuildDate = testBuildDate
	Commit = testCommit

	// Test Info() contains all components
	info := Info()
	if !strings.Contains(info, testVersion) {
		t.Errorf("Info() = %v, missing version %v", info, testVersion)
	}
	if !strings.Contains(info, testBuildDate) {
		t.Errorf("Info() = %v, missing build date %v", info, testBuildDate)
	}
	if !strings.Contains(info, testCommit) {
		t.Errorf("Info() = %v, missing commit %v", info, testCommit)
	}

	// Test individual getters
	if got := GetVersion(); got != testVersion {
		t.Errorf("GetVersion() = %v, want %v", got, testVersion)
	}
	if got := GetBuildDate(); got != testBuildDate {
		t.Errorf("GetBuildDate() = %v, want %v", got, testBuildDate)
	}
	if got := GetCommit(); got != testCommit {
		t.Errorf("GetCommit() = %v, want %v", got, testCommit)
	}

	// Restore original values
	Version = origVersion
	BuildDate = origBuildDate
	Commit = origCommit
}

// BenchmarkInfo benchmarks the Info function
func BenchmarkInfo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Info()
	}
}

// BenchmarkGetVersion benchmarks the GetVersion function
func BenchmarkGetVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GetVersion()
	}
}

// BenchmarkGetBuildDate benchmarks the GetBuildDate function
func BenchmarkGetBuildDate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GetBuildDate()
	}
}

// BenchmarkGetCommit benchmarks the GetCommit function
func BenchmarkGetCommit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GetCommit()
	}
}
