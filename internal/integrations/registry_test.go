package integrations

import (
	"fmt"
	"sync"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// mockIntegration is a simple mock implementation for testing
type mockIntegration struct {
	name     string
	detected bool
	enabled  bool
}

func newMockIntegration(name string) *mockIntegration {
	return &mockIntegration{
		name:     name,
		detected: true,
		enabled:  false,
	}
}

func (m *mockIntegration) withDetected(detected bool) *mockIntegration {
	m.detected = detected
	return m
}

func (m *mockIntegration) withEnabled(enabled bool) *mockIntegration {
	m.enabled = enabled
	return m
}

func (m *mockIntegration) GetName() string                                               { return m.name }
func (m *mockIntegration) GetDescription() string                                        { return "mock" }
func (m *mockIntegration) GetVersion() string                                            { return "1.0.0" }
func (m *mockIntegration) Detect() (bool, error)                                         { return m.detected, nil }
func (m *mockIntegration) IsEnabled() (bool, error)                                      { return m.enabled, nil }
func (m *mockIntegration) Setup(binaryPath string) error                                 { m.enabled = true; return nil }
func (m *mockIntegration) Update(binaryPath string) error                                { return nil }
func (m *mockIntegration) Remove() error                                                 { m.enabled = false; return nil }
func (m *mockIntegration) GetCommand(binaryPath string, format OutputFormat) string      { return "" }
func (m *mockIntegration) FormatOutput(*types.Index, OutputFormat) (string, error)       { return "", nil }
func (m *mockIntegration) Validate() error                                               { return nil }
func (m *mockIntegration) Reload(newConfig IntegrationConfig) error                      { return nil }

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if registry.integrations == nil {
		t.Fatal("Registry integrations map is nil")
	}

	if registry.Count() != 0 {
		t.Errorf("Expected empty registry, got %d integrations", registry.Count())
	}
}

func TestRegister(t *testing.T) {
	registry := NewRegistry()
	mockInt := newMockIntegration("test-integration")

	registry.Register(mockInt)

	if registry.Count() != 1 {
		t.Errorf("Expected 1 integration, got %d", registry.Count())
	}

	// Test retrieving the registered integration
	integration, err := registry.Get("test-integration")
	if err != nil {
		t.Fatalf("Failed to get integration: %v", err)
	}

	if integration.GetName() != "test-integration" {
		t.Errorf("Expected name 'test-integration', got '%s'", integration.GetName())
	}
}

func TestRegisterReplaces(t *testing.T) {
	registry := NewRegistry()

	// Register first integration
	mockInt1 := newMockIntegration("test-integration")
	registry.Register(mockInt1)

	// Register another integration with same name
	mockInt2 := newMockIntegration("test-integration").withDetected(false)
	registry.Register(mockInt2)

	if registry.Count() != 1 {
		t.Errorf("Expected 1 integration after replacement, got %d", registry.Count())
	}

	// Verify it's the second one
	integration, _ := registry.Get("test-integration")
	detected, _ := integration.Detect()
	if detected {
		t.Error("Expected replaced integration to have detected=false")
	}
}

func TestGet(t *testing.T) {
	registry := NewRegistry()
	mockInt := newMockIntegration("test-integration")
	registry.Register(mockInt)

	// Test successful retrieval
	integration, err := registry.Get("test-integration")
	if err != nil {
		t.Fatalf("Failed to get integration: %v", err)
	}

	if integration.GetName() != "test-integration" {
		t.Errorf("Expected name 'test-integration', got '%s'", integration.GetName())
	}

	// Test non-existent integration
	_, err = registry.Get("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent integration")
	}
}

func TestList(t *testing.T) {
	registry := NewRegistry()

	// Test empty list
	list := registry.List()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d integrations", len(list))
	}

	// Add integrations
	registry.Register(newMockIntegration("integration-1"))
	registry.Register(newMockIntegration("integration-2"))
	registry.Register(newMockIntegration("integration-3"))

	list = registry.List()
	if len(list) != 3 {
		t.Errorf("Expected 3 integrations, got %d", len(list))
	}
}

func TestDetectAvailable(t *testing.T) {
	registry := NewRegistry()

	// Add mix of detected and not detected integrations
	registry.Register(newMockIntegration("detected-1").withDetected(true))
	registry.Register(newMockIntegration("detected-2").withDetected(true))
	registry.Register(newMockIntegration("not-detected-1").withDetected(false))
	registry.Register(newMockIntegration("not-detected-2").withDetected(false))

	available := registry.DetectAvailable()
	if len(available) != 2 {
		t.Errorf("Expected 2 available integrations, got %d", len(available))
	}

	// Verify they're the right ones
	names := make(map[string]bool)
	for _, integration := range available {
		names[integration.GetName()] = true
	}

	if !names["detected-1"] || !names["detected-2"] {
		t.Error("Expected detected-1 and detected-2 in available list")
	}
}

func TestDetectEnabled(t *testing.T) {
	registry := NewRegistry()

	// Add mix of enabled and disabled integrations
	registry.Register(newMockIntegration("enabled-1").withEnabled(true))
	registry.Register(newMockIntegration("enabled-2").withEnabled(true))
	registry.Register(newMockIntegration("disabled-1").withEnabled(false))
	registry.Register(newMockIntegration("disabled-2").withEnabled(false))

	enabled := registry.DetectEnabled()
	if len(enabled) != 2 {
		t.Errorf("Expected 2 enabled integrations, got %d", len(enabled))
	}

	// Verify they're the right ones
	names := make(map[string]bool)
	for _, integration := range enabled {
		names[integration.GetName()] = true
	}

	if !names["enabled-1"] || !names["enabled-2"] {
		t.Error("Expected enabled-1 and enabled-2 in enabled list")
	}
}

func TestExists(t *testing.T) {
	registry := NewRegistry()
	registry.Register(newMockIntegration("test-integration"))

	if !registry.Exists("test-integration") {
		t.Error("Expected integration to exist")
	}

	if registry.Exists("non-existent") {
		t.Error("Expected non-existent integration to not exist")
	}
}

func TestCount(t *testing.T) {
	registry := NewRegistry()

	if registry.Count() != 0 {
		t.Errorf("Expected count 0, got %d", registry.Count())
	}

	registry.Register(newMockIntegration("integration-1"))
	if registry.Count() != 1 {
		t.Errorf("Expected count 1, got %d", registry.Count())
	}

	registry.Register(newMockIntegration("integration-2"))
	if registry.Count() != 2 {
		t.Errorf("Expected count 2, got %d", registry.Count())
	}
}

func TestNames(t *testing.T) {
	registry := NewRegistry()

	// Test empty names
	names := registry.Names()
	if len(names) != 0 {
		t.Errorf("Expected empty names list, got %d names", len(names))
	}

	// Add integrations
	registry.Register(newMockIntegration("integration-1"))
	registry.Register(newMockIntegration("integration-2"))
	registry.Register(newMockIntegration("integration-3"))

	names = registry.Names()
	if len(names) != 3 {
		t.Errorf("Expected 3 names, got %d", len(names))
	}

	// Verify all names are present
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	for _, expected := range []string{"integration-1", "integration-2", "integration-3"} {
		if !nameMap[expected] {
			t.Errorf("Expected name %s not found in names list", expected)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	registry := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			name := fmt.Sprintf("integration-%d", index)
			registry.Register(newMockIntegration(name))
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			registry.List()
			registry.Count()
			registry.Names()
		}()
	}

	wg.Wait()

	// Verify registry is still consistent
	if registry.Count() != 100 {
		t.Errorf("Expected 100 integrations after concurrent access, got %d", registry.Count())
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Get global registry
	registry1 := GlobalRegistry()
	if registry1 == nil {
		t.Fatal("GlobalRegistry returned nil")
	}

	// Verify singleton behavior
	registry2 := GlobalRegistry()
	if registry1 != registry2 {
		t.Error("GlobalRegistry did not return the same instance")
	}

	// Test that it works like a normal registry
	mockInt := newMockIntegration("global-test")
	registry1.Register(mockInt)

	integration, err := registry2.Get("global-test")
	if err != nil {
		t.Fatalf("Failed to get integration from global registry: %v", err)
	}

	if integration.GetName() != "global-test" {
		t.Errorf("Expected name 'global-test', got '%s'", integration.GetName())
	}
}
