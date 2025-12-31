package gemini

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/embeddings"
)

func TestNewProvider_MissingAPIKey(t *testing.T) {
	_, err := NewProvider(embeddings.ProviderConfig{
		Model:      "text-embedding-004",
		Dimensions: 768,
	}, nil)
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestProvider_Defaults(t *testing.T) {
	// Can't test with real client without API key, but we can verify constants
	if DefaultModel != "text-embedding-004" {
		t.Errorf("DefaultModel = %v, want text-embedding-004", DefaultModel)
	}
	if DefaultDimensions != 768 {
		t.Errorf("DefaultDimensions = %v, want 768", DefaultDimensions)
	}
	if DefaultRateLimitRPM != 1500 {
		t.Errorf("DefaultRateLimitRPM = %v, want 1500", DefaultRateLimitRPM)
	}
	if MaxBatchSize != 100 {
		t.Errorf("MaxBatchSize = %v, want 100", MaxBatchSize)
	}
	if ProviderName != "gemini" {
		t.Errorf("ProviderName = %v, want gemini", ProviderName)
	}
}

func TestProvider_InterfaceCompliance(t *testing.T) {
	// Verify Provider implements embeddings.Provider at compile time
	var _ embeddings.Provider = (*Provider)(nil)
}
