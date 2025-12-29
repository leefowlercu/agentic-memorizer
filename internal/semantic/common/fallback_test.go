package common

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestAnalyzeBinary(t *testing.T) {
	tests := []struct {
		name            string
		metadata        *types.FileMetadata
		expectedSummary string
		expectedTags    []string
	}{
		{
			name: "basic binary file",
			metadata: &types.FileMetadata{
				FileInfo: types.FileInfo{
					Type:     "bin",
					Category: "other",
				},
			},
			expectedSummary: "BIN file",
			expectedTags:    []string{"other", "bin"},
		},
		{
			name: "pdf with page count",
			metadata: &types.FileMetadata{
				FileInfo: types.FileInfo{
					Type:     "pdf",
					Category: "documents",
				},
				PageCount: intPtr(10),
			},
			expectedSummary: "PDF file with 10 pages",
			expectedTags:    []string{"documents", "pdf"},
		},
		{
			name: "pptx with slide count",
			metadata: &types.FileMetadata{
				FileInfo: types.FileInfo{
					Type:     "pptx",
					Category: "documents",
				},
				SlideCount: intPtr(25),
			},
			expectedSummary: "PPTX file with 25 slides",
			expectedTags:    []string{"documents", "pptx"},
		},
		{
			name: "type with dot prefix",
			metadata: &types.FileMetadata{
				FileInfo: types.FileInfo{
					Type:     ".exe",
					Category: "other",
				},
			},
			expectedSummary: "EXE file",
			expectedTags:    []string{"other", ".exe"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeBinary(tt.metadata)

			if result.Summary != tt.expectedSummary {
				t.Errorf("Summary = %q, want %q", result.Summary, tt.expectedSummary)
			}

			if len(result.Tags) != len(tt.expectedTags) {
				t.Errorf("Tags length = %d, want %d", len(result.Tags), len(tt.expectedTags))
			} else {
				for i, tag := range result.Tags {
					if tag != tt.expectedTags[i] {
						t.Errorf("Tags[%d] = %q, want %q", i, tag, tt.expectedTags[i])
					}
				}
			}

			if result.Confidence != 0.5 {
				t.Errorf("Confidence = %f, want 0.5", result.Confidence)
			}

			if result.DocumentType != tt.metadata.Category {
				t.Errorf("DocumentType = %q, want %q", result.DocumentType, tt.metadata.Category)
			}

			if len(result.KeyTopics) != 0 {
				t.Errorf("KeyTopics should be empty, got %v", result.KeyTopics)
			}

			if len(result.Entities) != 0 {
				t.Errorf("Entities should be empty, got %v", result.Entities)
			}

			if len(result.References) != 0 {
				t.Errorf("References should be empty, got %v", result.References)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
