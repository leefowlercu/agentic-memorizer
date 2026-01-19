package events

import "time"

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

// NewRebuildComplete creates a RebuildComplete event.
func NewRebuildComplete(filesQueued, dirsProcessed int, duration time.Duration, full bool) Event {
	return NewEvent(RebuildComplete, &RebuildCompleteEvent{
		FilesQueued:   filesQueued,
		DirsProcessed: dirsProcessed,
		Duration:      duration,
		Full:          full,
	})
}
