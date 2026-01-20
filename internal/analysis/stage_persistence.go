package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
)

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
