package daemon

import (
	"context"
	"fmt"
	"log/slog"
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
	"github.com/leefowlercu/agentic-memorizer/internal/storage"
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
	daemon          *Daemon
	builder         *ComponentBuilder
	supervisor      *ComponentSupervisor
	healthCollector *ComponentHealthCollector
	jobManager      *JobManager
	bag             *ComponentBag

	// components is the registry of component definitions (from builder)
	components *ComponentRegistry

	// Direct component references (populated from bag for convenience)
	bus              *events.EventBus
	registry         registry.Registry
	storage          *storage.Storage
	persistenceQueue storage.DurablePersistenceQueue
	graph            graph.Graph
	semanticProvider providers.SemanticProvider
	embedProvider    providers.EmbeddingsProvider
	queue            *analysis.Queue
	drainWorker      *analysis.DrainWorker
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

	// healthStopChan stops the health updater goroutine
	healthStopChan chan struct{}

	jobComponents map[string]JobComponent

	eventUnsubs []func()
	runCtx      context.Context
}

// NewOrchestrator creates a new orchestrator for the daemon.
func NewOrchestrator(d *Daemon) *Orchestrator {
	return &Orchestrator{
		daemon:        d,
		components:    NewComponentRegistry(),
		jobComponents: make(map[string]JobComponent),
	}
}

// Initialize sets up all components in the correct order.
// Startup sequence: Registry -> Graph -> Cache -> Providers -> Walker -> Watcher -> Queue -> MCP
func (o *Orchestrator) Initialize(ctx context.Context) error {
	cfg := config.Get()

	// Build components using the ComponentBuilder
	o.builder = NewComponentBuilder(cfg, WithBuilderLogger(slog.Default()))
	bag, err := o.builder.Build(ctx)
	if err != nil {
		return err
	}
	o.bag = bag
	o.components = o.builder.Registry()

	// Populate direct references from bag for convenience
	o.bus = bag.Bus
	o.registry = bag.Registry
	o.storage = bag.Storage
	o.persistenceQueue = bag.PersistenceQueue
	o.graph = bag.Graph
	o.semanticProvider = bag.SemanticProvider
	o.embedProvider = bag.EmbedProvider
	o.semanticCache = bag.SemanticCache
	o.embeddingsCache = bag.EmbeddingsCache
	o.queue = bag.Queue
	o.drainWorker = bag.DrainWorker
	o.walker = bag.Walker
	o.watcher = bag.Watcher
	o.cleaner = bag.Cleaner
	o.mcpServer = bag.MCPServer
	o.metricsCollector = bag.MetricsCollector
	o.graphDegraded = bag.GraphDegraded
	o.mcpDegraded = bag.MCPDegraded

	if o.queue != nil && o.registry != nil {
		o.queue.SetRegistry(o.registry)
	}

	if o.metricsCollector != nil {
		o.daemon.server.SetMetricsHandler(metrics.Handler())
	}

	// Wire MCP handler to HTTP server
	if o.mcpServer != nil {
		o.daemon.server.SetMCPHandler(o.mcpServer.Handler())
	}

	if o.registry != nil {
		rememberService := NewRememberService(o.registry, o.bus, cfg.Defaults, WithLogger(slog.Default().With("component", "remember")))
		o.daemon.server.SetRememberFunc(rememberService.Remember)
		o.daemon.server.SetForgetFunc(rememberService.Forget)
	}

	// Create supervisor for component lifecycle management
	o.supervisor = NewComponentSupervisor(o.daemon, WithSupervisorLogger(slog.Default()))

	// Create health collector for status aggregation
	o.healthCollector = NewComponentHealthCollector(o.bag, WithHealthCollectorLogger(slog.Default()))

	// Create job manager for rebuild/walk operations
	o.jobManager = NewJobManager(
		o.bus,
		o.walker,
		o.cleaner,
		o.registry,
		o.healthCollector,
		WithJobManagerLogger(slog.Default()),
	)

	// Set rebuild function on daemon server (delegates to job manager)
	o.daemon.server.SetRebuildFunc(func(ctx context.Context, full bool) (*RebuildResult, error) {
		jobName := "job.rebuild_incremental"
		if full {
			jobName = "job.rebuild_full"
		}
		return o.jobManager.RebuildWithRecord(ctx, full, jobName)
	})

	o.subscribeRememberedPathEvents()
	o.subscribeHealthAndMetricsEvents()

	return nil
}

// Start starts all orchestrated components.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.runCtx = ctx

	// Registry is already initialized and doesn't need a Start call

	// Validate and clean missing remembered paths before starting components
	removedPaths := o.jobManager.ValidateAndCleanPaths(ctx)
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
				o.supervisor.Supervise(ctx, name, def, func(c context.Context) error {
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
				o.supervisor.Supervise(ctx, name, def, func(c context.Context) error {
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
				o.supervisor.Supervise(ctx, name, def, func(c context.Context) error {
					if err := o.watcher.Start(c); err != nil {
						return err
					}
					o.watchRememberedPaths(c)
					return nil
				}, o.watcher.Errors())
			}
		case "cleaner":
			if o.cleaner != nil {
				o.supervisor.Supervise(ctx, name, def, func(c context.Context) error {
					return o.cleaner.Start(c)
				}, nil)
			}
		case "mcp":
			if o.mcpServer != nil {
				o.supervisor.Supervise(ctx, name, def, func(c context.Context) error {
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
				o.supervisor.Supervise(ctx, name, def, func(c context.Context) error {
					return o.metricsCollector.Start(c)
				}, fatalChan(def, o.metricsCollector))
			}
		case "drain_worker":
			if o.drainWorker != nil {
				o.supervisor.Supervise(ctx, name, def, func(c context.Context) error {
					return o.drainWorker.Start(c)
				}, nil)
			}
		}
	}

	// Trigger initial walk (async via job manager)
	if o.walker != nil {
		go func() {
			_, _ = o.jobManager.InitialWalk(ctx)
		}()
	}

	// Start health updater
	o.startHealthUpdater(ctx, 10*time.Second)

	// Start periodic rebuild (if configured)
	cfg := config.Get()
	if cfg.Daemon.RebuildInterval > 0 {
		o.jobManager.StartPeriodicRebuild(ctx, time.Duration(cfg.Daemon.RebuildInterval)*time.Second)
	}

	return nil
}

// Stop stops all orchestrated components in reverse order.
func (o *Orchestrator) Stop(ctx context.Context) error {
	slog.Info("stopping orchestrated components")

	for _, unsub := range o.eventUnsubs {
		unsub()
	}
	o.eventUnsubs = nil
	o.runCtx = nil

	// Cancel any component-level contexts via supervisor
	if o.supervisor != nil {
		o.supervisor.CancelAll()
	}

	// Stop health updater
	if o.healthStopChan != nil {
		close(o.healthStopChan)
		o.healthStopChan = nil
	}

	// Stop periodic rebuild via job manager
	if o.jobManager != nil {
		o.jobManager.StopPeriodicRebuild()
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
				_ = o.metricsCollector.Stop(ctx)
				slog.Debug("metrics collector stopped")
			}
		case "mcp":
			if o.mcpServer != nil {
				_ = o.mcpServer.Stop(ctx)
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
				_ = o.registry.Close()
				slog.Debug("registry closed")
			}
		case "drain_worker":
			if o.drainWorker != nil {
				if err := o.drainWorker.Stop(ctx); err != nil {
					slog.Warn("drain worker stop error", "error", err)
				} else {
					slog.Debug("drain worker stopped")
				}
			}
		case "storage":
			if o.storage != nil {
				if err := o.storage.Close(); err != nil {
					slog.Warn("storage close error", "error", err)
				} else {
					slog.Debug("storage closed")
				}
			}
		}
	}

	slog.Info("all components stopped")
	return nil
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

func (o *Orchestrator) subscribeRememberedPathEvents() {
	if o.bus == nil {
		return
	}

	o.eventUnsubs = append(o.eventUnsubs,
		o.bus.Subscribe(events.RememberedPathAdded, o.handleRememberedPathAdded),
		o.bus.Subscribe(events.RememberedPathUpdated, o.handleRememberedPathUpdated),
		o.bus.Subscribe(events.RememberedPathRemoved, o.handleRememberedPathRemoved),
	)
}

func (o *Orchestrator) handleRememberedPathAdded(event events.Event) {
	pe, ok := event.Payload.(*events.RememberedPathEvent)
	if !ok {
		slog.Warn("invalid remembered path added payload")
		return
	}
	if pe.Path == "" {
		slog.Warn("remembered path added event missing path")
		return
	}

	if o.watcher != nil {
		if err := o.watcher.Watch(pe.Path); err != nil {
			slog.Warn("failed to watch remembered path", "path", pe.Path, "error", err)
		}
	}

	o.triggerRememberedPathWalk(pe.Path)
}

func (o *Orchestrator) handleRememberedPathUpdated(event events.Event) {
	pe, ok := event.Payload.(*events.RememberedPathEvent)
	if !ok {
		slog.Warn("invalid remembered path updated payload")
		return
	}
	if pe.Path == "" {
		slog.Warn("remembered path updated event missing path")
		return
	}

	if o.watcher != nil {
		if err := o.watcher.Unwatch(pe.Path); err != nil {
			slog.Warn("failed to unwatch remembered path", "path", pe.Path, "error", err)
		}
		if err := o.watcher.Watch(pe.Path); err != nil {
			slog.Warn("failed to rewatch remembered path", "path", pe.Path, "error", err)
		}
	}

	o.triggerRememberedPathWalk(pe.Path)
}

func (o *Orchestrator) handleRememberedPathRemoved(event events.Event) {
	pe, ok := event.Payload.(*events.RememberedPathRemovedEvent)
	if !ok {
		slog.Warn("invalid remembered path removed payload")
		return
	}
	if pe.Path == "" {
		slog.Warn("remembered path removed event missing path")
		return
	}

	if o.watcher != nil {
		if err := o.watcher.Unwatch(pe.Path); err != nil {
			slog.Warn("failed to unwatch remembered path", "path", pe.Path, "error", err)
		}
	}
}

func (o *Orchestrator) triggerRememberedPathWalk(path string) {
	if o.jobManager == nil {
		return
	}

	ctx := o.eventContext()
	go func() {
		if err := o.jobManager.WalkPath(ctx, path); err != nil {
			if ctx.Err() != nil {
				slog.Debug("remembered path walk canceled", "path", path)
				return
			}
			slog.Warn("remembered path walk failed", "path", path, "error", err)
		}
	}()
}

// subscribeHealthAndMetricsEvents subscribes to new events for health updates and metrics.
func (o *Orchestrator) subscribeHealthAndMetricsEvents() {
	if o.bus == nil {
		return
	}

	o.eventUnsubs = append(o.eventUnsubs,
		// Queue degradation events
		o.bus.Subscribe(events.QueueDegradationChanged, o.handleQueueDegradationChanged),

		// Watcher degradation events
		o.bus.Subscribe(events.WatcherDegraded, o.handleWatcherDegraded),
		o.bus.Subscribe(events.WatcherRecovered, o.handleWatcherRecovered),

		// Graph connection events
		o.bus.Subscribe(events.GraphConnected, o.handleGraphConnected),
		o.bus.Subscribe(events.GraphDisconnected, o.handleGraphDisconnected),
		o.bus.Subscribe(events.GraphWriteQueueFull, o.handleGraphWriteQueueFull),

		// Analysis events
		o.bus.Subscribe(events.AnalysisSkipped, o.handleAnalysisSkipped),
		o.bus.Subscribe(events.AnalysisSemanticComplete, o.handleAnalysisSemanticComplete),
		o.bus.Subscribe(events.AnalysisEmbeddingsComplete, o.handleAnalysisEmbeddingsComplete),

		// Rebuild events
		o.bus.Subscribe(events.RebuildStarted, o.handleRebuildStarted),
	)
}

func (o *Orchestrator) handleQueueDegradationChanged(event events.Event) {
	payload, ok := event.Payload.(*events.QueueDegradationEvent)
	if !ok {
		return
	}

	metrics.QueueDegradationTransitionsTotal.WithLabelValues(payload.PreviousMode, payload.CurrentMode).Inc()

	// Update health status based on degradation mode
	if o.daemon != nil && o.daemon.health != nil {
		now := time.Now()
		if payload.CurrentMode == "metadata" {
			o.daemon.health.UpdateComponent("queue", ComponentHealth{
				Status:      ComponentStatusDegraded,
				LastChecked: now,
				Since:       now,
			})
		} else {
			o.daemon.health.UpdateComponent("queue", ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: now,
				LastSuccess: now,
			})
		}
	}
}

func (o *Orchestrator) handleWatcherDegraded(event events.Event) {
	metrics.WatcherDegradedTotal.Inc()

	if o.daemon != nil && o.daemon.health != nil {
		now := time.Now()
		o.daemon.health.UpdateComponent("watcher", ComponentHealth{
			Status:      ComponentStatusDegraded,
			LastChecked: now,
			Since:       now,
		})
	}
}

func (o *Orchestrator) handleWatcherRecovered(event events.Event) {
	metrics.WatcherRecoveredTotal.Inc()

	if o.daemon != nil && o.daemon.health != nil {
		now := time.Now()
		o.daemon.health.UpdateComponent("watcher", ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: now,
			LastSuccess: now,
		})
	}
}

func (o *Orchestrator) handleGraphConnected(event events.Event) {
	metrics.GraphConnectionsTotal.Inc()

	if o.daemon != nil && o.daemon.health != nil {
		now := time.Now()
		o.daemon.health.UpdateComponent("graph", ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: now,
			LastSuccess: now,
		})
	}
}

func (o *Orchestrator) handleGraphDisconnected(event events.Event) {
	metrics.GraphDisconnectionsTotal.Inc()

	if o.daemon != nil && o.daemon.health != nil {
		now := time.Now()
		o.daemon.health.UpdateComponent("graph", ComponentHealth{
			Status:      ComponentStatusDegraded,
			LastChecked: now,
			Since:       now,
		})
	}
}

func (o *Orchestrator) handleGraphWriteQueueFull(event events.Event) {
	metrics.GraphWriteQueueFullTotal.Inc()

	if o.daemon != nil && o.daemon.health != nil {
		now := time.Now()
		o.daemon.health.UpdateComponent("graph", ComponentHealth{
			Status:      ComponentStatusDegraded,
			LastChecked: now,
			Since:       now,
		})
	}
}

func (o *Orchestrator) handleAnalysisSkipped(event events.Event) {
	payload, ok := event.Payload.(*events.IngestDecisionEvent)
	if !ok {
		return
	}
	metrics.AnalysisSkippedTotal.WithLabelValues(payload.Decision, payload.Reason).Inc()
}

func (o *Orchestrator) handleAnalysisSemanticComplete(event events.Event) {
	metrics.AnalysisSemanticCompleteTotal.Inc()
}

func (o *Orchestrator) handleAnalysisEmbeddingsComplete(event events.Event) {
	metrics.AnalysisEmbeddingsCompleteTotal.Inc()
}

func (o *Orchestrator) handleRebuildStarted(event events.Event) {
	payload, ok := event.Payload.(*events.RebuildStartedEvent)
	if !ok {
		return
	}
	metrics.RebuildStartedTotal.WithLabelValues(payload.Trigger).Inc()
}

func (o *Orchestrator) eventContext() context.Context {
	if o.runCtx != nil {
		return o.runCtx
	}
	return context.Background()
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

// ComponentStatuses returns the health status of all orchestrated components.
func (o *Orchestrator) ComponentStatuses() map[string]ComponentHealth {
	if o.healthCollector == nil {
		return make(map[string]ComponentHealth)
	}
	return o.healthCollector.Collect()
}
