package analysis

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

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
// It modifies analyzedChunks in place to add embeddings to each chunk.
func (s *EmbeddingsStage) Generate(ctx context.Context, path string, analyzedChunks []AnalyzedChunk) ([]float32, error) {
	if s.provider == nil || !s.provider.Available() {
		return nil, nil
	}

	logger := loggerOrDefault(s.logger)
	fileEmbedding, embeddingsErr := generateEmbeddings(ctx, s.provider, s.cache, logger, analyzedChunks)

	if s.registry != nil {
		if err := s.registry.UpdateEmbeddingsState(ctx, path, embeddingsErr); err != nil {
			logger.Warn("failed to update embeddings state", "path", path, "error", err)
		}
	}

	return fileEmbedding, embeddingsErr
}

// generateEmbeddings generates embeddings for pre-built analyzed chunks.
// It modifies analyzedChunks in place to add embeddings to each chunk.
// Returns the file-level average embedding and any error.
func generateEmbeddings(ctx context.Context, provider providers.EmbeddingsProvider, embCache *cache.EmbeddingsCache, logger *slog.Logger, analyzedChunks []AnalyzedChunk) ([]float32, error) {
	if len(analyzedChunks) == 0 {
		return nil, nil
	}

	logger = loggerOrDefault(logger)
	var needsEmbedding []int

	// Check cache for existing embeddings
	for i := range analyzedChunks {
		if embCache != nil {
			cached, err := embCache.Get(analyzedChunks[i].ContentHash, analyzedChunks[i].Index)
			if err == nil {
				analyzedChunks[i].Embedding = cached.Embedding
				logger.Debug("embeddings cache hit", "chunk", i)
				continue
			}
		}
		needsEmbedding = append(needsEmbedding, i)
	}

	if len(needsEmbedding) > 0 {
		logger.Debug("generating embeddings for cache misses",
			"total_chunks", len(analyzedChunks),
			"cache_misses", len(needsEmbedding))

		texts := make([]string, len(needsEmbedding))
		for j, idx := range needsEmbedding {
			texts[j] = analyzedChunks[idx].Content
		}

		var embeddings []providers.EmbeddingsBatchResult
		var err error

		if len(texts) == 1 {
			req := providers.EmbeddingsRequest{Content: texts[0]}
			result, e := provider.Embed(ctx, req)
			if e != nil {
				return nil, fmt.Errorf("embedding failed; %w", e)
			}
			embeddings = []providers.EmbeddingsBatchResult{{
				Index:     0,
				Embedding: result.Embedding,
			}}
		} else {
			embeddings, err = provider.EmbedBatch(ctx, texts)
			if err != nil {
				return nil, fmt.Errorf("batch embeddings failed; %w", err)
			}
		}

		for j, emb := range embeddings {
			idx := needsEmbedding[j]
			analyzedChunks[idx].Embedding = emb.Embedding

			if embCache != nil {
				cacheResult := &providers.EmbeddingsResult{
					Embedding:  emb.Embedding,
					Dimensions: len(emb.Embedding),
				}
				if err := embCache.Set(analyzedChunks[idx].ContentHash, analyzedChunks[idx].Index, cacheResult); err != nil {
					logger.Warn("embeddings cache write error",
						"chunk", idx,
						"error", err)
				}
			}
		}
	}

	var allEmbeddings []providers.EmbeddingsBatchResult
	for i, ac := range analyzedChunks {
		if ac.Embedding != nil {
			allEmbeddings = append(allEmbeddings, providers.EmbeddingsBatchResult{
				Index:     i,
				Embedding: ac.Embedding,
			})
		}
	}

	fileEmbedding := averageEmbeddings(allEmbeddings)
	return fileEmbedding, nil
}
