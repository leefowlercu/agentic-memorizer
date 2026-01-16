package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTOMLChunker_Name(t *testing.T) {
	c := NewTOMLChunker()
	if c.Name() != "toml" {
		t.Errorf("expected name 'toml', got %q", c.Name())
	}
}

func TestTOMLChunker_Priority(t *testing.T) {
	c := NewTOMLChunker()
	if c.Priority() != 31 {
		t.Errorf("expected priority 31, got %d", c.Priority())
	}
}

func TestTOMLChunker_CanHandle(t *testing.T) {
	c := NewTOMLChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"application/toml", "", true},
		{"text/x-toml", "", true},
		{"", "toml", true},
		{"", "config.toml", true},
		{"", "Cargo.toml", true},
		{"", "pyproject.toml", true},
		{"text/plain", "", false},
		{"application/json", "", false},
		{"", "config.yaml", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestTOMLChunker_EmptyContent(t *testing.T) {
	c := NewTOMLChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "toml" {
		t.Errorf("expected chunker name 'toml', got %q", result.ChunkerUsed)
	}
}

func TestTOMLChunker_PreambleOnly(t *testing.T) {
	c := NewTOMLChunker()
	content := `# Configuration file
title = "My App"
version = "1.0.0"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Structured == nil {
			t.Fatal("expected Structured metadata")
		}
		// Preamble has empty table path
		if chunk.Metadata.Structured.TablePath != "" {
			t.Errorf("expected empty TablePath for preamble, got %q", chunk.Metadata.Structured.TablePath)
		}
		// Should have keys tracked
		if len(chunk.Metadata.Structured.KeyNames) < 2 {
			t.Errorf("expected at least 2 keys in preamble, got %d", len(chunk.Metadata.Structured.KeyNames))
		}
	}
}

func TestTOMLChunker_SingleTable(t *testing.T) {
	c := NewTOMLChunker()
	content := `[server]
host = "localhost"
port = 8080
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Structured == nil {
			t.Fatal("expected Structured metadata")
		}
		if chunk.Metadata.Structured.TablePath != "server" {
			t.Errorf("expected TablePath 'server', got %q", chunk.Metadata.Structured.TablePath)
		}
	}
}

func TestTOMLChunker_MultipleTables(t *testing.T) {
	c := NewTOMLChunker()
	content := `[server]
host = "localhost"
port = 8080

[database]
driver = "postgres"
host = "localhost"

[logging]
level = "info"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 separate chunks for 3 top-level tables
	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", result.TotalChunks)
	}

	// Verify table paths
	expectedPaths := []string{"server", "database", "logging"}
	for i, chunk := range result.Chunks {
		if i >= len(expectedPaths) {
			break
		}
		if chunk.Metadata.Structured.TablePath != expectedPaths[i] {
			t.Errorf("chunk %d: expected TablePath %q, got %q", i, expectedPaths[i], chunk.Metadata.Structured.TablePath)
		}
	}
}

func TestTOMLChunker_NestedTables(t *testing.T) {
	c := NewTOMLChunker()
	content := `[server]
host = "localhost"

[server.tls]
enabled = true
cert_file = "/etc/certs/server.crt"

[server.logging]
level = "debug"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nested tables under same top-level should be merged
	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk (nested tables merged), got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Structured.TablePath != "server" {
			t.Errorf("expected TablePath 'server', got %q", chunk.Metadata.Structured.TablePath)
		}
		// Content should include all nested tables
		if !strings.Contains(chunk.Content, "server.tls") {
			t.Error("expected chunk to contain nested server.tls table")
		}
		if !strings.Contains(chunk.Content, "server.logging") {
			t.Error("expected chunk to contain nested server.logging table")
		}
	}
}

func TestTOMLChunker_ArrayOfTables(t *testing.T) {
	c := NewTOMLChunker()
	content := `[[products]]
name = "Widget"
price = 9.99

[[products]]
name = "Gadget"
price = 19.99
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Arrays of tables with same name should be merged
	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk (array of tables merged), got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Structured.TablePath != "products" {
			t.Errorf("expected TablePath 'products', got %q", chunk.Metadata.Structured.TablePath)
		}
		// Content should include both array entries
		if !strings.Contains(chunk.Content, "Widget") {
			t.Error("expected chunk to contain first product")
		}
		if !strings.Contains(chunk.Content, "Gadget") {
			t.Error("expected chunk to contain second product")
		}
	}
}

func TestTOMLChunker_MixedContent(t *testing.T) {
	c := NewTOMLChunker()
	content := `# App configuration
title = "My App"

[server]
host = "localhost"

[[plugins]]
name = "auth"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have preamble included with first chunk, then server, then plugins
	if result.TotalChunks < 2 {
		t.Errorf("expected at least 2 chunks, got %d", result.TotalChunks)
	}
}

func TestTOMLChunker_Comments(t *testing.T) {
	c := NewTOMLChunker()
	content := `# This is a comment
[server]
# Port comment
port = 8080  # Inline comment
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Comments should be preserved
		if !strings.Contains(chunk.Content, "# Port comment") {
			t.Error("expected chunk to contain comments")
		}
	}
}

func TestTOMLChunker_KeyNames(t *testing.T) {
	c := NewTOMLChunker()
	content := `[database]
host = "localhost"
port = 5432
name = "mydb"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		keys := chunk.Metadata.Structured.KeyNames
		if len(keys) < 3 {
			t.Errorf("expected at least 3 keys, got %d", len(keys))
		}
	}
}

func TestTOMLChunker_ChunkType(t *testing.T) {
	c := NewTOMLChunker()
	content := `[test]
key = "value"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Type != ChunkTypeStructured {
			t.Errorf("expected ChunkTypeStructured, got %q", chunk.Metadata.Type)
		}
	}
}

func TestTOMLChunker_TestdataFixture(t *testing.T) {
	c := NewTOMLChunker()

	fixturePath := filepath.Join("..", "..", "testdata", "data", "sample.toml")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixture has: server (with nested tls), database, logging (with nested files), features, rate_limit
	// After merging nested tables: server, database, logging, features, rate_limit = 5 top-level
	if result.TotalChunks < 5 {
		t.Errorf("expected at least 5 chunks for fixture, got %d", result.TotalChunks)
	}

	// Verify some expected table paths
	foundServer := false
	foundDatabase := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Structured.TablePath == "server" {
			foundServer = true
		}
		if chunk.Metadata.Structured.TablePath == "database" {
			foundDatabase = true
		}
	}

	if !foundServer {
		t.Error("expected to find server table")
	}
	if !foundDatabase {
		t.Error("expected to find database table")
	}
}

func TestTOMLChunker_ContextCancellation(t *testing.T) {
	c := NewTOMLChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `[test]
key = "value"
`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestTOMLChunker_TokenEstimate(t *testing.T) {
	c := NewTOMLChunker()
	content := `[server]
host = "localhost"
port = 8080
description = "This is a longer description value for token estimation"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.TokenEstimate <= 0 {
			t.Error("expected positive TokenEstimate")
		}
	}
}

func TestTOMLChunker_LargeTable(t *testing.T) {
	c := NewTOMLChunker()

	// Create a large table that exceeds MaxChunkSize
	var builder strings.Builder
	builder.WriteString("[large]\n")
	for i := 0; i < 100; i++ {
		builder.WriteString("key" + string(rune('a'+i%26)) + " = \"value " + string(rune('0'+i%10)) + "\"\n")
	}

	opts := ChunkOptions{
		MaxChunkSize: 200,
		MaxTokens:    50,
	}

	result, err := c.Chunk(context.Background(), []byte(builder.String()), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Large table should be split into multiple chunks
	if result.TotalChunks < 2 {
		t.Errorf("expected large table to be split into multiple chunks, got %d", result.TotalChunks)
	}
}

func TestTOMLChunker_DottedKeys(t *testing.T) {
	c := NewTOMLChunker()
	content := `[server]
physical.host = "localhost"
logical.name = "main"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Dotted keys in values should be preserved in content
		if !strings.Contains(chunk.Content, "physical.host") {
			t.Error("expected chunk to contain dotted key")
		}
	}
}

func TestTOMLChunker_MultilineStrings(t *testing.T) {
	c := NewTOMLChunker()
	content := `[config]
description = """
This is a multiline
string value that spans
multiple lines.
"""
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Multiline string should be kept intact
		if !strings.Contains(chunk.Content, "multiple lines") {
			t.Error("expected chunk to contain complete multiline string")
		}
	}
}

func TestTOMLChunker_InlineTables(t *testing.T) {
	c := NewTOMLChunker()
	content := `[config]
point = { x = 1, y = 2 }
colors = ["red", "green", "blue"]
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Inline tables should be preserved
		if !strings.Contains(chunk.Content, "x = 1") {
			t.Error("expected chunk to contain inline table")
		}
	}
}

func TestTOMLChunker_LiteralStrings(t *testing.T) {
	c := NewTOMLChunker()
	content := `[paths]
# Literal strings don't escape backslashes
windows_path = 'C:\Users\name\Documents'
regex = '<\i\c*\s*>'
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Literal strings should be preserved
		if !strings.Contains(chunk.Content, `C:\Users`) {
			t.Error("expected chunk to contain literal string with backslashes")
		}
	}
}

func TestTOMLChunker_BooleanValues(t *testing.T) {
	c := NewTOMLChunker()
	content := `[features]
enabled = true
debug = false
experimental = true
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	if !strings.Contains(chunk.Content, "true") || !strings.Contains(chunk.Content, "false") {
		t.Error("expected chunk to contain boolean values")
	}
}

func TestTOMLChunker_IntegersWithUnderscores(t *testing.T) {
	c := NewTOMLChunker()
	content := `[limits]
max_size = 1_000_000
hex_value = 0xDEAD_BEEF
bin_value = 0b1101_0110
oct_value = 0o755
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Underscored integers should be preserved
		if !strings.Contains(chunk.Content, "1_000_000") {
			t.Error("expected chunk to contain underscored integer")
		}
	}
}

func TestTOMLChunker_DateTimeValues(t *testing.T) {
	c := NewTOMLChunker()
	content := `[timestamps]
# Various datetime formats
odt = 1979-05-27T07:32:00Z
ldt = 1979-05-27T07:32:00
ld = 1979-05-27
lt = 07:32:00
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Date/time values should be preserved
		if !strings.Contains(chunk.Content, "1979-05-27") {
			t.Error("expected chunk to contain date value")
		}
	}
}

func TestTOMLChunker_Arrays(t *testing.T) {
	c := NewTOMLChunker()
	content := `[data]
integers = [1, 2, 3]
floats = [1.0, 2.0, 3.5]
strings = ["alpha", "beta", "gamma"]
nested = [[1, 2], [3, 4]]
mixed = [1, "two", 3.0]
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Arrays should be preserved
		if !strings.Contains(chunk.Content, "[1, 2, 3]") {
			t.Error("expected chunk to contain integer array")
		}
		if !strings.Contains(chunk.Content, `["alpha", "beta", "gamma"]`) {
			t.Error("expected chunk to contain string array")
		}
	}
}

func TestTOMLChunker_EmptyTable(t *testing.T) {
	c := NewTOMLChunker()
	content := `[empty]

[non_empty]
key = "value"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle empty tables
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestTOMLChunker_UnicodeContent(t *testing.T) {
	c := NewTOMLChunker()
	content := `[i18n]
greeting_en = "Hello"
greeting_zh = "你好"
greeting_ja = "こんにちは"
greeting_emoji = "👋🌍"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Unicode should be preserved
		if !strings.Contains(chunk.Content, "你好") {
			t.Error("expected chunk to contain Chinese characters")
		}
		if !strings.Contains(chunk.Content, "👋") {
			t.Error("expected chunk to contain emoji")
		}
	}
}

func TestTOMLChunker_TableWithOnlyComments(t *testing.T) {
	c := NewTOMLChunker()
	content := `[config]
# This is a comment
# Another comment
# No actual keys here

[actual]
key = "value"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle tables with only comments
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestTOMLChunker_DeeplyNestedDottedKeys(t *testing.T) {
	c := NewTOMLChunker()
	content := `[deeply.nested.table]
config.sub.item = "value"
another.deep.key = 123
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Deeply dotted keys should be preserved
		if !strings.Contains(chunk.Content, "config.sub.item") {
			t.Error("expected chunk to contain deeply dotted key")
		}
	}
}

func TestTOMLChunker_EscapeSequences(t *testing.T) {
	c := NewTOMLChunker()
	content := `[strings]
backslash = "\\path\\to\\file"
newline = "line1\nline2"
tab = "col1\tcol2"
unicode = "\u0048\u0065\u006C\u006C\u006F"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Escape sequences should be preserved as-is in raw content
		if !strings.Contains(chunk.Content, `\n`) {
			t.Log("Note: newline escape may have been interpreted")
		}
	}
}

func TestTOMLChunker_FloatValues(t *testing.T) {
	c := NewTOMLChunker()
	content := `[numbers]
float1 = 3.14159
float2 = -0.001
exponent = 5e+22
negative_exp = 1e-10
inf = inf
neg_inf = -inf
nan = nan
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "3.14159") {
			t.Error("expected chunk to contain float value")
		}
		if !strings.Contains(chunk.Content, "inf") {
			t.Error("expected chunk to contain inf value")
		}
	}
}

func TestTOMLChunker_MultiLineBasicString(t *testing.T) {
	c := NewTOMLChunker()
	content := `[text]
multi = """
The quick brown fox
jumps over the lazy dog.
Multiple lines here."""
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "quick brown fox") {
			t.Error("expected chunk to contain multiline string content")
		}
	}
}

func TestTOMLChunker_MultiLineLiteralString(t *testing.T) {
	c := NewTOMLChunker()
	content := `[regex]
pattern = '''
^(\d{3})-(\d{3})-(\d{4})$
'''
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, `\d{3}`) {
			t.Error("expected chunk to contain literal regex pattern")
		}
	}
}

func TestTOMLChunker_WhitespaceOnlyContent(t *testing.T) {
	c := NewTOMLChunker()
	content := `


`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Whitespace-only should produce minimal chunks
	t.Logf("whitespace-only content produced %d chunks", result.TotalChunks)
}

func TestTOMLChunker_ArrayOfTablesWithDifferentPaths(t *testing.T) {
	c := NewTOMLChunker()
	content := `[[servers.production]]
ip = "10.0.0.1"
role = "frontend"

[[servers.production]]
ip = "10.0.0.2"
role = "backend"

[[servers.staging]]
ip = "192.168.1.1"
role = "all-in-one"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should merge arrays of tables under same top-level path
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestTOMLChunker_QuotedKeys(t *testing.T) {
	c := NewTOMLChunker()
	content := `["special.key"]
value = 1

["key with spaces"]
value = 2

[normal]
"quoted.dotted" = "value"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle quoted table names
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}
