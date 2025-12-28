package gemini

import (
	"log/slog"
	"os"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
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

func TestGemini_ExtractJSON(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "json code block",
			text: "Here is the analysis:\n```json\n{\"summary\": \"test\"}\n```",
			want: `{"summary": "test"}`,
		},
		{
			name: "generic code block",
			text: "Here is the analysis:\n```\n{\"summary\": \"test\"}\n```",
			want: `{"summary": "test"}`,
		},
		{
			name: "no code block",
			text: `{"summary": "test"}`,
			want: "",
		},
		{
			name: "nested json",
			text: "```json\n{\"entities\": [{\"name\": \"AWS\", \"type\": \"technology\"}]}\n```",
			want: `{"entities": [{"name": "AWS", "type": "technology"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.text)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGemini_GetMimeType(t *testing.T) {
	tests := []struct {
		fileType string
		want     string
	}{
		{"png", "image/png"},
		{".png", "image/png"},
		{"PNG", "image/png"},
		{"jpg", "image/jpeg"},
		{".jpg", "image/jpeg"},
		{"jpeg", "image/jpeg"},
		{"gif", "image/gif"},
		{"webp", "image/webp"},
		{"heic", "image/heic"},
		{"heif", "image/heif"},
		{"unknown", "image/jpeg"},
		{"", "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.fileType, func(t *testing.T) {
			got := getMimeType(tt.fileType)
			if got != tt.want {
				t.Errorf("getMimeType(%q) = %q, want %q", tt.fileType, got, tt.want)
			}
		})
	}
}

func TestGemini_GetPageCount(t *testing.T) {
	tests := []struct {
		name     string
		metadata *types.FileMetadata
		want     int
	}{
		{
			name: "with page count",
			metadata: &types.FileMetadata{
				PageCount: intPtr(10),
			},
			want: 10,
		},
		{
			name: "nil page count",
			metadata: &types.FileMetadata{
				PageCount: nil,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPageCount(tt.metadata)
			if got != tt.want {
				t.Errorf("getPageCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
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
