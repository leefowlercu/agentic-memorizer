package common

import (
	"encoding/json"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// ParseAnalysisResponse parses an LLM response into a SemanticAnalysis struct.
// It handles both raw JSON and JSON wrapped in code blocks.
func ParseAnalysisResponse(response string) (*types.SemanticAnalysis, error) {
	var analysis types.SemanticAnalysis

	// Try parsing as raw JSON first
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		// Try extracting JSON from code blocks
		if jsonStr := ExtractJSON(response); jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &analysis); err == nil {
				return &analysis, nil
			}
		}
		return nil, fmt.Errorf("failed to parse analysis response; %w", err)
	}

	return &analysis, nil
}
