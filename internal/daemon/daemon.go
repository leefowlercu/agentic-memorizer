package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/index"
	"github.com/leefowlercu/agentic-memorizer/internal/metadata"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/internal/version"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
	"github.com/leefowlercu/agentic-memorizer/internal/watcher"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Daemon manages background index computation
type Daemon struct {
	cfg               *config.Config
	indexManager      *index.Manager
	cacheManager      *cache.Manager
	metadataExtractor *metadata.Extractor
	semanticAnalyzer  *semantic.Analyzer
	fileWatcher       *watcher.Watcher
	healthMetrics     *HealthMetrics
	logger            *slog.Logger
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	pidFile           string
}

// New creates a new daemon instance
func New(cfg *config.Config, logger *slog.Logger) (*Daemon, error) {
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

	ctx, cancel := context.WithCancel(context.Background())

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

	return &Daemon{
		cfg:               cfg,
		indexManager:      indexManager,
		cacheManager:      cacheManager,
		metadataExtractor: metadataExtractor,
		semanticAnalyzer:  semanticAnalyzer,
		fileWatcher:       fileWatcher,
		healthMetrics:     healthMetrics,
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
		pidFile:           pidFile,
	}, nil
}

// Start starts the daemon
func (d *Daemon) Start() error {
	// Check if already running
	if err := checkPIDFile(d.pidFile); err != nil {
		return err
	}

	// Write PID file
	if err := writePIDFile(d.pidFile); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	d.logger.Info("daemon starting", "version", version.GetVersion())

	// Setup signal handling
	setupSignalHandler(d)

	// Try to load existing index for crash recovery
	existingIndex, err := d.indexManager.LoadComputed()
	if err == nil {
		d.logger.Info("loaded existing index from disk", "files", existingIndex.Index.Stats.TotalFiles)
		// Set the loaded index as current
		d.indexManager.SetIndex(existingIndex.Index, existingIndex.Metadata)
	} else {
		d.logger.Info("no existing index found, will perform full build")
	}

	// Perform initial full build
	d.logger.Info("performing initial index build")
	if err := d.rebuildIndex(); err != nil {
		d.logger.Error("initial build failed", "error", err)
		// If we have an existing index loaded, continue with it
		if existingIndex != nil {
			d.logger.Warn("continuing with existing index due to build failure")
		} else {
			return fmt.Errorf("initial build failed and no existing index available: %w", err)
		}
	}

	// Start file watcher
	if err := d.fileWatcher.Start(); err != nil {
		d.logger.Error("failed to start file watcher", "error", err)
		return fmt.Errorf("failed to start file watcher: %w", err)
	}
	d.healthMetrics.SetWatcherActive(true)

	// Start event processor
	d.wg.Add(1)
	go d.processWatcherEvents()

	// Start periodic rebuild ticker
	d.wg.Add(1)
	go d.periodicRebuild()

	// Start health check server if configured
	if d.cfg.Daemon.HealthCheckPort > 0 {
		if err := StartHealthCheckServer(d.cfg.Daemon.HealthCheckPort, d.healthMetrics); err != nil {
			d.logger.Warn("failed to start health check server", "error", err)
		} else {
			d.logger.Info("health check server started", "port", d.cfg.Daemon.HealthCheckPort)
		}
	}

	d.logger.Info("daemon started successfully")

	// Wait for context cancellation
	<-d.ctx.Done()

	d.logger.Info("daemon shutting down")

	// Stop file watcher
	d.fileWatcher.Stop()

	d.wg.Wait()

	// Remove PID file
	if err := removePIDFile(d.pidFile); err != nil {
		d.logger.Error("failed to remove PID file", "error", err)
	}

	d.logger.Info("daemon stopped")
	return nil
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	d.logger.Info("stop requested")
	d.cancel()
}

// periodicRebuild performs periodic full rebuilds
func (d *Daemon) periodicRebuild() {
	defer d.wg.Done()

	interval := time.Duration(d.cfg.Daemon.FullRebuildIntervalMinutes) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.logger.Info("performing periodic rebuild")
			if err := d.rebuildIndex(); err != nil {
				d.logger.Error("periodic rebuild failed", "error", err)
			}
		case <-d.ctx.Done():
			return
		}
	}
}

// rebuildIndex performs a full index rebuild using worker pool
func (d *Daemon) rebuildIndex() error {
	startTime := time.Now()

	skipDirs := []string{".cache", ".git"}
	skipFiles := d.cfg.Analysis.SkipFiles
	if len(skipFiles) == 0 {
		skipFiles = []string{"agentic-memorizer"}
	}

	// Create worker pool
	pool := NewWorkerPool(
		d.cfg.Daemon.Workers,
		d.cfg.Daemon.RateLimitPerMin,
		d.metadataExtractor,
		d.semanticAnalyzer,
		d.cacheManager,
		d.logger,
		d.ctx,
	)

	pool.Start()
	defer pool.Stop()

	// Collect all files to process
	var jobs []Job
	err := walker.Walk(d.cfg.MemoryRoot, skipDirs, skipFiles, func(path string, info os.FileInfo) error {
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

	d.logger.Info("processing files with worker pool", "files", len(jobs), "workers", d.cfg.Daemon.Workers)

	// Submit all jobs
	pool.SubmitBatch(jobs)

	// Collect results
	entries := make([]types.IndexEntry, 0, len(jobs))
	for i := 0; i < len(jobs); i++ {
		select {
		case result := <-pool.Results():
			// Set relative path
			relPath, _ := walker.GetRelPath(d.cfg.MemoryRoot, result.Entry.Metadata.Path)
			result.Entry.Metadata.RelPath = relPath
			entries = append(entries, result.Entry)

		case <-d.ctx.Done():
			return fmt.Errorf("rebuild cancelled")
		}
	}

	// Build index
	idx := &types.Index{
		Generated: time.Now(),
		Root:      d.cfg.MemoryRoot,
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

	// Record successful build metrics
	d.healthMetrics.RecordBuild(idx.Stats.TotalFiles, idx.Stats.AnalyzedFiles, idx.Stats.CachedFiles, idx.Stats.ErrorFiles, true)
	d.healthMetrics.SetIndexFileCount(idx.Stats.TotalFiles)

	d.logger.Info("index rebuilt successfully",
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
	d.logger.Info("manual rebuild requested")
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
	relPath, err := walker.GetRelPath(d.cfg.MemoryRoot, event.Path)
	if err != nil {
		d.logger.Warn("failed to get relative path", "path", event.Path, "error", err)
		return
	}

	switch event.Type {
	case watcher.EventCreate, watcher.EventModify:
		d.logger.Info("file changed", "path", relPath, "type", event.Type)

		// Check if file still exists (it might have been deleted quickly)
		info, err := os.Stat(event.Path)
		if err != nil {
			if os.IsNotExist(err) {
				// File was deleted, handle as delete
				d.handleFileDelete(event.Path, relPath)
				return
			}
			d.logger.Warn("failed to stat file", "path", event.Path, "error", err)
			return
		}

		// Skip directories
		if info.IsDir() {
			return
		}

		// Extract metadata
		fileMetadata, err := d.metadataExtractor.Extract(event.Path, info)
		if err != nil {
			d.logger.Warn("failed to extract metadata", "path", event.Path, "error", err)
			return
		}

		fileMetadata.RelPath = relPath

		// Hash file
		fileHash, err := cache.HashFile(event.Path)
		if err != nil {
			d.logger.Warn("failed to hash file", "path", event.Path, "error", err)
			fileHash = ""
		}
		fileMetadata.Hash = fileHash

		// Analyze semantically if enabled
		var semanticAnalysis *types.SemanticAnalysis
		if d.semanticAnalyzer != nil && fileHash != "" {
			// Check cache first
			cached, err := d.cacheManager.Get(fileHash)
			if err == nil && cached != nil && !d.cacheManager.IsStale(cached, fileHash) {
				semanticAnalysis = cached.Semantic
				d.logger.Debug("using cached analysis", "path", relPath)
			} else {
				// Analyze file
				d.logger.Debug("analyzing file", "path", relPath)
				analysis, err := d.semanticAnalyzer.Analyze(fileMetadata)
				if err != nil {
					d.logger.Warn("analysis failed", "path", event.Path, "error", err)
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
						d.logger.Warn("failed to cache analysis", "path", event.Path, "error", err)
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
			d.logger.Error("failed to update index", "path", event.Path, "error", err)
			return
		}

		// Write updated index
		if err := d.indexManager.WriteAtomic(version.GetVersion()); err != nil {
			d.logger.Error("failed to write index", "error", err)
		} else {
			d.logger.Debug("index updated", "path", relPath)
		}

	case watcher.EventDelete:
		d.handleFileDelete(event.Path, relPath)
	}
}

// handleFileDelete handles file deletion
func (d *Daemon) handleFileDelete(path string, relPath string) {
	d.logger.Info("file deleted", "path", relPath)

	if err := d.indexManager.RemoveFile(path); err != nil {
		d.logger.Error("failed to remove from index", "path", path, "error", err)
		return
	}

	// Write updated index
	if err := d.indexManager.WriteAtomic(version.GetVersion()); err != nil {
		d.logger.Error("failed to write index", "error", err)
	} else {
		d.logger.Debug("index updated after deletion", "path", relPath)
	}
}
