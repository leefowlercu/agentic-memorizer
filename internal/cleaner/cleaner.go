package cleaner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// ErrAlreadyStarted is returned when Start() is called on an already-started cleaner.
var ErrAlreadyStarted = errors.New("cleaner already started")

// ReconcileResult contains statistics from a reconciliation run.
type ReconcileResult struct {
	FilesChecked int
	StaleFound   int
	StaleRemoved int
	Errors       int
	Skipped      bool // True if reconciliation was skipped (e.g., empty discovered paths)
	Duration     time.Duration
}

// Cleaner handles file deletion cleanup from registry and graph.
type Cleaner struct {
	registry registry.Registry
	graph    graph.Graph
	bus      events.Bus
	logger   *slog.Logger

	mu          sync.Mutex
	started     bool
	unsubscribe func()

	// wg tracks in-flight operations for graceful shutdown
	wg sync.WaitGroup
}

// CleanerOption configures the Cleaner.
type CleanerOption func(*Cleaner)

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) CleanerOption {
	return func(c *Cleaner) {
		c.logger = logger
	}
}

// New creates a new Cleaner.
func New(reg registry.Registry, g graph.Graph, bus events.Bus, opts ...CleanerOption) *Cleaner {
	c := &Cleaner{
		registry: reg,
		graph:    g,
		bus:      bus,
		logger:   slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Start subscribes to FileDeleted events.
// Returns ErrAlreadyStarted if called more than once without Stop().
func (c *Cleaner) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return ErrAlreadyStarted
	}

	c.unsubscribe = c.bus.Subscribe(events.FileDeleted, c.handleDelete)
	c.started = true
	c.logger.Info("cleaner started")
	return nil
}

// Stop unsubscribes from events and waits for in-flight operations to complete.
func (c *Cleaner) Stop() error {
	c.mu.Lock()
	if c.unsubscribe != nil {
		c.unsubscribe()
		c.unsubscribe = nil
	}
	c.started = false
	c.mu.Unlock()

	// Wait for in-flight operations to complete (with timeout)
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All operations completed
	case <-time.After(35 * time.Second):
		// Timeout slightly longer than handleDelete's 30s timeout
		c.logger.Warn("cleaner stop timed out waiting for in-flight operations")
	}

	c.logger.Info("cleaner stopped")
	return nil
}

// DeleteFile removes a file from registry and graph.
// Called by event handler and reconciliation.
func (c *Cleaner) DeleteFile(ctx context.Context, path string) error {
	// Always try registry deletion
	if err := c.registry.DeleteFileState(ctx, path); err != nil {
		// Distinguish between "not found" (expected) and other errors (unexpected)
		if errors.Is(err, registry.ErrPathNotFound) {
			c.logger.Debug("registry delete: file not in registry", "path", path)
		} else {
			c.logger.Warn("registry delete failed", "path", path, "error", err)
		}
	}

	// Try graph deletion (may fail if degraded or unavailable)
	if c.graph != nil {
		if err := c.graph.DeleteFile(ctx, path); err != nil {
			c.logger.Warn("graph delete failed", "path", path, "error", err)
			// Don't return error - registry was cleaned
		}
	}

	return nil
}

// Reconcile compares discovered paths against file_state and cleans up stale entries.
// If discoveredPaths is empty but file_state has entries, reconciliation is skipped
// as a safeguard against accidental mass deletion (e.g., filter misconfiguration).
func (c *Cleaner) Reconcile(ctx context.Context, parentPath string, discoveredPaths map[string]struct{}) (*ReconcileResult, error) {
	start := time.Now()
	result := &ReconcileResult{}

	// Get all file_state entries under this parent path
	states, err := c.registry.ListFileStates(ctx, parentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list file states; %w", err)
	}

	result.FilesChecked = len(states)

	// Safeguard: if discoveredPaths is empty but we have file_state entries,
	// something might be wrong (filter misconfiguration, permissions issue).
	// Skip reconciliation to prevent accidental mass deletion.
	if len(discoveredPaths) == 0 && len(states) > 0 {
		c.logger.Warn("reconciliation skipped: no files discovered but file_state has entries",
			"parent_path", parentPath,
			"file_state_count", len(states),
		)
		result.Skipped = true
		result.Duration = time.Since(start)
		return result, nil
	}

	// Find stale entries (in file_state but not in discovered)
	for i, state := range states {
		// Check context periodically (every 100 files) to support cancellation
		if i%100 == 0 {
			if err := ctx.Err(); err != nil {
				c.logger.Debug("reconciliation cancelled", "processed", i, "total", len(states))
				result.Duration = time.Since(start)
				return result, err
			}
		}

		if _, exists := discoveredPaths[state.Path]; !exists {
			result.StaleFound++

			// Clean up stale entry
			if err := c.DeleteFile(ctx, state.Path); err != nil {
				c.logger.Warn("failed to clean up stale file",
					"path", state.Path,
					"error", err)
				result.Errors++
			} else {
				c.logger.Debug("cleaned up stale file", "path", state.Path)
				result.StaleRemoved++
			}
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// handleDelete is the event handler for FileDeleted events.
func (c *Cleaner) handleDelete(e events.Event) {
	fe, ok := e.Payload.(events.FileEvent)
	if !ok {
		c.logger.Warn("invalid FileDeleted payload type")
		return
	}

	// Validate path is non-empty
	if fe.Path == "" {
		c.logger.Warn("ignoring FileDeleted event with empty path")
		return
	}

	// Track in-flight operation for graceful shutdown
	c.wg.Add(1)
	defer c.wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.logger.Debug("cleaning up deleted file", "path", fe.Path)

	if err := c.DeleteFile(ctx, fe.Path); err != nil {
		c.logger.Error("delete handler failed", "path", fe.Path, "error", err)
	}
}

// IsStarted returns true if the cleaner has been started.
func (c *Cleaner) IsStarted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}
