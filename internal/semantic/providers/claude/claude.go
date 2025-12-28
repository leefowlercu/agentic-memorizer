package claude

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const Version = "1.0.0"

// ClaudeProvider implements the Provider interface for Claude API
type ClaudeProvider struct {
	client       *Client
	model        string
	enableVision bool
	maxFileSize  int64
	logger       *slog.Logger
}

// NewClaudeProvider creates a new Claude provider instance
func NewClaudeProvider(config semantic.ProviderConfig, logger *slog.Logger) (semantic.Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for Claude provider")
	}

	if config.Model == "" {
		return nil, fmt.Errorf("model is required for Claude provider")
	}

	client := NewClient(config.APIKey, config.Model, config.MaxTokens, config.Timeout)

	return &ClaudeProvider{
		client:       client,
		model:        config.Model,
		enableVision: config.EnableVision,
		maxFileSize:  config.MaxFileSize,
		logger:       logger,
	}, nil
}

// Name returns the provider identifier
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// Model returns the model being used
func (p *ClaudeProvider) Model() string {
	return p.model
}

// SupportsVision returns whether provider supports image analysis
func (p *ClaudeProvider) SupportsVision() bool {
	return true // Claude supports vision API
}

// SupportsDocuments returns whether provider supports PDF/document blocks
func (p *ClaudeProvider) SupportsDocuments() bool {
	return true // Claude supports document content blocks for PDFs
}

// Analyze generates semantic understanding for a file
// Content routing strategy:
// - Images -> analyzeImage (vision API with base64 encoding)
// - Office docs (pptx/docx) -> analyzeDocument (text extraction + analysis)
// - PDFs -> analyzeDocument (document content blocks API)
// - Everything else -> analyzeText (standard text analysis)
// Files exceeding maxFileSize are rejected before routing.
func (p *ClaudeProvider) Analyze(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	if metadata.Size > p.maxFileSize {
		return nil, fmt.Errorf("file too large for analysis: %d bytes", metadata.Size)
	}

	if metadata.Category == "images" && p.enableVision {
		return p.analyzeImage(metadata)
	}

	if metadata.Type == "pptx" || metadata.Type == "docx" {
		return p.analyzeDocument(metadata)
	}

	return p.analyzeText(metadata)
}

func (p *ClaudeProvider) analyzeText(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	content, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file; %w", err)
	}

	if !metadata.IsReadable {
		return p.analyzeBinary(metadata)
	}

	contentStr := string(content)
	if len(contentStr) > 100000 { // 100,000 byte limit for analysis
		contentStr = contentStr[:100000] + "\n\n[Content truncated...]"
	}

	prompt := p.buildPrompt(metadata, contentStr)

	response, err := p.client.SendMessage(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
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

func (p *ClaudeProvider) analyzeImage(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	imageData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image; %w", err)
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imageData)
	mediaType := getMediaType(metadata.Type)

	prompt := fmt.Sprintf(`Analyze this image and provide semantic understanding.

File: %s
Type: %s
Dimensions: %dx%d

Provide a JSON response with:
1. summary: 2-3 sentence description of what the image shows
2. tags: 3-5 semantic tags (lowercase, hyphenated)
3. key_topics: 3-5 main subjects or themes in the image
4. document_type: The type/purpose of this image (e.g., "diagram", "screenshot", "photo", "chart")
5. entities: Array of named entities visible or referenced in the image, each with:
   - name: The entity name (e.g., brand names, people, tools shown)
   - type: One of: technology, person, concept, organization, project
6. references: Array of topic references if applicable, each with:
   - topic: The referenced topic or concept
   - type: One of: requires, extends, related-to, implements
   - confidence: Float 0.0-1.0 indicating strength of reference

For entities, identify logos, product names, people, or technologies visible in the image.
For references, identify concepts the image relates to or explains.

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Dimensions.Width,
		metadata.Dimensions.Height,
	)

	response, err := p.client.SendMessageWithImage(prompt, imageBase64, mediaType)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze image with Claude; %w", err)
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

func (p *ClaudeProvider) analyzeDocument(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// For PPTX files, extract text and analyze as text
	// (Claude API only supports PDF for document content blocks)
	if metadata.Type == "pptx" {
		return p.analyzePptx(metadata)
	}

	// For DOCX files, extract text and analyze as text
	if metadata.Type == "docx" {
		return p.analyzeDocx(metadata)
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
5. entities: Array of named entities found in the document, each with:
   - name: The entity name (e.g., "Terraform", "AWS", "Docker")
   - type: One of: technology, person, concept, organization, project
6. references: Array of topic references/dependencies, each with:
   - topic: The referenced topic or concept
   - type: One of: requires, extends, related-to, implements
   - confidence: Float 0.0-1.0 indicating strength of reference

For entities, focus on specific technologies, tools, people, organizations, and key concepts.
For references, identify what the document depends on, builds upon, or relates to.

Be concise but informative. Respond with ONLY valid JSON.`

		response, err := p.client.SendMessageWithDocument(prompt, documentBase64, "application/pdf")
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
	return p.analyzeBinary(metadata)
}

func (p *ClaudeProvider) analyzePptx(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// Open PPTX as ZIP
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
		return p.analyzeBinary(metadata)
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
5. entities: Array of named entities found in the presentation, each with:
   - name: The entity name (e.g., "Terraform", "AWS", "Docker")
   - type: One of: technology, person, concept, organization, project
6. references: Array of topic references/dependencies, each with:
   - topic: The referenced topic or concept
   - type: One of: requires, extends, related-to, implements
   - confidence: Float 0.0-1.0 indicating strength of reference

For entities, focus on specific technologies, tools, people, organizations, and key concepts.
For references, identify what the presentation depends on, builds upon, or relates to.

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		*metadata.SlideCount,
		extractedText,
	)

	response, err := p.client.SendMessage(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
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

func (p *ClaudeProvider) analyzeDocx(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
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
		return p.analyzeBinary(metadata)
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
5. entities: Array of named entities found in the document, each with:
   - name: The entity name (e.g., "Terraform", "AWS", "Docker")
   - type: One of: technology, person, concept, organization, project
6. references: Array of topic references/dependencies, each with:
   - topic: The referenced topic or concept
   - type: One of: requires, extends, related-to, implements
   - confidence: Float 0.0-1.0 indicating strength of reference

For entities, focus on specific technologies, tools, people, organizations, and key concepts.
For references, identify what the document depends on, builds upon, or relates to.

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		*metadata.WordCount,
		extractedText,
	)

	response, err := p.client.SendMessage(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
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

func (p *ClaudeProvider) analyzeBinary(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
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
		Entities:     []types.Entity{},
		References:   []types.Reference{},
	}, nil
}

func (p *ClaudeProvider) buildPrompt(metadata *types.FileMetadata, content string) string {
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
5. entities: Array of named entities found in the content, each with:
   - name: The entity name (e.g., "Terraform", "AWS", "Docker")
   - type: One of: technology, person, concept, organization, project
6. references: Array of topic references/dependencies, each with:
   - topic: The referenced topic or concept
   - type: One of: requires, extends, related-to, implements
   - confidence: Float 0.0-1.0 indicating strength of reference

For entities, focus on specific technologies, tools, people, organizations, and key concepts.
For references, identify what the document depends on, builds upon, or relates to.

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Category,
		metadata.Size,
		content,
	)
}

// extractJSON extracts JSON from code block wrappers
func extractJSON(text string) string {
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

// getMediaType converts file type to media type
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
