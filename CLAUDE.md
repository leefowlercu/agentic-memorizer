# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

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

Agentic Memorizer is a local file memorizer for Claude Code that provides automatic awareness and understanding of files through AI-powered semantic analysis, plus user-defined facts that inject persistent context into every conversation. A background daemon watches a memory directory, extracts metadata, performs semantic analysis via multiple AI providers (Claude, OpenAI, Gemini), and maintains a knowledge graph in FalkorDB that integrates with Claude via hooks (SessionStart for files, UserPromptSubmit for facts) and MCP tools.

**Current Version:** v0.13.0

## Project Principles

The following principles guide development decisions for this project:

### Unix Philosophy
- Single-purpose components that do one thing well
- Text-based I/O and data formats (JSON, YAML, XML)
- Composability and loose coupling between subsystems
- Silence by default; support --quiet and --verbose
- Transparency with inspectable state and human-readable formats

### Separation of Concerns
- Metadata extraction is fast and deterministic
- Semantic analysis is slow and AI-powered
- These concerns are separated to enable efficient caching and parallel processing

### Provider Abstraction
- All external AI providers implement common interfaces
- New providers can be added without changing core logic
- Provider-specific behavior is encapsulated in adapter implementations

### Content-Addressable Storage
- Cache keys are content hashes, not file paths
- Enables cache hits across file renames/moves
- Automatic invalidation when content changes

### Convention Over Configuration
- Sensible defaults for all settings
- Users only configure what they need to customize
- Three tiers: minimal (shown), advanced (available), hardcoded (internal)

## High-Level Architecture

### Three-Phase Processing Pipeline

Files are processed through three distinct phases:

1. **Metadata Extraction** (`internal/metadata/`) - Fast, deterministic extraction using specialized handlers for 8 file type categories (Markdown, Docx, Pptx, PDF, Image, VTT, JSON, Code)
2. **Semantic Analysis** (`internal/semantic/`) - AI-powered content understanding via provider abstraction supporting Claude, OpenAI, and Gemini
3. **Knowledge Graph Storage** (`internal/graph/`) - FalkorDB stores files, tags, topics, entities, and relationships for semantic search

### Background Daemon

The daemon (`internal/daemon/`) orchestrates file discovery:
- **Walker** (`internal/walker/`) - Full directory scans during rebuilds
- **File Watcher** (`internal/watcher/`) - Real-time monitoring with fsnotify and debouncing
- **Worker Pool** - Parallel processing with rate limiting

Jobs flow through a worker pool:
1. Extract metadata and compute SHA-256 content hash
2. Check cache (if hit, skip analysis)
3. On cache miss: acquire rate limiter token, perform semantic analysis, store result
4. Store in FalkorDB knowledge graph with relationships

### FalkorDB Knowledge Graph

**Schema:**
- Nodes: File, Tag, Topic, Entity, Category, Directory, Fact
- Edges: HAS_TAG, COVERS_TOPIC, MENTIONS, IN_CATEGORY, REFERENCES, SIMILAR_TO, IN_DIRECTORY, PARENT_OF

**HTTP API** (`internal/daemon/api/`):
- `GET /health` - Health with graph metrics
- `POST /api/v1/search` - Graph-powered semantic search
- `GET /api/v1/files/{path}` - File metadata with connections
- `GET /api/v1/files/recent` - Recently modified files
- `GET /api/v1/files/related` - Related files by shared tags/topics
- `GET /api/v1/entities/search` - Files mentioning an entity
- `POST /api/v1/rebuild` - Trigger rebuild
- `GET /sse` - Server-Sent Events for real-time updates

### Integration Framework

The Integration Registry (`internal/integrations/`) provides framework-agnostic integration:
- **Adapter Pattern** - Common Integration interface with specialized implementations
- **Registry Pattern** - Thread-safe singleton managing adapter registration
- **Dual-Hook Architecture** - SessionStart for files, UserPromptSubmit/BeforeAgent for facts
- **MCP Servers** - On-demand tools (search, metadata, related files, entity search)

**Active Integrations:**
- Claude Code: `claude-code-hook` + `claude-code-mcp`
- Gemini CLI: `gemini-cli-hook` + `gemini-cli-mcp`
- Codex CLI: `codex-cli-mcp`

### MCP Server

The MCP server (`internal/mcp/`) implements Model Context Protocol:
- JSON-RPC 2.0 stdio transport
- Five tools: search_files, get_file_metadata, list_recent_files, get_related_files, search_entities
- Connects to daemon HTTP API for graph queries

### Configuration System

Layered configuration with precedence: defaults -> YAML file -> environment variables (MEMORIZER_* prefix).

**Key Sections:** semantic, daemon, mcp, graph, embeddings

**Hot-reloadable:** provider, api_key, model, workers, debounce_ms, log_level, http_port

**Requires restart:** memory_root, cache_dir, log_file

## Subsystems Reference

Detailed technical documentation for each subsystem is available in `docs/subsystems/`:

| Subsystem | Description |
|-----------|-------------|
| [cache](docs/subsystems/cache/) | Content-addressable caching with three-tier versioning and provider isolation |
| [cli](docs/subsystems/cli/) | Cobra-based CLI with hierarchical commands and PreRunE validation |
| [config](docs/subsystems/config/) | Layered configuration with YAML, environment overrides, and hot-reload |
| [daemon](docs/subsystems/daemon/) | Background file monitoring with parallel processing and HTTP API |
| [docker](docs/subsystems/docker/) | Docker container lifecycle management for FalkorDB |
| [document](docs/subsystems/document/) | Office document extraction (DOCX, PPTX) via ZIP/XML parsing |
| [e2e](docs/subsystems/e2e/) | End-to-end testing with isolated environments and Docker FalkorDB |
| [embeddings](docs/subsystems/embeddings/) | Vector embeddings for semantic similarity search |
| [format](docs/subsystems/format/) | Structured CLI output with builder pattern and pluggable formatters |
| [graph](docs/subsystems/graph/) | FalkorDB-powered knowledge graph for files and relationships |
| [integrations](docs/subsystems/integrations/) | Framework-agnostic integration with dual-hook architecture |
| [logging](docs/subsystems/logging/) | Structured logging with slog, rotation, and context propagation |
| [mcp](docs/subsystems/mcp/) | Model Context Protocol with JSON-RPC 2.0 and graph-powered tools |
| [metadata](docs/subsystems/metadata/) | Fast metadata extraction with handlers for 8 file type categories |
| [semantic](docs/subsystems/semantic/) | Multi-provider AI content understanding with intelligent routing |
| [tui](docs/subsystems/tui/) | Interactive setup wizard built on Bubble Tea |
| [version](docs/subsystems/version/) | Build-time version injection with embedded fallback |
| [walker](docs/subsystems/walker/) | Recursive directory scanning with configurable filtering |
| [watcher](docs/subsystems/watcher/) | Real-time filesystem monitoring with debounced event batching |

## Conventions & Patterns

### Go Standards

- Follow Go Style Guide (https://google.github.io/styleguide/go/guide)
- Use `log/slog` for logging
- Use `any` instead of `interface{}`

### Logging Standards

All log messages must be entirely lowercase. Put proper nouns and important data in structured fields:

```go
// CORRECT
logger.Info("starting daemon", "version", v)
logger.Info("connection established", "database", "FalkorDB")

// INCORRECT
logger.Info("Starting daemon")
logger.Info("FalkorDB connection established")
```

### Error Message Formatting

Use semicolons (`;`) instead of colons (`:`) when wrapping errors:

```go
// CORRECT
return fmt.Errorf("failed to initialize config; %w", err)

// INCORRECT
return fmt.Errorf("failed to initialize config: %w", err)
```

**Rationale:** Root command prefixes errors with "Error: ". Semicolons create cleaner output.

### CLI Error Handling Pattern

All CLI commands (except root) implement `PreRunE` for input validation:
- Input validation errors (before `cmd.SilenceUsage = true`) -> show usage
- Runtime errors (after `cmd.SilenceUsage = true`) -> suppress usage

### CLI Output Formatting

All CLI commands use `internal/format` package. **Never use `fmt.Printf` for command output.**

**Available Builders:** Status, Section, Table, List, Progress, Error, FilesContent

**Output Formats:** text (default), json, yaml, xml, markdown

### Git Workflow

- Commit messages: conventional commit format, lowercase, single line
- No Claude Code coauthoring mentions

### API Rate Limiting

Provider-specific defaults:
- Claude: 20 calls/minute
- OpenAI: 60 calls/minute
- Gemini: 100 calls/minute

### Integration Adapter Versioning

Bump version appropriately when modifying adapters:
- PATCH: Bug fixes, refactoring, docs
- MINOR: New features, backward-compatible changes
- MAJOR: Breaking changes affecting user configuration

### Cache Versioning

Three-tier versioning in `internal/cache/version.go`:
- **SchemaVersion** - CachedAnalysis struct format
- **MetadataVersion** - Metadata extraction logic
- **SemanticVersion** - Semantic analysis logic

**When Modifying Cache-Related Code:**
1. Identify scope (struct format, metadata, or semantic)
2. Bump appropriate version
3. Mention cache version bump in commit message

## Code Organization Principles

### Subsystem Independence

Each major subsystem operates independently with clean boundaries:
- `internal/daemon/` - Daemon orchestration and worker pool
- `internal/metadata/` - Metadata extraction handlers
- `internal/semantic/` - Multi-provider semantic analysis
- `internal/cache/` - Content-addressable caching
- `internal/graph/` - FalkorDB graph storage and queries
- `internal/config/` - Configuration management
- `internal/integrations/` - Integration adapters
- `internal/watcher/` - Real-time file monitoring
- `internal/walker/` - Directory scanning
- `internal/mcp/` - MCP server protocol and transport
- `internal/format/` - Structured output formatting
- `internal/logging/` - Structured logging infrastructure
- `internal/document/` - Office document extraction
- `internal/tui/` - Terminal UI for initialization

### Handler/Adapter Patterns

Both metadata extraction and integration systems use handler/adapter patterns with registries. New file types or frameworks can be added by implementing interfaces and registering.

### Key File Locations

**CLI Commands:**
- Root: `cmd/root.go`
- Commands: `cmd/{initialize,daemon,integrations,config,read,remember,forget,mcp,graph,cache}/`
- Daemon subcommands (8): start, stop, status, restart, logs, rebuild, systemctl, launchctl
- Integration subcommands (5): detect, list, setup, remove, health

**Core Types:** `pkg/types/` - FileIndex, FileEntry, Fact, FactsIndex

## Testing Approach

The project uses Go's standard testing package with table-based tests.

**Unit Testing Patterns:**
- Metadata extractors: Test files with test data in `testdata/`
- Integration adapters: Test registration, detection, output formatting
- Worker pool: Test parallel processing, cache integration
- Configuration: Test validation rules, error accumulation
- MCP server: Test JSON-RPC 2.0 protocol, tool handlers

**End-to-End Testing:**
- Test harness (`e2e/harness/`) provides isolated environments
- 18 test files covering CLI, daemon, filesystem, HTTP API, SSE, config, graph
- Build tags: `//go:build e2e` separates from unit tests
- Docker integration for realistic FalkorDB testing

**Testing Guidelines:**
- Use `t.Run()` for subtests in table-driven tests
- Place test data in `testdata/` directory
- Mock external dependencies (Claude API, filesystem)
- Test error paths explicitly

## Development Commands

### Building & Testing

```bash
# Build and install
make build                    # Build binary with git version info
make install                  # Install to ~/.local/bin

# Testing
make test                     # Run unit tests
make test-integration         # Run integration tests
make test-all                 # Run all non-e2e tests (unit + integration)
make test-race                # Run tests with race detector
make test-e2e                 # Run E2E tests
make test-e2e-quick           # Run quick E2E smoke tests

# Code quality
make check                    # Run format, vet, test
make fmt                      # Format code with gofmt
make vet                      # Run go vet
make lint                     # Run golangci-lint
make coverage                 # Run tests with coverage
make coverage-html            # Generate HTML coverage report

# Cleanup
make clean                    # Clean build artifacts
make clean-cache              # Clean cache files
make uninstall                # Remove installed binary
make deps                     # Download and tidy dependencies

# Daemon development
make daemon-start             # Build and start daemon
make daemon-stop              # Stop daemon
make daemon-status            # Check daemon status
make daemon-logs              # Tail daemon logs
make validate-config          # Validate configuration

# Release
make release-patch            # 0.10.0 -> 0.10.1
make release-minor            # 0.10.0 -> 0.11.0
make release-major            # 0.10.0 -> 1.0.0
make goreleaser-check         # Validate goreleaser config
make goreleaser-snapshot      # Test build without release
```

### Running the Application

```bash
# Initialization
./memorizer initialize                        # Interactive setup
./memorizer initialize --integrations claude-code-hook,claude-code-mcp

# Graph database (required before daemon)
./memorizer graph start                       # Start FalkorDB
./memorizer graph status                      # Check graph status
./memorizer graph stop                        # Stop FalkorDB

# Daemon management
./memorizer daemon start                      # Start daemon
./memorizer daemon status                     # Check daemon status
./memorizer daemon stop                       # Stop daemon
./memorizer daemon restart                    # Restart daemon
./memorizer daemon rebuild                    # Rebuild knowledge graph
./memorizer daemon rebuild --clear-stale      # Rebuild with stale cache clearing
./memorizer daemon logs                       # View daemon logs

# Configuration
./memorizer config validate                   # Validate configuration
./memorizer config reload                     # Hot-reload non-structural settings
./memorizer config show-schema                # Show all available settings

# Cache management
./memorizer cache status                      # Check cache status
./memorizer cache clear --stale               # Clear stale cache entries
./memorizer cache clear --all                 # Clear all cache entries

# Integration
./memorizer mcp start                         # Start MCP server
./memorizer read files                        # Read file index
./memorizer read facts                        # Read user facts

# Facts management
./memorizer remember fact "fact content"      # Add a new fact
./memorizer forget fact <fact-id>             # Remove a fact by ID

# Service manager integration
./memorizer daemon systemctl                  # Generate systemd unit file (Linux)
./memorizer daemon launchctl                  # Generate launchd plist (macOS)
```
