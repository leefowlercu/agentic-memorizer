package openai

import (
	"log/slog"
	"os"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
)

func TestNewOpenAIProvider(t *testing.T) {
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
				Model:        "gpt-4o",
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
				Model:     "gpt-4o",
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
			provider, err := NewOpenAIProvider(tt.config, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("NewOpenAIProvider() returned nil provider for valid config")
			}
		})
	}
}

func TestOpenAIProvider_Name(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider, err := NewOpenAIProvider(semantic.ProviderConfig{
		APIKey:    "test-key",
		Model:     "gpt-4o",
		MaxTokens: 4096,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "openai")
	}
}

func TestOpenAIProvider_Model(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	modelName := "gpt-5.2-chat-latest"
	provider, err := NewOpenAIProvider(semantic.ProviderConfig{
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

func TestOpenAIProvider_SupportsVision(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"gpt-5.2", true},
		{"gpt-5.2-chat-latest", true},
		{"gpt-4-vision", true},
		{"gpt-4-vision-preview", true},
		{"gpt-4", false},
		{"gpt-3.5-turbo", false},
		{"unknown-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider, err := NewOpenAIProvider(semantic.ProviderConfig{
				APIKey:    "test-key",
				Model:     tt.model,
				MaxTokens: 4096,
			}, logger)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			if provider.SupportsVision() != tt.want {
				t.Errorf("SupportsVision() for model %q = %v, want %v", tt.model, provider.SupportsVision(), tt.want)
			}
		})
	}
}

func TestOpenAIProvider_SupportsDocuments(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider, err := NewOpenAIProvider(semantic.ProviderConfig{
		APIKey:    "test-key",
		Model:     "gpt-4o",
		MaxTokens: 4096,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// OpenAI doesn't have native PDF support
	if provider.SupportsDocuments() {
		t.Error("SupportsDocuments() = true, want false for OpenAI")
	}
}

func TestOpenAIProvider_ImplementsInterface(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	provider, err := NewOpenAIProvider(semantic.ProviderConfig{
		APIKey:    "test-key",
		Model:     "gpt-4o",
		MaxTokens: 4096,
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Verify it implements the Provider interface
	var _ semantic.Provider = provider
}
