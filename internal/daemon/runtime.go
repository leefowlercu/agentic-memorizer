package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/analysis"
	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/cleaner"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/handlers"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp"
	"github.com/leefowlercu/agentic-memorizer/internal/metrics"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
	"github.com/leefowlercu/agentic-memorizer/internal/watcher"
)

// ComponentKind describes whether a component is long-running or a job.
type ComponentKind string

const (
	ComponentKindPersistent ComponentKind = "persistent"
	ComponentKindJob        ComponentKind = "job"
)

// Criticality describes whether a component is fatal to the daemon.
type Criticality string

const (
	CriticalityFatal      Criticality = "fatal"
	CriticalityDegradable Criticality = "degradable"
)

// RestartPolicy determines whether a component is restarted on failure.
type RestartPolicy string

const (
	RestartNever     RestartPolicy = "never"
	RestartOnFailure RestartPolicy = "on_failure"
	RestartAlways    RestartPolicy = "always"
)

// RunStatus describes the result of a job run.
type RunStatus string

const (
	RunSuccess RunStatus = "success"
	RunPartial RunStatus = "partial"
	RunFailed  RunStatus = "failed"
)

// RunResult captures the outcome of a job run.
type RunResult struct {
	Status     RunStatus
	StartedAt  time.Time
	FinishedAt time.Time
	Counts     map[string]int
	Error      string
	Details    map[string]any
}

// ManagedComponent describes a long-running component.
type ManagedComponent interface {
	Name() string
	Kind() ComponentKind          // expect ComponentKindPersistent
	Criticality() Criticality     // fatal vs degradable
	RestartPolicy() RestartPolicy // restart behavior
	Dependencies() []string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() ComponentHealth // lightweight snapshot
}

// JobComponent describes a discrete job.
type JobComponent interface {
	Name() string
	Kind() ComponentKind // expect ComponentKindJob
	Dependencies() []string
	Run(ctx context.Context) RunResult
}

// JobRunEvent is emitted on job start/completion.
type JobRunEvent struct {
	Name       string
	Status     RunStatus
	Error      string
	Counts     map[string]int
	Details    map[string]any
	StartedAt  time.Time
	FinishedAt time.Time
}

// ComponentDefinition declares how to build a component and its metadata.
type ComponentDefinition struct {
	Name          string
	Kind          ComponentKind
	Criticality   Criticality
	RestartPolicy RestartPolicy
	Dependencies  []string
	Build         func(ctx context.Context, deps ComponentContext) (any, error)
	// FatalChan returns a channel for runtime fatal errors (optional).
	FatalChan func(component any) <-chan error
}

// RestartConfig controls backoff for restartable components.
type RestartConfig struct {
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

// ComponentContext provides access to previously built components.
type ComponentContext struct {
	Bus              *events.EventBus
	Registry         registry.Registry
	Graph            graph.Graph
	Handlers         *handlers.Registry
	Cleaner          *cleaner.Cleaner
	Queue            *analysis.Queue
	Walker           walker.Walker
	Watcher          watcher.Watcher
	MCP              *mcp.Server
	MetricsCollector *metrics.Collector
	Providers        struct {
		Semantic providers.SemanticProvider
		Embed    providers.EmbeddingsProvider
	}
	Caches struct {
		Semantic   *cache.SemanticCache
		Embeddings *cache.EmbeddingsCache
	}
}

// ComponentRegistry stores component definitions.
type ComponentRegistry struct {
	defs map[string]ComponentDefinition
}

// NewComponentRegistry creates an empty registry.
func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		defs: make(map[string]ComponentDefinition),
	}
}

// Register adds a definition.
func (r *ComponentRegistry) Register(def ComponentDefinition) {
	r.defs[def.Name] = def
}

// Definitions returns all registered definitions.
func (r *ComponentRegistry) Definitions() map[string]ComponentDefinition {
	return r.defs
}

// FilterByKind returns component names filtered by kind.
func (r *ComponentRegistry) FilterByKind(kind ComponentKind) []string {
	var out []string
	for name, def := range r.defs {
		if def.Kind == kind {
			out = append(out, name)
		}
	}
	return out
}

// TopologicalOrder returns component names ordered by dependencies.
func (r *ComponentRegistry) TopologicalOrder() ([]string, error) {
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	var order []string

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if temp[name] {
			return fmt.Errorf("circular dependency detected at %s", name)
		}
		def, ok := r.defs[name]
		if !ok {
			return fmt.Errorf("component %s not registered", name)
		}
		temp[name] = true
		for _, dep := range def.Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		temp[name] = false
		visited[name] = true
		order = append(order, name)
		return nil
	}

	for name := range r.defs {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}
