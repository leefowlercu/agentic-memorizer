package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"google.golang.org/api/option"
)

const Version = "1.0.0"

// GeminiProvider implements the Provider interface for Google Gemini API
type GeminiProvider struct {
	client       *genai.Client
	model        *genai.GenerativeModel
	modelName    string
	maxTokens    int
	enableVision bool
	maxFileSize  int64
	logger       *slog.Logger
}

// NewGeminiProvider creates a new Gemini provider instance
func NewGeminiProvider(config semantic.ProviderConfig, logger *slog.Logger) (semantic.Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for Gemini provider")
	}

	if config.Model == "" {
		return nil, fmt.Errorf("model is required for Gemini provider")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client; %w", err)
	}

	model := client.GenerativeModel(config.Model)
	model.SetMaxOutputTokens(int32(config.MaxTokens))

	return &GeminiProvider{
		client:       client,
		model:        model,
		modelName:    config.Model,
		maxTokens:    config.MaxTokens,
		enableVision: config.EnableVision,
		maxFileSize:  config.MaxFileSize,
		logger:       logger,
	}, nil
}

// Name returns the provider identifier
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Model returns the model being used
func (p *GeminiProvider) Model() string {
	return p.modelName
}

// SupportsVision returns whether provider supports image analysis
func (p *GeminiProvider) SupportsVision() bool {
	// All Gemini 2.x and 3.x models support vision natively
	model := strings.ToLower(p.modelName)
	return strings.Contains(model, "gemini-2") ||
		strings.Contains(model, "gemini-3") ||
		strings.Contains(model, "gemini-pro-vision") ||
		strings.Contains(model, "gemini-1.5")
}

// SupportsDocuments returns whether provider supports PDF/document blocks
func (p *GeminiProvider) SupportsDocuments() bool {
	// Gemini supports native PDF analysis via multimodal content
	return true
}

// Analyze generates semantic understanding for a file
func (p *GeminiProvider) Analyze(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	if metadata.Size > p.maxFileSize {
		return nil, fmt.Errorf("file too large for analysis: %d bytes", metadata.Size)
	}

	if metadata.Category == "images" && p.enableVision && p.SupportsVision() {
		return p.analyzeImage(ctx, metadata)
	}

	if metadata.Type == "pdf" && p.SupportsDocuments() {
		return p.analyzeDocument(ctx, metadata)
	}

	if metadata.Type == "pptx" || metadata.Type == "docx" {
		// Use text extraction for Office formats
		return p.analyzeDocument(ctx, metadata)
	}

	return p.analyzeText(ctx, metadata)
}

func (p *GeminiProvider) analyzeText(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	content, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file; %w", err)
	}

	if !metadata.IsReadable {
		return p.analyzeBinary(metadata)
	}

	contentStr := string(content)
	if len(contentStr) > 100000 {
		contentStr = contentStr[:100000] + "\n\n[Content truncated...]"
	}

	prompt := p.buildPrompt(metadata, contentStr)

	resp, err := p.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Gemini; %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	response := extractTextFromParts(resp.Candidates[0].Content.Parts)
	return p.parseResponse(response)
}

func (p *GeminiProvider) analyzeImage(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	imageData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image; %w", err)
	}

	mimeType := getMimeType(metadata.Type)

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

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Dimensions.Width,
		metadata.Dimensions.Height,
	)

	imagePart := genai.ImageData(mimeType, imageData)
	textPart := genai.Text(prompt)

	resp, err := p.model.GenerateContent(ctx, imagePart, textPart)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze image with Gemini; %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	response := extractTextFromParts(resp.Candidates[0].Content.Parts)
	return p.parseResponse(response)
}

func (p *GeminiProvider) analyzeDocument(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// For PDFs, Gemini supports native multimodal analysis
	if metadata.Type == "pdf" {
		return p.analyzePDF(ctx, metadata)
	}

	// For Office docs, fall back to metadata-only analysis
	// TODO: Add text extraction for PPTX/DOCX
	return p.analyzeBinary(metadata)
}

func (p *GeminiProvider) analyzePDF(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	pdfData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF; %w", err)
	}

	prompt := fmt.Sprintf(`Analyze this PDF document and provide semantic understanding.

File: %s
Pages: %d

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

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		getPageCount(metadata),
	)

	pdfPart := genai.Blob{
		MIMEType: "application/pdf",
		Data:     pdfData,
	}
	textPart := genai.Text(prompt)

	resp, err := p.model.GenerateContent(ctx, pdfPart, textPart)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze PDF with Gemini; %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	response := extractTextFromParts(resp.Candidates[0].Content.Parts)
	return p.parseResponse(response)
}

func (p *GeminiProvider) analyzeBinary(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
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
		Confidence:   0.5,
		Entities:     []types.Entity{},
		References:   []types.Reference{},
	}, nil
}

func (p *GeminiProvider) buildPrompt(metadata *types.FileMetadata, content string) string {
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

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Category,
		metadata.Size,
		content,
	)
}

func (p *GeminiProvider) parseResponse(response string) (*types.SemanticAnalysis, error) {
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

func extractTextFromParts(parts []genai.Part) string {
	var result strings.Builder
	for _, part := range parts {
		if text, ok := part.(genai.Text); ok {
			result.WriteString(string(text))
		}
	}
	return result.String()
}

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

func getMimeType(fileType string) string {
	switch strings.ToLower(fileType) {
	case ".png", "png":
		return "image/png"
	case ".jpg", ".jpeg", "jpg", "jpeg":
		return "image/jpeg"
	case ".gif", "gif":
		return "image/gif"
	case ".webp", "webp":
		return "image/webp"
	case ".heic", "heic":
		return "image/heic"
	case ".heif", "heif":
		return "image/heif"
	default:
		return "image/jpeg"
	}
}

func getPageCount(metadata *types.FileMetadata) int {
	if metadata.PageCount != nil {
		return *metadata.PageCount
	}
	return 0
}
