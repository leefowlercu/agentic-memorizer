package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
	"github.com/leefowlercu/agentic-memorizer/internal/storage"
)

// PersistenceStage writes analysis results to the graph.
type PersistenceStage struct {
	graph  graph.Graph
	queue  storage.DurablePersistenceQueue
	logger *slog.Logger
}

// PersistenceStageOption configures a PersistenceStage.
type PersistenceStageOption func(*PersistenceStage)

// WithPersistenceQueue sets the durable queue for fallback when graph is unavailable.
func WithPersistenceQueue(q storage.DurablePersistenceQueue) PersistenceStageOption {
	return func(s *PersistenceStage) {
		s.queue = q
	}
}

// WithPersistenceLogger sets the logger for the persistence stage.
func WithPersistenceLogger(logger *slog.Logger) PersistenceStageOption {
	return func(s *PersistenceStage) {
		s.logger = logger
	}
}

// NewPersistenceStage creates a persistence stage.
func NewPersistenceStage(g graph.Graph, opts ...PersistenceStageOption) *PersistenceStage {
	s := &PersistenceStage{
		graph:  g,
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Persist writes analysis results to the graph. If the graph is unavailable or
// persistence fails, the result is queued for later retry (if a queue is configured).
func (s *PersistenceStage) Persist(ctx context.Context, result *AnalysisResult) error {
	logger := loggerOrDefault(s.logger)

	// If no graph configured and no queue, nothing to do
	if s.graph == nil && s.queue == nil {
		return nil
	}

	// Check if graph is available and connected
	graphAvailable := s.graph != nil && s.graph.IsConnected()

	// If graph not available, queue the result if possible
	if !graphAvailable {
		if s.queue != nil {
			logger.Info("graph unavailable; queuing for later persistence",
				"path", result.FilePath,
				"content_hash", result.ContentHash)
			return s.enqueueResult(ctx, result)
		}
		// No queue configured, nothing we can do
		return nil
	}

	// Graph is available, attempt direct persistence
	if err := s.persistToGraph(ctx, result); err != nil {
		// Persistence failed, try to queue for retry
		if s.queue != nil {
			logger.Warn("graph persistence failed; queuing for retry",
				"path", result.FilePath,
				"error", err)
			if qErr := s.enqueueResult(ctx, result); qErr != nil {
				return fmt.Errorf("persistence failed and queuing failed; persistence error: %w; queue error: %v", err, qErr)
			}
			return nil // Queued successfully, don't return the persistence error
		}
		return err
	}

	return nil
}

// enqueueResult serializes and enqueues an analysis result.
func (s *PersistenceStage) enqueueResult(ctx context.Context, result *AnalysisResult) error {
	resultJSON, err := storage.MarshalAnalysisResult(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result for queue; %w", err)
	}

	if err := s.queue.Enqueue(ctx, result.FilePath, result.ContentHash, resultJSON); err != nil {
		return fmt.Errorf("failed to enqueue result; %w", err)
	}

	return nil
}

// persistToGraph performs the actual persistence to the graph.
func (s *PersistenceStage) persistToGraph(ctx context.Context, result *AnalysisResult) error {
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
