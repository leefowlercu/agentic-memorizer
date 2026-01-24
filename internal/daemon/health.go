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

	// Since is when the component entered the current state.
	Since time.Time `json:"since,omitempty"`

	// LastSuccess is when the component last successfully reported healthy.
	LastSuccess time.Time `json:"last_success,omitempty"`

	// Details carries optional, non-sensitive diagnostic data.
	Details map[string]any `json:"details,omitempty"`
}

// IsHealthy returns true if the component health indicates healthy operation.
func (h ComponentHealth) IsHealthy() bool {
	return h.Status.IsHealthy()
}

// JobHealth represents the status of a job execution.
type JobHealth struct {
	// Status is the current job status.
	Status JobStatus `json:"status"`

	// Error contains the error message if the job failed.
	Error string `json:"error,omitempty"`

	// StartedAt is when the job last started.
	StartedAt time.Time `json:"started_at,omitempty"`

	// FinishedAt is when the job last completed.
	FinishedAt time.Time `json:"finished_at,omitempty"`

	// Counts contains job-specific counters (files processed, etc.).
	Counts map[string]int `json:"counts,omitempty"`

	// Details carries optional, non-sensitive job metadata.
	Details map[string]any `json:"details,omitempty"`
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

	// Jobs contains per-job status details.
	// Omitted for /healthz, included for /readyz.
	Jobs map[string]JobHealth `json:"jobs,omitempty"`
}

// HealthManager aggregates health status from multiple components.
// It is safe for concurrent use.
type HealthManager struct {
	mu         sync.RWMutex
	components map[string]ComponentHealth
	jobs       map[string]JobHealth
	startTime  time.Time
}

// NewHealthManager creates a new HealthManager instance.
func NewHealthManager() *HealthManager {
	return &HealthManager{
		components: make(map[string]ComponentHealth),
		jobs:       make(map[string]JobHealth),
		startTime:  time.Now(),
	}
}

// UpdateComponent updates the health status for a named component.
func (m *HealthManager) UpdateComponent(name string, health ComponentHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components[name] = health
}

// UpdateJob updates the status for a named job.
func (m *HealthManager) UpdateJob(name string, health JobHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[name] = health
}

// RemoveComponent removes a component from health tracking.
func (m *HealthManager) RemoveComponent(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.components, name)
}

// RemoveJob removes a job from health tracking.
func (m *HealthManager) RemoveJob(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, name)
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
		Jobs:       make(map[string]JobHealth),
	}

	// Copy components
	for name, health := range m.components {
		status.Components[name] = health
	}
	for name, health := range m.jobs {
		status.Jobs[name] = health
	}

	// Determine aggregate status
	for _, health := range m.components {
		if !health.IsHealthy() {
			status.Status = "degraded"
			// Still ready in degraded state
			break
		}
	}

	// Job failures also degrade overall health.
	if status.Status == "healthy" {
		for _, health := range m.jobs {
			if health.Status == JobStatusFailed || health.Status == JobStatusPartial {
				status.Status = "degraded"
				break
			}
		}
	}

	return status
}
