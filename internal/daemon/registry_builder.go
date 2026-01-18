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

// registerComponentDefinitions populates the component registry with definitions.
func (o *Orchestrator) registerComponentDefinitions(cfg *config.Config) {
	r := NewComponentRegistry()

	// Event bus
	r.Register(ComponentDefinition{
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

	// Registry (SQLite)
	r.Register(ComponentDefinition{
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
	r.Register(ComponentDefinition{
		Name:          "graph",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
		Dependencies:  nil,
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
			g := graph.NewFalkorDBGraph(
				graph.WithConfig(graphCfg),
				graph.WithLogger(slog.Default().With("component", "graph")),
			)
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
	r.Register(ComponentDefinition{
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

	r.Register(ComponentDefinition{
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
	r.Register(ComponentDefinition{
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

	r.Register(ComponentDefinition{
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
	r.Register(ComponentDefinition{
		Name:          "queue",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityFatal,
		RestartPolicy: RestartOnFailure,
		Dependencies:  []string{"bus"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			workerCount := runtime.NumCPU()
			if workerCount < 2 {
				workerCount = 2
			}
			if workerCount > 8 {
				workerCount = 8
			}
			q := analysis.NewQueue(deps.Bus,
				analysis.WithWorkerCount(workerCount),
				analysis.WithQueueCapacity(1000),
				analysis.WithLogger(slog.Default().With("component", "analysis-queue")),
			)
			slog.Info("analysis queue initialized", "workers", workerCount)
			return q, nil
		},
		FatalChan: func(component any) <-chan error {
			if q, ok := component.(*analysis.Queue); ok {
				return q.Errors()
			}
			return nil
		},
	})

	// Walker
	r.Register(ComponentDefinition{
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
	// These wrap existing capabilities; we register names for health/events.
	r.Register(ComponentDefinition{
		Name:          "job.initial_walk",
		Kind:          ComponentKindJob,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"walker", "queue", "cleaner"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			return &logicalJob{name: "job.initial_walk", mode: "full"}, nil
		},
	})
	r.Register(ComponentDefinition{
		Name:          "job.rebuild_full",
		Kind:          ComponentKindJob,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"walker", "queue", "cleaner"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			return &logicalJob{name: "job.rebuild_full", mode: "full"}, nil
		},
	})
	r.Register(ComponentDefinition{
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
	r.Register(ComponentDefinition{
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
	r.Register(ComponentDefinition{
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
	r.Register(ComponentDefinition{
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
	r.Register(ComponentDefinition{
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

	o.components = r
}

// buildComponents constructs components in dependency order.
func (o *Orchestrator) buildComponents(ctx context.Context, cfg *config.Config) error {
	order, err := o.components.TopologicalOrder()
	if err != nil {
		return err
	}

	bag := ComponentContext{}

	for _, name := range order {
		def := o.components.defs[name]
		obj, err := def.Build(ctx, bag)
		if err != nil {
			if def.Criticality == CriticalityFatal {
				return fmt.Errorf("failed to build component %s; %w", name, err)
			}
			slog.Warn("component build failed; continuing in degraded mode", "component", name, "error", err)
			continue
		}
		if obj == nil {
			continue
		}

		switch c := obj.(type) {
		case *events.EventBus:
			o.bus = c
			bag.Bus = c
		case registry.Registry:
			o.registry = c
			bag.Registry = c
		case graph.Graph:
			o.graph = c
			bag.Graph = c
		case *cache.SemanticCache:
			o.semanticCache = c
			bag.Caches.Semantic = c
		case *cache.EmbeddingsCache:
			o.embeddingsCache = c
			bag.Caches.Embeddings = c
		case providers.SemanticProvider:
			o.semanticProvider = c
			bag.Providers.Semantic = c
		case providers.EmbeddingsProvider:
			o.embedProvider = c
			bag.Providers.Embed = c
		case *analysis.Queue:
			o.queue = c
			bag.Queue = c
		case walker.Walker:
			o.walker = c
			bag.Walker = c
		case watcher.Watcher:
			o.watcher = c
			bag.Watcher = c
		case *cleaner.Cleaner:
			o.cleaner = c
			bag.Cleaner = c
		case *mcp.Server:
			o.mcpServer = c
			bag.MCP = c
		case *metrics.Collector:
			o.metricsCollector = c
			bag.MetricsCollector = c
		default:
			slog.Warn("unknown component type returned; ignoring", "component", name)
		}
	}

	// Log provider status after build
	logProviderStatus(o.semanticProvider, o.embedProvider)

	return nil
}
