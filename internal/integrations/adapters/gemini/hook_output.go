package gemini

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations/shared"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const (
	// SessionStartEvent is the hook event name for Gemini CLI
	SessionStartEvent = "SessionStart"
)

// GeminiHookOutput represents the Gemini CLI SessionStart hook response format
type GeminiHookOutput struct {
	HookSpecificOutput *GeminiHookSpecificOutput `json:"hookSpecificOutput"`
}

// GeminiHookSpecificOutput contains the hook-specific output data
type GeminiHookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// formatGeminiHookJSON wraps the formatted file index in a Gemini CLI hook JSON envelope
func formatGeminiHookJSON(index *types.FileIndex, outputFormat integrations.OutputFormat) (string, error) {
	// Step 1: Get the appropriate formatter
	var formatStr string
	switch outputFormat {
	case integrations.FormatXML:
		formatStr = "xml"
	case integrations.FormatMarkdown:
		formatStr = "markdown"
	case integrations.FormatJSON:
		formatStr = "json"
	default:
		return "", fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	formatter, err := format.GetFormatter(formatStr)
	if err != nil {
		return "", fmt.Errorf("failed to get formatter; %w", err)
	}

	// Step 2: Format the file index content
	filesContent := format.NewFilesContent(index)
	content, err := formatter.Format(filesContent)
	if err != nil {
		return "", fmt.Errorf("failed to format index; %w", err)
	}

	// Step 3: Wrap the formatted content in Gemini CLI hook JSON envelope
	wrapper := GeminiHookOutput{
		HookSpecificOutput: &GeminiHookSpecificOutput{
			HookEventName:     SessionStartEvent,
			AdditionalContext: content,
		},
	}

	// Marshal to JSON with indentation and no HTML escaping
	jsonBytes, err := shared.MarshalIndentNoEscape(wrapper, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal hook JSON; %w", err)
	}

	return string(jsonBytes), nil
}
