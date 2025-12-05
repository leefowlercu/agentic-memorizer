package worker

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/embeddings"
	"github.com/leefowlercu/agentic-memorizer/internal/metadata"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"golang.org/x/time/rate"
)

// PoolStats tracks worker pool statistics
type PoolStats struct {
	JobsProcessed      int
	JobsQueued         int
	CacheHits          int
	APICalls           int
	EmbeddingAPICalls  int
	EmbeddingCacheHits int
	Errors             int
}

// Pool manages parallel file processing with rate limiting
type Pool struct {
	workers           int
	jobQueue          chan Job
	resultQueue       chan JobResult
	rateLimiter       *rate.Limiter
	embeddingLimiter  *rate.Limiter // Separate rate limiter for embeddings API
	metadataExtractor *metadata.Extractor
	semanticAnalyzer  *semantic.Analyzer
	embeddingProvider embeddings.Provider
	embeddingCache    *embeddings.Cache
	cacheManager      *cache.Manager
	logger            *slog.Logger
	ctx               context.Context
	wg                sync.WaitGroup
	stats             PoolStats
	statsMu           sync.Mutex
}

// NewPool creates a new worker pool
func NewPool(
	workers int,
	rateLimitPerMin int,
	metadataExtractor *metadata.Extractor,
	semanticAnalyzer *semantic.Analyzer,
	embeddingProvider embeddings.Provider,
	embeddingCache *embeddings.Cache,
	cacheManager *cache.Manager,
	logger *slog.Logger,
	ctx context.Context,
) *Pool {
	// Convert per-minute rate to per-second for token bucket algorithm.
	// Burst of 3 allows small bursts while maintaining average rate limit.
	// Example: 20/min = 0.33/sec, handles Claude API quota gracefully.
	perSecond := float64(rateLimitPerMin) / 60.0
	limiter := rate.NewLimiter(rate.Limit(perSecond), 3)

	// Embedding API rate limiter (OpenAI allows 3000 RPM for embeddings).
	// Conservative 500 RPM default leaves headroom for other API usage.
	// Burst of 10 handles initial batch processing without hitting limits.
	embeddingLimiter := rate.NewLimiter(rate.Limit(500.0/60.0), 10)

	return &Pool{
		workers:           workers,
		jobQueue:          make(chan Job, 100),
		resultQueue:       make(chan JobResult, 100),
		rateLimiter:       limiter,
		embeddingLimiter:  embeddingLimiter,
		metadataExtractor: metadataExtractor,
		semanticAnalyzer:  semanticAnalyzer,
		embeddingProvider: embeddingProvider,
		embeddingCache:    embeddingCache,
		cacheManager:      cacheManager,
		logger:            logger,
		ctx:               ctx,
	}
}

// Start starts the worker pool
func (p *Pool) Start() {
	p.logger.Info("starting worker pool", "workers", p.workers)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the worker pool gracefully
func (p *Pool) Stop() {
	p.logger.Info("stopping worker pool")
	close(p.jobQueue)
	p.wg.Wait()
	close(p.resultQueue)
}

// Submit submits a job to the pool
func (p *Pool) Submit(job Job) {
	p.statsMu.Lock()
	p.stats.JobsQueued++
	p.statsMu.Unlock()

	select {
	case p.jobQueue <- job:
	case <-p.ctx.Done():
	}
}

// SubmitBatch submits multiple jobs, sorted by priority
func (p *Pool) SubmitBatch(jobs []Job) {
	// Sort by priority (highest first)
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].Priority > jobs[j].Priority
	})

	for _, job := range jobs {
		p.Submit(job)
	}
}

// Results returns the result channel
func (p *Pool) Results() <-chan JobResult {
	return p.resultQueue
}

// GetStats returns current pool statistics
func (p *Pool) GetStats() PoolStats {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	return p.stats
}

// worker processes jobs from the queue
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	p.logger.Debug("worker started", "worker_id", id)

	for {
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				p.logger.Debug("worker stopping", "worker_id", id)
				return
			}

			result := p.processJob(job)

			p.statsMu.Lock()
			p.stats.JobsProcessed++
			if result.Error != nil {
				p.stats.Errors++
			}
			p.statsMu.Unlock()

			select {
			case p.resultQueue <- result:
			case <-p.ctx.Done():
				return
			}

		case <-p.ctx.Done():
			p.logger.Debug("worker cancelled", "worker_id", id)
			return
		}
	}
}

// processJob processes a single job
func (p *Pool) processJob(job Job) JobResult {
	// Extract metadata
	fileMetadata, err := p.metadataExtractor.Extract(job.Path, job.Info)
	if err != nil {
		p.logger.Warn("failed to extract metadata", "path", job.Path, "error", err)
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
		p.logger.Warn("failed to hash file", "path", job.Path, "error", err)
		fileHash = ""
	}
	fileMetadata.Hash = fileHash

	// Analyze semantically if enabled
	var semanticAnalysis *types.SemanticAnalysis
	if p.semanticAnalyzer != nil && fileHash != "" {
		// Check cache first
		cached, err := p.cacheManager.Get(fileHash)
		if err == nil && cached != nil && !p.cacheManager.IsStale(cached, fileHash) {
			semanticAnalysis = cached.Semantic
			p.logger.Debug("cache hit",
				"path", job.Path,
				"version", cache.VersionString(cached),
			)

			p.statsMu.Lock()
			p.stats.CacheHits++
			p.statsMu.Unlock()
		} else {
			// Log reason for cache miss with version info
			if cached != nil && cache.IsStaleVersion(cached) {
				p.logger.Debug("cache version stale, re-analyzing",
					"path", job.Path,
					"cached_version", cache.VersionString(cached),
					"current_version", cache.CacheVersion(),
				)
			} else if cached != nil {
				p.logger.Debug("cache content stale, re-analyzing", "path", job.Path)
			}

			// Wait for rate limiter before making API call
			if err := p.rateLimiter.Wait(p.ctx); err != nil {
				p.logger.Warn("rate limiter cancelled", "path", job.Path, "error", err)
				return JobResult{
					Entry: types.IndexEntry{
						Metadata: *fileMetadata,
					},
					Error: err,
				}
			}

			p.logger.Debug("analyzing file", "path", job.Path)

			analysis, err := p.semanticAnalyzer.Analyze(fileMetadata)
			if err != nil {
				p.logger.Warn("analysis failed", "path", job.Path, "error", err)
			} else {
				semanticAnalysis = analysis

				p.statsMu.Lock()
				p.stats.APICalls++
				p.statsMu.Unlock()

				// Cache result (version fields set by cacheManager.Set)
				cachedAnalysis := &types.CachedAnalysis{
					FilePath:   job.Path,
					FileHash:   fileHash,
					AnalyzedAt: time.Now(),
					Metadata:   *fileMetadata,
					Semantic:   semanticAnalysis,
				}
				if err := p.cacheManager.Set(cachedAnalysis); err != nil {
					p.logger.Warn("failed to cache analysis", "path", job.Path, "error", err)
				}
			}
		}
	}

	// Generate embedding if provider is available and we have a summary
	var embedding []float32
	if p.embeddingProvider != nil && semanticAnalysis != nil && semanticAnalysis.Summary != "" {
		embedding = p.generateEmbedding(job.Path, fileHash, semanticAnalysis.Summary)
	}

	return JobResult{
		Entry: types.IndexEntry{
			Metadata: *fileMetadata,
			Semantic: semanticAnalysis,
		},
		Embedding: embedding,
		Error:     nil,
	}
}

// generateEmbedding generates an embedding for the given text, using cache when available
func (p *Pool) generateEmbedding(path, hash, text string) []float32 {
	// Check embedding cache first
	if p.embeddingCache != nil && hash != "" {
		if cached, found := p.embeddingCache.Get(hash); found {
			p.logger.Debug("embedding cache hit", "path", path)
			p.statsMu.Lock()
			p.stats.EmbeddingCacheHits++
			p.statsMu.Unlock()
			return cached
		}
	}

	// Wait for embedding rate limiter
	if err := p.embeddingLimiter.Wait(p.ctx); err != nil {
		p.logger.Warn("embedding rate limiter cancelled", "path", path, "error", err)
		return nil
	}

	// Generate embedding
	embedding, err := p.embeddingProvider.Embed(p.ctx, text)
	if err != nil {
		p.logger.Warn("failed to generate embedding", "path", path, "error", err)
		return nil
	}

	p.statsMu.Lock()
	p.stats.EmbeddingAPICalls++
	p.statsMu.Unlock()

	// Cache the embedding
	if p.embeddingCache != nil && hash != "" {
		if err := p.embeddingCache.Set(hash, embedding); err != nil {
			p.logger.Warn("failed to cache embedding", "path", path, "error", err)
		}
	}

	p.logger.Debug("generated embedding", "path", path, "dimensions", len(embedding))
	return embedding
}
