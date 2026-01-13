package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DefaultMaxDocSize is the default maximum size for documents (100MB).
const DefaultMaxDocSize = 100 * 1024 * 1024

// RichDocumentHandler handles Office documents (DOCX, XLSX, PPTX) and similar formats.
type RichDocumentHandler struct {
	maxSize int64
}

// RichDocumentHandlerOption configures the RichDocumentHandler.
type RichDocumentHandlerOption func(*RichDocumentHandler)

// WithMaxDocSize sets the maximum file size for document processing.
func WithMaxDocSize(size int64) RichDocumentHandlerOption {
	return func(h *RichDocumentHandler) {
		h.maxSize = size
	}
}

// NewRichDocumentHandler creates a new RichDocumentHandler with the given options.
func NewRichDocumentHandler(opts ...RichDocumentHandlerOption) *RichDocumentHandler {
	h := &RichDocumentHandler{
		maxSize: DefaultMaxDocSize,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Name returns the handler's unique identifier.
func (h *RichDocumentHandler) Name() string {
	return "rich_document"
}

// CanHandle returns true if this handler can process the given MIME type and extension.
func (h *RichDocumentHandler) CanHandle(mimeType string, ext string) bool {
	// Office Open XML formats
	docMIMEs := map[string]bool{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		"application/vnd.oasis.opendocument.text":                                   true,
		"application/vnd.oasis.opendocument.spreadsheet":                            true,
		"application/vnd.oasis.opendocument.presentation":                           true,
		"application/rtf": true,
	}

	if docMIMEs[mimeType] {
		return true
	}

	// Check extensions
	docExts := map[string]bool{
		".docx": true,
		".xlsx": true,
		".pptx": true,
		".odt":  true,
		".ods":  true,
		".odp":  true,
		".rtf":  true,
	}

	return docExts[strings.ToLower(ext)]
}

// Extract extracts content from the document file.
func (h *RichDocumentHandler) Extract(ctx context.Context, path string, size int64) (*ExtractedContent, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Check file size
	if size > h.maxSize {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        fmt.Sprintf("document too large: %d bytes (max %d)", size, h.maxSize),
			Metadata:     h.extractBasicMetadata(path, size),
		}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))

	var textContent string
	var extractErr error
	var docMetadata map[string]any

	switch ext {
	case ".docx":
		textContent, docMetadata, extractErr = extractDOCX(path)
	case ".xlsx":
		textContent, docMetadata, extractErr = extractXLSX(path)
	case ".pptx":
		textContent, docMetadata, extractErr = extractPPTX(path)
	case ".odt":
		textContent, docMetadata, extractErr = extractODT(path)
	case ".ods":
		textContent, docMetadata, extractErr = extractODS(path)
	case ".odp":
		textContent, docMetadata, extractErr = extractODP(path)
	case ".rtf":
		textContent, extractErr = extractRTF(path)
	default:
		extractErr = fmt.Errorf("unsupported document format: %s", ext)
	}

	metadata := h.extractBasicMetadata(path, size)
	if docMetadata != nil {
		metadata.Extra = docMetadata
	}

	if extractErr != nil {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        extractErr.Error(),
			Metadata:     metadata,
		}, nil
	}

	metadata.WordCount = countWords(textContent)

	return &ExtractedContent{
		Handler:     h.Name(),
		TextContent: textContent,
		Metadata:    metadata,
	}, nil
}

// MaxSize returns the maximum file size this handler will process.
func (h *RichDocumentHandler) MaxSize() int64 {
	return h.maxSize
}

// RequiresVision returns false as documents are text-extractable.
func (h *RichDocumentHandler) RequiresVision() bool {
	return false
}

// SupportedExtensions returns the file extensions this handler supports.
func (h *RichDocumentHandler) SupportedExtensions() []string {
	return []string{".docx", ".xlsx", ".pptx", ".odt", ".ods", ".odp", ".rtf"}
}

// extractBasicMetadata extracts basic file metadata.
func (h *RichDocumentHandler) extractBasicMetadata(path string, size int64) *FileMetadata {
	ext := filepath.Ext(path)
	info, _ := os.Stat(path)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	return &FileMetadata{
		Path:      path,
		Size:      size,
		ModTime:   modTime,
		MIMEType:  detectMIMEType(path, ext),
		Extension: ext,
	}
}

// extractDOCX extracts text from a DOCX file.
func extractDOCX(path string) (string, map[string]any, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open docx; %w", err)
	}
	defer r.Close()

	var textContent strings.Builder
	metadata := make(map[string]any)

	for _, f := range r.File {
		switch f.Name {
		case "word/document.xml":
			text, err := extractXMLText(f)
			if err == nil {
				textContent.WriteString(text)
			}
		case "docProps/core.xml":
			props, err := extractCoreProperties(f)
			if err == nil {
				for k, v := range props {
					metadata[k] = v
				}
			}
		}
	}

	return strings.TrimSpace(textContent.String()), metadata, nil
}

// extractXLSX extracts text from an XLSX file.
func extractXLSX(path string) (string, map[string]any, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open xlsx; %w", err)
	}
	defer r.Close()

	var textContent strings.Builder
	metadata := make(map[string]any)

	// First, extract shared strings
	var sharedStrings []string
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			sharedStrings, _ = extractSharedStrings(f)
			break
		}
	}

	// Then extract sheet content
	sheetCount := 0
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			sheetCount++
			text, err := extractSheetText(f, sharedStrings)
			if err == nil && text != "" {
				textContent.WriteString(fmt.Sprintf("=== Sheet %d ===\n", sheetCount))
				textContent.WriteString(text)
				textContent.WriteString("\n\n")
			}
		}
	}

	metadata["sheet_count"] = sheetCount

	// Extract core properties
	for _, f := range r.File {
		if f.Name == "docProps/core.xml" {
			props, err := extractCoreProperties(f)
			if err == nil {
				for k, v := range props {
					metadata[k] = v
				}
			}
			break
		}
	}

	return strings.TrimSpace(textContent.String()), metadata, nil
}

// extractPPTX extracts text from a PPTX file.
func extractPPTX(path string) (string, map[string]any, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open pptx; %w", err)
	}
	defer r.Close()

	var textContent strings.Builder
	metadata := make(map[string]any)

	slideCount := 0
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideCount++
			text, err := extractXMLText(f)
			if err == nil && text != "" {
				textContent.WriteString(fmt.Sprintf("=== Slide %d ===\n", slideCount))
				textContent.WriteString(text)
				textContent.WriteString("\n\n")
			}
		}
	}

	metadata["slide_count"] = slideCount

	// Extract core properties
	for _, f := range r.File {
		if f.Name == "docProps/core.xml" {
			props, err := extractCoreProperties(f)
			if err == nil {
				for k, v := range props {
					metadata[k] = v
				}
			}
			break
		}
	}

	return strings.TrimSpace(textContent.String()), metadata, nil
}

// extractODT extracts text from an ODT file.
func extractODT(path string) (string, map[string]any, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open odt; %w", err)
	}
	defer r.Close()

	var textContent strings.Builder
	metadata := make(map[string]any)

	for _, f := range r.File {
		if f.Name == "content.xml" {
			text, err := extractXMLText(f)
			if err == nil {
				textContent.WriteString(text)
			}
		}
	}

	return strings.TrimSpace(textContent.String()), metadata, nil
}

// extractODS extracts text from an ODS file.
func extractODS(path string) (string, map[string]any, error) {
	return extractODT(path) // Same structure
}

// extractODP extracts text from an ODP file.
func extractODP(path string) (string, map[string]any, error) {
	return extractODT(path) // Same structure
}

// extractRTF extracts text from an RTF file.
func extractRTF(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read rtf; %w", err)
	}

	// Simple RTF text extraction
	text := string(data)

	// Remove RTF control words
	controlWord := regexp.MustCompile(`\\[a-z]+\d*\s?`)
	text = controlWord.ReplaceAllString(text, "")

	// Remove braces
	text = strings.ReplaceAll(text, "{", "")
	text = strings.ReplaceAll(text, "}", "")

	// Clean up whitespace
	text = strings.TrimSpace(text)

	return text, nil
}

// extractXMLText extracts text content from an XML file in a ZIP archive.
func extractXMLText(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	// Simple XML text extraction - get content between tags
	var text strings.Builder
	decoder := xml.NewDecoder(bytes.NewReader(data))

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.CharData:
			content := strings.TrimSpace(string(t))
			if content != "" {
				text.WriteString(content)
				text.WriteString(" ")
			}
		}
	}

	return strings.TrimSpace(text.String()), nil
}

// extractCoreProperties extracts document properties from core.xml.
func extractCoreProperties(f *zip.File) (map[string]any, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	props := make(map[string]any)

	// Extract title
	titlePattern := regexp.MustCompile(`<dc:title>([^<]*)</dc:title>`)
	if match := titlePattern.FindSubmatch(data); len(match) >= 2 {
		props["title"] = string(match[1])
	}

	// Extract creator/author
	creatorPattern := regexp.MustCompile(`<dc:creator>([^<]*)</dc:creator>`)
	if match := creatorPattern.FindSubmatch(data); len(match) >= 2 {
		props["author"] = string(match[1])
	}

	// Extract subject
	subjectPattern := regexp.MustCompile(`<dc:subject>([^<]*)</dc:subject>`)
	if match := subjectPattern.FindSubmatch(data); len(match) >= 2 {
		props["subject"] = string(match[1])
	}

	return props, nil
}

// extractSharedStrings extracts shared strings from an XLSX file.
func extractSharedStrings(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var strings []string
	pattern := regexp.MustCompile(`<t[^>]*>([^<]*)</t>`)
	matches := pattern.FindAllSubmatch(data, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			strings = append(strings, string(match[1]))
		}
	}

	return strings, nil
}

// extractSheetText extracts text from an XLSX sheet.
func extractSheetText(f *zip.File, sharedStrings []string) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	var text strings.Builder

	// Find all cell values
	// Cells with t="s" use shared string index
	// Other cells have inline values
	cellPattern := regexp.MustCompile(`<c[^>]*(?:t="s")?[^>]*><v>([^<]*)</v></c>`)
	sharedPattern := regexp.MustCompile(`t="s"`)

	for _, match := range cellPattern.FindAllSubmatch(data, -1) {
		if len(match) >= 2 {
			value := string(match[1])
			cellTag := string(match[0])

			// Check if it's a shared string reference
			if sharedPattern.Match([]byte(cellTag)) {
				// Parse index and look up in shared strings
				var idx int
				if _, err := fmt.Sscanf(value, "%d", &idx); err == nil && idx < len(sharedStrings) {
					text.WriteString(sharedStrings[idx])
				}
			} else {
				text.WriteString(value)
			}
			text.WriteString(" ")
		}
	}

	return strings.TrimSpace(text.String()), nil
}
