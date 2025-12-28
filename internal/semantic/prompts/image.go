package prompts

import (
	"fmt"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// BuildImagePrompt creates a prompt for analyzing images via vision API.
// Provider-agnostic JSON output format ensures consistent analysis across providers.
func BuildImagePrompt(metadata *types.FileMetadata) string {
	prompt := fmt.Sprintf(`Analyze this image and provide semantic understanding.

File: %s
Type: %s`,
		filepath.Base(metadata.Path),
		metadata.Type,
	)

	// Add dimensions if available
	if metadata.Dimensions.Width > 0 && metadata.Dimensions.Height > 0 {
		prompt += fmt.Sprintf("\nDimensions: %dx%d", metadata.Dimensions.Width, metadata.Dimensions.Height)
	}

	prompt += `

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

	return prompt
}
