package api

import (
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// NOTE: graph import is still needed for other types (SearchResult, RelatedFile, RebuildHandler)

// SearchRequest is the request body for POST /api/v1/search
type SearchRequest struct {
	Query    string `json:"query"`
	Limit    int    `json:"limit,omitempty"`
	Category string `json:"category,omitempty"`
}

// SearchResponse is the response for search endpoints
type SearchResponse struct {
	Results []graph.SearchResult `json:"results"`
	Count   int                  `json:"count"`
}

// FileMetadataResponse is the response for GET /api/v1/files/{path}
// Uses the new graph-native FileEntry format which includes related files
type FileMetadataResponse struct {
	File *types.FileEntry `json:"file"`
}

// RecentFilesResponse is the response for GET /api/v1/files/recent
type RecentFilesResponse struct {
	Files []graph.SearchResult `json:"files"`
	Count int                  `json:"count"`
}

// RelatedFilesResponse is the response for GET /api/v1/files/related
type RelatedFilesResponse struct {
	Files []graph.RelatedFile `json:"files"`
	Count int                 `json:"count"`
}

// EntitySearchResponse is the response for GET /api/v1/entities/search
type EntitySearchResponse struct {
	Results []graph.SearchResult `json:"results"`
	Count   int                  `json:"count"`
}

// IndexResponse is the response for GET /api/v1/index
type IndexResponse struct {
	Index *types.GraphIndex `json:"index"`
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
	// ClearGraph clears all data from the graph
	ClearGraph() error
	// IsRebuilding returns true if a rebuild is in progress
	IsRebuilding() bool
}

// SSEEvent represents an SSE event with typed data
type SSEEvent struct {
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Data      *types.GraphIndex `json:"data,omitempty"`
}
