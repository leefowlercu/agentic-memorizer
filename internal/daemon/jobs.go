package daemon

import (
	"context"
	"time"
)

// logicalJob is a placeholder to register job names; orchestration runs jobs directly.
type logicalJob struct {
	name string
	mode string
}

func (j *logicalJob) Name() string           { return j.name }
func (j *logicalJob) Kind() ComponentKind    { return ComponentKindJob }
func (j *logicalJob) Dependencies() []string { return nil }
func (j *logicalJob) Run(ctx context.Context) RunResult {
	// Actual execution is routed through the orchestrator job flow.
	return RunResult{
		Status:     RunFailed,
		StartedAt:  time.Now(),
		FinishedAt: time.Now(),
		Error:      "not implemented",
	}
}
