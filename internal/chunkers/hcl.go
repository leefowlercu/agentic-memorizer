package chunkers

import (
	"context"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

const (
	hclChunkerName     = "hcl"
	hclChunkerPriority = 43
)

// HCLChunker splits HCL/Terraform content by top-level blocks.
type HCLChunker struct{}

// NewHCLChunker creates a new HCL chunker.
func NewHCLChunker() *HCLChunker {
	return &HCLChunker{}
}

// Name returns the chunker's identifier.
func (c *HCLChunker) Name() string {
	return hclChunkerName
}

// CanHandle returns true for HCL content (Terraform, HCL files).
func (c *HCLChunker) CanHandle(mimeType string, language string) bool {
	lang := strings.ToLower(language)
	mime := strings.ToLower(mimeType)

	// Match by MIME type
	if mime == "text/x-hcl" ||
		mime == "application/x-hcl" ||
		mime == "text/x-terraform" ||
		mime == "application/x-terraform" {
		return true
	}

	// Match by filename pattern
	if strings.HasSuffix(lang, ".tf") ||
		strings.HasSuffix(lang, ".tfvars") ||
		strings.HasSuffix(lang, ".hcl") {
		return true
	}

	return false
}

// Priority returns the chunker's priority.
func (c *HCLChunker) Priority() int {
	return hclChunkerPriority
}

// Chunk splits HCL content by top-level blocks.
func (c *HCLChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  hclChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	// Parse HCL content
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(content, "input.hcl")

	var warnings []ChunkWarning
	if diags.HasErrors() {
		// Convert HCL diagnostics to warnings
		for _, diag := range diags {
			if diag.Severity == hcl.DiagError {
				offset := 0
				if diag.Subject != nil {
					offset = diag.Subject.Start.Byte
				}
				warnings = append(warnings, ChunkWarning{
					Offset:  offset,
					Message: diag.Summary + ": " + diag.Detail,
					Code:    "HCL_PARSE_ERROR",
				})
			}
		}
		// If we can't parse at all, fall back to simple splitting
		if file == nil {
			return c.fallbackChunk(ctx, content, opts, warnings)
		}
	}

	blocks := c.extractBlocks(file, content)

	var chunks []Chunk
	offset := 0

	for _, block := range blocks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If block is too large, split it
		if len(block.content) > maxSize {
			subChunks := c.splitLargeBlock(ctx, block, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(block.content) != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     block.content,
				StartOffset: offset,
				EndOffset:   offset + len(block.content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(block.content),
					Infra: &InfraMetadata{
						BlockType:    block.blockType,
						ResourceType: block.resourceType,
						ResourceName: block.resourceName,
					},
				},
			})
		}

		offset += len(block.content)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  hclChunkerName,
		OriginalSize: len(content),
	}, nil
}

// hclBlock represents a parsed HCL block.
type hclBlock struct {
	blockType    string
	resourceType string
	resourceName string
	content      string
	startByte    int
	endByte      int
}

// extractBlocks extracts top-level blocks from an HCL file.
func (c *HCLChunker) extractBlocks(file *hcl.File, content []byte) []hclBlock {
	var blocks []hclBlock

	// Define the schema for Terraform/HCL blocks we want to extract
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "terraform"},
			{Type: "provider", LabelNames: []string{"name"}},
			{Type: "variable", LabelNames: []string{"name"}},
			{Type: "output", LabelNames: []string{"name"}},
			{Type: "locals"},
			{Type: "data", LabelNames: []string{"type", "name"}},
			{Type: "resource", LabelNames: []string{"type", "name"}},
			{Type: "module", LabelNames: []string{"name"}},
		},
	}

	body := file.Body
	hclBody, _ := body.Content(schema)

	if hclBody == nil || len(hclBody.Blocks) == 0 {
		// If we can't extract blocks, return whole content as one block
		blocks = append(blocks, hclBlock{
			blockType: "unknown",
			content:   string(content),
			startByte: 0,
			endByte:   len(content),
		})
		return blocks
	}

	// Sort blocks by position and extract their content
	for _, block := range hclBody.Blocks {
		blockType := block.Type
		var resourceType, resourceName string

		// Extract labels based on block type
		switch blockType {
		case "resource", "data":
			if len(block.Labels) >= 2 {
				resourceType = block.Labels[0]
				resourceName = block.Labels[1]
			} else if len(block.Labels) >= 1 {
				resourceType = block.Labels[0]
			}
		case "provider", "variable", "output", "module":
			if len(block.Labels) >= 1 {
				resourceName = block.Labels[0]
			}
		}

		// Get the block content from source
		startByte := block.DefRange.Start.Byte
		endByte := block.DefRange.End.Byte

		// Try to get the actual end of the block (including body)
		if block.Body != nil {
			// Get the range of the body
			bodyRange := block.Body.MissingItemRange()
			if bodyRange.End.Byte > endByte {
				endByte = bodyRange.End.Byte
			}
		}

		// Find the actual end of the block by looking for the closing brace
		endByte = c.findBlockEnd(content, startByte)

		blockContent := string(content[startByte:endByte])

		blocks = append(blocks, hclBlock{
			blockType:    blockType,
			resourceType: resourceType,
			resourceName: resourceName,
			content:      blockContent,
			startByte:    startByte,
			endByte:      endByte,
		})
	}

	// If no blocks were found, treat as single block
	if len(blocks) == 0 {
		blocks = append(blocks, hclBlock{
			blockType: "unknown",
			content:   string(content),
			startByte: 0,
			endByte:   len(content),
		})
	}

	return blocks
}

// findBlockEnd finds the end of an HCL block by tracking braces.
func (c *HCLChunker) findBlockEnd(content []byte, start int) int {
	braceCount := 0
	inString := false
	escaped := false
	foundFirstBrace := false

	for i := start; i < len(content); i++ {
		ch := content[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch ch {
		case '{':
			braceCount++
			foundFirstBrace = true
		case '}':
			braceCount--
			if foundFirstBrace && braceCount == 0 {
				return i + 1
			}
		}
	}

	return len(content)
}

// splitLargeBlock splits a large HCL block into smaller chunks.
func (c *HCLChunker) splitLargeBlock(ctx context.Context, block hclBlock, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk
	var current strings.Builder
	offset := baseOffset

	lines := strings.Split(block.content, "\n")

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
				StartOffset: offset - len(content),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(content),
					Infra: &InfraMetadata{
						BlockType:    block.blockType,
						ResourceType: block.resourceType,
						ResourceName: block.resourceName,
					},
				},
			})
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
		offset += len(line) + 1
	}

	// Finalize last chunk
	if current.Len() > 0 {
		content := current.String()
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset - len(content),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(content),
				Infra: &InfraMetadata{
					BlockType:    block.blockType,
					ResourceType: block.resourceType,
					ResourceName: block.resourceName,
				},
			},
		})
	}

	return chunks
}

// fallbackChunk creates chunks when HCL parsing fails.
func (c *HCLChunker) fallbackChunk(ctx context.Context, content []byte, opts ChunkOptions, warnings []ChunkWarning) (*ChunkResult, error) {
	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	var chunks []Chunk

	// Simple line-based splitting
	var current strings.Builder
	offset := 0

	for _, line := range strings.Split(text, "\n") {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if current.Len()+len(line)+1 > maxSize && current.Len() > 0 {
			chunkContent := current.String()
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     chunkContent,
				StartOffset: offset - len(chunkContent),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(chunkContent),
					Infra: &InfraMetadata{
						BlockType: "unknown",
					},
				},
			})
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
		offset += len(line) + 1
	}

	if current.Len() > 0 {
		chunkContent := current.String()
		chunks = append(chunks, Chunk{
			Index:       len(chunks),
			Content:     chunkContent,
			StartOffset: offset - len(chunkContent),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(chunkContent),
				Infra: &InfraMetadata{
					BlockType: "unknown",
				},
			},
		})
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  hclChunkerName,
		OriginalSize: len(content),
	}, nil
}
