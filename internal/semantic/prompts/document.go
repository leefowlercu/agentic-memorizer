package prompts

import (
	"fmt"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// BuildPDFPrompt creates a prompt for analyzing PDF documents.
// Used with providers that support native PDF document blocks.
func BuildPDFPrompt(metadata *types.FileMetadata) string {
	prompt := fmt.Sprintf(`Analyze this PDF document and provide semantic understanding.

File: %s
Type: PDF
Size: %d bytes`,
		filepath.Base(metadata.Path),
		metadata.Size,
	)

	// Add page count if available
	if metadata.PageCount != nil {
		prompt += fmt.Sprintf("\nPages: %d", *metadata.PageCount)
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

	return prompt
}

// BuildPptxPrompt creates a prompt for analyzing PowerPoint presentations.
// Used with extracted text content from PPTX files.
func BuildPptxPrompt(metadata *types.FileMetadata, extractedText string) string {
	prompt := fmt.Sprintf(`Analyze this PowerPoint presentation and provide semantic understanding.

File: %s
Type: PPTX`,
		filepath.Base(metadata.Path),
	)

	// Add slide count if available
	if metadata.SlideCount != nil {
		prompt += fmt.Sprintf("\nSlides: %d", *metadata.SlideCount)
	}

	prompt += fmt.Sprintf(`

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
		extractedText,
	)

	return prompt
}

// BuildDocxPrompt creates a prompt for analyzing Word documents.
// Used with extracted text content from DOCX files.
func BuildDocxPrompt(metadata *types.FileMetadata, extractedText string) string {
	prompt := fmt.Sprintf(`Analyze this Word document and provide semantic understanding.

File: %s
Type: DOCX`,
		filepath.Base(metadata.Path),
	)

	// Add word count if available
	if metadata.WordCount != nil {
		prompt += fmt.Sprintf("\nWord Count: %d", *metadata.WordCount)
	}

	prompt += fmt.Sprintf(`

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
		extractedText,
	)

	return prompt
}
