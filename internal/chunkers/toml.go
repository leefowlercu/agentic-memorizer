package chunkers

import (
	"context"
	"regexp"
	"strings"
)

const (
	tomlChunkerName     = "toml"
	tomlChunkerPriority = 31
)

// Regex patterns for TOML parsing
var (
	// Matches [table] or [[array.of.tables]]
	tomlTableRegex = regexp.MustCompile(`^\s*\[+([^\]]+)\]+\s*$`)
	// Matches key = value
	tomlKeyValueRegex = regexp.MustCompile(`^\s*([a-zA-Z0-9_-]+(?:\.[a-zA-Z0-9_-]+)*)\s*=`)
)

// TOMLChunker splits TOML content by top-level tables.
type TOMLChunker struct{}

// NewTOMLChunker creates a new TOML chunker.
func NewTOMLChunker() *TOMLChunker {
	return &TOMLChunker{}
}

// Name returns the chunker's identifier.
func (c *TOMLChunker) Name() string {
	return tomlChunkerName
}

// CanHandle returns true for TOML content.
func (c *TOMLChunker) CanHandle(mimeType string, language string) bool {
	mime := strings.ToLower(mimeType)
	lang := strings.ToLower(language)

	// Match by MIME type
	if mime == "application/toml" || mime == "text/x-toml" {
		return true
	}

	// Match by file extension
	if strings.HasSuffix(lang, ".toml") {
		return true
	}

	// Match by language hint
	if lang == "toml" {
		return true
	}

	return false
}

// Priority returns the chunker's priority.
func (c *TOMLChunker) Priority() int {
	return tomlChunkerPriority
}

// Chunk splits TOML content by top-level tables.
func (c *TOMLChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  tomlChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	tables := c.parseTomlTables(string(content))

	var chunks []Chunk
	offset := 0

	for _, table := range tables {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If table block is too large, split it further
		if len(table.content) > maxSize {
			subChunks := c.splitLargeTable(ctx, table, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(table.content) != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     table.content,
				StartOffset: offset,
				EndOffset:   offset + len(table.content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(table.content),
					Structured: &StructuredMetadata{
						TablePath: table.path,
						KeyNames:  table.keys,
					},
				},
			})
		}

		offset += len(table.content)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     nil,
		TotalChunks:  len(chunks),
		ChunkerUsed:  tomlChunkerName,
		OriginalSize: len(content),
	}, nil
}

// tomlTable represents a section in a TOML file.
type tomlTable struct {
	path    string   // Table path (e.g., "server.tls")
	content string   // Full content including header
	keys    []string // Keys defined in this table
	isArray bool     // True if [[array.of.tables]]
}

// parseTomlTables splits TOML content into tables.
func (c *TOMLChunker) parseTomlTables(text string) []tomlTable {
	lines := strings.Split(text, "\n")
	var tables []tomlTable

	var preamble strings.Builder
	var currentTable *tomlTable
	var currentContent strings.Builder
	var currentKeys []string

	flushCurrent := func() {
		if currentTable != nil {
			currentTable.content = currentContent.String()
			currentTable.keys = currentKeys
			tables = append(tables, *currentTable)
			currentContent.Reset()
			currentKeys = nil
		}
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments in structure detection
		isEmpty := trimmed == "" || strings.HasPrefix(trimmed, "#")

		// Check for table header
		if match := tomlTableRegex.FindStringSubmatch(trimmed); match != nil {
			flushCurrent()

			tablePath := strings.TrimSpace(match[1])
			isArray := strings.HasPrefix(trimmed, "[[")

			// Get top-level table name for grouping
			topLevel := strings.Split(tablePath, ".")[0]

			// If we have preamble and this is first table, include preamble
			if len(tables) == 0 && preamble.Len() > 0 {
				currentContent.WriteString(preamble.String())
			}

			currentTable = &tomlTable{
				path:    topLevel,
				isArray: isArray,
			}
			currentContent.WriteString(line)
			if i < len(lines)-1 {
				currentContent.WriteString("\n")
			}
			continue
		}

		// Check for key definition (for metadata)
		if !isEmpty && currentTable != nil {
			if match := tomlKeyValueRegex.FindStringSubmatch(trimmed); match != nil {
				key := match[1]
				// Only track direct keys (not dotted paths that belong elsewhere)
				if !strings.Contains(key, ".") {
					currentKeys = append(currentKeys, key)
				}
			}
		}

		// Accumulate content
		if currentTable != nil {
			currentContent.WriteString(line)
			if i < len(lines)-1 {
				currentContent.WriteString("\n")
			}
		} else {
			// Before first table - this is preamble (top-level keys)
			preamble.WriteString(line)
			preamble.WriteString("\n")

			// Track preamble keys
			if !isEmpty {
				if match := tomlKeyValueRegex.FindStringSubmatch(trimmed); match != nil {
					key := match[1]
					if !strings.Contains(key, ".") {
						currentKeys = append(currentKeys, key)
					}
				}
			}
		}
	}

	// Flush last table
	flushCurrent()

	// If there's preamble but no tables, make preamble a chunk
	if len(tables) == 0 && preamble.Len() > 0 {
		tables = append(tables, tomlTable{
			path:    "",
			content: strings.TrimSuffix(preamble.String(), "\n"),
			keys:    currentKeys,
		})
	}

	// Merge tables with the same top-level path
	tables = c.mergeTables(tables)

	return tables
}

// mergeTables merges consecutive tables with the same top-level path.
func (c *TOMLChunker) mergeTables(tables []tomlTable) []tomlTable {
	if len(tables) <= 1 {
		return tables
	}

	// Group tables by top-level path
	groups := make(map[string][]tomlTable)
	var order []string

	for _, table := range tables {
		topLevel := table.path
		if topLevel == "" {
			topLevel = "_preamble"
		}

		if _, exists := groups[topLevel]; !exists {
			order = append(order, topLevel)
		}
		groups[topLevel] = append(groups[topLevel], table)
	}

	// Merge each group
	var merged []tomlTable
	for _, path := range order {
		group := groups[path]
		if len(group) == 1 {
			merged = append(merged, group[0])
			continue
		}

		// Merge multiple tables into one
		var content strings.Builder
		var keys []string
		keySet := make(map[string]bool)

		for _, t := range group {
			content.WriteString(t.content)
			for _, k := range t.keys {
				if !keySet[k] {
					keys = append(keys, k)
					keySet[k] = true
				}
			}
		}

		displayPath := path
		if path == "_preamble" {
			displayPath = ""
		}

		merged = append(merged, tomlTable{
			path:    displayPath,
			content: content.String(),
			keys:    keys,
			isArray: group[0].isArray,
		})
	}

	return merged
}

// splitLargeTable splits a large TOML table into smaller chunks.
func (c *TOMLChunker) splitLargeTable(ctx context.Context, table tomlTable, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk
	var current strings.Builder
	offset := baseOffset

	lines := strings.Split(table.content, "\n")
	var currentKeys []string

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		// If adding this line exceeds max, finalize current chunk
		if current.Len()+len(line)+1 > maxSize && current.Len() > 0 {
			content := current.String()
			chunks = append(chunks, Chunk{
				Content:     content,
				StartOffset: offset,
				EndOffset:   offset + len(content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(content),
					Structured: &StructuredMetadata{
						TablePath: table.path,
						KeyNames:  currentKeys,
					},
				},
			})
			offset += len(content)
			current.Reset()
			currentKeys = nil
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)

		// Track keys in this chunk
		trimmed := strings.TrimSpace(line)
		if match := tomlKeyValueRegex.FindStringSubmatch(trimmed); match != nil {
			key := match[1]
			if !strings.Contains(key, ".") {
				currentKeys = append(currentKeys, key)
			}
		}
	}

	// Finalize last chunk
	if current.Len() > 0 {
		content := current.String()
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset,
			EndOffset:   offset + len(content),
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(content),
				Structured: &StructuredMetadata{
					TablePath: table.path,
					KeyNames:  currentKeys,
				},
			},
		})
	}

	return chunks
}
