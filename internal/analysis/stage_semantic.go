package analysis

import (
	"context"
	"errors"
	"log/slog"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

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
			logger.Debug("semantic cache hit", "path", path, "content_hash", contentHash[:12])
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
		return nil, errors.New("semantic provider not configured")
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
