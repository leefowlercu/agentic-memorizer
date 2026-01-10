// Package version provides version and build information for the application.
package version

import (
	_ "embed"
	"fmt"
	"runtime/debug"
	"strings"
)

//go:embed VERSION
var versionFile string

// Linker-injected variables. Set via:
//
//	go build -ldflags "-X github.com/leefowlercu/agentic-memorizer/internal/version.gitCommit=VALUE"
var (
	gitCommit string // Set via -ldflags
	buildDate string // Set via -ldflags
)

// Info represents version and build information.
type Info struct {
	// Version is the semantic version (e.g., "0.1.0", "1.0.0-alpha.1+build.456").
	Version string

	// GitCommit is the short git commit hash with optional "-dirty" suffix.
	GitCommit string

	// BuildDate is the ISO 8601 timestamp (e.g., "2026-01-10T15:04:05Z").
	BuildDate string
}

// String formats Info for human-readable display.
func (i Info) String() string {
	return fmt.Sprintf("Version:    %s\nGit Commit: %s\nBuild Date: %s",
		i.Version, i.GitCommit, i.BuildDate)
}

// Get returns the populated Info struct with version, git commit, and build date.
func Get() Info {
	return Info{
		Version:   getVersion(),
		GitCommit: getGitCommit(),
		BuildDate: getBuildDate(),
	}
}

// getVersion returns the semantic version from the embedded VERSION file.
func getVersion() string {
	return strings.TrimSpace(versionFile)
}

// getGitCommit returns the git commit hash.
// Priority: linker flag > debug.ReadBuildInfo > "unknown"
func getGitCommit() string {
	if gitCommit != "" {
		return gitCommit
	}

	// Fallback to debug.ReadBuildInfo for go install builds
	revision, dirty := readBuildInfo()
	if revision != "" {
		if dirty {
			return revision + "-dirty"
		}
		return revision
	}

	return "unknown"
}

// getBuildDate returns the build date timestamp.
// Returns linker-injected value or "unknown" if not set.
func getBuildDate() string {
	if buildDate != "" {
		return buildDate
	}
	return "unknown"
}

// readBuildInfo extracts VCS info from debug.ReadBuildInfo.
// Returns the commit revision (shortened to 7 chars) and dirty status.
func readBuildInfo() (revision string, dirty bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
			// Shorten to 7 characters for display
			if len(revision) > 7 {
				revision = revision[:7]
			}
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	return revision, dirty
}
