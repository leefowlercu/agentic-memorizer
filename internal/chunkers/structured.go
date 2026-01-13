package chunkers

import (
	"context"
	"encoding/json"
	"strings"
)

const (
	structuredChunkerName     = "structured"
	structuredChunkerPriority = 40
)

// StructuredChunker splits structured data (JSON, YAML, CSV) by records.
type StructuredChunker struct{}

// NewStructuredChunker creates a new structured data chunker.
func NewStructuredChunker() *StructuredChunker {
	return &StructuredChunker{}
}

// Name returns the chunker's identifier.
func (c *StructuredChunker) Name() string {
	return structuredChunkerName
}

// CanHandle returns true for structured data content.
func (c *StructuredChunker) CanHandle(mimeType string, language string) bool {
	switch mimeType {
	case "application/json", "text/json":
		return true
	case "application/x-yaml", "text/yaml", "text/x-yaml":
		return true
	case "text/csv":
		return true
	}
	return false
}

// Priority returns the chunker's priority.
func (c *StructuredChunker) Priority() int {
	return structuredChunkerPriority
}

// Chunk splits structured content by records.
func (c *StructuredChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) ([]Chunk, error) {
	if len(content) == 0 {
		return []Chunk{}, nil
	}

	mimeType := opts.MIMEType
	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	switch {
	case strings.Contains(mimeType, "json"):
		return c.chunkJSON(ctx, content, maxSize)
	case strings.Contains(mimeType, "csv"):
		return c.chunkCSV(ctx, content, maxSize)
	default:
		// Fallback to line-based chunking for unknown structured formats
		return c.chunkLines(ctx, content, maxSize)
	}
}

// chunkJSON splits JSON content by array elements or object keys.
func (c *StructuredChunker) chunkJSON(ctx context.Context, content []byte, maxSize int) ([]Chunk, error) {
	// Try to parse as array
	var arr []json.RawMessage
	if err := json.Unmarshal(content, &arr); err == nil {
		return c.chunkJSONArray(ctx, arr, maxSize)
	}

	// Try to parse as object
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(content, &obj); err == nil {
		return c.chunkJSONObject(ctx, obj, content, maxSize)
	}

	// Fall back to treating as single chunk
	return []Chunk{{
		Index:       0,
		Content:     string(content),
		StartOffset: 0,
		EndOffset:   len(content),
		Metadata: ChunkMetadata{
			Type:          ChunkTypeStructured,
			TokenEstimate: EstimateTokens(string(content)),
		},
	}}, nil
}

// chunkJSONArray splits a JSON array into chunks of records.
func (c *StructuredChunker) chunkJSONArray(ctx context.Context, arr []json.RawMessage, maxSize int) ([]Chunk, error) {
	var chunks []Chunk
	var currentRecords []json.RawMessage
	currentSize := 2 // "[]"
	offset := 0

	for i, record := range arr {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		recordSize := len(record)
		if currentSize+recordSize+1 > maxSize && len(currentRecords) > 0 {
			chunk := c.createArrayChunk(currentRecords, len(chunks), offset)
			chunks = append(chunks, chunk)
			offset += len(chunk.Content)
			currentRecords = nil
			currentSize = 2
		}

		currentRecords = append(currentRecords, record)
		currentSize += recordSize + 1 // +1 for comma

		// Store record index in metadata
		if len(chunks) == 0 && i == 0 {
			// First chunk starts at record 0
		}
	}

	// Finalize remaining records
	if len(currentRecords) > 0 {
		chunk := c.createArrayChunk(currentRecords, len(chunks), offset)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// createArrayChunk creates a chunk from array records.
func (c *StructuredChunker) createArrayChunk(records []json.RawMessage, index, offset int) Chunk {
	// Re-marshal as array
	data, _ := json.Marshal(records)
	content := string(data)

	return Chunk{
		Index:       index,
		Content:     content,
		StartOffset: offset,
		EndOffset:   offset + len(content),
		Metadata: ChunkMetadata{
			Type:          ChunkTypeStructured,
			RecordIndex:   index,
			TokenEstimate: EstimateTokens(content),
		},
	}
}

// chunkJSONObject splits a JSON object by top-level keys.
func (c *StructuredChunker) chunkJSONObject(ctx context.Context, obj map[string]json.RawMessage, original []byte, maxSize int) ([]Chunk, error) {
	// If object fits in one chunk, return as-is
	if len(original) <= maxSize {
		return []Chunk{{
			Index:       0,
			Content:     string(original),
			StartOffset: 0,
			EndOffset:   len(original),
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(string(original)),
			},
		}}, nil
	}

	// Split by top-level keys
	var chunks []Chunk
	var currentKeys []string
	var currentVals []json.RawMessage
	currentSize := 2 // "{}"
	offset := 0

	for key, val := range obj {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		entrySize := len(key) + len(val) + 4 // "key":val,

		if currentSize+entrySize > maxSize && len(currentKeys) > 0 {
			chunk := c.createObjectChunk(currentKeys, currentVals, len(chunks), offset)
			chunks = append(chunks, chunk)
			offset += len(chunk.Content)
			currentKeys = nil
			currentVals = nil
			currentSize = 2
		}

		currentKeys = append(currentKeys, key)
		currentVals = append(currentVals, val)
		currentSize += entrySize
	}

	// Finalize remaining
	if len(currentKeys) > 0 {
		chunk := c.createObjectChunk(currentKeys, currentVals, len(chunks), offset)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// createObjectChunk creates a chunk from object entries.
func (c *StructuredChunker) createObjectChunk(keys []string, vals []json.RawMessage, index, offset int) Chunk {
	obj := make(map[string]json.RawMessage)
	for i, k := range keys {
		obj[k] = vals[i]
	}
	data, _ := json.Marshal(obj)
	content := string(data)

	return Chunk{
		Index:       index,
		Content:     content,
		StartOffset: offset,
		EndOffset:   offset + len(content),
		Metadata: ChunkMetadata{
			Type:          ChunkTypeStructured,
			RecordIndex:   index,
			TokenEstimate: EstimateTokens(content),
		},
	}
}

// chunkCSV splits CSV content by rows.
func (c *StructuredChunker) chunkCSV(ctx context.Context, content []byte, maxSize int) ([]Chunk, error) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return []Chunk{}, nil
	}

	// Keep header for context
	header := ""
	startIdx := 0
	if len(lines) > 0 && len(lines[0]) > 0 {
		header = lines[0] + "\n"
		startIdx = 1
	}

	var chunks []Chunk
	var current strings.Builder
	current.WriteString(header)
	offset := len(header)
	recordIndex := 0

	for i := startIdx; i < len(lines); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}

		lineLen := len(line) + 1 // +1 for newline
		if current.Len()+lineLen > maxSize && current.Len() > len(header) {
			content := current.String()
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     content,
				StartOffset: offset - current.Len(),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					RecordIndex:   recordIndex,
					TokenEstimate: EstimateTokens(content),
				},
			})
			current.Reset()
			current.WriteString(header)
			recordIndex = i
		}

		current.WriteString(line)
		current.WriteString("\n")
		offset += lineLen
	}

	// Finalize
	if current.Len() > len(header) {
		content := current.String()
		chunks = append(chunks, Chunk{
			Index:       len(chunks),
			Content:     content,
			StartOffset: offset - current.Len(),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				RecordIndex:   recordIndex,
				TokenEstimate: EstimateTokens(content),
			},
		})
	}

	return chunks, nil
}

// chunkLines splits content by lines.
func (c *StructuredChunker) chunkLines(ctx context.Context, content []byte, maxSize int) ([]Chunk, error) {
	lines := strings.Split(string(content), "\n")

	var chunks []Chunk
	var current strings.Builder
	offset := 0

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		lineLen := len(line) + 1
		if current.Len()+lineLen > maxSize && current.Len() > 0 {
			content := current.String()
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     content,
				StartOffset: offset - current.Len(),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(content),
				},
			})
			current.Reset()
		}

		current.WriteString(line)
		current.WriteString("\n")
		offset += lineLen
	}

	if current.Len() > 0 {
		content := current.String()
		chunks = append(chunks, Chunk{
			Index:       len(chunks),
			Content:     content,
			StartOffset: offset - current.Len(),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(content),
			},
		})
	}

	return chunks, nil
}
