package prompts

import (
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// BuildBinaryAnalysis creates a basic semantic analysis for binary files
// based solely on metadata (no content analysis).
// This is used as a fallback when content cannot be extracted or analyzed.
func BuildBinaryAnalysis(metadata *types.FileMetadata) *types.SemanticAnalysis {
	// Create summary from file type and metadata
	summary := fmt.Sprintf("%s file", strings.ToUpper(strings.TrimPrefix(metadata.Type, ".")))

	if metadata.PageCount != nil {
		summary += fmt.Sprintf(" with %d pages", *metadata.PageCount)
	} else if metadata.SlideCount != nil {
		summary += fmt.Sprintf(" with %d slides", *metadata.SlideCount)
	}

	return &types.SemanticAnalysis{
		Summary:      summary,
		Tags:         []string{metadata.Category, metadata.Type},
		KeyTopics:    []string{},
		DocumentType: metadata.Category,
		Confidence:   0.5, // Lower confidence for metadata-only analysis
		Entities:     []types.Entity{},
		References:   []types.Reference{},
	}
}
