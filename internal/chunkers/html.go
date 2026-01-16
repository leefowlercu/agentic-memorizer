package chunkers

import (
	"bytes"
	"context"
	"strings"

	"golang.org/x/net/html"
)

const (
	htmlChunkerName     = "html"
	htmlChunkerPriority = 75
)

// headingTags maps HTML heading tags to their levels.
var headingTags = map[string]int{
	"h1": 1,
	"h2": 2,
	"h3": 3,
	"h4": 4,
	"h5": 5,
	"h6": 6,
}

// excludedTags contains elements whose content should be excluded.
var excludedTags = map[string]bool{
	"script":   true,
	"style":    true,
	"noscript": true,
	"head":     true,
	"meta":     true,
	"link":     true,
}

// HTMLChunker splits HTML content by heading sections.
type HTMLChunker struct{}

// NewHTMLChunker creates a new HTML chunker.
func NewHTMLChunker() *HTMLChunker {
	return &HTMLChunker{}
}

// Name returns the chunker's identifier.
func (c *HTMLChunker) Name() string {
	return htmlChunkerName
}

// CanHandle returns true for HTML content.
func (c *HTMLChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "text/html" ||
		mimeType == "application/xhtml+xml" ||
		strings.HasSuffix(strings.ToLower(language), ".html") ||
		strings.HasSuffix(strings.ToLower(language), ".htm")
}

// Priority returns the chunker's priority.
func (c *HTMLChunker) Priority() int {
	return htmlChunkerPriority
}

// Chunk splits HTML content by heading boundaries.
func (c *HTMLChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  htmlChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	// Parse HTML
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Extract sections based on headings
	sections := c.extractSections(doc)

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
					Type:          ChunkTypeMarkdown, // HTML is closest to markdown type
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
		ChunkerUsed:  htmlChunkerName,
		OriginalSize: len(content),
	}, nil
}

// htmlSection represents a section of HTML content.
type htmlSection struct {
	heading     string
	level       int
	text        string
	sectionPath string
	startOffset int
	endOffset   int
}

// extractSections traverses the HTML document and extracts sections based on headings.
func (c *HTMLChunker) extractSections(doc *html.Node) []htmlSection {
	var sections []htmlSection
	var currentSection *htmlSection
	var headingStack []string
	var offset int

	// Initialize with root section for content before first heading
	currentSection = &htmlSection{
		heading:     "",
		level:       0,
		text:        "",
		sectionPath: "",
		startOffset: 0,
	}

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n == nil {
			return
		}

		// Check if this is an excluded element
		if n.Type == html.ElementNode && excludedTags[n.Data] {
			return
		}

		// Check if this is a heading element
		if n.Type == html.ElementNode {
			if level, isHeading := headingTags[n.Data]; isHeading {
				// Save current section if it has content
				if currentSection != nil && strings.TrimSpace(currentSection.text) != "" {
					currentSection.endOffset = offset
					sections = append(sections, *currentSection)
				}

				// Update heading stack for section path
				headingText := c.extractText(n)
				for len(headingStack) >= level {
					headingStack = headingStack[:len(headingStack)-1]
				}
				headingStack = append(headingStack, headingText)

				// Start new section
				currentSection = &htmlSection{
					heading:     headingText,
					level:       level,
					text:        headingText + "\n\n",
					sectionPath: strings.Join(headingStack, " > "),
					startOffset: offset,
				}

				offset += len(headingText) + 2
				return
			}
		}

		// Extract text from text nodes
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" && currentSection != nil {
				// Add spacing between text elements
				if currentSection.text != "" && !strings.HasSuffix(currentSection.text, "\n") {
					currentSection.text += " "
					offset++
				}
				currentSection.text += text
				offset += len(text)
			}
		}

		// Handle block elements by adding newlines
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "br", "li", "tr", "article", "section", "aside", "main", "header", "footer", "nav":
				if currentSection != nil && currentSection.text != "" && !strings.HasSuffix(currentSection.text, "\n\n") {
					if strings.HasSuffix(currentSection.text, "\n") {
						currentSection.text += "\n"
						offset++
					} else {
						currentSection.text += "\n\n"
						offset += 2
					}
				}
			}
		}

		// Recursively process children
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}

		// Add newline after block elements
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "li", "tr", "article", "section", "aside", "main", "header", "footer", "nav":
				if currentSection != nil && !strings.HasSuffix(currentSection.text, "\n") {
					currentSection.text += "\n"
					offset++
				}
			}
		}
	}

	traverse(doc)

	// Save final section
	if currentSection != nil && strings.TrimSpace(currentSection.text) != "" {
		currentSection.endOffset = offset
		sections = append(sections, *currentSection)
	}

	return sections
}

// extractText extracts all text content from a node.
func (c *HTMLChunker) extractText(n *html.Node) string {
	if n == nil {
		return ""
	}

	var text strings.Builder

	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}
	}

	traverse(n)
	return strings.TrimSpace(text.String())
}

// splitLargeSection splits a large section into smaller chunks.
func (c *HTMLChunker) splitLargeSection(ctx context.Context, section htmlSection, maxSize int) []Chunk {
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
