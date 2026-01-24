package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/analysis"
	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
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

// ComponentBuilder constructs daemon components in dependency order.
type ComponentBuilder struct {
	registry *ComponentRegistry
	cfg      *config.Config
	logger   *slog.Logger
}

// BuilderOption configures ComponentBuilder.
type BuilderOption func(*ComponentBuilder)

// WithBuilderLogger sets the logger for build operations.
func WithBuilderLogger(l *slog.Logger) BuilderOption {
	return func(b *ComponentBuilder) {
		b.logger = l
	}
}

// NewComponentBuilder creates a builder with registered component definitions.
func NewComponentBuilder(cfg *config.Config, opts ...BuilderOption) *ComponentBuilder {
	b := &ComponentBuilder{
		registry: NewComponentRegistry(),
		cfg:      cfg,
		logger:   slog.Default(),
	}

	for _, opt := range opts {
		opt(b)
	}

	b.registerDefinitions()
	return b
}

// Registry returns the underlying ComponentRegistry for ordering queries.
func (b *ComponentBuilder) Registry() *ComponentRegistry {
	return b.registry
}

// Build constructs all components in topological order, returning a ComponentBag.
// Fatal components that fail cause Build to return an error.
// Degradable components that fail are logged and skipped.
func (b *ComponentBuilder) Build(ctx context.Context) (*ComponentBag, error) {
	order, err := b.registry.TopologicalOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to determine build order; %w", err)
	}

	bag := &ComponentBag{}
	compCtx := ComponentContext{}

	for _, name := range order {
		def := b.registry.defs[name]
		obj, err := def.Build(ctx, compCtx)
		if err != nil {
			if def.Criticality == CriticalityFatal {
				return nil, fmt.Errorf("failed to build component %s; %w", name, err)
			}
			b.logger.Warn("component build failed; continuing in degraded mode",
				"component", name,
				"error", err,
			)
			continue
		}
		if obj == nil {
			continue
		}

		b.assignComponent(name, obj, bag, &compCtx)
	}

	logProviderStatus(bag.SemanticProvider, bag.EmbedProvider)

	return bag, nil
}

// assignComponent assigns the built object to the appropriate bag and context fields.
// Note: Concrete types must appear before interface types in the switch to avoid
// unreachable cases when a concrete type implements an interface.
func (b *ComponentBuilder) assignComponent(name string, obj any, bag *ComponentBag, ctx *ComponentContext) {
	switch c := obj.(type) {
	case *events.EventBus:
		bag.Bus = c
		ctx.Bus = c
		config.SetEventBus(c)
	case *storage.Storage:
		bag.Storage = c
		ctx.Storage = c
	case *storage.PersistenceQueue:
		bag.PersistenceQueue = c
		ctx.PersistenceQueue = c
	case registry.Registry:
		// Must come after *storage.Storage since Storage implements Registry
		bag.Registry = c
		ctx.Registry = c
	case graph.Graph:
		bag.Graph = c
		ctx.Graph = c
	case *cache.SemanticCache:
		bag.SemanticCache = c
		ctx.Caches.Semantic = c
	case *cache.EmbeddingsCache:
		bag.EmbeddingsCache = c
		ctx.Caches.Embeddings = c
	case providers.SemanticProvider:
		bag.SemanticProvider = c
		ctx.Providers.Semantic = c
	case providers.EmbeddingsProvider:
		bag.EmbedProvider = c
		ctx.Providers.Embed = c
	case *analysis.Queue:
		bag.Queue = c
		ctx.Queue = c
	case *analysis.DrainWorker:
		bag.DrainWorker = c
		ctx.DrainWorker = c
	case walker.Walker:
		bag.Walker = c
		ctx.Walker = c
	case watcher.Watcher:
		bag.Watcher = c
		ctx.Watcher = c
	case *cleaner.Cleaner:
		bag.Cleaner = c
		ctx.Cleaner = c
	case *mcp.Server:
		bag.MCPServer = c
		ctx.MCP = c
	case *metrics.Collector:
		bag.MetricsCollector = c
		ctx.MetricsCollector = c
	default:
		b.logger.Warn("unknown component type returned; ignoring", "component", name)
	}
}

// registerDefinitions populates the component registry with definitions.
func (b *ComponentBuilder) registerDefinitions() {
	cfg := b.cfg

	// Event bus
	b.registry.Register(ComponentDefinition{
		Name:          "bus",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityFatal,
		RestartPolicy: RestartNever,
		Dependencies:  nil,
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			criticalQueuePath := config.ExpandPath(cfg.Daemon.EventBus.CriticalQueuePath)
			if err := os.MkdirAll(filepath.Dir(criticalQueuePath), 0o755); err != nil {
				slog.Warn("failed to ensure critical queue directory", "error", err)
			}
			cq, err := events.NewSQLiteCriticalQueue(criticalQueuePath, cfg.Daemon.EventBus.CriticalQueueCapacity)
			if err != nil {
				slog.Warn("failed to initialize critical queue; continuing without persistence", "error", err)
				cq = nil
			}

			busOpts := []events.BusOption{events.WithBufferSize(cfg.Daemon.EventBus.BufferSize)}
			if cq != nil {
				busOpts = append(busOpts, events.WithCriticalQueue(cq, []events.EventType{events.PathDeleted, events.FileDiscovered}))
			}
			return events.NewBus(busOpts...), nil
		},
	})

	// Consolidated storage (SQLite)
	b.registry.Register(ComponentDefinition{
		Name:          "storage",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityFatal,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"bus"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			dbPath := config.ExpandPath(cfg.Storage.DatabasePath)
			s, err := storage.Open(ctx, dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open storage; %w", err)
			}
			slog.Info("storage initialized", "path", dbPath)
			return s, nil
		},
	})

	// Persistence queue (uses storage)
	b.registry.Register(ComponentDefinition{
		Name:          "persistence_queue",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"storage"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			if deps.Storage == nil {
				slog.Warn("storage not available; persistence queue disabled")
				return nil, nil
			}
			pq := deps.Storage.PersistenceQueue()
			slog.Info("persistence queue initialized")
			return pq, nil
		},
	})

	// Registry (SQLite)
	b.registry.Register(ComponentDefinition{
		Name:          "registry",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityFatal,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"bus"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			registryPath := config.ExpandPath(cfg.Daemon.RegistryPath)
			reg, err := registry.Open(ctx, registryPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open registry; %w", err)
			}
			slog.Info("registry initialized", "path", registryPath)
			return reg, nil
		},
	})

	// Graph client
	b.registry.Register(ComponentDefinition{
		Name:          "graph",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"bus"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			graphCfg := graph.Config{
				Host:               cfg.Graph.Host,
				Port:               cfg.Graph.Port,
				GraphName:          cfg.Graph.Name,
				PasswordEnv:        cfg.Graph.PasswordEnv,
				MaxRetries:         cfg.Graph.MaxRetries,
				RetryDelay:         time.Duration(cfg.Graph.RetryDelayMs) * time.Millisecond,
				EmbeddingDimension: cfg.Embeddings.Dimensions,
				WriteQueueSize:     cfg.Graph.WriteQueueSize,
			}
			opts := []graph.Option{
				graph.WithConfig(graphCfg),
				graph.WithLogger(slog.Default().With("component", "graph")),
			}
			if deps.Bus != nil {
				opts = append(opts, graph.WithBus(deps.Bus))
			}
			g := graph.NewFalkorDBGraph(opts...)
			slog.Info("graph client initialized",
				"host", graphCfg.Host,
				"port", graphCfg.Port,
				"graph", graphCfg.GraphName,
			)
			return g, nil
		},
		FatalChan: func(component any) <-chan error {
			if g, ok := component.(*graph.FalkorDBGraph); ok {
				return g.Errors()
			}
			return nil
		},
	})

	// Caches
	b.registry.Register(ComponentDefinition{
		Name:          "semantic_cache",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  nil,
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			cacheBaseDir := cache.GetCacheBaseDir()
			semanticCache, err := cache.NewSemanticCache(cache.SemanticCacheConfig{
				BaseDir: cacheBaseDir,
				Version: cache.SemanticCacheVersion,
			})
			if err != nil {
				slog.Warn("semantic cache initialization failed", "error", err)
				return nil, nil
			}
			slog.Info("semantic cache initialized",
				"base_dir", cacheBaseDir,
				"version", cache.SemanticCacheVersion,
			)
			return semanticCache, nil
		},
	})

	b.registry.Register(ComponentDefinition{
		Name:          "embeddings_cache",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  nil,
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			if !cfg.Embeddings.Enabled {
				return nil, nil
			}
			cacheBaseDir := cache.GetCacheBaseDir()
			embeddingsCache, err := cache.NewEmbeddingsCache(cache.EmbeddingsCacheConfig{
				BaseDir:  cacheBaseDir,
				Version:  cache.EmbeddingsCacheVersion,
				Provider: cfg.Embeddings.Provider,
				Model:    cfg.Embeddings.Model,
			})
			if err != nil {
				slog.Warn("embeddings cache initialization failed", "error", err)
				return nil, nil
			}
			slog.Info("embeddings cache initialized",
				"base_dir", cacheBaseDir,
				"provider", cfg.Embeddings.Provider,
				"model", cfg.Embeddings.Model,
				"version", cache.EmbeddingsCacheVersion,
			)
			return embeddingsCache, nil
		},
	})

	// Providers
	b.registry.Register(ComponentDefinition{
		Name:          "semantic_provider",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"semantic_cache"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			provider, err := createSemanticProvider(&cfg.Semantic)
			if err != nil {
				slog.Warn("semantic provider initialization failed; analysis disabled", "error", err)
				return nil, nil
			}
			return provider, nil
		},
	})

	b.registry.Register(ComponentDefinition{
		Name:          "embeddings_provider",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"embeddings_cache"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			provider, err := createEmbeddingsProvider(&cfg.Embeddings)
			if err != nil {
				slog.Warn("embeddings provider initialization failed; embeddings disabled", "error", err)
				return nil, nil
			}
			return provider, nil
		},
	})

	// Queue
	b.registry.Register(ComponentDefinition{
		Name:          "queue",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityFatal,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"bus", "registry", "graph", "persistence_queue", "semantic_provider", "embeddings_provider", "semantic_cache", "embeddings_cache"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			workerCount := min(max(runtime.NumCPU(), 2), 8)
			logger := slog.Default().With("component", "analysis-queue")

			// Build PipelineConfig from available dependencies
			pipelineCfg := &analysis.PipelineConfig{
				Registry:           deps.Registry,
				ChunkerRegistry:    chunkers.DefaultRegistry(),
				SemanticProvider:   deps.Providers.Semantic,
				SemanticCache:      deps.Caches.Semantic,
				EmbeddingsProvider: deps.Providers.Embed,
				EmbeddingsCache:    deps.Caches.Embeddings,
				Graph:              deps.Graph,
				PersistenceQueue:   deps.PersistenceQueue,
				AnalysisVersion:    "1.0.0",
				Logger:             logger,
			}

			q := analysis.NewQueue(deps.Bus,
				analysis.WithWorkerCount(workerCount),
				analysis.WithQueueCapacity(1000),
				analysis.WithLogger(logger),
				analysis.WithPipelineConfig(pipelineCfg),
			)
			slog.Info("analysis queue initialized",
				"workers", workerCount,
				"pipeline", true,
				"persistence_queue", deps.PersistenceQueue != nil,
			)
			return q, nil
		},
		FatalChan: func(component any) <-chan error {
			if q, ok := component.(*analysis.Queue); ok {
				return q.Errors()
			}
			return nil
		},
	})

	// Drain worker (drains persistence queue when graph becomes available)
	b.registry.Register(ComponentDefinition{
		Name:          "drain_worker",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"bus", "graph", "persistence_queue"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			if deps.PersistenceQueue == nil {
				slog.Debug("persistence queue not available; drain worker disabled")
				return nil, nil
			}
			if deps.Graph == nil {
				slog.Debug("graph not available; drain worker disabled")
				return nil, nil
			}

			// Build drain config from application config
			drainCfg := analysis.DrainConfig{
				BatchSize:          cfg.PersistenceQueue.DrainBatchSize,
				MaxRetries:         cfg.PersistenceQueue.MaxRetries,
				RetryBackoff:       time.Duration(cfg.PersistenceQueue.RetryBackoffMs) * time.Millisecond,
				CompletedRetention: time.Duration(cfg.PersistenceQueue.CompletedRetentionMin) * time.Minute,
				FailedRetention:    time.Duration(cfg.PersistenceQueue.FailedRetentionDays) * 24 * time.Hour,
			}

			dw := analysis.NewDrainWorker(
				deps.PersistenceQueue,
				deps.Graph,
				deps.Bus,
				analysis.WithDrainConfig(drainCfg),
				analysis.WithDrainLogger(slog.Default().With("component", "drain_worker")),
			)
			slog.Info("drain worker initialized",
				"batch_size", drainCfg.BatchSize,
				"max_retries", drainCfg.MaxRetries,
			)
			return dw, nil
		},
	})

	// Walker
	b.registry.Register(ComponentDefinition{
		Name:          "walker",
		Kind:          ComponentKindJob,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"registry", "bus"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			w := walker.New(deps.Registry, deps.Bus)
			slog.Info("walker initialized")
			return w, nil
		},
	})

	// Job components registration (logical jobs)
	b.registry.Register(ComponentDefinition{
		Name:          "job.initial_walk",
		Kind:          ComponentKindJob,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"walker", "queue", "cleaner"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			return &logicalJob{name: "job.initial_walk", mode: "full"}, nil
		},
	})

	b.registry.Register(ComponentDefinition{
		Name:          "job.rebuild_full",
		Kind:          ComponentKindJob,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"walker", "queue", "cleaner"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			return &logicalJob{name: "job.rebuild_full", mode: "full"}, nil
		},
	})

	b.registry.Register(ComponentDefinition{
		Name:          "job.rebuild_incremental",
		Kind:          ComponentKindJob,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"walker", "queue", "cleaner"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			return &logicalJob{name: "job.rebuild_incremental", mode: "incremental"}, nil
		},
	})

	// Watcher
	b.registry.Register(ComponentDefinition{
		Name:          "watcher",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"bus", "registry"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			w, err := watcher.New(deps.Bus, deps.Registry,
				watcher.WithLogger(slog.Default().With("component", "watcher")),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create watcher; %w", err)
			}
			slog.Info("watcher initialized")
			return w, nil
		},
	})

	// Cleaner
	b.registry.Register(ComponentDefinition{
		Name:          "cleaner",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"registry", "graph", "bus"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			cl := cleaner.New(
				deps.Registry,
				deps.Graph,
				deps.Bus,
				cleaner.WithLogger(slog.Default().With("component", "cleaner")),
			)
			slog.Info("cleaner initialized")
			return cl, nil
		},
	})

	// MCP
	b.registry.Register(ComponentDefinition{
		Name:          "mcp",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"graph", "registry", "bus"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			mcpCfg := mcp.DefaultConfig()
			regAdapter := newRegistryAdapter(deps.Registry)
			server := mcp.NewServer(deps.Graph, regAdapter, deps.Bus, mcpCfg)
			slog.Info("MCP server initialized", "name", mcpCfg.Name, "base_path", mcpCfg.BasePath)
			return server, nil
		},
		FatalChan: func(component any) <-chan error {
			if srv, ok := component.(*mcp.Server); ok {
				return srv.Errors()
			}
			return nil
		},
	})

	// Metrics collector
	b.registry.Register(ComponentDefinition{
		Name:          "metrics_collector",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"queue", "watcher", "graph"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			metricsInterval := time.Duration(cfg.Daemon.Metrics.CollectionInterval) * time.Second
			if metricsInterval == 0 {
				metricsInterval = 15 * time.Second
			}
			collector := metrics.NewCollector(metricsInterval)
			if deps.Queue != nil {
				collector.Register("queue", deps.Queue)
			}
			if deps.Watcher != nil {
				collector.Register("watcher", deps.Watcher)
			}
			if g, ok := deps.Graph.(*graph.FalkorDBGraph); ok && g != nil {
				collector.Register("graph", g)
			}
			return collector, nil
		},
		FatalChan: func(component any) <-chan error {
			if c, ok := component.(*metrics.Collector); ok {
				return c.Errors()
			}
			return nil
		},
	})
}
