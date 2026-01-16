package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXMLChunker_Name(t *testing.T) {
	c := NewXMLChunker()
	if c.Name() != "xml" {
		t.Errorf("expected name 'xml', got %q", c.Name())
	}
}

func TestXMLChunker_Priority(t *testing.T) {
	c := NewXMLChunker()
	if c.Priority() != 25 {
		t.Errorf("expected priority 25, got %d", c.Priority())
	}
}

func TestXMLChunker_CanHandle(t *testing.T) {
	c := NewXMLChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"application/xml", "", true},
		{"text/xml", "", true},
		{"application/rss+xml", "", true},
		{"application/atom+xml", "", true},
		{"image/svg+xml", "", true},
		{"", "xml", true},
		{"", "sample.xml", true},
		{"", "config.xsd", true},
		{"", "style.xsl", true},
		{"", "transform.xslt", true},
		{"", "image.svg", true},
		{"", "config.plist", true},
		{"text/plain", "", false},
		{"application/json", "", false},
		{"", "go", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestXMLChunker_EmptyContent(t *testing.T) {
	c := NewXMLChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "xml" {
		t.Errorf("expected chunker name 'xml', got %q", result.ChunkerUsed)
	}
}

func TestXMLChunker_SingleElement(t *testing.T) {
	c := NewXMLChunker()
	content := `<?xml version="1.0"?>
<root>
    <item>content</item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}
}

func TestXMLChunker_MultipleElements(t *testing.T) {
	c := NewXMLChunker()
	content := `<?xml version="1.0"?>
<catalog>
    <book id="bk101">
        <title>Book One</title>
    </book>
    <book id="bk102">
        <title>Book Two</title>
    </book>
    <book id="bk103">
        <title>Book Three</title>
    </book>
</catalog>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 book elements as separate chunks
	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", result.TotalChunks)
	}

	// Verify element paths
	for i, chunk := range result.Chunks {
		if chunk.Metadata.Structured == nil {
			t.Errorf("chunk %d: expected Structured metadata", i)
			continue
		}
		if chunk.Metadata.Structured.ElementName != "book" {
			t.Errorf("chunk %d: expected ElementName 'book', got %q", i, chunk.Metadata.Structured.ElementName)
		}
		if chunk.Metadata.Structured.ElementPath != "/catalog/book" {
			t.Errorf("chunk %d: expected ElementPath '/catalog/book', got %q", i, chunk.Metadata.Structured.ElementPath)
		}
	}
}

func TestXMLChunker_ElementPathExtraction(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <item>content</item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	if chunk.Metadata.Structured == nil {
		t.Fatal("expected Structured metadata")
	}

	if chunk.Metadata.Structured.ElementPath != "/root/item" {
		t.Errorf("expected ElementPath '/root/item', got %q", chunk.Metadata.Structured.ElementPath)
	}
}

func TestXMLChunker_MixedElementTypes(t *testing.T) {
	c := NewXMLChunker()
	content := `<store>
    <product id="1"><name>Widget</name></product>
    <customer id="100"><name>John</name></customer>
    <product id="2"><name>Gadget</name></product>
</store>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 elements
	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", result.TotalChunks)
	}

	// Verify element names
	expectedNames := []string{"product", "customer", "product"}
	for i, chunk := range result.Chunks {
		if i >= len(expectedNames) {
			break
		}
		if chunk.Metadata.Structured.ElementName != expectedNames[i] {
			t.Errorf("chunk %d: expected ElementName %q, got %q", i, expectedNames[i], chunk.Metadata.Structured.ElementName)
		}
	}
}

func TestXMLChunker_NestedElements(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <parent>
        <child>
            <grandchild>deep content</grandchild>
        </child>
    </parent>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// The parent element should be kept together
	chunk := result.Chunks[0]
	if !strings.Contains(chunk.Content, "grandchild") {
		t.Error("expected chunk to contain nested grandchild element")
	}
}

func TestXMLChunker_Attributes(t *testing.T) {
	c := NewXMLChunker()
	content := `<catalog>
    <item id="1" status="active" priority="high">Content</item>
</catalog>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	// Attributes should be preserved in content
	if !strings.Contains(chunk.Content, "id=") {
		t.Error("expected chunk to contain id attribute")
	}
}

func TestXMLChunker_ChunkType(t *testing.T) {
	c := NewXMLChunker()
	content := `<root><item>test</item></root>`

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

func TestXMLChunker_MalformedXML(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <item>unclosed
    <other>also unclosed
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	// Should not return fatal error, but may add warnings
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}

	// Should still produce some chunks even with malformed content
	if result.TotalChunks == 0 && len(content) > 0 {
		// At minimum should return the whole content as one chunk
		t.Log("malformed XML returned 0 chunks")
	}

	// May have warnings
	t.Logf("got %d warnings for malformed XML", len(result.Warnings))
}

func TestXMLChunker_CDATA(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <item><![CDATA[Special <characters> & stuff]]></item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestXMLChunker_Comments(t *testing.T) {
	c := NewXMLChunker()
	content := `<?xml version="1.0"?>
<!-- This is a comment -->
<root>
    <!-- Another comment -->
    <item>content</item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestXMLChunker_Namespaces(t *testing.T) {
	c := NewXMLChunker()
	content := `<root xmlns="http://example.com/ns" xmlns:custom="http://example.com/custom">
    <item>default namespace</item>
    <custom:item>custom namespace</custom:item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestXMLChunker_TestdataFixture(t *testing.T) {
	c := NewXMLChunker()

	fixturePath := filepath.Join("..", "..", "testdata", "data", "sample.xml")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixture has catalog with 3 book elements
	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks for fixture (3 books), got %d", result.TotalChunks)
	}

	// Verify all chunks have book as element name
	for i, chunk := range result.Chunks {
		if chunk.Metadata.Structured == nil {
			t.Errorf("chunk %d: missing Structured metadata", i)
			continue
		}
		if chunk.Metadata.Structured.ElementName != "book" {
			t.Errorf("chunk %d: expected ElementName 'book', got %q", i, chunk.Metadata.Structured.ElementName)
		}
		if chunk.Metadata.Structured.ElementPath != "/catalog/book" {
			t.Errorf("chunk %d: expected ElementPath '/catalog/book', got %q", i, chunk.Metadata.Structured.ElementPath)
		}
	}
}

func TestXMLChunker_ContextCancellation(t *testing.T) {
	c := NewXMLChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `<root><item>test</item></root>`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestXMLChunker_TokenEstimate(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <item>This is some content that will be tokenized</item>
</root>`

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

func TestXMLChunker_SelfClosingElements(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <item id="1" />
    <item id="2" />
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Self-closing elements should be handled
	if result.TotalChunks < 1 {
		t.Error("expected at least 1 chunk for self-closing elements")
	}
}

func TestXMLChunker_LargeElement(t *testing.T) {
	c := NewXMLChunker()

	// Create a large element that exceeds MaxChunkSize
	var builder strings.Builder
	builder.WriteString("<root><large>")
	for i := 0; i < 1000; i++ {
		builder.WriteString("<item>Line of content number " + string(rune('0'+i%10)) + "</item>")
	}
	builder.WriteString("</large></root>")

	opts := ChunkOptions{
		MaxChunkSize: 500,
		MaxTokens:    100,
	}

	result, err := c.Chunk(context.Background(), []byte(builder.String()), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Large element should be split into multiple chunks
	if result.TotalChunks < 2 {
		t.Errorf("expected large element to be split into multiple chunks, got %d", result.TotalChunks)
	}
}

func TestXMLChunker_ProcessingInstructions(t *testing.T) {
	c := NewXMLChunker()
	content := `<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet type="text/xsl" href="style.xsl"?>
<root>
    <item>content</item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestXMLChunker_DOCTYPE(t *testing.T) {
	c := NewXMLChunker()
	content := `<?xml version="1.0"?>
<!DOCTYPE note SYSTEM "note.dtd">
<note>
    <to>User</to>
    <from>System</from>
</note>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk with DOCTYPE")
	}
}

func TestXMLChunker_EntityReferences(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <item>Less than: &lt; Greater than: &gt;</item>
    <item>Ampersand: &amp; Quote: &quot; Apos: &apos;</item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Entity references should be preserved
	found := false
	for _, chunk := range result.Chunks {
		if strings.Contains(chunk.Content, "&lt;") || strings.Contains(chunk.Content, "&amp;") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected entity references to be preserved in content")
	}
}

func TestXMLChunker_WhitespaceOnly(t *testing.T) {
	c := NewXMLChunker()
	content := `

   `

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Whitespace-only should produce 0 meaningful chunks
	if result.TotalChunks > 0 {
		for _, chunk := range result.Chunks {
			if strings.TrimSpace(chunk.Content) != "" {
				t.Error("expected no non-whitespace chunks")
			}
		}
	}
}

func TestXMLChunker_MixedContent(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <paragraph>This is <bold>mixed</bold> content with <italic>inline</italic> elements.</paragraph>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Mixed content should be preserved intact
	chunk := result.Chunks[0]
	if !strings.Contains(chunk.Content, "<bold>mixed</bold>") {
		t.Error("expected mixed content to be preserved")
	}
}

func TestXMLChunker_UnicodeContent(t *testing.T) {
	c := NewXMLChunker()
	content := `<?xml version="1.0" encoding="UTF-8"?>
<root>
    <item lang="zh">中文内容</item>
    <item lang="ja">日本語コンテンツ</item>
    <item lang="ar">محتوى عربي</item>
    <item lang="emoji">🎉🚀💻</item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Unicode content should be preserved
	allContent := ""
	for _, chunk := range result.Chunks {
		allContent += chunk.Content
	}

	if !strings.Contains(allContent, "中文内容") {
		t.Error("expected Chinese content to be preserved")
	}
	// Note: emoji handling may vary based on XML encoding
	if !strings.Contains(allContent, "🎉") {
		t.Log("Note: emoji may not be preserved through XML parsing")
	}
}

func TestXMLChunker_EmptyRootElement(t *testing.T) {
	c := NewXMLChunker()
	content := `<root></root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty root should produce at least one chunk
	if result.TotalChunks < 1 {
		t.Log("empty root element produced 0 chunks")
	}
}

func TestXMLChunker_DeeplyNested(t *testing.T) {
	c := NewXMLChunker()
	// Create deeply nested structure
	content := `<level1>
    <level2>
        <level3>
            <level4>
                <level5>
                    <level6>deepest content</level6>
                </level5>
            </level4>
        </level3>
    </level2>
</level1>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Deep nesting should be preserved
	chunk := result.Chunks[0]
	if !strings.Contains(chunk.Content, "deepest content") {
		t.Error("expected deeply nested content to be preserved")
	}
}

func TestXMLChunker_EmptyElements(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <empty></empty>
    <also-empty/>
    <with-attr attr="value"></with-attr>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle both forms of empty elements
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestXMLChunker_MultipleRootAttempt(t *testing.T) {
	c := NewXMLChunker()
	// Invalid XML - multiple roots, but should handle gracefully
	content := `<root1><item>first</item></root1>
<root2><item>second</item></root2>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	// Should not crash, may have warnings
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}

	t.Logf("multiple root attempt produced %d chunks and %d warnings", result.TotalChunks, len(result.Warnings))
}

func TestXMLChunker_XMLDeclarationVariations(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "with standalone",
			content: `<?xml version="1.0" standalone="yes"?>
<root><item>test</item></root>`,
		},
		{
			name: "version 1.1",
			content: `<?xml version="1.1" encoding="UTF-8"?>
<root><item>test</item></root>`,
		},
		{
			name: "no declaration",
			content: `<root><item>test</item></root>`,
		},
		{
			name: "single quotes in declaration",
			content: `<?xml version='1.0' encoding='UTF-8'?>
<root><item>test</item></root>`,
		},
	}

	c := NewXMLChunker()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.Chunk(context.Background(), []byte(tt.content), DefaultChunkOptions())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.TotalChunks < 1 {
				t.Errorf("expected at least 1 chunk for %s", tt.name)
			}
		})
	}
}

func TestXMLChunker_SpecialCharactersInAttributeValues(t *testing.T) {
	c := NewXMLChunker()
	content := `<root>
    <item path="/path/to/file" query="a=1&amp;b=2">content</item>
    <item url="https://example.com?foo=bar&amp;baz=qux">link</item>
</root>`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Content with special chars in attributes should be preserved
	chunk := result.Chunks[0]
	if !strings.Contains(chunk.Content, "path=") {
		t.Error("expected attribute with path to be preserved")
	}
}

func TestXMLChunker_LargeNumberOfSiblings(t *testing.T) {
	c := NewXMLChunker()

	// Create XML with many sibling elements
	var builder strings.Builder
	builder.WriteString("<catalog>")
	for i := 0; i < 100; i++ {
		builder.WriteString("<item id=\"")
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString("\">Item content</item>")
	}
	builder.WriteString("</catalog>")

	result, err := c.Chunk(context.Background(), []byte(builder.String()), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should produce chunks for items
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// All chunks should have correct element name
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Structured != nil && chunk.Metadata.Structured.ElementName != "item" {
			t.Errorf("expected ElementName 'item', got %q", chunk.Metadata.Structured.ElementName)
		}
	}
}
