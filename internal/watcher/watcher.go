package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/metrics"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
)

// Watcher monitors filesystem changes and publishes events.
type Watcher interface {
	// Watch starts watching a path and its subdirectories.
	Watch(path string) error

	// Unwatch stops watching a path.
	Unwatch(path string) error

	// WatchedPaths returns the list of currently watched paths.
	WatchedPaths() []string

	// Start begins processing filesystem events.
	Start(ctx context.Context) error

	// Stop stops the watcher.
	Stop() error

	// Stats returns current watcher statistics.
	Stats() WatcherStats

	// CollectMetrics implements metrics.MetricsProvider.
	CollectMetrics(ctx context.Context) error

	// Errors reports fatal watcher errors.
	Errors() <-chan error
}

// WatcherStats contains statistics about watcher activity.
type WatcherStats struct {
	WatchedPaths    int
	EventsReceived  int64
	EventsPublished int64
	EventsCoalesced int64
	Errors          int64
	IsRunning       bool
	DegradedMode    bool
}

// WatcherOption configures the Watcher.
type WatcherOption func(*watcher)

// WithDebounceWindow sets the debounce window for event coalescing.
func WithDebounceWindow(d time.Duration) WatcherOption {
	return func(w *watcher) {
		w.debounceWindow = d
	}
}

// WithDeleteGracePeriod sets the grace period before publishing delete events.
func WithDeleteGracePeriod(d time.Duration) WatcherOption {
	return func(w *watcher) {
		w.deleteGracePeriod = d
	}
}

// WithLogger sets the logger for the watcher.
func WithLogger(logger *slog.Logger) WatcherOption {
	return func(w *watcher) {
		w.logger = logger
	}
}

// watcher implements the Watcher interface.
type watcher struct {
	fsWatcher *fsnotify.Watcher
	bus       events.Bus
	reg       registry.Registry
	coalescer *Coalescer
	logger    *slog.Logger

	debounceWindow    time.Duration
	deleteGracePeriod time.Duration

	mu           sync.RWMutex
	watchedPaths map[string]bool
	stats        WatcherStats
	running      bool
	stopCh       chan struct{}
	doneCh       chan struct{}
	stopOnce     sync.Once

	// errChan reports fatal errors (fsnotify error channel).
	errChan chan error
}

// New creates a new Watcher with the given dependencies.
func New(bus events.Bus, reg registry.Registry, opts ...WatcherOption) (Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher; %w", err)
	}

	w := &watcher{
		fsWatcher:         fsw,
		bus:               bus,
		reg:               reg,
		logger:            slog.Default(),
		debounceWindow:    500 * time.Millisecond,
		deleteGracePeriod: 5 * time.Second,
		watchedPaths:      make(map[string]bool),
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		errChan:           make(chan error, 1),
	}

	for _, opt := range opts {
		opt(w)
	}

	w.coalescer = NewCoalescer(w.debounceWindow, w.deleteGracePeriod)

	return w, nil
}

// Watch starts watching a path and its subdirectories.
func (w *watcher) Watch(path string) error {
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

	// Get PathConfig for filtering
	ctx := context.Background()
	pathConfig, err := w.reg.GetEffectiveConfig(ctx, absPath)
	if err != nil {
		// If no config found, use default (skip hidden)
		pathConfig = &registry.PathConfig{SkipHidden: true}
	}
	filter := walker.NewFilter(pathConfig)

	// Add recursive watches
	err = filepath.WalkDir(absPath, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // Skip directories we can't access
		}

		if !d.IsDir() {
			return nil
		}

		// Use PathConfig filter for directory decisions (except root)
		if p != absPath && !filter.ShouldProcessDir(p) {
			return fs.SkipDir
		}

		if err := w.addWatch(p); err != nil {
			// Log but don't fail on watch errors (may hit limits)
			w.logger.Warn("failed to add watch", "path", p, "error", err)
			w.mu.Lock()
			w.stats.Errors++
			w.mu.Unlock()
		}

		return nil
	})

	// Only mark as watched if walk succeeded
	if err != nil {
		return fmt.Errorf("failed to walk directory; %w", err)
	}

	w.mu.Lock()
	w.watchedPaths[absPath] = true
	w.mu.Unlock()

	return nil
}

// addWatch adds a single directory to the fsnotify watcher.
func (w *watcher) addWatch(path string) error {
	if err := w.fsWatcher.Add(path); err != nil {
		// Check for watch limit exhaustion
		if isWatchLimitError(err) {
			w.mu.Lock()
			w.stats.DegradedMode = true
			w.mu.Unlock()
			w.logger.Warn("watch limit reached, entering degraded mode", "path", path)
			return nil
		}
		return err
	}
	return nil
}

// Unwatch stops watching a path.
func (w *watcher) Unwatch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path; %w", err)
	}

	w.mu.Lock()
	delete(w.watchedPaths, absPath)
	w.mu.Unlock()

	// Remove all watches under this path
	for _, watched := range w.fsWatcher.WatchList() {
		if watched == absPath || strings.HasPrefix(watched, absPath+string(filepath.Separator)) {
			_ = w.fsWatcher.Remove(watched)
		}
	}

	return nil
}

// WatchedPaths returns the list of currently watched root paths.
func (w *watcher) WatchedPaths() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	paths := make([]string, 0, len(w.watchedPaths))
	for p := range w.watchedPaths {
		paths = append(paths, p)
	}
	return paths
}

// Start begins processing filesystem events.
func (w *watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}
	w.running = true
	w.stats.IsRunning = true
	w.mu.Unlock()

	go w.processEvents(ctx)
	go w.processCoalescedEvents(ctx)

	return nil
}

// Stop stops the watcher.
func (w *watcher) Stop() error {
	var stopErr error
	w.stopOnce.Do(func() {
		w.mu.Lock()
		if !w.running {
			w.mu.Unlock()
			return
		}
		w.running = false
		w.stats.IsRunning = false
		w.mu.Unlock()

		// Stop coalescer first to unblock processCoalescedEvents
		w.coalescer.Stop()

		// Then signal stop and wait for goroutines
		close(w.stopCh)
		<-w.doneCh

		stopErr = w.fsWatcher.Close()
	})
	return stopErr
}

// Stats returns current watcher statistics.
func (w *watcher) Stats() WatcherStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// Errors returns a channel for fatal watcher errors.
func (w *watcher) Errors() <-chan error {
	return w.errChan
}

// CollectMetrics implements metrics.MetricsProvider.
func (w *watcher) CollectMetrics(ctx context.Context) error {
	stats := w.Stats()
	metrics.WatcherPathsTotal.Set(float64(stats.WatchedPaths))
	return nil
}

// processEvents reads from fsnotify and feeds to coalescer.
func (w *watcher) processEvents(ctx context.Context) {
	defer close(w.doneCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleFsEvent(event)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.mu.Lock()
			w.stats.Errors++
			w.mu.Unlock()
			w.logger.Error("fsnotify error", "error", err)
			select {
			case w.errChan <- err:
			default:
			}
		}
	}
}

// handleFsEvent processes a single fsnotify event.
func (w *watcher) handleFsEvent(event fsnotify.Event) {
	w.mu.Lock()
	w.stats.EventsReceived++
	w.mu.Unlock()

	// Skip transient editor artifacts (vim swap files, etc.)
	if isEditorNoise(event.Name) {
		return
	}

	// Get PathConfig for filtering
	ctx := context.Background()
	pathConfig, err := w.reg.GetEffectiveConfig(ctx, event.Name)
	if err != nil {
		// File not under a remembered path, skip silently
		return
	}
	filter := walker.NewFilter(pathConfig)

	// Handle directory creation (add recursive watch if not filtered)
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			if filter.ShouldProcessDir(event.Name) {
				if err := w.addWatch(event.Name); err != nil {
					w.logger.Warn("failed to add watch for new directory", "path", event.Name, "error", err)
				}
			}
			return // Don't process directory events further
		}
	}

	// For file events, check if file should be processed
	if !filter.ShouldProcessFile(event.Name) {
		return
	}

	// Determine event type
	var eventType CoalescedEventType
	switch {
	case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
		eventType = EventDelete
	case event.Has(fsnotify.Create):
		eventType = EventCreate
	case event.Has(fsnotify.Write):
		eventType = EventModify
	default:
		return // Ignore chmod-only events
	}

	// Feed to coalescer
	w.coalescer.Add(CoalescedEvent{
		Path:      event.Name,
		Type:      eventType,
		Timestamp: time.Now(),
	})
}

// processCoalescedEvents publishes coalesced events to the bus.
func (w *watcher) processCoalescedEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case ce, ok := <-w.coalescer.Events():
			if !ok {
				return
			}
			w.publishEvent(ctx, ce)
		}
	}
}

// publishEvent publishes a coalesced event to the event bus.
func (w *watcher) publishEvent(ctx context.Context, ce CoalescedEvent) {
	// Check if path is under a watched root
	if !w.isUnderWatchedPath(ce.Path) {
		return
	}

	// Get file info for non-delete events
	var size int64
	var modTime time.Time
	var contentHash string

	if ce.Type != EventDelete {
		info, err := os.Stat(ce.Path)
		if err != nil {
			// File may have been deleted before we could process
			if os.IsNotExist(err) {
				ce.Type = EventDelete
			} else {
				w.logger.Warn("failed to stat file", "path", ce.Path, "error", err)
				return
			}
		} else {
			if info.IsDir() {
				return // Skip directories
			}
			size = info.Size()
			modTime = info.ModTime()

			// Compute content hash
			hash, err := computeFileHash(ce.Path)
			if err != nil {
				w.logger.Warn("failed to compute hash", "path", ce.Path, "error", err)
				return
			}
			contentHash = hash
		}
	}

	// Publish appropriate event
	var event events.Event
	switch ce.Type {
	case EventCreate, EventModify:
		event = events.Event{
			Type:      events.FileChanged,
			Timestamp: time.Now(),
			Payload: &events.FileEvent{
				Path:        ce.Path,
				ContentHash: contentHash,
				Size:        size,
				ModTime:     modTime,
				IsNew:       ce.Type == EventCreate,
			},
		}
	case EventDelete:
		event = events.Event{
			Type:      events.PathDeleted,
			Timestamp: time.Now(),
			Payload: &events.FileEvent{
				Path: ce.Path,
			},
		}
	}

	if err := w.bus.Publish(ctx, event); err != nil {
		w.logger.Error("failed to publish event", "path", ce.Path, "error", err)
		return
	}

	w.mu.Lock()
	w.stats.EventsPublished++
	w.mu.Unlock()
}

// isUnderWatchedPath checks if a path is under one of the watched roots.
func (w *watcher) isUnderWatchedPath(path string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for watched := range w.watchedPaths {
		if path == watched || strings.HasPrefix(path, watched+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// isEditorNoise returns true if the file is a transient editor artifact.
// These are files that appear and disappear rapidly during editing
// and should be filtered for performance. Other patterns like hidden files,
// .DS_Store, etc. are handled by PathConfig.
func isEditorNoise(path string) bool {
	name := filepath.Base(path)

	// Vim swap files (created during active editing)
	if strings.HasSuffix(name, ".swp") || strings.HasSuffix(name, ".swo") || strings.HasSuffix(name, ".swn") {
		return true
	}

	// Vim temporary file during save
	if name == "4913" {
		return true
	}

	// Emacs auto-save files
	if strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#") {
		return true
	}

	// Backup files created during save (ending with ~)
	if strings.HasSuffix(name, "~") {
		return true
	}

	return false
}

// shouldIgnoreFile is deprecated, use isEditorNoise for transient artifacts
// and PathConfig filtering for other patterns.
func shouldIgnoreFile(path string) bool {
	return isEditorNoise(path)
}

// isWatchLimitError checks if an error indicates watch limit exhaustion.
func isWatchLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "too many open files") ||
		strings.Contains(errStr, "no space left on device") ||
		strings.Contains(errStr, "user limit on total number of inotify watches")
}

// computeFileHash computes the SHA-256 hash of a file's contents.
func computeFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
