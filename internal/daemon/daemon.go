// Package daemon provides the core daemon process management functionality.
// It implements lifecycle management, component coordination, and health monitoring
// for the memorizer daemon.
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// DaemonState represents the lifecycle state of the daemon.
type DaemonState string

const (
	// DaemonStateStarting indicates the daemon is initializing.
	DaemonStateStarting DaemonState = "starting"

	// DaemonStateRunning indicates all components are healthy and serving.
	DaemonStateRunning DaemonState = "running"

	// DaemonStateDegraded indicates some non-critical components have failed.
	DaemonStateDegraded DaemonState = "degraded"

	// DaemonStateStopping indicates graceful shutdown is in progress.
	DaemonStateStopping DaemonState = "stopping"

	// DaemonStateStopped indicates the daemon has terminated.
	DaemonStateStopped DaemonState = "stopped"
)

// IsTerminal returns true if this state is a terminal state (no further transitions).
func (s DaemonState) IsTerminal() bool {
	return s == DaemonStateStopped
}

// CanTransitionTo returns true if transitioning to the target state is valid.
func (s DaemonState) CanTransitionTo(target DaemonState) bool {
	switch s {
	case DaemonStateStarting:
		return target == DaemonStateRunning || target == DaemonStateStopped
	case DaemonStateRunning:
		return target == DaemonStateDegraded || target == DaemonStateStopping
	case DaemonStateDegraded:
		return target == DaemonStateRunning || target == DaemonStateStopping
	case DaemonStateStopping:
		return target == DaemonStateStopped
	case DaemonStateStopped:
		return false
	default:
		return false
	}
}

// DaemonConfig holds the configuration values for the daemon.
type DaemonConfig struct {
	// HTTPPort is the port for the HTTP health check server.
	HTTPPort int

	// HTTPBind is the address to bind the HTTP server.
	HTTPBind string

	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout time.Duration

	// PIDFile is the path to the PID file.
	PIDFile string
}

// DefaultDaemonConfig returns the default daemon configuration.
func DefaultDaemonConfig() DaemonConfig {
	return DaemonConfig{
		HTTPPort:        7600,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 30 * time.Second,
		PIDFile:         "~/.config/memorizer/daemon.pid",
	}
}

// ConfigReloadFunc is a callback function invoked when config is reloaded.
type ConfigReloadFunc func() error

// Daemon is the main daemon process manager.
// It is safe for concurrent use.
type Daemon struct {
	mu              sync.RWMutex
	config          DaemonConfig
	state           DaemonState
	server          *Server
	health          *HealthManager
	pidFile         *PIDFile
	reloadCallbacks []ConfigReloadFunc
}

// NewDaemon creates a new Daemon instance with the given configuration.
func NewDaemon(cfg DaemonConfig) *Daemon {
	health := NewHealthManager()
	server := NewServer(health, ServerConfig{
		Port: cfg.HTTPPort,
		Bind: cfg.HTTPBind,
	})
	pidFile := NewPIDFile(cfg.PIDFile)

	return &Daemon{
		config:  cfg,
		state:   DaemonStateStopped,
		server:  server,
		health:  health,
		pidFile: pidFile,
	}
}

// State returns the current daemon state.
func (d *Daemon) State() DaemonState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.state
}

// setState sets the daemon state with proper locking.
func (d *Daemon) setState(state DaemonState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state = state
}

// Health returns the current aggregate health status.
func (d *Daemon) Health() HealthStatus {
	return d.health.Status()
}

// OnConfigReload registers a callback to be invoked when config is reloaded.
func (d *Daemon) OnConfigReload(fn ConfigReloadFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.reloadCallbacks = append(d.reloadCallbacks, fn)
}

// UpdateComponentHealth updates health status for multiple components.
func (d *Daemon) UpdateComponentHealth(statuses map[string]ComponentHealth) {
	for name, health := range statuses {
		d.health.UpdateComponent(name, health)
	}
}

// TriggerConfigReload invokes all registered config reload callbacks.
// Errors are logged and aggregated, but all callbacks are attempted.
// Returns an error if any callbacks failed.
func (d *Daemon) TriggerConfigReload() error {
	slog.Info("config reload triggered")

	// Copy callbacks under lock to avoid race with OnConfigReload
	d.mu.RLock()
	callbacks := make([]ConfigReloadFunc, len(d.reloadCallbacks))
	copy(callbacks, d.reloadCallbacks)
	d.mu.RUnlock()

	var failedCount int
	for i, fn := range callbacks {
		if err := fn(); err != nil {
			slog.Error("config reload callback failed",
				"callback_index", i,
				"error", err)
			failedCount++
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("%d of %d reload callbacks failed", failedCount, len(callbacks))
	}
	return nil
}

// Start starts the daemon and blocks until the context is canceled.
// It claims the PID file, starts the HTTP server, and then blocks until
// shutdown is requested.
func (d *Daemon) Start(ctx context.Context) error {
	d.setState(DaemonStateStarting)

	// Claim PID file
	if err := d.pidFile.CheckAndClaim(); err != nil {
		d.setState(DaemonStateStopped)
		return fmt.Errorf("failed to claim PID file; %w", err)
	}

	// Ensure PID file is cleaned up on exit (only after successful claim)
	defer func() { _ = d.pidFile.Remove() }()

	d.setState(DaemonStateRunning)
	slog.Info("daemon started",
		"state", d.State(),
	)

	// Start HTTP server in background
	serverErr := make(chan error, 1)
	go func() {
		if err := d.server.Start(ctx); err != nil {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			slog.Error("http server error", "error", err)
		}
	}

	// Graceful shutdown
	return d.Stop()
}

// Stop performs graceful shutdown of the daemon.
func (d *Daemon) Stop() error {
	d.setState(DaemonStateStopping)
	slog.Info("stopping daemon")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), d.config.ShutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := d.server.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shutdown http server", "error", err)
	}

	d.setState(DaemonStateStopped)
	slog.Info("daemon stopped")

	return nil
}
