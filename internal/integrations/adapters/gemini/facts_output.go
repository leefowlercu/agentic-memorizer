package gemini

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const (
	// BeforeAgentEvent is the hook event name for facts injection
	BeforeAgentEvent = "BeforeAgent"
)

// BeforeAgentOutput represents the Gemini CLI BeforeAgent hook response format
type BeforeAgentOutput struct {
	HookSpecificOutput *GeminiHookSpecificOutput `json:"hookSpecificOutput"`
}

// formatBeforeAgentJSON wraps the formatted facts index in a Gemini CLI BeforeAgent JSON envelope
func formatBeforeAgentJSON(facts *types.FactsIndex, outputFormat integrations.OutputFormat) (string, error) {
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

	// Step 2: Format the facts content
	factsContent := format.NewFactsContent(facts)
	content, err := formatter.Format(factsContent)
	if err != nil {
		return "", fmt.Errorf("failed to format facts; %w", err)
	}

	// Step 3: Wrap the formatted content in BeforeAgent JSON envelope
	wrapper := BeforeAgentOutput{
		HookSpecificOutput: &GeminiHookSpecificOutput{
			HookEventName:     BeforeAgentEvent,
			AdditionalContext: content,
		},
	}

	// Marshal to JSON with indentation and no HTML escaping
	jsonBytes, err := marshalIndentNoEscape(wrapper, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal BeforeAgent JSON; %w", err)
	}

	return string(jsonBytes), nil
}
