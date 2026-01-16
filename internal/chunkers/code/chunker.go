package code

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
)

const (
	treeSitterChunkerName     = "treesitter"
	treeSitterChunkerPriority = 100 // Same as old ASTChunker
)

// TreeSitterChunker uses tree-sitter to parse and chunk source code
// across multiple programming languages.
type TreeSitterChunker struct {
	registry *StrategyRegistry
}

// NewTreeSitterChunker creates a new tree-sitter based chunker.
func NewTreeSitterChunker() *TreeSitterChunker {
	return &TreeSitterChunker{
		registry: NewStrategyRegistry(),
	}
}

// RegisterStrategy adds a language strategy to the chunker.
func (c *TreeSitterChunker) RegisterStrategy(strategy LanguageStrategy) {
	c.registry.Register(strategy)
}

// Name returns the chunker's identifier.
func (c *TreeSitterChunker) Name() string {
	return treeSitterChunkerName
}

// CanHandle returns true for supported programming languages.
func (c *TreeSitterChunker) CanHandle(mimeType string, language string) bool {
	return c.registry.CanHandle(mimeType, language)
}

// Priority returns the chunker's priority.
func (c *TreeSitterChunker) Priority() int {
	return treeSitterChunkerPriority
}

// Chunk parses source code and splits it by AST structure.
func (c *TreeSitterChunker) Chunk(ctx context.Context, content []byte, opts chunkers.ChunkOptions) (*chunkers.ChunkResult, error) {
	if len(content) == 0 {
		return &chunkers.ChunkResult{
			Chunks:       []chunkers.Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  treeSitterChunkerName,
			OriginalSize: 0,
		}, nil
	}

	// Find strategy for this content
	strategy := c.registry.Resolve(opts.MIMEType, opts.Language)
	if strategy == nil {
		return nil, fmt.Errorf("no tree-sitter strategy for mime=%s lang=%s", opts.MIMEType, opts.Language)
	}

	// Parse with tree-sitter
	parser := sitter.NewParser()
	parser.SetLanguage(strategy.GetLanguage())

	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse failed; %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return nil, fmt.Errorf("tree-sitter produced nil root node")
	}

	// Check for parse errors
	var warnings []chunkers.ChunkWarning
	if root.HasError() {
		warnings = append(warnings, chunkers.ChunkWarning{
			Offset:  0,
			Message: "source contains parse errors",
			Code:    "PARSE_ERROR",
		})
	}

	// Extract chunks from AST
	chunkList, err := c.extractChunks(ctx, root, content, strategy, opts)
	if err != nil {
		return nil, err
	}

	// If no chunks extracted, return whole file as single chunk
	if len(chunkList) == 0 {
		contentStr := string(content)
		chunkList = []chunkers.Chunk{{
			Index:       0,
			Content:     contentStr,
			StartOffset: 0,
			EndOffset:   len(content),
			Metadata: chunkers.ChunkMetadata{
				Type:          chunkers.ChunkTypeCode,
				TokenEstimate: chunkers.EstimateTokens(contentStr),
				Code: &chunkers.CodeMetadata{
					Language: strategy.Language(),
				},
			},
		}}
	}

	return &chunkers.ChunkResult{
		Chunks:       chunkList,
		Warnings:     warnings,
		TotalChunks:  len(chunkList),
		ChunkerUsed:  treeSitterChunkerName,
		OriginalSize: len(content),
	}, nil
}

// extractChunks walks the AST and extracts significant nodes as chunks.
func (c *TreeSitterChunker) extractChunks(
	ctx context.Context,
	root *sitter.Node,
	source []byte,
	strategy LanguageStrategy,
	opts chunkers.ChunkOptions,
) ([]chunkers.Chunk, error) {
	var chunks []chunkers.Chunk
	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = chunkers.DefaultChunkOptions().MaxChunkSize
	}

	// First, extract package/import header if present
	headerEnd := c.findHeaderEnd(root, source, strategy)
	if headerEnd > 0 {
		headerContent := strings.TrimSpace(string(source[:headerEnd]))
		if headerContent != "" {
			chunks = append(chunks, chunkers.Chunk{
				Index:       len(chunks),
				Content:     headerContent,
				StartOffset: 0,
				EndOffset:   headerEnd,
				Metadata: chunkers.ChunkMetadata{
					Type:          chunkers.ChunkTypeCode,
					TokenEstimate: chunkers.EstimateTokens(headerContent),
					Code: &chunkers.CodeMetadata{
						Language: strategy.Language(),
					},
				},
			})
		}
	}

	// Walk tree and collect chunkable nodes
	nodeTypes := strategy.NodeTypes()
	cursor := sitter.NewTreeCursor(root)
	defer cursor.Close()

	var walk func() error
	walk = func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		node := cursor.CurrentNode()
		nodeType := node.Type()

		// Check if this node should be a chunk
		if nodeTypes.IsChunkable(nodeType) && strategy.ShouldChunk(node) {
			start := int(node.StartByte())
			end := int(node.EndByte())

			// Skip if this overlaps with header
			if start < headerEnd {
				start = headerEnd
			}
			if start >= end {
				goto descend
			}

			content := strings.TrimSpace(string(source[start:end]))
			if content == "" {
				goto descend
			}

			// Extract metadata for this node
			metadata := strategy.ExtractMetadata(node, source)
			if metadata == nil {
				metadata = &chunkers.CodeMetadata{}
			}
			metadata.Language = strategy.Language()

			// Split if too large
			if len(content) > maxSize {
				subChunks := c.splitLargeNode(content, metadata, maxSize, start)
				for _, sc := range subChunks {
					sc.Index = len(chunks)
					chunks = append(chunks, sc)
				}
			} else {
				chunks = append(chunks, chunkers.Chunk{
					Index:       len(chunks),
					Content:     content,
					StartOffset: start,
					EndOffset:   end,
					Metadata: chunkers.ChunkMetadata{
						Type:          chunkers.ChunkTypeCode,
						TokenEstimate: chunkers.EstimateTokens(content),
						Code:          metadata,
					},
				})
			}

			// Don't descend into children of chunked nodes
			goto next
		}

	descend:
		// Descend into children
		if cursor.GoToFirstChild() {
			if err := walk(); err != nil {
				return err
			}
			for cursor.GoToNextSibling() {
				if err := walk(); err != nil {
					return err
				}
			}
			cursor.GoToParent()
		}

	next:
		return nil
	}

	if err := walk(); err != nil {
		return nil, err
	}

	return chunks, nil
}

// findHeaderEnd finds the end position of package/import declarations.
func (c *TreeSitterChunker) findHeaderEnd(root *sitter.Node, source []byte, strategy LanguageStrategy) int {
	// Look for common header patterns at the start
	headerEnd := 0
	cursor := sitter.NewTreeCursor(root)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			node := cursor.CurrentNode()
			nodeType := node.Type()

			// Common header node types across languages
			isHeader := false
			switch nodeType {
			case "package_clause", "package_declaration", // Go, Java
				"import_declaration", "import_statement", "import_spec_list", // Various
				"preproc_include", "preproc_define", // C/C++
				"use_declaration", "extern_crate_declaration", // Rust
				"module_declaration": // Various
				isHeader = true
			}

			if isHeader {
				end := int(node.EndByte())
				if end > headerEnd {
					headerEnd = end
				}
			} else if headerEnd > 0 {
				// Stop at first non-header node after finding headers
				break
			}

			if !cursor.GoToNextSibling() {
				break
			}
		}
	}

	return headerEnd
}

// splitLargeNode splits a large AST node into smaller chunks.
func (c *TreeSitterChunker) splitLargeNode(content string, baseMeta *chunkers.CodeMetadata, maxSize, baseOffset int) []chunkers.Chunk {
	var chunks []chunkers.Chunk
	lines := strings.Split(content, "\n")

	var current strings.Builder
	offset := baseOffset

	for _, line := range lines {
		lineLen := len(line) + 1

		if current.Len()+lineLen > maxSize && current.Len() > 0 {
			chunkContent := current.String()
			meta := *baseMeta // Copy metadata

			chunks = append(chunks, chunkers.Chunk{
				Content:     chunkContent,
				StartOffset: offset - current.Len(),
				EndOffset:   offset,
				Metadata: chunkers.ChunkMetadata{
					Type:          chunkers.ChunkTypeCode,
					TokenEstimate: chunkers.EstimateTokens(chunkContent),
					Code:          &meta,
				},
			})
			current.Reset()
		}

		current.WriteString(line)
		current.WriteString("\n")
		offset += lineLen
	}

	if current.Len() > 0 {
		chunkContent := strings.TrimRight(current.String(), "\n")
		meta := *baseMeta // Copy metadata

		chunks = append(chunks, chunkers.Chunk{
			Content:     chunkContent,
			StartOffset: offset - current.Len(),
			EndOffset:   offset,
			Metadata: chunkers.ChunkMetadata{
				Type:          chunkers.ChunkTypeCode,
				TokenEstimate: chunkers.EstimateTokens(chunkContent),
				Code:          &meta,
			},
		})
	}

	return chunks
}

// Languages returns all supported language names.
func (c *TreeSitterChunker) Languages() []string {
	return c.registry.Languages()
}
