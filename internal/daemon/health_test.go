package daemon

import (
	"sync"
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

// Edge case tests for HealthManager

func TestHealthManager_RemoveComponent(t *testing.T) {
	hm := NewHealthManager()

	// Add a component
	hm.UpdateComponent("test-component", ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	})

	// Verify it exists
	status := hm.Status()
	if len(status.Components) != 1 {
		t.Fatalf("Expected 1 component, got %d", len(status.Components))
	}

	// Remove it
	hm.RemoveComponent("test-component")

	// Verify it's gone
	status = hm.Status()
	if len(status.Components) != 0 {
		t.Errorf("Expected 0 components after removal, got %d", len(status.Components))
	}
}

func TestHealthManager_RemoveComponent_NonExistent(t *testing.T) {
	hm := NewHealthManager()

	// Remove non-existent component should not panic
	hm.RemoveComponent("non-existent")

	status := hm.Status()
	if len(status.Components) != 0 {
		t.Errorf("Expected 0 components, got %d", len(status.Components))
	}
}

func TestHealthManager_UpdateComponent_Overwrite(t *testing.T) {
	hm := NewHealthManager()

	// Add a component as running
	hm.UpdateComponent("test-component", ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	})

	status := hm.Status()
	if status.Status != "healthy" {
		t.Errorf("Initial status = %q, want %q", status.Status, "healthy")
	}

	// Update same component to failed
	hm.UpdateComponent("test-component", ComponentHealth{
		Status:      ComponentStatusFailed,
		Error:       "updated error",
		LastChecked: time.Now(),
	})

	status = hm.Status()
	if status.Status != "degraded" {
		t.Errorf("Updated status = %q, want %q", status.Status, "degraded")
	}

	// Should still be just 1 component
	if len(status.Components) != 1 {
		t.Errorf("Expected 1 component after overwrite, got %d", len(status.Components))
	}

	comp := status.Components["test-component"]
	if comp.Error != "updated error" {
		t.Errorf("Component error = %q, want %q", comp.Error, "updated error")
	}
}

func TestHealthManager_AllComponentsFailed(t *testing.T) {
	hm := NewHealthManager()

	// Add multiple failed components
	hm.UpdateComponent("failed-1", ComponentHealth{
		Status:      ComponentStatusFailed,
		Error:       "error 1",
		LastChecked: time.Now(),
	})
	hm.UpdateComponent("failed-2", ComponentHealth{
		Status:      ComponentStatusFailed,
		Error:       "error 2",
		LastChecked: time.Now(),
	})

	status := hm.Status()

	// Should be degraded, not unhealthy (we don't have unhealthy state)
	if status.Status != "degraded" {
		t.Errorf("Status = %q, want %q", status.Status, "degraded")
	}

	// Should still be ready in degraded state
	if !status.Ready {
		t.Error("Ready = false, want true even when all components failed")
	}
}

func TestHealthManager_Uptime(t *testing.T) {
	hm := NewHealthManager()

	// Get initial status
	status1 := hm.Status()

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Get status again
	status2 := hm.Status()

	// Uptime should have increased
	if status2.Uptime <= status1.Uptime {
		t.Errorf("Uptime did not increase: first=%v, second=%v", status1.Uptime, status2.Uptime)
	}

	// Uptime should be at least 50ms
	if status2.Uptime < 50*time.Millisecond {
		t.Errorf("Uptime = %v, expected at least 50ms", status2.Uptime)
	}
}

func TestHealthManager_ComponentsAreCopied(t *testing.T) {
	hm := NewHealthManager()

	hm.UpdateComponent("test", ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	})

	// Get status
	status := hm.Status()

	// Modify the returned map
	status.Components["test"] = ComponentHealth{
		Status:      ComponentStatusFailed,
		Error:       "modified",
		LastChecked: time.Now(),
	}

	// Original should be unchanged
	status2 := hm.Status()
	if status2.Components["test"].Status != ComponentStatusRunning {
		t.Error("Modifying returned components map affected internal state")
	}
}

// Concurrency tests for HealthManager

func TestHealthManager_ConcurrentUpdates(t *testing.T) {
	hm := NewHealthManager()

	var wg sync.WaitGroup
	numGoroutines := 100
	numUpdates := 100

	// Spawn goroutines that all update components concurrently
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numUpdates {
				hm.UpdateComponent("component", ComponentHealth{
					Status:      ComponentStatusRunning,
					Error:       "",
					LastChecked: time.Now(),
				})
				// Also read status to create read/write contention
				if j%10 == 0 {
					_ = hm.Status()
				}
			}
		}(i)
	}

	wg.Wait()

	// Should complete without race condition panic
	status := hm.Status()
	if status.Status != "healthy" {
		t.Errorf("Expected healthy status, got %s", status.Status)
	}
}

func TestHealthManager_ConcurrentReadsAndWrites(t *testing.T) {
	hm := NewHealthManager()

	// Pre-populate with some components
	for i := range 10 {
		name := "component-" + string(rune('a'+i))
		hm.UpdateComponent(name, ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
		})
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writers
	for i := range 5 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := "writer-" + string(rune('0'+id))
			for {
				select {
				case <-stop:
					return
				default:
					hm.UpdateComponent(name, ComponentHealth{
						Status:      ComponentStatusRunning,
						LastChecked: time.Now(),
					})
				}
			}
		}(i)
	}

	// Readers
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					status := hm.Status()
					// Access the components to ensure we're actually reading
					_ = len(status.Components)
				}
			}
		}()
	}

	// Removers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				hm.RemoveComponent("nonexistent")
			}
		}
	}()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Should complete without race condition
}

func TestHealthManager_EmptyComponentName(t *testing.T) {
	hm := NewHealthManager()

	// Empty string as component name should work (though not recommended)
	hm.UpdateComponent("", ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	})

	status := hm.Status()
	if len(status.Components) != 1 {
		t.Errorf("Expected 1 component, got %d", len(status.Components))
	}

	if _, exists := status.Components[""]; !exists {
		t.Error("Empty string component not found")
	}
}
