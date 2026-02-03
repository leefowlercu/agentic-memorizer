package analysis

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	_ "github.com/leefowlercu/agentic-memorizer/internal/chunkers/code/languages" // Register tree-sitter chunker factory.
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
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
	IngestKind   ingest.Kind
	IngestMode   ingest.Mode
	IngestReason string

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

	// Summary is an optional per-chunk summary (unused in per-file semantics).
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

	// Pipeline for analysis (when set, takes precedence over individual stages)
	pipeline *Pipeline

	// Providers (injected or looked up) - used when pipeline is not set
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

// defaultChunkerRegistry creates a chunker registry with all standard chunkers.
func defaultChunkerRegistry() *chunkers.Registry {
	return chunkers.DefaultRegistry()
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
			if err := w.processItem(ctx, item); err != nil {
				select {
				case w.queue.errChan <- err:
				default:
				}
			}
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
func (w *Worker) processItem(ctx context.Context, item WorkItem) error {
	start := time.Now()

	result, err := w.analyze(ctx, item)
	if err != nil {
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
			return nil
		}

		w.logger.Error("analysis failed permanently",
			"path", item.FilePath,
			"error", err,
			"retries", item.Retries)
		w.queue.recordAnalysisFailure()
		w.queue.publishAnalysisFailed(item.FilePath, err)
		return fmt.Errorf("analysis failed permanently; %w", err)
	}

	duration := time.Since(start)
	result.ProcessingTime = duration

	persistenceStage := NewPersistenceStage(w.graph, WithPersistenceLogger(w.logger))
	if err := persistenceStage.Persist(ctx, result); err != nil {
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
			return nil
		}

		w.logger.Error("graph persistence failed permanently",
			"path", item.FilePath,
			"error", err,
			"retries", item.Retries)
		w.queue.recordPersistenceFailure()
		w.queue.publishGraphPersistenceFailed(item.FilePath, err, item.Retries)
		return fmt.Errorf("graph persistence failed permanently; %w", err)
	}

	w.queue.recordSuccess(duration)
	w.queue.publishAnalysisComplete(item.FilePath, result)
	w.logger.Info("analysis complete",
		"path", item.FilePath,
		"chunks", result.ChunksProcessed,
		"duration", duration)

	return nil
}

// calculateBackoff returns the delay for a retry attempt.
func (w *Worker) calculateBackoff(retries int) time.Duration {
	// Exponential backoff: base * 2^(retries-1)
	delay := float64(w.queue.retryDelay) * math.Pow(2, float64(retries-1))
	return time.Duration(delay)
}

// analyze performs the full analysis pipeline.
func (w *Worker) analyze(ctx context.Context, item WorkItem) (*AnalysisResult, error) {
	stats := w.queue.Stats()
	mode := stats.DegradationMode

	// Use pipeline if available
	if w.pipeline != nil {
		return w.analyzeWithPipeline(ctx, item, mode)
	}

	semanticEnabled := w.semanticProvider != nil && w.semanticProvider.Available()
	fileReader := NewFileReader(w.registry, WithSemanticEnabled(semanticEnabled))
	fileResult, err := fileReader.Read(ctx, item, mode)
	if err != nil {
		return nil, err
	}

	result := &AnalysisResult{
		FilePath:     item.FilePath,
		FileSize:     fileResult.Info.Size(),
		ModTime:      fileResult.Info.ModTime(),
		MIMEType:     fileResult.MIMEType,
		Language:     fileResult.Language,
		IngestKind:   fileResult.Kind,
		IngestMode:   fileResult.IngestMode,
		IngestReason: fileResult.IngestReason,
		AnalyzedAt:   time.Now(),
		ContentHash:  fileResult.ContentHash,
		MetadataHash: fileResult.MetadataHash,
	}

	w.syncMetadataState(ctx, result)

	if fileResult.IngestMode == ingest.ModeMetadataOnly || fileResult.IngestMode == ingest.ModeSkip {
		w.updateRegistryForMetadataOnly(ctx, result, fileResult.DegradedMetadata, fileResult.IngestReason)
		// Publish skipped event for observability
		decision := "metadata_only"
		if fileResult.IngestMode == ingest.ModeSkip {
			decision = "skipped"
		}
		w.queue.publishAnalysisSkipped(item.FilePath, decision, fileResult.IngestReason)
		return result, nil
	}

	if fileResult.IngestMode == ingest.ModeSemanticOnly {
		if w.semanticProvider != nil && w.semanticProvider.Available() {
			semanticStart := time.Now()
			semanticStage := NewSemanticStage(w.semanticProvider, w.semanticCache, w.registry, w.analysisVersion, w.logger)
			input, buildErr := BuildSemanticInput(item.FilePath, fileResult, nil, w.semanticProvider)
			if buildErr != nil {
				w.logger.Warn("semantic input build failed", "path", item.FilePath, "error", buildErr)
			} else {
				semanticResult, semanticErr := semanticStage.Analyze(ctx, input, result.ContentHash)
				semanticDuration := time.Since(semanticStart)
				if semanticErr != nil {
					w.logger.Warn("semantic analysis failed",
						"path", item.FilePath,
						"error", semanticErr)
					w.queue.publishSemanticAnalysisFailed(item.FilePath, semanticErr)
				}

				if semanticResult != nil {
					result.Summary = semanticResult.Summary
					result.Tags = semanticResult.Tags
					result.Topics = semanticResult.Topics
					result.Entities = semanticResult.Entities
					result.References = semanticResult.References
					result.Complexity = semanticResult.Complexity
					result.Keywords = semanticResult.Keywords
					// Publish semantic analysis complete event
					w.queue.publishAnalysisSemanticComplete(item.FilePath, result.ContentHash, semanticDuration)
				}
			}
		}
		if w.registry != nil {
			if err := w.registry.UpdateEmbeddingsState(ctx, result.FilePath, nil); err != nil {
				w.logger.Warn("failed to update embeddings state", "path", result.FilePath, "error", err)
			}
		}
		return result, nil
	}

	chunkerStage := NewChunkerStage(w.chunkerRegistry)
	chunkResult, err := chunkerStage.Chunk(ctx, fileResult.Content, fileResult.MIMEType, fileResult.Language)
	if err != nil {
		return nil, fmt.Errorf("chunking failed; %w", err)
	}

	result.ChunkerUsed = chunkResult.ChunkerUsed
	result.ChunksProcessed = chunkResult.TotalChunks

	// Build analyzed chunks immediately after chunking.
	// This decouples chunk persistence from embeddings generation.
	analyzedChunks := BuildAnalyzedChunks(chunkResult.Chunks)

	if w.semanticProvider != nil && w.semanticProvider.Available() {
		semanticStart := time.Now()
		semanticStage := NewSemanticStage(w.semanticProvider, w.semanticCache, w.registry, w.analysisVersion, w.logger)
		input, buildErr := BuildSemanticInput(item.FilePath, fileResult, chunkResult, w.semanticProvider)
		if buildErr != nil {
			w.logger.Warn("semantic input build failed", "path", item.FilePath, "error", buildErr)
		} else {
			semanticResult, semanticErr := semanticStage.Analyze(ctx, input, result.ContentHash)
			semanticDuration := time.Since(semanticStart)
			if semanticErr != nil {
				w.logger.Warn("semantic analysis failed",
					"path", item.FilePath,
					"error", semanticErr)
				w.queue.publishSemanticAnalysisFailed(item.FilePath, semanticErr)
			}

			if semanticResult != nil {
				result.Summary = semanticResult.Summary
				result.Tags = semanticResult.Tags
				result.Topics = semanticResult.Topics
				result.Entities = semanticResult.Entities
				result.References = semanticResult.References
				result.Complexity = semanticResult.Complexity
				result.Keywords = semanticResult.Keywords
				// Publish semantic analysis complete event
				w.queue.publishAnalysisSemanticComplete(item.FilePath, result.ContentHash, semanticDuration)
			}
		}
	}

	// Always populate result.Chunks regardless of embeddings mode.
	// This ensures chunks are persisted even when embeddings are skipped.
	result.Chunks = analyzedChunks

	if mode == DegradationNoEmbed {
		return result, nil
	}

	if w.embeddingsProvider != nil && w.embeddingsProvider.Available() {
		embeddingsStart := time.Now()
		embeddingsStage := NewEmbeddingsStage(w.embeddingsProvider, w.embeddingsCache, w.registry, w.logger)
		embeddings, embeddingsErr := embeddingsStage.Generate(ctx, item.FilePath, result.Chunks)
		embeddingsDuration := time.Since(embeddingsStart)
		if embeddingsErr != nil {
			w.logger.Warn("embeddings generation failed",
				"path", item.FilePath,
				"error", embeddingsErr)
			w.queue.publishEmbeddingsGenerationFailed(item.FilePath, embeddingsErr)
		} else {
			result.Embeddings = embeddings
			// Publish embeddings generation complete event
			w.queue.publishAnalysisEmbeddingsComplete(item.FilePath, result.ContentHash, embeddingsDuration)
		}
	}

	return result, nil
}

func (w *Worker) syncMetadataState(ctx context.Context, result *AnalysisResult) {
	if w.registry == nil {
		return
	}

	// Check if content has changed (requires clearing previous analysis state)
	// Note: This check-then-act is subject to race conditions if multiple workers
	// process the same file concurrently. This is rare in practice but possible
	// during full rebuilds. The registry should ideally support atomic compare-and-swap.
	existingState, err := w.registry.GetFileState(ctx, result.FilePath)
	if err == nil && existingState.ContentHash != result.ContentHash {
		w.logger.Debug("content changed; clearing analysis state",
			"path", result.FilePath,
			"old_hash", existingState.ContentHash[:8],
			"new_hash", result.ContentHash[:8])
		if clearErr := w.registry.ClearAnalysisState(ctx, result.FilePath); clearErr != nil {
			w.logger.Warn("failed to clear analysis state", "path", result.FilePath, "error", clearErr)
		}
	}

	if err := w.registry.UpdateMetadataState(ctx, result.FilePath, result.ContentHash, result.MetadataHash, result.FileSize, result.ModTime); err != nil {
		w.logger.Warn("failed to update metadata state", "path", result.FilePath, "error", err)
	}
}

func (w *Worker) updateRegistryForMetadataOnly(ctx context.Context, result *AnalysisResult, degradedMetadata bool, ingestReason string) {
	if w.registry == nil || degradedMetadata {
		return
	}
	if ingestReason == ingest.ReasonSemanticDisabled {
		return
	}

	version := analysisVersionOrDefault(w.analysisVersion)
	if err := w.registry.UpdateSemanticState(ctx, result.FilePath, version, nil); err != nil {
		w.logger.Warn("failed to update semantic state", "path", result.FilePath, "error", err)
	}
	if err := w.registry.UpdateEmbeddingsState(ctx, result.FilePath, nil); err != nil {
		w.logger.Warn("failed to update embeddings state", "path", result.FilePath, "error", err)
	}
}

// analyzeWithPipeline delegates analysis to the configured pipeline.
func (w *Worker) analyzeWithPipeline(ctx context.Context, item WorkItem, mode DegradationMode) (*AnalysisResult, error) {
	pctx := NewPipelineContext(item, mode, w.logger)

	if err := w.pipeline.Execute(ctx, pctx); err != nil {
		return nil, err
	}

	// Publish stage-specific events for observability
	w.publishPipelineEvents(ctx, pctx)

	return pctx.AnalysisResult, nil
}

// publishPipelineEvents publishes events based on pipeline execution results.
func (w *Worker) publishPipelineEvents(_ context.Context, pctx *PipelineContext) {
	if pctx.FileResult == nil {
		return
	}

	// Publish skip event for metadata-only or skipped files
	if pctx.IsMetadataOnly() || pctx.ShouldSkip() {
		decision := "metadata_only"
		if pctx.ShouldSkip() {
			decision = "skipped"
		}
		w.queue.publishAnalysisSkipped(pctx.WorkItem.FilePath, decision, pctx.FileResult.IngestReason)
		return
	}

	result := pctx.AnalysisResult
	if result == nil {
		return
	}

	// Publish semantic completion event if semantic analysis was performed
	if pctx.SemanticResult != nil {
		w.queue.publishAnalysisSemanticComplete(pctx.WorkItem.FilePath, result.ContentHash, 0)
	}

	// Publish embeddings completion event if embeddings were generated
	if pctx.Embeddings != nil {
		w.queue.publishAnalysisEmbeddingsComplete(pctx.WorkItem.FilePath, result.ContentHash, 0)
	}
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

// SetPipeline sets the analysis pipeline.
// When a pipeline is set, the worker delegates analysis to it instead of
// using individual stages directly.
func (w *Worker) SetPipeline(p *Pipeline) {
	w.pipeline = p
}

// Helper functions

func readHead(path string, size int) ([]byte, error) {
	if size <= 0 {
		size = 4096
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := make([]byte, size)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buf[:n], nil
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
