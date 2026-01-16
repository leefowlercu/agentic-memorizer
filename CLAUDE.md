# CLAUDE.md

## Table of Contents

- [Project Overview](#project-overview)
- [Project Principles](#project-principles)
- [High-Level Architecture](#high-level-architecture)
- [Subsystems Reference](#subsystems-reference)
- [Conventions & Patterns](#conventions--patterns)
- [Code Organization Principles](#code-organization-principles)
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

1. **Unix Philosophy**: Single-purpose components, text-based I/O, composability, transparency
2. **Graceful Degradation**: System continues operating with reduced functionality when external services fail
3. **Type Safety**: Typed configuration structs, compile-time safety over runtime string keys
4. **Test-Driven Development**: Table-driven tests, comprehensive coverage, stdlib testing only
5. **Minimal Dependencies**: Prefer stdlib solutions; external dependencies must justify their inclusion

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

## Conventions & Patterns

### Error Handling
Use semicolons (not colons) when wrapping errors:
```go
return fmt.Errorf("failed to initialize config; %w", err)  // Correct
return fmt.Errorf("failed to initialize config: %w", err)  // Incorrect
```

### Typed Configuration
Access config via typed structs, never string keys:
```go
cfg := config.Get()
port := cfg.Daemon.HTTPPort           // Typed access
pidFile := config.ExpandPath(cfg.Daemon.PIDFile)  // Path expansion
```

### CLI Command Organization
```
cmd/{parent}/
├── {parent}.go              # Parent command definition
└── subcommands/
    ├── {subcommand}.go      # One file per subcommand
    └── helpers.go           # Shared utilities
```

### PreRunE Validation Pattern
All commands implement PreRunE for input validation:
```go
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

### Flag Storage
Use variable-based storage with `{commandName}{FlagName}` naming:
```go
var rebuildForce bool

func init() {
    RebuildCmd.Flags().BoolVar(&rebuildForce, "force", false, "Force rebuild")
}
```

### Logging
Use `log/slog` with structured key-value pairs:
```go
slog.Info("starting daemon", "http_port", cfg.Daemon.HTTPPort)
```

## Code Organization Principles

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

**Principles:**
- Follow Golang Standards Project Layout (without `pkg/`)
- Internal packages in `internal/` are not importable externally
- Commands organized by domain with subcommands in nested directories
- Test files colocated with source (`*_test.go`)

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
