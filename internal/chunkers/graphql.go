package chunkers

import (
	"context"
	"regexp"
	"strings"
)

const (
	graphqlChunkerName     = "graphql"
	graphqlChunkerPriority = 41
)

// Matches GraphQL type definitions.
// type Name, input Name, interface Name, enum Name, scalar Name, union Name
var graphqlTypeRegex = regexp.MustCompile(`(?m)^(type|input|interface|enum|scalar|union)\s+(\w+)`)

// Matches GraphQL schema definition.
var graphqlSchemaRegex = regexp.MustCompile(`(?m)^schema\s*\{`)

// Matches GraphQL extend definitions.
var graphqlExtendRegex = regexp.MustCompile(`(?m)^extend\s+(type|input|interface|enum|scalar|union)\s+(\w+)`)

// Matches GraphQL directive definitions.
var graphqlDirectiveRegex = regexp.MustCompile(`(?m)^directive\s+@(\w+)`)

// GraphQLChunker splits GraphQL schema content by type definitions.
type GraphQLChunker struct{}

// NewGraphQLChunker creates a new GraphQL chunker.
func NewGraphQLChunker() *GraphQLChunker {
	return &GraphQLChunker{}
}

// Name returns the chunker's identifier.
func (c *GraphQLChunker) Name() string {
	return graphqlChunkerName
}

// CanHandle returns true for GraphQL content.
func (c *GraphQLChunker) CanHandle(mimeType string, language string) bool {
	lang := strings.ToLower(language)
	mime := strings.ToLower(mimeType)

	// Match by MIME type
	if mime == "application/graphql" ||
		mime == "text/x-graphql" ||
		mime == "application/x-graphql" {
		return true
	}

	// Match by filename pattern
	if strings.HasSuffix(lang, ".graphql") ||
		strings.HasSuffix(lang, ".gql") ||
		strings.HasSuffix(lang, ".graphqls") {
		return true
	}

	return false
}

// Priority returns the chunker's priority.
func (c *GraphQLChunker) Priority() int {
	return graphqlChunkerPriority
}

// Chunk splits GraphQL content by type definitions.
func (c *GraphQLChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  graphqlChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	definitions := c.extractDefinitions(text)

	var chunks []Chunk
	offset := 0

	for _, def := range definitions {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If definition is too large, split it (especially for Query/Mutation)
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
						TypeName:    def.typeName,
						TypeKind:    def.typeKind,
						ServiceName: def.serviceName,
					},
				},
			})
		}

		offset += len(def.content)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     nil,
		TotalChunks:  len(chunks),
		ChunkerUsed:  graphqlChunkerName,
		OriginalSize: len(content),
	}, nil
}

// graphqlDefinition represents a parsed GraphQL definition.
type graphqlDefinition struct {
	typeName    string
	typeKind    string // type, input, interface, enum, scalar, union, schema, directive
	serviceName string // For schema-level service name (Query, Mutation, Subscription)
	content     string
	startLine   int
	endLine     int
}

// extractDefinitions extracts top-level definitions from GraphQL content.
func (c *GraphQLChunker) extractDefinitions(text string) []graphqlDefinition {
	var definitions []graphqlDefinition
	lines := strings.Split(text, "\n")

	// Track scalars and other preamble (before first type)
	var preambleLines []string
	inPreamble := true
	var currentDef *graphqlDefinition
	var currentLines []string
	braceCount := 0
	inBlockComment := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle block comments
		if strings.Contains(trimmed, `"""`) {
			count := strings.Count(trimmed, `"""`)
			if count%2 == 1 {
				inBlockComment = !inBlockComment
			}
		}

		// Check for new definition start (not inside braces or comment)
		if !inBlockComment && braceCount == 0 {
			// Check for type definitions
			if match := graphqlTypeRegex.FindStringSubmatch(line); match != nil {
				// Save previous definition
				if currentDef != nil {
					currentDef.content = strings.Join(currentLines, "\n") + "\n"
					currentDef.endLine = i - 1
					definitions = append(definitions, *currentDef)
				} else if len(preambleLines) > 0 && inPreamble {
					// Save preamble
					preambleContent := strings.Join(preambleLines, "\n")
					if strings.TrimSpace(preambleContent) != "" {
						definitions = append(definitions, graphqlDefinition{
							typeKind:  "preamble",
							content:   preambleContent + "\n",
							startLine: 0,
							endLine:   i - 1,
						})
					}
				}

				inPreamble = false
				currentDef = &graphqlDefinition{
					typeName:  match[2],
					typeKind:  match[1],
					startLine: i,
				}
				currentLines = []string{}
			} else if match := graphqlExtendRegex.FindStringSubmatch(line); match != nil {
				// Handle extend
				if currentDef != nil {
					currentDef.content = strings.Join(currentLines, "\n") + "\n"
					currentDef.endLine = i - 1
					definitions = append(definitions, *currentDef)
				}

				inPreamble = false
				currentDef = &graphqlDefinition{
					typeName:  match[2],
					typeKind:  "extend " + match[1],
					startLine: i,
				}
				currentLines = []string{}
			} else if graphqlSchemaRegex.MatchString(line) {
				// Handle schema definition
				if currentDef != nil {
					currentDef.content = strings.Join(currentLines, "\n") + "\n"
					currentDef.endLine = i - 1
					definitions = append(definitions, *currentDef)
				}

				inPreamble = false
				currentDef = &graphqlDefinition{
					typeName:  "schema",
					typeKind:  "schema",
					startLine: i,
				}
				currentLines = []string{}
			} else if match := graphqlDirectiveRegex.FindStringSubmatch(line); match != nil {
				// Handle directive
				if currentDef != nil {
					currentDef.content = strings.Join(currentLines, "\n") + "\n"
					currentDef.endLine = i - 1
					definitions = append(definitions, *currentDef)
				}

				inPreamble = false
				currentDef = &graphqlDefinition{
					typeName:  "@" + match[1],
					typeKind:  "directive",
					startLine: i,
				}
				currentLines = []string{}
			}
		}

		// Track brace depth
		if !inBlockComment {
			braceCount += strings.Count(line, "{")
			braceCount -= strings.Count(line, "}")
		}

		// Accumulate lines
		if currentDef != nil {
			currentLines = append(currentLines, line)
		} else if inPreamble {
			preambleLines = append(preambleLines, line)
		}
	}

	// Save final definition
	if currentDef != nil {
		currentDef.content = strings.Join(currentLines, "\n")
		currentDef.endLine = len(lines) - 1
		definitions = append(definitions, *currentDef)
	} else if len(preambleLines) > 0 {
		// Only preamble, no definitions
		content := strings.Join(preambleLines, "\n")
		if strings.TrimSpace(content) != "" {
			definitions = append(definitions, graphqlDefinition{
				typeKind:  "unknown",
				content:   content,
				startLine: 0,
				endLine:   len(lines) - 1,
			})
		}
	}

	// If no definitions found, return whole content
	if len(definitions) == 0 {
		definitions = append(definitions, graphqlDefinition{
			typeKind: "unknown",
			content:  text,
		})
	}

	// Include preceding comments with definitions
	definitions = c.attachComments(lines, definitions)

	return definitions
}

// attachComments attaches preceding comments to definitions.
func (c *GraphQLChunker) attachComments(lines []string, definitions []graphqlDefinition) []graphqlDefinition {
	result := make([]graphqlDefinition, 0, len(definitions))

	for i, def := range definitions {
		if i == 0 || def.typeKind == "preamble" {
			result = append(result, def)
			continue
		}

		// Look backwards from definition start for comments
		prevEnd := definitions[i-1].endLine
		commentStart := def.startLine

		for j := def.startLine - 1; j > prevEnd; j-- {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "#") ||
				strings.HasPrefix(trimmed, "\"\"\"") ||
				strings.HasPrefix(trimmed, "\"") ||
				strings.HasSuffix(trimmed, "\"\"\"") {
				commentStart = j
			} else {
				break
			}
		}

		if commentStart < def.startLine {
			commentLines := strings.Join(lines[commentStart:def.startLine], "\n")
			def.content = commentLines + "\n" + def.content
			def.startLine = commentStart
		}

		result = append(result, def)
	}

	return result
}

// splitLargeDefinition splits a large GraphQL definition into smaller chunks.
// This is particularly useful for large Query or Mutation types.
func (c *GraphQLChunker) splitLargeDefinition(ctx context.Context, def graphqlDefinition, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk

	// Try to split by field definitions for type/input/interface
	if def.typeKind == "type" || def.typeKind == "input" || def.typeKind == "interface" {
		return c.splitByFields(ctx, def, maxSize, baseOffset)
	}

	// For other types, do simple line-based splitting
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
						TypeName:    def.typeName,
						TypeKind:    def.typeKind,
						ServiceName: def.serviceName,
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
					TypeName:    def.typeName,
					TypeKind:    def.typeKind,
					ServiceName: def.serviceName,
				},
			},
		})
	}

	return chunks
}

// splitByFields splits a type/input/interface by its field definitions.
func (c *GraphQLChunker) splitByFields(ctx context.Context, def graphqlDefinition, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk
	var current strings.Builder
	var header strings.Builder
	offset := baseOffset

	lines := strings.Split(def.content, "\n")
	inBody := false
	braceCount := 0

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		// Track if we're inside the body
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")

		if !inBody && openBraces > 0 {
			inBody = true
			header.WriteString(line)
			header.WriteString("\n")
			braceCount += openBraces - closeBraces
			continue
		}

		if !inBody {
			header.WriteString(line)
			header.WriteString("\n")
			continue
		}

		braceCount += openBraces - closeBraces

		// If this would overflow and we have content, flush
		headerLen := header.Len()
		if current.Len()+len(line)+headerLen+3 > maxSize && current.Len() > 0 {
			content := header.String() + current.String() + "\n}"
			chunks = append(chunks, Chunk{
				Content:     content,
				StartOffset: offset - len(content),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(content),
					Schema: &SchemaMetadata{
						TypeName:    def.typeName,
						TypeKind:    def.typeKind,
						ServiceName: def.serviceName,
					},
				},
			})
			current.Reset()
		}

		// Don't add closing brace to current (we add it at flush)
		if braceCount > 0 || closeBraces == 0 {
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(line)
		}
		offset += len(line) + 1
	}

	// Final chunk
	if current.Len() > 0 {
		content := header.String() + current.String() + "\n}"
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset - len(content),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(content),
				Schema: &SchemaMetadata{
					TypeName:    def.typeName,
					TypeKind:    def.typeKind,
					ServiceName: def.serviceName,
				},
			},
		})
	} else if header.Len() > 0 {
		// Just header, no fields
		content := header.String()
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset - len(content),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(content),
				Schema: &SchemaMetadata{
					TypeName:    def.typeName,
					TypeKind:    def.typeKind,
					ServiceName: def.serviceName,
				},
			},
		})
	}

	return chunks
}
