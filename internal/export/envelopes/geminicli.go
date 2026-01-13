package envelopes

import (
	"bytes"
	"fmt"
)

// GeminiCLIEnvelope wraps content for Gemini CLI SessionStart hooks.
type GeminiCLIEnvelope struct{}

// NewGeminiCLIEnvelope creates a new Gemini CLI envelope.
func NewGeminiCLIEnvelope() *GeminiCLIEnvelope {
	return &GeminiCLIEnvelope{}
}

// Name returns the envelope name.
func (e *GeminiCLIEnvelope) Name() string {
	return "gemini-cli"
}

// ContentType returns the MIME content type.
func (e *GeminiCLIEnvelope) ContentType() string {
	return "text/plain"
}

// Description returns a human-readable description.
func (e *GeminiCLIEnvelope) Description() string {
	return "Gemini CLI SessionStart hook envelope"
}

// Wrap wraps content in Gemini CLI SessionStart format.
// Uses a markdown-friendly format that Gemini can parse.
func (e *GeminiCLIEnvelope) Wrap(content []byte, stats *ExportStats) ([]byte, error) {
	var buf bytes.Buffer

	// Write header in markdown format
	buf.WriteString("## Memorizer Knowledge Graph\n\n")
	buf.WriteString("The following is an automatically generated knowledge graph export from the Memorizer daemon.\n\n")

	// Write stats table
	buf.WriteString("### Statistics\n\n")
	buf.WriteString("| Metric | Count |\n")
	buf.WriteString("|--------|-------|\n")
	fmt.Fprintf(&buf, "| Files | %d |\n", stats.FileCount)
	fmt.Fprintf(&buf, "| Directories | %d |\n", stats.DirectoryCount)
	fmt.Fprintf(&buf, "| Chunks | %d |\n", stats.ChunkCount)
	fmt.Fprintf(&buf, "| Tags | %d |\n", stats.TagCount)
	fmt.Fprintf(&buf, "| Topics | %d |\n", stats.TopicCount)
	fmt.Fprintf(&buf, "| Entities | %d |\n", stats.EntityCount)
	buf.WriteString("\n")

	// Write content in code block
	buf.WriteString("### Knowledge Graph Data\n\n")
	buf.WriteString("```\n")
	buf.Write(content)
	buf.WriteString("\n```\n")

	return buf.Bytes(), nil
}
