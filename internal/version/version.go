// Package version provides access to the application version information
package version

import (
	"fmt"
	"runtime"
)

// These variables are set at build time via ldflags
var (
	// Version is the semantic version of the application
	Version = "dev"

	// BuildDate is the date when the binary was built
	BuildDate = "unknown"

	// Commit is the git commit hash from which the binary was built
	Commit = "unknown"
)

// Info returns a formatted string with version information
func Info() string {
	return fmt.Sprintf("Monitorly Probe v%s (commit: %s, built: %s, %s/%s)",
		Version, Commit, BuildDate, runtime.GOOS, runtime.GOARCH)
}

// GetVersion returns the current version string
func GetVersion() string {
	return Version
}

// GetBuildDate returns the build date
func GetBuildDate() string {
	return BuildDate
}

// GetCommit returns the git commit hash
func GetCommit() string {
	return Commit
}
