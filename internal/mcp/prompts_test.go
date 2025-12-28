package mcp

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestPromptRegistry_NewPromptRegistry(t *testing.T) {
	registry := NewPromptRegistry()

	if registry == nil {
		t.Fatal("NewPromptRegistry returned nil")
	}

	if registry.prompts == nil {
		t.Error("Expected prompts map to be initialized")
	}

	// Should have 3 default prompts
	prompts := registry.ListPrompts()
	if len(prompts) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(prompts))
	}
}

func TestPromptRegistry_ListPrompts(t *testing.T) {
	registry := NewPromptRegistry()
	prompts := registry.ListPrompts()

	// Verify count
	if len(prompts) != 3 {
		t.Fatalf("Expected 3 prompts, got %d", len(prompts))
	}

	// Verify all expected prompts are present
	expectedPrompts := map[string]bool{
		"analyze-file":    false,
		"search-context":  false,
		"explain-summary": false,
	}

	for _, prompt := range prompts {
		if _, ok := expectedPrompts[prompt.Name]; ok {
			expectedPrompts[prompt.Name] = true
		} else {
			t.Errorf("Unexpected prompt found: %s", prompt.Name)
		}
	}

	for name, found := range expectedPrompts {
		if !found {
			t.Errorf("Expected prompt %q not found in list", name)
		}
	}
}

func TestPromptRegistry_GetPrompt(t *testing.T) {
	registry := NewPromptRegistry()

	tests := []struct {
		name        string
		promptName  string
		shouldExist bool
	}{
		{
			name:        "get analyze-file prompt",
			promptName:  "analyze-file",
			shouldExist: true,
		},
		{
			name:        "get search-context prompt",
			promptName:  "search-context",
			shouldExist: true,
		},
		{
			name:        "get explain-summary prompt",
			promptName:  "explain-summary",
			shouldExist: true,
		},
		{
			name:        "get nonexistent prompt",
			promptName:  "nonexistent-prompt",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := registry.GetPrompt(tt.promptName)

			if tt.shouldExist {
				if err != nil {
					t.Errorf("Expected to find prompt %q, got error: %v", tt.promptName, err)
				}
				if prompt.Name != tt.promptName {
					t.Errorf("Expected prompt name %q, got %q", tt.promptName, prompt.Name)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for nonexistent prompt %q, got nil", tt.promptName)
				}
			}
		})
	}
}

func TestPromptRegistry_AnalyzeFilePrompt(t *testing.T) {
	registry := NewPromptRegistry()

	prompt, err := registry.GetPrompt("analyze-file")
	if err != nil {
		t.Fatalf("Failed to get analyze-file prompt: %v", err)
	}

	// Verify prompt structure
	if prompt.Name != "analyze-file" {
		t.Errorf("Expected name 'analyze-file', got %q", prompt.Name)
	}

	if prompt.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Verify arguments
	if len(prompt.Arguments) == 0 {
		t.Fatal("Expected at least one argument")
	}

	// Check for file_path argument (required)
	hasFilePath := false
	for _, arg := range prompt.Arguments {
		if arg.Name == "file_path" {
			hasFilePath = true
			if !arg.Required {
				t.Error("Expected file_path argument to be required")
			}
			if arg.Description == "" {
				t.Error("Expected file_path to have a description")
			}
		}
	}

	if !hasFilePath {
		t.Error("Expected analyze-file prompt to have file_path argument")
	}
}

func TestPromptRegistry_SearchContextPrompt(t *testing.T) {
	registry := NewPromptRegistry()

	prompt, err := registry.GetPrompt("search-context")
	if err != nil {
		t.Fatalf("Failed to get search-context prompt: %v", err)
	}

	// Verify prompt structure
	if prompt.Name != "search-context" {
		t.Errorf("Expected name 'search-context', got %q", prompt.Name)
	}

	if prompt.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Verify arguments
	if len(prompt.Arguments) == 0 {
		t.Fatal("Expected at least one argument")
	}

	// Check for topic argument (required)
	hasTopic := false
	for _, arg := range prompt.Arguments {
		if arg.Name == "topic" {
			hasTopic = true
			if !arg.Required {
				t.Error("Expected topic argument to be required")
			}
		}
		// category should be optional
		if arg.Name == "category" && arg.Required {
			t.Error("Expected category argument to be optional")
		}
	}

	if !hasTopic {
		t.Error("Expected search-context prompt to have topic argument")
	}
}

func TestPromptRegistry_ExplainSummaryPrompt(t *testing.T) {
	registry := NewPromptRegistry()

	prompt, err := registry.GetPrompt("explain-summary")
	if err != nil {
		t.Fatalf("Failed to get explain-summary prompt: %v", err)
	}

	// Verify prompt structure
	if prompt.Name != "explain-summary" {
		t.Errorf("Expected name 'explain-summary', got %q", prompt.Name)
	}

	if prompt.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Verify arguments
	if len(prompt.Arguments) == 0 {
		t.Fatal("Expected at least one argument")
	}

	// Check for file_path argument (required)
	hasFilePath := false
	for _, arg := range prompt.Arguments {
		if arg.Name == "file_path" {
			hasFilePath = true
			if !arg.Required {
				t.Error("Expected file_path argument to be required")
			}
		}
	}

	if !hasFilePath {
		t.Error("Expected explain-summary prompt to have file_path argument")
	}
}

func TestPromptRegistry_GeneratePromptMessages_MissingRequiredArgument(t *testing.T) {
	registry := NewPromptRegistry()

	// Create minimal server for testing
	index := &types.FileIndex{
		Files: []types.FileEntry{},
	}
	server := &Server{
		index:          index,
		promptRegistry: registry,
	}

	tests := []struct {
		name       string
		promptName string
		arguments  map[string]string
	}{
		{
			name:       "analyze-file missing file_path",
			promptName: "analyze-file",
			arguments:  map[string]string{},
		},
		{
			name:       "search-context missing topic",
			promptName: "search-context",
			arguments:  map[string]string{},
		},
		{
			name:       "explain-summary missing file_path",
			promptName: "explain-summary",
			arguments:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registry.GeneratePromptMessages(tt.promptName, tt.arguments, server)
			if err == nil {
				t.Error("Expected error for missing required argument")
			}
		})
	}
}

func TestPromptRegistry_GeneratePromptMessages_FileNotFound(t *testing.T) {
	registry := NewPromptRegistry()

	// Create empty index
	index := &types.FileIndex{
		Files: []types.FileEntry{},
	}
	server := &Server{
		index:          index,
		promptRegistry: registry,
	}

	tests := []struct {
		name       string
		promptName string
		arguments  map[string]string
	}{
		{
			name:       "analyze-file with nonexistent file",
			promptName: "analyze-file",
			arguments: map[string]string{
				"file_path": "/nonexistent/file.txt",
			},
		},
		{
			name:       "explain-summary with nonexistent file",
			promptName: "explain-summary",
			arguments: map[string]string{
				"file_path": "/nonexistent/file.txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registry.GeneratePromptMessages(tt.promptName, tt.arguments, server)
			if err == nil {
				t.Error("Expected error for file not found in index")
			}
		})
	}
}

func TestPromptRegistry_GeneratePromptMessages_SearchContext(t *testing.T) {
	registry := NewPromptRegistry()

	// Create minimal server (search-context doesn't need index data)
	index := &types.FileIndex{
		Files: []types.FileEntry{},
	}
	server := &Server{
		index:          index,
		promptRegistry: registry,
	}

	tests := []struct {
		name      string
		arguments map[string]string
	}{
		{
			name: "with topic only",
			arguments: map[string]string{
				"topic": "authentication",
			},
		},
		{
			name: "with topic and category",
			arguments: map[string]string{
				"topic":    "authentication",
				"category": "code",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := registry.GeneratePromptMessages("search-context", tt.arguments, server)
			if err != nil {
				t.Fatalf("Expected successful generation, got error: %v", err)
			}

			if len(messages) == 0 {
				t.Error("Expected at least one message")
			}

			// Verify message structure
			for i, msg := range messages {
				if msg.Role == "" {
					t.Errorf("Message %d has empty role", i)
				}
				if msg.Content.Type == "" {
					t.Errorf("Message %d has empty content type", i)
				}
				if msg.Content.Text == "" {
					t.Errorf("Message %d has empty text content", i)
				}
			}
		})
	}
}

func TestPromptRegistry_GeneratePromptMessages_WithIndexData(t *testing.T) {
	registry := NewPromptRegistry()

	// Create index with test file
	index := &types.FileIndex{
		Files: []types.FileEntry{
			{
				Path:         "/test/file.txt",
				Name:         "file.txt",
				Type:         "text/plain",
				Category:     "documents",
				Size:         1024,
				Summary:      "A test file containing sample data",
				Tags:         []string{"test", "sample"},
				Topics:       []string{"testing", "examples"},
				DocumentType: "text document",
			},
		},
	}

	server := &Server{
		index:          index,
		promptRegistry: registry,
	}

	tests := []struct {
		name       string
		promptName string
		arguments  map[string]string
	}{
		{
			name:       "analyze-file",
			promptName: "analyze-file",
			arguments: map[string]string{
				"file_path": "/test/file.txt",
			},
		},
		{
			name:       "explain-summary",
			promptName: "explain-summary",
			arguments: map[string]string{
				"file_path": "/test/file.txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := registry.GeneratePromptMessages(tt.promptName, tt.arguments, server)
			if err != nil {
				t.Fatalf("Expected successful generation, got error: %v", err)
			}

			if len(messages) == 0 {
				t.Fatal("Expected at least one message")
			}

			// Verify first message
			msg := messages[0]
			if msg.Role != "user" {
				t.Errorf("Expected role 'user', got %q", msg.Role)
			}

			if msg.Content.Type != "text" {
				t.Errorf("Expected content type 'text', got %q", msg.Content.Type)
			}

			if msg.Content.Text == "" {
				t.Error("Expected non-empty text content")
			}

			// Verify message includes semantic data
			text := msg.Content.Text
			if tt.promptName == "analyze-file" || tt.promptName == "explain-summary" {
				// Should include summary
				if len(text) < 50 {
					t.Error("Expected substantial prompt text with semantic data")
				}
			}
		})
	}
}

func TestPromptRegistry_GeneratePromptMessages_FileWithoutSemanticAnalysis(t *testing.T) {
	registry := NewPromptRegistry()

	// Create index with file that has no semantic analysis (empty Summary)
	index := &types.FileIndex{
		Files: []types.FileEntry{
			{
				Path:     "/test/file.txt",
				Name:     "file.txt",
				Type:     "text/plain",
				Category: "documents",
				Size:     1024,
				Summary:  "", // No semantic analysis
			},
		},
	}

	server := &Server{
		index:          index,
		promptRegistry: registry,
	}

	tests := []struct {
		name       string
		promptName string
	}{
		{
			name:       "analyze-file without semantic data",
			promptName: "analyze-file",
		},
		{
			name:       "explain-summary without semantic data",
			promptName: "explain-summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := map[string]string{
				"file_path": "/test/file.txt",
			}

			_, err := registry.GeneratePromptMessages(tt.promptName, arguments, server)
			if err == nil {
				t.Error("Expected error for file without semantic analysis")
			}
		})
	}
}

func TestPromptRegistry_GeneratePromptMessages_UnknownPrompt(t *testing.T) {
	registry := NewPromptRegistry()

	index := &types.FileIndex{
		Files: []types.FileEntry{},
	}
	server := &Server{
		index:          index,
		promptRegistry: registry,
	}

	_, err := registry.GeneratePromptMessages("nonexistent-prompt", map[string]string{}, server)
	if err == nil {
		t.Error("Expected error for unknown prompt")
	}
}

func TestPromptRegistry_AllPromptsHaveDescriptions(t *testing.T) {
	registry := NewPromptRegistry()
	prompts := registry.ListPrompts()

	for _, prompt := range prompts {
		if prompt.Description == "" {
			t.Errorf("Prompt %q has empty description", prompt.Name)
		}
	}
}

func TestPromptRegistry_AllPromptsHaveArguments(t *testing.T) {
	registry := NewPromptRegistry()
	prompts := registry.ListPrompts()

	for _, prompt := range prompts {
		if len(prompt.Arguments) == 0 {
			t.Errorf("Prompt %q has no arguments", prompt.Name)
		}

		// Verify all arguments have descriptions
		for _, arg := range prompt.Arguments {
			if arg.Description == "" {
				t.Errorf("Prompt %q argument %q has empty description", prompt.Name, arg.Name)
			}
		}
	}
}

func TestPromptRegistry_PromptMessageFormat(t *testing.T) {
	registry := NewPromptRegistry()

	// Create index with test data
	index := &types.FileIndex{
		Files: []types.FileEntry{
			{
				Path:     "/test/file.txt",
				Name:     "file.txt",
				Type:     "text/plain",
				Category: "documents",
				Size:     1024,
				Summary:  "Test summary",
				Tags:     []string{"test"},
				Topics:   []string{"testing"},
			},
		},
	}

	server := &Server{
		index:          index,
		promptRegistry: registry,
	}

	// Test all prompt types return valid protocol.PromptMessage format
	tests := []struct {
		name       string
		promptName string
		arguments  map[string]string
	}{
		{
			name:       "analyze-file",
			promptName: "analyze-file",
			arguments:  map[string]string{"file_path": "/test/file.txt"},
		},
		{
			name:       "search-context",
			promptName: "search-context",
			arguments:  map[string]string{"topic": "testing"},
		},
		{
			name:       "explain-summary",
			promptName: "explain-summary",
			arguments:  map[string]string{"file_path": "/test/file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := registry.GeneratePromptMessages(tt.promptName, tt.arguments, server)
			if err != nil {
				t.Fatalf("Failed to generate messages: %v", err)
			}

			for i, msg := range messages {
				// Verify it's a valid PromptMessage
				var _ protocol.PromptMessage = msg

				// Role must be "user" or "assistant"
				if msg.Role != "user" && msg.Role != "assistant" {
					t.Errorf("Message %d has invalid role: %q", i, msg.Role)
				}

				// Content must have type and text
				if msg.Content.Type == "" {
					t.Errorf("Message %d has empty content type", i)
				}
				if msg.Content.Type == "text" && msg.Content.Text == "" {
					t.Errorf("Message %d has text content type but empty text", i)
				}
			}
		})
	}
}
