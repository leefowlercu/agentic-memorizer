package analysis

import (
	"log/slog"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
)

// PipelineContext carries data between pipeline stages using an accumulator pattern.
// Each stage reads from and writes to this context, allowing data to flow through
// the pipeline without requiring direct coupling between stages.
type PipelineContext struct {
	// Input data
	WorkItem        WorkItem
	DegradationMode DegradationMode

	// Stage outputs (populated progressively as pipeline executes)
	FileResult     *FileReadResult
	ChunkResult    *chunkers.ChunkResult
	AnalyzedChunks []AnalyzedChunk
	SemanticResult *SemanticResult
	Embeddings     []float32
	AnalysisResult *AnalysisResult

	// Processing metadata
	StartTime time.Time
	Logger    *slog.Logger
}

// NewPipelineContext creates a new pipeline context for processing a work item.
func NewPipelineContext(item WorkItem, mode DegradationMode, logger *slog.Logger) *PipelineContext {
	return &PipelineContext{
		WorkItem:        item,
		DegradationMode: mode,
		StartTime:       time.Now(),
		Logger:          loggerOrDefault(logger),
	}
}

// BuildAnalysisResult constructs the final AnalysisResult from accumulated stage outputs.
// This should be called after all stages have executed to assemble the complete result.
func (p *PipelineContext) BuildAnalysisResult() *AnalysisResult {
	if p.FileResult == nil {
		return nil
	}

	result := &AnalysisResult{
		FilePath:     p.WorkItem.FilePath,
		FileSize:     p.FileResult.Info.Size(),
		ModTime:      p.FileResult.Info.ModTime(),
		MIMEType:     p.FileResult.MIMEType,
		Language:     p.FileResult.Language,
		IngestKind:   p.FileResult.Kind,
		IngestMode:   p.FileResult.IngestMode,
		IngestReason: p.FileResult.IngestReason,
		ContentHash:  p.FileResult.ContentHash,
		MetadataHash: p.FileResult.MetadataHash,
		AnalyzedAt:   time.Now(),
	}

	// Add chunk information if chunking was performed
	if p.ChunkResult != nil {
		result.ChunkerUsed = p.ChunkResult.ChunkerUsed
		result.ChunksProcessed = p.ChunkResult.TotalChunks
	}

	// Add semantic analysis results
	if p.SemanticResult != nil {
		result.Summary = p.SemanticResult.Summary
		result.Tags = p.SemanticResult.Tags
		result.Topics = p.SemanticResult.Topics
		result.Entities = p.SemanticResult.Entities
		result.References = p.SemanticResult.References
		result.Complexity = p.SemanticResult.Complexity
		result.Keywords = p.SemanticResult.Keywords
	}

	// Add embeddings (file-level average)
	result.Embeddings = p.Embeddings

	// Add per-chunk data
	result.Chunks = p.AnalyzedChunks

	// Calculate processing time
	result.ProcessingTime = time.Since(p.StartTime)

	return result
}

// IsMetadataOnly returns true if the file should only have metadata extracted.
func (p *PipelineContext) IsMetadataOnly() bool {
	if p.FileResult == nil {
		return false
	}
	return p.FileResult.IngestMode == ingest.ModeMetadataOnly
}

// ShouldSkip returns true if the file should be skipped entirely.
func (p *PipelineContext) ShouldSkip() bool {
	if p.FileResult == nil {
		return false
	}
	return p.FileResult.IngestMode == ingest.ModeSkip
}

// IsSemanticOnly returns true if the file should receive semantic analysis without chunking.
func (p *PipelineContext) IsSemanticOnly() bool {
	if p.FileResult == nil {
		return false
	}
	return p.FileResult.IngestMode == ingest.ModeSemanticOnly
}

// ShouldChunk returns true if the file should be chunked and fully analyzed.
func (p *PipelineContext) ShouldChunk() bool {
	if p.FileResult == nil {
		return false
	}
	return p.FileResult.IngestMode == ingest.ModeChunk
}

// ShouldGenerateEmbeddings returns true if embeddings should be generated.
// This checks both the ingest mode and the degradation mode.
func (p *PipelineContext) ShouldGenerateEmbeddings() bool {
	if !p.ShouldChunk() {
		return false
	}
	// Skip embeddings in both DegradationNoEmbed and DegradationMetadata modes
	return p.DegradationMode == DegradationFull
}
