package daemon

import (
	"testing"
	"time"
)

// T006: Tests for ComponentHealth struct

func TestComponentHealth_NewComponentHealth(t *testing.T) {
	health := ComponentHealth{
		Status:      ComponentStatusRunning,
		Error:       "",
		LastChecked: time.Now(),
	}

	if health.Status != ComponentStatusRunning {
		t.Errorf("ComponentHealth.Status = %v, want %v", health.Status, ComponentStatusRunning)
	}

	if health.Error != "" {
		t.Errorf("ComponentHealth.Error = %v, want empty string", health.Error)
	}
}

func TestComponentHealth_WithError(t *testing.T) {
	errMsg := "component failed to start"
	health := ComponentHealth{
		Status:      ComponentStatusFailed,
		Error:       errMsg,
		LastChecked: time.Now(),
	}

	if health.Status != ComponentStatusFailed {
		t.Errorf("ComponentHealth.Status = %v, want %v", health.Status, ComponentStatusFailed)
	}

	if health.Error != errMsg {
		t.Errorf("ComponentHealth.Error = %v, want %v", health.Error, errMsg)
	}
}

func TestComponentHealth_IsHealthy(t *testing.T) {
	tests := []struct {
		name   string
		health ComponentHealth
		want   bool
	}{
		{
			name: "running component is healthy",
			health: ComponentHealth{
				Status:      ComponentStatusRunning,
				LastChecked: time.Now(),
			},
			want: true,
		},
		{
			name: "failed component is not healthy",
			health: ComponentHealth{
				Status:      ComponentStatusFailed,
				Error:       "some error",
				LastChecked: time.Now(),
			},
			want: false,
		},
		{
			name: "stopped component is not healthy",
			health: ComponentHealth{
				Status:      ComponentStatusStopped,
				LastChecked: time.Now(),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.health.IsHealthy(); got != tt.want {
				t.Errorf("ComponentHealth.IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

// T019: Tests for HealthManager aggregate status

func TestHealthManager_NewHealthManager(t *testing.T) {
	hm := NewHealthManager()

	if hm == nil {
		t.Fatal("NewHealthManager() returned nil")
	}
}

func TestHealthManager_Status_NoComponents(t *testing.T) {
	hm := NewHealthManager()

	status := hm.Status()

	if status.Status != "healthy" {
		t.Errorf("HealthManager.Status().Status = %q, want %q", status.Status, "healthy")
	}

	if !status.Ready {
		t.Error("HealthManager.Status().Ready = false, want true")
	}

	if len(status.Components) != 0 {
		t.Errorf("HealthManager.Status().Components has %d entries, want 0", len(status.Components))
	}
}

func TestHealthManager_Status_AllHealthy(t *testing.T) {
	hm := NewHealthManager()

	// Register a mock healthy component
	hm.UpdateComponent("test-component", ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	})

	status := hm.Status()

	if status.Status != "healthy" {
		t.Errorf("HealthManager.Status().Status = %q, want %q", status.Status, "healthy")
	}

	if !status.Ready {
		t.Error("HealthManager.Status().Ready = false, want true")
	}

	if len(status.Components) != 1 {
		t.Errorf("HealthManager.Status().Components has %d entries, want 1", len(status.Components))
	}
}

func TestHealthManager_Status_Degraded(t *testing.T) {
	hm := NewHealthManager()

	// Register a healthy and a failed component
	hm.UpdateComponent("healthy-component", ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	})
	hm.UpdateComponent("failed-component", ComponentHealth{
		Status:      ComponentStatusFailed,
		Error:       "component failed",
		LastChecked: time.Now(),
	})

	status := hm.Status()

	if status.Status != "degraded" {
		t.Errorf("HealthManager.Status().Status = %q, want %q", status.Status, "degraded")
	}

	// Should still be ready in degraded state
	if !status.Ready {
		t.Error("HealthManager.Status().Ready = false, want true (degraded but ready)")
	}

	if len(status.Components) != 2 {
		t.Errorf("HealthManager.Status().Components has %d entries, want 2", len(status.Components))
	}
}
