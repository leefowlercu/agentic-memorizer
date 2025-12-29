package claude

import (
	"log/slog"
	"os"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
)

func TestNewClaudeProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		config  semantic.ProviderConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: semantic.ProviderConfig{
				APIKey:       "test-api-key",
				Model:        "claude-sonnet-4-5-20250929",
				MaxTokens:    4096,
				Timeout:      30,
				EnableVision: true,
				MaxFileSize:  10 * 1024 * 1024,
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: semantic.ProviderConfig{
				Model:     "claude-sonnet-4-5-20250929",
				MaxTokens: 4096,
			},
			wantErr: true,
		},
		{
			name: "missing model",
			config: semantic.ProviderConfig{
				APIKey:    "test-api-key",
				MaxTokens: 4096,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewClaudeProvider(tt.config, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClaudeProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("NewClaudeProvider() returned nil provider for valid config")
			}
		})
	}
}

func TestClaudeProvider_Name(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider, err := NewClaudeProvider(semantic.ProviderConfig{
		APIKey:    "test-key",
		Model:     "test-model",
		MaxTokens: 4096,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "claude")
	}
}

func TestClaudeProvider_Model(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	modelName := "claude-sonnet-4-5-20250929"
	provider, err := NewClaudeProvider(semantic.ProviderConfig{
		APIKey:    "test-key",
		Model:     modelName,
		MaxTokens: 4096,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Model() != modelName {
		t.Errorf("Model() = %q, want %q", provider.Model(), modelName)
	}
}

func TestClaudeProvider_SupportsVision(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider, err := NewClaudeProvider(semantic.ProviderConfig{
		APIKey:    "test-key",
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: 4096,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Claude supports vision API
	if !provider.SupportsVision() {
		t.Error("SupportsVision() = false, want true for Claude")
	}
}

func TestClaudeProvider_SupportsDocuments(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider, err := NewClaudeProvider(semantic.ProviderConfig{
		APIKey:    "test-key",
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: 4096,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Claude supports document content blocks
	if !provider.SupportsDocuments() {
		t.Error("SupportsDocuments() = false, want true for Claude")
	}
}

func TestClaudeProvider_ImplementsInterface(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider, err := NewClaudeProvider(semantic.ProviderConfig{
		APIKey:    "test-key",
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: 4096,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Verify it implements the Provider interface
	var _ semantic.Provider = provider
}
