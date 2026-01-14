package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// T004: Tests for DaemonState type and transitions

func TestDaemonState_String(t *testing.T) {
	tests := []struct {
		name  string
		state DaemonState
		want  string
	}{
		{"starting state", DaemonStateStarting, "starting"},
		{"running state", DaemonStateRunning, "running"},
		{"degraded state", DaemonStateDegraded, "degraded"},
		{"stopping state", DaemonStateStopping, "stopping"},
		{"stopped state", DaemonStateStopped, "stopped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.state); got != tt.want {
				t.Errorf("DaemonState = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDaemonState_IsTerminal(t *testing.T) {
	tests := []struct {
		name  string
		state DaemonState
		want  bool
	}{
		{"starting is not terminal", DaemonStateStarting, false},
		{"running is not terminal", DaemonStateRunning, false},
		{"degraded is not terminal", DaemonStateDegraded, false},
		{"stopping is not terminal", DaemonStateStopping, false},
		{"stopped is terminal", DaemonStateStopped, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.want {
				t.Errorf("DaemonState.IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDaemonState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name string
		from DaemonState
		to   DaemonState
		want bool
	}{
		// From Starting
		{"starting to running", DaemonStateStarting, DaemonStateRunning, true},
		{"starting to stopped", DaemonStateStarting, DaemonStateStopped, true},
		{"starting to degraded", DaemonStateStarting, DaemonStateDegraded, false},
		{"starting to stopping", DaemonStateStarting, DaemonStateStopping, false},

		// From Running
		{"running to degraded", DaemonStateRunning, DaemonStateDegraded, true},
		{"running to stopping", DaemonStateRunning, DaemonStateStopping, true},
		{"running to starting", DaemonStateRunning, DaemonStateStarting, false},
		{"running to stopped", DaemonStateRunning, DaemonStateStopped, false},

		// From Degraded
		{"degraded to running", DaemonStateDegraded, DaemonStateRunning, true},
		{"degraded to stopping", DaemonStateDegraded, DaemonStateStopping, true},
		{"degraded to starting", DaemonStateDegraded, DaemonStateStarting, false},

		// From Stopping
		{"stopping to stopped", DaemonStateStopping, DaemonStateStopped, true},
		{"stopping to running", DaemonStateStopping, DaemonStateRunning, false},

		// From Stopped
		{"stopped to any", DaemonStateStopped, DaemonStateStarting, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("DaemonState(%v).CanTransitionTo(%v) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// T022: Tests for Daemon Start lifecycle

func TestDaemon_NewDaemon(t *testing.T) {
	cfg := DaemonConfig{
		HTTPPort:        7600,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 30,
		PIDFile:         "/tmp/test-daemon.pid",
	}

	d := NewDaemon(cfg)

	if d == nil {
		t.Fatal("NewDaemon() returned nil")
	}

	if d.State() != DaemonStateStopped {
		t.Errorf("NewDaemon().State() = %v, want %v", d.State(), DaemonStateStopped)
	}
}

func TestDaemon_State(t *testing.T) {
	cfg := DaemonConfig{
		HTTPPort:        7600,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 30,
		PIDFile:         "/tmp/test-daemon.pid",
	}

	d := NewDaemon(cfg)

	// Initial state should be Stopped
	if d.State() != DaemonStateStopped {
		t.Errorf("Daemon.State() = %v, want %v", d.State(), DaemonStateStopped)
	}
}

func TestDaemon_Health(t *testing.T) {
	cfg := DaemonConfig{
		HTTPPort:        7600,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 30,
		PIDFile:         "/tmp/test-daemon.pid",
	}

	d := NewDaemon(cfg)

	health := d.Health()

	// Before start, should still report healthy (no components)
	if health.Status != "healthy" {
		t.Errorf("Daemon.Health().Status = %v, want %v", health.Status, "healthy")
	}
}

// T046: Tests for Daemon.Stop()

func TestDaemon_Stop(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0, // Use any available port
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Stop should transition state to stopped
	err := d.Stop()
	if err != nil {
		t.Fatalf("Daemon.Stop() error = %v", err)
	}

	if d.State() != DaemonStateStopped {
		t.Errorf("Daemon.State() after Stop() = %v, want %v", d.State(), DaemonStateStopped)
	}
}

// T047: Tests for signal handling (context cancellation)

func TestDaemon_Start_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0, // Use any available port
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start daemon in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Verify daemon is running
	if d.State() != DaemonStateRunning {
		t.Errorf("Daemon.State() during run = %v, want %v", d.State(), DaemonStateRunning)
	}

	// Cancel context (simulates SIGINT/SIGTERM)
	cancel()

	// Wait for shutdown
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Daemon.Start() returned error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Daemon.Start() did not return after context cancellation")
	}

	// Verify daemon stopped
	if d.State() != DaemonStateStopped {
		t.Errorf("Daemon.State() after cancel = %v, want %v", d.State(), DaemonStateStopped)
	}
}

// T048: Tests for component shutdown ordering

// mockComponent is a test component that tracks start/stop order
type mockComponent struct {
	name       string
	startOrder *[]string
	stopOrder  *[]string
	mu         *sync.Mutex
}

func (m *mockComponent) Name() string {
	return m.name
}

func (m *mockComponent) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m.startOrder = append(*m.startOrder, m.name)
	return nil
}

func (m *mockComponent) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m.stopOrder = append(*m.stopOrder, m.name)
	return nil
}

func (m *mockComponent) Health() ComponentHealth {
	return ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	}
}

func TestDaemon_ComponentShutdownOrder(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	var startOrder, stopOrder []string
	var mu sync.Mutex

	// Register components in order: A, B, C
	for _, name := range []string{"A", "B", "C"} {
		d.RegisterComponent(&mockComponent{
			name:       name,
			startOrder: &startOrder,
			stopOrder:  &stopOrder,
			mu:         &mu,
		})
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start daemon in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Verify start order is A, B, C
	mu.Lock()
	if len(startOrder) != 3 || startOrder[0] != "A" || startOrder[1] != "B" || startOrder[2] != "C" {
		t.Errorf("Component start order = %v, want [A B C]", startOrder)
	}
	mu.Unlock()

	// Trigger shutdown
	cancel()

	// Wait for shutdown
	select {
	case <-errChan:
	case <-time.After(10 * time.Second):
		t.Fatal("Daemon.Start() did not return after context cancellation")
	}

	// Verify stop order is C, B, A (reverse)
	mu.Lock()
	if len(stopOrder) != 3 || stopOrder[0] != "C" || stopOrder[1] != "B" || stopOrder[2] != "A" {
		t.Errorf("Component stop order = %v, want [C B A]", stopOrder)
	}
	mu.Unlock()
}

// T077: Tests for daemon config reload callback

func TestDaemon_OnConfigReload(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Track reload callback invocations
	var callCount int
	var mu sync.Mutex

	d.OnConfigReload(func() error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	})

	// Trigger reload manually
	d.TriggerConfigReload()

	mu.Lock()
	if callCount != 1 {
		t.Errorf("Config reload callback called %d times, want 1", callCount)
	}
	mu.Unlock()
}

// T078: Tests for invalid config rejection

func TestDaemon_OnConfigReload_Error(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Register a callback that returns an error
	expectedErr := errors.New("invalid config")
	d.OnConfigReload(func() error {
		return expectedErr
	})

	// Trigger reload - should not panic and error should be logged
	d.TriggerConfigReload()

	// The reload happened but returned an error - this is expected behavior
	// In production, errors are logged but the daemon continues running
}

// T083: Tests for daemon degraded state transition

// failingComponent is a test component that fails on start
type failingComponent struct {
	name string
}

func (f *failingComponent) Name() string {
	return f.name
}

func (f *failingComponent) Start(ctx context.Context) error {
	return errors.New("component failed to start")
}

func (f *failingComponent) Stop(ctx context.Context) error {
	return nil
}

func (f *failingComponent) Health() ComponentHealth {
	return ComponentHealth{
		Status:      ComponentStatusFailed,
		Error:       "component failed to start",
		LastChecked: time.Now(),
	}
}

func TestDaemon_DegradedState_ComponentFailure(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Register a failing component
	d.RegisterComponent(&failingComponent{name: "failing-component"})

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start daemon in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Daemon should still be running (not crashed)
	if d.State() != DaemonStateRunning {
		t.Errorf("Daemon.State() = %v, want %v (daemon should continue despite component failure)", d.State(), DaemonStateRunning)
	}

	// Health should show degraded status
	health := d.Health()
	if health.Status != "degraded" {
		t.Errorf("Daemon.Health().Status = %q, want %q", health.Status, "degraded")
	}

	// Cleanup
	cancel()
	<-errChan
}

// T084: Tests for HealthManager degraded status aggregation
// (Already covered in health_test.go TestHealthManager_Status_Degraded)

// T085: Tests for /readyz degraded response
// (Already covered in server_test.go TestServer_Readyz_Degraded)

// Edge case tests for Daemon

func TestDaemon_Stop_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Stop multiple times should not panic or error
	for i := 0; i < 3; i++ {
		err := d.Stop()
		if err != nil {
			t.Errorf("Daemon.Stop() call %d error = %v, want nil", i+1, err)
		}
	}

	if d.State() != DaemonStateStopped {
		t.Errorf("Daemon.State() = %v, want %v", d.State(), DaemonStateStopped)
	}
}

func TestDaemon_UpdateComponentHealth_EmptyMap(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Should not panic with empty map
	d.UpdateComponentHealth(map[string]ComponentHealth{})

	health := d.Health()
	if health.Status != "healthy" {
		t.Errorf("Daemon.Health().Status = %q, want %q", health.Status, "healthy")
	}
}

func TestDaemon_UpdateComponentHealth_MultipleComponents(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Update with multiple components at once
	statuses := map[string]ComponentHealth{
		"component-a": {Status: ComponentStatusRunning, LastChecked: time.Now()},
		"component-b": {Status: ComponentStatusRunning, LastChecked: time.Now()},
		"component-c": {Status: ComponentStatusFailed, Error: "test error", LastChecked: time.Now()},
	}
	d.UpdateComponentHealth(statuses)

	health := d.Health()
	if health.Status != "degraded" {
		t.Errorf("Daemon.Health().Status = %q, want %q", health.Status, "degraded")
	}

	if len(health.Components) != 3 {
		t.Errorf("Daemon.Health().Components has %d entries, want 3", len(health.Components))
	}
}

func TestDaemon_MultipleReloadCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Track callback invocations
	var callOrder []int
	var mu sync.Mutex

	// Register multiple callbacks
	for i := range 3 {
		idx := i + 1
		d.OnConfigReload(func() error {
			mu.Lock()
			callOrder = append(callOrder, idx)
			mu.Unlock()
			return nil
		})
	}

	// Trigger reload
	d.TriggerConfigReload()

	mu.Lock()
	defer mu.Unlock()

	if len(callOrder) != 3 {
		t.Errorf("Expected 3 callbacks, got %d", len(callOrder))
	}

	// Verify order
	for i, v := range callOrder {
		if v != i+1 {
			t.Errorf("Callback order[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestDaemon_ReloadCallback_ContinuesOnError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	var callCount int
	var mu sync.Mutex

	// First callback succeeds
	d.OnConfigReload(func() error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	})

	// Second callback fails
	d.OnConfigReload(func() error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return errors.New("callback error")
	})

	// Third callback should still be called
	d.OnConfigReload(func() error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	})

	d.TriggerConfigReload()

	mu.Lock()
	defer mu.Unlock()

	if callCount != 3 {
		t.Errorf("Expected all 3 callbacks to be called despite error, got %d", callCount)
	}
}

// slowComponent is a test component that takes time to stop
type slowComponent struct {
	name        string
	stopDelay   time.Duration
	stopStarted chan struct{}
}

func (s *slowComponent) Name() string {
	return s.name
}

func (s *slowComponent) Start(ctx context.Context) error {
	return nil
}

func (s *slowComponent) Stop(ctx context.Context) error {
	if s.stopStarted != nil {
		close(s.stopStarted)
	}
	select {
	case <-time.After(s.stopDelay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *slowComponent) Health() ComponentHealth {
	return ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	}
}

func TestDaemon_ShutdownTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 100 * time.Millisecond, // Very short timeout
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	stopStarted := make(chan struct{})
	d.RegisterComponent(&slowComponent{
		name:        "slow-component",
		stopDelay:   5 * time.Second, // Much longer than timeout
		stopStarted: stopStarted,
	})

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	cancel()

	// Wait for stop to start
	select {
	case <-stopStarted:
	case <-time.After(time.Second):
		t.Fatal("Component stop not started")
	}

	// Shutdown should complete despite slow component (timeout)
	select {
	case <-errChan:
		// Expected - shutdown completed
	case <-time.After(2 * time.Second):
		t.Error("Daemon did not shutdown within expected time (timeout should have triggered)")
	}
}

func TestDaemon_ComponentPartialFailure(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	var startOrder []string
	var mu sync.Mutex

	// First component succeeds
	d.RegisterComponent(&mockComponent{
		name:       "success-first",
		startOrder: &startOrder,
		stopOrder:  &[]string{},
		mu:         &mu,
	})

	// Second component fails
	d.RegisterComponent(&failingComponent{name: "failing-middle"})

	// Third component should still start
	d.RegisterComponent(&mockComponent{
		name:       "success-last",
		startOrder: &startOrder,
		stopOrder:  &[]string{},
		mu:         &mu,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// All components should have been attempted
	mu.Lock()
	// success-first and success-last should be in startOrder
	if len(startOrder) != 2 {
		t.Errorf("Expected 2 successful component starts, got %d", len(startOrder))
	}
	mu.Unlock()

	// Daemon should be in degraded state
	health := d.Health()
	if health.Status != "degraded" {
		t.Errorf("Daemon.Health().Status = %q, want %q", health.Status, "degraded")
	}

	cancel()
	<-errChan
}

// Additional edge case tests

func TestDefaultDaemonConfig(t *testing.T) {
	cfg := DefaultDaemonConfig()

	if cfg.HTTPPort != 7600 {
		t.Errorf("DefaultDaemonConfig().HTTPPort = %d, want 7600", cfg.HTTPPort)
	}

	if cfg.HTTPBind != "127.0.0.1" {
		t.Errorf("DefaultDaemonConfig().HTTPBind = %q, want %q", cfg.HTTPBind, "127.0.0.1")
	}

	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("DefaultDaemonConfig().ShutdownTimeout = %v, want 30s", cfg.ShutdownTimeout)
	}

	if cfg.PIDFile != "~/.config/memorizer/daemon.pid" {
		t.Errorf("DefaultDaemonConfig().PIDFile = %q, want %q", cfg.PIDFile, "~/.config/memorizer/daemon.pid")
	}
}

func TestDaemon_RegisterComponent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	var startOrder, stopOrder []string
	var mu sync.Mutex

	// Register multiple components
	d.RegisterComponent(&mockComponent{
		name:       "component-1",
		startOrder: &startOrder,
		stopOrder:  &stopOrder,
		mu:         &mu,
	})
	d.RegisterComponent(&mockComponent{
		name:       "component-2",
		startOrder: &startOrder,
		stopOrder:  &stopOrder,
		mu:         &mu,
	})

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Verify both components started
	mu.Lock()
	if len(startOrder) != 2 {
		t.Errorf("Expected 2 components started, got %d", len(startOrder))
	}
	mu.Unlock()

	cancel()
	<-errChan
}

func TestDaemon_NoComponents(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         filepath.Join(tmpDir, "test-daemon.pid"),
	}

	d := NewDaemon(cfg)

	// Don't register any components - daemon should still work

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Daemon should be running
	if d.State() != DaemonStateRunning {
		t.Errorf("Daemon.State() = %v, want %v", d.State(), DaemonStateRunning)
	}

	// Health should be healthy with no components
	health := d.Health()
	if health.Status != "healthy" {
		t.Errorf("Daemon.Health().Status = %q, want %q", health.Status, "healthy")
	}

	cancel()
	<-errChan
}

func TestDaemon_PIDFileClaimedByOther(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test-daemon.pid")

	// Write current process PID to simulate another daemon running
	pf := NewPIDFile(pidPath)
	if err := pf.Write(); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	cfg := DaemonConfig{
		HTTPPort:        0,
		HTTPBind:        "127.0.0.1",
		ShutdownTimeout: 5 * time.Second,
		PIDFile:         pidPath,
	}

	d := NewDaemon(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should fail because PID file is claimed
	err := d.Start(ctx)
	if err == nil {
		t.Error("Expected error when PID file is already claimed")
	}

	if d.State() != DaemonStateStopped {
		t.Errorf("Daemon.State() = %v, want %v after failed start", d.State(), DaemonStateStopped)
	}
}
