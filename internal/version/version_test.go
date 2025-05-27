package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalBuildDate := BuildDate
	originalCommit := Commit

	// Restore original values after test
	defer func() {
		Version = originalVersion
		BuildDate = originalBuildDate
		Commit = originalCommit
	}()

	tests := []struct {
		name      string
		version   string
		buildDate string
		commit    string
		want      []string // strings that should be present in the output
	}{
		{
			name:      "default values",
			version:   "dev",
			buildDate: "unknown",
			commit:    "unknown",
			want: []string{
				"Monitorly Probe v",
				"dev",
				"commit: unknown",
				"built: unknown",
				runtime.GOOS,
				runtime.GOARCH,
			},
		},
		{
			name:      "production values",
			version:   "1.2.3",
			buildDate: "2023-12-01T10:00:00Z",
			commit:    "abc123def456",
			want: []string{
				"Monitorly Probe v",
				"1.2.3",
				"commit: abc123def456",
				"built: 2023-12-01T10:00:00Z",
				runtime.GOOS,
				runtime.GOARCH,
			},
		},
		{
			name:      "empty values",
			version:   "",
			buildDate: "",
			commit:    "",
			want: []string{
				"Monitorly Probe v",
				"commit: ",
				"built: ",
				runtime.GOOS,
				runtime.GOARCH,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test values
			Version = tt.version
			BuildDate = tt.buildDate
			Commit = tt.commit

			got := Info()

			// Check that all expected strings are present
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("Info() = %v, want to contain %v", got, want)
				}
			}

			// Check that the format is correct
			expectedFormat := "Monitorly Probe v" + tt.version + " (commit: " + tt.commit + ", built: " + tt.buildDate + ", " + runtime.GOOS + "/" + runtime.GOARCH + ")"
			if got != expectedFormat {
				t.Errorf("Info() = %v, want %v", got, expectedFormat)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	// Save original value
	originalVersion := Version

	// Restore original value after test
	defer func() {
		Version = originalVersion
	}()

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
			version: "1.2.3-alpha.1",
			want:    "1.2.3-alpha.1",
		},
		{
			name:    "empty version",
			version: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			got := GetVersion()
			if got != tt.want {
				t.Errorf("GetVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBuildDate(t *testing.T) {
	// Save original value
	originalBuildDate := BuildDate

	// Restore original value after test
	defer func() {
		BuildDate = originalBuildDate
	}()

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
			buildDate: "2023-12-01T10:00:00Z",
			want:      "2023-12-01T10:00:00Z",
		},
		{
			name:      "custom format",
			buildDate: "Dec 1, 2023",
			want:      "Dec 1, 2023",
		},
		{
			name:      "empty build date",
			buildDate: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BuildDate = tt.buildDate
			got := GetBuildDate()
			if got != tt.want {
				t.Errorf("GetBuildDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCommit(t *testing.T) {
	// Save original value
	originalCommit := Commit

	// Restore original value after test
	defer func() {
		Commit = originalCommit
	}()

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
			commit: "abc123def456789012345678901234567890abcd",
			want:   "abc123def456789012345678901234567890abcd",
		},
		{
			name:   "short commit hash",
			commit: "abc123d",
			want:   "abc123d",
		},
		{
			name:   "empty commit",
			commit: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Commit = tt.commit
			got := GetCommit()
			if got != tt.want {
				t.Errorf("GetCommit() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestVersionVariablesIntegration tests that all version variables work together
func TestVersionVariablesIntegration(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalBuildDate := BuildDate
	originalCommit := Commit

	// Restore original values after test
	defer func() {
		Version = originalVersion
		BuildDate = originalBuildDate
		Commit = originalCommit
	}()

	// Set test values
	testVersion := "2.0.0"
	testBuildDate := "2023-12-01T15:30:00Z"
	testCommit := "def456abc789"

	Version = testVersion
	BuildDate = testBuildDate
	Commit = testCommit

	// Test that all getters return the correct values
	if GetVersion() != testVersion {
		t.Errorf("GetVersion() = %v, want %v", GetVersion(), testVersion)
	}

	if GetBuildDate() != testBuildDate {
		t.Errorf("GetBuildDate() = %v, want %v", GetBuildDate(), testBuildDate)
	}

	if GetCommit() != testCommit {
		t.Errorf("GetCommit() = %v, want %v", GetCommit(), testCommit)
	}

	// Test that Info() incorporates all values
	info := Info()
	if !strings.Contains(info, testVersion) {
		t.Errorf("Info() does not contain version %v", testVersion)
	}
	if !strings.Contains(info, testBuildDate) {
		t.Errorf("Info() does not contain build date %v", testBuildDate)
	}
	if !strings.Contains(info, testCommit) {
		t.Errorf("Info() does not contain commit %v", testCommit)
	}
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
