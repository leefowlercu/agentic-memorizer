# CLAUDE.md

## Table of Contents

- [Project Overview](#project-overview)
- [Project Principles](#project-principles)
- [Project Conventions & Patterns](#project-conventions--patterns)
- [High-Level Architecture](#high-level-architecture)
- [Subsystems Reference](#subsystems-reference)
- [Code Organization](#code-organization)
- [Testing Approach](#testing-approach)
- [Development Commands](#development-commands)

## Project Overview

Agentic Memorizer is an automated knowledge graph builder that monitors user-configured filesystem paths, applies a set of filters, analyzes content using AI providers, and maintains a searchable graph. The daemon watches and walks registered directories for changes and automatically processes files through format-specific chunkers, semantic analysis, and embeddings generation. Results are exposed to AI assistants via the Model Context Protocol (MCP), Hooks, and Plugins.

Key capabilities:
- **Filesystem Monitoring**: Watches registered directories for file changes with event coalescing
- **Intelligent Chunking**: 20+ format-specific chunkers for code (Tree-sitter AST), documents (PDF, DOCX, ODT), markup (Markdown, LaTeX, HTML), configuration (TOML, HCL, Dockerfile), data formats (JSON, YAML, SQL), and notebooks (Jupyter)
- **Semantic Analysis**: Pluggable AI providers (Anthropic, OpenAI, Google) extract topics, entities, and summaries
- **Vector Embeddings**: OpenAI, Voyage AI, and Google providers for semantic similarity search
- **Knowledge Graph**: FalkorDB (Redis Graph) backend with typed metadata relationships
- **AI Tool Integration**: MCP server and hooks for Claude Code, Gemini CLI, Codex, and OpenCode

## Project Principles

1. **Unix Philosophy**: Each component does one thing well. Data flows through text-based formats (JSON, YAML). Components are composable and can be scripted. Output is silent by default; verbosity is opt-in. All state is inspectable and human-readable.

2. **Graceful Component Degradation**: The system continues operating with reduced functionality when external services fail. If the graph connection fails, the daemon enters degraded mode but continues processing. If a provider is unavailable, analysis proceeds without that capability. Failures are logged and surfaced via health endpoints, never silently ignored.

3. **Loose Coupling**: Components communicate via an event bus rather than direct method calls. The watcher publishes events; the queue subscribes. The cleaner subscribes to deletion events. Components can be replaced or extended without modifying their consumers.

4. **Eventual Consistency**: The filesystem is the source of truth. Changes produce events that propagate asynchronously through the system. The knowledge graph reflects filesystem state only after processing completes. Queries may return stale data during processing; this is acceptable.

5. **Observability**: Comprehensive logging, health checks, and status commands provide visibility into system state. Each component logs key events and errors with context. Health endpoints report component status and degradation.

6. **Extensibility**: Interface-first design, registry patterns, and event-driven architecture enables easy addition or replacement of the concrete types of individual components.

### Adhering to Project Principles

1. **Unix Philosophy**:
   - New components should have a single, clear responsibility
   - Prefer text-based serialization (JSON, YAML) over binary formats
   - Support `--quiet` and `--verbose` flags where applicable
   - Expose internal state via health endpoints or status commands
   - Design for scriptability: predictable exit codes, machine-parseable output options

2. **Graceful Component Degradation**:
   - Initialize optional components (graph, providers, MCP) with error handling that logs warnings and continues
   - Track degraded state via boolean flags (e.g., `graphDegraded`, `mcpDegraded`)
   - Surface degradation in health endpoints and status commands
   - Never crash the daemon due to external service failures
   - Use retry logic with configurable backoff for transient failures

3. **Loose Coupling**:
   - Components should depend on interfaces, not concrete implementations
   - Use the event bus for cross-component communication instead of direct calls
   - New functionality should subscribe to existing events rather than modify publishers
   - Registries (chunkers, handlers, providers) enable runtime component selection
   - Avoid circular dependencies between packages

4. **Eventual Consistency**:
   - Accept that queries may return stale data during processing
   - Design reconciliation logic to handle filesystem changes that occurred during walks
   - Use the cleaner to remove stale graph entries after walks complete
   - Emit events when state changes so dependent components can react
   - Avoid synchronous dependencies between the filesystem state and graph state

## Project Conventions & Patterns

1. **Typed Configuration & User Input**: Access configuration via typed structs, never string keys. CLI flags use variable-based storage with `{commandName}{FlagName}` naming. All user input is validated in `PreRunE` hooks before business logic executes.
   ```go
   // Typed config access
   cfg := config.Get()
   port := cfg.Daemon.HTTPPort
   pidFile := config.ExpandPath(cfg.Daemon.PIDFile)

   // Variable-based flag storage
   var rebuildForce bool
   func init() {
       RebuildCmd.Flags().BoolVar(&rebuildForce, "force", false, "Force rebuild")
   }

   // PreRunE validation pattern
   var MyCmd = &cobra.Command{
       PreRunE: validateMy,
       RunE:    runMy,
   }
   func validateMy(cmd *cobra.Command, args []string) error {
       // Validate input...
       cmd.SilenceUsage = true  // Set AFTER validation passes
       return nil
   }
   ```

2. **Interface-First Design**: Major subsystems define behavior through interfaces before implementation. `Graph`, `Walker`, `Watcher`, `Registry`, `Bus`, `Chunker`, and `Provider` are all interfaces with concrete implementations. This enables testing via mocks, component substitution, and clear contracts between packages.

3. **Functional Options Pattern**: Constructors use `WithXXX` option functions for configuration:
   ```go
   q := analysis.NewQueue(bus,
       analysis.WithWorkerCount(4),
       analysis.WithLogger(slog.Default()),
   )
   ```

4. **Registry Pattern with Priority Selection**: Pluggable components use centralized registries with priority ordering. When a component fails, the system falls through to lower-priority alternatives. Chunkers, handlers, and providers all use this pattern.

5. **Ordered Component Lifecycle**: Components follow Initialize→Start→Stop lifecycle. The Orchestrator initializes in dependency order and shuts down in reverse order. This ensures pending work drains before dependencies close.

6. **Panic Recovery in Concurrent Code**: Event handlers and worker goroutines recover from panics to prevent cascading failures. Panics are logged with context but don't crash the daemon.

7. **Error Handling**: Use semicolons (not colons) when wrapping errors for cleaner CLI output:
   ```go
   return fmt.Errorf("failed to initialize config; %w", err)  // Correct
   return fmt.Errorf("failed to initialize config: %w", err)  // Incorrect
   ```

8. **Structured Logging**: Use `log/slog` with key-value pairs. Add component context via `.With()`:
   ```go
   slog.Info("starting daemon", "http_port", cfg.Daemon.HTTPPort)
   logger := slog.Default().With("component", "graph")
   ```

9. **CLI Command Organization**: Commands organized by domain with subcommands in nested directories:
   ```
   cmd/{parent}/
   ├── {parent}.go              # Parent command definition
   └── subcommands/
       ├── {subcommand}.go      # One file per subcommand
       └── helpers.go           # Shared utilities
   ```

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLI Layer (Cobra)                           │
│  [daemon] [remember] [forget] [list] [read] [integrations] [config] │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────────┐
│                         Daemon Core                                 │
│  Component Lifecycle │ Health Manager │ HTTP Server (7600)          │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
      ┌────────────────────────┼────────────────────────┐
      │                        │                        │
┌─────▼─────┐  ┌───────────────▼───────────────┐  ┌─────▼───────┐
│  Watcher  │  │     Analysis Pipeline         │  │   Graph     │
│ (fsnotify)│  │  Queue → Workers → Handlers   │  │ (FalkorDB)  │
└─────┬─────┘  └───────────────┬───────────────┘  └─────────────┘
      │                        │
      │         ┌──────────────┼──────────────┐
      │         │              │              │
      │    ┌────▼────┐   ┌─────▼─────┐  ┌─────▼──────┐
      │    │Chunkers │   │ Semantic  │  │ Embeddings │
      │    │ (20+)   │   │ Providers │  │ Providers  │
      │    └─────────┘   └───────────┘  └────────────┘
      │
      └──→ Event Bus ──→ Analysis Queue
```

**Data Flow:**
1. Watcher detects filesystem changes in remembered directories
2. Events are coalesced and published to the event bus
3. Analysis workers process files through chunkers and AI providers
4. Results are persisted to the FalkorDB knowledge graph
5. CLI, MCP server, and integrations query the graph

**Key External Dependencies:**
- **FalkorDB**: Redis Graph for knowledge storage
- **SQLite**: Registry database for remembered paths
- **AI Providers**: Anthropic, OpenAI, Google, Voyage AI

## Subsystems Reference

No subsystem documentation exists yet at `docs/subsystems/`. Key internal packages:

| Package | Purpose |
|---------|---------|
| `internal/daemon` | Daemon lifecycle, health monitoring, component orchestration |
| `internal/watcher` | Filesystem monitoring with fsnotify |
| `internal/analysis` | Work queue, workers, and analysis pipeline |
| `internal/chunkers` | 20+ format-specific file chunkers |
| `internal/providers` | Semantic analysis and embeddings providers |
| `internal/graph` | FalkorDB client and schema definitions |
| `internal/config` | Typed configuration with Viper |
| `internal/registry` | SQLite registry for remembered paths |
| `internal/integrations` | Hook, MCP, and plugin integrations |
| `internal/mcp` | Model Context Protocol server |

## Code Organization

```
agentic-memorizer/
├── main.go                 # Entry point
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go             # Root command with PersistentPreRunE
│   ├── daemon/             # Daemon subcommands
│   ├── remember/           # Path registration
│   └── ...                 # Other command groups
├── internal/               # Internal packages
│   ├── config/             # Configuration (types, defaults, validate, load)
│   ├── daemon/             # Daemon lifecycle management
│   ├── chunkers/           # File chunkers by format
│   │   └── code/           # Tree-sitter code chunkers
│   ├── providers/          # AI provider implementations
│   │   ├── semantic/       # Anthropic, OpenAI, Google
│   │   └── embeddings/     # OpenAI, Voyage, Google
│   ├── graph/              # FalkorDB client and models
│   └── ...                 # Other subsystems
├── testdata/               # Test fixtures organized by type
├── Makefile                # Build automation
└── config.yaml.example     # Example configuration
```

## Testing Approach

**Framework**: Go stdlib `testing` package only (no testify, ginkgo, etc.)

**Table-Driven Tests**: Standard pattern throughout:
```go
tests := []struct {
    name string
    // fields...
}{
    {"case 1", ...},
    {"case 2", ...},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test logic
    })
}
```

**Test Utilities** (`internal/testutil/`):
- `TestEnv` provides isolated test environments with temp directories
- Automatic cleanup via `t.Cleanup()`
- Environment variable isolation via `t.Setenv()`

**Coverage**: 90+ test files across all major subsystems including config, chunkers, graph, commands, and providers.

## Development Commands

### Building & Testing

```bash
# Build the binary
make build

# Build and install to ~/.local/bin
make install

# Run all tests
make test

# Run tests with race detector
make test-race

# Run linter
make lint

# Run linter with auto-fix
make lint-fix

# Clean build artifacts
make clean
```

### Running the Application

```bash
# Run interactive setup wizard
memorizer initialize

# Start the daemon (foreground)
memorizer daemon start

# Stop the daemon
memorizer daemon stop

# Check daemon status
memorizer daemon status

# Remember a directory
memorizer remember ~/projects/myapp

# List remembered directories
memorizer list

# Export knowledge graph
memorizer read --format json

# Setup an integration
memorizer integrations setup claude-code-mcp
```
