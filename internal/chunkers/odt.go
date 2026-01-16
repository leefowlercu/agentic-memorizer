package chunkers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	odtChunkerName     = "odt"
	odtChunkerPriority = 71
)

// ODTChunker splits ODT content by heading sections.
type ODTChunker struct{}

// NewODTChunker creates a new ODT chunker.
func NewODTChunker() *ODTChunker {
	return &ODTChunker{}
}

// Name returns the chunker's identifier.
func (c *ODTChunker) Name() string {
	return odtChunkerName
}

// CanHandle returns true for ODT content.
func (c *ODTChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "application/vnd.oasis.opendocument.text" ||
		strings.HasSuffix(strings.ToLower(language), ".odt")
}

// Priority returns the chunker's priority.
func (c *ODTChunker) Priority() int {
	return odtChunkerPriority
}

// Chunk splits ODT content by heading boundaries.
func (c *ODTChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  odtChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	// Open as ZIP
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to open ODT as zip; %w", err)
	}

	// Parse styles.xml for automatic styles with outline levels
	autoStyles, err := c.parseStyles(zipReader)
	if err != nil {
		// Non-fatal
		autoStyles = make(map[string]int)
	}

	// Parse content.xml
	elements, err := c.parseContent(zipReader, autoStyles)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content.xml; %w", err)
	}

	// Extract sections
	sections := c.extractSections(elements)

	var chunks []Chunk
	var warnings []ChunkWarning

	for _, section := range sections {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Skip empty sections
		text := strings.TrimSpace(section.text)
		if text == "" {
			continue
		}

		// If section is too large, split it
		if len(text) > maxSize {
			subChunks := c.splitLargeSection(ctx, section, maxSize)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     text,
				StartOffset: section.startOffset,
				EndOffset:   section.endOffset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeMarkdown,
					TokenEstimate: EstimateTokens(text),
					Document: &DocumentMetadata{
						Heading:      section.heading,
						HeadingLevel: section.level,
						SectionPath:  section.sectionPath,
					},
				},
			})
		}
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  odtChunkerName,
		OriginalSize: len(content),
	}, nil
}

// odtSection represents a section of ODT content.
type odtSection struct {
	heading     string
	level       int
	text        string
	sectionPath string
	startOffset int
	endOffset   int
}

// odtElement represents a parsed element from content.xml.
type odtElement struct {
	isHeading    bool
	outlineLevel int
	text         string
}

// parseStyles extracts automatic style outline levels from styles.xml.
func (c *ODTChunker) parseStyles(zipReader *zip.Reader) (map[string]int, error) {
	styles := make(map[string]int)

	// Try styles.xml first
	for _, f := range zipReader.File {
		if f.Name == "styles.xml" || f.Name == "content.xml" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			c.extractStyleOutlineLevels(data, styles)
		}
	}

	return styles, nil
}

// extractStyleOutlineLevels parses XML data for style definitions.
func (c *ODTChunker) extractStyleOutlineLevels(data []byte, styles map[string]int) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var currentStyleName string
	inStyle := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			localName := elem.Name.Local
			if localName == "style" {
				for _, attr := range elem.Attr {
					if attr.Name.Local == "name" {
						currentStyleName = attr.Value
						inStyle = true
					}
				}
			}
			if inStyle && (localName == "paragraph-properties" || localName == "text-properties") {
				for _, attr := range elem.Attr {
					if attr.Name.Local == "outline-level" {
						if level, err := strconv.Atoi(attr.Value); err == nil && level > 0 {
							styles[currentStyleName] = level
						}
					}
				}
			}
		case xml.EndElement:
			if elem.Name.Local == "style" {
				inStyle = false
				currentStyleName = ""
			}
		}
	}
}

// parseContent parses content.xml and extracts elements.
func (c *ODTChunker) parseContent(zipReader *zip.Reader, autoStyles map[string]int) ([]odtElement, error) {
	var contentFile *zip.File
	for _, f := range zipReader.File {
		if f.Name == "content.xml" {
			contentFile = f
			break
		}
	}
	if contentFile == nil {
		return nil, fmt.Errorf("content.xml not found")
	}

	rc, err := contentFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return c.parseODTElements(data, autoStyles), nil
}

// parseODTElements parses the XML and extracts text elements.
func (c *ODTChunker) parseODTElements(data []byte, autoStyles map[string]int) []odtElement {
	var elements []odtElement
	decoder := xml.NewDecoder(bytes.NewReader(data))

	var inHeading bool
	var inParagraph bool
	var currentOutlineLevel int
	var textBuffer strings.Builder
	var currentStyleName string

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			localName := elem.Name.Local

			switch localName {
			case "h": // text:h - heading
				inHeading = true
				inParagraph = false
				textBuffer.Reset()
				currentOutlineLevel = 1 // Default level

				for _, attr := range elem.Attr {
					switch attr.Name.Local {
					case "outline-level":
						if level, err := strconv.Atoi(attr.Value); err == nil {
							currentOutlineLevel = level
						}
					case "style-name":
						currentStyleName = attr.Value
						if level, ok := autoStyles[currentStyleName]; ok {
							currentOutlineLevel = level
						}
					}
				}

			case "p": // text:p - paragraph
				if !inHeading {
					inParagraph = true
					textBuffer.Reset()
					currentStyleName = ""

					for _, attr := range elem.Attr {
						if attr.Name.Local == "style-name" {
							currentStyleName = attr.Value
						}
					}
				}

			case "tab": // text:tab
				if inHeading || inParagraph {
					textBuffer.WriteString("\t")
				}

			case "s": // text:s - space
				if inHeading || inParagraph {
					count := 1
					for _, attr := range elem.Attr {
						if attr.Name.Local == "c" {
							if c, err := strconv.Atoi(attr.Value); err == nil {
								count = c
							}
						}
					}
					for i := 0; i < count; i++ {
						textBuffer.WriteString(" ")
					}
				}

			case "line-break": // text:line-break
				if inHeading || inParagraph {
					textBuffer.WriteString("\n")
				}
			}

		case xml.EndElement:
			localName := elem.Name.Local

			switch localName {
			case "h":
				if inHeading {
					elements = append(elements, odtElement{
						isHeading:    true,
						outlineLevel: currentOutlineLevel,
						text:         strings.TrimSpace(textBuffer.String()),
					})
					inHeading = false
				}

			case "p":
				if inParagraph {
					text := strings.TrimSpace(textBuffer.String())
					if text != "" {
						// Check if this paragraph style implies a heading
						level := 0
						if currentStyleName != "" {
							level = autoStyles[currentStyleName]
						}
						elements = append(elements, odtElement{
							isHeading:    level > 0,
							outlineLevel: level,
							text:         text,
						})
					}
					inParagraph = false
				}
			}

		case xml.CharData:
			if inHeading || inParagraph {
				textBuffer.Write(elem)
			}
		}
	}

	return elements
}

// extractSections groups elements into sections based on headings.
func (c *ODTChunker) extractSections(elements []odtElement) []odtSection {
	var sections []odtSection
	var currentSection *odtSection
	var headingStack []string
	var offset int

	// Initialize root section
	currentSection = &odtSection{
		heading:     "",
		level:       0,
		text:        "",
		sectionPath: "",
		startOffset: 0,
	}

	for _, elem := range elements {
		if elem.isHeading && elem.outlineLevel > 0 {
			// Save current section if it has content
			if currentSection != nil && strings.TrimSpace(currentSection.text) != "" {
				currentSection.endOffset = offset
				sections = append(sections, *currentSection)
			}

			// Update heading stack
			for len(headingStack) >= elem.outlineLevel {
				headingStack = headingStack[:len(headingStack)-1]
			}
			headingStack = append(headingStack, elem.text)

			// Start new section
			currentSection = &odtSection{
				heading:     elem.text,
				level:       elem.outlineLevel,
				text:        elem.text + "\n\n",
				sectionPath: strings.Join(headingStack, " > "),
				startOffset: offset,
			}
			offset += len(elem.text) + 2
		} else if elem.text != "" {
			// Regular paragraph
			if currentSection != nil {
				currentSection.text += elem.text + "\n\n"
				offset += len(elem.text) + 2
			}
		}
	}

	// Save final section
	if currentSection != nil && strings.TrimSpace(currentSection.text) != "" {
		currentSection.endOffset = offset
		sections = append(sections, *currentSection)
	}

	return sections
}

// splitLargeSection splits a large section into smaller chunks.
func (c *ODTChunker) splitLargeSection(ctx context.Context, section odtSection, maxSize int) []Chunk {
	var chunks []Chunk

	// Split by paragraphs (double newlines)
	paragraphs := strings.Split(section.text, "\n\n")
	var current strings.Builder
	offset := section.startOffset

	for _, para := range paragraphs {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// If adding this paragraph exceeds max, finalize current chunk
		if current.Len()+len(para)+2 > maxSize && current.Len() > 0 {
			content := current.String()
			chunks = append(chunks, Chunk{
				Content:     content,
				StartOffset: offset - len(content),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeMarkdown,
					TokenEstimate: EstimateTokens(content),
					Document: &DocumentMetadata{
						Heading:      section.heading,
						HeadingLevel: section.level,
						SectionPath:  section.sectionPath,
					},
				},
			})
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
		offset += len(para) + 2
	}

	// Finalize last chunk
	if current.Len() > 0 {
		content := current.String()
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset - len(content),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeMarkdown,
				TokenEstimate: EstimateTokens(content),
				Document: &DocumentMetadata{
					Heading:      section.heading,
					HeadingLevel: section.level,
					SectionPath:  section.sectionPath,
				},
			},
		})
	}

	return chunks
}
