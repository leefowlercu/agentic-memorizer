package events

import "time"

// FileEvent contains data for file-related events (discovered, changed, deleted).
type FileEvent struct {
	// Path is the absolute path to the file.
	Path string

	// ContentHash is the SHA256 hash of the file content (empty for deleted files).
	ContentHash string

	// Size is the file size in bytes (0 for deleted files).
	Size int64

	// ModTime is the file modification time (zero for deleted files).
	ModTime time.Time

	// IsNew indicates if this is a newly discovered file (for FileDiscovered events).
	IsNew bool
}

// RememberedPathRemovedEvent contains data for remembered path removal events.
type RememberedPathRemovedEvent struct {
	// Path is the remembered path that was removed.
	Path string

	// Reason describes why the path was removed (e.g., "not_found").
	Reason string

	// KeepData indicates whether existing data should be preserved.
	KeepData bool
}

// RememberedPathEvent contains data for remembered path lifecycle events.
type RememberedPathEvent struct {
	// Path is the remembered path.
	Path string
}

// WatcherDegradedEvent contains data for watcher degradation events.
type WatcherDegradedEvent struct {
	// Reason describes why the watcher entered degraded mode.
	Reason string

	// WatchCount is the number of active watches at the time of degradation.
	WatchCount int

	// AffectedPath is the path that triggered the degradation (if applicable).
	AffectedPath string
}

// NewFileDiscovered creates a FileDiscovered event.
func NewFileDiscovered(path, contentHash string, size int64, modTime time.Time, isNew bool) Event {
	return NewEvent(FileDiscovered, &FileEvent{
		Path:        path,
		ContentHash: contentHash,
		Size:        size,
		ModTime:     modTime,
		IsNew:       isNew,
	})
}

// NewFileChanged creates a FileChanged event.
func NewFileChanged(path, contentHash string, size int64, modTime time.Time, isNew bool) Event {
	return NewEvent(FileChanged, &FileEvent{
		Path:        path,
		ContentHash: contentHash,
		Size:        size,
		ModTime:     modTime,
		IsNew:       isNew,
	})
}

// NewPathDeleted creates a PathDeleted event.
func NewPathDeleted(path string) Event {
	return NewEvent(PathDeleted, &FileEvent{
		Path: path,
	})
}

// NewRememberedPathRemoved creates a RememberedPathRemoved event.
func NewRememberedPathRemoved(path, reason string, keepData bool) Event {
	return NewEvent(RememberedPathRemoved, &RememberedPathRemovedEvent{
		Path:     path,
		Reason:   reason,
		KeepData: keepData,
	})
}

// NewRememberedPathAdded creates a RememberedPathAdded event.
func NewRememberedPathAdded(path string) Event {
	return NewEvent(RememberedPathAdded, &RememberedPathEvent{
		Path: path,
	})
}

// NewRememberedPathUpdated creates a RememberedPathUpdated event.
func NewRememberedPathUpdated(path string) Event {
	return NewEvent(RememberedPathUpdated, &RememberedPathEvent{
		Path: path,
	})
}

// NewWatcherDegraded creates a WatcherDegraded event.
func NewWatcherDegraded(reason string, watchCount int, affectedPath string) Event {
	return NewEvent(WatcherDegraded, &WatcherDegradedEvent{
		Reason:       reason,
		WatchCount:   watchCount,
		AffectedPath: affectedPath,
	})
}

// NewWatcherRecovered creates a WatcherRecovered event.
func NewWatcherRecovered(watchCount int) Event {
	return NewEvent(WatcherRecovered, &WatcherDegradedEvent{
		Reason:     "recovered",
		WatchCount: watchCount,
	})
}
