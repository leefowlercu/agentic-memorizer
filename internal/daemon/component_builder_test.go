package daemon

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

func TestNewComponentBuilder(t *testing.T) {
	cfg := config.NewDefaultConfig()

	builder := NewComponentBuilder(&cfg)

	if builder == nil {
		t.Fatal("expected non-nil builder")
	}
	if builder.registry == nil {
		t.Error("expected non-nil registry")
	}
	if builder.cfg != &cfg {
		t.Error("expected config to be set")
	}
}

func TestComponentBuilder_Registry(t *testing.T) {
	cfg := config.NewDefaultConfig()
	builder := NewComponentBuilder(&cfg)

	reg := builder.Registry()
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}

	// Verify component definitions were registered
	defs := reg.Definitions()
	if len(defs) == 0 {
		t.Error("expected component definitions to be registered")
	}

	// Verify key components are registered
	expectedComponents := []string{"bus", "registry", "graph", "queue", "walker", "watcher", "cleaner", "mcp"}
	for _, name := range expectedComponents {
		if _, ok := defs[name]; !ok {
			t.Errorf("expected component %q to be registered", name)
		}
	}
}

func TestComponentBuilder_TopologicalOrder(t *testing.T) {
	cfg := config.NewDefaultConfig()
	builder := NewComponentBuilder(&cfg)

	order, err := builder.Registry().TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder() error = %v", err)
	}

	// Verify bus comes before components that depend on it
	busIndex := indexOf(order, "bus")
	registryIndex := indexOf(order, "registry")
	graphIndex := indexOf(order, "graph")
	queueIndex := indexOf(order, "queue")

	if busIndex == -1 {
		t.Fatal("bus not in order")
	}
	if registryIndex == -1 {
		t.Fatal("registry not in order")
	}
	if graphIndex == -1 {
		t.Fatal("graph not in order")
	}

	// bus should come before components that depend on it
	if registryIndex < busIndex {
		t.Error("registry should come after bus")
	}
	if graphIndex < busIndex {
		t.Error("graph should come after bus")
	}
	if queueIndex < busIndex {
		t.Error("queue should come after bus")
	}
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func TestComponentBuilder_Build_FatalFailure(t *testing.T) {
	cfg := config.NewDefaultConfig()
	builder := &ComponentBuilder{
		registry: NewComponentRegistry(),
		cfg:      &cfg,
		logger:   slog.Default(),
	}

	// Register a fatal component that will fail
	builder.registry.Register(ComponentDefinition{
		Name:          "fatal_component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityFatal,
		RestartPolicy: RestartNever,
		Dependencies:  nil,
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			return nil, errors.New("simulated fatal failure")
		},
	})

	_, err := builder.Build(context.Background())
	if err == nil {
		t.Error("expected error for fatal component failure")
	}
}

func TestComponentBuilder_Build_DegradableFailure(t *testing.T) {
	cfg := config.NewDefaultConfig()
	builder := &ComponentBuilder{
		registry: NewComponentRegistry(),
		cfg:      &cfg,
		logger:   slog.Default(),
	}

	// Register a degradable component that will fail
	builder.registry.Register(ComponentDefinition{
		Name:          "degradable_component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  nil,
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			return nil, errors.New("simulated degradable failure")
		},
	})

	bag, err := builder.Build(context.Background())
	if err != nil {
		t.Errorf("expected no error for degradable component failure, got: %v", err)
	}
	if bag == nil {
		t.Error("expected non-nil bag even with degradable failures")
	}
}

func TestComponentBuilder_Build_NilReturn(t *testing.T) {
	cfg := config.NewDefaultConfig()
	builder := &ComponentBuilder{
		registry: NewComponentRegistry(),
		cfg:      &cfg,
		logger:   slog.Default(),
	}

	// Register a component that returns nil (e.g., disabled feature)
	builder.registry.Register(ComponentDefinition{
		Name:          "disabled_component",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  nil,
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			return nil, nil // Disabled, no error
		},
	})

	bag, err := builder.Build(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bag == nil {
		t.Error("expected non-nil bag")
	}
}

func TestComponentBuilder_Build_DependencyOrder(t *testing.T) {
	cfg := config.NewDefaultConfig()
	builder := &ComponentBuilder{
		registry: NewComponentRegistry(),
		cfg:      &cfg,
		logger:   slog.Default(),
	}

	var buildOrder []string

	// Register components with dependencies
	builder.registry.Register(ComponentDefinition{
		Name:          "base",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  nil,
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			buildOrder = append(buildOrder, "base")
			return "base", nil
		},
	})

	builder.registry.Register(ComponentDefinition{
		Name:          "dependent",
		Kind:          ComponentKindPersistent,
		Criticality:   CriticalityDegradable,
		RestartPolicy: RestartNever,
		Dependencies:  []string{"base"},
		Build: func(ctx context.Context, deps ComponentContext) (any, error) {
			buildOrder = append(buildOrder, "dependent")
			return "dependent", nil
		},
	})

	_, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(buildOrder) != 2 {
		t.Fatalf("expected 2 builds, got %d", len(buildOrder))
	}

	// base should be built before dependent
	if buildOrder[0] != "base" || buildOrder[1] != "dependent" {
		t.Errorf("wrong build order: %v", buildOrder)
	}
}

func TestComponentBuilder_WithBuilderLogger(t *testing.T) {
	cfg := config.NewDefaultConfig()

	// Just verify the option doesn't panic
	builder := NewComponentBuilder(&cfg, WithBuilderLogger(nil))
	if builder == nil {
		t.Fatal("expected non-nil builder")
	}
}
