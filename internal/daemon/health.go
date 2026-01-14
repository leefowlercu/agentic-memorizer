package daemon

import (
	"sync"
	"time"
)

// ComponentHealth represents the health status of a single component.
type ComponentHealth struct {
	// Status is the current health state.
	Status ComponentStatus `json:"status"`

	// Error contains the error message if Status is "failed".
	// Empty string if component is healthy.
	Error string `json:"error,omitempty"`

	// LastChecked is when the health was last evaluated.
	LastChecked time.Time `json:"last_checked"`
}

// IsHealthy returns true if the component health indicates healthy operation.
func (h ComponentHealth) IsHealthy() bool {
	return h.Status.IsHealthy()
}

// HealthStatus represents the aggregate health of the daemon.
// This is the response format for /healthz and /readyz endpoints.
type HealthStatus struct {
	// Status is the overall daemon health: "healthy", "degraded", or "unhealthy".
	Status string `json:"status"`

	// Ready indicates whether the daemon can serve requests.
	// True for "healthy" and "degraded" states.
	Ready bool `json:"ready"`

	// Uptime is how long the daemon has been running.
	Uptime time.Duration `json:"uptime"`

	// Components contains per-component health status.
	// Omitted for /healthz, included for /readyz.
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

// HealthManager aggregates health status from multiple components.
// It is safe for concurrent use.
type HealthManager struct {
	mu         sync.RWMutex
	components map[string]ComponentHealth
	startTime  time.Time
}

// NewHealthManager creates a new HealthManager instance.
func NewHealthManager() *HealthManager {
	return &HealthManager{
		components: make(map[string]ComponentHealth),
		startTime:  time.Now(),
	}
}

// UpdateComponent updates the health status for a named component.
func (m *HealthManager) UpdateComponent(name string, health ComponentHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components[name] = health
}

// RemoveComponent removes a component from health tracking.
func (m *HealthManager) RemoveComponent(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.components, name)
}

// Status returns the aggregate health status of all components.
func (m *HealthManager) Status() HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := HealthStatus{
		Status:     "healthy",
		Ready:      true,
		Uptime:     time.Since(m.startTime),
		Components: make(map[string]ComponentHealth),
	}

	// Copy components
	for name, health := range m.components {
		status.Components[name] = health
	}

	// Determine aggregate status
	for _, health := range m.components {
		if !health.IsHealthy() {
			status.Status = "degraded"
			// Still ready in degraded state
			break
		}
	}

	return status
}
