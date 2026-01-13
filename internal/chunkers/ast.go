package chunkers

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

const (
	astChunkerName     = "ast"
	astChunkerPriority = 100 // Highest priority for supported languages
)

// ASTChunker splits code by AST nodes (functions, types, etc.).
// Currently supports Go code with plans to add tree-sitter support for other languages.
type ASTChunker struct {
	supportedLanguages map[string]bool
}

// NewASTChunker creates a new AST-based chunker.
func NewASTChunker() *ASTChunker {
	return &ASTChunker{
		supportedLanguages: map[string]bool{
			"go":     true,
			"golang": true,
			".go":    true,
		},
	}
}

// Name returns the chunker's identifier.
func (c *ASTChunker) Name() string {
	return astChunkerName
}

// CanHandle returns true for supported programming languages.
func (c *ASTChunker) CanHandle(mimeType string, language string) bool {
	lang := strings.ToLower(language)

	// Check direct language match
	if c.supportedLanguages[lang] {
		return true
	}

	// Check file extension
	if strings.HasSuffix(lang, ".go") {
		return true
	}

	// Check MIME types
	switch mimeType {
	case "text/x-go", "application/x-go":
		return true
	}

	return false
}

// Priority returns the chunker's priority.
func (c *ASTChunker) Priority() int {
	return astChunkerPriority
}

// Chunk splits code content by AST structure.
func (c *ASTChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) ([]Chunk, error) {
	if len(content) == 0 {
		return []Chunk{}, nil
	}

	language := strings.ToLower(opts.Language)

	// Route to language-specific parser
	if c.supportedLanguages[language] || strings.HasSuffix(language, ".go") {
		return c.chunkGo(ctx, content, opts)
	}

	// Fall back to simple code chunking
	return c.chunkGenericCode(ctx, content, opts)
}

// chunkGo parses Go code and splits by declarations.
func (c *ASTChunker) chunkGo(ctx context.Context, content []byte, opts ChunkOptions) ([]Chunk, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		// If parsing fails, fall back to generic chunking
		return c.chunkGenericCode(ctx, content, opts)
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	var chunks []Chunk
	contentStr := string(content)

	// Process package declaration and imports as first chunk if they exist
	if file.Package.IsValid() {
		headerEnd := 0
		if len(file.Decls) > 0 {
			headerEnd = int(fset.Position(file.Decls[0].Pos()).Offset)
		} else {
			headerEnd = len(content)
		}

		if headerEnd > 0 {
			headerContent := strings.TrimSpace(contentStr[:headerEnd])
			if headerContent != "" {
				chunks = append(chunks, Chunk{
					Index:       len(chunks),
					Content:     headerContent,
					StartOffset: 0,
					EndOffset:   headerEnd,
					Metadata: ChunkMetadata{
						Type:          ChunkTypeCode,
						Language:      "go",
						TokenEstimate: EstimateTokens(headerContent),
					},
				})
			}
		}
	}

	// Process each declaration
	for _, decl := range file.Decls {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		start := fset.Position(decl.Pos()).Offset
		end := fset.Position(decl.End()).Offset

		// Include leading comments
		if comments := c.getLeadingComments(file, decl, fset); comments != "" {
			commentStart := start - len(comments) - 1
			if commentStart >= 0 && commentStart < start {
				start = commentStart
			}
		}

		declContent := strings.TrimSpace(contentStr[start:end])
		if declContent == "" {
			continue
		}

		// Extract metadata based on declaration type
		metadata := c.extractDeclMetadata(decl)
		metadata.Type = ChunkTypeCode
		metadata.Language = "go"
		metadata.TokenEstimate = EstimateTokens(declContent)

		// If declaration is too large, split it
		if len(declContent) > maxSize {
			subChunks := c.splitLargeDecl(ctx, declContent, metadata, maxSize, start)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     declContent,
				StartOffset: start,
				EndOffset:   end,
				Metadata:    metadata,
			})
		}
	}

	// If no chunks were created, return the whole file as one chunk
	if len(chunks) == 0 {
		return []Chunk{{
			Index:       0,
			Content:     contentStr,
			StartOffset: 0,
			EndOffset:   len(content),
			Metadata: ChunkMetadata{
				Type:          ChunkTypeCode,
				Language:      "go",
				TokenEstimate: EstimateTokens(contentStr),
			},
		}}, nil
	}

	return chunks, nil
}

// getLeadingComments extracts doc comments before a declaration.
func (c *ASTChunker) getLeadingComments(file *ast.File, decl ast.Decl, fset *token.FileSet) string {
	declPos := fset.Position(decl.Pos()).Line

	for _, cg := range file.Comments {
		cgEnd := fset.Position(cg.End()).Line
		if cgEnd == declPos-1 || cgEnd == declPos {
			return cg.Text()
		}
	}
	return ""
}

// extractDeclMetadata extracts function/type names from declarations.
func (c *ASTChunker) extractDeclMetadata(decl ast.Decl) ChunkMetadata {
	metadata := ChunkMetadata{}

	switch d := decl.(type) {
	case *ast.FuncDecl:
		metadata.FunctionName = d.Name.Name
		if d.Recv != nil && len(d.Recv.List) > 0 {
			// Method - extract receiver type
			if t, ok := d.Recv.List[0].Type.(*ast.StarExpr); ok {
				if ident, ok := t.X.(*ast.Ident); ok {
					metadata.ClassName = ident.Name
				}
			} else if ident, ok := d.Recv.List[0].Type.(*ast.Ident); ok {
				metadata.ClassName = ident.Name
			}
		}
	case *ast.GenDecl:
		if d.Tok == token.TYPE && len(d.Specs) > 0 {
			if ts, ok := d.Specs[0].(*ast.TypeSpec); ok {
				metadata.ClassName = ts.Name.Name
			}
		}
	}

	return metadata
}

// splitLargeDecl splits a large declaration into smaller chunks.
func (c *ASTChunker) splitLargeDecl(ctx context.Context, content string, baseMeta ChunkMetadata, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk
	lines := strings.Split(content, "\n")

	var current strings.Builder
	offset := baseOffset

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		lineLen := len(line) + 1

		if current.Len()+lineLen > maxSize && current.Len() > 0 {
			chunkContent := current.String()
			meta := baseMeta
			meta.TokenEstimate = EstimateTokens(chunkContent)

			chunks = append(chunks, Chunk{
				Content:     chunkContent,
				StartOffset: offset - current.Len(),
				EndOffset:   offset,
				Metadata:    meta,
			})
			current.Reset()
		}

		current.WriteString(line)
		current.WriteString("\n")
		offset += lineLen
	}

	if current.Len() > 0 {
		chunkContent := strings.TrimRight(current.String(), "\n")
		meta := baseMeta
		meta.TokenEstimate = EstimateTokens(chunkContent)

		chunks = append(chunks, Chunk{
			Content:     chunkContent,
			StartOffset: offset - current.Len(),
			EndOffset:   offset,
			Metadata:    meta,
		})
	}

	return chunks
}

// chunkGenericCode provides fallback chunking for unsupported languages.
func (c *ASTChunker) chunkGenericCode(ctx context.Context, content []byte, opts ChunkOptions) ([]Chunk, error) {
	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	lines := strings.Split(text, "\n")

	var chunks []Chunk
	var current strings.Builder
	offset := 0
	blankCount := 0

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		trimmed := strings.TrimSpace(line)
		lineLen := len(line) + 1

		// Track blank lines for natural break detection
		if trimmed == "" {
			blankCount++
		} else {
			blankCount = 0
		}

		// Create new chunk at natural breaks (2+ blank lines) or size limit
		shouldBreak := (blankCount >= 2 && current.Len() > 100) ||
			(current.Len()+lineLen > maxSize && current.Len() > 0)

		if shouldBreak {
			chunkContent := strings.TrimSpace(current.String())
			if chunkContent != "" {
				chunks = append(chunks, Chunk{
					Index:       len(chunks),
					Content:     chunkContent,
					StartOffset: offset - current.Len(),
					EndOffset:   offset,
					Metadata: ChunkMetadata{
						Type:          ChunkTypeCode,
						Language:      opts.Language,
						TokenEstimate: EstimateTokens(chunkContent),
					},
				})
			}
			current.Reset()
			blankCount = 0
		}

		current.WriteString(line)
		current.WriteString("\n")
		offset += lineLen
	}

	// Finalize remaining content
	if current.Len() > 0 {
		chunkContent := strings.TrimSpace(current.String())
		if chunkContent != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     chunkContent,
				StartOffset: offset - current.Len(),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeCode,
					Language:      opts.Language,
					TokenEstimate: EstimateTokens(chunkContent),
				},
			})
		}
	}

	return chunks, nil
}
