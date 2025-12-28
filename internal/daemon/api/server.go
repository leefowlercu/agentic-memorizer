package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// HealthMetricsProvider provides health metrics for the health endpoint
type HealthMetricsProvider interface {
	GetSnapshot() HealthSnapshot
}

// HealthSnapshot contains health metrics data
type HealthSnapshot struct {
	StartTime        time.Time
	Uptime           string
	UptimeSeconds    float64
	FilesProcessed   int
	APICalls         int
	CacheHits        int
	Errors           int
	LastBuildTime    time.Time
	LastBuildSuccess bool
	IndexFileCount   int
	WatcherActive    bool

	// Semantic provider info
	SemanticEnabled  bool
	SemanticProvider string
	SemanticModel    string

	// Cache versioning stats
	CacheVersion       string
	CacheTotalEntries  int
	CacheLegacyEntries int
	CacheTotalSize     int64
}

// HTTPServer manages the daemon's unified HTTP API
type HTTPServer struct {
	server         *http.Server
	hub            *SSEHub
	metrics        HealthMetricsProvider
	rebuildHandler RebuildHandler
	graphManager   *graph.Manager
	exporter       *graph.Exporter
	memoryRoot     string
	logger         *slog.Logger
	mu             sync.Mutex
}

// NewHTTPServer creates a new unified HTTP server
func NewHTTPServer(hub *SSEHub, metrics HealthMetricsProvider, graphManager *graph.Manager, memoryRoot string, logger *slog.Logger) *HTTPServer {
	var exporter *graph.Exporter
	if graphManager != nil {
		exporter = graph.NewExporter(graphManager, logger)
	}

	return &HTTPServer{
		hub:          hub,
		metrics:      metrics,
		graphManager: graphManager,
		exporter:     exporter,
		memoryRoot:   memoryRoot,
		logger:       logger,
	}
}

// SetRebuildHandler sets the rebuild handler for the API
func (s *HTTPServer) SetRebuildHandler(handler RebuildHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebuildHandler = handler
}

// withTimeout wraps a handler with http.TimeoutHandler to enforce request timeouts.
// This is used for API endpoints to prevent hung requests while allowing streaming
// endpoints (SSE) to maintain long-lived connections.
func withTimeout(handler http.HandlerFunc, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.TimeoutHandler(handler, timeout, "Request timeout").ServeHTTP(w, r)
	}
}

// Start starts the HTTP server on the specified port
// If a server is already running, it will be stopped first (supports hot-reload)
func (s *HTTPServer) Start(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing server if running (supports hot-reload)
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Warn("http server shutdown failed during restart", "error", err)
		}
		s.server = nil
	}

	// Don't start if disabled (port 0)
	if port == 0 {
		s.logger.Info("http server disabled")
		return nil
	}

	// Create router with all endpoints
	mux := http.NewServeMux()

	// Health and SSE endpoints - no timeout
	// Health check should always respond quickly for monitoring
	// SSE is a long-lived streaming connection with 30s keepalive
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/sse", s.hub.HandleSSE)

	// API v1 endpoints - 60 second timeout
	// Prevents hung requests while allowing reasonable processing time
	apiTimeout := 60 * time.Second
	mux.HandleFunc("/api/v1/index", withTimeout(s.handleGetIndex, apiTimeout))
	mux.HandleFunc("/api/v1/search", withTimeout(s.handleSearch, apiTimeout))
	mux.HandleFunc("/api/v1/rebuild", withTimeout(s.handleRebuild, apiTimeout))
	mux.HandleFunc("/api/v1/files/recent", withTimeout(s.handleRecentFiles, apiTimeout))
	mux.HandleFunc("/api/v1/files/related", withTimeout(s.handleRelatedFiles, apiTimeout))
	mux.HandleFunc("/api/v1/files/", withTimeout(s.handleGetFile, apiTimeout)) // Catch-all for /api/v1/files/{path}
	mux.HandleFunc("/api/v1/entities/search", withTimeout(s.handleEntitySearch, apiTimeout))

	// Create server without global timeouts
	// Timeouts are handled per-endpoint via http.TimeoutHandler middleware
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	s.logger.Info("starting HTTP server", "port", port)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("http server failed", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (s *HTTPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.server.Shutdown(ctx)
	s.server = nil

	return err
}

// handleHealth handles health check requests
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := s.metrics.GetSnapshot()

	// Get SSE client count from hub
	sseClients := s.hub.ClientCount()

	// Determine overall health status
	status := "healthy"
	if !snapshot.LastBuildSuccess && !snapshot.LastBuildTime.IsZero() {
		status = "degraded"
	}
	if snapshot.Errors > 10 {
		status = "unhealthy"
	}

	response := map[string]any{
		"status": status,
		"metrics": map[string]any{
			"start_time":         snapshot.StartTime,
			"uptime":             snapshot.Uptime,
			"uptime_seconds":     snapshot.UptimeSeconds,
			"files_processed":    snapshot.FilesProcessed,
			"api_calls":          snapshot.APICalls,
			"cache_hits":         snapshot.CacheHits,
			"errors":             snapshot.Errors,
			"last_build_time":    snapshot.LastBuildTime,
			"last_build_success": snapshot.LastBuildSuccess,
			"index_file_count":   snapshot.IndexFileCount,
			"watcher_active":     snapshot.WatcherActive,
			"sse_clients":        sseClients,
		},
		"semantic": map[string]any{
			"enabled":  snapshot.SemanticEnabled,
			"provider": snapshot.SemanticProvider,
			"model":    snapshot.SemanticModel,
		},
		"cache": map[string]any{
			"version":        snapshot.CacheVersion,
			"total_entries":  snapshot.CacheTotalEntries,
			"legacy_entries": snapshot.CacheLegacyEntries,
			"total_size":     snapshot.CacheTotalSize,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetIndex handles GET /api/v1/index
func (s *HTTPServer) handleGetIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.exporter == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	ctx := r.Context()
	index, err := s.exporter.ToGraphIndex(ctx, s.memoryRoot)
	if err != nil {
		s.logger.Error("failed to export index", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to export index", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, index)
}

// handleRebuild handles POST /api/v1/rebuild
func (s *HTTPServer) handleRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.rebuildHandler == nil {
		s.writeError(w, http.StatusServiceUnavailable, "rebuild handler not available", "")
		return
	}

	// Check if already rebuilding
	if s.rebuildHandler.IsRebuilding() {
		s.writeJSON(w, http.StatusConflict, RebuildResponse{
			Status:  "in_progress",
			Message: "A rebuild is already in progress",
		})
		return
	}

	// Check for force flag (clear graph first)
	force := r.URL.Query().Get("force") == "true"

	if force {
		s.logger.Info("clearing graph before rebuild")
		if err := s.rebuildHandler.ClearGraph(); err != nil {
			s.logger.Error("failed to clear graph", "error", err)
			s.writeError(w, http.StatusInternalServerError, "failed to clear graph", err.Error())
			return
		}
	}

	// Trigger rebuild in background
	go func() {
		s.logger.Info("starting rebuild via API", "force", force)
		if err := s.rebuildHandler.Rebuild(); err != nil {
			s.logger.Error("rebuild failed", "error", err)
		} else {
			s.logger.Info("rebuild completed via API")
		}
	}()

	s.writeJSON(w, http.StatusAccepted, RebuildResponse{
		Status:  "started",
		Message: "Rebuild started in background",
	})
}

// handleSearch handles POST /api/v1/search
func (s *HTTPServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.graphManager == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if req.Query == "" {
		s.writeError(w, http.StatusBadRequest, "query is required", "")
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	ctx := r.Context()
	results, err := s.graphManager.Search(ctx, req.Query, limit, req.Category)
	if err != nil {
		s.logger.Error("search failed", "error", err)
		s.writeError(w, http.StatusInternalServerError, "search failed", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, SearchResponse{
		Results: results,
		Count:   len(results),
	})
}

// handleGetFile handles GET /api/v1/files/{path}
func (s *HTTPServer) handleGetFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.exporter == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	// Extract path from URL (everything after /api/v1/files/)
	prefix := "/api/v1/files/"
	if len(r.URL.Path) <= len(prefix) {
		s.writeError(w, http.StatusBadRequest, "path is required", "")
		return
	}

	encodedPath := r.URL.Path[len(prefix):]
	filePath, err := url.PathUnescape(encodedPath)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid path encoding", err.Error())
		return
	}

	// Parse related files limit from query params (default 10)
	relatedLimit := 10
	if limitStr := r.URL.Query().Get("related_limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l >= 0 {
			relatedLimit = l
		}
	}

	ctx := r.Context()

	// Get file as FileEntry with related files
	fileEntry, err := s.exporter.GetFileEntry(ctx, filePath, relatedLimit)
	if err != nil {
		s.logger.Error("failed to get file", "path", filePath, "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get file", err.Error())
		return
	}

	if fileEntry == nil {
		s.writeError(w, http.StatusNotFound, "file not found", "")
		return
	}

	s.writeJSON(w, http.StatusOK, FileMetadataResponse{
		File: fileEntry,
	})
}

// handleRecentFiles handles GET /api/v1/files/recent
func (s *HTTPServer) handleRecentFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.graphManager == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	// Parse query parameters
	days := 7
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	ctx := r.Context()
	files, err := s.graphManager.GetRecentFiles(ctx, days, limit)
	if err != nil {
		s.logger.Error("failed to get recent files", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get recent files", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, RecentFilesResponse{
		Files: files,
		Count: len(files),
	})
}

// handleRelatedFiles handles GET /api/v1/files/related
func (s *HTTPServer) handleRelatedFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.graphManager == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		s.writeError(w, http.StatusBadRequest, "path query parameter is required", "")
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	ctx := r.Context()
	files, err := s.graphManager.GetRelatedFiles(ctx, filePath, limit)
	if err != nil {
		s.logger.Error("failed to get related files", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get related files", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, RelatedFilesResponse{
		Files: files,
		Count: len(files),
	})
}

// handleEntitySearch handles GET /api/v1/entities/search
func (s *HTTPServer) handleEntitySearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.graphManager == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	entity := r.URL.Query().Get("entity")
	if entity == "" {
		s.writeError(w, http.StatusBadRequest, "entity query parameter is required", "")
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	ctx := r.Context()

	// Use the queries directly to search by entity
	results, err := s.graphManager.Queries().SearchByEntity(ctx, entity, limit)
	if err != nil {
		s.logger.Error("failed to search entities", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to search entities", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, EntitySearchResponse{
		Results: results,
		Count:   len(results),
	})
}

// writeJSON writes a JSON response
func (s *HTTPServer) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode JSON response", "error", err)
	}
}

// writeError writes an error response
func (s *HTTPServer) writeError(w http.ResponseWriter, status int, message string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIError{
		Error:   message,
		Details: details,
	})
}

// GetIndex exports the current index (for SSE hub to include in events)
func (s *HTTPServer) GetIndex(ctx context.Context) (*types.GraphIndex, error) {
	if s.exporter == nil {
		return nil, fmt.Errorf("graph exporter not available")
	}
	return s.exporter.ToGraphIndex(ctx, s.memoryRoot)
}
