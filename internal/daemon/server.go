package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Port int
	Bind string
}

// Server is the HTTP server for daemon health endpoints.
type Server struct {
	health *HealthManager
	config ServerConfig
	server *http.Server
	router *chi.Mux
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

// Start starts the HTTP server and blocks until it's stopped.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Bind, s.config.Port)

	s.server = &http.Server{
		Addr:    addr,
		Handler: s.router,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error; %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown http server; %w", err)
	}

	return nil
}
