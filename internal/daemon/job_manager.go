package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cleaner"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
)

// JobManager orchestrates rebuild and walk operations.
type JobManager struct {
	bus             *events.EventBus
	walker          walker.Walker
	cleaner         *cleaner.Cleaner
	registry        registry.Registry
	healthCollector *ComponentHealthCollector
	jobRunner       *JobRunner

	rebuildMu       sync.Mutex
	rebuildStopChan chan struct{}

	logger *slog.Logger
}

// JobManagerOption configures JobManager.
type JobManagerOption func(*JobManager)

// WithJobManagerLogger sets the logger.
func WithJobManagerLogger(l *slog.Logger) JobManagerOption {
	return func(m *JobManager) {
		m.logger = l
	}
}

// NewJobManager creates a new job manager.
func NewJobManager(
	bus *events.EventBus,
	w walker.Walker,
	c *cleaner.Cleaner,
	reg registry.Registry,
	hc *ComponentHealthCollector,
	opts ...JobManagerOption,
) *JobManager {
	m := &JobManager{
		bus:             bus,
		walker:          w,
		cleaner:         c,
		registry:        reg,
		healthCollector: hc,
		logger:          slog.Default(),
	}

	for _, opt := range opts {
		opt(m)
	}

	m.jobRunner = NewJobRunner(bus)

	return m
}

// Rebuild performs a rebuild operation (full or incremental).
func (m *JobManager) Rebuild(ctx context.Context, full bool) (*RebuildResult, error) {
	m.rebuildMu.Lock()
	defer m.rebuildMu.Unlock()

	start := time.Now()

	// Publish rebuild started event
	pathCount := 0
	if m.registry != nil {
		if paths, err := m.registry.ListPaths(ctx); err == nil {
			pathCount = len(paths)
		}
	}
	trigger := "manual"
	if m.bus != nil {
		_ = m.bus.Publish(ctx, events.NewRebuildStarted(full, pathCount, trigger))
	}

	// Validate and clean missing remembered paths before rebuild
	removedPaths := m.ValidateAndCleanPaths(ctx)

	if m.walker == nil {
		return nil, fmt.Errorf("walker not initialized")
	}

	var err error
	if full {
		err = m.walker.WalkAll(ctx)
	} else {
		err = m.walker.WalkAllIncremental(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("rebuild walk failed; %w", err)
	}

	// Run reconciliation after walk completes to clean up stale entries
	discoveredPaths := m.walker.DrainDiscoveredPaths()
	if discoveredPaths != nil && m.cleaner != nil {
		// Get remembered paths to reconcile against
		rememberedPaths, listErr := m.registry.ListPaths(ctx)
		if listErr != nil {
			m.logger.Warn("failed to list paths for reconciliation", "error", listErr)
		} else {
			for _, rp := range rememberedPaths {
				result, reconcileErr := m.cleaner.Reconcile(ctx, rp.Path, discoveredPaths)
				if reconcileErr != nil {
					m.logger.Warn("reconciliation failed", "path", rp.Path, "error", reconcileErr)
				} else if result.StaleRemoved > 0 {
					m.logger.Info("reconciliation complete",
						"path", rp.Path,
						"stale_removed", result.StaleRemoved,
						"duration", result.Duration)
				}
			}
		}
	}

	stats := m.walker.Stats()
	duration := time.Since(start)

	// Publish rebuild complete event for MCP notifications
	if m.bus != nil {
		_ = m.bus.Publish(ctx, events.NewRebuildComplete(
			int(stats.FilesDiscovered),
			int(stats.DirsTraversed),
			duration,
			full,
		))
	}

	return &RebuildResult{
		Status:        "completed",
		FilesQueued:   int(stats.FilesDiscovered),
		DirsProcessed: int(stats.DirsTraversed),
		Duration:      duration.Round(time.Millisecond).String(),
		RemovedPaths:  removedPaths,
	}, nil
}

// RebuildWithRecord wraps Rebuild to record job results for health/status.
func (m *JobManager) RebuildWithRecord(ctx context.Context, full bool, jobName string) (*RebuildResult, error) {
	mode := "incremental"
	if full {
		mode = "full"
	}

	var rebuildResult *RebuildResult
	var runErr error

	runResult := m.jobRunner.Run(ctx, jobName, func(runCtx context.Context) RunResult {
		result := RunResult{
			Status:     RunFailed,
			StartedAt:  time.Now(),
			Counts:     map[string]int{"files_queued": 0, "dirs_processed": 0},
			Details:    map[string]any{"full": full, "walk_mode": mode, "duration": "", "removed": 0},
			FinishedAt: time.Now(),
		}

		var err error
		rebuildResult, err = m.Rebuild(runCtx, full)
		runErr = err
		result.FinishedAt = time.Now()

		if rebuildResult != nil {
			result.Counts["files_queued"] = rebuildResult.FilesQueued
			result.Counts["dirs_processed"] = rebuildResult.DirsProcessed
			result.Details["duration"] = rebuildResult.Duration
			result.Details["removed"] = len(rebuildResult.RemovedPaths)
		}

		if err != nil {
			result.Error = err.Error()
			result.Status = RunFailed
			return result
		}

		result.Status = RunSuccess
		return result
	})

	if m.healthCollector != nil {
		m.healthCollector.RecordJobResult(jobName, runResult)
	}

	return rebuildResult, runErr
}

// StartPeriodicRebuild starts a goroutine that triggers incremental rebuilds at the configured interval.
func (m *JobManager) StartPeriodicRebuild(ctx context.Context, interval time.Duration) {
	m.rebuildStopChan = make(chan struct{})

	m.logger.Info("periodic rebuild enabled", "interval", interval, "mode", "incremental")

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.rebuildStopChan:
				return
			case <-ticker.C:
				m.logger.Info("starting periodic rebuild", "mode", "incremental")
				result, err := m.RebuildWithRecord(ctx, false, "job.rebuild_incremental") // Always incremental
				if err != nil {
					m.logger.Error("periodic rebuild failed", "error", err)
					continue
				}
				m.logger.Info("periodic rebuild complete",
					"files_queued", result.FilesQueued,
					"dirs_processed", result.DirsProcessed,
					"duration", result.Duration)
			}
		}
	}()
}

// StopPeriodicRebuild stops the periodic rebuild goroutine.
func (m *JobManager) StopPeriodicRebuild() {
	if m.rebuildStopChan != nil {
		close(m.rebuildStopChan)
		m.rebuildStopChan = nil
		m.logger.Debug("periodic rebuild stopped")
	}
}

// InitialWalk performs a full walk of all remembered paths at daemon start.
func (m *JobManager) InitialWalk(ctx context.Context) (*RebuildResult, error) {
	m.logger.Info("starting initial walk of remembered paths", "mode", "full")

	result, err := m.RebuildWithRecord(ctx, true, "job.initial_walk") // Full rebuild on daemon start
	if err != nil {
		if ctx.Err() != nil {
			m.logger.Debug("initial walk canceled")
			return nil, ctx.Err()
		}
		m.logger.Warn("initial walk failed", "error", err)
		return nil, err
	}

	m.logger.Info("initial walk complete",
		"files_queued", result.FilesQueued,
		"dirs_processed", result.DirsProcessed,
		"duration", result.Duration,
	)

	return result, nil
}

// ValidateAndCleanPaths checks all remembered paths and removes those that
// no longer exist. Returns the list of removed paths.
func (m *JobManager) ValidateAndCleanPaths(ctx context.Context) []string {
	if m.registry == nil {
		return nil
	}

	removed, err := m.registry.ValidateAndCleanPaths(ctx)
	if err != nil {
		m.logger.Warn("failed to validate remembered paths", "error", err)
		return nil
	}

	// For each removed path, clean up graph nodes and emit events
	for _, path := range removed {
		m.logger.Warn("removed missing remembered path", "path", path)

		// Emit event for observability
		if m.bus != nil {
			_ = m.bus.Publish(ctx, events.NewRememberedPathRemoved(path, "not_found", false))
		}
	}

	return removed
}

// WalkPath performs a walk of a single path (used for remembered path events).
func (m *JobManager) WalkPath(ctx context.Context, path string) error {
	m.rebuildMu.Lock()
	defer m.rebuildMu.Unlock()

	if m.walker == nil {
		return nil
	}

	return m.walker.Walk(ctx, path)
}
