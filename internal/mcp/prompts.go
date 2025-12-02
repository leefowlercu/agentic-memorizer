package mcp

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// PromptRegistry manages MCP prompts for file analysis
type PromptRegistry struct {
	prompts map[string]*protocol.Prompt
}

// NewPromptRegistry creates a new prompt registry with default prompts
func NewPromptRegistry() *PromptRegistry {
	registry := &PromptRegistry{
		prompts: make(map[string]*protocol.Prompt),
	}

	// Register default prompts
	registry.registerPrompt("analyze-file", &protocol.Prompt{
		Name:        "analyze-file",
		Description: "Generate a detailed analysis of a specific file from the memory index",
		Arguments: []protocol.PromptArgument{
			{
				Name:        "file_path",
				Description: "Path to the file to analyze",
				Required:    true,
			},
		},
	})

	registry.registerPrompt("search-context", &protocol.Prompt{
		Name:        "search-context",
		Description: "Create a comprehensive search query to find relevant files",
		Arguments: []protocol.PromptArgument{
			{
				Name:        "topic",
				Description: "Topic or concept to search for",
				Required:    true,
			},
			{
				Name:        "category",
				Description: "Optional file category filter (documents, code, images, etc.)",
				Required:    false,
			},
		},
	})

	registry.registerPrompt("explain-summary", &protocol.Prompt{
		Name:        "explain-summary",
		Description: "Generate a detailed explanation of a file's semantic summary",
		Arguments: []protocol.PromptArgument{
			{
				Name:        "file_path",
				Description: "Path to the file whose summary to explain",
				Required:    true,
			},
		},
	})

	return registry
}

// registerPrompt adds a prompt to the registry
func (r *PromptRegistry) registerPrompt(name string, prompt *protocol.Prompt) {
	r.prompts[name] = prompt
}

// ListPrompts returns all registered prompts
func (r *PromptRegistry) ListPrompts() []protocol.Prompt {
	prompts := make([]protocol.Prompt, 0, len(r.prompts))
	for _, prompt := range r.prompts {
		prompts = append(prompts, *prompt)
	}
	return prompts
}

// GetPrompt retrieves a specific prompt by name
func (r *PromptRegistry) GetPrompt(name string) (*protocol.Prompt, error) {
	prompt, ok := r.prompts[name]
	if !ok {
		return nil, fmt.Errorf("prompt not found: %s", name)
	}
	return prompt, nil
}

// GeneratePromptMessages generates messages for a specific prompt with arguments
func (r *PromptRegistry) GeneratePromptMessages(name string, arguments map[string]string, server *Server) ([]protocol.PromptMessage, error) {
	prompt, err := r.GetPrompt(name)
	if err != nil {
		return nil, err
	}

	// Validate required arguments
	for _, arg := range prompt.Arguments {
		if arg.Required {
			if _, ok := arguments[arg.Name]; !ok {
				return nil, fmt.Errorf("missing required argument: %s", arg.Name)
			}
		}
	}

	// Generate messages based on prompt type
	switch name {
	case "analyze-file":
		return r.generateAnalyzeFileMessages(arguments, server)
	case "search-context":
		return r.generateSearchContextMessages(arguments, server)
	case "explain-summary":
		return r.generateExplainSummaryMessages(arguments, server)
	default:
		return nil, fmt.Errorf("unknown prompt: %s", name)
	}
}

// generateAnalyzeFileMessages generates messages for the analyze-file prompt
func (r *PromptRegistry) generateAnalyzeFileMessages(arguments map[string]string, server *Server) ([]protocol.PromptMessage, error) {
	filePath := arguments["file_path"]

	// Find file in index
	index := server.GetIndex()
	var found *types.FileEntry
	for i := range index.Files {
		if index.Files[i].Path == filePath {
			found = &index.Files[i]
			break
		}
	}

	if found == nil {
		return nil, fmt.Errorf("file not found in index: %s", filePath)
	}

	if found.Summary == "" {
		return nil, fmt.Errorf("file has no semantic analysis: %s", filePath)
	}

	// Build analysis message
	content := fmt.Sprintf(`Please provide a detailed analysis of this file:

**File:** %s
**Type:** %s
**Category:** %s
**Size:** %d bytes

**Summary:**
%s

**Tags:**
%s

**Key Topics:**
%s

Please explain:
1. What is the purpose of this file?
2. What are the main concepts or functionality it contains?
3. How might this file relate to other parts of the project?
4. Are there any notable patterns or approaches used?`,
		found.Path,
		found.Type,
		found.Category,
		found.Size,
		found.Summary,
		formatStringSlice(found.Tags),
		formatStringSlice(found.Topics),
	)

	return []protocol.PromptMessage{
		{
			Role: "user",
			Content: protocol.PromptContent{
				Type: "text",
				Text: content,
			},
		},
	}, nil
}

// generateSearchContextMessages generates messages for the search-context prompt
func (r *PromptRegistry) generateSearchContextMessages(arguments map[string]string, server *Server) ([]protocol.PromptMessage, error) {
	topic := arguments["topic"]
	category := arguments["category"]

	categoryFilter := ""
	if category != "" {
		categoryFilter = fmt.Sprintf("\n**Category Filter:** %s", category)
	}

	content := fmt.Sprintf(`I need to search the memory index for files related to:

**Topic:** %s%s

Please help me construct an effective search query by:
1. Identifying key terms and concepts related to this topic
2. Suggesting related tags that might be relevant
3. Recommending file types or categories to focus on
4. Proposing alternative search terms or synonyms

Based on these suggestions, please provide a ranked list of search strategies I should try.`,
		topic,
		categoryFilter,
	)

	return []protocol.PromptMessage{
		{
			Role: "user",
			Content: protocol.PromptContent{
				Type: "text",
				Text: content,
			},
		},
	}, nil
}

// generateExplainSummaryMessages generates messages for the explain-summary prompt
func (r *PromptRegistry) generateExplainSummaryMessages(arguments map[string]string, server *Server) ([]protocol.PromptMessage, error) {
	filePath := arguments["file_path"]

	// Find file in index
	index := server.GetIndex()
	var found *types.FileEntry
	for i := range index.Files {
		if index.Files[i].Path == filePath {
			found = &index.Files[i]
			break
		}
	}

	if found == nil {
		return nil, fmt.Errorf("file not found in index: %s", filePath)
	}

	if found.Summary == "" {
		return nil, fmt.Errorf("file has no semantic analysis: %s", filePath)
	}

	content := fmt.Sprintf(`Please provide a detailed explanation of this file's semantic analysis:

**File:** %s (%s)

**Summary:**
%s

**Tags:** %s
**Key Topics:** %s
**Document Type:** %s

Please explain:
1. What does this summary tell us about the file's content?
2. How were these tags and topics determined?
3. What is the significance of the document type classification?
4. How should I interpret and use this information?`,
		found.Path,
		found.Category,
		found.Summary,
		formatStringSlice(found.Tags),
		formatStringSlice(found.Topics),
		found.DocumentType,
	)

	return []protocol.PromptMessage{
		{
			Role: "user",
			Content: protocol.PromptContent{
				Type: "text",
				Text: content,
			},
		},
	}, nil
}

// formatStringSlice formats a slice of strings as a comma-separated list
func formatStringSlice(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ", "
		}
		result += item
	}
	return result
}
