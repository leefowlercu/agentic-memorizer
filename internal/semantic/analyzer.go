package semantic

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Analyzer performs semantic analysis on files
type Analyzer struct {
	client       *Client
	enableVision bool
	maxFileSize  int64
}

// NewAnalyzer creates a new semantic analyzer
func NewAnalyzer(client *Client, enableVision bool, maxFileSize int64) *Analyzer {
	return &Analyzer{
		client:       client,
		enableVision: enableVision,
		maxFileSize:  maxFileSize,
	}
}

// Analyze performs semantic analysis on a file
func (a *Analyzer) Analyze(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// Skip if file is too large
	if metadata.Size > a.maxFileSize {
		return nil, fmt.Errorf("file too large for analysis: %d bytes", metadata.Size)
	}

	// Check if this is an image and vision is enabled
	if metadata.Category == "images" && a.enableVision {
		return a.analyzeImage(metadata)
	}

	// Check if this is a PPTX or DOCX file - try document analysis
	if metadata.Type == "pptx" || metadata.Type == "docx" {
		return a.analyzeDocument(metadata)
	}

	// For other files, read content and analyze
	return a.analyzeText(metadata)
}

// analyzeText analyzes text-based files
func (a *Analyzer) analyzeText(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// Read file content
	content, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file; %w", err)
	}

	// For binary files, provide limited analysis
	if !metadata.IsReadable {
		return a.analyzeBinary(metadata)
	}

	// Truncate if needed (100KB limit for analysis)
	contentStr := string(content)
	if len(contentStr) > 100000 {
		contentStr = contentStr[:100000] + "\n\n[Content truncated...]"
	}

	// Build prompt
	prompt := a.buildPrompt(metadata, contentStr)

	// Call Claude API
	response, err := a.client.SendMessage(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
	}

	// Parse response as JSON
	var analysis types.SemanticAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		// If JSON parsing fails, try to extract from markdown code block
		if jsonStr := extractJSON(response); jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &analysis); err == nil {
				return &analysis, nil
			}
		}
		return nil, fmt.Errorf("failed to parse analysis response; %w", err)
	}

	return &analysis, nil
}

// analyzeImage analyzes image files using vision
func (a *Analyzer) analyzeImage(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// Read image file
	imageData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image; %w", err)
	}

	// Encode as base64
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	// Determine media type
	mediaType := getMediaType(metadata.Type)

	// Build prompt
	prompt := fmt.Sprintf(`Analyze this image and provide semantic understanding.

File: %s
Type: %s
Dimensions: %dx%d

Provide a JSON response with:
1. summary: 2-3 sentence description of what the image shows
2. tags: 3-5 semantic tags (lowercase, hyphenated)
3. key_topics: 3-5 main subjects or themes in the image
4. document_type: The type/purpose of this image (e.g., "diagram", "screenshot", "photo", "chart")

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Dimensions.Width,
		metadata.Dimensions.Height,
	)

	// Call Claude API with image
	response, err := a.client.SendMessageWithImage(prompt, imageBase64, mediaType)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze image with Claude; %w", err)
	}

	// Parse response
	var analysis types.SemanticAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		if jsonStr := extractJSON(response); jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &analysis); err == nil {
				return &analysis, nil
			}
		}
		return nil, fmt.Errorf("failed to parse analysis response; %w", err)
	}

	return &analysis, nil
}

// analyzeDocument analyzes document files (PPTX, DOCX, PDF) using the Claude API
func (a *Analyzer) analyzeDocument(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// For PPTX files, extract text and analyze as text
	// (Claude API only supports PDF for document content blocks)
	if metadata.Type == "pptx" {
		return a.analyzePptx(metadata)
	}

	// For DOCX files, extract text and analyze as text
	if metadata.Type == "docx" {
		return a.analyzeDocx(metadata)
	}

	// For PDF files, send as document
	if metadata.Type == "pdf" {
		documentData, err := os.ReadFile(metadata.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read document; %w", err)
		}

		documentBase64 := base64.StdEncoding.EncodeToString(documentData)

		prompt := fmt.Sprintf(`Analyze this PDF document and provide semantic understanding.

File: %s
Type: PDF
Size: %d bytes
`,
			filepath.Base(metadata.Path),
			metadata.Size,
		)

		if metadata.PageCount != nil {
			prompt += fmt.Sprintf("Pages: %d\n", *metadata.PageCount)
		}

		prompt += `
Provide a JSON response with:
1. summary: 2-3 sentence summary of the document's main content and purpose
2. tags: 3-5 semantic tags (lowercase, hyphenated)
3. key_topics: 3-5 main topics or themes covered in the document
4. document_type: The type/genre of this document (e.g., "technical-presentation", "report", "proposal", "training-material")

Be concise but informative. Respond with ONLY valid JSON.`

		response, err := a.client.SendMessageWithDocument(prompt, documentBase64, "application/pdf")
		if err != nil {
			return nil, fmt.Errorf("failed to analyze document with Claude; %w", err)
		}

		var analysis types.SemanticAnalysis
		if err := json.Unmarshal([]byte(response), &analysis); err != nil {
			if jsonStr := extractJSON(response); jsonStr != "" {
				if err := json.Unmarshal([]byte(jsonStr), &analysis); err == nil {
					return &analysis, nil
				}
			}
			return nil, fmt.Errorf("failed to parse analysis response; %w", err)
		}

		return &analysis, nil
	}

	// For other document types, fall back to generic analysis
	return a.analyzeBinary(metadata)
}

// analyzePptx extracts text from PPTX and analyzes it
func (a *Analyzer) analyzePptx(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// Import the metadata package to access the PptxHandler
	// We'll extract text directly here to avoid circular dependencies
	zipReader, err := os.Open(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open PPTX; %w", err)
	}
	defer zipReader.Close()

	stat, err := zipReader.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat PPTX; %w", err)
	}

	// Use the archive/zip package to read PPTX
	var allText strings.Builder
	zipFile, err := os.Open(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PPTX; %w", err)
	}
	defer zipFile.Close()

	reader, err := zip.NewReader(zipFile, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to open PPTX zip; %w", err)
	}

	// Extract text from each slide
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			rc, err := file.Open()
			if err != nil {
				continue
			}

			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Extract text between <a:t> tags
			text := string(data)
			for {
				start := strings.Index(text, "<a:t")
				if start == -1 {
					break
				}

				tagEnd := strings.Index(text[start:], ">")
				if tagEnd == -1 {
					break
				}

				end := strings.Index(text[start:], "</a:t>")
				if end == -1 {
					break
				}

				content := text[start+tagEnd+1 : start+end]
				if content != "" {
					allText.WriteString(content)
					allText.WriteString(" ")
				}

				text = text[start+end+6:]
			}

			allText.WriteString("\n\n")
		}
	}

	extractedText := strings.TrimSpace(allText.String())
	if extractedText == "" {
		// No text extracted, fall back to metadata-only analysis
		return a.analyzeBinary(metadata)
	}

	// Truncate if too long
	if len(extractedText) > 50000 {
		extractedText = extractedText[:50000] + "\n\n[Content truncated...]"
	}

	// Build prompt with extracted text
	prompt := fmt.Sprintf(`Analyze this PowerPoint presentation and provide semantic understanding.

File: %s
Type: PPTX
Slides: %d

Extracted text content:
%s

Provide a JSON response with:
1. summary: 2-3 sentence summary of the presentation's main content and purpose
2. tags: 3-5 semantic tags (lowercase, hyphenated)
3. key_topics: 3-5 main topics or themes covered in the presentation
4. document_type: The type/genre of this presentation (e.g., "technical-presentation", "training-material", "business-proposal")

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		*metadata.SlideCount,
		extractedText,
	)

	// Call Claude API
	response, err := a.client.SendMessage(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
	}

	// Parse response
	var analysis types.SemanticAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		if jsonStr := extractJSON(response); jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &analysis); err == nil {
				return &analysis, nil
			}
		}
		return nil, fmt.Errorf("failed to parse analysis response; %w", err)
	}

	return &analysis, nil
}

// analyzeDocx extracts text from DOCX and analyzes it
func (a *Analyzer) analyzeDocx(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// Open DOCX as ZIP
	zipReader, err := os.Open(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open DOCX; %w", err)
	}
	defer zipReader.Close()

	stat, err := zipReader.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat DOCX; %w", err)
	}

	// Use the archive/zip package to read DOCX
	var allText strings.Builder
	zipFile, err := os.Open(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read DOCX; %w", err)
	}
	defer zipFile.Close()

	reader, err := zip.NewReader(zipFile, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to open DOCX zip; %w", err)
	}

	// Extract text from word/document.xml
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				continue
			}

			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Extract text between <w:t> tags
			text := string(data)
			for {
				start := strings.Index(text, "<w:t")
				if start == -1 {
					break
				}

				tagEnd := strings.Index(text[start:], ">")
				if tagEnd == -1 {
					break
				}

				end := strings.Index(text[start:], "</w:t>")
				if end == -1 {
					break
				}

				content := text[start+tagEnd+1 : start+end]
				if content != "" {
					allText.WriteString(content)
					allText.WriteString(" ")
				}

				text = text[start+end+6:]
			}
			break
		}
	}

	extractedText := strings.TrimSpace(allText.String())
	if extractedText == "" {
		// No text extracted, fall back to metadata-only analysis
		return a.analyzeBinary(metadata)
	}

	// Truncate if too long
	if len(extractedText) > 50000 {
		extractedText = extractedText[:50000] + "\n\n[Content truncated...]"
	}

	// Build prompt with extracted text
	prompt := fmt.Sprintf(`Analyze this Word document and provide semantic understanding.

File: %s
Type: DOCX
Word Count: %d

Extracted text content:
%s

Provide a JSON response with:
1. summary: 2-3 sentence summary of the document's main content and purpose
2. tags: 3-5 semantic tags (lowercase, hyphenated)
3. key_topics: 3-5 main topics or themes covered in the document
4. document_type: The type/genre of this document (e.g., "technical-guide", "report", "article", "proposal")

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		*metadata.WordCount,
		extractedText,
	)

	// Call Claude API
	response, err := a.client.SendMessage(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
	}

	// Parse response
	var analysis types.SemanticAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		if jsonStr := extractJSON(response); jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &analysis); err == nil {
				return &analysis, nil
			}
		}
		return nil, fmt.Errorf("failed to parse analysis response; %w", err)
	}

	return &analysis, nil
}

// analyzeBinary provides limited analysis for binary files
func (a *Analyzer) analyzeBinary(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// For binary files, create a basic analysis based on metadata
	summary := fmt.Sprintf("%s file", strings.ToUpper(strings.TrimPrefix(metadata.Type, ".")))

	if metadata.PageCount != nil {
		summary += fmt.Sprintf(" with %d pages", *metadata.PageCount)
	} else if metadata.SlideCount != nil {
		summary += fmt.Sprintf(" with %d slides", *metadata.SlideCount)
	}

	return &types.SemanticAnalysis{
		Summary:      summary,
		Tags:         []string{metadata.Category, metadata.Type},
		KeyTopics:    []string{},
		DocumentType: metadata.Category,
		Confidence:   0.5, // Lower confidence for metadata-only analysis
	}, nil
}

// buildPrompt builds the analysis prompt
func (a *Analyzer) buildPrompt(metadata *types.FileMetadata, content string) string {
	return fmt.Sprintf(`Analyze this file and provide semantic understanding.

File: %s
Type: %s
Category: %s
Size: %d bytes

Content:
%s

Provide a JSON response with:
1. summary: 2-3 sentence summary capturing the main purpose/content
2. tags: 3-5 semantic tags (lowercase, hyphenated)
3. key_topics: 3-5 main topics or themes
4. document_type: The purpose/genre of this document

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Category,
		metadata.Size,
		content,
	)
}

// extractJSON extracts JSON from a markdown code block
func extractJSON(text string) string {
	// Look for ```json ... ``` or ``` ... ```
	if start := strings.Index(text, "```json"); start != -1 {
		start += 7
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	if start := strings.Index(text, "```"); start != -1 {
		start += 3
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	return ""
}

// getMediaType returns the media type for an image
func getMediaType(fileType string) string {
	switch strings.ToLower(fileType) {
	case ".png", "png":
		return "image/png"
	case ".jpg", ".jpeg", "jpg", "jpeg":
		return "image/jpeg"
	case ".gif", "gif":
		return "image/gif"
	case ".webp", "webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
