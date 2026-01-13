package envelopes

import (
	"bytes"
	"fmt"
)

// ClaudeCodeEnvelope wraps content for Claude Code SessionStart hooks.
type ClaudeCodeEnvelope struct{}

// NewClaudeCodeEnvelope creates a new Claude Code envelope.
func NewClaudeCodeEnvelope() *ClaudeCodeEnvelope {
	return &ClaudeCodeEnvelope{}
}

// Name returns the envelope name.
func (e *ClaudeCodeEnvelope) Name() string {
	return "claude-code"
}

// ContentType returns the MIME content type.
func (e *ClaudeCodeEnvelope) ContentType() string {
	return "text/plain"
}

// Description returns a human-readable description.
func (e *ClaudeCodeEnvelope) Description() string {
	return "Claude Code SessionStart hook envelope"
}

// Wrap wraps content in Claude Code SessionStart format.
// The format uses XML-style tags that Claude Code recognizes.
func (e *ClaudeCodeEnvelope) Wrap(content []byte, stats *ExportStats) ([]byte, error) {
	var buf bytes.Buffer

	// Write header comment
	buf.WriteString("# Memorizer Knowledge Graph Export\n")
	buf.WriteString("# This content is automatically injected by the Memorizer daemon.\n\n")

	// Write stats summary
	fmt.Fprintf(&buf, "# Stats: %d files, %d directories, %d chunks, %d tags, %d topics, %d entities\n\n",
		stats.FileCount,
		stats.DirectoryCount,
		stats.ChunkCount,
		stats.TagCount,
		stats.TopicCount,
		stats.EntityCount)

	// Write knowledge graph tag
	buf.WriteString("<memorizer-knowledge-graph>\n")
	buf.Write(content)
	buf.WriteString("\n</memorizer-knowledge-graph>\n")

	return buf.Bytes(), nil
}
