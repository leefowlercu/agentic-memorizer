package version

import (
	_ "embed"
	"fmt"
	"runtime/debug"
	"strings"
)

//go:embed VERSION
var embeddedVersion string

var (
	// Version is the application version
	// Set at build time with: go build -ldflags "-X github.com/leefowlercu/agentic-memorizer/internal/version.Version=v1.0.0"
	// If not set via ldflags, will fall back to embedded VERSION file
	Version = ""

	// GitCommit is the git commit hash
	// Set at build time with: go build -ldflags "-X github.com/leefowlercu/agentic-memorizer/internal/version.GitCommit=$(git rev-parse HEAD)"
	GitCommit = "unknown"

	// BuildDate is the build date
	// Set at build time with: go build -ldflags "-X github.com/leefowlercu/agentic-memorizer/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	BuildDate = "unknown"
)

// getVersion returns the version string, using ldflags if set, otherwise embedded VERSION
func getVersion() string {
	if Version != "" {
		return Version
	}
	return strings.TrimSpace(embeddedVersion)
}

// getGitCommit returns the git commit hash, using ldflags if set, otherwise build info
func getGitCommit() string {
	// Priority 1: Use ldflags if set (make build-release)
	if GitCommit != "unknown" {
		return GitCommit
	}

	// Priority 2: Use build info (go install, go build)
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	var commit string
	var modified bool

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			commit = setting.Value
			// Use short hash (first 7 characters)
			if len(commit) > 7 {
				commit = commit[:7]
			}
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	// Add dirty flag if workspace has uncommitted changes
	if commit != "" && modified {
		commit += "-dirty"
	}

	if commit == "" {
		return "unknown"
	}

	return commit
}

// getBuildDate returns the build date, using ldflags if set, otherwise build info
func getBuildDate() string {
	// Priority 1: Use ldflags if set (make build-release)
	if BuildDate != "unknown" {
		return BuildDate
	}

	// Priority 2: Use build info (go install, go build)
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	for _, setting := range info.Settings {
		if setting.Key == "vcs.time" {
			return setting.Value
		}
	}

	return "unknown"
}

// GetVersion returns a formatted version string
func GetVersion() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", getVersion(), getGitCommit(), getBuildDate())
}

// GetShortVersion returns just the version number
func GetShortVersion() string {
	return getVersion()
}

// GetGitCommit returns the git commit hash
func GetGitCommit() string {
	return getGitCommit()
}

// GetBuildDate returns the build date
func GetBuildDate() string {
	return getBuildDate()
}
