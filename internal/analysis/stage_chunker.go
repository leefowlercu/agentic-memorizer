package analysis

import (
	"context"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
)

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
