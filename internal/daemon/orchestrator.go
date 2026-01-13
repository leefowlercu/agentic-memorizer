package daemon

import (
	"context"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp"
	"github.com/leefowlercu/agentic-memorizer/internal/metrics"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// Orchestrator manages the initialization and wiring of all daemon components.
type Orchestrator struct {
	daemon           *Daemon
	registry         registry.Registry
	mcpServer        *mcp.Server
	metricsCollector *metrics.Collector
}

// NewOrchestrator creates a new orchestrator for the daemon.
func NewOrchestrator(d *Daemon) *Orchestrator {
	return &Orchestrator{
		daemon: d,
	}
}

// Initialize sets up all components in the correct order.
// Startup sequence: Registry -> Graph -> Cache -> Providers -> Walker -> Watcher -> Queue -> MCP
func (o *Orchestrator) Initialize(ctx context.Context) error {
	// 1. Initialize SQLite Registry
	registryPath := config.GetPath("database.registry_path")
	reg, err := registry.Open(ctx, registryPath)
	if err != nil {
		return err
	}
	o.registry = reg

	// Note: Graph, Cache, Providers, Walker, Watcher, and Queue components
	// are not yet integrated as they require external dependencies (FalkorDB)
	// or further implementation. This orchestrator sets up the framework for
	// adding these components.

	// Initialize MCP Server (without graph for now)
	// MCP server will be fully initialized when graph component is available
	mcpCfg := mcp.DefaultConfig()
	// TODO: Initialize MCP server with graph when available
	_ = mcpCfg

	// Initialize Metrics Collector
	metricsInterval := time.Duration(config.GetInt("metrics.collection_interval")) * time.Second
	if metricsInterval == 0 {
		metricsInterval = 15 * time.Second
	}
	o.metricsCollector = metrics.NewCollector(metricsInterval)

	// Set metrics handler on daemon server
	o.daemon.server.SetMetricsHandler(metrics.Handler())

	// Set rebuild function on daemon server
	o.daemon.server.SetRebuildFunc(o.handleRebuild)

	return nil
}

// Start starts all orchestrated components.
func (o *Orchestrator) Start(ctx context.Context) error {
	// Registry is already initialized and doesn't need a Start call

	// Start MCP server
	if o.mcpServer != nil {
		if err := o.mcpServer.Start(ctx); err != nil {
			return err
		}
	}

	// Start metrics collector
	if o.metricsCollector != nil {
		if err := o.metricsCollector.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Stop stops all orchestrated components in reverse order.
func (o *Orchestrator) Stop(ctx context.Context) error {
	// Stop metrics collector
	if o.metricsCollector != nil {
		o.metricsCollector.Stop(ctx)
	}

	// Stop MCP server
	if o.mcpServer != nil {
		o.mcpServer.Stop(ctx)
	}

	// Close registry
	if o.registry != nil {
		o.registry.Close()
	}

	return nil
}

// handleRebuild handles rebuild requests from the HTTP API.
func (o *Orchestrator) handleRebuild(ctx context.Context, full bool) (*RebuildResult, error) {
	start := time.Now()

	// Get all remembered paths from registry
	paths, err := o.registry.ListPaths(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: Trigger walker for each path when walker component is integrated
	// For now, return a stub result showing the paths that would be rebuilt

	filesQueued := 0
	dirsProcessed := len(paths)

	// Placeholder: In a full implementation, this would:
	// 1. For each path, trigger the walker
	// 2. Walker publishes file.discovered events
	// 3. Analysis queue processes events
	// 4. Return actual counts

	duration := time.Since(start)

	return &RebuildResult{
		Status:        "completed",
		FilesQueued:   filesQueued,
		DirsProcessed: dirsProcessed,
		Duration:      duration.Round(time.Millisecond).String(),
	}, nil
}

// Registry returns the initialized registry.
func (o *Orchestrator) Registry() registry.Registry {
	return o.registry
}

// MCPServer returns the initialized MCP server.
func (o *Orchestrator) MCPServer() *mcp.Server {
	return o.mcpServer
}

// MetricsCollector returns the initialized metrics collector.
func (o *Orchestrator) MetricsCollector() *metrics.Collector {
	return o.metricsCollector
}
