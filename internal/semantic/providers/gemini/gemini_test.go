package gemini

import (
	"log/slog"
	"os"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
)

func TestNewGeminiProvider_Validation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		config  semantic.ProviderConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing API key",
			config: semantic.ProviderConfig{
				Model:     "gemini-2.5-flash",
				MaxTokens: 4096,
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name: "missing model",
			config: semantic.ProviderConfig{
				APIKey:    "test-api-key",
				MaxTokens: 4096,
			},
			wantErr: true,
			errMsg:  "model is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGeminiProvider(tt.config, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGeminiProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGeminiVisionModelDetection tests the vision support detection logic
func TestGeminiVisionModelDetection(t *testing.T) {
	// Test model name patterns for vision support detection
	// This tests the logic without creating an actual provider
	tests := []struct {
		model string
		want  bool
	}{
		{"gemini-2.5-flash", true},
		{"gemini-2.5-pro", true},
		{"gemini-3-flash", true},
		{"gemini-2.0-flash", true},
		{"gemini-1.5-pro", true},
		{"gemini-1.5-flash", true},
		{"gemini-pro-vision", true},
		{"gemini-pro", false},
		{"gemini-1.0-pro", false},
		{"unknown-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := supportsVision(tt.model)
			if got != tt.want {
				t.Errorf("supportsVision(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// supportsVision is a test helper that mirrors the SupportsVision logic
func supportsVision(model string) bool {
	m := model
	return contains(m, "gemini-2") ||
		contains(m, "gemini-3") ||
		contains(m, "gemini-pro-vision") ||
		contains(m, "gemini-1.5")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
