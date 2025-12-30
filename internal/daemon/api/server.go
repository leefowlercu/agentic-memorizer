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

	// Files endpoints
	mux.HandleFunc("/api/v1/files", withTimeout(s.handleFilesQuery, apiTimeout))       // Unified file search
	mux.HandleFunc("/api/v1/files/index", withTimeout(s.handleFilesIndex, apiTimeout)) // File index export
	mux.HandleFunc("/api/v1/files/", withTimeout(s.handleGetFile, apiTimeout))         // Catch-all for /api/v1/files/{path}

	// Facts endpoints
	mux.HandleFunc("/api/v1/facts/index", withTimeout(s.handleFactsIndex, apiTimeout)) // Facts listing
	mux.HandleFunc("/api/v1/facts/", withTimeout(s.handleGetFact, apiTimeout))         // Catch-all for /api/v1/facts/{id}

	// Management endpoints
	mux.HandleFunc("/api/v1/rebuild", withTimeout(s.handleRebuild, apiTimeout))

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
	// Check for sync flag (remove stale graph nodes after rebuild)
	sync := r.URL.Query().Get("sync") == "true"

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
		s.logger.Info("starting rebuild via API", "force", force, "sync", sync)
		if err := s.rebuildHandler.RebuildWithSync(sync); err != nil {
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

// handleFilesQuery handles GET /api/v1/files with unified query parameters
// This replaces POST /api/v1/search, GET /api/v1/files/recent, and GET /api/v1/entities/search
func (s *HTTPServer) handleFilesQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.graphManager == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	// Parse query parameters
	q := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	entity := r.URL.Query().Get("entity")
	tag := r.URL.Query().Get("tag")
	topic := r.URL.Query().Get("topic")

	days := 0
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	ctx := r.Context()
	var results []graph.SearchResult
	var err error

	queries := s.graphManager.Queries()

	// Route to appropriate query based on parameters (in priority order)
	switch {
	case entity != "":
		results, err = queries.SearchByEntity(ctx, entity, limit)
	case tag != "":
		results, err = queries.SearchByTag(ctx, tag, limit)
	case topic != "":
		results, err = queries.SearchByTopic(ctx, topic, limit)
	case days > 0:
		results, err = queries.GetRecentFiles(ctx, days, limit)
	case q != "":
		results, err = s.graphManager.Search(ctx, q, limit, category)
	case category != "":
		results, err = queries.SearchByCategory(ctx, category, limit)
	default:
		// No filter - return recent files (default 7 days)
		results, err = queries.GetRecentFiles(ctx, 7, limit)
	}

	if err != nil {
		s.logger.Error("files query failed", "error", err)
		s.writeError(w, http.StatusInternalServerError, "query failed", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, FilesQueryResponse{
		Files: results,
		Count: len(results),
		Query: FilesQueryParams{
			Q:        q,
			Category: category,
			Days:     days,
			Entity:   entity,
			Tag:      tag,
			Topic:    topic,
			Limit:    limit,
		},
	})
}

// handleFilesIndex handles GET /api/v1/files/index
func (s *HTTPServer) handleFilesIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.exporter == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	ctx := r.Context()
	index, err := s.exporter.ToFileIndex(ctx, s.memoryRoot)
	if err != nil {
		s.logger.Error("failed to export index", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to export index", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, FilesIndexResponse{Index: index})
}

// handleFactsIndex handles GET /api/v1/facts/index
func (s *HTTPServer) handleFactsIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.graphManager == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	ctx := r.Context()
	facts := s.graphManager.Facts()

	factNodes, err := facts.List(ctx)
	if err != nil {
		s.logger.Error("failed to list facts", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list facts", err.Error())
		return
	}

	// Convert FactNode to types.Fact
	typeFacts := make([]types.Fact, len(factNodes))
	for i, fn := range factNodes {
		typeFacts[i] = types.Fact{
			ID:        fn.ID,
			Content:   fn.Content,
			CreatedAt: fn.CreatedAt,
			UpdatedAt: fn.UpdatedAt,
			Source:    fn.Source,
		}
	}

	s.writeJSON(w, http.StatusOK, FactsIndexResponse{
		Facts: typeFacts,
		Count: len(typeFacts),
		Stats: FactsStats{
			TotalFacts: len(typeFacts),
			MaxFacts:   graph.MaxTotalFacts,
		},
	})
}

// handleGetFact handles GET /api/v1/facts/{id}
func (s *HTTPServer) handleGetFact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	if s.graphManager == nil {
		s.writeError(w, http.StatusServiceUnavailable, "graph not available", "")
		return
	}

	// Extract ID from URL (everything after /api/v1/facts/)
	prefix := "/api/v1/facts/"
	if len(r.URL.Path) <= len(prefix) {
		s.writeError(w, http.StatusBadRequest, "fact ID is required", "")
		return
	}

	factID := r.URL.Path[len(prefix):]
	if factID == "" || factID == "index" {
		// This shouldn't happen due to routing, but handle it
		s.writeError(w, http.StatusBadRequest, "fact ID is required", "")
		return
	}

	ctx := r.Context()
	facts := s.graphManager.Facts()

	factNode, err := facts.GetByID(ctx, factID)
	if err != nil {
		s.logger.Error("failed to get fact", "id", factID, "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get fact", err.Error())
		return
	}

	if factNode == nil {
		s.writeError(w, http.StatusNotFound, "fact not found", "")
		return
	}

	s.writeJSON(w, http.StatusOK, FactResponse{
		Fact: &types.Fact{
			ID:        factNode.ID,
			Content:   factNode.Content,
			CreatedAt: factNode.CreatedAt,
			UpdatedAt: factNode.UpdatedAt,
			Source:    factNode.Source,
		},
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
func (s *HTTPServer) GetIndex(ctx context.Context) (*types.FileIndex, error) {
	if s.exporter == nil {
		return nil, fmt.Errorf("graph exporter not available")
	}
	return s.exporter.ToFileIndex(ctx, s.memoryRoot)
}
