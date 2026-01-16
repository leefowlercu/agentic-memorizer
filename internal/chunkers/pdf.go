package chunkers

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

const (
	pdfChunkerName     = "pdf"
	pdfChunkerPriority = 73
)

// Regex patterns for heading detection in untagged PDFs
var (
	// Lines that are likely headings (short, potentially numbered, title-like)
	// Matches patterns like "1. Introduction", "1.2 Background", "1.2.3 Details"
	headingPatternNumeric = regexp.MustCompile(`^(\d+\.?)+\s+[A-Za-z]`)
	headingPatternUpper   = regexp.MustCompile(`^[A-Z][A-Z\s]+$`)
	headingPatternTitle   = regexp.MustCompile(`(?i)^(Chapter|Section|Part|Appendix)\s+\w+`)
)

// PDFChunker splits PDF content by pages and detected sections.
type PDFChunker struct{}

// NewPDFChunker creates a new PDF chunker.
func NewPDFChunker() *PDFChunker {
	return &PDFChunker{}
}

// Name returns the chunker's identifier.
func (c *PDFChunker) Name() string {
	return pdfChunkerName
}

// CanHandle returns true for PDF content.
func (c *PDFChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "application/pdf" ||
		strings.HasSuffix(strings.ToLower(language), ".pdf")
}

// Priority returns the chunker's priority.
func (c *PDFChunker) Priority() int {
	return pdfChunkerPriority
}

// Chunk splits PDF content by pages and sections.
func (c *PDFChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  pdfChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	// Parse PDF
	reader := bytes.NewReader(content)
	conf := model.NewDefaultConfiguration()
	pdfCtx, err := api.ReadValidateAndOptimize(reader, conf)
	if err != nil {
		return nil, err
	}

	// Get page count
	pageCount := pdfCtx.PageCount

	// Determine extraction quality
	quality := c.detectQuality(pdfCtx)

	// Extract text from all pages
	pages, warnings := c.extractPages(ctx, pdfCtx, pageCount)

	// Build sections based on heading detection
	sections := c.buildSections(pages, quality, pageCount)

	var chunks []Chunk

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
			subChunks := c.splitLargeSection(ctx, section, maxSize, quality, pageCount)
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
					Type:          ChunkTypeProse,
					TokenEstimate: EstimateTokens(text),
					Document: &DocumentMetadata{
						Heading:           section.heading,
						HeadingLevel:      section.level,
						SectionPath:       section.sectionPath,
						PageNumber:        section.pageNumber,
						PageCount:         pageCount,
						ExtractionQuality: quality,
					},
				},
			})
		}
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  pdfChunkerName,
		OriginalSize: len(content),
	}, nil
}

// pdfSection represents a section of PDF content.
type pdfSection struct {
	heading     string
	level       int
	text        string
	sectionPath string
	pageNumber  int
	startOffset int
	endOffset   int
}

// pdfPage represents extracted text from a page.
type pdfPage struct {
	pageNumber int
	text       string
}

// detectQuality determines the extraction quality of the PDF.
func (c *PDFChunker) detectQuality(pdfCtx *model.Context) string {
	// Check for fonts which indicate text extraction capability
	if pdfCtx.Optimize != nil && pdfCtx.Optimize.FontObjects != nil {
		if len(pdfCtx.Optimize.FontObjects) > 0 {
			return "medium"
		}
	}

	// Check XRefTable size as indicator of document complexity
	if pdfCtx.XRefTable != nil && len(pdfCtx.XRefTable.Table) > 100 {
		return "medium"
	}

	return "low"
}

// extractPages extracts text from all pages.
func (c *PDFChunker) extractPages(ctx context.Context, pdfCtx *model.Context, pageCount int) ([]pdfPage, []ChunkWarning) {
	var pages []pdfPage
	var warnings []ChunkWarning

	for i := 1; i <= pageCount; i++ {
		select {
		case <-ctx.Done():
			return pages, warnings
		default:
		}

		// Extract content for this page
		reader, err := pdfcpu.ExtractPageContent(pdfCtx, i)
		if err != nil {
			warnings = append(warnings, ChunkWarning{
				Offset:  0,
				Message: "failed to extract page " + string(rune('0'+i)) + " content",
				Code:    "PDF_PAGE_EXTRACT_FAILED",
			})
			pages = append(pages, pdfPage{
				pageNumber: i,
				text:       "",
			})
			continue
		}

		// Read content stream
		contentBytes, err := io.ReadAll(reader)
		if err != nil {
			pages = append(pages, pdfPage{
				pageNumber: i,
				text:       "",
			})
			continue
		}

		// PDF content streams contain PostScript-like operators
		// Extract text from content stream (basic extraction)
		text := c.extractTextFromContentStream(contentBytes)

		pages = append(pages, pdfPage{
			pageNumber: i,
			text:       text,
		})
	}

	return pages, warnings
}

// extractTextFromContentStream extracts readable text from a PDF content stream.
// This is a basic implementation - PDF content streams use PostScript-like operators.
func (c *PDFChunker) extractTextFromContentStream(content []byte) string {
	var text strings.Builder
	str := string(content)

	// Look for text between parentheses (literal strings) and in text blocks
	// PDF text is typically in Tj/TJ operators with (string) or <hex> format
	inParens := 0
	var current strings.Builder

	for i := 0; i < len(str); i++ {
		ch := str[i]

		switch {
		case ch == '(' && (i == 0 || str[i-1] != '\\'):
			inParens++
			if inParens == 1 {
				current.Reset()
			}
		case ch == ')' && (i == 0 || str[i-1] != '\\'):
			if inParens > 0 {
				inParens--
				if inParens == 0 {
					// Process the extracted string
					extracted := current.String()
					if len(extracted) > 0 {
						text.WriteString(extracted)
						text.WriteString(" ")
					}
				}
			}
		case inParens > 0:
			// Handle escape sequences
			if ch == '\\' && i+1 < len(str) {
				next := str[i+1]
				switch next {
				case 'n':
					current.WriteString("\n")
					i++
				case 'r':
					current.WriteString("\r")
					i++
				case 't':
					current.WriteString("\t")
					i++
				case '(', ')', '\\':
					current.WriteByte(next)
					i++
				default:
					current.WriteByte(ch)
				}
			} else {
				current.WriteByte(ch)
			}
		}
	}

	result := text.String()
	// Clean up multiple spaces
	result = strings.Join(strings.Fields(result), " ")
	return result
}

// buildSections creates sections from pages based on heading detection.
func (c *PDFChunker) buildSections(pages []pdfPage, quality string, pageCount int) []pdfSection {
	var sections []pdfSection
	var currentSection *pdfSection
	var headingStack []string
	var offset int

	for _, page := range pages {
		pageText := strings.TrimSpace(page.text)
		if pageText == "" {
			continue
		}

		lines := strings.Split(pageText, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Detect potential headings
			isHeading, level := c.detectHeading(line, quality)

			if isHeading {
				// Save current section
				if currentSection != nil && strings.TrimSpace(currentSection.text) != "" {
					currentSection.endOffset = offset
					sections = append(sections, *currentSection)
				}

				// Update heading stack
				for len(headingStack) >= level {
					headingStack = headingStack[:len(headingStack)-1]
				}
				headingStack = append(headingStack, line)

				// Start new section
				currentSection = &pdfSection{
					heading:     line,
					level:       level,
					text:        line + "\n\n",
					sectionPath: strings.Join(headingStack, " > "),
					pageNumber:  page.pageNumber,
					startOffset: offset,
				}
				offset += len(line) + 2
			} else {
				// Regular text
				if currentSection == nil {
					currentSection = &pdfSection{
						heading:     "",
						level:       0,
						text:        "",
						sectionPath: "",
						pageNumber:  page.pageNumber,
						startOffset: offset,
					}
				}
				currentSection.text += line + "\n"
				offset += len(line) + 1
			}
		}
	}

	// Save final section
	if currentSection != nil && strings.TrimSpace(currentSection.text) != "" {
		currentSection.endOffset = offset
		sections = append(sections, *currentSection)
	}

	// If no sections were created, create one from all content
	if len(sections) == 0 && len(pages) > 0 {
		var allText strings.Builder
		for _, page := range pages {
			if strings.TrimSpace(page.text) != "" {
				allText.WriteString(page.text)
				allText.WriteString("\n\n")
			}
		}
		text := strings.TrimSpace(allText.String())
		if text != "" {
			pageNum := 1
			if len(pages) > 0 {
				pageNum = pages[0].pageNumber
			}
			sections = append(sections, pdfSection{
				heading:     "",
				level:       0,
				text:        text,
				sectionPath: "",
				pageNumber:  pageNum,
				startOffset: 0,
				endOffset:   len(text),
			})
		}
	}

	return sections
}

// detectHeading determines if a line is a heading.
func (c *PDFChunker) detectHeading(line string, quality string) (bool, int) {
	line = strings.TrimSpace(line)
	if len(line) < 3 || len(line) > 200 {
		return false, 0
	}

	// Check for numbered headings (e.g., "1.2.3 Introduction")
	if headingPatternNumeric.MatchString(line) {
		// Count number segments to determine level
		// "1. Intro" -> level 1, "1.2 Bg" -> level 2, "1.2.3 Details" -> level 3
		parts := strings.SplitN(line, " ", 2)
		if len(parts) > 0 {
			numPart := strings.TrimRight(parts[0], ".")
			segments := strings.Split(numPart, ".")
			if len(segments) > 0 {
				return true, len(segments)
			}
		}
		return true, 1
	}

	// Check for title-case patterns (Chapter, Section, etc.)
	if headingPatternTitle.MatchString(line) {
		return true, 1
	}

	// Check for all-caps short lines (often titles)
	if len(line) < 80 && headingPatternUpper.MatchString(line) {
		return true, 1
	}

	return false, 0
}

// splitLargeSection splits a large section into smaller chunks.
func (c *PDFChunker) splitLargeSection(ctx context.Context, section pdfSection, maxSize int, quality string, pageCount int) []Chunk {
	var chunks []Chunk

	// Split by paragraphs (double newlines or single newlines)
	paragraphs := strings.Split(section.text, "\n\n")
	if len(paragraphs) == 1 {
		paragraphs = strings.Split(section.text, "\n")
	}

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
					Type:          ChunkTypeProse,
					TokenEstimate: EstimateTokens(content),
					Document: &DocumentMetadata{
						Heading:           section.heading,
						HeadingLevel:      section.level,
						SectionPath:       section.sectionPath,
						PageNumber:        section.pageNumber,
						PageCount:         pageCount,
						ExtractionQuality: quality,
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
				Type:          ChunkTypeProse,
				TokenEstimate: EstimateTokens(content),
				Document: &DocumentMetadata{
					Heading:           section.heading,
					HeadingLevel:      section.level,
					SectionPath:       section.sectionPath,
					PageNumber:        section.pageNumber,
					PageCount:         pageCount,
					ExtractionQuality: quality,
				},
			},
		})
	}

	return chunks
}
