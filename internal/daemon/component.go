package daemon

// ComponentStatus represents the health state of a component.
type ComponentStatus string

const (
	// ComponentStatusRunning indicates the component is operating normally.
	ComponentStatusRunning ComponentStatus = "running"

	// ComponentStatusFailed indicates the component has encountered an error.
	ComponentStatusFailed ComponentStatus = "failed"

	// ComponentStatusDegraded indicates the component is running with reduced capabilities.
	ComponentStatusDegraded ComponentStatus = "degraded"

	// ComponentStatusStopped indicates the component has been intentionally stopped.
	ComponentStatusStopped ComponentStatus = "stopped"
)

// IsHealthy returns true if the component status indicates healthy operation.
func (s ComponentStatus) IsHealthy() bool {
	return s == ComponentStatusRunning
}
