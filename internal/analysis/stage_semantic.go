package analysis

import (
	"context"
	"errors"
	"log/slog"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/fsutil"
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
func (s *SemanticStage) Analyze(ctx context.Context, input providers.SemanticInput, contentHash string) (*SemanticResult, error) {
	if s.provider == nil || !s.provider.Available() {
		return nil, nil
	}

	logger := loggerOrDefault(s.logger)
	var semanticResult *SemanticResult
	var semanticErr error
	cacheHit := false

	cacheKey := semanticCacheKey(contentHash, input.Type, s.provider.ModelName())

	if s.cache != nil {
		cachedResult, err := s.cache.Get(cacheKey)
		if err == nil {
			semanticResult = convertCachedSemantic(cachedResult)
			cacheHit = true
			hashPreview := contentHash
			if len(hashPreview) > 12 {
				hashPreview = hashPreview[:12]
			}
			logger.Debug("semantic cache hit", "path", input.Path, "content_hash", hashPreview)
		} else if !errors.Is(err, cache.ErrCacheMiss) {
			logger.Warn("semantic cache read error", "path", input.Path, "error", err)
		}
	}

	if !cacheHit {
		providerResult, err := s.provider.Analyze(ctx, input)
		if err != nil {
			semanticErr = err
		} else if providerResult != nil {
			semanticResult = convertProviderSemantic(providerResult)
		}

		if semanticErr == nil && s.cache != nil && providerResult != nil {
			if cacheErr := s.cache.Set(cacheKey, providerResult); cacheErr != nil {
				logger.Warn("semantic cache write error", "path", input.Path, "error", cacheErr)
			}
		}
	}

	if s.registry != nil {
		version := analysisVersionOrDefault(s.analysisVersion)
		if err := s.registry.UpdateSemanticState(ctx, input.Path, version, semanticErr); err != nil {
			logger.Warn("failed to update semantic state", "path", input.Path, "error", err)
		}
	}

	return semanticResult, semanticErr
}

func semanticCacheKey(contentHash string, inputType providers.SemanticInputType, model string) string {
	key := contentHash + ":" + string(inputType)
	if model != "" {
		key += ":" + model
	}
	return fsutil.HashBytes([]byte(key))
}

func convertProviderSemantic(result *providers.SemanticResult) *SemanticResult {
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
	}
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
