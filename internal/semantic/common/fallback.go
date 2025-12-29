package common

import (
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// AnalyzeBinary creates a metadata-only analysis for binary/unsupported files.
// Returns a basic SemanticAnalysis with lower confidence score.
func AnalyzeBinary(metadata *types.FileMetadata) *types.SemanticAnalysis {
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
