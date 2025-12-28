package prompts

import (
	"fmt"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// BuildTextPrompt creates a prompt for analyzing text files.
// Provider-agnostic JSON output format ensures consistent analysis across providers.
func BuildTextPrompt(metadata *types.FileMetadata, content string) string {
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
