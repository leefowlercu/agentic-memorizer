package claude

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations/output"
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

// formatSessionStartJSON wraps the formatted index in a Claude Code SessionStart JSON envelope
func formatSessionStartJSON(index *types.Index, format integrations.OutputFormat) (string, error) {
	// Step 1: Generate formatted content using the appropriate output processor
	var content string
	var err error

	switch format {
	case integrations.FormatXML:
		processor := output.NewXMLProcessor()
		content, err = processor.Format(index)
	case integrations.FormatMarkdown:
		processor := output.NewMarkdownProcessor()
		content, err = processor.Format(index)
	case integrations.FormatJSON:
		processor := output.NewJSONProcessor()
		content, err = processor.Format(index)
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}

	if err != nil {
		return "", fmt.Errorf("failed to format index: %w", err)
	}

	// Step 2: Wrap the formatted content in SessionStart JSON envelope
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
	jsonBytes, err := marshalIndentNoEscape(wrapper, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal SessionStart JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// marshalIndentNoEscape marshals JSON with indentation but without HTML escaping.
// This prevents <, >, and & from being escaped to \u003c, \u003e, and \u0026.
// Unlike json.MarshalIndent which hard-codes escapeHTML: true, this function uses
// an Encoder with SetEscapeHTML(false) to produce cleaner output for CLI contexts.
func marshalIndentNoEscape(v any, prefix, indent string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	// Encoder.Encode adds a trailing newline, trim it to match MarshalIndent behavior
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}

// generateSystemMessage creates a concise system message for the SessionStart hook
func generateSystemMessage(index *types.Index) string {
	categories := groupByCategory(index.Entries)
	categoryCount := len(categories)

	msg := fmt.Sprintf("Memory index updated: %d files", index.Stats.TotalFiles)

	if categoryCount > 0 {
		categoryParts := []string{}
		categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
		for _, category := range categoryOrder {
			if entries, ok := categories[category]; ok && len(entries) > 0 {
				categoryParts = append(categoryParts, fmt.Sprintf("%d %s", len(entries), category))
			}
		}
		if len(categoryParts) > 0 {
			msg += fmt.Sprintf(" (%s)", join(categoryParts, ", "))
		}
	}

	msg += fmt.Sprintf(", %s total", formatSize(index.Stats.TotalSize))

	if index.Stats.CachedFiles > 0 || index.Stats.AnalyzedFiles > 0 {
		msg += fmt.Sprintf(" — %d cached, %d analyzed", index.Stats.CachedFiles, index.Stats.AnalyzedFiles)
	}

	return msg
}

// groupByCategory groups index entries by their category
func groupByCategory(entries []types.IndexEntry) map[string][]types.IndexEntry {
	categories := make(map[string][]types.IndexEntry)
	for _, entry := range entries {
		category := entry.Metadata.Category
		categories[category] = append(categories[category], entry)
	}
	return categories
}

// formatSize formats bytes into human-readable size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// join concatenates strings with a separator
func join(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}
