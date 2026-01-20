package daemon

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// mockHealthUpdater implements HealthUpdater for testing.
type mockHealthUpdater struct {
	mu       sync.Mutex
	updates  []map[string]ComponentHealth
	statuses map[string]ComponentHealth
}

func newMockHealthUpdater() *mockHealthUpdater {
	return &mockHealthUpdater{
		statuses: make(map[string]ComponentHealth),
	}
}

func (m *mockHealthUpdater) UpdateComponentHealth(statuses map[string]ComponentHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, statuses)
	for name, health := range statuses {
		m.statuses[name] = health
	}
}

func (m *mockHealthUpdater) getStatus(name string) (ComponentHealth, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.statuses[name]
	return h, ok
}

func (m *mockHealthUpdater) updateCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.updates)
}

func TestNewComponentSupervisor(t *testing.T) {
	health := newMockHealthUpdater()
	supervisor := NewComponentSupervisor(health)

	if supervisor == nil {
		t.Fatal("expected non-nil supervisor")
	}
	if supervisor.healthUpdater == nil {
		t.Error("expected healthUpdater to be set")
	}
	if supervisor.componentCancels == nil {
		t.Error("expected componentCancels to be initialized")
	}
}

func TestComponentSupervisor_Supervise_Success(t *testing.T) {
	health := newMockHealthUpdater()
	supervisor := NewComponentSupervisor(health, WithSupervisorLogger(slog.Default()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	completed := make(chan struct{})
	def := ComponentDefinition{
		Name:          "test_component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
	}

	// startFn returns nil (success) immediately, then supervisor updates health
	supervisor.Supervise(ctx, "test_component", def, func(ctx context.Context) error {
		close(completed)
		return nil // Success - this triggers health update to running
	}, nil)

	// Wait for component start function to complete
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("component start function did not complete")
	}

	// Give time for health update to propagate
	time.Sleep(20 * time.Millisecond)

	// Verify health was updated to running
	status, ok := health.getStatus("test_component")
	if !ok {
		t.Fatal("expected health status to be set")
	}
	if status.Status != ComponentStatusRunning {
		t.Errorf("expected status running, got %s", status.Status)
	}

	// Verify component is tracked
	if supervisor.SupervisedCount() != 1 {
		t.Errorf("expected 1 supervised component, got %d", supervisor.SupervisedCount())
	}
}

func TestComponentSupervisor_Supervise_StartFailure_RestartNever(t *testing.T) {
	health := newMockHealthUpdater()
	supervisor := NewComponentSupervisor(health, WithSupervisorLogger(slog.Default()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	def := ComponentDefinition{
		Name:          "failing_component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
	}

	callCount := 0
	supervisor.Supervise(ctx, "failing_component", def, func(ctx context.Context) error {
		callCount++
		return errors.New("simulated failure")
	}, nil)

	// Wait for component to fail
	time.Sleep(50 * time.Millisecond)

	// Should only be called once with RestartNever
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}

	// Verify health was updated to failed
	status, ok := health.getStatus("failing_component")
	if !ok {
		t.Fatal("expected health status to be set")
	}
	if status.Status != ComponentStatusFailed {
		t.Errorf("expected status failed, got %s", status.Status)
	}
}

func TestComponentSupervisor_Supervise_StartFailure_RestartOnFailure(t *testing.T) {
	health := newMockHealthUpdater()
	supervisor := NewComponentSupervisor(health,
		WithSupervisorLogger(slog.Default()),
		WithBackoff(10*time.Millisecond, 50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())

	def := ComponentDefinition{
		Name:          "restartable_component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
	}

	callCount := 0
	mu := sync.Mutex{}
	supervisor.Supervise(ctx, "restartable_component", def, func(ctx context.Context) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return errors.New("simulated failure")
	}, nil)

	// Wait for a few restart attempts
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for goroutine to finish
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	calls := callCount
	mu.Unlock()

	// Should have been called multiple times
	if calls < 2 {
		t.Errorf("expected multiple calls with RestartOnFailure, got %d", calls)
	}
}

func TestComponentSupervisor_Supervise_RuntimeFatal(t *testing.T) {
	health := newMockHealthUpdater()
	supervisor := NewComponentSupervisor(health,
		WithSupervisorLogger(slog.Default()),
		WithBackoff(10*time.Millisecond, 50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	def := ComponentDefinition{
		Name:          "component_with_fatal",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartOnFailure,
	}

	fatalCh := make(chan error, 1)
	callCount := 0
	mu := sync.Mutex{}

	supervisor.Supervise(ctx, "component_with_fatal", def, func(ctx context.Context) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil // Start succeeds
	}, fatalCh)

	// Wait for component to start
	time.Sleep(50 * time.Millisecond)

	// Send a fatal error
	fatalCh <- errors.New("runtime fatal error")

	// Wait for restart
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	calls := callCount
	mu.Unlock()

	// Should have been called at least twice (initial + restart)
	if calls < 2 {
		t.Errorf("expected at least 2 calls after fatal error, got %d", calls)
	}
}

func TestComponentSupervisor_Cancel(t *testing.T) {
	health := newMockHealthUpdater()
	supervisor := NewComponentSupervisor(health, WithSupervisorLogger(slog.Default()))

	ctx := context.Background()

	stopped := make(chan struct{})
	def := ComponentDefinition{
		Name:          "cancelable_component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
	}

	supervisor.Supervise(ctx, "cancelable_component", def, func(ctx context.Context) error {
		<-ctx.Done()
		close(stopped)
		return nil
	}, nil)

	// Wait for component to start
	time.Sleep(20 * time.Millisecond)

	if supervisor.SupervisedCount() != 1 {
		t.Errorf("expected 1 supervised component, got %d", supervisor.SupervisedCount())
	}

	// Cancel the component
	supervisor.Cancel("cancelable_component")

	// Wait for component to stop
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("component did not stop after cancel")
	}

	if supervisor.SupervisedCount() != 0 {
		t.Errorf("expected 0 supervised components after cancel, got %d", supervisor.SupervisedCount())
	}
}

func TestComponentSupervisor_CancelAll(t *testing.T) {
	health := newMockHealthUpdater()
	supervisor := NewComponentSupervisor(health, WithSupervisorLogger(slog.Default()))

	ctx := context.Background()

	stopped1 := make(chan struct{})
	stopped2 := make(chan struct{})

	def := ComponentDefinition{
		Name:          "component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
	}

	supervisor.Supervise(ctx, "component1", def, func(ctx context.Context) error {
		<-ctx.Done()
		close(stopped1)
		return nil
	}, nil)

	supervisor.Supervise(ctx, "component2", def, func(ctx context.Context) error {
		<-ctx.Done()
		close(stopped2)
		return nil
	}, nil)

	// Wait for components to start
	time.Sleep(20 * time.Millisecond)

	if supervisor.SupervisedCount() != 2 {
		t.Errorf("expected 2 supervised components, got %d", supervisor.SupervisedCount())
	}

	// Cancel all
	supervisor.CancelAll()

	// Wait for components to stop
	select {
	case <-stopped1:
	case <-time.After(time.Second):
		t.Fatal("component1 did not stop after CancelAll")
	}

	select {
	case <-stopped2:
	case <-time.After(time.Second):
		t.Fatal("component2 did not stop after CancelAll")
	}

	if supervisor.SupervisedCount() != 0 {
		t.Errorf("expected 0 supervised components after CancelAll, got %d", supervisor.SupervisedCount())
	}
}

func TestComponentSupervisor_NilHealthUpdater(t *testing.T) {
	// Should not panic with nil healthUpdater
	supervisor := NewComponentSupervisor(nil, WithSupervisorLogger(slog.Default()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	def := ComponentDefinition{
		Name:          "no_health_component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
	}

	started := make(chan struct{})
	supervisor.Supervise(ctx, "no_health_component", def, func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return nil
	}, nil)

	// Should not panic
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("component did not start with nil healthUpdater")
	}
}
