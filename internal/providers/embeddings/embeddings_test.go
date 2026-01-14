package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

func TestOpenAIEmbeddingsProvider_EmbedBatch(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// Parse request body
		var reqBody struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Return mock response with embeddings for each input
		resp := openaiEmbeddingsResponse{
			Usage: struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			}{
				PromptTokens: 10,
				TotalTokens:  10,
			},
		}

		for i := range reqBody.Input {
			resp.Data = append(resp.Data, struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				Embedding: []float64{0.1, 0.2, 0.3},
				Index:     i,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with mock server
	p := &OpenAIEmbeddingsProvider{
		apiKey:     "test-key",
		model:      "text-embedding-3-small",
		dimensions: 3,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	p.rateLimiter = providers.NewRateLimiter(p.RateLimit())

	// Patch the URL - need to use a custom client or modify for testing
	// For now, test that the interface is implemented correctly
	t.Run("interface compliance", func(t *testing.T) {
		var _ providers.EmbeddingsProvider = p
	})

	t.Run("empty input", func(t *testing.T) {
		results, err := p.EmbedBatch(context.Background(), []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results != nil && len(results) != 0 {
			t.Errorf("expected nil or empty results, got %v", results)
		}
	})
}

func TestVoyageEmbeddingsProvider_InterfaceCompliance(t *testing.T) {
	p := NewVoyageEmbeddingsProvider()
	var _ providers.EmbeddingsProvider = p
}

func TestGoogleEmbeddingsProvider_InterfaceCompliance(t *testing.T) {
	p := NewGoogleEmbeddingsProvider()
	var _ providers.EmbeddingsProvider = p
}

func TestOpenAIEmbeddingsProvider_EmbedBatch_NotAvailable(t *testing.T) {
	p := &OpenAIEmbeddingsProvider{
		apiKey: "", // Not available
	}

	_, err := p.EmbedBatch(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error when provider not available")
	}
}

func TestVoyageEmbeddingsProvider_EmbedBatch_NotAvailable(t *testing.T) {
	p := &VoyageEmbeddingsProvider{
		apiKey: "", // Not available
	}

	_, err := p.EmbedBatch(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error when provider not available")
	}
}

func TestGoogleEmbeddingsProvider_EmbedBatch_NotAvailable(t *testing.T) {
	p := &GoogleEmbeddingsProvider{
		apiKey: "", // Not available
	}

	_, err := p.EmbedBatch(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error when provider not available")
	}
}
