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

// Start subscribes to PathDeleted events.
// Returns ErrAlreadyStarted if called more than once without Stop().
func (c *Cleaner) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return ErrAlreadyStarted
	}

	unsubPathDeleted := c.bus.Subscribe(events.PathDeleted, c.handlePathDeleted)
	unsubRemembered := c.bus.Subscribe(events.RememberedPathRemoved, c.handleRememberedPathRemoved)
	c.unsubscribe = func() {
		unsubPathDeleted()
		unsubRemembered()
	}
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
		// Timeout slightly longer than handlePathDeleted's 30s timeout
		c.logger.Warn("cleaner stop timed out waiting for in-flight operations")
	}

	c.logger.Info("cleaner stopped")
	return nil
}

// DeletePath removes a file or directory from registry and graph.
// Handles both file and directory deletions by attempting cleanup for both types.
// All operations are best-effort; errors are logged but don't fail the operation.
func (c *Cleaner) DeletePath(ctx context.Context, path string) error {
	// 0. Delete discovery state from registry
	if err := c.registry.DeleteDiscoveryState(ctx, path); err != nil {
		if errors.Is(err, registry.ErrPathNotFound) {
			c.logger.Debug("registry delete: path not in discovery", "path", path)
		} else {
			c.logger.Warn("registry discovery delete failed", "path", path, "error", err)
		}
	}

	// 0b. Bulk delete child discovery records (if path was a directory)
	if err := c.registry.DeleteDiscoveryStatesForPath(ctx, path); err != nil {
		c.logger.Warn("registry discovery bulk delete failed", "path", path, "error", err)
	}

	// 1. Delete file state from registry (if path was a file)
	if err := c.registry.DeleteFileState(ctx, path); err != nil {
		if errors.Is(err, registry.ErrPathNotFound) {
			c.logger.Debug("registry delete: path not in file_state", "path", path)
		} else {
			c.logger.Warn("registry delete failed", "path", path, "error", err)
		}
	}

	// 2. Bulk delete child file states (if path was a directory)
	if err := c.registry.DeleteFileStatesForPath(ctx, path); err != nil {
		c.logger.Warn("registry bulk delete failed", "path", path, "error", err)
	}

	// Skip graph operations if graph is unavailable
	if c.graph == nil {
		return nil
	}

	// 3. Delete File node from graph (succeeds if path was a file)
	if err := c.graph.DeleteFile(ctx, path); err != nil {
		c.logger.Debug("graph delete file failed", "path", path, "error", err)
	}

	// 4. Delete Directory node from graph (succeeds if path was a directory)
	if err := c.graph.DeleteDirectory(ctx, path); err != nil {
		c.logger.Debug("graph delete directory failed", "path", path, "error", err)
	}

	// 5. Delete all child File nodes from graph
	if err := c.graph.DeleteFilesUnderPath(ctx, path); err != nil {
		c.logger.Warn("graph delete files under path failed", "path", path, "error", err)
	}

	// 6. Delete all child Directory nodes from graph
	if err := c.graph.DeleteDirectoriesUnderPath(ctx, path); err != nil {
		c.logger.Warn("graph delete directories under path failed", "path", path, "error", err)
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

	// Get all discovery entries under this parent path
	discoveryStates, err := c.registry.ListDiscoveryStates(ctx, parentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list discovery states; %w", err)
	}

	result.FilesChecked = len(states)

	// Safeguard: if discoveredPaths is empty but we have file_state entries,
	// something might be wrong (filter misconfiguration, permissions issue).
	// Skip reconciliation to prevent accidental mass deletion.
	if len(discoveredPaths) == 0 && (len(states) > 0 || len(discoveryStates) > 0) {
		c.logger.Warn("reconciliation skipped: no files discovered but file_state has entries",
			"parent_path", parentPath,
			"file_state_count", len(states),
			"discovery_count", len(discoveryStates),
		)
		result.Skipped = true
		result.Duration = time.Since(start)
		return result, nil
	}

	staleFileStates := make(map[string]struct{})

	// Find stale entries (in file_state but not in discovered)
	for i, state := range states {
		// Check context periodically (every 100 files) to support cancellation
		if i%100 == 0 {
			if err := ctx.Err(); err != nil {
				c.logger.Debug("reconciliation canceled", "processed", i, "total", len(states))
				result.Duration = time.Since(start)
				return result, err
			}
		}

		if _, exists := discoveredPaths[state.Path]; !exists {
			staleFileStates[state.Path] = struct{}{}
			result.StaleFound++

			// Clean up stale entry
			if err := c.DeletePath(ctx, state.Path); err != nil {
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

	// Clean up stale discovery-only entries (no file_state)
	for i, state := range discoveryStates {
		if i%100 == 0 {
			if err := ctx.Err(); err != nil {
				c.logger.Debug("reconciliation canceled", "processed", i, "total", len(discoveryStates))
				result.Duration = time.Since(start)
				return result, err
			}
		}

		if _, exists := discoveredPaths[state.Path]; exists {
			continue
		}
		if _, alreadyHandled := staleFileStates[state.Path]; alreadyHandled {
			continue
		}

		if err := c.registry.DeleteDiscoveryState(ctx, state.Path); err != nil {
			if errors.Is(err, registry.ErrPathNotFound) {
				continue
			}
			c.logger.Warn("failed to clean up stale discovery state",
				"path", state.Path,
				"error", err)
			result.Errors++
		} else {
			c.logger.Debug("cleaned up stale discovery state", "path", state.Path)
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// handlePathDeleted is the event handler for PathDeleted events.
func (c *Cleaner) handlePathDeleted(e events.Event) {
	fe, ok := e.Payload.(*events.FileEvent)
	if !ok {
		c.logger.Warn("invalid PathDeleted payload type")
		return
	}

	// Validate path is non-empty
	if fe.Path == "" {
		c.logger.Warn("ignoring PathDeleted event with empty path")
		return
	}

	// Check if still running and track in-flight operation atomically
	// This prevents a race between wg.Add() and wg.Wait() in Stop()
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	c.wg.Add(1)
	c.mu.Unlock()
	defer c.wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.logger.Debug("cleaning up deleted file", "path", fe.Path)

	if err := c.DeletePath(ctx, fe.Path); err != nil {
		c.logger.Error("delete handler failed", "path", fe.Path, "error", err)
	}
}

// handleRememberedPathRemoved handles cleanup for a removed remembered path.
func (c *Cleaner) handleRememberedPathRemoved(e events.Event) {
	pe, ok := e.Payload.(*events.RememberedPathRemovedEvent)
	if !ok {
		c.logger.Warn("invalid RememberedPathRemoved payload type")
		return
	}

	if pe.Path == "" {
		c.logger.Warn("ignoring RememberedPathRemoved event with empty path")
		return
	}

	if pe.KeepData {
		c.logger.Debug("skipping cleanup for remembered path", "path", pe.Path, "reason", pe.Reason)
		return
	}

	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	c.wg.Add(1)
	c.mu.Unlock()
	defer c.wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.logger.Debug("cleaning up removed remembered path", "path", pe.Path, "reason", pe.Reason)

	if err := c.DeletePath(ctx, pe.Path); err != nil {
		c.logger.Error("remembered path cleanup failed", "path", pe.Path, "error", err)
	}
}

// IsStarted returns true if the cleaner has been started.
func (c *Cleaner) IsStarted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}
