package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon/api"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon/worker"
	"github.com/leefowlercu/agentic-memorizer/internal/embeddings"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/metadata"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/internal/version"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
	"github.com/leefowlercu/agentic-memorizer/internal/watcher"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Daemon manages background index computation
type Daemon struct {
	// Configuration (thread-safe access)
	cfgMu sync.RWMutex
	cfg   *config.Config

	// Semantic analyzer (atomic replacement)
	semanticAnalyzer atomic.Value // *semantic.Analyzer

	// Logger (thread-safe replacement)
	loggerMu  sync.RWMutex
	logger    *slog.Logger
	logWriter *lumberjack.Logger // Reuse for log level changes

	// HTTP server (unified health + SSE endpoints)
	httpServer *api.HTTPServer

	// SSE notification hub
	sseHub *api.SSEHub

	// Reload signaling channels
	rebuildIntervalCh chan time.Duration // Signal rebuild interval change

	// Components
	graphManager      *graph.Manager // FalkorDB-based storage (required)
	cacheManager      *cache.Manager
	metadataExtractor *metadata.Extractor
	fileWatcher       *watcher.Watcher
	healthMetrics     *HealthMetrics

	// Rebuild state
	rebuilding atomic.Bool

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	pidFile string
}

// GetConfig returns the current configuration (thread-safe)
func (d *Daemon) GetConfig() *config.Config {
	d.cfgMu.RLock()
	defer d.cfgMu.RUnlock()
	return d.cfg
}

// SetConfig sets the configuration atomically (thread-safe)
func (d *Daemon) SetConfig(cfg *config.Config) {
	d.cfgMu.Lock()
	defer d.cfgMu.Unlock()
	d.cfg = cfg
}

// GetSemanticAnalyzer returns the current semantic analyzer (lock-free)
func (d *Daemon) GetSemanticAnalyzer() *semantic.Analyzer {
	val := d.semanticAnalyzer.Load()
	if val == nil {
		return nil
	}
	return val.(*semantic.Analyzer)
}

// SetSemanticAnalyzer sets the semantic analyzer atomically (lock-free)
func (d *Daemon) SetSemanticAnalyzer(a *semantic.Analyzer) {
	d.semanticAnalyzer.Store(a)
}

// GetLogger returns the current logger (thread-safe)
func (d *Daemon) GetLogger() *slog.Logger {
	d.loggerMu.RLock()
	defer d.loggerMu.RUnlock()
	return d.logger
}

// SetLogger sets the logger atomically (thread-safe)
func (d *Daemon) SetLogger(l *slog.Logger) {
	d.loggerMu.Lock()
	defer d.loggerMu.Unlock()
	d.logger = l
}

// New creates a new daemon instance
func New(cfg *config.Config, logger *slog.Logger, logWriter *lumberjack.Logger) (*Daemon, error) {
	cacheManager, err := cache.NewManager(cfg.Analysis.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache manager: %w", err)
	}

	metadataExtractor := metadata.NewExtractor()

	var semanticAnalyzer *semantic.Analyzer
	if cfg.Analysis.Enabled {
		client := semantic.NewClient(
			cfg.Claude.APIKey,
			cfg.Claude.Model,
			cfg.Claude.MaxTokens,
			config.ClaudeTimeoutSeconds, // See internal/config/constants.go
		)
		semanticAnalyzer = semantic.NewAnalyzer(
			client,
			config.ClaudeEnableVision, // See internal/config/constants.go
			cfg.Analysis.MaxFileSize,
		)
	}

	pidFile, err := config.GetPIDPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get PID path: %w", err)
	}

	// Create file watcher
	skipDirs := []string{".cache", ".git"}
	skipFiles := cfg.Analysis.SkipFiles
	if len(skipFiles) == 0 {
		skipFiles = []string{"agentic-memorizer"}
	}

	fileWatcher, err := watcher.New(
		cfg.MemoryRoot,
		skipDirs,
		skipFiles,
		cfg.Daemon.DebounceMs,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Initialize graph manager (required for daemon operation)
	graphConfig := graph.ManagerConfig{
		Client: graph.ClientConfig{
			Host:     cfg.Graph.Host,
			Port:     cfg.Graph.Port,
			Database: config.GraphDatabase, // Hardcoded convention
			Password: cfg.Graph.Password,
		},
		Schema:     graph.DefaultSchemaConfig(),
		MemoryRoot: cfg.MemoryRoot,
	}

	graphManager := graph.NewManager(graphConfig, logger)
	if err := graphManager.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize graph; %w", err)
	}

	logger.Info("FalkorDB graph manager initialized",
		"host", cfg.Graph.Host,
		"port", cfg.Graph.Port,
		"database", config.GraphDatabase,
	)

	// Create health metrics tracker
	healthMetrics := NewHealthMetrics()

	// Create SSE notification hub
	sseHub := api.NewSSEHub(logger)

	// Create unified HTTP server with graph manager
	httpServer := api.NewHTTPServer(sseHub, healthMetrics, graphManager, cfg.MemoryRoot, logger)

	// Set index provider on SSE hub for including index data in events
	sseHub.SetIndexProvider(httpServer)

	// Create context after all fallible operations to avoid leaks on early returns
	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		cfg:               cfg,
		graphManager:      graphManager,
		cacheManager:      cacheManager,
		metadataExtractor: metadataExtractor,
		fileWatcher:       fileWatcher,
		healthMetrics:     healthMetrics,
		sseHub:            sseHub,
		httpServer:        httpServer,
		logger:            logger,
		logWriter:         logWriter,
		ctx:               ctx,
		cancel:            cancel,
		pidFile:           pidFile,
		rebuildIntervalCh: make(chan time.Duration, 1),
	}

	// Set semantic analyzer atomically
	d.SetSemanticAnalyzer(semanticAnalyzer)

	// Set rebuild handler on HTTP server (Daemon implements api.RebuildHandler)
	httpServer.SetRebuildHandler(d)

	return d, nil
}

// Start starts the daemon
func (d *Daemon) Start() error {
	logger := d.GetLogger()

	// Check if already running
	if err := checkPIDFile(d.pidFile); err != nil {
		return err
	}

	// Write PID file
	if err := writePIDFile(d.pidFile); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	logger.Info("daemon starting", "version", version.GetVersion())

	// Setup signal handling
	setupSignalHandler(d)

	// Graph initialization with graceful degradation:
	// - If existing data found: log count, attempt rebuild
	// - If rebuild fails BUT existing data exists: continue with stale data (degraded mode)
	// - If rebuild fails AND no existing data: fail startup (can't operate without index)
	// This allows daemon to start even if initial rebuild hits transient errors.

	// Check if graph has existing data
	stats, err := d.graphManager.GetStats(d.ctx)
	if err == nil && stats.TotalFiles > 0 {
		logger.Info("found existing data in graph", "files", stats.TotalFiles)
	} else {
		logger.Info("no existing graph data, will perform full build")
	}

	// Perform initial full build
	logger.Info("performing initial index build")
	if err := d.rebuildIndex(); err != nil {
		logger.Error("initial build failed", "error", err)
		// If graph has existing data, we can continue
		if stats != nil && stats.TotalFiles > 0 {
			logger.Warn("continuing with existing graph data due to build failure")
		} else {
			return fmt.Errorf("initial build failed and no existing data available; %w", err)
		}
	}

	// Start file watcher
	if err := d.fileWatcher.Start(); err != nil {
		logger.Error("failed to start file watcher", "error", err)
		return fmt.Errorf("failed to start file watcher: %w", err)
	}
	d.healthMetrics.SetWatcherActive(true)

	// Start event processor
	d.wg.Add(1)
	go d.processWatcherEvents()

	// Start periodic rebuild ticker
	d.wg.Add(1)
	go d.periodicRebuild()

	// Start HTTP server if configured (provides health check and SSE endpoints)
	cfg := d.GetConfig()
	if cfg.Daemon.HTTPPort > 0 {
		if err := d.httpServer.Start(cfg.Daemon.HTTPPort); err != nil {
			logger.Warn("failed to start HTTP server", "error", err)
		} else {
			logger.Info("HTTP server started", "port", cfg.Daemon.HTTPPort)
		}
	}

	// Notify systemd we're ready (Type=notify integration).
	// SdNotify sends readiness signal after health server starts.
	// Allows systemd to know daemon is fully operational before marking 'active'.
	// Gracefully no-ops if not running under systemd.
	if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		logger.Warn("failed to notify systemd", "error", err)
	} else if supported {
		logger.Info("notified systemd of readiness")
	}

	logger.Info("daemon started successfully")

	// Wait for context cancellation
	<-d.ctx.Done()

	logger.Info("daemon shutting down")

	// Shutdown order principle: Stop inbound requests (HTTP), stop event sources
	// (watcher), drain workers (wg.Wait), close external connections (graph),
	// cleanup state (PID). Each step must complete before proceeding.

	// Stop HTTP server
	if d.httpServer != nil {
		if err := d.httpServer.Stop(); err != nil {
			logger.Warn("error stopping HTTP server", "error", err)
		}
	}

	// Stop file watcher
	d.fileWatcher.Stop()

	d.wg.Wait()

	// Close graph manager
	if err := d.graphManager.Close(); err != nil {
		logger.Warn("error closing graph manager", "error", err)
	}

	// Remove PID file
	if err := removePIDFile(d.pidFile); err != nil {
		logger.Error("failed to remove PID file", "error", err)
	}

	logger.Info("daemon stopped")
	return nil
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	logger := d.GetLogger()
	logger.Info("stop requested")
	d.cancel()
}

// periodicRebuild performs periodic full rebuilds
func (d *Daemon) periodicRebuild() {
	defer d.wg.Done()

	cfg := d.GetConfig()
	logger := d.GetLogger()
	interval := time.Duration(cfg.Daemon.FullRebuildIntervalMinutes) * time.Minute

	if interval <= 0 {
		logger.Info("periodic rebuilds disabled")
		// Wait for signal to enable
		select {
		case newInterval := <-d.rebuildIntervalCh:
			if newInterval <= 0 {
				logger.Info("periodic rebuilds remain disabled")
				return
			}
			interval = newInterval
			logger.Info("periodic rebuilds enabled", "interval_minutes", interval.Minutes())
		case <-d.ctx.Done():
			return
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("periodic rebuild scheduler started", "interval_minutes", interval.Minutes())

	for {
		select {
		case <-ticker.C:
			logger.Info("triggering periodic rebuild")
			if err := d.rebuildIndex(); err != nil {
				d.GetLogger().Error("periodic rebuild failed", "error", err)
			}

		case newInterval := <-d.rebuildIntervalCh:
			ticker.Stop()
			if newInterval <= 0 {
				logger.Info("periodic rebuilds disabled, exiting scheduler")
				return
			}
			ticker = time.NewTicker(newInterval)
			logger.Info("rebuild interval changed", "new_interval_minutes", newInterval.Minutes())

		case <-d.ctx.Done():
			logger.Info("periodic rebuild scheduler stopping")
			return
		}
	}
}

// rebuildIndex performs a full index rebuild using worker pool
func (d *Daemon) rebuildIndex() error {
	// Set rebuilding flag (used by API to report status)
	d.rebuilding.Store(true)
	defer d.rebuilding.Store(false)

	startTime := time.Now()

	cfg := d.GetConfig()
	logger := d.GetLogger()
	analyzer := d.GetSemanticAnalyzer()

	skipDirs := []string{".cache", ".git"}
	skipFiles := cfg.Analysis.SkipFiles
	if len(skipFiles) == 0 {
		skipFiles = []string{"agentic-memorizer"}
	}

	// Create embedding provider and cache if embeddings are enabled
	var embeddingProvider embeddings.Provider
	var embeddingCache *embeddings.Cache
	if cfg.Embeddings.Enabled {
		// API key already resolved from env in GetConfig()
		apiKey := cfg.Embeddings.APIKey

		if apiKey != "" {
			embConfig := embeddings.OpenAIConfig{
				APIKey:     apiKey,
				Model:      config.EmbeddingsModel,      // Hardcoded convention
				Dimensions: config.EmbeddingsDimensions, // Hardcoded convention
			}
			var err error
			embeddingProvider, err = embeddings.NewOpenAIProvider(embConfig, logger)
			if err != nil {
				logger.Warn("failed to create embedding provider", "error", err)
			} else {
				logger.Info("embedding provider initialized",
					"model", config.EmbeddingsModel,
					"dimensions", config.EmbeddingsDimensions,
				)
			}

			// Create embedding cache
			embCacheDir := filepath.Join(cfg.Analysis.CacheDir, "embeddings")
			embeddingCache, err = embeddings.NewCache(embCacheDir, logger)
			if err != nil {
				logger.Warn("failed to create embedding cache", "error", err)
			}
		} else {
			logger.Warn("embeddings enabled but no API key found",
				"api_key_env", config.EmbeddingsAPIKeyEnv,
			)
		}
	}

	// Create worker pool
	pool := worker.NewPool(
		cfg.Daemon.Workers,
		cfg.Daemon.RateLimitPerMin,
		d.metadataExtractor,
		analyzer,
		embeddingProvider,
		embeddingCache,
		d.cacheManager,
		logger,
		d.ctx,
	)

	pool.Start()
	defer pool.Stop()

	// Collect all files to process
	var jobs []worker.Job
	err := walker.Walk(cfg.MemoryRoot, skipDirs, skipFiles, func(path string, info os.FileInfo) error {
		job := worker.Job{
			Path:     path,
			Info:     info,
			Priority: worker.CalculatePriority(info),
		}
		jobs = append(jobs, job)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	logger.Info("processing files with worker pool", "files", len(jobs), "workers", cfg.Daemon.Workers)

	// Submit all jobs
	pool.SubmitBatch(jobs)

	// Track stats for health metrics
	var totalFiles, errorFiles int
	var totalSize int64

	// Collect results and write to graph
	var embeddingsGenerated int
	for i := 0; i < len(jobs); i++ {
		select {
		case result := <-pool.Results():
			// Set relative path
			relPath, _ := walker.GetRelPath(cfg.MemoryRoot, result.Entry.Metadata.Path)
			result.Entry.Metadata.RelPath = relPath

			// Write to graph with embedding if available
			graphEntry := types.IndexEntry{
				Metadata: result.Entry.Metadata,
				Semantic: result.Entry.Semantic,
			}

			if len(result.Embedding) > 0 {
				if _, err := d.graphManager.UpdateSingleWithEmbedding(d.ctx, graphEntry, graph.UpdateInfo{}, result.Embedding); err != nil {
					logger.Warn("failed to update graph with embedding", "path", relPath, "error", err)
				}
				embeddingsGenerated++
			} else {
				if _, err := d.graphManager.UpdateSingle(d.ctx, graphEntry, graph.UpdateInfo{}); err != nil {
					logger.Warn("failed to update graph", "path", relPath, "error", err)
				}
			}

			// Track stats
			totalFiles++
			totalSize += result.Entry.Metadata.Size
			if result.Entry.Error != nil {
				errorFiles++
			}

		case <-d.ctx.Done():
			return fmt.Errorf("rebuild cancelled")
		}
	}

	// Get pool stats for cache/API tracking
	poolStats := pool.GetStats()
	buildDuration := time.Since(startTime)

	// Broadcast SSE notification
	if d.sseHub != nil {
		d.sseHub.BroadcastIndexUpdate()
	}

	// Record successful build metrics
	d.healthMetrics.RecordBuild(totalFiles, poolStats.APICalls, poolStats.CacheHits, errorFiles, true)
	d.healthMetrics.SetIndexFileCount(totalFiles)

	logger.Info("index rebuilt successfully",
		"duration_ms", buildDuration.Milliseconds(),
		"files", totalFiles,
		"analyzed", poolStats.APICalls,
		"cached", poolStats.CacheHits,
		"embeddings", embeddingsGenerated,
		"embedding_api_calls", poolStats.EmbeddingAPICalls,
		"embedding_cache_hits", poolStats.EmbeddingCacheHits,
		"errors", errorFiles,
	)

	return nil
}

// Rebuild forces an immediate index rebuild
func (d *Daemon) Rebuild() error {
	d.GetLogger().Info("manual rebuild requested")
	return d.rebuildIndex()
}

// ClearGraph clears all data from the graph (implements api.RebuildHandler)
func (d *Daemon) ClearGraph() error {
	d.GetLogger().Info("clearing graph")
	return d.graphManager.ClearGraph(d.ctx)
}

// IsRebuilding returns true if a rebuild is in progress (implements api.RebuildHandler)
func (d *Daemon) IsRebuilding() bool {
	return d.rebuilding.Load()
}

// processWatcherEvents processes file system events from the watcher
func (d *Daemon) processWatcherEvents() {
	defer d.wg.Done()

	for {
		select {
		case event, ok := <-d.fileWatcher.Events():
			if !ok {
				return
			}
			d.handleFileEvent(event)

		case <-d.ctx.Done():
			return
		}
	}
}

// handleFileEvent handles a single file system event
func (d *Daemon) handleFileEvent(event watcher.Event) {
	cfg := d.GetConfig()
	logger := d.GetLogger()
	analyzer := d.GetSemanticAnalyzer()

	relPath, err := walker.GetRelPath(cfg.MemoryRoot, event.Path)
	if err != nil {
		logger.Warn("failed to get relative path", "path", event.Path, "error", err)
		return
	}

	switch event.Type {
	case watcher.EventCreate, watcher.EventModify:
		logger.Info("file changed", "path", relPath, "type", event.Type)

		// Track what happened during processing
		var wasAnalyzed, wasCached, hadError bool

		// Check if file still exists (it might have been deleted quickly)
		info, err := os.Stat(event.Path)
		if err != nil {
			if os.IsNotExist(err) {
				// File was deleted, handle as delete
				d.handleFileDelete(event.Path, relPath)
				return
			}
			logger.Warn("failed to stat file", "path", event.Path, "error", err)
			return
		}

		// Skip directories
		if info.IsDir() {
			return
		}

		// Extract metadata
		fileMetadata, err := d.metadataExtractor.Extract(event.Path, info)
		if err != nil {
			logger.Warn("failed to extract metadata", "path", event.Path, "error", err)
			return
		}

		fileMetadata.RelPath = relPath

		// Hash file
		fileHash, err := cache.HashFile(event.Path)
		if err != nil {
			logger.Warn("failed to hash file", "path", event.Path, "error", err)
			fileHash = ""
		}
		fileMetadata.Hash = fileHash

		// Analyze semantically if enabled
		var semanticAnalysis *types.SemanticAnalysis
		if analyzer != nil && fileHash != "" {
			// Check cache first
			cached, err := d.cacheManager.Get(fileHash)
			if err == nil && cached != nil && !d.cacheManager.IsStale(cached, fileHash) {
				semanticAnalysis = cached.Semantic
				wasCached = true
				d.healthMetrics.RecordCacheHit()
				logger.Debug("using cached analysis", "path", relPath)
			} else {
				// Analyze file
				logger.Debug("analyzing file", "path", relPath)
				analysis, err := analyzer.Analyze(fileMetadata)
				if err != nil {
					logger.Warn("analysis failed", "path", event.Path, "error", err)
					hadError = true
					d.healthMetrics.RecordError()
				} else {
					semanticAnalysis = analysis
					wasAnalyzed = true
					d.healthMetrics.RecordAPICall()

					// Cache result
					cachedAnalysis := &types.CachedAnalysis{
						FilePath:   event.Path,
						FileHash:   fileHash,
						AnalyzedAt: time.Now(),
						Metadata:   *fileMetadata,
						Semantic:   semanticAnalysis,
					}
					if err := d.cacheManager.Set(cachedAnalysis); err != nil {
						logger.Warn("failed to cache analysis", "path", event.Path, "error", err)
					}
				}
			}
		}

		// Update graph with entry
		entry := types.IndexEntry{
			Metadata: *fileMetadata,
			Semantic: semanticAnalysis,
		}

		graphInfo := graph.UpdateInfo{
			WasAnalyzed: wasAnalyzed,
			WasCached:   wasCached,
			HadError:    hadError,
		}

		result, err := d.graphManager.UpdateSingle(d.ctx, entry, graphInfo)
		if err != nil {
			logger.Error("failed to update graph", "path", event.Path, "error", err)
			d.healthMetrics.RecordError()
			return
		}

		// Update health metrics based on result
		d.healthMetrics.RecordFileProcessed()
		if result.Added {
			d.healthMetrics.IncrementIndexFileCount()
		}

		// Broadcast SSE notification
		if d.sseHub != nil {
			d.sseHub.BroadcastIndexUpdate()
		}
		logger.Debug("graph updated", "path", relPath)

	case watcher.EventDelete:
		d.handleFileDelete(event.Path, relPath)
	}
}

// handleFileDelete handles file deletion
func (d *Daemon) handleFileDelete(path string, relPath string) {
	logger := d.GetLogger()
	logger.Info("file deleted", "path", relPath)

	// Remove from graph
	if err := d.graphManager.RemoveFile(d.ctx, path); err != nil {
		logger.Error("failed to remove from graph", "path", path, "error", err)
		return
	}

	d.healthMetrics.DecrementIndexFileCount()

	// Broadcast SSE notification
	if d.sseHub != nil {
		d.sseHub.BroadcastIndexUpdate()
	}
	logger.Debug("graph updated after deletion", "path", relPath)
}

// ReloadConfig reloads configuration and applies changes
func (d *Daemon) ReloadConfig() error {
	logger := d.GetLogger()
	logger.Info("starting configuration reload")

	// Load new configuration
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to reload config; %w", err)
	}

	newCfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get config; %w", err)
	}

	// Validate configuration
	if err := config.ValidateConfig(newCfg); err != nil {
		return fmt.Errorf("config validation failed; %w", err)
	}

	// Validate reload compatibility
	oldCfg := d.GetConfig()
	if err := config.ValidateReload(oldCfg, newCfg); err != nil {
		return fmt.Errorf("reload validation failed; %w", err)
	}

	// Detect changes
	changes := d.detectChanges(oldCfg, newCfg)

	// Apply configuration atomically
	d.SetConfig(newCfg)
	logger.Info("configuration swapped atomically")

	// Apply component-specific changes
	d.applyComponentChanges(changes, newCfg)

	logger.Info("configuration reload complete",
		"changes_applied", countChanges(changes))

	return nil
}

// detectChanges compares old and new configs to detect changes
func (d *Daemon) detectChanges(oldCfg, newCfg *config.Config) map[string]bool {
	return map[string]bool{
		"claude":           !reflect.DeepEqual(newCfg.Claude, oldCfg.Claude),
		"log_level":        newCfg.Daemon.LogLevel != oldCfg.Daemon.LogLevel,
		"debounce":         newCfg.Daemon.DebounceMs != oldCfg.Daemon.DebounceMs,
		"rebuild_interval": newCfg.Daemon.FullRebuildIntervalMinutes != oldCfg.Daemon.FullRebuildIntervalMinutes,
		"http_port":        newCfg.Daemon.HTTPPort != oldCfg.Daemon.HTTPPort,
		"workers":          newCfg.Daemon.Workers != oldCfg.Daemon.Workers,
		"rate_limit":       newCfg.Daemon.RateLimitPerMin != oldCfg.Daemon.RateLimitPerMin,
		"skip_patterns": !reflect.DeepEqual(newCfg.Analysis.SkipFiles, oldCfg.Analysis.SkipFiles) ||
			!reflect.DeepEqual(newCfg.Analysis.SkipExtensions, oldCfg.Analysis.SkipExtensions),
	}
}

// applyComponentChanges applies component-specific configuration changes
func (d *Daemon) applyComponentChanges(changes map[string]bool, newCfg *config.Config) {
	logger := d.GetLogger()

	if changes["claude"] {
		if err := d.updateSemanticAnalyzer(newCfg); err != nil {
			logger.Warn("failed to update semantic analyzer", "error", err)
		} else {
			logger.Info("semantic analyzer updated")
		}
	}

	if changes["log_level"] {
		if err := d.updateLogLevel(newCfg); err != nil {
			logger.Warn("failed to update log level", "error", err)
		} else {
			logger.Info("log level updated", "level", newCfg.Daemon.LogLevel)
		}
	}

	if changes["debounce"] {
		d.fileWatcher.UpdateDebounceInterval(newCfg.Daemon.DebounceMs)
		logger.Info("debounce interval updated", "ms", newCfg.Daemon.DebounceMs)
	}

	if changes["rebuild_interval"] {
		interval := time.Duration(newCfg.Daemon.FullRebuildIntervalMinutes) * time.Minute
		select {
		case d.rebuildIntervalCh <- interval:
			logger.Info("rebuild interval updated", "minutes", newCfg.Daemon.FullRebuildIntervalMinutes)
		default:
			logger.Warn("failed to signal rebuild interval change")
		}
	}

	if changes["http_port"] {
		if err := d.httpServer.Start(newCfg.Daemon.HTTPPort); err != nil {
			logger.Warn("failed to restart HTTP server", "error", err)
		} else {
			if newCfg.Daemon.HTTPPort == 0 {
				logger.Info("HTTP server disabled")
			} else {
				logger.Info("HTTP server restarted", "port", newCfg.Daemon.HTTPPort)
			}
		}
	}

	if changes["workers"] || changes["rate_limit"] {
		logger.Info("worker pool settings will apply on next rebuild",
			"workers", newCfg.Daemon.Workers,
			"rate_limit", newCfg.Daemon.RateLimitPerMin)
	}

	if changes["skip_patterns"] {
		logger.Info("skip patterns will apply on next file processing")
	}
}

// updateSemanticAnalyzer creates and sets a new semantic analyzer
func (d *Daemon) updateSemanticAnalyzer(cfg *config.Config) error {
	if !cfg.Analysis.Enabled {
		d.SetSemanticAnalyzer(nil)
		return nil
	}

	client := semantic.NewClient(
		cfg.Claude.APIKey,
		cfg.Claude.Model,
		cfg.Claude.MaxTokens,
		config.ClaudeTimeoutSeconds, // Hardcoded convention
	)
	analyzer := semantic.NewAnalyzer(
		client,
		config.ClaudeEnableVision, // Hardcoded convention
		cfg.Analysis.MaxFileSize,
	)

	d.SetSemanticAnalyzer(analyzer)
	return nil
}

// updateLogLevel creates a new logger with the specified log level
func (d *Daemon) updateLogLevel(cfg *config.Config) error {
	var logLevel slog.Level
	switch strings.ToLower(cfg.Daemon.LogLevel) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		return fmt.Errorf("invalid log level; %s", cfg.Daemon.LogLevel)
	}

	handler := slog.NewJSONHandler(d.logWriter, &slog.HandlerOptions{
		Level: logLevel,
	})

	newLogger := slog.New(handler)
	d.SetLogger(newLogger)

	return nil
}

// countChanges counts the number of true values in the changes map
func countChanges(changes map[string]bool) int {
	count := 0
	for _, changed := range changes {
		if changed {
			count++
		}
	}
	return count
}
