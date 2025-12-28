package claude

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations/shared"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// SessionStartOutput represents the Claude Code SessionStart hook response format
type SessionStartOutput struct {
	Continue       bool                `json:"continue"`
	SuppressOutput bool                `json:"suppressOutput"`
	SystemMessage  string              `json:"systemMessage,omitempty"`
	HookSpecific   *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput contains the hook-specific output data
type HookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// formatSessionStartJSON wraps the formatted file index in a Claude Code SessionStart JSON envelope
func formatSessionStartJSON(index *types.FileIndex, outputFormat integrations.OutputFormat) (string, error) {
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

	// Step 3: Wrap the formatted content in SessionStart JSON envelope
	wrapper := SessionStartOutput{
		Continue:       true,
		SuppressOutput: true,
		SystemMessage:  generateSystemMessage(index),
		HookSpecific: &HookSpecificOutput{
			HookEventName:     SessionStartEvent,
			AdditionalContext: content,
		},
	}

	// Marshal to JSON with indentation and no HTML escaping
	jsonBytes, err := shared.MarshalIndentNoEscape(wrapper, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal SessionStart JSON; %w", err)
	}

	return string(jsonBytes), nil
}

// generateSystemMessage creates a concise system message for the SessionStart hook
func generateSystemMessage(index *types.FileIndex) string {
	categories := shared.GroupByCategory(index.Files)
	categoryCount := len(categories)

	msg := fmt.Sprintf("Memory index updated: %d files", index.Stats.TotalFiles)

	if categoryCount > 0 {
		categoryParts := []string{}
		categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
		for _, category := range categoryOrder {
			if files, ok := categories[category]; ok && len(files) > 0 {
				categoryParts = append(categoryParts, fmt.Sprintf("%d %s", len(files), category))
			}
		}
		if len(categoryParts) > 0 {
			msg += fmt.Sprintf(" (%s)", shared.Join(categoryParts, ", "))
		}
	}

	msg += fmt.Sprintf(", %s total", shared.FormatSize(index.Stats.TotalSize))

	if index.Stats.CachedFiles > 0 || index.Stats.AnalyzedFiles > 0 {
		msg += fmt.Sprintf(" — %d cached, %d analyzed", index.Stats.CachedFiles, index.Stats.AnalyzedFiles)
	}

	return msg
}
