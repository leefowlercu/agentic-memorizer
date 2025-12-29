package claude

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations/shared"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// UserPromptSubmitOutput represents the Claude Code UserPromptSubmit hook response format
type UserPromptSubmitOutput struct {
	Continue       bool                `json:"continue"`
	SuppressOutput bool                `json:"suppressOutput"`
	SystemMessage  string              `json:"systemMessage,omitempty"`
	HookSpecific   *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// formatUserPromptSubmitJSON wraps the formatted facts index in a Claude Code UserPromptSubmit JSON envelope
func formatUserPromptSubmitJSON(facts *types.FactsIndex, outputFormat integrations.OutputFormat) (string, error) {
	// Step 1: Get the appropriate formatter
	formatStr, err := shared.OutputFormatToString(outputFormat)
	if err != nil {
		return "", err
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

	// Step 3: Wrap the formatted content in UserPromptSubmit JSON envelope
	wrapper := UserPromptSubmitOutput{
		Continue:       true,
		SuppressOutput: true,
		SystemMessage:  generateFactsSystemMessage(facts),
		HookSpecific: &HookSpecificOutput{
			HookEventName:     UserPromptSubmitEvent,
			AdditionalContext: content,
		},
	}

	// Marshal to JSON with indentation and no HTML escaping
	jsonBytes, err := shared.MarshalIndentNoEscape(wrapper, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal UserPromptSubmit JSON; %w", err)
	}

	return string(jsonBytes), nil
}

// generateFactsSystemMessage creates a concise system message for the UserPromptSubmit hook
func generateFactsSystemMessage(facts *types.FactsIndex) string {
	count := len(facts.Facts)
	if count == 0 {
		return "No user-defined facts stored"
	}

	return fmt.Sprintf("User facts loaded: %d/%d", count, facts.Stats.MaxFacts)
}
