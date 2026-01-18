package walker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/fsutil"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// Walker scans directories and publishes file discovery events.
type Walker interface {
	// Walk performs a full walk of the specified path.
	Walk(ctx context.Context, path string) error

	// WalkAll walks all remembered paths.
	WalkAll(ctx context.Context) error

	// WalkIncremental walks a path but only processes files that have changed.
	WalkIncremental(ctx context.Context, path string) error

	// WalkAllIncremental walks all remembered paths incrementally.
	WalkAllIncremental(ctx context.Context) error

	// Stats returns current walker statistics.
	Stats() WalkerStats

	// DrainDiscoveredPaths returns paths discovered during the last walk and clears the set.
	// Returns nil if no walk has occurred. Used for reconciliation against file_state.
	DrainDiscoveredPaths() map[string]struct{}
}

// WalkerStats contains statistics about walker activity.
type WalkerStats struct {
	FilesDiscovered int64
	FilesSkipped    int64
	FilesUnchanged  int64
	DirsTraversed   int64
	LastWalkAt      time.Time
	LastWalkPath    string
	IsWalking       bool
}

// WalkerOption configures the Walker.
type WalkerOption func(*walker)

// WithPaceInterval sets the interval between file discoveries to prevent overwhelming downstream.
func WithPaceInterval(d time.Duration) WalkerOption {
	return func(w *walker) {
		w.paceInterval = d
	}
}

// WithBatchSize sets the number of files to process before a pace delay.
func WithBatchSize(size int) WalkerOption {
	return func(w *walker) {
		w.batchSize = size
	}
}

// walker implements the Walker interface.
type walker struct {
	registry registry.Registry
	bus      events.Bus

	paceInterval time.Duration
	batchSize    int

	mu              sync.RWMutex
	stats           WalkerStats
	discoveredPaths map[string]struct{}
}

// New creates a new Walker with the given dependencies.
func New(reg registry.Registry, bus events.Bus, opts ...WalkerOption) Walker {
	w := &walker{
		registry:     reg,
		bus:          bus,
		paceInterval: 0,
		batchSize:    100,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Walk performs a full walk of the specified path.
func (w *walker) Walk(ctx context.Context, path string) error {
	return w.walkPath(ctx, path, false)
}

// WalkAll walks all remembered paths.
func (w *walker) WalkAll(ctx context.Context) error {
	paths, err := w.registry.ListPaths(ctx)
	if err != nil {
		return fmt.Errorf("failed to list remembered paths; %w", err)
	}

	// Initialize discovered paths map for reconciliation
	w.mu.Lock()
	w.discoveredPaths = make(map[string]struct{})
	w.mu.Unlock()

	for _, rp := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := w.Walk(ctx, rp.Path); err != nil {
			return fmt.Errorf("failed to walk %s; %w", rp.Path, err)
		}
	}

	return nil
}

// WalkAllIncremental walks all remembered paths incrementally.
func (w *walker) WalkAllIncremental(ctx context.Context) error {
	paths, err := w.registry.ListPaths(ctx)
	if err != nil {
		return fmt.Errorf("failed to list remembered paths; %w", err)
	}

	// Initialize discovered paths map for reconciliation
	w.mu.Lock()
	w.discoveredPaths = make(map[string]struct{})
	w.mu.Unlock()

	for _, rp := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := w.WalkIncremental(ctx, rp.Path); err != nil {
			return fmt.Errorf("failed to walk %s; %w", rp.Path, err)
		}
	}

	return nil
}

// WalkIncremental walks a path but only processes files that have changed.
func (w *walker) WalkIncremental(ctx context.Context, path string) error {
	return w.walkPath(ctx, path, true)
}

// Stats returns current walker statistics.
func (w *walker) Stats() WalkerStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// DrainDiscoveredPaths returns paths discovered during the last walk and clears the set.
// Returns nil if no walk has occurred. Used for reconciliation against file_state.
func (w *walker) DrainDiscoveredPaths() map[string]struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	paths := w.discoveredPaths
	w.discoveredPaths = nil
	return paths
}

// walkPath performs the actual directory walk.
func (w *walker) walkPath(ctx context.Context, path string, incremental bool) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path; %w", err)
	}

	// Verify path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat path; %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Get the remembered path and its config
	rp, err := w.registry.FindContainingPath(ctx, absPath)
	if err != nil {
		return fmt.Errorf("path not remembered; %w", err)
	}

	// Create filter from config
	filter := NewFilter(rp.Config)

	// Update stats
	w.mu.Lock()
	w.stats.IsWalking = true
	w.stats.LastWalkPath = absPath
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.stats.IsWalking = false
		w.stats.LastWalkAt = time.Now()
		w.mu.Unlock()

		// Update last_walk_at in registry
		_ = w.registry.UpdatePathLastWalk(ctx, rp.Path, time.Now())
	}()

	var filesInBatch int

	err = filepath.WalkDir(absPath, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Check context
		if err := ctx.Err(); err != nil {
			return err
		}

		// Skip symlinks
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		// Handle directories
		if d.IsDir() {
			// Skip the root directory itself from filtering
			if filePath == absPath {
				w.mu.Lock()
				w.stats.DirsTraversed++
				w.mu.Unlock()
				return nil
			}

			if !filter.ShouldProcessDir(filePath) {
				w.mu.Lock()
				w.stats.FilesSkipped++
				w.mu.Unlock()
				return fs.SkipDir
			}

			w.mu.Lock()
			w.stats.DirsTraversed++
			w.mu.Unlock()
			return nil
		}

		// Check if file should be processed
		if !filter.ShouldProcessFile(filePath) {
			w.mu.Lock()
			w.stats.FilesSkipped++
			w.mu.Unlock()
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		// Track discovered path for reconciliation
		// This happens before incremental check so unchanged files are still tracked
		w.mu.Lock()
		if w.discoveredPaths != nil {
			w.discoveredPaths[filePath] = struct{}{}
		}
		w.mu.Unlock()

		// For incremental walks, check if file has changed
		if incremental {
			changed, err := w.hasFileChanged(ctx, filePath, info)
			if err != nil {
				return nil // Skip on error
			}
			if !changed {
				w.mu.Lock()
				w.stats.FilesUnchanged++
				w.mu.Unlock()
				return nil
			}
		}

		// Compute content hash
		contentHash, err := fsutil.HashFile(filePath)
		if err != nil {
			return nil // Skip files we can't hash
		}

		// Publish file discovered event
		event := events.Event{
			Type:      events.FileDiscovered,
			Timestamp: time.Now(),
			Payload: &events.FileEvent{
				Path:        filePath,
				ContentHash: contentHash,
				Size:        info.Size(),
				ModTime:     info.ModTime(),
				IsNew:       !incremental,
			},
		}

		if err := w.bus.Publish(ctx, event); err != nil {
			return fmt.Errorf("failed to publish event; %w", err)
		}

		w.mu.Lock()
		w.stats.FilesDiscovered++
		w.mu.Unlock()

		// Apply pacing
		filesInBatch++
		if w.paceInterval > 0 && filesInBatch >= w.batchSize {
			filesInBatch = 0
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(w.paceInterval):
			}
		}

		return nil
	})

	return err
}

// hasFileChanged checks if a file has changed since last analysis.
func (w *walker) hasFileChanged(ctx context.Context, path string, info fs.FileInfo) (bool, error) {
	state, err := w.registry.GetFileState(ctx, path)
	if err != nil {
		// File not in registry, treat as changed (new file)
		return true, nil
	}

	// Check mod time and size first (quick check)
	if state.ModTime.Equal(info.ModTime()) && state.Size == info.Size() {
		return false, nil
	}

	// File metadata changed, likely content changed too
	return true, nil
}

// computeFileHash computes the SHA-256 hash of a file's contents.
