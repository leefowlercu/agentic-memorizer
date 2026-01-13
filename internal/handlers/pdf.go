package handlers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultMaxPDFSize is the default maximum size for PDFs (100MB).
const DefaultMaxPDFSize = 100 * 1024 * 1024

// PDFHandler handles PDF files with optional vision API support.
type PDFHandler struct {
	maxSize   int64
	useVision bool
}

// PDFHandlerOption configures the PDFHandler.
type PDFHandlerOption func(*PDFHandler)

// WithMaxPDFSize sets the maximum file size for PDF processing.
func WithMaxPDFSize(size int64) PDFHandlerOption {
	return func(h *PDFHandler) {
		h.maxSize = size
	}
}

// WithPDFVision enables vision API processing for PDFs.
func WithPDFVision(enabled bool) PDFHandlerOption {
	return func(h *PDFHandler) {
		h.useVision = enabled
	}
}

// NewPDFHandler creates a new PDFHandler with the given options.
func NewPDFHandler(opts ...PDFHandlerOption) *PDFHandler {
	h := &PDFHandler{
		maxSize:   DefaultMaxPDFSize,
		useVision: true, // Default to using vision for scanned PDFs
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Name returns the handler's unique identifier.
func (h *PDFHandler) Name() string {
	return "pdf"
}

// CanHandle returns true if this handler can process the given MIME type and extension.
func (h *PDFHandler) CanHandle(mimeType string, ext string) bool {
	return mimeType == "application/pdf" || strings.ToLower(ext) == ".pdf"
}

// Extract extracts content from the PDF file.
func (h *PDFHandler) Extract(ctx context.Context, path string, size int64) (*ExtractedContent, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Check file size
	if size > h.maxSize {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        fmt.Sprintf("PDF too large: %d bytes (max %d)", size, h.maxSize),
			Metadata:     h.extractBasicMetadata(path, size),
		}, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF; %w", err)
	}

	// Verify PDF header
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        "invalid PDF header",
			Metadata:     h.extractBasicMetadata(path, size),
		}, nil
	}

	// Extract metadata
	metadata := h.extractMetadata(path, size, data)

	// Try to extract text content
	textContent := extractPDFText(data)

	result := &ExtractedContent{
		Handler:  h.Name(),
		Metadata: metadata,
	}

	// If we got meaningful text, use it
	if len(strings.TrimSpace(textContent)) > 100 {
		result.TextContent = textContent
		metadata.WordCount = countWords(textContent)
	} else if h.useVision {
		// No meaningful text, prepare for vision API
		result.VisionContent = &VisionContent{
			ImageData: data,
			MIMEType:  "application/pdf",
		}
	} else {
		// No text and no vision, skip analysis
		result.SkipAnalysis = true
		if textContent != "" {
			result.TextContent = textContent
		}
	}

	return result, nil
}

// MaxSize returns the maximum file size this handler will process.
func (h *PDFHandler) MaxSize() int64 {
	return h.maxSize
}

// RequiresVision returns true if vision API is enabled for this handler.
func (h *PDFHandler) RequiresVision() bool {
	return h.useVision
}

// SupportedExtensions returns the file extensions this handler supports.
func (h *PDFHandler) SupportedExtensions() []string {
	return []string{".pdf"}
}

// extractBasicMetadata extracts basic file metadata without parsing PDF.
func (h *PDFHandler) extractBasicMetadata(path string, size int64) *FileMetadata {
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
		MIMEType:  "application/pdf",
		Extension: ext,
	}
}

// extractMetadata extracts metadata from PDF data.
func (h *PDFHandler) extractMetadata(path string, size int64, data []byte) *FileMetadata {
	metadata := h.extractBasicMetadata(path, size)
	metadata.Extra = make(map[string]any)

	// Extract PDF version from header
	if len(data) >= 8 {
		version := string(data[5:8])
		metadata.Extra["pdf_version"] = version
	}

	// Try to find page count
	pageCount := extractPDFPageCount(data)
	if pageCount > 0 {
		metadata.PageCount = pageCount
	}

	// Try to extract title and author from PDF metadata
	title, author := extractPDFInfo(data)
	if title != "" {
		metadata.Title = title
	}
	if author != "" {
		metadata.Author = author
	}

	return metadata
}

// extractPDFPageCount attempts to extract page count from PDF data.
func extractPDFPageCount(data []byte) int {
	// Look for /Count N in Pages dictionary
	countPattern := regexp.MustCompile(`/Count\s+(\d+)`)
	matches := countPattern.FindAllSubmatch(data, -1)

	maxCount := 0
	for _, match := range matches {
		if len(match) >= 2 {
			count, err := strconv.Atoi(string(match[1]))
			if err == nil && count > maxCount {
				maxCount = count
			}
		}
	}

	return maxCount
}

// extractPDFInfo attempts to extract title and author from PDF Info dictionary.
func extractPDFInfo(data []byte) (title, author string) {
	// Simple extraction - look for /Title and /Author
	titlePattern := regexp.MustCompile(`/Title\s*\(([^)]*)\)`)
	authorPattern := regexp.MustCompile(`/Author\s*\(([^)]*)\)`)

	if match := titlePattern.FindSubmatch(data); len(match) >= 2 {
		title = decodePDFString(string(match[1]))
	}

	if match := authorPattern.FindSubmatch(data); len(match) >= 2 {
		author = decodePDFString(string(match[1]))
	}

	return title, author
}

// decodePDFString performs basic PDF string decoding.
func decodePDFString(s string) string {
	// Handle basic escape sequences
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\(", "(")
	s = strings.ReplaceAll(s, "\\)", ")")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return strings.TrimSpace(s)
}

// extractPDFText attempts to extract text content from PDF data.
// This is a simplified extraction that works for basic PDFs.
func extractPDFText(data []byte) string {
	var textContent strings.Builder

	// Find all stream...endstream sections and try to extract text
	streamPattern := regexp.MustCompile(`stream\r?\n([\s\S]*?)\r?\nendstream`)
	streams := streamPattern.FindAllSubmatch(data, -1)

	for _, stream := range streams {
		if len(stream) >= 2 {
			text := extractTextFromStream(stream[1])
			if text != "" {
				textContent.WriteString(text)
				textContent.WriteString("\n")
			}
		}
	}

	// Also look for BT...ET (text blocks) in uncompressed content
	btPattern := regexp.MustCompile(`BT\s*([\s\S]*?)\s*ET`)
	btBlocks := btPattern.FindAllSubmatch(data, -1)

	for _, block := range btBlocks {
		if len(block) >= 2 {
			text := extractTextFromBTBlock(block[1])
			if text != "" {
				textContent.WriteString(text)
				textContent.WriteString("\n")
			}
		}
	}

	return strings.TrimSpace(textContent.String())
}

// extractTextFromStream attempts to extract text from a PDF stream.
func extractTextFromStream(stream []byte) string {
	// Look for text showing operators: Tj, TJ, ', "
	var text strings.Builder

	scanner := bufio.NewScanner(bytes.NewReader(stream))
	for scanner.Scan() {
		line := scanner.Text()

		// Look for (text) Tj or [(text)] TJ patterns
		tjPattern := regexp.MustCompile(`\(([^)]*)\)\s*Tj`)
		if matches := tjPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, m := range matches {
				if len(m) >= 2 {
					text.WriteString(decodePDFString(m[1]))
					text.WriteString(" ")
				}
			}
		}
	}

	return strings.TrimSpace(text.String())
}

// extractTextFromBTBlock extracts text from a BT...ET text block.
func extractTextFromBTBlock(block []byte) string {
	var text strings.Builder

	// Look for (text) Tj patterns
	tjPattern := regexp.MustCompile(`\(([^)]*)\)\s*Tj`)
	matches := tjPattern.FindAllSubmatch(block, -1)

	for _, m := range matches {
		if len(m) >= 2 {
			text.WriteString(decodePDFString(string(m[1])))
			text.WriteString(" ")
		}
	}

	return strings.TrimSpace(text.String())
}
