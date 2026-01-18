package analysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/fsutil"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// FileReadResult captures the output of the file reader stage.
type FileReadResult struct {
	Info             os.FileInfo
	Peek             []byte
	Kind             ingest.Kind
	MIMEType         string
	Language         string
	IngestMode       ingest.Mode
	IngestReason     string
	DegradedMetadata bool
	Content          []byte
	ContentHash      string
	MetadataHash     string
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

// FileReader performs file stat, head read, ingest decision, and hashing.
type FileReader struct {
	registry registry.Registry
}

// NewFileReader creates a file reader stage.
func NewFileReader(reg registry.Registry) *FileReader {
	return &FileReader{registry: reg}
}

// Read collects file metadata, ingest decisions, and content hash.
func (r *FileReader) Read(ctx context.Context, item WorkItem, mode DegradationMode) (*FileReadResult, error) {
	info, err := os.Stat(item.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file; %w", err)
	}
	peek, err := readHead(item.FilePath, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to read file head; %w", err)
	}

	kind, mimeType, language := ingest.Probe(item.FilePath, info, peek)
	var pathConfig *registry.PathConfig
	if r.registry != nil {
		cfg, err := r.registry.GetEffectiveConfig(ctx, item.FilePath)
		if err == nil {
			pathConfig = cfg
		}
	}

	ingestMode, ingestReason := ingest.Decide(kind, pathConfig, info.Size())
	degradedMetadata := false
	if mode == DegradationMetadata && ingestMode == ingest.ModeChunk {
		ingestMode = ingest.ModeMetadataOnly
		degradedMetadata = true
	}

	var content []byte
	var contentHash string
	if ingestMode == ingest.ModeChunk {
		content, err = os.ReadFile(item.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file; %w", err)
		}
		contentHash = fsutil.HashBytes(content)
	} else {
		contentHash, err = fsutil.HashFile(item.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to hash file; %w", err)
		}
	}

	return &FileReadResult{
		Info:             info,
		Peek:             peek,
		Kind:             kind,
		MIMEType:         mimeType,
		Language:         language,
		IngestMode:       ingestMode,
		IngestReason:     ingestReason,
		DegradedMetadata: degradedMetadata,
		Content:          content,
		ContentHash:      contentHash,
		MetadataHash:     computeMetadataHash(item.FilePath, info.Size(), info.ModTime()),
	}, nil
}

// ChunkerStage performs content chunking.
type ChunkerStage struct {
	registry *chunkers.Registry
}

// NewChunkerStage creates a chunker stage.
func NewChunkerStage(registry *chunkers.Registry) *ChunkerStage {
	return &ChunkerStage{registry: registry}
}

// Chunk splits content using the configured chunker registry.
func (s *ChunkerStage) Chunk(ctx context.Context, content []byte, mimeType, language string) (*chunkers.ChunkResult, error) {
	if s.registry == nil {
		return nil, fmt.Errorf("chunker registry not configured")
	}

	opts := chunkers.DefaultChunkOptions()
	opts.MIMEType = mimeType
	opts.Language = language

	return s.registry.Chunk(ctx, content, opts)
}

// SemanticStage performs semantic analysis with optional caching.
type SemanticStage struct {
	provider        providers.SemanticProvider
	cache           *cache.SemanticCache
	registry        registry.Registry
	analysisVersion string
	logger          *slog.Logger
}

// NewSemanticStage creates a semantic stage.
func NewSemanticStage(provider providers.SemanticProvider, cache *cache.SemanticCache, reg registry.Registry, analysisVersion string, logger *slog.Logger) *SemanticStage {
	return &SemanticStage{
		provider:        provider,
		cache:           cache,
		registry:        reg,
		analysisVersion: analysisVersion,
		logger:          logger,
	}
}

// Analyze runs semantic analysis and updates registry state.
func (s *SemanticStage) Analyze(ctx context.Context, path, contentHash string, chunks []chunkers.Chunk) (*SemanticResult, []string, error) {
	if s.provider == nil || !s.provider.Available() {
		return nil, nil, nil
	}

	logger := loggerOrDefault(s.logger)
	var semanticResult *SemanticResult
	var chunkSummaries []string
	var semanticErr error
	cacheHit := false

	if s.cache != nil {
		cachedResult, err := s.cache.Get(contentHash)
		if err == nil {
			semanticResult = convertCachedSemantic(cachedResult)
			cacheHit = true
			logger.Debug("semantic cache hit", "path", path)
		} else if !errors.Is(err, cache.ErrCacheMiss) {
			logger.Warn("semantic cache read error", "path", path, "error", err)
		}
	}

	if !cacheHit {
		semanticResult, chunkSummaries, semanticErr = analyzeSemantics(ctx, s.provider, chunks, logger)
		if semanticErr == nil && s.cache != nil && semanticResult != nil {
			providerResult := convertToProviderSemantic(semanticResult)
			if cacheErr := s.cache.Set(contentHash, providerResult); cacheErr != nil {
				logger.Warn("semantic cache write error", "path", path, "error", cacheErr)
			}
		}
	}

	if s.registry != nil {
		version := analysisVersionOrDefault(s.analysisVersion)
		if err := s.registry.UpdateSemanticState(ctx, path, version, semanticErr); err != nil {
			logger.Warn("failed to update semantic state", "path", path, "error", err)
		}
	}

	return semanticResult, chunkSummaries, semanticErr
}

// EmbeddingsStage generates embeddings and updates registry state.
type EmbeddingsStage struct {
	provider providers.EmbeddingsProvider
	cache    *cache.EmbeddingsCache
	registry registry.Registry
	logger   *slog.Logger
}

// NewEmbeddingsStage creates an embeddings stage.
func NewEmbeddingsStage(provider providers.EmbeddingsProvider, cache *cache.EmbeddingsCache, reg registry.Registry, logger *slog.Logger) *EmbeddingsStage {
	return &EmbeddingsStage{
		provider: provider,
		cache:    cache,
		registry: reg,
		logger:   logger,
	}
}

// Generate runs embeddings generation and updates registry state.
func (s *EmbeddingsStage) Generate(ctx context.Context, path string, chunks []chunkers.Chunk, chunkSummaries []string) ([]float32, []AnalyzedChunk, error) {
	if s.provider == nil || !s.provider.Available() {
		return nil, nil, nil
	}

	logger := loggerOrDefault(s.logger)
	embeddings, chunkData, embeddingsErr := generateEmbeddings(ctx, s.provider, s.cache, logger, chunks, chunkSummaries)

	if s.registry != nil {
		if err := s.registry.UpdateEmbeddingsState(ctx, path, embeddingsErr); err != nil {
			logger.Warn("failed to update embeddings state", "path", path, "error", err)
		}
	}

	return embeddings, chunkData, embeddingsErr
}

// PersistenceStage writes analysis results to the graph.
type PersistenceStage struct {
	graph  graph.Graph
	logger *slog.Logger
}

// NewPersistenceStage creates a persistence stage.
func NewPersistenceStage(g graph.Graph, logger *slog.Logger) *PersistenceStage {
	return &PersistenceStage{
		graph:  g,
		logger: logger,
	}
}

// Persist writes analysis results to the graph.
func (s *PersistenceStage) Persist(ctx context.Context, result *AnalysisResult) error {
	if s.graph == nil {
		return nil
	}

	logger := loggerOrDefault(s.logger)
	if result.IngestMode == ingest.ModeSkip {
		if err := s.graph.DeleteFile(ctx, result.FilePath); err != nil {
			return fmt.Errorf("failed to delete skipped file; %w", err)
		}
		return nil
	}

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
		IngestKind:   string(result.IngestKind),
		IngestMode:   string(result.IngestMode),
		IngestReason: result.IngestReason,
	}

	if err := s.graph.UpsertFile(ctx, fileNode); err != nil {
		return fmt.Errorf("failed to upsert file; %w", err)
	}

	if err := s.graph.DeleteChunks(ctx, result.FilePath); err != nil {
		return fmt.Errorf("failed to delete existing chunks; %w", err)
	}

	for _, chunk := range result.Chunks {
		chunkNode := &graph.ChunkNode{
			ID:          chunk.ContentHash,
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

		if err := s.graph.UpsertChunkWithMetadata(ctx, chunkNode, chunk.Metadata); err != nil {
			logger.Warn("failed to upsert chunk with metadata",
				"path", result.FilePath,
				"chunk", chunk.Index,
				"error", err)
			continue
		}

		if len(chunk.Embedding) > 0 {
			embNode := &graph.ChunkEmbeddingNode{
				Provider:   "default",
				Model:      "default",
				Dimensions: len(chunk.Embedding),
				Embedding:  chunk.Embedding,
			}
			if err := s.graph.UpsertChunkEmbedding(ctx, chunk.ContentHash, embNode); err != nil {
				logger.Warn("failed to upsert embedding",
					"path", result.FilePath,
					"chunk", chunk.Index,
					"error", err)
			}
		}
	}

	if len(result.Tags) > 0 {
		if err := s.graph.SetFileTags(ctx, result.FilePath, result.Tags); err != nil {
			return fmt.Errorf("failed to set tags; %w", err)
		}
	}

	if len(result.Topics) > 0 {
		topics := make([]graph.Topic, len(result.Topics))
		for i, t := range result.Topics {
			topics[i] = graph.Topic{Name: t, Confidence: 1.0}
		}
		if err := s.graph.SetFileTopics(ctx, result.FilePath, topics); err != nil {
			return fmt.Errorf("failed to set topics; %w", err)
		}
	}

	if len(result.Entities) > 0 {
		entities := make([]graph.Entity, len(result.Entities))
		for i, e := range result.Entities {
			entities[i] = graph.Entity{Name: e.Name, Type: e.Type}
		}
		if err := s.graph.SetFileEntities(ctx, result.FilePath, entities); err != nil {
			return fmt.Errorf("failed to set entities; %w", err)
		}
	}

	if len(result.References) > 0 {
		refs := make([]graph.Reference, len(result.References))
		for i, r := range result.References {
			refs[i] = graph.Reference{Type: r.Type, Target: r.Target}
		}
		if err := s.graph.SetFileReferences(ctx, result.FilePath, refs); err != nil {
			return fmt.Errorf("failed to set references; %w", err)
		}
	}

	return nil
}

func analyzeSemantics(ctx context.Context, provider providers.SemanticProvider, chunks []chunkers.Chunk, logger *slog.Logger) (*SemanticResult, []string, error) {
	if len(chunks) == 0 {
		return &SemanticResult{}, nil, nil
	}

	logger = loggerOrDefault(logger)
	chunkSummaries := make([]string, len(chunks))

	if len(chunks) == 1 {
		result, err := analyzeChunk(ctx, provider, chunks[0])
		if err != nil {
			return nil, nil, err
		}
		chunkSummaries[chunks[0].Index] = result.Summary
		return result, chunkSummaries, nil
	}

	var chunkResults []*SemanticResult
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		result, err := analyzeChunk(ctx, provider, chunk)
		if err != nil {
			logger.Debug("chunk analysis failed", "chunk", chunk.Index, "error", err)
			continue
		}
		chunkResults = append(chunkResults, result)
		if chunk.Index >= 0 && chunk.Index < len(chunkSummaries) {
			chunkSummaries[chunk.Index] = result.Summary
		} else {
			logger.Warn("chunk index out of bounds for summary storage",
				"chunk_index", chunk.Index,
				"summaries_len", len(chunkSummaries))
		}
	}

	merged, err := mergeSemanticResults(ctx, provider, chunkResults)
	return merged, chunkSummaries, err
}

func analyzeChunk(ctx context.Context, provider providers.SemanticProvider, chunk chunkers.Chunk) (*SemanticResult, error) {
	if provider == nil {
		return nil, fmt.Errorf("semantic provider not configured")
	}

	req := providers.SemanticRequest{
		Content:  chunk.Content,
		MIMEType: "text/plain",
	}

	result, err := provider.Analyze(ctx, req)
	if err != nil {
		return nil, err
	}

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

func generateEmbeddings(ctx context.Context, provider providers.EmbeddingsProvider, cache *cache.EmbeddingsCache, logger *slog.Logger, chunks []chunkers.Chunk, chunkSummaries []string) ([]float32, []AnalyzedChunk, error) {
	if len(chunks) == 0 {
		return nil, nil, nil
	}

	logger = loggerOrDefault(logger)
	chunkResults := make([]AnalyzedChunk, len(chunks))
	var needsEmbedding []int

	for i, chunk := range chunks {
		chunkHash := fsutil.HashBytes([]byte(chunk.Content))
		chunkResults[i] = AnalyzedChunk{
			Index:       chunk.Index,
			Content:     chunk.Content,
			ContentHash: chunkHash,
			StartOffset: chunk.StartOffset,
			EndOffset:   chunk.EndOffset,
			ChunkType:   string(chunk.Metadata.Type),
			TokenCount:  chunk.Metadata.TokenEstimate,
			Metadata:    &chunk.Metadata,
		}
		if chunkSummaries != nil && chunk.Index < len(chunkSummaries) {
			chunkResults[i].Summary = chunkSummaries[chunk.Index]
		}

		if cache != nil {
			cached, err := cache.Get(chunkHash, chunk.Index)
			if err == nil {
				chunkResults[i].Embedding = cached.Embedding
				logger.Debug("embeddings cache hit", "chunk", i)
				continue
			}
		}
		needsEmbedding = append(needsEmbedding, i)
	}

	if len(needsEmbedding) > 0 {
		logger.Debug("generating embeddings for cache misses",
			"total_chunks", len(chunks),
			"cache_misses", len(needsEmbedding))

		texts := make([]string, len(needsEmbedding))
		for j, idx := range needsEmbedding {
			texts[j] = chunks[idx].Content
		}

		var embeddings []providers.EmbeddingsBatchResult
		var err error

		if len(texts) == 1 {
			req := providers.EmbeddingsRequest{Content: texts[0]}
			result, e := provider.Embed(ctx, req)
			if e != nil {
				return nil, nil, fmt.Errorf("embedding failed; %w", e)
			}
			embeddings = []providers.EmbeddingsBatchResult{{
				Index:     0,
				Embedding: result.Embedding,
			}}
		} else {
			embeddings, err = provider.EmbedBatch(ctx, texts)
			if err != nil {
				return nil, nil, fmt.Errorf("batch embeddings failed; %w", err)
			}
		}

		for j, emb := range embeddings {
			idx := needsEmbedding[j]
			chunkResults[idx].Embedding = emb.Embedding

			if cache != nil {
				cacheResult := &providers.EmbeddingsResult{
					Embedding:  emb.Embedding,
					Dimensions: len(emb.Embedding),
				}
				if err := cache.Set(chunkResults[idx].ContentHash, chunkResults[idx].Index, cacheResult); err != nil {
					logger.Warn("embeddings cache write error",
						"chunk", idx,
						"error", err)
				}
			}
		}
	}

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

func convertCachedSemantic(cached *providers.SemanticResult) *SemanticResult {
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

func convertToProviderSemantic(result *SemanticResult) *providers.SemanticResult {
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

func analysisVersionOrDefault(version string) string {
	if version == "" {
		return "1.0.0"
	}
	return version
}

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
