package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/storage"
)

// Pipeline orchestrates the analysis stages in sequence.
// It provides a clean separation between stage coordination and stage implementation.
type Pipeline struct {
	fileReader  FileReaderStage
	chunker     ChunkerStageInterface
	semantic    SemanticStageInterface
	embeddings  EmbeddingsStageInterface
	persistence PersistenceStageInterface
	logger      *slog.Logger

	semanticProvider providers.SemanticProvider

	// Registry for tracking file state
	registry registry.Registry

	// Analysis version for tracking schema changes
	analysisVersion string
}

// PipelineConfig holds all dependencies needed to construct a Pipeline.
// This provides a single configuration object for the component builder.
type PipelineConfig struct {
	Registry           registry.Registry
	ChunkerRegistry    *chunkers.Registry
	SemanticProvider   providers.SemanticProvider
	SemanticCache      *cache.SemanticCache
	EmbeddingsProvider providers.EmbeddingsProvider
	EmbeddingsCache    *cache.EmbeddingsCache
	Graph              graph.Graph
	PersistenceQueue   storage.DurablePersistenceQueue
	AnalysisVersion    string
	Logger             *slog.Logger
}

// PipelineOption configures a Pipeline.
type PipelineOption func(*Pipeline)

// WithFileReader replaces the default file reader stage (for testing).
func WithFileReader(r FileReaderStage) PipelineOption {
	return func(p *Pipeline) {
		p.fileReader = r
	}
}

// WithChunker replaces the default chunker stage (for testing).
func WithChunker(c ChunkerStageInterface) PipelineOption {
	return func(p *Pipeline) {
		p.chunker = c
	}
}

// WithSemantic replaces the default semantic stage (for testing).
func WithSemantic(s SemanticStageInterface) PipelineOption {
	return func(p *Pipeline) {
		p.semantic = s
	}
}

// WithEmbeddings replaces the default embeddings stage (for testing).
func WithEmbeddings(e EmbeddingsStageInterface) PipelineOption {
	return func(p *Pipeline) {
		p.embeddings = e
	}
}

// WithPersistence replaces the default persistence stage (for testing).
func WithPersistence(ps PersistenceStageInterface) PipelineOption {
	return func(p *Pipeline) {
		p.persistence = ps
	}
}

// WithPipelineLogger sets the logger for the pipeline.
func WithPipelineLogger(l *slog.Logger) PipelineOption {
	return func(p *Pipeline) {
		p.logger = l
	}
}

// NewPipeline creates a new analysis pipeline with the given configuration.
// Default stages are created from the config; options can override them for testing.
func NewPipeline(cfg PipelineConfig, opts ...PipelineOption) *Pipeline {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Build persistence stage with optional queue for fallback
	persistenceOpts := []PersistenceStageOption{WithPersistenceLogger(logger)}
	if cfg.PersistenceQueue != nil {
		persistenceOpts = append(persistenceOpts, WithPersistenceQueue(cfg.PersistenceQueue))
	}

	p := &Pipeline{
		fileReader:       NewFileReader(cfg.Registry),
		chunker:          NewChunkerStage(cfg.ChunkerRegistry),
		semantic:         NewSemanticStage(cfg.SemanticProvider, cfg.SemanticCache, cfg.Registry, cfg.AnalysisVersion, logger),
		embeddings:       NewEmbeddingsStage(cfg.EmbeddingsProvider, cfg.EmbeddingsCache, cfg.Registry, logger),
		persistence:      NewPersistenceStage(cfg.Graph, persistenceOpts...),
		logger:           logger,
		semanticProvider: cfg.SemanticProvider,
		registry:         cfg.Registry,
		analysisVersion:  cfg.AnalysisVersion,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Execute runs the analysis pipeline for the given context.
// It executes stages in order, populating the PipelineContext with results.
func (p *Pipeline) Execute(ctx context.Context, pctx *PipelineContext) error {
	// Stage 1: Read file and determine ingest mode
	fileResult, err := p.fileReader.Read(ctx, pctx.WorkItem, pctx.DegradationMode)
	if err != nil {
		return fmt.Errorf("file read stage failed; %w", err)
	}
	pctx.FileResult = fileResult

	// Build initial analysis result for metadata tracking
	pctx.AnalysisResult = pctx.BuildAnalysisResult()

	// Sync metadata state in registry
	p.syncMetadataState(ctx, pctx)

	// Early return for metadata-only or skip modes
	if pctx.IsMetadataOnly() || pctx.ShouldSkip() {
		p.updateRegistryForMetadataOnly(ctx, pctx)
		return nil
	}

	// Semantic-only files skip chunking/embeddings
	if pctx.IsSemanticOnly() {
		if p.semantic != nil {
			semanticStart := time.Now()
			input, buildErr := BuildSemanticInput(pctx.WorkItem.FilePath, pctx.FileResult, nil, p.semanticProvider)
			if buildErr != nil {
				p.logger.Warn("semantic input build failed",
					"path", pctx.WorkItem.FilePath,
					"error", buildErr)
			} else {
				semanticResult, semanticErr := p.semantic.Analyze(ctx, input, fileResult.ContentHash)
				if semanticErr != nil {
					p.logger.Warn("semantic analysis failed",
						"path", pctx.WorkItem.FilePath,
						"error", semanticErr,
						"duration", time.Since(semanticStart))
				}
				pctx.SemanticResult = semanticResult
			}
		}
		p.updateRegistryForSemanticOnly(ctx, pctx)
		pctx.AnalysisResult = pctx.BuildAnalysisResult()
		return nil
	}

	// Stage 2: Chunk content
	chunkResult, err := p.chunker.Chunk(ctx, fileResult.Content, fileResult.MIMEType, fileResult.Language)
	if err != nil {
		return fmt.Errorf("chunking stage failed; %w", err)
	}
	pctx.ChunkResult = chunkResult

	// Build analyzed chunks structure
	pctx.AnalyzedChunks = BuildAnalyzedChunks(chunkResult.Chunks)

	// Stage 3: Semantic analysis (optional)
	if p.semantic != nil {
		semanticStart := time.Now()
		input, buildErr := BuildSemanticInput(pctx.WorkItem.FilePath, pctx.FileResult, chunkResult, p.semanticProvider)
		if buildErr != nil {
			p.logger.Warn("semantic input build failed",
				"path", pctx.WorkItem.FilePath,
				"error", buildErr)
		} else {
			semanticResult, semanticErr := p.semantic.Analyze(ctx, input, fileResult.ContentHash)
			if semanticErr != nil {
				p.logger.Warn("semantic analysis failed",
					"path", pctx.WorkItem.FilePath,
					"error", semanticErr,
					"duration", time.Since(semanticStart))
				// Continue pipeline - semantic failures are non-fatal
			}
			pctx.SemanticResult = semanticResult
		}
	}

	// Stage 4: Embeddings generation (conditional)
	if pctx.ShouldGenerateEmbeddings() && p.embeddings != nil {
		embeddingsStart := time.Now()
		embeddings, embeddingsErr := p.embeddings.Generate(ctx, pctx.WorkItem.FilePath, pctx.AnalyzedChunks)
		if embeddingsErr != nil {
			p.logger.Warn("embeddings generation failed",
				"path", pctx.WorkItem.FilePath,
				"error", embeddingsErr,
				"duration", time.Since(embeddingsStart))
			// Continue pipeline - embeddings failures are non-fatal
		} else {
			pctx.Embeddings = embeddings
		}
	}

	// Build final analysis result
	pctx.AnalysisResult = pctx.BuildAnalysisResult()

	return nil
}

// Persist writes the analysis result to the graph.
// This is separated from Execute to allow the caller to control when persistence happens.
func (p *Pipeline) Persist(ctx context.Context, pctx *PipelineContext) error {
	if p.persistence == nil || pctx.AnalysisResult == nil {
		return nil
	}
	return p.persistence.Persist(ctx, pctx.AnalysisResult)
}

// syncMetadataState updates the registry with current file metadata.
func (p *Pipeline) syncMetadataState(ctx context.Context, pctx *PipelineContext) {
	if p.registry == nil || pctx.AnalysisResult == nil {
		return
	}

	result := pctx.AnalysisResult

	// Check if content has changed (requires clearing previous analysis state)
	existingState, err := p.registry.GetFileState(ctx, result.FilePath)
	if err == nil && existingState.ContentHash != result.ContentHash {
		p.logger.Debug("content changed; clearing analysis state",
			"path", result.FilePath,
			"old_hash", existingState.ContentHash[:8],
			"new_hash", result.ContentHash[:8])
		if clearErr := p.registry.ClearAnalysisState(ctx, result.FilePath); clearErr != nil {
			p.logger.Warn("failed to clear analysis state", "path", result.FilePath, "error", clearErr)
		}
	}

	if err := p.registry.UpdateMetadataState(ctx, result.FilePath, result.ContentHash, result.MetadataHash, result.FileSize, result.ModTime); err != nil {
		p.logger.Warn("failed to update metadata state", "path", result.FilePath, "error", err)
	}
}

// updateRegistryForMetadataOnly updates registry state for files that only have metadata extracted.
func (p *Pipeline) updateRegistryForMetadataOnly(ctx context.Context, pctx *PipelineContext) {
	if p.registry == nil {
		return
	}

	// Don't update if in degraded metadata mode (file would normally be chunked)
	if pctx.FileResult != nil && pctx.FileResult.DegradedMetadata {
		return
	}

	result := pctx.AnalysisResult
	if result == nil {
		return
	}

	version := analysisVersionOrDefault(p.analysisVersion)
	if err := p.registry.UpdateSemanticState(ctx, result.FilePath, version, nil); err != nil {
		p.logger.Warn("failed to update semantic state", "path", result.FilePath, "error", err)
	}
	if err := p.registry.UpdateEmbeddingsState(ctx, result.FilePath, nil); err != nil {
		p.logger.Warn("failed to update embeddings state", "path", result.FilePath, "error", err)
	}
}

// updateRegistryForSemanticOnly marks embeddings as completed for semantic-only files.
func (p *Pipeline) updateRegistryForSemanticOnly(ctx context.Context, pctx *PipelineContext) {
	if p.registry == nil || pctx.AnalysisResult == nil {
		return
	}

	if err := p.registry.UpdateEmbeddingsState(ctx, pctx.AnalysisResult.FilePath, nil); err != nil {
		p.logger.Warn("failed to update embeddings state", "path", pctx.AnalysisResult.FilePath, "error", err)
	}
}

// GetIngestMode returns the determined ingest mode from the pipeline context.
// This is useful for publishing events with the correct analysis type.
func (p *Pipeline) GetIngestMode(pctx *PipelineContext) ingest.Mode {
	if pctx.FileResult == nil {
		return ingest.ModeSkip
	}
	return pctx.FileResult.IngestMode
}
