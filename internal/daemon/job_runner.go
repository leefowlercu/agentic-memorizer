package daemon

import (
	"context"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

// JobRunner executes JobComponents and updates health/events.
type JobRunner struct {
	bus *events.EventBus
}

// NewJobRunner creates a new JobRunner.
func NewJobRunner(bus *events.EventBus) *JobRunner {
	return &JobRunner{bus: bus}
}

// Run executes a job and emits start/complete/failed events.
func (jr *JobRunner) Run(ctx context.Context, job JobComponent, fn func(context.Context) RunResult) RunResult {
	started := time.Now()
	if jr.bus != nil {
		jr.bus.Publish(ctx, events.NewEvent(events.JobStarted, map[string]any{
			"name":       job.Name(),
			"started_at": started,
		}))
	}

	result := fn(ctx)
	if result.StartedAt.IsZero() {
		result.StartedAt = started
	}
	if result.FinishedAt.IsZero() {
		result.FinishedAt = time.Now()
	}

	if jr.bus != nil {
		jr.bus.Publish(ctx, events.NewEvent(events.JobCompleted, map[string]any{
			"name":        job.Name(),
			"status":      result.Status,
			"error":       result.Error,
			"counts":      result.Counts,
			"details":     result.Details,
			"started_at":  result.StartedAt,
			"finished_at": result.FinishedAt,
		}))
		if result.Status == RunFailed {
			jr.bus.Publish(ctx, events.NewEvent(events.JobFailed, map[string]any{
				"name":        job.Name(),
				"error":       result.Error,
				"counts":      result.Counts,
				"details":     result.Details,
				"started_at":  result.StartedAt,
				"finished_at": result.FinishedAt,
			}))
		}
	}

	return result
}
