package chunkers

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"strings"
)

const (
	xmlChunkerName     = "xml"
	xmlChunkerPriority = 25
)

// XMLChunker splits XML content by top-level elements.
type XMLChunker struct{}

// NewXMLChunker creates a new XML chunker.
func NewXMLChunker() *XMLChunker {
	return &XMLChunker{}
}

// Name returns the chunker's identifier.
func (c *XMLChunker) Name() string {
	return xmlChunkerName
}

// CanHandle returns true for XML content.
func (c *XMLChunker) CanHandle(mimeType string, language string) bool {
	mime := strings.ToLower(mimeType)
	lang := strings.ToLower(language)

	// Match by MIME type
	if mime == "application/xml" ||
		mime == "text/xml" ||
		strings.HasSuffix(mime, "+xml") {
		return true
	}

	// Match by file extension
	if strings.HasSuffix(lang, ".xml") ||
		strings.HasSuffix(lang, ".xsd") ||
		strings.HasSuffix(lang, ".xsl") ||
		strings.HasSuffix(lang, ".xslt") ||
		strings.HasSuffix(lang, ".svg") ||
		strings.HasSuffix(lang, ".plist") {
		return true
	}

	// Match by language hint
	if lang == "xml" {
		return true
	}

	return false
}

// Priority returns the chunker's priority.
func (c *XMLChunker) Priority() int {
	return xmlChunkerPriority
}

// Chunk splits XML content by top-level elements.
func (c *XMLChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  xmlChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	elements, warnings := c.parseXMLElements(content)

	var chunks []Chunk
	offset := 0

	for _, elem := range elements {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If element is too large, split it further
		if len(elem.content) > maxSize {
			subChunks := c.splitLargeElement(ctx, elem, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(elem.content) != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     elem.content,
				StartOffset: offset,
				EndOffset:   offset + len(elem.content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(elem.content),
					Structured: &StructuredMetadata{
						ElementName: elem.name,
						ElementPath: elem.path,
					},
				},
			})
		}

		offset += len(elem.content)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  xmlChunkerName,
		OriginalSize: len(content),
	}, nil
}

// xmlElement represents an XML element with its metadata.
type xmlElement struct {
	name    string
	path    string
	content string
}

// parseXMLElements extracts top-level elements from XML content.
func (c *XMLChunker) parseXMLElements(content []byte) ([]xmlElement, []ChunkWarning) {
	var elements []xmlElement
	var warnings []ChunkWarning

	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = false

	var rootName string
	var currentPath []string
	var elementDepth int
	var currentElementName string

	// Track the start position of each top-level element
	offset := 0

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Non-fatal parse error - add warning and try to continue
			warnings = append(warnings, ChunkWarning{
				Offset:  int(decoder.InputOffset()),
				Message: err.Error(),
				Code:    "XML_PARSE_ERROR",
			})
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if rootName == "" {
				// This is the root element
				rootName = t.Name.Local
				currentPath = append(currentPath, rootName)
			} else if len(currentPath) == 1 {
				// This is a top-level child element (direct child of root)
				currentElementName = t.Name.Local
				currentPath = append(currentPath, currentElementName)
				elementDepth = 1
			} else if elementDepth > 0 {
				// Inside a top-level element
				currentPath = append(currentPath, t.Name.Local)
				elementDepth++
			}

		case xml.EndElement:
			if elementDepth > 0 {
				elementDepth--
				if len(currentPath) > 0 {
					currentPath = currentPath[:len(currentPath)-1]
				}

				if elementDepth == 0 {
					// End of top-level element
					elementEnd := int(decoder.InputOffset())

					// Extract the element content from original bytes
					// We need to find the actual element bounds in the source
					elemContent := c.extractElementContent(content, currentElementName, offset)
					if elemContent != "" {
						elements = append(elements, xmlElement{
							name:    currentElementName,
							path:    "/" + rootName + "/" + currentElementName,
							content: elemContent,
						})
						offset += len(elemContent)
					}

					_ = elementEnd // Track position for future use
					currentElementName = ""
				}
			} else if len(currentPath) > 0 {
				// End of root element
				currentPath = currentPath[:len(currentPath)-1]
			}
		}
	}

	// If no elements were parsed, return the whole content as one chunk
	if len(elements) == 0 && len(content) > 0 {
		contentStr := string(content)
		elements = append(elements, xmlElement{
			name:    rootName,
			path:    "/" + rootName,
			content: contentStr,
		})
	}

	return elements, warnings
}

// extractElementContent extracts a single element's content from the XML source.
func (c *XMLChunker) extractElementContent(content []byte, elementName string, searchStart int) string {
	text := string(content[searchStart:])

	// Find opening tag
	openTag := "<" + elementName
	startIdx := strings.Index(text, openTag)
	if startIdx == -1 {
		return ""
	}

	// Find the end of this element
	// Handle both self-closing and normal closing tags
	depth := 0
	pos := startIdx

	for pos < len(text) {
		if pos+1 < len(text) && text[pos] == '<' {
			if text[pos+1] == '/' {
				// Potential closing tag
				closeTag := "</" + elementName
				if strings.HasPrefix(text[pos:], closeTag) {
					depth--
					if depth == 0 {
						// Find the > that closes this tag
						endPos := strings.Index(text[pos:], ">")
						if endPos != -1 {
							return text[startIdx : pos+endPos+1]
						}
					}
				}
				pos++
			} else if text[pos+1] != '!' && text[pos+1] != '?' {
				// Opening tag - check if it's our element
				if strings.HasPrefix(text[pos+1:], elementName) {
					// Check if it's actually the same tag (not a prefix match)
					rest := text[pos+1+len(elementName):]
					if len(rest) > 0 && (rest[0] == ' ' || rest[0] == '>' || rest[0] == '/' || rest[0] == '\t' || rest[0] == '\n') {
						depth++
					}
				}
				pos++
			} else {
				pos++
			}
		} else if pos+1 < len(text) && text[pos] == '/' && text[pos+1] == '>' {
			// Self-closing tag
			if depth == 1 {
				return text[startIdx : pos+2]
			}
			depth--
			pos += 2
		} else {
			pos++
		}
	}

	// If we couldn't find proper closing, return from start to end
	return text[startIdx:]
}

// splitLargeElement splits a large XML element into smaller chunks.
func (c *XMLChunker) splitLargeElement(ctx context.Context, elem xmlElement, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk
	content := elem.content
	offset := baseOffset

	// Try to split by child elements first
	childElems := c.extractChildElements(content, elem.name)

	if len(childElems) > 1 {
		// Group child elements into chunks
		var current strings.Builder
		currentChildren := 0

		for _, child := range childElems {
			select {
			case <-ctx.Done():
				return chunks
			default:
			}

			if current.Len()+len(child.content) > maxSize && current.Len() > 0 {
				chunkContent := current.String()
				chunks = append(chunks, Chunk{
					Content:     chunkContent,
					StartOffset: offset,
					EndOffset:   offset + len(chunkContent),
					Metadata: ChunkMetadata{
						Type:          ChunkTypeStructured,
						TokenEstimate: EstimateTokens(chunkContent),
						Structured: &StructuredMetadata{
							ElementName: elem.name,
							ElementPath: elem.path,
							RecordCount: currentChildren,
						},
					},
				})
				offset += len(chunkContent)
				current.Reset()
				currentChildren = 0
			}

			current.WriteString(child.content)
			currentChildren++
		}

		// Finalize remaining
		if current.Len() > 0 {
			chunkContent := current.String()
			chunks = append(chunks, Chunk{
				Content:     chunkContent,
				StartOffset: offset,
				EndOffset:   offset + len(chunkContent),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(chunkContent),
					Structured: &StructuredMetadata{
						ElementName: elem.name,
						ElementPath: elem.path,
						RecordCount: currentChildren,
					},
				},
			})
		}

		return chunks
	}

	// Fall back to line-based splitting
	lines := strings.Split(content, "\n")
	var current strings.Builder

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		if current.Len()+len(line)+1 > maxSize && current.Len() > 0 {
			chunkContent := current.String()
			chunks = append(chunks, Chunk{
				Content:     chunkContent,
				StartOffset: offset,
				EndOffset:   offset + len(chunkContent),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(chunkContent),
					Structured: &StructuredMetadata{
						ElementName: elem.name,
						ElementPath: elem.path,
					},
				},
			})
			offset += len(chunkContent)
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	// Finalize last chunk
	if current.Len() > 0 {
		chunkContent := current.String()
		chunks = append(chunks, Chunk{
			Content:     chunkContent,
			StartOffset: offset,
			EndOffset:   offset + len(chunkContent),
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(chunkContent),
				Structured: &StructuredMetadata{
					ElementName: elem.name,
					ElementPath: elem.path,
				},
			},
		})
	}

	return chunks
}

// extractChildElements extracts immediate child elements from XML content.
func (c *XMLChunker) extractChildElements(content string, parentName string) []xmlElement {
	var children []xmlElement

	decoder := xml.NewDecoder(strings.NewReader(content))
	decoder.Strict = false

	depth := 0
	var currentChild strings.Builder
	var currentName string
	inChild := false

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				// Start of immediate child element
				inChild = true
				currentName = t.Name.Local
				currentChild.Reset()

				// Write opening tag
				currentChild.WriteString("<" + t.Name.Local)
				for _, attr := range t.Attr {
					currentChild.WriteString(" " + attr.Name.Local + "=\"" + attr.Value + "\"")
				}
				currentChild.WriteString(">")
			} else if inChild {
				// Nested element inside child
				currentChild.WriteString("<" + t.Name.Local)
				for _, attr := range t.Attr {
					currentChild.WriteString(" " + attr.Name.Local + "=\"" + attr.Value + "\"")
				}
				currentChild.WriteString(">")
			}

		case xml.EndElement:
			if inChild {
				currentChild.WriteString("</" + t.Name.Local + ">")
			}
			depth--
			if depth == 1 && inChild {
				// End of immediate child element
				children = append(children, xmlElement{
					name:    currentName,
					path:    "/" + parentName + "/" + currentName,
					content: currentChild.String(),
				})
				inChild = false
				currentName = ""
			}

		case xml.CharData:
			if inChild {
				currentChild.Write(t)
			}
		}
	}

	return children
}
