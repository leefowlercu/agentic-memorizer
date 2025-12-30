package api

import (
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// NOTE: graph import is still needed for other types (SearchResult, RelatedFile, RebuildHandler)

// FilesQueryResponse is the unified response for GET /api/v1/files
// This replaces POST /api/v1/search, GET /api/v1/files/recent, and GET /api/v1/entities/search
type FilesQueryResponse struct {
	Files []graph.SearchResult `json:"files"`
	Count int                  `json:"count"`
	Query FilesQueryParams     `json:"query"`
}

// FilesQueryParams echoes back the query parameters used
type FilesQueryParams struct {
	Q        string `json:"q,omitempty"`
	Category string `json:"category,omitempty"`
	Days     int    `json:"days,omitempty"`
	Entity   string `json:"entity,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Topic    string `json:"topic,omitempty"`
	Limit    int    `json:"limit"`
}

// FileMetadataResponse is the response for GET /api/v1/files/{path}
// Uses the new graph-native FileEntry format which includes related files
type FileMetadataResponse struct {
	File *types.FileEntry `json:"file"`
}

// FilesIndexResponse is the response for GET /api/v1/files/index
type FilesIndexResponse struct {
	Index *types.FileIndex `json:"index"`
}

// FactsIndexResponse is the response for GET /api/v1/facts/index
type FactsIndexResponse struct {
	Facts []types.Fact `json:"facts"`
	Count int          `json:"count"`
	Stats FactsStats   `json:"stats"`
}

// FactsStats provides summary statistics for facts
type FactsStats struct {
	TotalFacts int `json:"total_facts"`
	MaxFacts   int `json:"max_facts"`
}

// FactResponse is the response for GET /api/v1/facts/{id}
type FactResponse struct {
	Fact *types.Fact `json:"fact"`
}

// APIError represents an error response
type APIError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// RebuildResponse is the response for POST /api/v1/rebuild
type RebuildResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// RebuildHandler provides rebuild functionality from the daemon
type RebuildHandler interface {
	// Rebuild triggers an immediate index rebuild
	Rebuild() error
	// RebuildWithSync triggers a rebuild and optionally syncs graph with filesystem
	// When sync is true, stale graph nodes (files no longer on disk) are removed
	RebuildWithSync(sync bool) error
	// ClearGraph clears all data from the graph
	ClearGraph() error
	// IsRebuilding returns true if a rebuild is in progress
	IsRebuilding() bool
}

// SSEEvent represents an SSE event with typed data
type SSEEvent struct {
	Type      string           `json:"type"`
	Timestamp string           `json:"timestamp"`
	Data      *types.FileIndex `json:"data,omitempty"`
}
