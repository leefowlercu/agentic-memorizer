package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
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

	// Embeddings
	Embeddings []float32

	// Processing info
	ChunkerUsed     string
	ChunksProcessed int
	ProcessingTime  time.Duration
	AnalyzedAt      time.Time
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
	stopped  bool

	// Providers (injected or looked up)
	semanticProvider   providers.SemanticProvider
	embeddingsProvider providers.EmbeddingsProvider
	chunkerRegistry    *chunkers.Registry
}

// NewWorker creates a new analysis worker.
func NewWorker(id int, queue *Queue) *Worker {
	return &Worker{
		id:              id,
		queue:           queue,
		logger:          queue.logger.With("worker_id", id),
		stopChan:        make(chan struct{}),
		chunkerRegistry: chunkers.DefaultRegistry(),
	}
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
	if !w.stopped {
		close(w.stopChan)
		w.stopped = true
	}
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

		w.queue.recordFailure()
		w.queue.publishAnalysisFailed(item.FilePath, err)
		return
	}

	duration := time.Since(start)
	result.ProcessingTime = duration

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
	if w.semanticProvider != nil && w.semanticProvider.Available() {
		semanticResult, err := w.analyzeSemantics(ctx, chunkResult.Chunks)
		if err != nil {
			w.logger.Warn("semantic analysis failed", "error", err)
			// Continue without semantic analysis
		} else {
			result.Summary = semanticResult.Summary
			result.Tags = semanticResult.Tags
			result.Topics = semanticResult.Topics
			result.Entities = semanticResult.Entities
			result.References = semanticResult.References
			result.Complexity = semanticResult.Complexity
			result.Keywords = semanticResult.Keywords
		}
	}

	if mode == DegradationNoEmbed {
		// Skip embeddings
		return result, nil
	}

	// Step 4: Generate embeddings (if provider available)
	if w.embeddingsProvider != nil && w.embeddingsProvider.Available() {
		embeddings, err := w.generateEmbeddings(ctx, content)
		if err != nil {
			w.logger.Warn("embeddings generation failed", "error", err)
			// Continue without embeddings
		} else {
			result.Embeddings = embeddings
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
func (w *Worker) analyzeSemantics(ctx context.Context, chunks []chunkers.Chunk) (*SemanticResult, error) {
	if len(chunks) == 0 {
		return &SemanticResult{}, nil
	}

	// For single chunk, analyze directly
	if len(chunks) == 1 {
		return w.analyzeChunk(ctx, chunks[0])
	}

	// For multiple chunks, analyze each and merge
	var chunkResults []*SemanticResult
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := w.analyzeChunk(ctx, chunk)
		if err != nil {
			w.logger.Debug("chunk analysis failed", "chunk", chunk.Index, "error", err)
			continue
		}
		chunkResults = append(chunkResults, result)
	}

	// Merge results
	return mergeSemanticResults(ctx, w.semanticProvider, chunkResults)
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

// generateEmbeddings creates embeddings for the content.
func (w *Worker) generateEmbeddings(ctx context.Context, content []byte) ([]float32, error) {
	req := providers.EmbeddingsRequest{
		Content: string(content),
	}

	result, err := w.embeddingsProvider.Embed(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.Embedding, nil
}

// SetSemanticProvider sets the semantic analysis provider.
func (w *Worker) SetSemanticProvider(p providers.SemanticProvider) {
	w.semanticProvider = p
}

// SetEmbeddingsProvider sets the embeddings provider.
func (w *Worker) SetEmbeddingsProvider(p providers.EmbeddingsProvider) {
	w.embeddingsProvider = p
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
	// Use xxHash for performance (placeholder using simple checksum)
	var sum uint64
	for _, b := range content {
		sum = sum*31 + uint64(b)
	}
	return fmt.Sprintf("%016x", sum)
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
