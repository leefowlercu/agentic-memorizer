package common

import (
	"fmt"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// TextAnalysisPromptTemplate is the prompt template for text file analysis.
const TextAnalysisPromptTemplate = `Analyze this file and provide semantic understanding.

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

Be concise but informative. Respond with ONLY valid JSON.`

// ImageAnalysisPromptTemplate is the prompt template for image analysis.
const ImageAnalysisPromptTemplate = `Analyze this image and provide semantic understanding.

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

Be concise but informative. Respond with ONLY valid JSON.`

// DocumentAnalysisPromptTemplate is the prompt template for document analysis (PDF, PPTX, DOCX).
const DocumentAnalysisPromptTemplate = `Analyze this %s document and provide semantic understanding.

File: %s
Type: %s
%s

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

// BuildTextPrompt builds a prompt for text file analysis.
func BuildTextPrompt(metadata *types.FileMetadata, content string) string {
	return fmt.Sprintf(TextAnalysisPromptTemplate,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Category,
		metadata.Size,
		content,
	)
}

// BuildImagePrompt builds a prompt for image analysis.
func BuildImagePrompt(metadata *types.FileMetadata) string {
	return fmt.Sprintf(ImageAnalysisPromptTemplate,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Dimensions.Width,
		metadata.Dimensions.Height,
	)
}

// BuildPptxPrompt builds a prompt for PowerPoint presentation analysis.
func BuildPptxPrompt(metadata *types.FileMetadata, extractedText string) string {
	slideInfo := ""
	if metadata.SlideCount != nil {
		slideInfo = fmt.Sprintf("Slides: %d", *metadata.SlideCount)
	}

	return fmt.Sprintf(DocumentAnalysisPromptTemplate,
		"PowerPoint presentation",
		filepath.Base(metadata.Path),
		"PPTX",
		slideInfo+"\n\nExtracted text content:\n"+extractedText,
	)
}

// BuildDocxPrompt builds a prompt for Word document analysis.
func BuildDocxPrompt(metadata *types.FileMetadata, extractedText string) string {
	wordInfo := ""
	if metadata.WordCount != nil {
		wordInfo = fmt.Sprintf("Word Count: %d", *metadata.WordCount)
	}

	return fmt.Sprintf(DocumentAnalysisPromptTemplate,
		"Word document",
		filepath.Base(metadata.Path),
		"DOCX",
		wordInfo+"\n\nExtracted text content:\n"+extractedText,
	)
}

// BuildPdfPrompt builds a prompt for PDF document analysis.
func BuildPdfPrompt(metadata *types.FileMetadata) string {
	pageInfo := fmt.Sprintf("Size: %d bytes", metadata.Size)
	if metadata.PageCount != nil {
		pageInfo += fmt.Sprintf("\nPages: %d", *metadata.PageCount)
	}

	return fmt.Sprintf(DocumentAnalysisPromptTemplate,
		"PDF",
		filepath.Base(metadata.Path),
		"PDF",
		pageInfo,
	)
}
