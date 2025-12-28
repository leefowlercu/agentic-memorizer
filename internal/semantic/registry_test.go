package semantic

import (
	"log/slog"
	"sort"
	"sync"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.providers == nil {
		t.Fatal("NewRegistry returned registry with nil providers map")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	mockFactory := func(config ProviderConfig, logger *slog.Logger) (Provider, error) {
		return nil, nil
	}

	// Register a provider
	r.Register("test-provider", mockFactory)

	// Verify it was registered
	factory, err := r.Get("test-provider")
	if err != nil {
		t.Fatalf("Get failed after Register: %v", err)
	}
	if factory == nil {
		t.Fatal("Get returned nil factory after Register")
	}
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	r := NewRegistry()

	var callCount int
	mockFactory1 := func(config ProviderConfig, logger *slog.Logger) (Provider, error) {
		callCount = 1
		return nil, nil
	}
	mockFactory2 := func(config ProviderConfig, logger *slog.Logger) (Provider, error) {
		callCount = 2
		return nil, nil
	}

	// Register first factory
	r.Register("test", mockFactory1)

	// Overwrite with second factory
	r.Register("test", mockFactory2)

	// Verify second factory is used
	factory, _ := r.Get("test")
	_, _ = factory(ProviderConfig{}, nil)

	if callCount != 2 {
		t.Errorf("Expected factory2 to be called (callCount=2), got callCount=%d", callCount)
	}
}

func TestRegistry_Get(t *testing.T) {
	tests := []struct {
		name         string
		registerName string
		lookupName   string
		wantErr      bool
	}{
		{
			name:         "existing provider",
			registerName: "claude",
			lookupName:   "claude",
			wantErr:      false,
		},
		{
			name:         "non-existing provider",
			registerName: "claude",
			lookupName:   "openai",
			wantErr:      true,
		},
		{
			name:         "empty name lookup",
			registerName: "test",
			lookupName:   "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			r.Register(tt.registerName, func(config ProviderConfig, logger *slog.Logger) (Provider, error) {
				return nil, nil
			})

			_, err := r.Get(tt.lookupName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	// Empty registry
	names := r.List()
	if len(names) != 0 {
		t.Errorf("Expected empty list for empty registry, got %v", names)
	}

	// Register providers
	mockFactory := func(config ProviderConfig, logger *slog.Logger) (Provider, error) {
		return nil, nil
	}
	r.Register("claude", mockFactory)
	r.Register("openai", mockFactory)
	r.Register("gemini", mockFactory)

	// Verify all registered
	names = r.List()
	if len(names) != 3 {
		t.Errorf("Expected 3 providers, got %d", len(names))
	}

	// Sort for consistent comparison
	sort.Strings(names)
	expected := []string{"claude", "gemini", "openai"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected %s at index %d, got %s", expected[i], i, name)
		}
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := NewRegistry()
	mockFactory := func(config ProviderConfig, logger *slog.Logger) (Provider, error) {
		return nil, nil
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Register("provider", mockFactory)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.List()
			_, _ = r.Get("provider")
		}()
	}

	wg.Wait()

	// Verify final state
	names := r.List()
	if len(names) != 1 {
		t.Errorf("Expected 1 provider after concurrent access, got %d", len(names))
	}
}

func TestGlobalRegistry(t *testing.T) {
	// GlobalRegistry should return same instance
	r1 := GlobalRegistry()
	r2 := GlobalRegistry()

	if r1 != r2 {
		t.Error("GlobalRegistry should return the same instance")
	}

	if r1 == nil {
		t.Error("GlobalRegistry returned nil")
	}
}
