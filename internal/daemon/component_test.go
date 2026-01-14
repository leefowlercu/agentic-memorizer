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

// Edge case tests for ComponentStatus

func TestComponentStatus_ZeroValue(t *testing.T) {
	var status ComponentStatus

	// Zero value should be empty string
	if status != "" {
		t.Errorf("Zero value ComponentStatus = %q, want empty string", status)
	}

	// Zero value should NOT be healthy
	if status.IsHealthy() {
		t.Error("Zero value ComponentStatus.IsHealthy() = true, want false")
	}
}

func TestComponentStatus_UnknownValue(t *testing.T) {
	// Custom/unknown status value
	status := ComponentStatus("unknown")

	// Unknown status should NOT be healthy (only "running" is healthy)
	if status.IsHealthy() {
		t.Error("Unknown ComponentStatus.IsHealthy() = true, want false")
	}

	// String representation should work
	if string(status) != "unknown" {
		t.Errorf("ComponentStatus string = %q, want %q", string(status), "unknown")
	}
}

func TestComponentStatus_CaseSensitive(t *testing.T) {
	// Status values are case-sensitive
	tests := []struct {
		status  ComponentStatus
		healthy bool
	}{
		{ComponentStatus("Running"), false}, // Wrong case
		{ComponentStatus("RUNNING"), false}, // Wrong case
		{ComponentStatus("running"), true},  // Correct
	}

	for _, tt := range tests {
		if got := tt.status.IsHealthy(); got != tt.healthy {
			t.Errorf("ComponentStatus(%q).IsHealthy() = %v, want %v", tt.status, got, tt.healthy)
		}
	}
}
