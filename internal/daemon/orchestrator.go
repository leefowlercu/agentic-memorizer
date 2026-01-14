package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/analysis"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/handlers"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp"
	"github.com/leefowlercu/agentic-memorizer/internal/metrics"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
	"github.com/leefowlercu/agentic-memorizer/internal/watcher"
)

// Orchestrator manages the initialization and wiring of all daemon components.
type Orchestrator struct {
	daemon           *Daemon
	bus              *events.EventBus
	registry         registry.Registry
	graph            graph.Graph
	handlers         *handlers.Registry
	semanticProvider providers.SemanticProvider
	embedProvider    providers.EmbeddingsProvider
	queue            *analysis.Queue
	walker           walker.Walker
	watcher          watcher.Watcher
	mcpServer        *mcp.Server
	metricsCollector *metrics.Collector

	// graphDegraded tracks if graph connection failed during startup
	graphDegraded bool

	// rebuildStopChan signals the periodic rebuild goroutine to stop
	rebuildStopChan chan struct{}
}

// NewOrchestrator creates a new orchestrator for the daemon.
func NewOrchestrator(d *Daemon) *Orchestrator {
	return &Orchestrator{
		daemon: d,
	}
}

// Initialize sets up all components in the correct order.
// Startup sequence: Registry -> Graph -> Cache -> Providers -> Walker -> Watcher -> Queue -> MCP
func (o *Orchestrator) Initialize(ctx context.Context) error {
	cfg := config.Get()

	// 1. Initialize Event Bus
	o.bus = events.NewBus(events.WithBufferSize(100))
	slog.Info("event bus initialized")

	// 2. Initialize SQLite Registry
	registryPath := config.ExpandPath(cfg.Daemon.RegistryPath)
	reg, err := registry.Open(ctx, registryPath)
	if err != nil {
		return fmt.Errorf("failed to open registry; %w", err)
	}
	o.registry = reg
	slog.Info("registry initialized", "path", registryPath)

	// 3. Initialize Graph Client
	graphCfg := graph.Config{
		Host:        cfg.Graph.Host,
		Port:        cfg.Graph.Port,
		GraphName:   cfg.Graph.Name,
		PasswordEnv: cfg.Graph.PasswordEnv,
		MaxRetries:  cfg.Graph.MaxRetries,
		RetryDelay:  time.Duration(cfg.Graph.RetryDelayMs) * time.Millisecond,
	}
	o.graph = graph.NewFalkorDBGraph(
		graph.WithConfig(graphCfg),
		graph.WithLogger(slog.Default().With("component", "graph")),
	)
	slog.Info("graph client initialized",
		"host", graphCfg.Host,
		"port", graphCfg.Port,
		"graph", graphCfg.GraphName,
	)

	// 4. Initialize Semantic Provider
	semanticProvider, err := createSemanticProvider(&cfg.Semantic)
	if err != nil {
		slog.Warn("semantic provider initialization failed; analysis disabled",
			"error", err,
		)
	} else {
		o.semanticProvider = semanticProvider
	}

	// 5. Initialize Embeddings Provider (if enabled)
	embedProvider, err := createEmbeddingsProvider(&cfg.Embeddings)
	if err != nil {
		slog.Warn("embeddings provider initialization failed; embeddings disabled",
			"error", err,
		)
	} else {
		o.embedProvider = embedProvider
	}

	// Log provider status
	logProviderStatus(o.semanticProvider, o.embedProvider)

	// 6. Initialize Handler Registry
	o.handlers = handlers.NewRegistry(
		handlers.NewTextHandler(),
		handlers.NewImageHandler(),
		handlers.NewPDFHandler(),
		handlers.NewRichDocumentHandler(),
		handlers.NewStructuredDataHandler(),
		handlers.NewArchiveHandler(),
	)
	o.handlers.SetFallback(handlers.NewUnsupportedHandler())
	slog.Info("handler registry initialized",
		"handlers", len(o.handlers.ListHandlers()),
	)

	// 7. Initialize Analysis Queue
	workerCount := runtime.NumCPU()
	if workerCount < 2 {
		workerCount = 2
	}
	if workerCount > 8 {
		workerCount = 8
	}
	o.queue = analysis.NewQueue(o.bus,
		analysis.WithWorkerCount(workerCount),
		analysis.WithQueueCapacity(1000),
		analysis.WithLogger(slog.Default().With("component", "analysis-queue")),
	)
	slog.Info("analysis queue initialized",
		"workers", workerCount,
	)

	// 8. Initialize Walker
	o.walker = walker.New(o.registry, o.bus, o.handlers)
	slog.Info("walker initialized")

	// 9. Initialize Watcher
	w, err := watcher.New(o.bus, o.registry,
		watcher.WithLogger(slog.Default().With("component", "watcher")),
	)
	if err != nil {
		return fmt.Errorf("failed to create watcher; %w", err)
	}
	o.watcher = w
	slog.Info("watcher initialized")

	// 10. Initialize MCP Server
	mcpCfg := mcp.DefaultConfig()
	o.mcpServer = mcp.NewServer(o.graph, mcpCfg)
	slog.Info("MCP server initialized",
		"name", mcpCfg.Name,
		"base_path", mcpCfg.BasePath,
	)

	// Initialize Metrics Collector
	metricsInterval := time.Duration(cfg.Daemon.Metrics.CollectionInterval) * time.Second
	if metricsInterval == 0 {
		metricsInterval = 15 * time.Second
	}
	o.metricsCollector = metrics.NewCollector(metricsInterval)

	// Register metrics providers
	if o.queue != nil {
		o.metricsCollector.Register("queue", o.queue)
	}
	if o.watcher != nil {
		o.metricsCollector.Register("watcher", o.watcher)
	}
	if g, ok := o.graph.(*graph.FalkorDBGraph); ok && g != nil {
		o.metricsCollector.Register("graph", g)
	}

	// Set metrics handler on daemon server
	o.daemon.server.SetMetricsHandler(metrics.Handler())

	// Set rebuild function on daemon server
	o.daemon.server.SetRebuildFunc(o.handleRebuild)

	return nil
}

// Start starts all orchestrated components.
func (o *Orchestrator) Start(ctx context.Context) error {
	// Registry is already initialized and doesn't need a Start call

	// Start graph client (graceful degradation if connection fails)
	if o.graph != nil {
		if err := o.graph.Start(ctx); err != nil {
			slog.Warn("graph connection failed; entering degraded mode",
				"error", err,
			)
			o.graphDegraded = true
			// Continue without graph - graceful degradation
		} else {
			slog.Info("graph client connected")
		}
	}

	// Start analysis queue
	if o.queue != nil {
		if err := o.queue.Start(ctx); err != nil {
			return fmt.Errorf("failed to start analysis queue; %w", err)
		}
		// Inject providers into workers
		o.queue.SetProviders(o.semanticProvider, o.embedProvider)

		// Inject graph for result persistence (only if not degraded)
		if !o.graphDegraded && o.graph != nil {
			o.queue.SetGraph(o.graph)
		}
	}

	// Start watcher
	if o.watcher != nil {
		if err := o.watcher.Start(ctx); err != nil {
			return fmt.Errorf("failed to start watcher; %w", err)
		}
		// Watch all remembered paths
		o.watchRememberedPaths(ctx)
	}

	// Trigger initial walk (async)
	if o.walker != nil {
		go o.walkRememberedPaths(ctx)
	}

	// Start MCP server
	if o.mcpServer != nil {
		if err := o.mcpServer.Start(ctx); err != nil {
			return fmt.Errorf("failed to start MCP server; %w", err)
		}
	}

	// Start metrics collector
	if o.metricsCollector != nil {
		if err := o.metricsCollector.Start(ctx); err != nil {
			return fmt.Errorf("failed to start metrics collector; %w", err)
		}
	}

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

	// Stop periodic rebuild
	if o.rebuildStopChan != nil {
		close(o.rebuildStopChan)
		slog.Debug("periodic rebuild stopped")
	}

	// Stop metrics collector
	if o.metricsCollector != nil {
		o.metricsCollector.Stop(ctx)
		slog.Debug("metrics collector stopped")
	}

	// Stop MCP server
	if o.mcpServer != nil {
		o.mcpServer.Stop(ctx)
		slog.Debug("MCP server stopped")
	}

	// Stop watcher
	if o.watcher != nil {
		if err := o.watcher.Stop(); err != nil {
			slog.Warn("watcher stop error", "error", err)
		} else {
			slog.Debug("watcher stopped")
		}
	}

	// Stop analysis queue (before graph so pending writes complete)
	if o.queue != nil {
		if err := o.queue.Stop(ctx); err != nil {
			slog.Warn("analysis queue stop error", "error", err)
		} else {
			slog.Debug("analysis queue stopped")
		}
	}

	// Stop graph client (drains write queue)
	if o.graph != nil {
		if err := o.graph.Stop(ctx); err != nil {
			slog.Warn("graph shutdown error", "error", err)
		} else {
			slog.Debug("graph client stopped")
		}
	}

	// Close event bus
	if o.bus != nil {
		if err := o.bus.Close(); err != nil {
			slog.Warn("event bus close error", "error", err)
		} else {
			slog.Debug("event bus closed")
		}
	}

	// Close registry (last - other components may use it during shutdown)
	if o.registry != nil {
		o.registry.Close()
		slog.Debug("registry closed")
	}

	slog.Info("all components stopped")
	return nil
}

// handleRebuild handles rebuild requests from the HTTP API.
func (o *Orchestrator) handleRebuild(ctx context.Context, full bool) (*RebuildResult, error) {
	start := time.Now()

	if o.walker == nil {
		return nil, fmt.Errorf("walker not initialized")
	}

	// Get all remembered paths from registry
	paths, err := o.registry.ListPaths(ctx)
	if err != nil {
		return nil, err
	}

	// Walk each path (full or incremental based on flag)
	for _, rp := range paths {
		var walkErr error
		if full {
			walkErr = o.walker.Walk(ctx, rp.Path)
		} else {
			walkErr = o.walker.WalkIncremental(ctx, rp.Path)
		}
		if walkErr != nil {
			slog.Warn("rebuild walk failed",
				"path", rp.Path,
				"error", walkErr,
			)
		}
	}

	// Get actual stats
	stats := o.walker.Stats()
	duration := time.Since(start)

	return &RebuildResult{
		Status:        "completed",
		FilesQueued:   int(stats.FilesDiscovered),
		DirsProcessed: len(paths),
		Duration:      duration.Round(time.Millisecond).String(),
	}, nil
}

// startPeriodicRebuild starts a goroutine that triggers full rebuilds at the configured interval.
func (o *Orchestrator) startPeriodicRebuild(ctx context.Context, interval time.Duration) {
	o.rebuildStopChan = make(chan struct{})

	slog.Info("periodic rebuild enabled", "interval", interval)

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
				slog.Info("starting periodic rebuild")
				result, err := o.handleRebuild(ctx, true)
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

// MCPServer returns the initialized MCP server.
func (o *Orchestrator) MCPServer() *mcp.Server {
	return o.mcpServer
}

// MetricsCollector returns the initialized metrics collector.
func (o *Orchestrator) MetricsCollector() *metrics.Collector {
	return o.metricsCollector
}

// Handlers returns the initialized handler registry.
func (o *Orchestrator) Handlers() *handlers.Registry {
	return o.handlers
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

// walkRememberedPaths performs an initial walk of all remembered paths.
func (o *Orchestrator) walkRememberedPaths(ctx context.Context) {
	slog.Info("starting initial walk of remembered paths")

	if err := o.walker.WalkAll(ctx); err != nil {
		if ctx.Err() != nil {
			slog.Debug("initial walk cancelled")
			return
		}
		slog.Warn("initial walk failed", "error", err)
		return
	}

	stats := o.walker.Stats()
	slog.Info("initial walk complete",
		"files_discovered", stats.FilesDiscovered,
		"dirs_traversed", stats.DirsTraversed,
		"files_skipped", stats.FilesSkipped,
	)
}

// ComponentStatuses returns the health status of all orchestrated components.
func (o *Orchestrator) ComponentStatuses() map[string]ComponentHealth {
	statuses := make(map[string]ComponentHealth)

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
				Status:      ComponentStatusFailed,
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
				Status:      ComponentStatusRunning,
				Error:       "degraded mode (watch limit reached)",
				LastChecked: time.Now(),
			}
		} else {
			statuses["watcher"] = ComponentHealth{
				Status:      status,
				LastChecked: time.Now(),
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
		statuses["queue"] = ComponentHealth{
			Status:      status,
			LastChecked: time.Now(),
		}
	}

	// MCP server status
	if o.mcpServer != nil {
		statuses["mcp"] = ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
		}
	}

	// Semantic provider status
	if o.semanticProvider != nil {
		if o.semanticProvider.Available() {
			statuses["semantic_provider"] = ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
			}
		} else {
			statuses["semantic_provider"] = ComponentHealth{
				Status:      ComponentStatusFailed,
				Error:       "not available (missing API key)",
				LastChecked: time.Now(),
			}
		}
	}

	// Embeddings provider status
	if o.embedProvider != nil {
		if o.embedProvider.Available() {
			statuses["embeddings_provider"] = ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
			}
		} else {
			statuses["embeddings_provider"] = ComponentHealth{
				Status:      ComponentStatusFailed,
				Error:       "not available (missing API key)",
				LastChecked: time.Now(),
			}
		}
	}

	return statuses
}
