package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/analysis"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

// ComponentHealthCollector gathers health status from daemon components.
type ComponentHealthCollector struct {
	bag         *ComponentBag
	busDegraded bool
	jobResults  map[string]RunResult
	jobRunning  map[string]time.Time
	jobMu       sync.Mutex
	logger      *slog.Logger
}

// HealthCollectorOption configures ComponentHealthCollector.
type HealthCollectorOption func(*ComponentHealthCollector)

// WithHealthCollectorLogger sets the logger.
func WithHealthCollectorLogger(l *slog.Logger) HealthCollectorOption {
	return func(c *ComponentHealthCollector) {
		c.logger = l
	}
}

// NewComponentHealthCollector creates a health collector for the given component bag.
func NewComponentHealthCollector(bag *ComponentBag, opts ...HealthCollectorOption) *ComponentHealthCollector {
	c := &ComponentHealthCollector{
		bag:        bag,
		jobResults: make(map[string]RunResult),
		jobRunning: make(map[string]time.Time),
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// CollectComponents gathers health status from all components and returns a status map.
func (c *ComponentHealthCollector) CollectComponents() map[string]ComponentHealth {
	statuses := make(map[string]ComponentHealth)

	// Event bus status
	if c.bag.Bus != nil {
		busStats := c.bag.Bus.Stats()
		c.updateBusDegradation(busStats)

		status := ComponentStatusRunning
		var errMsg string
		if c.busDegraded {
			status = ComponentStatusDegraded
			errMsg = "bus degraded (drops/backlog)"
		}
		statuses["bus"] = ComponentHealth{
			Status:      status,
			Error:       errMsg,
			LastChecked: time.Now(),
			Details: map[string]any{
				"subscriber_count":    busStats.SubscriberCount,
				"dropped_events":      busStats.Dropped,
				"drop_rate_per_sec":   busStats.DropRatePerSec,
				"is_closed":           busStats.IsClosed,
				"critical_len":        busStats.CriticalLen,
				"critical_cap":        busStats.CriticalCap,
				"rebuild_recommended": c.busDegraded,
			},
		}
	}

	// Registry status - always ok if we got here
	if c.bag.Registry != nil {
		statuses["registry"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
		}
	}

	// Graph status
	if c.bag.Graph != nil {
		if c.bag.GraphDegraded {
			statuses["graph"] = ComponentHealth{
				Status:      ComponentStatusDegraded,
				Error:       "connection failed",
				LastChecked: time.Now(),
			}
		} else {
			statuses["graph"] = ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
			}
		}
	}

	// Walker status
	if c.bag.Walker != nil {
		stats := c.bag.Walker.Stats()
		status := ComponentStatusRunning
		if stats.IsWalking {
			status = ComponentStatusRunning
		}
		statuses["walker"] = ComponentHealth{
			Status:      status,
			LastChecked: time.Now(),
		}
	}

	// Watcher status
	if c.bag.Watcher != nil {
		stats := c.bag.Watcher.Stats()
		status := ComponentStatusStopped
		if stats.IsRunning {
			status = ComponentStatusRunning
		}
		if stats.DegradedMode {
			statuses["watcher"] = ComponentHealth{
				Status:      ComponentStatusDegraded,
				Error:       "degraded mode (watch limit reached)",
				LastChecked: time.Now(),
				Details: map[string]any{
					"watched_paths":    stats.WatchedPaths,
					"events_received":  stats.EventsReceived,
					"events_published": stats.EventsPublished,
					"events_coalesced": stats.EventsCoalesced,
					"errors":           stats.Errors,
					"degraded_mode":    stats.DegradedMode,
				},
			}
		} else {
			statuses["watcher"] = ComponentHealth{
				Status:      status,
				LastChecked: time.Now(),
				Details: map[string]any{
					"watched_paths":    stats.WatchedPaths,
					"events_received":  stats.EventsReceived,
					"events_published": stats.EventsPublished,
					"events_coalesced": stats.EventsCoalesced,
					"errors":           stats.Errors,
					"degraded_mode":    stats.DegradedMode,
				},
			}
		}
	}

	// Queue status
	if c.bag.Queue != nil {
		stats := c.bag.Queue.Stats()
		var status ComponentStatus
		switch stats.State {
		case analysis.QueueStateRunning:
			status = ComponentStatusRunning
		case analysis.QueueStateStopped:
			status = ComponentStatusStopped
		default:
			status = ComponentStatusStopped
		}
		if stats.DegradationMode != analysis.DegradationFull && status == ComponentStatusRunning {
			status = ComponentStatusDegraded
		}
		statuses["queue"] = ComponentHealth{
			Status:      status,
			LastChecked: time.Now(),
			Details: map[string]any{
				"pending":              stats.PendingItems,
				"in_progress":          stats.ActiveWorkers,
				"processed":            stats.ProcessedItems,
				"analysis_failures":    stats.AnalysisFailures,
				"persistence_failures": stats.PersistenceFailures,
				"degradation_mode":     stats.DegradationMode,
			},
		}
	}

	// Persistence queue status
	if c.bag.PersistenceQueue != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		queueStats, err := c.bag.PersistenceQueue.Stats(ctx)
		cancel()

		if err != nil {
			statuses["persistence_queue"] = ComponentHealth{
				Status:      ComponentStatusDegraded,
				Error:       err.Error(),
				LastChecked: time.Now(),
			}
		} else {
			status := ComponentStatusRunning
			var errMsg string

			// Warn if items are pending (graph was/is unavailable)
			if queueStats.Pending > 0 || queueStats.Inflight > 0 {
				status = ComponentStatusDegraded
				errMsg = "items pending graph persistence"
			}

			// Failed items indicate a more serious issue
			if queueStats.Failed > 0 {
				status = ComponentStatusDegraded
				errMsg = "items failed after max retries"
			}

			statuses["persistence_queue"] = ComponentHealth{
				Status:      status,
				Error:       errMsg,
				LastChecked: time.Now(),
				Details: map[string]any{
					"pending":   queueStats.Pending,
					"inflight":  queueStats.Inflight,
					"completed": queueStats.Completed,
					"failed":    queueStats.Failed,
				},
			}
		}
	}

	// MCP server status
	if c.bag.MCPServer != nil {
		if c.bag.MCPDegraded {
			statuses["mcp"] = ComponentHealth{
				Status:      ComponentStatusDegraded,
				Error:       "startup failed",
				LastChecked: time.Now(),
			}
		} else {
			statuses["mcp"] = ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
			}
		}
	}

	// Semantic provider status
	if c.bag.SemanticProvider != nil {
		if c.bag.SemanticProvider.Available() {
			statuses["semantic_provider"] = ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
				Details: map[string]any{
					"provider": c.bag.SemanticProvider.Name(),
				},
			}
		} else {
			statuses["semantic_provider"] = ComponentHealth{
				Status:      ComponentStatusFailed,
				Error:       "not available (missing API key)",
				LastChecked: time.Now(),
				Details: map[string]any{
					"provider": c.bag.SemanticProvider.Name(),
				},
			}
		}
	}

	// Embeddings provider status
	if c.bag.EmbedProvider != nil {
		if c.bag.EmbedProvider.Available() {
			statuses["embeddings_provider"] = ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
				Details: map[string]any{
					"provider":   c.bag.EmbedProvider.Name(),
					"model":      c.bag.EmbedProvider.ModelName(),
					"dimensions": c.bag.EmbedProvider.Dimensions(),
				},
			}
		} else {
			statuses["embeddings_provider"] = ComponentHealth{
				Status:      ComponentStatusFailed,
				Error:       "not available (missing API key)",
				LastChecked: time.Now(),
				Details: map[string]any{
					"provider":   c.bag.EmbedProvider.Name(),
					"model":      c.bag.EmbedProvider.ModelName(),
					"dimensions": c.bag.EmbedProvider.Dimensions(),
				},
			}
		}
	}

	// Caches status
	if c.bag.SemanticCache != nil {
		statuses["semantic_cache"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
			Details: map[string]any{
				"enabled": true,
			},
		}
	}
	if c.bag.EmbeddingsCache != nil {
		statuses["embeddings_cache"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
			Details: map[string]any{
				"enabled": true,
			},
		}
	}

	// Metrics collector
	if c.bag.MetricsCollector != nil {
		statuses["metrics_collector"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
		}
	}

	return statuses
}

// CollectJobs gathers job status for reporting.
func (c *ComponentHealthCollector) CollectJobs() map[string]JobHealth {
	jobs := make(map[string]JobHealth)

	c.jobMu.Lock()
	defer c.jobMu.Unlock()

	for name, startedAt := range c.jobRunning {
		jobs[name] = JobHealth{
			Status:    JobStatusRunning,
			StartedAt: startedAt,
		}
	}

	for name, jr := range c.jobResults {
		if _, running := c.jobRunning[name]; running {
			continue
		}

		status := JobStatusSuccess
		switch jr.Status {
		case RunFailed:
			status = JobStatusFailed
		case RunPartial:
			status = JobStatusPartial
		}

		jobs[name] = JobHealth{
			Status:     status,
			Error:      jr.Error,
			StartedAt:  jr.StartedAt,
			FinishedAt: jr.FinishedAt,
			Counts:     jr.Counts,
			Details:    jr.Details,
		}
	}

	return jobs
}

// RecordJobResult stores the latest RunResult for a job.
func (c *ComponentHealthCollector) RecordJobResult(name string, result RunResult) {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	c.jobResults[name] = result
	delete(c.jobRunning, name)
}

// RecordJobStart records that a job has started running.
func (c *ComponentHealthCollector) RecordJobStart(name string, startedAt time.Time) {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	c.jobRunning[name] = startedAt
}

// GetJobResult retrieves the last result for a job (for testing/inspection).
func (c *ComponentHealthCollector) GetJobResult(name string) (RunResult, bool) {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	result, ok := c.jobResults[name]
	return result, ok
}

// SetBusDegraded updates the bus degradation flag.
func (c *ComponentHealthCollector) SetBusDegraded(degraded bool) {
	c.busDegraded = degraded
}

// IsBusDegraded returns the current bus degradation state.
func (c *ComponentHealthCollector) IsBusDegraded() bool {
	return c.busDegraded
}

// updateBusDegradation checks bus stats and updates degradation state.
func (c *ComponentHealthCollector) updateBusDegradation(busStats events.BusStats) {
	if c.busDegraded {
		if busStats.DropRatePerSec < busDropRecoverThreshold && !busBacklogHigh(busStats) {
			c.busDegraded = false
		}
	} else if busStats.DropRatePerSec > busDropDegradeThreshold || busBacklogHigh(busStats) {
		c.busDegraded = true
	}
}
