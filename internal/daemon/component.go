package daemon

import (
	"context"
)

// ComponentStatus represents the health state of a component.
type ComponentStatus string

const (
	// ComponentStatusRunning indicates the component is operating normally.
	ComponentStatusRunning ComponentStatus = "running"

	// ComponentStatusFailed indicates the component has encountered an error.
	ComponentStatusFailed ComponentStatus = "failed"

	// ComponentStatusStopped indicates the component has been intentionally stopped.
	ComponentStatusStopped ComponentStatus = "stopped"
)

// IsHealthy returns true if the component status indicates healthy operation.
func (s ComponentStatus) IsHealthy() bool {
	return s == ComponentStatusRunning
}

// Component defines the interface for managed daemon subsystems.
// All components must implement this interface to be managed by the daemon.
type Component interface {
	// Name returns a unique identifier for this component.
	Name() string

	// Start initializes and starts the component.
	// The context may be cancelled to abort startup.
	// Returns error if startup fails.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the component.
	// The context includes a deadline for shutdown.
	// Returns error if shutdown fails or times out.
	Stop(ctx context.Context) error

	// Health returns the current health status of the component.
	Health() ComponentHealth
}
