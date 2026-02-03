package analysis

import (
	"context"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

// FileReaderStage defines the interface for the file reading stage.
// It reads file metadata, determines ingest mode, and computes content hashes.
type FileReaderStage interface {
	Read(ctx context.Context, item WorkItem, mode DegradationMode) (*FileReadResult, error)
}

// ChunkerStageInterface defines the interface for the chunking stage.
// It splits file content into semantic chunks using format-specific chunkers.
type ChunkerStageInterface interface {
	Chunk(ctx context.Context, content []byte, mimeType, language string) (*chunkers.ChunkResult, error)
}

// SemanticStageInterface defines the interface for the semantic analysis stage.
// It analyzes file-level inputs using AI providers to extract summaries, topics, entities, etc.
type SemanticStageInterface interface {
	Analyze(ctx context.Context, input providers.SemanticInput, contentHash string) (*SemanticResult, error)
}

// EmbeddingsStageInterface defines the interface for the embeddings generation stage.
// It generates vector embeddings for chunks and modifies them in place.
type EmbeddingsStageInterface interface {
	Generate(ctx context.Context, path string, analyzedChunks []AnalyzedChunk) ([]float32, error)
}

// PersistenceStageInterface defines the interface for the graph persistence stage.
// It writes analysis results to the knowledge graph.
type PersistenceStageInterface interface {
	Persist(ctx context.Context, result *AnalysisResult) error
}

// Compile-time interface assertions for existing stage implementations.
var (
	_ FileReaderStage           = (*FileReader)(nil)
	_ ChunkerStageInterface     = (*ChunkerStage)(nil)
	_ SemanticStageInterface    = (*SemanticStage)(nil)
	_ EmbeddingsStageInterface  = (*EmbeddingsStage)(nil)
	_ PersistenceStageInterface = (*PersistenceStage)(nil)
)
