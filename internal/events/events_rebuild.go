package events

import "time"

// RebuildStartedEvent contains data for rebuild start events.
type RebuildStartedEvent struct {
	// Full indicates if this is a full rebuild (true) or incremental (false).
	Full bool

	// PathCount is the number of remembered paths to be walked.
	PathCount int

	// Trigger describes what initiated the rebuild ("startup", "periodic", "manual").
	Trigger string
}

// RebuildCompleteEvent contains data for rebuild completion events.
type RebuildCompleteEvent struct {
	// FilesQueued is the number of files queued for analysis.
	FilesQueued int

	// DirsProcessed is the number of directories traversed.
	DirsProcessed int

	// Duration is how long the rebuild took.
	Duration time.Duration

	// Full indicates if this was a full rebuild (true) or incremental (false).
	Full bool
}

// NewRebuildStarted creates a RebuildStarted event.
func NewRebuildStarted(full bool, pathCount int, trigger string) Event {
	return NewEvent(RebuildStarted, &RebuildStartedEvent{
		Full:      full,
		PathCount: pathCount,
		Trigger:   trigger,
	})
}

// NewRebuildComplete creates a RebuildComplete event.
func NewRebuildComplete(filesQueued, dirsProcessed int, duration time.Duration, full bool) Event {
	return NewEvent(RebuildComplete, &RebuildCompleteEvent{
		FilesQueued:   filesQueued,
		DirsProcessed: dirsProcessed,
		Duration:      duration,
		Full:          full,
	})
}
