package daemon

import (
	"context"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/metadata"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"golang.org/x/time/rate"
)

// Job represents a file processing job
type Job struct {
	Path     string
	Info     os.FileInfo
	Priority int // Higher priority = process first
}

// JobResult represents the result of processing a job
type JobResult struct {
	Entry types.IndexEntry
	Error error
}

// WorkerPool manages parallel file processing with rate limiting
type WorkerPool struct {
	workers           int
	jobQueue          chan Job
	resultQueue       chan JobResult
	rateLimiter       *rate.Limiter
	metadataExtractor *metadata.Extractor
	semanticAnalyzer  *semantic.Analyzer
	cacheManager      *cache.Manager
	logger            *slog.Logger
	ctx               context.Context
	wg                sync.WaitGroup
	stats             PoolStats
	statsMu           sync.Mutex
}

// PoolStats tracks worker pool statistics
type PoolStats struct {
	JobsProcessed int
	JobsQueued    int
	CacheHits     int
	APICalls      int
	Errors        int
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	workers int,
	rateLimitPerMin int,
	metadataExtractor *metadata.Extractor,
	semanticAnalyzer *semantic.Analyzer,
	cacheManager *cache.Manager,
	logger *slog.Logger,
	ctx context.Context,
) *WorkerPool {
	// Convert per-minute rate to per-second rate
	// Use burst of 3 to allow some flexibility
	perSecond := float64(rateLimitPerMin) / 60.0
	limiter := rate.NewLimiter(rate.Limit(perSecond), 3)

	return &WorkerPool{
		workers:           workers,
		jobQueue:          make(chan Job, 100),
		resultQueue:       make(chan JobResult, 100),
		rateLimiter:       limiter,
		metadataExtractor: metadataExtractor,
		semanticAnalyzer:  semanticAnalyzer,
		cacheManager:      cacheManager,
		logger:            logger,
		ctx:               ctx,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	wp.logger.Info("starting worker pool", "workers", wp.workers)

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop stops the worker pool gracefully
func (wp *WorkerPool) Stop() {
	wp.logger.Info("stopping worker pool")
	close(wp.jobQueue)
	wp.wg.Wait()
	close(wp.resultQueue)
}

// Submit submits a job to the pool
func (wp *WorkerPool) Submit(job Job) {
	wp.statsMu.Lock()
	wp.stats.JobsQueued++
	wp.statsMu.Unlock()

	select {
	case wp.jobQueue <- job:
	case <-wp.ctx.Done():
	}
}

// SubmitBatch submits multiple jobs, sorted by priority
func (wp *WorkerPool) SubmitBatch(jobs []Job) {
	// Sort by priority (highest first)
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].Priority > jobs[j].Priority
	})

	for _, job := range jobs {
		wp.Submit(job)
	}
}

// Results returns the result channel
func (wp *WorkerPool) Results() <-chan JobResult {
	return wp.resultQueue
}

// GetStats returns current pool statistics
func (wp *WorkerPool) GetStats() PoolStats {
	wp.statsMu.Lock()
	defer wp.statsMu.Unlock()
	return wp.stats
}

// worker processes jobs from the queue
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	wp.logger.Debug("worker started", "worker_id", id)

	for {
		select {
		case job, ok := <-wp.jobQueue:
			if !ok {
				wp.logger.Debug("worker stopping", "worker_id", id)
				return
			}

			result := wp.processJob(job)

			wp.statsMu.Lock()
			wp.stats.JobsProcessed++
			if result.Error != nil {
				wp.stats.Errors++
			}
			wp.statsMu.Unlock()

			select {
			case wp.resultQueue <- result:
			case <-wp.ctx.Done():
				return
			}

		case <-wp.ctx.Done():
			wp.logger.Debug("worker cancelled", "worker_id", id)
			return
		}
	}
}

// processJob processes a single job
func (wp *WorkerPool) processJob(job Job) JobResult {
	// Extract metadata
	fileMetadata, err := wp.metadataExtractor.Extract(job.Path, job.Info)
	if err != nil {
		wp.logger.Warn("failed to extract metadata", "path", job.Path, "error", err)
		errStr := err.Error()
		return JobResult{
			Entry: types.IndexEntry{
				Metadata: *fileMetadata,
				Error:    &errStr,
			},
			Error: err,
		}
	}

	// Hash file
	fileHash, err := cache.HashFile(job.Path)
	if err != nil {
		wp.logger.Warn("failed to hash file", "path", job.Path, "error", err)
		fileHash = ""
	}
	fileMetadata.Hash = fileHash

	// Analyze semantically if enabled
	var semanticAnalysis *types.SemanticAnalysis
	if wp.semanticAnalyzer != nil && fileHash != "" {
		// Check cache first
		cached, err := wp.cacheManager.Get(fileHash)
		if err == nil && cached != nil && !wp.cacheManager.IsStale(cached, fileHash) {
			semanticAnalysis = cached.Semantic
			wp.logger.Debug("cache hit", "path", job.Path)

			wp.statsMu.Lock()
			wp.stats.CacheHits++
			wp.statsMu.Unlock()
		} else {
			// Wait for rate limiter before making API call
			if err := wp.rateLimiter.Wait(wp.ctx); err != nil {
				wp.logger.Warn("rate limiter cancelled", "path", job.Path, "error", err)
				return JobResult{
					Entry: types.IndexEntry{
						Metadata: *fileMetadata,
					},
					Error: err,
				}
			}

			wp.logger.Debug("analyzing file", "path", job.Path)

			analysis, err := wp.semanticAnalyzer.Analyze(fileMetadata)
			if err != nil {
				wp.logger.Warn("analysis failed", "path", job.Path, "error", err)
			} else {
				semanticAnalysis = analysis

				wp.statsMu.Lock()
				wp.stats.APICalls++
				wp.statsMu.Unlock()

				// Cache result
				cachedAnalysis := &types.CachedAnalysis{
					FilePath:   job.Path,
					FileHash:   fileHash,
					AnalyzedAt: time.Now(),
					Metadata:   *fileMetadata,
					Semantic:   semanticAnalysis,
				}
				if err := wp.cacheManager.Set(cachedAnalysis); err != nil {
					wp.logger.Warn("failed to cache analysis", "path", job.Path, "error", err)
				}
			}
		}
	}

	return JobResult{
		Entry: types.IndexEntry{
			Metadata: *fileMetadata,
			Semantic: semanticAnalysis,
		},
		Error: nil,
	}
}

// CalculatePriority calculates job priority based on file modification time
// More recently modified files get higher priority
func CalculatePriority(info os.FileInfo) int {
	age := time.Since(info.ModTime())

	// Files modified in last hour: priority 100
	if age < time.Hour {
		return 100
	}

	// Files modified in last day: priority 50
	if age < 24*time.Hour {
		return 50
	}

	// Files modified in last week: priority 25
	if age < 7*24*time.Hour {
		return 25
	}

	// Older files: priority 10
	return 10
}
