package envelopes

import (
	"strings"
	"testing"
)

func testStats() *ExportStats {
	return &ExportStats{
		FileCount:         10,
		DirectoryCount:    5,
		ChunkCount:        100,
		TagCount:          8,
		TopicCount:        3,
		EntityCount:       12,
		RelationshipCount: 50,
		Format:            "xml",
		OutputSize:        5000,
	}
}

func TestNoneEnvelope(t *testing.T) {
	envelope := NewNoneEnvelope()

	t.Run("Name", func(t *testing.T) {
		if envelope.Name() != "none" {
			t.Errorf("Name() = %q, want %q", envelope.Name(), "none")
		}
	})

	t.Run("Description", func(t *testing.T) {
		if envelope.Description() == "" {
			t.Error("Description should not be empty")
		}
	})

	t.Run("Wrap", func(t *testing.T) {
		content := []byte("test content")
		stats := testStats()

		output, err := envelope.Wrap(content, stats)
		if err != nil {
			t.Fatalf("Wrap failed: %v", err)
		}

		// None envelope should return content unchanged
		if string(output) != string(content) {
			t.Errorf("Output = %q, want %q", string(output), string(content))
		}
	})
}

func TestClaudeCodeEnvelope(t *testing.T) {
	envelope := NewClaudeCodeEnvelope()

	t.Run("Name", func(t *testing.T) {
		if envelope.Name() != "claude-code" {
			t.Errorf("Name() = %q, want %q", envelope.Name(), "claude-code")
		}
	})

	t.Run("ContentType", func(t *testing.T) {
		if envelope.ContentType() != "text/plain" {
			t.Errorf("ContentType() = %q, want %q", envelope.ContentType(), "text/plain")
		}
	})

	t.Run("Description", func(t *testing.T) {
		if !strings.Contains(envelope.Description(), "Claude Code") {
			t.Error("Description should mention Claude Code")
		}
	})

	t.Run("Wrap", func(t *testing.T) {
		content := []byte("<test>data</test>")
		stats := testStats()

		output, err := envelope.Wrap(content, stats)
		if err != nil {
			t.Fatalf("Wrap failed: %v", err)
		}

		outputStr := string(output)

		// Check for header
		if !strings.Contains(outputStr, "# Memorizer Knowledge Graph") {
			t.Error("Output should contain header comment")
		}

		// Check for stats
		if !strings.Contains(outputStr, "10 files") {
			t.Error("Output should contain file count")
		}

		// Check for XML-style tags
		if !strings.Contains(outputStr, "<memorizer-knowledge-graph>") {
			t.Error("Output should contain opening tag")
		}
		if !strings.Contains(outputStr, "</memorizer-knowledge-graph>") {
			t.Error("Output should contain closing tag")
		}

		// Check content is included
		if !strings.Contains(outputStr, "<test>data</test>") {
			t.Error("Output should contain original content")
		}
	})
}

func TestGeminiCLIEnvelope(t *testing.T) {
	envelope := NewGeminiCLIEnvelope()

	t.Run("Name", func(t *testing.T) {
		if envelope.Name() != "gemini-cli" {
			t.Errorf("Name() = %q, want %q", envelope.Name(), "gemini-cli")
		}
	})

	t.Run("ContentType", func(t *testing.T) {
		if envelope.ContentType() != "text/plain" {
			t.Errorf("ContentType() = %q, want %q", envelope.ContentType(), "text/plain")
		}
	})

	t.Run("Description", func(t *testing.T) {
		if !strings.Contains(envelope.Description(), "Gemini CLI") {
			t.Error("Description should mention Gemini CLI")
		}
	})

	t.Run("Wrap", func(t *testing.T) {
		content := []byte("test data here")
		stats := testStats()

		output, err := envelope.Wrap(content, stats)
		if err != nil {
			t.Fatalf("Wrap failed: %v", err)
		}

		outputStr := string(output)

		// Check for markdown header
		if !strings.Contains(outputStr, "## Memorizer Knowledge Graph") {
			t.Error("Output should contain markdown header")
		}

		// Check for stats table
		if !strings.Contains(outputStr, "| Files | 10 |") {
			t.Error("Output should contain stats table with file count")
		}
		if !strings.Contains(outputStr, "| Directories | 5 |") {
			t.Error("Output should contain stats table with directory count")
		}

		// Check for code block
		if !strings.Contains(outputStr, "```\ntest data here\n```") {
			t.Error("Output should contain content in code block")
		}
	})
}

func TestEnvelopeInterface(t *testing.T) {
	// Verify all envelope types implement the interface
	var _ Envelope = (*NoneEnvelope)(nil)
	var _ Envelope = (*ClaudeCodeEnvelope)(nil)
	var _ Envelope = (*GeminiCLIEnvelope)(nil)
}
