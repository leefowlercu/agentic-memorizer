package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/analysis"
	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/cleaner"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp"
	"github.com/leefowlercu/agentic-memorizer/internal/metrics"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
	"github.com/leefowlercu/agentic-memorizer/internal/watcher"
)

const (
	busDropDegradeThreshold = 1.0 // drops/sec to mark degraded
	busDropRecoverThreshold = 0.2 // drops/sec to clear degraded
	busBacklogDegradePct    = 0.9
)

// Orchestrator manages the initialization and wiring of all daemon components.
type Orchestrator struct {
	daemon           *Daemon
	components       *ComponentRegistry
	jobRunner        *JobRunner
	bus              *events.EventBus
	registry         registry.Registry
	graph            graph.Graph
	semanticProvider providers.SemanticProvider
	embedProvider    providers.EmbeddingsProvider
	queue            *analysis.Queue
	walker           walker.Walker
	watcher          watcher.Watcher
	cleaner          *cleaner.Cleaner
	mcpServer        *mcp.Server
	metricsCollector *metrics.Collector

	// Caches for avoiding redundant API calls
	semanticCache   *cache.SemanticCache
	embeddingsCache *cache.EmbeddingsCache

	// graphDegraded tracks if graph connection failed during startup
	graphDegraded bool

	// mcpDegraded tracks if MCP server failed during startup
	mcpDegraded bool

	// rebuildStopChan signals the periodic rebuild goroutine to stop
	rebuildStopChan chan struct{}

	// healthStopChan stops the health updater goroutine
	healthStopChan chan struct{}

	// rebuildMu serializes rebuild operations to prevent concurrent map corruption
	rebuildMu sync.Mutex

	// jobResults tracks last run results for jobs (initial_walk, rebuilds)
	jobResults map[string]RunResult
	jobMu      sync.Mutex

	componentCancels map[string]context.CancelFunc
	busDegraded      bool

	jobComponents map[string]JobComponent
}

// NewOrchestrator creates a new orchestrator for the daemon.
func NewOrchestrator(d *Daemon) *Orchestrator {
	return &Orchestrator{
		daemon:           d,
		jobResults:       make(map[string]RunResult),
		components:       NewComponentRegistry(),
		componentCancels: make(map[string]context.CancelFunc),
		jobComponents:    make(map[string]JobComponent),
	}
}

// Initialize sets up all components in the correct order.
// Startup sequence: Registry -> Graph -> Cache -> Providers -> Walker -> Watcher -> Queue -> MCP
func (o *Orchestrator) Initialize(ctx context.Context) error {
	cfg := config.Get()

	o.registerComponentDefinitions(cfg)

	if err := o.buildComponents(ctx, cfg); err != nil {
		return err
	}

	if o.queue != nil && o.registry != nil {
		o.queue.SetRegistry(o.registry)
	}

	if o.metricsCollector != nil {
		o.daemon.server.SetMetricsHandler(metrics.Handler())
	}

	// Set up job runner
	o.jobRunner = NewJobRunner(o.bus)

	// Wire MCP handler to HTTP server
	if o.mcpServer != nil {
		o.daemon.server.SetMCPHandler(o.mcpServer.Handler())
	}

	// Set rebuild function on daemon server
	o.daemon.server.SetRebuildFunc(func(ctx context.Context, full bool) (*RebuildResult, error) {
		jobName := "job.rebuild_incremental"
		if full {
			jobName = "job.rebuild_full"
		}
		return o.handleRebuildWithRecord(ctx, full, jobName)
	})

	return nil
}

// Start starts all orchestrated components.
func (o *Orchestrator) Start(ctx context.Context) error {
	// Registry is already initialized and doesn't need a Start call

	// Validate and clean missing remembered paths before starting components
	removedPaths := o.validateRememberedPaths(ctx)
	if len(removedPaths) > 0 {
		// Print summary to stdout for visibility
		fmt.Printf("Removed %d missing remembered paths:\n", len(removedPaths))
		for _, p := range removedPaths {
			fmt.Printf("  %s (not found)\n", p)
		}
	}

	order, err := o.components.TopologicalOrder()
	if err != nil {
		return fmt.Errorf("failed to order components; %w", err)
	}

	for _, name := range order {
		def := o.components.defs[name]
		if def.Kind != ComponentKindPersistent {
			continue
		}

		switch name {
		case "graph":
			if o.graph != nil {
				o.runSupervisor(ctx, name, def, func(c context.Context) error {
					if err := o.graph.Start(c); err != nil {
						o.graphDegraded = true
						return err
					}
					o.graphDegraded = false
					slog.Info("graph client connected")
					return nil
				}, fatalChan(def, o.graph))
			}
		case "queue":
			if o.queue != nil {
				o.runSupervisor(ctx, name, def, func(c context.Context) error {
					if err := o.queue.Start(c); err != nil {
						return err
					}
					o.queue.SetProviders(o.semanticProvider, o.embedProvider)
					o.queue.SetCaches(o.semanticCache, o.embeddingsCache)
					if !o.graphDegraded && o.graph != nil {
						o.queue.SetGraph(o.graph)
					}
					return nil
				}, fatalChan(def, o.queue))
			}
		case "watcher":
			if o.watcher != nil {
				o.runSupervisor(ctx, name, def, func(c context.Context) error {
					if err := o.watcher.Start(c); err != nil {
						return err
					}
					o.watchRememberedPaths(c)
					return nil
				}, o.watcher.Errors())
			}
		case "cleaner":
			if o.cleaner != nil {
				o.runSupervisor(ctx, name, def, func(c context.Context) error {
					return o.cleaner.Start(c)
				}, nil)
			}
		case "mcp":
			if o.mcpServer != nil {
				o.runSupervisor(ctx, name, def, func(c context.Context) error {
					if err := o.mcpServer.Start(c); err != nil {
						o.mcpDegraded = true
						return err
					}
					o.mcpDegraded = false
					slog.Info("MCP server started")
					return nil
				}, fatalChan(def, o.mcpServer))
			}
		case "metrics_collector":
			if o.metricsCollector != nil {
				o.runSupervisor(ctx, name, def, func(c context.Context) error {
					return o.metricsCollector.Start(c)
				}, fatalChan(def, o.metricsCollector))
			}
		}
	}

	// Trigger initial walk (async)
	if o.walker != nil {
		go o.walkRememberedPaths(ctx)
	}

	// Start health updater
	o.startHealthUpdater(ctx, 10*time.Second)

	// Start periodic rebuild (if configured)
	cfg := config.Get()
	if cfg.Daemon.RebuildInterval > 0 {
		o.startPeriodicRebuild(ctx, time.Duration(cfg.Daemon.RebuildInterval)*time.Second)
	}

	return nil
}

// Stop stops all orchestrated components in reverse order.
func (o *Orchestrator) Stop(ctx context.Context) error {
	slog.Info("stopping orchestrated components")

	// Cancel any component-level contexts
	for name, cancel := range o.componentCancels {
		slog.Debug("canceling component", "component", name)
		cancel()
	}
	o.componentCancels = make(map[string]context.CancelFunc)

	// Stop health updater
	if o.healthStopChan != nil {
		close(o.healthStopChan)
		o.healthStopChan = nil
	}

	// Stop periodic rebuild
	if o.rebuildStopChan != nil {
		close(o.rebuildStopChan)
		o.rebuildStopChan = nil
		slog.Debug("periodic rebuild stopped")
	}

	// Stop components in reverse topological order
	order, err := o.components.TopologicalOrder()
	if err != nil {
		slog.Warn("failed to derive stop order", "error", err)
	}

	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		def := o.components.defs[name]
		if def.Kind != ComponentKindPersistent {
			continue
		}

		switch name {
		case "metrics_collector":
			if o.metricsCollector != nil {
				o.metricsCollector.Stop(ctx)
				slog.Debug("metrics collector stopped")
			}
		case "mcp":
			if o.mcpServer != nil {
				o.mcpServer.Stop(ctx)
				slog.Debug("MCP server stopped")
			}
		case "watcher":
			if o.watcher != nil {
				if err := o.watcher.Stop(); err != nil {
					slog.Warn("watcher stop error", "error", err)
				} else {
					slog.Debug("watcher stopped")
				}
			}
		case "cleaner":
			if o.cleaner != nil {
				if err := o.cleaner.Stop(); err != nil {
					slog.Warn("cleaner stop error", "error", err)
				} else {
					slog.Debug("cleaner stopped")
				}
			}
		case "queue":
			if o.queue != nil {
				if err := o.queue.Stop(ctx); err != nil {
					slog.Warn("analysis queue stop error", "error", err)
				} else {
					slog.Debug("analysis queue stopped")
				}
			}
		case "graph":
			if o.graph != nil {
				if err := o.graph.Stop(ctx); err != nil {
					slog.Warn("graph shutdown error", "error", err)
				} else {
					slog.Debug("graph client stopped")
				}
			}
		case "bus":
			if o.bus != nil {
				if err := o.bus.Close(); err != nil {
					slog.Warn("event bus close error", "error", err)
				} else {
					slog.Debug("event bus closed")
				}
				config.SetEventBus(nil)
			}
		case "registry":
			if o.registry != nil {
				o.registry.Close()
				slog.Debug("registry closed")
			}
		}
	}

	slog.Info("all components stopped")
	return nil
}

// handleRebuild handles all rebuild operations (daemon start, periodic, manual).
// Uses mutex to prevent concurrent rebuilds from corrupting discoveredPaths map.
func (o *Orchestrator) handleRebuild(ctx context.Context, full bool) (*RebuildResult, error) {
	o.rebuildMu.Lock()
	defer o.rebuildMu.Unlock()

	start := time.Now()

	// Validate and clean missing remembered paths before rebuild
	removedPaths := o.validateRememberedPaths(ctx)

	if o.walker == nil {
		return nil, fmt.Errorf("walker not initialized")
	}

	var err error
	if full {
		err = o.walker.WalkAll(ctx)
	} else {
		err = o.walker.WalkAllIncremental(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("rebuild walk failed; %w", err)
	}

	// Run reconciliation after walk completes to clean up stale entries
	discoveredPaths := o.walker.DrainDiscoveredPaths()
	if discoveredPaths != nil && o.cleaner != nil {
		// Get remembered paths to reconcile against
		rememberedPaths, listErr := o.registry.ListPaths(ctx)
		if listErr != nil {
			slog.Warn("failed to list paths for reconciliation", "error", listErr)
		} else {
			for _, rp := range rememberedPaths {
				result, reconcileErr := o.cleaner.Reconcile(ctx, rp.Path, discoveredPaths)
				if reconcileErr != nil {
					slog.Warn("reconciliation failed", "path", rp.Path, "error", reconcileErr)
				} else if result.StaleRemoved > 0 {
					slog.Info("reconciliation complete",
						"path", rp.Path,
						"stale_removed", result.StaleRemoved,
						"duration", result.Duration)
				}
			}
		}
	}

	stats := o.walker.Stats()
	duration := time.Since(start)

	// Publish rebuild complete event for MCP notifications
	if o.bus != nil {
		o.bus.Publish(ctx, events.NewEvent(events.RebuildComplete,
			events.RebuildCompleteEvent{
				FilesQueued:   int(stats.FilesDiscovered),
				DirsProcessed: int(stats.DirsTraversed),
				Duration:      duration,
				Full:          full,
			},
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

// handleRebuildWithRecord wraps handleRebuild to record job results for health/status.
func (o *Orchestrator) handleRebuildWithRecord(ctx context.Context, full bool, jobName string) (*RebuildResult, error) {
	if o.jobRunner == nil {
		o.jobRunner = NewJobRunner(o.bus)
	}

	mode := "incremental"
	if full {
		mode = "full"
	}

	var rebuildResult *RebuildResult
	var runErr error

	runResult := o.jobRunner.Run(ctx, &logicalJob{name: jobName, mode: mode}, func(runCtx context.Context) RunResult {
		result := RunResult{
			Status:     RunFailed,
			StartedAt:  time.Now(),
			Counts:     map[string]int{"files_queued": 0, "dirs_processed": 0},
			Details:    map[string]any{"full": full, "walk_mode": mode, "duration": "", "removed": 0},
			FinishedAt: time.Now(),
		}

		var err error
		rebuildResult, err = o.handleRebuild(runCtx, full)
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

	o.recordJobResult(jobName, runResult)

	return rebuildResult, runErr
}

// startPeriodicRebuild starts a goroutine that triggers incremental rebuilds at the configured interval.
func (o *Orchestrator) startPeriodicRebuild(ctx context.Context, interval time.Duration) {
	o.rebuildStopChan = make(chan struct{})

	slog.Info("periodic rebuild enabled", "interval", interval, "mode", "incremental")

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-o.rebuildStopChan:
				return
			case <-ticker.C:
				slog.Info("starting periodic rebuild", "mode", "incremental")
				result, err := o.handleRebuildWithRecord(ctx, false, "job.rebuild_incremental") // Always incremental
				if err != nil {
					slog.Error("periodic rebuild failed", "error", err)
					continue
				}
				slog.Info("periodic rebuild complete",
					"files_queued", result.FilesQueued,
					"dirs_processed", result.DirsProcessed,
					"duration", result.Duration)
			}
		}
	}()
}

// recordJobResult stores the latest RunResult for a job.
func (o *Orchestrator) recordJobResult(name string, result RunResult) {
	o.jobMu.Lock()
	defer o.jobMu.Unlock()
	o.jobResults[name] = result
}

// runSupervisor runs startFn with backoff until context cancellation, updating health.
// If fatalCh is non-nil, runtime fatal errors can be signaled for restart.
func (o *Orchestrator) runSupervisor(ctx context.Context, name string, def ComponentDefinition, startFn func(context.Context) error, fatalCh <-chan error) {
	startCtx, cancel := context.WithCancel(ctx)
	o.componentCancels[name] = cancel

	backoff := time.Second
	maxBackoff := 30 * time.Second

	go func() {
		for {
			err := startFn(startCtx)
			now := time.Now()
			if err != nil {
				slog.Warn("component start/run failed", "component", name, "error", err)
				o.daemon.UpdateComponentHealth(map[string]ComponentHealth{
					name: {
						Status:      ComponentStatusFailed,
						Error:       err.Error(),
						LastChecked: now,
					},
				})

				if def.RestartPolicy == RestartNever {
					if def.Criticality == CriticalityFatal {
						slog.Error("fatal component failed and will not restart", "component", name, "error", err)
					}
					return
				}
			} else {
				o.daemon.UpdateComponentHealth(map[string]ComponentHealth{
					name: {
						Status:      ComponentStatusRunning,
						LastChecked: now,
						LastSuccess: now,
					},
				})
				if def.RestartPolicy == RestartNever {
					return
				}

				// Wait for fatal error or cancellation before restart
				if fatalCh != nil {
					select {
					case <-startCtx.Done():
						return
					case fatalErr := <-fatalCh:
						if fatalErr != nil {
							slog.Warn("component runtime error; restarting", "component", name, "error", fatalErr)
							err = fatalErr
						}
					}
				} else {
					// No fatal channel; stay running until context cancel
					<-startCtx.Done()
					return
				}
			}

			select {
			case <-startCtx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}()
}

func busBacklogHigh(stats events.BusStats) bool {
	if stats.CriticalCap == 0 {
		return false
	}
	return float64(stats.CriticalLen) >= float64(stats.CriticalCap)*busBacklogDegradePct
}

// startHealthUpdater runs a periodic health sync into the daemon HealthManager.
func (o *Orchestrator) startHealthUpdater(ctx context.Context, interval time.Duration) {
	o.healthStopChan = make(chan struct{})
	if interval <= 0 {
		interval = 10 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-o.healthStopChan:
				return
			case <-ticker.C:
				if o.daemon != nil {
					o.daemon.UpdateComponentHealth(o.ComponentStatuses())
				}
			}
		}
	}()
}

// Bus returns the initialized event bus.
func (o *Orchestrator) Bus() *events.EventBus {
	return o.bus
}

// Registry returns the initialized registry.
func (o *Orchestrator) Registry() registry.Registry {
	return o.registry
}

// Graph returns the initialized graph client.
func (o *Orchestrator) Graph() graph.Graph {
	return o.graph
}

// IsGraphDegraded returns true if graph connection failed during startup.
func (o *Orchestrator) IsGraphDegraded() bool {
	return o.graphDegraded
}

// IsMCPDegraded returns true if MCP server startup failed.
func (o *Orchestrator) IsMCPDegraded() bool {
	return o.mcpDegraded
}

// MCPServer returns the initialized MCP server.
func (o *Orchestrator) MCPServer() *mcp.Server {
	return o.mcpServer
}

// MetricsCollector returns the initialized metrics collector.
func (o *Orchestrator) MetricsCollector() *metrics.Collector {
	return o.metricsCollector
}

// SemanticProvider returns the initialized semantic provider.
func (o *Orchestrator) SemanticProvider() providers.SemanticProvider {
	return o.semanticProvider
}

// EmbeddingsProvider returns the initialized embeddings provider.
func (o *Orchestrator) EmbeddingsProvider() providers.EmbeddingsProvider {
	return o.embedProvider
}

// Queue returns the initialized analysis queue.
func (o *Orchestrator) Queue() *analysis.Queue {
	return o.queue
}

// Walker returns the initialized walker.
func (o *Orchestrator) Walker() walker.Walker {
	return o.walker
}

// Watcher returns the initialized watcher.
func (o *Orchestrator) Watcher() watcher.Watcher {
	return o.watcher
}

// Cleaner returns the initialized cleaner.
func (o *Orchestrator) Cleaner() *cleaner.Cleaner {
	return o.cleaner
}

// validateRememberedPaths checks all remembered paths and removes those that
// no longer exist. Returns the list of removed paths. Also cleans up associated
// data (file_state entries and graph nodes) for removed paths.
func (o *Orchestrator) validateRememberedPaths(ctx context.Context) []string {
	if o.registry == nil {
		return nil
	}

	removed, err := o.registry.ValidateAndCleanPaths(ctx)
	if err != nil {
		slog.Warn("failed to validate remembered paths", "error", err)
		return nil
	}

	// For each removed path, clean up graph nodes and emit events
	for _, path := range removed {
		slog.Warn("removed missing remembered path", "path", path)

		// Clean up graph nodes (best effort)
		if o.cleaner != nil {
			if err := o.cleaner.DeletePath(ctx, path); err != nil {
				slog.Warn("failed to clean up graph for removed path",
					"path", path,
					"error", err,
				)
			}
		}

		// Emit event for observability
		if o.bus != nil {
			o.bus.Publish(ctx, events.NewEvent(events.RememberedPathRemoved,
				events.RememberedPathRemovedEvent{
					Path:   path,
					Reason: "not_found",
				},
			))
		}
	}

	return removed
}

// watchRememberedPaths starts watching all remembered paths.
func (o *Orchestrator) watchRememberedPaths(ctx context.Context) {
	paths, err := o.registry.ListPaths(ctx)
	if err != nil {
		slog.Warn("failed to list paths for watching", "error", err)
		return
	}

	for _, rp := range paths {
		if err := o.watcher.Watch(rp.Path); err != nil {
			slog.Warn("failed to watch path",
				"path", rp.Path,
				"error", err,
			)
		} else {
			slog.Debug("watching path", "path", rp.Path)
		}
	}

	slog.Info("watching remembered paths", "count", len(paths))
}

// walkRememberedPaths performs an initial full walk of all remembered paths.
// Uses full rebuild on daemon start to ensure schema version changes are applied.
func (o *Orchestrator) walkRememberedPaths(ctx context.Context) {
	slog.Info("starting initial walk of remembered paths", "mode", "full")

	result, err := o.handleRebuildWithRecord(ctx, true, "job.initial_walk") // Full rebuild on daemon start
	if err != nil {
		if ctx.Err() != nil {
			slog.Debug("initial walk canceled")
			return
		}
		slog.Warn("initial walk failed", "error", err)
		return
	}

	slog.Info("initial walk complete",
		"files_queued", result.FilesQueued,
		"dirs_processed", result.DirsProcessed,
		"duration", result.Duration,
	)
}

// ComponentStatuses returns the health status of all orchestrated components.
func (o *Orchestrator) ComponentStatuses() map[string]ComponentHealth {
	statuses := make(map[string]ComponentHealth)

	// Event bus status
	if o.bus != nil {
		busStats := o.bus.Stats()
		if o.busDegraded {
			if busStats.DropRatePerSec < busDropRecoverThreshold && !busBacklogHigh(busStats) {
				o.busDegraded = false
			}
		} else if busStats.DropRatePerSec > busDropDegradeThreshold || busBacklogHigh(busStats) {
			o.busDegraded = true
		}

		busDegraded := o.busDegraded
		status := ComponentStatusRunning
		var errMsg string
		if busDegraded {
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
				"rebuild_recommended": busDegraded,
			},
		}
	}

	// Registry status - always ok if we got here
	if o.registry != nil {
		statuses["registry"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
		}
	}

	// Graph status
	if o.graph != nil {
		if o.graphDegraded {
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
	if o.walker != nil {
		stats := o.walker.Stats()
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
	if o.watcher != nil {
		stats := o.watcher.Stats()
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
	if o.queue != nil {
		stats := o.queue.Stats()
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

	// MCP server status
	if o.mcpServer != nil {
		if o.mcpDegraded {
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
	if o.semanticProvider != nil {
		if o.semanticProvider.Available() {
			statuses["semantic_provider"] = ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
				Details: map[string]any{
					"provider": o.semanticProvider.Name(),
				},
			}
		} else {
			statuses["semantic_provider"] = ComponentHealth{
				Status:      ComponentStatusFailed,
				Error:       "not available (missing API key)",
				LastChecked: time.Now(),
				Details: map[string]any{
					"provider": o.semanticProvider.Name(),
				},
			}
		}
	}

	// Embeddings provider status
	if o.embedProvider != nil {
		if o.embedProvider.Available() {
			statuses["embeddings_provider"] = ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
				Details: map[string]any{
					"provider":   o.embedProvider.Name(),
					"model":      o.embedProvider.ModelName(),
					"dimensions": o.embedProvider.Dimensions(),
				},
			}
		} else {
			statuses["embeddings_provider"] = ComponentHealth{
				Status:      ComponentStatusFailed,
				Error:       "not available (missing API key)",
				LastChecked: time.Now(),
				Details: map[string]any{
					"provider":   o.embedProvider.Name(),
					"model":      o.embedProvider.ModelName(),
					"dimensions": o.embedProvider.Dimensions(),
				},
			}
		}
	}

	// Caches status
	if o.semanticCache != nil {
		statuses["semantic_cache"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
			Details: map[string]any{
				"enabled": true,
			},
		}
	}
	if o.embeddingsCache != nil {
		statuses["embeddings_cache"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
			Details: map[string]any{
				"enabled": true,
			},
		}
	}

	// Metrics collector
	if o.metricsCollector != nil {
		statuses["metrics_collector"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
		}
	}

	// Job results (synthetic components)
	o.jobMu.Lock()
	for name, jr := range o.jobResults {
		componentName := name
		state := ComponentStatusRunning
		if jr.Status == RunFailed {
			state = ComponentStatusFailed
		} else if jr.Status == RunPartial {
			state = ComponentStatusDegraded
		}

		statuses[componentName] = ComponentHealth{
			Status:      state,
			LastChecked: time.Now(),
			Details: map[string]any{
				"last_run_at": jr.FinishedAt,
				"started_at":  jr.StartedAt,
				"counts":      jr.Counts,
				"details":     jr.Details,
			},
			Error: jr.Error,
		}
	}
	o.jobMu.Unlock()

	return statuses
}
