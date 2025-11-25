package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/index"
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
	httpServer *HTTPServer

	// SSE notification hub
	sseHub *SSEHub

	// Reload signaling channels
	rebuildIntervalCh chan time.Duration // Signal rebuild interval change

	// Components
	indexManager      *index.Manager
	cacheManager      *cache.Manager
	metadataExtractor *metadata.Extractor
	fileWatcher       *watcher.Watcher
	healthMetrics     *HealthMetrics

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
	indexPath, err := config.GetIndexPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get index path: %w", err)
	}
	indexManager := index.NewManager(indexPath)

	cacheManager, err := cache.NewManager(cfg.Analysis.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache manager: %w", err)
	}

	metadataExtractor := metadata.NewExtractor()

	var semanticAnalyzer *semantic.Analyzer
	if cfg.Analysis.Enable {
		client := semantic.NewClient(
			cfg.Claude.APIKey,
			cfg.Claude.Model,
			cfg.Claude.MaxTokens,
			cfg.Claude.TimeoutSeconds,
		)
		semanticAnalyzer = semantic.NewAnalyzer(
			client,
			cfg.Claude.EnableVision,
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

	// Create health metrics tracker
	healthMetrics := NewHealthMetrics()

	// Create SSE notification hub
	sseHub := NewSSEHub(logger)

	// Create unified HTTP server
	httpServer := NewHTTPServer(sseHub, healthMetrics, logger)

	// Create context after all fallible operations to avoid leaks on early returns
	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		cfg:               cfg,
		indexManager:      indexManager,
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

	// Try to load existing index for crash recovery
	existingIndex, err := d.indexManager.LoadComputed()
	if err == nil {
		logger.Info("loaded existing index from disk", "files", existingIndex.Index.Stats.TotalFiles)
		// Set the loaded index as current
		d.indexManager.SetIndex(existingIndex.Index, existingIndex.Metadata)
	} else {
		logger.Info("no existing index found, will perform full build")
	}

	// Perform initial full build
	logger.Info("performing initial index build")
	if err := d.rebuildIndex(); err != nil {
		logger.Error("initial build failed", "error", err)
		// If we have an existing index loaded, continue with it
		if existingIndex != nil {
			logger.Warn("continuing with existing index due to build failure")
		} else {
			return fmt.Errorf("initial build failed and no existing index available: %w", err)
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

	// Notify systemd we're ready (if running under systemd)
	if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		logger.Warn("failed to notify systemd", "error", err)
	} else if supported {
		logger.Info("notified systemd of readiness")
	}

	logger.Info("daemon started successfully")

	// Wait for context cancellation
	<-d.ctx.Done()

	logger.Info("daemon shutting down")

	// Shutdown order:
	// 1. HTTP Server (stop health check and SSE endpoints)
	// 2. File Watcher (stop file system monitoring)
	// 3. Wait for goroutines (let workers finish)
	// 4. PID file cleanup (final cleanup)

	// Stop HTTP server
	if d.httpServer != nil {
		if err := d.httpServer.Stop(); err != nil {
			logger.Warn("error stopping HTTP server", "error", err)
		}
	}

	// Stop file watcher
	d.fileWatcher.Stop()

	d.wg.Wait()

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
	startTime := time.Now()

	cfg := d.GetConfig()
	logger := d.GetLogger()
	analyzer := d.GetSemanticAnalyzer()

	skipDirs := []string{".cache", ".git"}
	skipFiles := cfg.Analysis.SkipFiles
	if len(skipFiles) == 0 {
		skipFiles = []string{"agentic-memorizer"}
	}

	// Create worker pool
	pool := NewWorkerPool(
		cfg.Daemon.Workers,
		cfg.Daemon.RateLimitPerMin,
		d.metadataExtractor,
		analyzer,
		d.cacheManager,
		logger,
		d.ctx,
	)

	pool.Start()
	defer pool.Stop()

	// Collect all files to process
	var jobs []Job
	err := walker.Walk(cfg.MemoryRoot, skipDirs, skipFiles, func(path string, info os.FileInfo) error {
		job := Job{
			Path:     path,
			Info:     info,
			Priority: CalculatePriority(info),
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

	// Collect results
	entries := make([]types.IndexEntry, 0, len(jobs))
	for i := 0; i < len(jobs); i++ {
		select {
		case result := <-pool.Results():
			// Set relative path
			relPath, _ := walker.GetRelPath(cfg.MemoryRoot, result.Entry.Metadata.Path)
			result.Entry.Metadata.RelPath = relPath
			entries = append(entries, result.Entry)

		case <-d.ctx.Done():
			return fmt.Errorf("rebuild cancelled")
		}
	}

	// Build index
	idx := &types.Index{
		Generated: time.Now(),
		Root:      cfg.MemoryRoot,
		Entries:   entries,
		Stats:     types.IndexStats{},
	}

	// Calculate stats
	poolStats := pool.GetStats()
	idx.Stats.TotalFiles = len(entries)
	idx.Stats.CachedFiles = poolStats.CacheHits
	idx.Stats.AnalyzedFiles = poolStats.APICalls

	for _, entry := range entries {
		idx.Stats.TotalSize += entry.Metadata.Size
		if entry.Error != nil {
			idx.Stats.ErrorFiles++
		}
	}

	// Set index in manager
	buildDuration := time.Since(startTime)
	metadata := index.BuildMetadata{
		BuildDurationMs: int(buildDuration.Milliseconds()),
		FilesProcessed:  idx.Stats.TotalFiles,
		CacheHits:       idx.Stats.CachedFiles,
		APICalls:        idx.Stats.AnalyzedFiles,
	}

	d.indexManager.SetIndex(idx, metadata)

	// Write to disk atomically
	if err := d.indexManager.WriteAtomic(version.GetVersion()); err != nil {
		d.healthMetrics.RecordBuild(idx.Stats.TotalFiles, idx.Stats.AnalyzedFiles, idx.Stats.CachedFiles, idx.Stats.ErrorFiles, false)
		d.healthMetrics.RecordError()
		return fmt.Errorf("failed to write index: %w", err)
	}

	// Broadcast SSE notification
	if d.sseHub != nil {
		d.sseHub.BroadcastIndexUpdate()
	}

	// Record successful build metrics
	d.healthMetrics.RecordBuild(idx.Stats.TotalFiles, idx.Stats.AnalyzedFiles, idx.Stats.CachedFiles, idx.Stats.ErrorFiles, true)
	d.healthMetrics.SetIndexFileCount(idx.Stats.TotalFiles)

	logger.Info("index rebuilt successfully",
		"duration_ms", buildDuration.Milliseconds(),
		"files", idx.Stats.TotalFiles,
		"analyzed", idx.Stats.AnalyzedFiles,
		"cached", idx.Stats.CachedFiles,
		"errors", idx.Stats.ErrorFiles,
	)

	return nil
}

// Rebuild forces an immediate index rebuild
func (d *Daemon) Rebuild() error {
	d.GetLogger().Info("manual rebuild requested")
	return d.rebuildIndex()
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
				logger.Debug("using cached analysis", "path", relPath)
			} else {
				// Analyze file
				logger.Debug("analyzing file", "path", relPath)
				analysis, err := analyzer.Analyze(fileMetadata)
				if err != nil {
					logger.Warn("analysis failed", "path", event.Path, "error", err)
				} else {
					semanticAnalysis = analysis

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

		// Update index
		entry := types.IndexEntry{
			Metadata: *fileMetadata,
			Semantic: semanticAnalysis,
		}

		if err := d.indexManager.UpdateSingle(entry); err != nil {
			logger.Error("failed to update index", "path", event.Path, "error", err)
			return
		}

		// Write updated index
		if err := d.indexManager.WriteAtomic(version.GetVersion()); err != nil {
			logger.Error("failed to write index", "error", err)
		} else {
			// Broadcast SSE notification
			if d.sseHub != nil {
				d.sseHub.BroadcastIndexUpdate()
			}
			logger.Debug("index updated", "path", relPath)
		}

	case watcher.EventDelete:
		d.handleFileDelete(event.Path, relPath)
	}
}

// handleFileDelete handles file deletion
func (d *Daemon) handleFileDelete(path string, relPath string) {
	logger := d.GetLogger()
	logger.Info("file deleted", "path", relPath)

	if err := d.indexManager.RemoveFile(path); err != nil {
		logger.Error("failed to remove from index", "path", path, "error", err)
		return
	}

	// Write updated index
	if err := d.indexManager.WriteAtomic(version.GetVersion()); err != nil {
		logger.Error("failed to write index", "error", err)
	} else {
		// Broadcast SSE notification
		if d.sseHub != nil {
			d.sseHub.BroadcastIndexUpdate()
		}
		logger.Debug("index updated after deletion", "path", relPath)
	}
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
	if !cfg.Analysis.Enable {
		d.SetSemanticAnalyzer(nil)
		return nil
	}

	client := semantic.NewClient(
		cfg.Claude.APIKey,
		cfg.Claude.Model,
		cfg.Claude.MaxTokens,
		cfg.Claude.TimeoutSeconds,
	)
	analyzer := semantic.NewAnalyzer(
		client,
		cfg.Claude.EnableVision,
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
