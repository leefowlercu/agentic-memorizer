package integrations

import (
	"context"
	"testing"
)

// mockIntegration is a test implementation of Integration.
type mockIntegration struct {
	name        string
	harness     string
	intType     IntegrationType
	description string
	installed   bool
	setupCalled bool
	teardownCalled bool
}

func (m *mockIntegration) Name() string                              { return m.name }
func (m *mockIntegration) Harness() string                           { return m.harness }
func (m *mockIntegration) Type() IntegrationType                     { return m.intType }
func (m *mockIntegration) Description() string                       { return m.description }
func (m *mockIntegration) Setup(ctx context.Context) error           { m.setupCalled = true; return nil }
func (m *mockIntegration) Teardown(ctx context.Context) error        { m.teardownCalled = true; return nil }
func (m *mockIntegration) IsInstalled() (bool, error)                { return m.installed, nil }
func (m *mockIntegration) Validate() error                           { return nil }
func (m *mockIntegration) Status() (*StatusInfo, error) {
	status := StatusNotInstalled
	if m.installed {
		status = StatusInstalled
	}
	return &StatusInfo{Status: status, Message: "test"}, nil
}

func TestRegistry(t *testing.T) {
	t.Run("NewRegistry", func(t *testing.T) {
		reg := NewRegistry()
		if reg == nil {
			t.Fatal("NewRegistry returned nil")
		}
		if reg.Count() != 0 {
			t.Errorf("Count() = %d, want 0", reg.Count())
		}
	})

	t.Run("Register", func(t *testing.T) {
		reg := NewRegistry()
		mock := &mockIntegration{
			name:    "test-integration",
			harness: "test-harness",
			intType: IntegrationTypeHook,
		}

		err := reg.Register(mock)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		if reg.Count() != 1 {
			t.Errorf("Count() = %d, want 1", reg.Count())
		}
	})

	t.Run("RegisterDuplicate", func(t *testing.T) {
		reg := NewRegistry()
		mock := &mockIntegration{name: "test"}

		_ = reg.Register(mock)
		err := reg.Register(mock)

		if err == nil {
			t.Error("Expected error for duplicate registration")
		}
	})

	t.Run("Get", func(t *testing.T) {
		reg := NewRegistry()
		mock := &mockIntegration{name: "test-get"}

		_ = reg.Register(mock)

		retrieved, err := reg.Get("test-get")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if retrieved.Name() != "test-get" {
			t.Errorf("Name() = %q, want %q", retrieved.Name(), "test-get")
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		reg := NewRegistry()

		_, err := reg.Get("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent integration")
		}
	})

	t.Run("List", func(t *testing.T) {
		reg := NewRegistry()
		_ = reg.Register(&mockIntegration{name: "alpha"})
		_ = reg.Register(&mockIntegration{name: "beta"})
		_ = reg.Register(&mockIntegration{name: "gamma"})

		list := reg.List()
		if len(list) != 3 {
			t.Errorf("List length = %d, want 3", len(list))
		}

		// Should be sorted by name
		if list[0].Name() != "alpha" || list[1].Name() != "beta" || list[2].Name() != "gamma" {
			t.Error("List not sorted by name")
		}
	})

	t.Run("ListByHarness", func(t *testing.T) {
		reg := NewRegistry()
		_ = reg.Register(&mockIntegration{name: "a1", harness: "harness-a"})
		_ = reg.Register(&mockIntegration{name: "a2", harness: "harness-a"})
		_ = reg.Register(&mockIntegration{name: "b1", harness: "harness-b"})

		listA := reg.ListByHarness("harness-a")
		if len(listA) != 2 {
			t.Errorf("ListByHarness(harness-a) length = %d, want 2", len(listA))
		}

		listB := reg.ListByHarness("harness-b")
		if len(listB) != 1 {
			t.Errorf("ListByHarness(harness-b) length = %d, want 1", len(listB))
		}
	})

	t.Run("ListByType", func(t *testing.T) {
		reg := NewRegistry()
		_ = reg.Register(&mockIntegration{name: "hook1", intType: IntegrationTypeHook})
		_ = reg.Register(&mockIntegration{name: "hook2", intType: IntegrationTypeHook})
		_ = reg.Register(&mockIntegration{name: "mcp1", intType: IntegrationTypeMCP})

		hooks := reg.ListByType(IntegrationTypeHook)
		if len(hooks) != 2 {
			t.Errorf("ListByType(hook) length = %d, want 2", len(hooks))
		}

		mcps := reg.ListByType(IntegrationTypeMCP)
		if len(mcps) != 1 {
			t.Errorf("ListByType(mcp) length = %d, want 1", len(mcps))
		}
	})

	t.Run("Names", func(t *testing.T) {
		reg := NewRegistry()
		_ = reg.Register(&mockIntegration{name: "z-test"})
		_ = reg.Register(&mockIntegration{name: "a-test"})

		names := reg.Names()
		if len(names) != 2 {
			t.Errorf("Names length = %d, want 2", len(names))
		}

		// Should be sorted
		if names[0] != "a-test" || names[1] != "z-test" {
			t.Error("Names not sorted")
		}
	})

	t.Run("Harnesses", func(t *testing.T) {
		reg := NewRegistry()
		_ = reg.Register(&mockIntegration{name: "i1", harness: "harness-z"})
		_ = reg.Register(&mockIntegration{name: "i2", harness: "harness-a"})

		harnesses := reg.Harnesses()
		if len(harnesses) != 2 {
			t.Errorf("Harnesses length = %d, want 2", len(harnesses))
		}

		// Should be sorted
		if harnesses[0] != "harness-a" || harnesses[1] != "harness-z" {
			t.Error("Harnesses not sorted")
		}
	})
}

func TestIntegrationTypes(t *testing.T) {
	tests := []struct {
		intType IntegrationType
		want    string
	}{
		{IntegrationTypeHook, "hook"},
		{IntegrationTypeMCP, "mcp"},
		{IntegrationTypePlugin, "plugin"},
	}

	for _, tt := range tests {
		if string(tt.intType) != tt.want {
			t.Errorf("IntegrationType = %q, want %q", tt.intType, tt.want)
		}
	}
}

func TestIntegrationStatus(t *testing.T) {
	tests := []struct {
		status IntegrationStatus
		want   string
	}{
		{StatusNotInstalled, "not_installed"},
		{StatusInstalled, "installed"},
		{StatusError, "error"},
		{StatusMissingHarness, "missing_harness"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("IntegrationStatus = %q, want %q", tt.status, tt.want)
		}
	}
}
