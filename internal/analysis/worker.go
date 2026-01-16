package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/code"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/code/languages"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// WorkItemType indicates the type of work item.
type WorkItemType int

const (
	WorkItemNew WorkItemType = iota
	WorkItemChanged
	WorkItemReanalyze
)

// WorkItem represents a file to be analyzed.
type WorkItem struct {
	FilePath  string
	FileSize  int64
	ModTime   time.Time
	EventType WorkItemType
	Retries   int
}

// AnalysisResult contains the complete analysis of a file.
type AnalysisResult struct {
	// Metadata
	FilePath     string
	FileSize     int64
	ModTime      time.Time
	ContentHash  string
	MetadataHash string
	MIMEType     string
	Language     string

	// Semantic analysis
	Summary    string
	Tags       []string
	Topics     []string
	Entities   []Entity
	References []Reference
	Complexity int
	Keywords   []string

	// Embeddings (file-level average)
	Embeddings []float32

	// Per-chunk data for graph persistence
	Chunks []AnalyzedChunk

	// Processing info
	ChunkerUsed     string
	ChunksProcessed int
	ProcessingTime  time.Duration
	AnalyzedAt      time.Time
}

// AnalyzedChunk contains data for a single analyzed chunk including embedding.
type AnalyzedChunk struct {
	Index       int
	Content     string
	ContentHash string
	StartOffset int
	EndOffset   int
	ChunkType   string
	Embedding   []float32

	// Metadata contains typed metadata from chunking (Code, Document, etc.)
	Metadata *chunkers.ChunkMetadata

	// TokenCount is the estimated token count.
	TokenCount int

	// Summary is the per-chunk semantic summary.
	Summary string
}

// Entity represents an extracted entity.
type Entity struct {
	Name string
	Type string
}

// Reference represents an extracted reference.
type Reference struct {
	Type   string // "file", "url", "symbol"
	Target string
}

// Worker processes work items from the queue.
type Worker struct {
	id       int
	queue    *Queue
	logger   *slog.Logger
	stopChan chan struct{}
	stopOnce sync.Once

	// Providers (injected or looked up)
	semanticProvider   providers.SemanticProvider
	embeddingsProvider providers.EmbeddingsProvider
	chunkerRegistry    *chunkers.Registry

	// Graph client for persisting results
	graph graph.Graph

	// Registry for tracking file state
	registry registry.Registry

	// Analysis version for tracking schema changes
	analysisVersion string

	// Caches for avoiding redundant API calls
	semanticCache   *cache.SemanticCache
	embeddingsCache *cache.EmbeddingsCache
}

// NewWorker creates a new analysis worker.
func NewWorker(id int, queue *Queue) *Worker {
	return &Worker{
		id:              id,
		queue:           queue,
		logger:          queue.logger.With("worker_id", id),
		stopChan:        make(chan struct{}),
		chunkerRegistry: defaultChunkerRegistry(),
	}
}

// defaultChunkerRegistry creates a chunker registry with all standard chunkers
// including the tree-sitter multi-language chunker.
func defaultChunkerRegistry() *chunkers.Registry {
	r := chunkers.DefaultRegistry()

	// Create tree-sitter chunker with all language strategies
	tsChunker := code.NewTreeSitterChunker()
	tsChunker.RegisterStrategy(languages.NewGoStrategy())
	tsChunker.RegisterStrategy(languages.NewPythonStrategy())
	tsChunker.RegisterStrategy(languages.NewJavaScriptStrategy())
	tsChunker.RegisterStrategy(languages.NewTypeScriptStrategy())
	tsChunker.RegisterStrategy(languages.NewJavaStrategy())
	tsChunker.RegisterStrategy(languages.NewRustStrategy())
	tsChunker.RegisterStrategy(languages.NewCStrategy())
	tsChunker.RegisterStrategy(languages.NewCPPStrategy())

	r.Register(tsChunker)

	return r
}

// Run starts the worker processing loop.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Debug("worker started")
	w.queue.activeWorkers.Add(1)
	defer w.queue.activeWorkers.Add(-1)

	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("worker stopping due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Debug("worker stopping due to stop signal")
			return
		case item, ok := <-w.queue.workChan:
			if !ok {
				w.logger.Debug("worker stopping due to closed channel")
				return
			}
			w.processItem(ctx, item)
		}
	}
}

// Stop signals the worker to stop.
func (w *Worker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopChan)
	})
}

// processItem handles a single work item with retry logic.
func (w *Worker) processItem(ctx context.Context, item WorkItem) {
	start := time.Now()

	result, err := w.analyze(ctx, item)
	if err != nil {
		// Retry logic with exponential backoff
		if item.Retries < w.queue.maxRetries {
			item.Retries++
			delay := w.calculateBackoff(item.Retries)

			w.logger.Warn("analysis failed; scheduling retry",
				"path", item.FilePath,
				"error", err,
				"retry", item.Retries,
				"delay", delay)

			time.AfterFunc(delay, func() {
				if err := w.queue.Enqueue(item); err != nil {
					w.logger.Error("failed to re-queue item", "path", item.FilePath, "error", err)
				}
			})
			return
		}

		// Max retries exceeded
		w.logger.Error("analysis failed permanently",
			"path", item.FilePath,
			"error", err,
			"retries", item.Retries)

		w.queue.recordAnalysisFailure()
		w.queue.publishAnalysisFailed(item.FilePath, err)
		return
	}

	duration := time.Since(start)
	result.ProcessingTime = duration

	// Persist to graph (if configured)
	if err := w.persistToGraph(ctx, result); err != nil {
		// Treat graph write failure like analysis failure - retry
		if item.Retries < w.queue.maxRetries {
			item.Retries++
			delay := w.calculateBackoff(item.Retries)

			w.logger.Warn("graph persistence failed; scheduling retry",
				"path", item.FilePath,
				"error", err,
				"retry", item.Retries,
				"delay", delay)

			time.AfterFunc(delay, func() {
				if err := w.queue.Enqueue(item); err != nil {
					w.logger.Error("failed to re-queue item", "path", item.FilePath, "error", err)
				}
			})
			return
		}

		// Max retries exceeded for graph persistence
		w.logger.Error("graph persistence failed permanently",
			"path", item.FilePath,
			"error", err,
			"retries", item.Retries)

		w.queue.recordPersistenceFailure()
		w.queue.publishGraphPersistenceFailed(item.FilePath, err, item.Retries)
		return
	}

	w.queue.recordSuccess(duration)
	w.queue.publishAnalysisComplete(item.FilePath, result)

	w.logger.Info("analysis complete",
		"path", item.FilePath,
		"chunks", result.ChunksProcessed,
		"duration", duration)
}

// calculateBackoff returns the delay for a retry attempt.
func (w *Worker) calculateBackoff(retries int) time.Duration {
	// Exponential backoff: base * 2^(retries-1)
	delay := float64(w.queue.retryDelay) * math.Pow(2, float64(retries-1))
	return time.Duration(delay)
}

// analyze performs the full analysis pipeline.
func (w *Worker) analyze(ctx context.Context, item WorkItem) (*AnalysisResult, error) {
	// Maximum file size: 100MB (prevents OOM on large files)
	const maxFileSize = 100 * 1024 * 1024

	// Check file size before reading
	info, err := os.Stat(item.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file; %w", err)
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxFileSize)
	}

	// Read file content
	content, err := os.ReadFile(item.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file; %w", err)
	}

	// Determine MIME type and language
	mimeType := detectMIMEType(item.FilePath, content)
	language := detectLanguage(item.FilePath)

	// Check degradation mode
	stats := w.queue.Stats()
	mode := stats.DegradationMode

	result := &AnalysisResult{
		FilePath:    item.FilePath,
		FileSize:    item.FileSize,
		ModTime:     item.ModTime,
		MIMEType:    mimeType,
		Language:    language,
		AnalyzedAt:  time.Now(),
		ContentHash: computeContentHash(content),
	}

	// Step 1: Metadata analysis (always performed)
	result.MetadataHash = computeMetadataHash(item.FilePath, item.FileSize, item.ModTime)

	// Check if content has changed (requires clearing previous analysis state)
	// Note: This check-then-act is subject to race conditions if multiple workers
	// process the same file concurrently. This is rare in practice but possible
	// during full rebuilds. The registry should ideally support atomic compare-and-swap.
	if w.registry != nil {
		existingState, err := w.registry.GetFileState(ctx, item.FilePath)
		if err == nil && existingState.ContentHash != result.ContentHash {
			// Content changed - clear previous analysis state
			w.logger.Debug("content changed; clearing analysis state",
				"path", item.FilePath,
				"old_hash", existingState.ContentHash[:8],
				"new_hash", result.ContentHash[:8])
			if clearErr := w.registry.ClearAnalysisState(ctx, item.FilePath); clearErr != nil {
				w.logger.Warn("failed to clear analysis state", "path", item.FilePath, "error", clearErr)
			}
		}

		// Update metadata state
		if err := w.registry.UpdateMetadataState(ctx, item.FilePath, result.ContentHash, result.MetadataHash, item.FileSize, item.ModTime); err != nil {
			w.logger.Warn("failed to update metadata state", "path", item.FilePath, "error", err)
		}
	}

	if mode == DegradationMetadata {
		// Metadata only mode
		return result, nil
	}

	// Step 2: Chunk content
	chunkResult, err := w.chunkContent(ctx, content, mimeType, language)
	if err != nil {
		return nil, fmt.Errorf("chunking failed; %w", err)
	}

	result.ChunkerUsed = chunkResult.ChunkerUsed
	result.ChunksProcessed = chunkResult.TotalChunks

	// Step 3: Semantic analysis (if provider available)
	var semanticErr error
	var chunkSummaries []string // Per-chunk summaries for graph persistence
	if w.semanticProvider != nil && w.semanticProvider.Available() {
		var semanticResult *SemanticResult
		var cacheHit bool

		// Check semantic cache first
		if w.semanticCache != nil {
			cachedResult, err := w.semanticCache.Get(result.ContentHash)
			if err == nil {
				// Cache hit - convert providers.SemanticResult to SemanticResult
				semanticResult = w.convertCachedSemantic(cachedResult)
				cacheHit = true
				w.logger.Debug("semantic cache hit", "path", item.FilePath)
				// Note: Per-chunk summaries are not cached, so chunkSummaries stays nil
			} else if !errors.Is(err, cache.ErrCacheMiss) {
				w.logger.Warn("semantic cache read error", "path", item.FilePath, "error", err)
			}
		}

		// Cache miss - perform analysis
		if !cacheHit {
			semanticResult, chunkSummaries, semanticErr = w.analyzeSemantics(ctx, chunkResult.Chunks)
			if semanticErr != nil {
				w.logger.Warn("semantic analysis failed",
					"path", item.FilePath,
					"error", semanticErr)
				w.queue.publishSemanticAnalysisFailed(item.FilePath, semanticErr)
				// Continue without semantic analysis (soft failure)
			} else if w.semanticCache != nil {
				// Store in cache
				providerResult := w.convertToProviderSemantic(semanticResult)
				if cacheErr := w.semanticCache.Set(result.ContentHash, providerResult); cacheErr != nil {
					w.logger.Warn("semantic cache write error", "path", item.FilePath, "error", cacheErr)
				}
			}
		}

		if semanticResult != nil {
			result.Summary = semanticResult.Summary
			result.Tags = semanticResult.Tags
			result.Topics = semanticResult.Topics
			result.Entities = semanticResult.Entities
			result.References = semanticResult.References
			result.Complexity = semanticResult.Complexity
			result.Keywords = semanticResult.Keywords
		}

		// Update semantic state in registry
		if w.registry != nil {
			version := w.analysisVersion
			if version == "" {
				version = "1.0.0"
			}
			if err := w.registry.UpdateSemanticState(ctx, item.FilePath, version, semanticErr); err != nil {
				w.logger.Warn("failed to update semantic state", "path", item.FilePath, "error", err)
			}
		}
	}

	if mode == DegradationNoEmbed {
		// Skip embeddings
		return result, nil
	}

	// Step 4: Generate embeddings (if provider available)
	var embeddingsErr error
	if w.embeddingsProvider != nil && w.embeddingsProvider.Available() {
		var embeddings []float32
		var chunkData []AnalyzedChunk
		embeddings, chunkData, embeddingsErr = w.generateEmbeddings(ctx, chunkResult.Chunks, chunkSummaries)
		if embeddingsErr != nil {
			w.logger.Warn("embeddings generation failed",
				"path", item.FilePath,
				"error", embeddingsErr)
			w.queue.publishEmbeddingsGenerationFailed(item.FilePath, embeddingsErr)
			// Continue without embeddings (soft failure)
		} else {
			result.Embeddings = embeddings
			result.Chunks = chunkData
		}

		// Update embeddings state in registry
		if w.registry != nil {
			if err := w.registry.UpdateEmbeddingsState(ctx, item.FilePath, embeddingsErr); err != nil {
				w.logger.Warn("failed to update embeddings state", "path", item.FilePath, "error", err)
			}
		}
	}

	return result, nil
}

// chunkContent splits content using the appropriate chunker.
func (w *Worker) chunkContent(ctx context.Context, content []byte, mimeType, language string) (*chunkers.ChunkResult, error) {
	opts := chunkers.DefaultChunkOptions()
	opts.MIMEType = mimeType
	opts.Language = language

	return w.chunkerRegistry.Chunk(ctx, content, opts)
}

// SemanticResult contains merged semantic analysis results.
type SemanticResult struct {
	Summary    string
	Tags       []string
	Topics     []string
	Entities   []Entity
	References []Reference
	Complexity int
	Keywords   []string
}

// analyzeSemantics performs semantic analysis on chunks and merges results.
// Returns the merged result, per-chunk summaries (indexed by chunk index), and any error.
func (w *Worker) analyzeSemantics(ctx context.Context, chunks []chunkers.Chunk) (*SemanticResult, []string, error) {
	if len(chunks) == 0 {
		return &SemanticResult{}, nil, nil
	}

	// Initialize per-chunk summaries slice (indexed by chunk.Index)
	chunkSummaries := make([]string, len(chunks))

	// For single chunk, analyze directly
	if len(chunks) == 1 {
		result, err := w.analyzeChunk(ctx, chunks[0])
		if err != nil {
			return nil, nil, err
		}
		chunkSummaries[chunks[0].Index] = result.Summary
		return result, chunkSummaries, nil
	}

	// For multiple chunks, analyze each and merge
	var chunkResults []*SemanticResult
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		result, err := w.analyzeChunk(ctx, chunk)
		if err != nil {
			w.logger.Debug("chunk analysis failed", "chunk", chunk.Index, "error", err)
			continue
		}
		chunkResults = append(chunkResults, result)
		// Store per-chunk summary indexed by chunk position
		if chunk.Index >= 0 && chunk.Index < len(chunkSummaries) {
			chunkSummaries[chunk.Index] = result.Summary
		} else {
			w.logger.Warn("chunk index out of bounds for summary storage",
				"chunk_index", chunk.Index,
				"summaries_len", len(chunkSummaries))
		}
	}

	// Merge results
	merged, err := mergeSemanticResults(ctx, w.semanticProvider, chunkResults)
	return merged, chunkSummaries, err
}

// analyzeChunk performs semantic analysis on a single chunk.
func (w *Worker) analyzeChunk(ctx context.Context, chunk chunkers.Chunk) (*SemanticResult, error) {
	req := providers.SemanticRequest{
		Content:  chunk.Content,
		MIMEType: "text/plain",
	}

	result, err := w.semanticProvider.Analyze(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert provider result to SemanticResult
	entities := make([]Entity, 0, len(result.Entities))
	for _, e := range result.Entities {
		entities = append(entities, Entity{Name: e.Name, Type: e.Type})
	}

	refs := make([]Reference, 0, len(result.References))
	for _, r := range result.References {
		refs = append(refs, Reference{Type: r.Type, Target: r.Target})
	}

	topics := make([]string, 0, len(result.Topics))
	for _, t := range result.Topics {
		topics = append(topics, t.Name)
	}

	return &SemanticResult{
		Summary:    result.Summary,
		Tags:       result.Tags,
		Topics:     topics,
		Entities:   entities,
		References: refs,
		Complexity: result.Complexity,
		Keywords:   result.Keywords,
	}, nil
}

// generateEmbeddings creates embeddings for chunks using batch API with caching.
// Returns both the file-level average embedding and per-chunk results.
// The chunkSummaries parameter is optional and provides per-chunk semantic summaries.
func (w *Worker) generateEmbeddings(ctx context.Context, chunks []chunkers.Chunk, chunkSummaries []string) ([]float32, []AnalyzedChunk, error) {
	if len(chunks) == 0 {
		return nil, nil, nil
	}

	// Prepare chunk results and identify cache hits/misses
	chunkResults := make([]AnalyzedChunk, len(chunks))
	var needsEmbedding []int // indices of chunks that need embedding

	for i, chunk := range chunks {
		chunkHash := computeContentHash([]byte(chunk.Content))
		chunkResults[i] = AnalyzedChunk{
			Index:       chunk.Index,
			Content:     chunk.Content,
			ContentHash: chunkHash,
			StartOffset: chunk.StartOffset,
			EndOffset:   chunk.EndOffset,
			ChunkType:   string(chunk.Metadata.Type),
			TokenCount:  chunk.Metadata.TokenEstimate,
			Metadata:    &chunk.Metadata, // Preserve full typed metadata
		}
		// Set per-chunk summary if available
		if chunkSummaries != nil && chunk.Index < len(chunkSummaries) {
			chunkResults[i].Summary = chunkSummaries[chunk.Index]
		}

		// Check embeddings cache
		if w.embeddingsCache != nil {
			cached, err := w.embeddingsCache.Get(chunkHash, chunk.Index)
			if err == nil {
				chunkResults[i].Embedding = cached.Embedding
				w.logger.Debug("embeddings cache hit", "chunk", i)
				continue
			}
		}
		needsEmbedding = append(needsEmbedding, i)
	}

	// Generate embeddings for cache misses
	if len(needsEmbedding) > 0 {
		w.logger.Debug("generating embeddings for cache misses",
			"total_chunks", len(chunks),
			"cache_misses", len(needsEmbedding))

		// Collect texts for batch embedding
		texts := make([]string, len(needsEmbedding))
		for j, idx := range needsEmbedding {
			texts[j] = chunks[idx].Content
		}

		var embeddings []providers.EmbeddingsBatchResult
		var err error

		if len(texts) == 1 {
			// Single embedding - use simple API
			req := providers.EmbeddingsRequest{Content: texts[0]}
			result, e := w.embeddingsProvider.Embed(ctx, req)
			if e != nil {
				return nil, nil, fmt.Errorf("embedding failed; %w", e)
			}
			embeddings = []providers.EmbeddingsBatchResult{{
				Index:     0,
				Embedding: result.Embedding,
			}}
		} else {
			// Batch embedding
			embeddings, err = w.embeddingsProvider.EmbedBatch(ctx, texts)
			if err != nil {
				return nil, nil, fmt.Errorf("batch embeddings failed; %w", err)
			}
		}

		// Store embeddings in results and cache
		for j, emb := range embeddings {
			idx := needsEmbedding[j]
			chunkResults[idx].Embedding = emb.Embedding

			// Cache the embedding
			if w.embeddingsCache != nil {
				cacheResult := &providers.EmbeddingsResult{
					Embedding:  emb.Embedding,
					Dimensions: len(emb.Embedding),
				}
				if err := w.embeddingsCache.Set(chunkResults[idx].ContentHash, chunkResults[idx].Index, cacheResult); err != nil {
					w.logger.Warn("embeddings cache write error",
						"chunk", idx,
						"error", err)
				}
			}
		}
	}

	// Compute file-level average embedding
	var allEmbeddings []providers.EmbeddingsBatchResult
	for i, cr := range chunkResults {
		if cr.Embedding != nil {
			allEmbeddings = append(allEmbeddings, providers.EmbeddingsBatchResult{
				Index:     i,
				Embedding: cr.Embedding,
			})
		}
	}

	fileEmbedding := averageEmbeddings(allEmbeddings)
	return fileEmbedding, chunkResults, nil
}

// averageEmbeddings computes the element-wise average of multiple embeddings.
func averageEmbeddings(results []providers.EmbeddingsBatchResult) []float32 {
	if len(results) == 0 {
		return nil
	}

	dims := len(results[0].Embedding)
	avg := make([]float32, dims)

	for _, r := range results {
		for i, v := range r.Embedding {
			avg[i] += v
		}
	}

	n := float32(len(results))
	for i := range avg {
		avg[i] /= n
	}

	return avg
}

// SetSemanticProvider sets the semantic analysis provider.
func (w *Worker) SetSemanticProvider(p providers.SemanticProvider) {
	w.semanticProvider = p
}

// SetEmbeddingsProvider sets the embeddings provider.
func (w *Worker) SetEmbeddingsProvider(p providers.EmbeddingsProvider) {
	w.embeddingsProvider = p
}

// SetGraph sets the graph client for persisting analysis results.
func (w *Worker) SetGraph(g graph.Graph) {
	w.graph = g
}

// SetRegistry sets the registry for tracking file state.
func (w *Worker) SetRegistry(r registry.Registry) {
	w.registry = r
}

// SetAnalysisVersion sets the version string for tracking schema changes.
func (w *Worker) SetAnalysisVersion(version string) {
	w.analysisVersion = version
}

// SetCaches sets the semantic and embeddings caches.
func (w *Worker) SetCaches(semantic *cache.SemanticCache, embeddings *cache.EmbeddingsCache) {
	w.semanticCache = semantic
	w.embeddingsCache = embeddings
}

// convertCachedSemantic converts a cached providers.SemanticResult to the local SemanticResult type.
func (w *Worker) convertCachedSemantic(cached *providers.SemanticResult) *SemanticResult {
	entities := make([]Entity, 0, len(cached.Entities))
	for _, e := range cached.Entities {
		entities = append(entities, Entity{Name: e.Name, Type: e.Type})
	}

	refs := make([]Reference, 0, len(cached.References))
	for _, r := range cached.References {
		refs = append(refs, Reference{Type: r.Type, Target: r.Target})
	}

	topics := make([]string, 0, len(cached.Topics))
	for _, t := range cached.Topics {
		topics = append(topics, t.Name)
	}

	return &SemanticResult{
		Summary:    cached.Summary,
		Tags:       cached.Tags,
		Topics:     topics,
		Entities:   entities,
		References: refs,
		Complexity: cached.Complexity,
		Keywords:   cached.Keywords,
	}
}

// convertToProviderSemantic converts the local SemanticResult to providers.SemanticResult for caching.
func (w *Worker) convertToProviderSemantic(result *SemanticResult) *providers.SemanticResult {
	entities := make([]providers.Entity, 0, len(result.Entities))
	for _, e := range result.Entities {
		entities = append(entities, providers.Entity{Name: e.Name, Type: e.Type})
	}

	refs := make([]providers.Reference, 0, len(result.References))
	for _, r := range result.References {
		refs = append(refs, providers.Reference{Type: r.Type, Target: r.Target})
	}

	topics := make([]providers.Topic, 0, len(result.Topics))
	for _, t := range result.Topics {
		topics = append(topics, providers.Topic{Name: t, Confidence: 1.0})
	}

	return &providers.SemanticResult{
		Summary:    result.Summary,
		Tags:       result.Tags,
		Topics:     topics,
		Entities:   entities,
		References: refs,
		Complexity: result.Complexity,
		Keywords:   result.Keywords,
	}
}

// persistToGraph writes analysis results to the graph database.
func (w *Worker) persistToGraph(ctx context.Context, result *AnalysisResult) error {
	if w.graph == nil {
		return nil // Graph not configured, skip persistence
	}

	// Build FileNode from analysis result
	fileNode := &graph.FileNode{
		Path:         result.FilePath,
		Name:         filepath.Base(result.FilePath),
		Extension:    filepath.Ext(result.FilePath),
		MIMEType:     result.MIMEType,
		Language:     result.Language,
		Size:         result.FileSize,
		ModTime:      result.ModTime,
		ContentHash:  result.ContentHash,
		MetadataHash: result.MetadataHash,
		Summary:      result.Summary,
		Complexity:   result.Complexity,
		AnalyzedAt:   result.AnalyzedAt,
	}

	// Upsert file node
	if err := w.graph.UpsertFile(ctx, fileNode); err != nil {
		return fmt.Errorf("failed to upsert file; %w", err)
	}

	// Delete existing chunks before inserting new ones (clean slate on reanalysis)
	if err := w.graph.DeleteChunks(ctx, result.FilePath); err != nil {
		return fmt.Errorf("failed to delete existing chunks; %w", err)
	}

	// Persist chunks with metadata and embeddings
	for _, chunk := range result.Chunks {
		chunkNode := &graph.ChunkNode{
			ID:          chunk.ContentHash, // Use content hash as ID for deduplication
			FilePath:    result.FilePath,
			Index:       chunk.Index,
			Content:     chunk.Content,
			ContentHash: chunk.ContentHash,
			StartOffset: chunk.StartOffset,
			EndOffset:   chunk.EndOffset,
			ChunkType:   chunk.ChunkType,
			Summary:     chunk.Summary,
			TokenCount:  chunk.TokenCount,
		}

		// Upsert chunk with its typed metadata
		if err := w.graph.UpsertChunkWithMetadata(ctx, chunkNode, chunk.Metadata); err != nil {
			w.logger.Warn("failed to upsert chunk with metadata",
				"path", result.FilePath,
				"chunk", chunk.Index,
				"error", err)
			continue // Skip embedding if chunk failed
		}

		// Upsert embedding if present
		if len(chunk.Embedding) > 0 {
			embNode := &graph.ChunkEmbeddingNode{
				Provider:   "default", // TODO: get from config
				Model:      "default", // TODO: get from config
				Dimensions: len(chunk.Embedding),
				Embedding:  chunk.Embedding,
			}
			if err := w.graph.UpsertChunkEmbedding(ctx, chunk.ContentHash, embNode); err != nil {
				w.logger.Warn("failed to upsert embedding",
					"path", result.FilePath,
					"chunk", chunk.Index,
					"error", err)
			}
		}
	}

	// Set tags
	if len(result.Tags) > 0 {
		if err := w.graph.SetFileTags(ctx, result.FilePath, result.Tags); err != nil {
			return fmt.Errorf("failed to set tags; %w", err)
		}
	}

	// Set topics (convert to graph.Topic)
	if len(result.Topics) > 0 {
		topics := make([]graph.Topic, len(result.Topics))
		for i, t := range result.Topics {
			topics[i] = graph.Topic{Name: t, Confidence: 1.0}
		}
		if err := w.graph.SetFileTopics(ctx, result.FilePath, topics); err != nil {
			return fmt.Errorf("failed to set topics; %w", err)
		}
	}

	// Set entities (convert to graph.Entity)
	if len(result.Entities) > 0 {
		entities := make([]graph.Entity, len(result.Entities))
		for i, e := range result.Entities {
			entities[i] = graph.Entity{Name: e.Name, Type: e.Type}
		}
		if err := w.graph.SetFileEntities(ctx, result.FilePath, entities); err != nil {
			return fmt.Errorf("failed to set entities; %w", err)
		}
	}

	// Set references (convert to graph.Reference)
	if len(result.References) > 0 {
		refs := make([]graph.Reference, len(result.References))
		for i, r := range result.References {
			refs[i] = graph.Reference{Type: r.Type, Target: r.Target}
		}
		if err := w.graph.SetFileReferences(ctx, result.FilePath, refs); err != nil {
			return fmt.Errorf("failed to set references; %w", err)
		}
	}

	return nil
}

// Helper functions

// detectMIMEType determines the MIME type of content.
func detectMIMEType(path string, content []byte) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".js":
		return "text/javascript"
	case ".ts":
		return "text/typescript"
	case ".md", ".markdown":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".txt":
		return "text/plain"
	case ".csv":
		return "text/csv"
	default:
		// Basic content sniffing
		if len(content) > 0 {
			if content[0] == '{' || content[0] == '[' {
				return "application/json"
			}
		}
		return "application/octet-stream"
	}
}

// detectLanguage determines the programming language from file extension.
func detectLanguage(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh", ".bash":
		return "bash"
	case ".sql":
		return "sql"
	default:
		return ""
	}
}

// computeContentHash computes a hash of the content.
func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// computeMetadataHash computes a hash of file metadata.
func computeMetadataHash(path string, size int64, modTime time.Time) string {
	data := path + "|" + strconv.FormatInt(size, 10) + "|" + strconv.FormatInt(modTime.UnixNano(), 10)
	var sum uint64
	for _, b := range []byte(data) {
		sum = sum*31 + uint64(b)
	}
	return fmt.Sprintf("%016x", sum)
}
