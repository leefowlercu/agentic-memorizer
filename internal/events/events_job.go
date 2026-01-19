package events

import "time"

// JobStartedEvent contains data for job start events.
type JobStartedEvent struct {
	Name      string
	StartedAt time.Time
}

// JobCompletedEvent contains data for job completion events.
type JobCompletedEvent struct {
	Name       string
	Status     string
	Error      string
	Counts     map[string]int
	Details    map[string]any
	StartedAt  time.Time
	FinishedAt time.Time
}

// JobFailedEvent contains data for job failure events.
type JobFailedEvent struct {
	Name       string
	Error      string
	Counts     map[string]int
	Details    map[string]any
	StartedAt  time.Time
	FinishedAt time.Time
}

// NewJobStarted creates a JobStarted event.
func NewJobStarted(name string, startedAt time.Time) Event {
	return NewEvent(JobStarted, &JobStartedEvent{
		Name:      name,
		StartedAt: startedAt,
	})
}

// NewJobCompleted creates a JobCompleted event.
func NewJobCompleted(name, status, err string, counts map[string]int, details map[string]any, startedAt, finishedAt time.Time) Event {
	return NewEvent(JobCompleted, &JobCompletedEvent{
		Name:       name,
		Status:     status,
		Error:      err,
		Counts:     counts,
		Details:    details,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	})
}

// NewJobFailed creates a JobFailed event.
func NewJobFailed(name, err string, counts map[string]int, details map[string]any, startedAt, finishedAt time.Time) Event {
	return NewEvent(JobFailed, &JobFailedEvent{
		Name:       name,
		Error:      err,
		Counts:     counts,
		Details:    details,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	})
}
