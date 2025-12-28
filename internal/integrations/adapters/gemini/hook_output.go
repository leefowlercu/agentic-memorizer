package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
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
	jsonBytes, err := marshalIndentNoEscape(wrapper, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal hook JSON; %w", err)
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
