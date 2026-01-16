package chunkers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	docxChunkerName     = "docx"
	docxChunkerPriority = 72
)

// DOCXChunker splits DOCX content by heading sections.
type DOCXChunker struct{}

// NewDOCXChunker creates a new DOCX chunker.
func NewDOCXChunker() *DOCXChunker {
	return &DOCXChunker{}
}

// Name returns the chunker's identifier.
func (c *DOCXChunker) Name() string {
	return docxChunkerName
}

// CanHandle returns true for DOCX content.
func (c *DOCXChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
		strings.HasSuffix(strings.ToLower(language), ".docx")
}

// Priority returns the chunker's priority.
func (c *DOCXChunker) Priority() int {
	return docxChunkerPriority
}

// Chunk splits DOCX content by heading boundaries.
func (c *DOCXChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  docxChunkerName,
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
		return nil, fmt.Errorf("failed to open DOCX as zip; %w", err)
	}

	// Parse styles.xml to get heading style mappings
	styles, err := c.parseStyles(zipReader)
	if err != nil {
		// Non-fatal: continue without style mappings
		styles = make(map[string]int)
	}

	// Parse document.xml
	doc, err := c.parseDocument(zipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document.xml; %w", err)
	}

	// Extract sections
	sections := c.extractSections(doc, styles)

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
						IsTable:      section.isTable,
					},
				},
			})
		}
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  docxChunkerName,
		OriginalSize: len(content),
	}, nil
}

// docxSection represents a section of DOCX content.
type docxSection struct {
	heading     string
	level       int
	text        string
	sectionPath string
	startOffset int
	endOffset   int
	isTable     bool
}

// DOCX XML structures

// docxDocument represents the document.xml structure.
type docxDocument struct {
	XMLName xml.Name    `xml:"document"`
	Body    docxBody    `xml:"body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
	Tables     []docxTable     `xml:"tbl"`
	Elements   []docxElement   `xml:",any"`
}

type docxParagraph struct {
	Properties docxParagraphProps `xml:"pPr"`
	Runs       []docxRun          `xml:"r"`
}

type docxParagraphProps struct {
	Style docxStyle `xml:"pStyle"`
}

type docxStyle struct {
	Val string `xml:"val,attr"`
}

type docxRun struct {
	Text []docxText `xml:"t"`
}

type docxText struct {
	Content string `xml:",chardata"`
	Space   string `xml:"space,attr"`
}

type docxTable struct {
	Rows []docxTableRow `xml:"tr"`
}

type docxTableRow struct {
	Cells []docxTableCell `xml:"tc"`
}

type docxTableCell struct {
	Paragraphs []docxParagraph `xml:"p"`
}

type docxElement struct {
	XMLName xml.Name
	Content []byte `xml:",innerxml"`
}

// docxStyles represents the styles.xml structure.
type docxStyles struct {
	XMLName xml.Name     `xml:"styles"`
	Styles  []docxStyleDef `xml:"style"`
}

type docxStyleDef struct {
	Type    string `xml:"type,attr"`
	StyleID string `xml:"styleId,attr"`
	Name    struct {
		Val string `xml:"val,attr"`
	} `xml:"name"`
	BasedOn struct {
		Val string `xml:"val,attr"`
	} `xml:"basedOn"`
	PPr struct {
		OutlineLvl struct {
			Val int `xml:"val,attr"`
		} `xml:"outlineLvl"`
	} `xml:"pPr"`
}

// parseStyles extracts heading style mappings from styles.xml.
func (c *DOCXChunker) parseStyles(zipReader *zip.Reader) (map[string]int, error) {
	styles := make(map[string]int)

	// Find styles.xml
	var stylesFile *zip.File
	for _, f := range zipReader.File {
		if f.Name == "word/styles.xml" {
			stylesFile = f
			break
		}
	}
	if stylesFile == nil {
		return styles, nil
	}

	rc, err := stylesFile.Open()
	if err != nil {
		return styles, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return styles, err
	}

	var stylesDoc docxStyles
	if err := xml.Unmarshal(data, &stylesDoc); err != nil {
		return styles, err
	}

	// Map style IDs to heading levels
	for _, style := range stylesDoc.Styles {
		if style.Type != "paragraph" {
			continue
		}

		// Check for explicit outline level
		if style.PPr.OutlineLvl.Val > 0 || style.StyleID == "Heading1" {
			styles[style.StyleID] = style.PPr.OutlineLvl.Val + 1
			continue
		}

		// Check for heading-based styles by name
		nameLower := strings.ToLower(style.Name.Val)
		if strings.HasPrefix(nameLower, "heading ") || strings.HasPrefix(nameLower, "heading") {
			// Try to extract level from name
			for i := 1; i <= 9; i++ {
				if strings.Contains(nameLower, fmt.Sprintf("%d", i)) {
					styles[style.StyleID] = i
					break
				}
			}
		}

		// Common heading style IDs
		switch style.StyleID {
		case "Heading1", "heading1", "Title":
			styles[style.StyleID] = 1
		case "Heading2", "heading2", "Subtitle":
			styles[style.StyleID] = 2
		case "Heading3", "heading3":
			styles[style.StyleID] = 3
		case "Heading4", "heading4":
			styles[style.StyleID] = 4
		case "Heading5", "heading5":
			styles[style.StyleID] = 5
		case "Heading6", "heading6":
			styles[style.StyleID] = 6
		}
	}

	return styles, nil
}

// parseDocument parses document.xml from the DOCX.
func (c *DOCXChunker) parseDocument(zipReader *zip.Reader) (*docxDocument, error) {
	var docFile *zip.File
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return nil, fmt.Errorf("document.xml not found")
	}

	rc, err := docFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var doc docxDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// extractSections traverses the document and extracts sections.
func (c *DOCXChunker) extractSections(doc *docxDocument, styles map[string]int) []docxSection {
	var sections []docxSection
	var currentSection *docxSection
	var headingStack []string
	var offset int

	// Initialize root section
	currentSection = &docxSection{
		heading:     "",
		level:       0,
		text:        "",
		sectionPath: "",
		startOffset: 0,
	}

	// Process elements in order by parsing raw XML to maintain order
	for _, para := range doc.Body.Paragraphs {
		text := c.extractParagraphText(para)
		styleID := para.Properties.Style.Val
		level := styles[styleID]

		if level > 0 {
			// This is a heading - save current section and start new one
			if currentSection != nil && strings.TrimSpace(currentSection.text) != "" {
				currentSection.endOffset = offset
				sections = append(sections, *currentSection)
			}

			// Update heading stack
			for len(headingStack) >= level {
				headingStack = headingStack[:len(headingStack)-1]
			}
			headingStack = append(headingStack, text)

			currentSection = &docxSection{
				heading:     text,
				level:       level,
				text:        text + "\n\n",
				sectionPath: strings.Join(headingStack, " > "),
				startOffset: offset,
			}
			offset += len(text) + 2
		} else if strings.TrimSpace(text) != "" {
			// Regular paragraph
			if currentSection != nil {
				currentSection.text += text + "\n\n"
				offset += len(text) + 2
			}
		}
	}

	// Process tables - convert to CSV format
	for _, table := range doc.Body.Tables {
		tableText := c.tableToCSV(table)
		if strings.TrimSpace(tableText) != "" {
			if currentSection != nil {
				currentSection.text += tableText + "\n\n"
				offset += len(tableText) + 2
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

// extractParagraphText extracts text from a paragraph.
func (c *DOCXChunker) extractParagraphText(para docxParagraph) string {
	var text strings.Builder
	for _, run := range para.Runs {
		for _, t := range run.Text {
			text.WriteString(t.Content)
		}
	}
	return strings.TrimSpace(text.String())
}

// tableToCSV converts a DOCX table to CSV format.
func (c *DOCXChunker) tableToCSV(table docxTable) string {
	var csv strings.Builder

	for _, row := range table.Rows {
		var cells []string
		for _, cell := range row.Cells {
			var cellText strings.Builder
			for _, para := range cell.Paragraphs {
				text := c.extractParagraphText(para)
				if text != "" {
					if cellText.Len() > 0 {
						cellText.WriteString(" ")
					}
					cellText.WriteString(text)
				}
			}
			cells = append(cells, c.escapeCSV(cellText.String()))
		}
		csv.WriteString(strings.Join(cells, ","))
		csv.WriteString("\n")
	}

	return csv.String()
}

// escapeCSV escapes a value for CSV format.
func (c *DOCXChunker) escapeCSV(value string) string {
	// If value contains comma, quote, or newline, wrap in quotes and escape quotes
	if strings.ContainsAny(value, ",\"\n") {
		return "\"" + strings.ReplaceAll(value, "\"", "\"\"") + "\""
	}
	return value
}

// splitLargeSection splits a large section into smaller chunks.
func (c *DOCXChunker) splitLargeSection(ctx context.Context, section docxSection, maxSize int) []Chunk {
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
