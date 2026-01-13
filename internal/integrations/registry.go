package integrations

import (
	"fmt"
	"sort"
	"sync"
)

// Registry manages all available integrations.
type Registry struct {
	mu           sync.RWMutex
	integrations map[string]Integration
	byHarness    map[string][]Integration
	byType       map[IntegrationType][]Integration
}

// NewRegistry creates a new integration registry.
func NewRegistry() *Registry {
	return &Registry{
		integrations: make(map[string]Integration),
		byHarness:    make(map[string][]Integration),
		byType:       make(map[IntegrationType][]Integration),
	}
}

// Register adds an integration to the registry.
func (r *Registry) Register(integration Integration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := integration.Name()
	if _, exists := r.integrations[name]; exists {
		return fmt.Errorf("integration %q already registered", name)
	}

	r.integrations[name] = integration
	r.byHarness[integration.Harness()] = append(r.byHarness[integration.Harness()], integration)
	r.byType[integration.Type()] = append(r.byType[integration.Type()], integration)

	return nil
}

// Get retrieves an integration by name.
func (r *Registry) Get(name string) (Integration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	integration, exists := r.integrations[name]
	if !exists {
		return nil, fmt.Errorf("integration %q not found", name)
	}

	return integration, nil
}

// List returns all registered integrations.
func (r *Registry) List() []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Integration, 0, len(r.integrations))
	for _, integration := range r.integrations {
		result = append(result, integration)
	}

	// Sort by name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})

	return result
}

// ListByHarness returns all integrations for a specific harness.
func (r *Registry) ListByHarness(harness string) []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Integration, len(r.byHarness[harness]))
	copy(result, r.byHarness[harness])

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})

	return result
}

// ListByType returns all integrations of a specific type.
func (r *Registry) ListByType(integrationType IntegrationType) []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Integration, len(r.byType[integrationType]))
	copy(result, r.byType[integrationType])

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})

	return result
}

// Names returns all integration names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.integrations))
	for name := range r.integrations {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// Harnesses returns all unique harness names.
func (r *Registry) Harnesses() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	harnesses := make([]string, 0, len(r.byHarness))
	for harness := range r.byHarness {
		harnesses = append(harnesses, harness)
	}

	sort.Strings(harnesses)
	return harnesses
}

// Count returns the total number of registered integrations.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.integrations)
}

// globalRegistry is the singleton registry instance.
var (
	globalRegistry     *Registry
	globalRegistryOnce sync.Once
)

// GlobalRegistry returns the global integration registry.
func GlobalRegistry() *Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// RegisterIntegration adds an integration to the global registry.
func RegisterIntegration(integration Integration) error {
	return GlobalRegistry().Register(integration)
}
