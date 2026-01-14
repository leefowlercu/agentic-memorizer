package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Port int
	Bind string
}

// RebuildResult contains the result of a rebuild operation.
type RebuildResult struct {
	Status        string `json:"status"`
	FilesQueued   int    `json:"files_queued"`
	DirsProcessed int    `json:"dirs_processed"`
	Duration      string `json:"duration"`
	Error         string `json:"error,omitempty"`
}

// RebuildFunc is a function that triggers a rebuild operation.
type RebuildFunc func(ctx context.Context, full bool) (*RebuildResult, error)

// Server is the HTTP server for daemon health endpoints.
// It is safe for concurrent use.
type Server struct {
	mu             sync.RWMutex
	health         *HealthManager
	config         ServerConfig
	server         *http.Server
	router         *chi.Mux
	mcpHandler     http.Handler
	metricsHandler http.Handler
	rebuildFunc    RebuildFunc
}

// NewServer creates a new HTTP server with the given health manager and config.
func NewServer(health *HealthManager, config ServerConfig) *Server {
	s := &Server{
		health: health,
		config: config,
		router: chi.NewRouter(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes.
func (s *Server) setupRoutes() {
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/readyz", s.handleReadyz)
	s.router.Post("/rebuild", s.handleRebuild)

	// Mount MCP endpoints if handler is set
	if s.mcpHandler != nil {
		s.router.Mount("/mcp", s.mcpHandler)
	}

	// Mount metrics endpoint if handler is set
	if s.metricsHandler != nil {
		s.router.Handle("/metrics", s.metricsHandler)
	}
}

// SetMCPHandler sets the MCP server handler.
func (s *Server) SetMCPHandler(handler http.Handler) {
	s.mcpHandler = handler
	// Re-setup routes to include MCP
	s.router = chi.NewRouter()
	s.setupRoutes()
}

// SetMetricsHandler sets the Prometheus metrics handler.
func (s *Server) SetMetricsHandler(handler http.Handler) {
	s.metricsHandler = handler
	// Re-setup routes to include metrics
	s.router = chi.NewRouter()
	s.setupRoutes()
}

// SetRebuildFunc sets the function to call when rebuild is requested.
func (s *Server) SetRebuildFunc(fn RebuildFunc) {
	s.rebuildFunc = fn
}

// Handler returns the HTTP handler for testing purposes.
func (s *Server) Handler() http.Handler {
	return s.router
}

// LivezResponse is the response format for /healthz endpoint.
type LivezResponse struct {
	Status string `json:"status"`
}

// handleHealthz handles the /healthz endpoint (liveness probe).
// Returns 200 OK if the daemon process is alive.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	response := LivezResponse{
		Status: "alive",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleReadyz handles the /readyz endpoint (readiness probe).
// Returns 200 OK with health status for both healthy and degraded states.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	status := s.health.Status()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// handleRebuild handles the /rebuild endpoint.
// Triggers a rebuild of the knowledge graph.
func (s *Server) handleRebuild(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.rebuildFunc == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(RebuildResult{
			Status: "error",
			Error:  "rebuild not available",
		})
		return
	}

	// Check for full rebuild flag
	full := r.URL.Query().Get("full") == "true"

	// Execute rebuild
	result, err := s.rebuildFunc(r.Context(), full)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(RebuildResult{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// Start starts the HTTP server and blocks until it's stopped.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Bind, s.config.Port)

	s.mu.Lock()
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.router,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}
	server := s.server
	s.mu.Unlock()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error; %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	server := s.server
	s.mu.RUnlock()

	if server == nil {
		return nil
	}

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown http server; %w", err)
	}

	return nil
}
