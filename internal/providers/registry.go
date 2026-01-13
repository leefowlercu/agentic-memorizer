package providers

import (
	"errors"
	"sync"
)

var (
	// ErrProviderNotFound is returned when a provider is not registered.
	ErrProviderNotFound = errors.New("provider not found")

	// ErrProviderExists is returned when trying to register a duplicate provider.
	ErrProviderExists = errors.New("provider already exists")

	// ErrNoAvailableProvider is returned when no provider is available.
	ErrNoAvailableProvider = errors.New("no available provider")
)

// Registry manages provider registration and lookup.
type Registry struct {
	mu                 sync.RWMutex
	semanticProviders  map[string]SemanticProvider
	embeddingsProviders map[string]EmbeddingsProvider
	defaultSemantic    string
	defaultEmbeddings  string
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		semanticProviders:   make(map[string]SemanticProvider),
		embeddingsProviders: make(map[string]EmbeddingsProvider),
	}
}

// RegisterSemantic registers a semantic provider.
func (r *Registry) RegisterSemantic(p SemanticProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.semanticProviders[name]; exists {
		return ErrProviderExists
	}

	r.semanticProviders[name] = p

	// Set as default if first available provider
	if r.defaultSemantic == "" && p.Available() {
		r.defaultSemantic = name
	}

	return nil
}

// RegisterEmbeddings registers an embeddings provider.
func (r *Registry) RegisterEmbeddings(p EmbeddingsProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.embeddingsProviders[name]; exists {
		return ErrProviderExists
	}

	r.embeddingsProviders[name] = p

	// Set as default if first available provider
	if r.defaultEmbeddings == "" && p.Available() {
		r.defaultEmbeddings = name
	}

	return nil
}

// GetSemantic returns a semantic provider by name.
func (r *Registry) GetSemantic(name string) (SemanticProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.semanticProviders[name]
	if !exists {
		return nil, ErrProviderNotFound
	}

	return p, nil
}

// GetEmbeddings returns an embeddings provider by name.
func (r *Registry) GetEmbeddings(name string) (EmbeddingsProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.embeddingsProviders[name]
	if !exists {
		return nil, ErrProviderNotFound
	}

	return p, nil
}

// DefaultSemantic returns the default semantic provider.
func (r *Registry) DefaultSemantic() (SemanticProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultSemantic == "" {
		// Find first available
		for _, p := range r.semanticProviders {
			if p.Available() {
				return p, nil
			}
		}
		return nil, ErrNoAvailableProvider
	}

	return r.semanticProviders[r.defaultSemantic], nil
}

// DefaultEmbeddings returns the default embeddings provider.
func (r *Registry) DefaultEmbeddings() (EmbeddingsProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultEmbeddings == "" {
		// Find first available
		for _, p := range r.embeddingsProviders {
			if p.Available() {
				return p, nil
			}
		}
		return nil, ErrNoAvailableProvider
	}

	return r.embeddingsProviders[r.defaultEmbeddings], nil
}

// SetDefaultSemantic sets the default semantic provider by name.
func (r *Registry) SetDefaultSemantic(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.semanticProviders[name]; !exists {
		return ErrProviderNotFound
	}

	r.defaultSemantic = name
	return nil
}

// SetDefaultEmbeddings sets the default embeddings provider by name.
func (r *Registry) SetDefaultEmbeddings(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.embeddingsProviders[name]; !exists {
		return ErrProviderNotFound
	}

	r.defaultEmbeddings = name
	return nil
}

// ListSemantic returns all registered semantic providers.
func (r *Registry) ListSemantic() []SemanticProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]SemanticProvider, 0, len(r.semanticProviders))
	for _, p := range r.semanticProviders {
		providers = append(providers, p)
	}
	return providers
}

// ListEmbeddings returns all registered embeddings providers.
func (r *Registry) ListEmbeddings() []EmbeddingsProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]EmbeddingsProvider, 0, len(r.embeddingsProviders))
	for _, p := range r.embeddingsProviders {
		providers = append(providers, p)
	}
	return providers
}

// AvailableSemantic returns all available semantic providers.
func (r *Registry) AvailableSemantic() []SemanticProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var providers []SemanticProvider
	for _, p := range r.semanticProviders {
		if p.Available() {
			providers = append(providers, p)
		}
	}
	return providers
}

// AvailableEmbeddings returns all available embeddings providers.
func (r *Registry) AvailableEmbeddings() []EmbeddingsProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var providers []EmbeddingsProvider
	for _, p := range r.embeddingsProviders {
		if p.Available() {
			providers = append(providers, p)
		}
	}
	return providers
}
