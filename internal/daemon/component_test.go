package daemon

import (
	"testing"
)

// T005: Tests for ComponentStatus type

func TestComponentStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status ComponentStatus
		want   string
	}{
		{"running status", ComponentStatusRunning, "running"},
		{"failed status", ComponentStatusFailed, "failed"},
		{"stopped status", ComponentStatusStopped, "stopped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.status); got != tt.want {
				t.Errorf("ComponentStatus = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponentStatus_IsHealthy(t *testing.T) {
	tests := []struct {
		name   string
		status ComponentStatus
		want   bool
	}{
		{"running is healthy", ComponentStatusRunning, true},
		{"failed is not healthy", ComponentStatusFailed, false},
		{"stopped is not healthy", ComponentStatusStopped, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsHealthy(); got != tt.want {
				t.Errorf("ComponentStatus.IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}
