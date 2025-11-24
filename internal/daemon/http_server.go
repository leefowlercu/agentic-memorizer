package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// HTTPServer manages the daemon's unified HTTP API
type HTTPServer struct {
	server  *http.Server
	hub     *SSEHub
	metrics *HealthMetrics
	logger  *slog.Logger
	mu      sync.Mutex
}

// NewHTTPServer creates a new unified HTTP server
func NewHTTPServer(hub *SSEHub, metrics *HealthMetrics, logger *slog.Logger) *HTTPServer {
	return &HTTPServer{
		hub:     hub,
		metrics: metrics,
		logger:  logger,
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
			s.logger.Warn("HTTP server shutdown failed during restart", "error", err)
		}
		s.server = nil
	}

	// Don't start if disabled (port 0)
	if port == 0 {
		s.logger.Info("HTTP server disabled")
		return nil
	}

	// Create router with all endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/notifications/stream", s.hub.handleSSEStream)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	s.logger.Info("starting HTTP server", "port", port)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server failed", "error", err)
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
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
