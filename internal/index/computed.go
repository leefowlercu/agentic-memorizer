package index

import (
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// ComputedIndex represents the precomputed index file structure.
// This wraps the core types.Index with additional metadata about
// the index file itself (version, generation time, daemon info).
type ComputedIndex struct {
	Version       string       `json:"version"`
	GeneratedAt   time.Time    `json:"generated_at"`
	DaemonVersion string       `json:"daemon_version"`
	Index         *types.Index `json:"index"`
	Metadata      BuildMetadata `json:"metadata"`
}

// BuildMetadata contains information about the index build process
type BuildMetadata struct {
	BuildDurationMs int `json:"build_duration_ms"`
	FilesProcessed  int `json:"files_processed"`
	CacheHits       int `json:"cache_hits"`
	APICalls        int `json:"api_calls"`
}
