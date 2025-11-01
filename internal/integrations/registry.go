package integrations

import (
	"fmt"
	"sync"
)

// Registry manages all available integrations.
// It provides thread-safe registration, lookup, and discovery operations.
type Registry struct {
	integrations map[string]Integration
	mu           sync.RWMutex
}

var (
	// globalRegistry is the singleton registry instance
	globalRegistry *Registry
	once           sync.Once
)

// NewRegistry creates a new integration registry
func NewRegistry() *Registry {
	return &Registry{
		integrations: make(map[string]Integration),
	}
}

// GlobalRegistry returns the singleton registry instance
func GlobalRegistry() *Registry {
	once.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// Register adds an integration to the registry.
// If an integration with the same name already exists, it will be replaced.
func (r *Registry) Register(integration Integration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := integration.GetName()
	r.integrations[name] = integration
}

// Get retrieves an integration by name.
// Returns error if the integration is not found.
func (r *Registry) Get(name string) (Integration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	integration, ok := r.integrations[name]
	if !ok {
		return nil, fmt.Errorf("integration %q not found in registry", name)
	}

	return integration, nil
}

// List returns all registered integrations.
// The returned slice is a copy and safe for concurrent use.
func (r *Registry) List() []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	integrations := make([]Integration, 0, len(r.integrations))
	for _, integration := range r.integrations {
		integrations = append(integrations, integration)
	}

	return integrations
}

// DetectAvailable returns all integrations that are detected on the system.
// This scans for frameworks by checking if their configuration directories/files exist.
func (r *Registry) DetectAvailable() []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	available := []Integration{}
	for _, integration := range r.integrations {
		detected, err := integration.Detect()
		if err == nil && detected {
			available = append(available, integration)
		}
	}

	return available
}

// DetectEnabled returns all integrations that are currently enabled.
// This checks if the integration has been configured (hooks installed, etc).
func (r *Registry) DetectEnabled() []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	enabled := []Integration{}
	for _, integration := range r.integrations {
		isEnabled, err := integration.IsEnabled()
		if err == nil && isEnabled {
			enabled = append(enabled, integration)
		}
	}

	return enabled
}

// Exists checks if an integration with the given name is registered
func (r *Registry) Exists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.integrations[name]
	return ok
}

// Count returns the number of registered integrations
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.integrations)
}

// Names returns all registered integration names
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.integrations))
	for name := range r.integrations {
		names = append(names, name)
	}

	return names
}
