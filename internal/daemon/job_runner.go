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
		jr.bus.Publish(ctx, events.NewJobStarted(job.Name(), started))
	}

	result := fn(ctx)
	if result.StartedAt.IsZero() {
		result.StartedAt = started
	}
	if result.FinishedAt.IsZero() {
		result.FinishedAt = time.Now()
	}

	if jr.bus != nil {
		jr.bus.Publish(ctx, events.NewJobCompleted(
			job.Name(),
			string(result.Status),
			result.Error,
			result.Counts,
			result.Details,
			result.StartedAt,
			result.FinishedAt,
		))
		if result.Status == RunFailed {
			jr.bus.Publish(ctx, events.NewJobFailed(
				job.Name(),
				result.Error,
				result.Counts,
				result.Details,
				result.StartedAt,
				result.FinishedAt,
			))
		}
	}

	return result
}
