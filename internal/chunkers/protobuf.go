package chunkers

import (
	"context"
	"strings"

	"github.com/yoheimuta/go-protoparser/v4"
	"github.com/yoheimuta/go-protoparser/v4/parser"
)

const (
	protobufChunkerName     = "protobuf"
	protobufChunkerPriority = 42
)

// ProtobufChunker splits Protocol Buffer content by message, service, and enum definitions.
type ProtobufChunker struct{}

// NewProtobufChunker creates a new Protocol Buffer chunker.
func NewProtobufChunker() *ProtobufChunker {
	return &ProtobufChunker{}
}

// Name returns the chunker's identifier.
func (c *ProtobufChunker) Name() string {
	return protobufChunkerName
}

// CanHandle returns true for Protocol Buffer content.
func (c *ProtobufChunker) CanHandle(mimeType string, language string) bool {
	lang := strings.ToLower(language)
	mime := strings.ToLower(mimeType)

	// Match by MIME type
	if mime == "text/x-protobuf" ||
		mime == "application/x-protobuf" ||
		mime == "text/protobuf" {
		return true
	}

	// Match by filename pattern
	if strings.HasSuffix(lang, ".proto") {
		return true
	}

	return false
}

// Priority returns the chunker's priority.
func (c *ProtobufChunker) Priority() int {
	return protobufChunkerPriority
}

// Chunk splits Protocol Buffer content by definitions.
func (c *ProtobufChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  protobufChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	// Parse protobuf content
	reader := strings.NewReader(string(content))
	got, err := protoparser.Parse(reader)

	var warnings []ChunkWarning
	if err != nil {
		warnings = append(warnings, ChunkWarning{
			Offset:  0,
			Message: "protobuf parse error: " + err.Error(),
			Code:    "PROTOBUF_PARSE_ERROR",
		})
		// Fall back to simple splitting
		return c.fallbackChunk(ctx, content, opts, warnings)
	}

	definitions := c.extractDefinitions(got, content)

	var chunks []Chunk
	offset := 0

	for _, def := range definitions {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If definition is too large, split it
		if len(def.content) > maxSize {
			subChunks := c.splitLargeDefinition(ctx, def, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(def.content) != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     def.content,
				StartOffset: offset,
				EndOffset:   offset + len(def.content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(def.content),
					Schema: &SchemaMetadata{
						MessageName: def.messageName,
						ServiceName: def.serviceName,
						RPCName:     def.rpcName,
						TypeKind:    def.typeKind,
					},
				},
			})
		}

		offset += len(def.content)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  protobufChunkerName,
		OriginalSize: len(content),
	}, nil
}

// protobufDefinition represents a parsed protobuf definition.
type protobufDefinition struct {
	messageName string
	serviceName string
	rpcName     string
	typeKind    string // message, enum, service
	content     string
	startLine   int
	endLine     int
}

// extractDefinitions extracts top-level definitions from a protobuf file.
func (c *ProtobufChunker) extractDefinitions(proto *parser.Proto, content []byte) []protobufDefinition {
	var definitions []protobufDefinition
	lines := strings.Split(string(content), "\n")

	// First, collect preamble (syntax, package, imports, options)
	var preambleEnd int
	for _, element := range proto.ProtoBody {
		switch v := element.(type) {
		case *parser.Syntax:
			if v.Meta.Pos.Line > preambleEnd {
				preambleEnd = v.Meta.Pos.Line
			}
		case *parser.Package:
			if v.Meta.Pos.Line > preambleEnd {
				preambleEnd = v.Meta.Pos.Line
			}
		case *parser.Import:
			if v.Meta.Pos.Line > preambleEnd {
				preambleEnd = v.Meta.Pos.Line
			}
		case *parser.Option:
			if v.Meta.Pos.Line > preambleEnd {
				preambleEnd = v.Meta.Pos.Line
			}
		}
	}

	// Add preamble as first chunk if exists
	if preambleEnd > 0 {
		preambleContent := strings.Join(lines[:preambleEnd], "\n")
		if strings.TrimSpace(preambleContent) != "" {
			definitions = append(definitions, protobufDefinition{
				typeKind:  "preamble",
				content:   preambleContent + "\n",
				startLine: 1,
				endLine:   preambleEnd,
			})
		}
	}

	// Extract messages, enums, and services
	for _, element := range proto.ProtoBody {
		switch v := element.(type) {
		case *parser.Message:
			startLine := v.Meta.Pos.Line - 1 // 0-indexed
			endLine := c.findDefinitionEnd(lines, startLine)
			msgContent := strings.Join(lines[startLine:endLine], "\n")

			// Include preceding comments
			commentedContent := c.includeComments(lines, startLine, msgContent)

			definitions = append(definitions, protobufDefinition{
				messageName: v.MessageName,
				typeKind:    "message",
				content:     commentedContent + "\n",
				startLine:   startLine + 1,
				endLine:     endLine,
			})

		case *parser.Enum:
			startLine := v.Meta.Pos.Line - 1
			endLine := c.findDefinitionEnd(lines, startLine)
			enumContent := strings.Join(lines[startLine:endLine], "\n")

			commentedContent := c.includeComments(lines, startLine, enumContent)

			definitions = append(definitions, protobufDefinition{
				messageName: v.EnumName,
				typeKind:    "enum",
				content:     commentedContent + "\n",
				startLine:   startLine + 1,
				endLine:     endLine,
			})

		case *parser.Service:
			startLine := v.Meta.Pos.Line - 1
			endLine := c.findDefinitionEnd(lines, startLine)
			svcContent := strings.Join(lines[startLine:endLine], "\n")

			commentedContent := c.includeComments(lines, startLine, svcContent)

			definitions = append(definitions, protobufDefinition{
				serviceName: v.ServiceName,
				typeKind:    "service",
				content:     commentedContent + "\n",
				startLine:   startLine + 1,
				endLine:     endLine,
			})
		}
	}

	// If no definitions found, return whole content
	if len(definitions) == 0 {
		definitions = append(definitions, protobufDefinition{
			typeKind: "unknown",
			content:  string(content),
		})
	}

	return definitions
}

// findDefinitionEnd finds the end line of a definition by tracking braces.
func (c *ProtobufChunker) findDefinitionEnd(lines []string, startLine int) int {
	braceCount := 0
	foundFirstBrace := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			switch ch {
			case '{':
				braceCount++
				foundFirstBrace = true
			case '}':
				braceCount--
				if foundFirstBrace && braceCount == 0 {
					return i + 1 // Return next line (exclusive)
				}
			}
		}
	}

	return len(lines)
}

// includeComments includes preceding comments with the definition.
func (c *ProtobufChunker) includeComments(lines []string, startLine int, content string) string {
	// Look backwards for comment lines
	commentStart := startLine
	for i := startLine - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasSuffix(trimmed, "*/") {
			commentStart = i
		} else if trimmed == "" {
			// Allow empty lines between comments and definition
			continue
		} else {
			break
		}
	}

	if commentStart < startLine {
		commentLines := strings.Join(lines[commentStart:startLine], "\n")
		return commentLines + "\n" + content
	}

	return content
}

// splitLargeDefinition splits a large definition into smaller chunks.
func (c *ProtobufChunker) splitLargeDefinition(ctx context.Context, def protobufDefinition, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk
	var current strings.Builder
	offset := baseOffset

	lines := strings.Split(def.content, "\n")

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		if current.Len()+len(line)+1 > maxSize && current.Len() > 0 {
			content := current.String()
			chunks = append(chunks, Chunk{
				Content:     content,
				StartOffset: offset - len(content),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(content),
					Schema: &SchemaMetadata{
						MessageName: def.messageName,
						ServiceName: def.serviceName,
						RPCName:     def.rpcName,
						TypeKind:    def.typeKind,
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
		content := current.String()
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset - len(content),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(content),
				Schema: &SchemaMetadata{
					MessageName: def.messageName,
					ServiceName: def.serviceName,
					RPCName:     def.rpcName,
					TypeKind:    def.typeKind,
				},
			},
		})
	}

	return chunks
}

// fallbackChunk creates chunks when protobuf parsing fails.
func (c *ProtobufChunker) fallbackChunk(ctx context.Context, content []byte, opts ChunkOptions, warnings []ChunkWarning) (*ChunkResult, error) {
	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	var chunks []Chunk
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
					Schema: &SchemaMetadata{
						TypeKind: "unknown",
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
				Schema: &SchemaMetadata{
					TypeKind: "unknown",
				},
			},
		})
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  protobufChunkerName,
		OriginalSize: len(content),
	}, nil
}
