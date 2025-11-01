package version

import "fmt"

var (
	// Version is the application version
	// Can be set at build time with: go build -ldflags "-X github.com/leefowlercu/agentic-memorizer/internal/version.Version=1.0.0"
	Version = "dev"

	// GitCommit is the git commit hash
	// Can be set at build time with: go build -ldflags "-X github.com/leefowlercu/agentic-memorizer/internal/version.GitCommit=$(git rev-parse HEAD)"
	GitCommit = "unknown"

	// BuildDate is the build date
	// Can be set at build time with: go build -ldflags "-X github.com/leefowlercu/agentic-memorizer/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	BuildDate = "unknown"
)

// GetVersion returns a formatted version string
func GetVersion() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}

// GetShortVersion returns just the version number
func GetShortVersion() string {
	return Version
}
