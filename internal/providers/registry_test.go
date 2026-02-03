package providers

import (
	"context"
	"testing"
)

// mockSemanticProvider implements SemanticProvider for testing.
type mockSemanticProvider struct {
	name      string
	available bool
}

func (p *mockSemanticProvider) Name() string               { return p.name }
func (p *mockSemanticProvider) Type() ProviderType         { return ProviderTypeSemantic }
func (p *mockSemanticProvider) Available() bool            { return p.available }
func (p *mockSemanticProvider) RateLimit() RateLimitConfig { return RateLimitConfig{} }
func (p *mockSemanticProvider) ModelName() string          { return "mock-model" }
func (p *mockSemanticProvider) Capabilities() SemanticCapabilities {
	return SemanticCapabilities{}
}
func (p *mockSemanticProvider) Analyze(ctx context.Context, input SemanticInput) (*SemanticResult, error) {
	return nil, nil
}

// mockEmbeddingsProvider implements EmbeddingsProvider for testing.
type mockEmbeddingsProvider struct {
	name      string
	available bool
}

func (p *mockEmbeddingsProvider) Name() string               { return p.name }
func (p *mockEmbeddingsProvider) Type() ProviderType         { return ProviderTypeEmbeddings }
func (p *mockEmbeddingsProvider) Available() bool            { return p.available }
func (p *mockEmbeddingsProvider) RateLimit() RateLimitConfig { return RateLimitConfig{} }
func (p *mockEmbeddingsProvider) ModelName() string          { return "mock-model" }
func (p *mockEmbeddingsProvider) Dimensions() int            { return 1536 }
func (p *mockEmbeddingsProvider) MaxTokens() int             { return 8000 }
func (p *mockEmbeddingsProvider) Embed(ctx context.Context, req EmbeddingsRequest) (*EmbeddingsResult, error) {
	return nil, nil
}
func (p *mockEmbeddingsProvider) EmbedBatch(ctx context.Context, texts []string) ([]EmbeddingsBatchResult, error) {
	return nil, nil
}

func TestRegistry_RegisterSemantic(t *testing.T) {
	r := NewRegistry()

	p := &mockSemanticProvider{name: "test", available: true}
	err := r.RegisterSemantic(p)
	if err != nil {
		t.Fatalf("RegisterSemantic failed: %v", err)
	}

	// Duplicate registration should fail
	err = r.RegisterSemantic(p)
	if err != ErrProviderExists {
		t.Errorf("expected ErrProviderExists, got %v", err)
	}
}

func TestRegistry_RegisterEmbeddings(t *testing.T) {
	r := NewRegistry()

	p := &mockEmbeddingsProvider{name: "test", available: true}
	err := r.RegisterEmbeddings(p)
	if err != nil {
		t.Fatalf("RegisterEmbeddings failed: %v", err)
	}

	// Duplicate registration should fail
	err = r.RegisterEmbeddings(p)
	if err != ErrProviderExists {
		t.Errorf("expected ErrProviderExists, got %v", err)
	}
}

func TestRegistry_GetSemantic(t *testing.T) {
	r := NewRegistry()

	p := &mockSemanticProvider{name: "test", available: true}
	_ = r.RegisterSemantic(p)

	// Get existing provider
	got, err := r.GetSemantic("test")
	if err != nil {
		t.Fatalf("GetSemantic failed: %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("expected name 'test', got %s", got.Name())
	}

	// Get non-existent provider
	_, err = r.GetSemantic("nonexistent")
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestRegistry_GetEmbeddings(t *testing.T) {
	r := NewRegistry()

	p := &mockEmbeddingsProvider{name: "test", available: true}
	_ = r.RegisterEmbeddings(p)

	// Get existing provider
	got, err := r.GetEmbeddings("test")
	if err != nil {
		t.Fatalf("GetEmbeddings failed: %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("expected name 'test', got %s", got.Name())
	}

	// Get non-existent provider
	_, err = r.GetEmbeddings("nonexistent")
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestRegistry_DefaultSemantic(t *testing.T) {
	r := NewRegistry()

	// No providers registered
	_, err := r.DefaultSemantic()
	if err != ErrNoAvailableProvider {
		t.Errorf("expected ErrNoAvailableProvider, got %v", err)
	}

	// Register available provider
	p := &mockSemanticProvider{name: "test", available: true}
	_ = r.RegisterSemantic(p)

	got, err := r.DefaultSemantic()
	if err != nil {
		t.Fatalf("DefaultSemantic failed: %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("expected name 'test', got %s", got.Name())
	}
}

func TestRegistry_DefaultEmbeddings(t *testing.T) {
	r := NewRegistry()

	// No providers registered
	_, err := r.DefaultEmbeddings()
	if err != ErrNoAvailableProvider {
		t.Errorf("expected ErrNoAvailableProvider, got %v", err)
	}

	// Register available provider
	p := &mockEmbeddingsProvider{name: "test", available: true}
	_ = r.RegisterEmbeddings(p)

	got, err := r.DefaultEmbeddings()
	if err != nil {
		t.Fatalf("DefaultEmbeddings failed: %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("expected name 'test', got %s", got.Name())
	}
}

func TestRegistry_SetDefaultSemantic(t *testing.T) {
	r := NewRegistry()

	p1 := &mockSemanticProvider{name: "provider1", available: true}
	p2 := &mockSemanticProvider{name: "provider2", available: true}
	_ = r.RegisterSemantic(p1)
	_ = r.RegisterSemantic(p2)

	// Set default to provider2
	err := r.SetDefaultSemantic("provider2")
	if err != nil {
		t.Fatalf("SetDefaultSemantic failed: %v", err)
	}

	got, _ := r.DefaultSemantic()
	if got.Name() != "provider2" {
		t.Errorf("expected default 'provider2', got %s", got.Name())
	}

	// Set default to non-existent provider
	err = r.SetDefaultSemantic("nonexistent")
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestRegistry_ListSemantic(t *testing.T) {
	r := NewRegistry()

	p1 := &mockSemanticProvider{name: "provider1", available: true}
	p2 := &mockSemanticProvider{name: "provider2", available: false}
	_ = r.RegisterSemantic(p1)
	_ = r.RegisterSemantic(p2)

	all := r.ListSemantic()
	if len(all) != 2 {
		t.Errorf("expected 2 providers, got %d", len(all))
	}

	available := r.AvailableSemantic()
	if len(available) != 1 {
		t.Errorf("expected 1 available provider, got %d", len(available))
	}
}

func TestRegistry_ListEmbeddings(t *testing.T) {
	r := NewRegistry()

	p1 := &mockEmbeddingsProvider{name: "provider1", available: true}
	p2 := &mockEmbeddingsProvider{name: "provider2", available: false}
	_ = r.RegisterEmbeddings(p1)
	_ = r.RegisterEmbeddings(p2)

	all := r.ListEmbeddings()
	if len(all) != 2 {
		t.Errorf("expected 2 providers, got %d", len(all))
	}

	available := r.AvailableEmbeddings()
	if len(available) != 1 {
		t.Errorf("expected 1 available provider, got %d", len(available))
	}
}
