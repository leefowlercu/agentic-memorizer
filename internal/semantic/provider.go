package semantic

import (
	"context"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Provider defines the interface for semantic analysis providers.
// Implementations provide AI-powered content understanding for different file types.
type Provider interface {
	// Analyze generates semantic understanding for a file.
	// Routes content through appropriate analysis method based on file type:
	// - Images → vision API (if supported)
	// - PDFs → document API (if supported) or text extraction
	// - Office docs → text extraction + analysis
	// - Text files → standard text analysis
	// - Binary files → metadata-only fallback
	Analyze(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error)

	// Name returns the provider identifier (e.g., "claude", "openai", "gemini")
	Name() string

	// Model returns the model being used (e.g., "claude-sonnet-4-5-20250929")
	Model() string

	// SupportsVision returns whether provider supports image analysis
	SupportsVision() bool

	// SupportsDocuments returns whether provider supports native document (PDF) blocks
	SupportsDocuments() bool
}
