package shared

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

func TestGetFilesCommand(t *testing.T) {
	tests := []struct {
		name            string
		binaryPath      string
		format          integrations.OutputFormat
		integrationName string
		expected        string
	}{
		{
			name:            "xml format",
			binaryPath:      "/usr/local/bin/memorizer",
			format:          integrations.FormatXML,
			integrationName: "claude-code-hook",
			expected:        "/usr/local/bin/memorizer read files --format xml --integration claude-code-hook",
		},
		{
			name:            "markdown format",
			binaryPath:      "~/.local/bin/memorizer",
			format:          integrations.FormatMarkdown,
			integrationName: "gemini-cli-hook",
			expected:        "~/.local/bin/memorizer read files --format markdown --integration gemini-cli-hook",
		},
		{
			name:            "json format",
			binaryPath:      "memorizer",
			format:          integrations.FormatJSON,
			integrationName: "test-integration",
			expected:        "memorizer read files --format json --integration test-integration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFilesCommand(tt.binaryPath, tt.format, tt.integrationName)
			if result != tt.expected {
				t.Errorf("GetFilesCommand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetFactsCommand(t *testing.T) {
	tests := []struct {
		name            string
		binaryPath      string
		format          integrations.OutputFormat
		integrationName string
		expected        string
	}{
		{
			name:            "xml format",
			binaryPath:      "/usr/local/bin/memorizer",
			format:          integrations.FormatXML,
			integrationName: "claude-code-hook",
			expected:        "/usr/local/bin/memorizer read facts --format xml --integration claude-code-hook",
		},
		{
			name:            "markdown format",
			binaryPath:      "~/.local/bin/memorizer",
			format:          integrations.FormatMarkdown,
			integrationName: "gemini-cli-hook",
			expected:        "~/.local/bin/memorizer read facts --format markdown --integration gemini-cli-hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFactsCommand(tt.binaryPath, tt.format, tt.integrationName)
			if result != tt.expected {
				t.Errorf("GetFactsCommand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestContainsMemorizer(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "current binary name",
			command:  "/usr/local/bin/memorizer read files",
			expected: true,
		},
		{
			name:     "old binary name",
			command:  "/usr/local/bin/agentic-memorizer read files",
			expected: true,
		},
		{
			name:     "other command",
			command:  "echo hello",
			expected: false,
		},
		{
			name:     "empty command",
			command:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsMemorizer(tt.command)
			if result != tt.expected {
				t.Errorf("ContainsMemorizer(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestContainsOldBinaryName(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "old binary name",
			command:  "/usr/local/bin/agentic-memorizer read files",
			expected: true,
		},
		{
			name:     "current binary name",
			command:  "/usr/local/bin/memorizer read files",
			expected: false,
		},
		{
			name:     "other command",
			command:  "echo hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsOldBinaryName(tt.command)
			if result != tt.expected {
				t.Errorf("ContainsOldBinaryName(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestIsMemorizerHook(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "current binary name",
			command:  "/usr/local/bin/memorizer read files",
			expected: true,
		},
		{
			name:     "old binary name - should return false",
			command:  "/usr/local/bin/agentic-memorizer read files",
			expected: false,
		},
		{
			name:     "other command",
			command:  "echo hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMemorizerHook(tt.command)
			if result != tt.expected {
				t.Errorf("IsMemorizerHook(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestAggregateErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []string
		expected string
	}{
		{
			name:     "single error",
			errors:   []string{"error 1"},
			expected: "error 1",
		},
		{
			name:     "multiple errors",
			errors:   []string{"error 1", "error 2", "error 3"},
			expected: "error 1; error 2; error 3",
		},
		{
			name:     "empty errors",
			errors:   []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggregateErrors(tt.errors)
			if result != tt.expected {
				t.Errorf("AggregateErrors() = %q, want %q", result, tt.expected)
			}
		})
	}
}
