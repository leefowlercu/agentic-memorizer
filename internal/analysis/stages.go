package analysis

import (
	"log/slog"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/fsutil"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
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

// BuildAnalyzedChunks converts raw chunker output to analyzed chunks.
// This creates the data structure without generating embeddings.
func BuildAnalyzedChunks(chunks []chunkers.Chunk) []AnalyzedChunk {
	if len(chunks) == 0 {
		return nil
	}

	result := make([]AnalyzedChunk, len(chunks))
	for i, chunk := range chunks {
		chunkHash := fsutil.HashBytes([]byte(chunk.Content))
		result[i] = AnalyzedChunk{
			Index:       chunk.Index,
			Content:     chunk.Content,
			ContentHash: chunkHash,
			StartOffset: chunk.StartOffset,
			EndOffset:   chunk.EndOffset,
			ChunkType:   string(chunk.Metadata.Type),
			TokenCount:  chunk.Metadata.TokenEstimate,
			Metadata:    &chunk.Metadata,
		}
	}
	return result
}

// EnhanceChunksWithSummaries adds semantic summaries to pre-built chunks.
func EnhanceChunksWithSummaries(chunks []AnalyzedChunk, summaries []string) {
	if summaries == nil {
		return
	}
	for i := range chunks {
		if chunks[i].Index < len(summaries) {
			chunks[i].Summary = summaries[chunks[i].Index]
		}
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
