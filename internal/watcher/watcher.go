package watcher

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventType represents the type of file system event
type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
)

// Event represents a file system event
type Event struct {
	Type EventType
	Path string
}

// Watcher watches a directory for file changes
type Watcher struct {
	rootPath           string
	skipDirs           []string
	skipFiles          []string
	debounceMs         int
	fsWatcher          *fsnotify.Watcher
	logger             *slog.Logger
	eventChan          chan Event
	batchedEvents      map[string]Event
	eventMu            sync.Mutex
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	debounceIntervalCh chan time.Duration // For updating debounce interval
}

// New creates a new file system watcher
func New(rootPath string, skipDirs []string, skipFiles []string, debounceMs int, logger *slog.Logger) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	w := &Watcher{
		rootPath:           rootPath,
		skipDirs:           skipDirs,
		skipFiles:          skipFiles,
		debounceMs:         debounceMs,
		fsWatcher:          fsWatcher,
		logger:             logger,
		eventChan:          make(chan Event, 100),
		batchedEvents:      make(map[string]Event),
		stopChan:           make(chan struct{}),
		debounceIntervalCh: make(chan time.Duration, 1),
	}

	return w, nil
}

// Start starts watching the directory
func (w *Watcher) Start() error {
	// Add all directories recursively
	if err := w.addRecursive(w.rootPath); err != nil {
		return fmt.Errorf("failed to add directories: %w", err)
	}

	// Start event processing goroutine
	w.wg.Add(2)
	go w.processEvents()
	go w.debounceBatch()

	w.logger.Info("watcher started", "root", w.rootPath)
	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	close(w.stopChan)
	w.wg.Wait()
	w.fsWatcher.Close()
	close(w.eventChan)
	w.logger.Info("watcher stopped")
}

// Events returns the channel for receiving batched events
func (w *Watcher) Events() <-chan Event {
	return w.eventChan
}

// addRecursive adds all directories recursively to the watcher
func (w *Watcher) addRecursive(path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			w.logger.Warn("error walking path", "path", p, "error", err)
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		// Skip excluded directories
		if w.shouldSkipDir(p) {
			w.logger.Debug("skipping directory", "path", p)
			return filepath.SkipDir
		}

		// Add directory to watcher
		if err := w.fsWatcher.Add(p); err != nil {
			w.logger.Warn("failed to watch directory", "path", p, "error", err)
			return nil
		}

		w.logger.Debug("watching directory", "path", p)
		return nil
	})
}

// processEvents processes events from fsnotify
func (w *Watcher) processEvents() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("watcher error", "error", err)

		case <-w.stopChan:
			return
		}
	}
}

// handleEvent handles a single fsnotify event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Skip if it's a file/dir we should ignore
	if w.shouldSkip(event.Name) {
		return
	}

	w.eventMu.Lock()
	defer w.eventMu.Unlock()

	// Determine event type
	var eventType EventType
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		eventType = EventCreate
		w.logger.Debug("create event", "path", event.Name)

		// If it's a new directory, add it to the watcher
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if !w.shouldSkipDir(event.Name) {
				go func() {
					if err := w.addRecursive(event.Name); err != nil {
						w.logger.Error("failed to watch new directory", "path", event.Name, "error", err)
					}
				}()
			}
		}

	case event.Op&fsnotify.Write == fsnotify.Write:
		eventType = EventModify
		w.logger.Debug("modify event", "path", event.Name)

	case event.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = EventDelete
		w.logger.Debug("delete event", "path", event.Name)

	case event.Op&fsnotify.Rename == fsnotify.Rename:
		// Treat rename source as DELETE - fsnotify fires separate events:
		// Source path gets RENAME (handled here as DELETE)
		// Destination path gets CREATE (handled by CREATE case)
		// This ensures both old and new paths are processed correctly.
		eventType = EventDelete
		w.logger.Debug("rename event", "path", event.Name)

	default:
		return
	}

	// Batch the event (last write wins for the same path)
	w.batchedEvents[event.Name] = Event{
		Type: eventType,
		Path: event.Name,
	}
}

// debounceBatch sends batched events after debounce period
func (w *Watcher) debounceBatch() {
	defer w.wg.Done()

	ticker := time.NewTicker(time.Duration(w.debounceMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.sendBatchedEvents()

		case newInterval := <-w.debounceIntervalCh:
			ticker.Stop()
			ticker = time.NewTicker(newInterval)
			w.debounceMs = int(newInterval.Milliseconds())
			w.logger.Info("debounce interval changed", "new_interval_ms", w.debounceMs)

		case <-w.stopChan:
			return
		}
	}
}

// sendBatchedEvents sends accumulated events to the event channel
func (w *Watcher) sendBatchedEvents() {
	w.eventMu.Lock()
	events := w.batchedEvents
	w.batchedEvents = make(map[string]Event)
	w.eventMu.Unlock()

	if len(events) == 0 {
		return
	}

	w.logger.Debug("sending batched events", "count", len(events))

	for _, event := range events {
		select {
		case w.eventChan <- event:
		case <-w.stopChan:
			return
		}
	}
}

// shouldSkip checks if a path should be skipped
func (w *Watcher) shouldSkip(path string) bool {
	// Skip hidden files and directories
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return true
	}

	// Check if it's in skip files list
	for _, skipFile := range w.skipFiles {
		if base == skipFile {
			return true
		}
	}

	return false
}

// shouldSkipDir checks if a directory should be skipped
func (w *Watcher) shouldSkipDir(path string) bool {
	base := filepath.Base(path)

	// Skip hidden directories
	if strings.HasPrefix(base, ".") {
		return true
	}

	// Check skip directories list
	for _, skipDir := range w.skipDirs {
		if base == skipDir {
			return true
		}
	}

	return false
}

// UpdateDebounceInterval signals the watcher to update its debounce interval
func (w *Watcher) UpdateDebounceInterval(intervalMs int) {
	select {
	case w.debounceIntervalCh <- time.Duration(intervalMs) * time.Millisecond:
		// Signal sent successfully
	default:
		// Channel full, interval will be updated on next signal
		w.logger.Warn("debounce interval update channel full, skipping")
	}
}
