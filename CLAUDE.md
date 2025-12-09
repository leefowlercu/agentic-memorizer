# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Table of Contents

- [Project Overview](#project-overview)
- [Development Commands](#development-commands)
  - [Building and Testing](#building-and-testing)
  - [Daemon Development](#daemon-development)
  - [Running the Application](#running-the-application)
  - [Service Manager Integration](#service-manager-integration)
- [High-Level Architecture](#high-level-architecture)
  - [Three-Phase Processing Pipeline](#three-phase-processing-pipeline)
  - [Background Daemon Architecture](#background-daemon-architecture)
  - [FalkorDB Knowledge Graph](#falkordb-knowledge-graph)
  - [Semantic Search](#semantic-search)
  - [Integration Framework](#integration-framework)
  - [MCP Server](#mcp-server)
  - [Configuration System](#configuration-system)
- [Code Organization Principles](#code-organization-principles)
  - [Subsystem Independence](#subsystem-independence)
  - [Separation of Metadata and Semantics](#separation-of-metadata-and-semantics)
  - [Content-Addressable Caching](#content-addressable-caching)
  - [Cache Versioning](#cache-versioning)
  - [Handler/Adapter Patterns](#handleradapter-patterns)
- [Testing Approach](#testing-approach)
- [Key File Locations](#key-file-locations)
- [Development Notes](#development-notes)
  - [Go Standards](#go-standards)
  - [Git Workflow](#git-workflow)
  - [API Rate Limiting](#api-rate-limiting)
  - [Configuration Hot-Reload](#configuration-hot-reload)
  - [Binary Path in Integrations](#binary-path-in-integrations)
  - [Integration Adapter Versioning](#integration-adapter-versioning)
  - [CLI Error Handling Pattern](#cli-error-handling-pattern)
  - [Error Message Formatting](#error-message-formatting)
  - [CLI Output Formatting](#cli-output-formatting)
  - [Releasing](#releasing)

## Project Overview

Agentic Memorizer is a local file memorizer for Claude Code that provides automatic awareness and understanding of files through AI-powered semantic analysis. A background daemon watches a memory directory, extracts metadata, performs semantic analysis via Claude API, and maintains a precomputed index that loads into Claude's context via SessionStart hooks.

## Development Commands

### Building and Testing

```bash
# Build the binary with version information from git
make build

# Install to ~/.local/bin
make install

# Run all tests
make test

# Run tests with race detector (important for concurrent code)
make test-race

# Run tests with coverage
make coverage

# Generate HTML coverage report
make coverage-html

# Run code quality checks (format, vet, test)
make check

# Format code with gofmt
make fmt

# Run go vet
make vet

# Run golangci-lint (requires golangci-lint installation)
make lint

# Download and tidy dependencies
make deps

# Clean build artifacts
make clean

# Clean cache files
make clean-cache

# Run a specific test
go test -v -run TestName ./path/to/package

# Run E2E tests
make test-e2e

# Run quick E2E smoke tests
make test-e2e-quick
```

### Daemon Development

```bash
# Build and start daemon
make daemon-start

# Stop daemon
make daemon-stop

# Check daemon status
make daemon-status

# Tail daemon logs
make daemon-logs

# Validate configuration
make validate-config
```

### Running the Application

```bash
# Initialize with interactive prompts
./agentic-memorizer initialize

# Initialize with automated setup
./agentic-memorizer initialize --setup-integrations

# Start FalkorDB (required before daemon)
./agentic-memorizer graph start

# Check graph status
./agentic-memorizer graph status

# Start the daemon
./agentic-memorizer daemon start

# Check daemon status
./agentic-memorizer daemon status

# Stop the daemon
./agentic-memorizer daemon stop

# Stop FalkorDB
./agentic-memorizer graph stop

# Read the precomputed index
./agentic-memorizer read

# Validate configuration
./agentic-memorizer config validate

# Hot-reload configuration (non-structural settings)
./agentic-memorizer config reload

# Start MCP server (for Claude Code integration)
./agentic-memorizer mcp start

# Rebuild index (use --force to clear graph first)
./agentic-memorizer daemon rebuild

# Rebuild with stale cache clearing
./agentic-memorizer daemon rebuild --clear-old-cache

# Clear graph data by deleting persistence files
rm -rf ~/.agentic-memorizer/falkordb/*
docker restart memorizer-falkordb

# Check cache status
./agentic-memorizer cache status

# Clear stale cache entries
./agentic-memorizer cache clear --old-versions

# Clear all cache entries
./agentic-memorizer cache clear --all

# Generate systemd unit file (Linux)
./agentic-memorizer daemon systemctl

# Generate launchd plist file (macOS)
./agentic-memorizer daemon launchctl
```

### Service Manager Integration

**Implementation Philosophy:**
The daemon follows modern Go best practices by running in foreground mode and delegating process supervision to external service managers. This avoids self-daemonization anti-patterns (os/exec re-execution, fork-based approaches) in favor of battle-tested external tools.

**Why External Process Managers?**
- **Idiomatic**: Go community consensus strongly recommends against self-daemonization due to runtime complexity (goroutines, thread pools make fork() unreliable)
- **Reliable**: systemd, launchd, and supervisor provide production-grade process supervision, automatic restarts, and health monitoring
- **Observable**: Native integration with journald, Console.app, and centralized logging
- **Portable**: Same codebase works across Linux, macOS, and all distros without platform-specific fork logic
- **Simple**: Eliminates complex daemonization code, reduces edge cases, simplifies debugging

**Implementation Details:**

#### systemd Integration (`cmd/daemon/subcommands/systemctl.go`)
- **Command**: `daemon systemctl` - Generates systemd unit files
- **Type=notify Integration**: `internal/daemon/daemon.go:251-256` calls `daemon.SdNotify(false, daemon.SdNotifyReady)` after initialization completes
- **Dependency**: `github.com/coreos/go-systemd/v22/daemon` - Added in go.mod
- **Readiness Signal**: Sent after health server starts, ensuring systemd knows daemon is fully operational
- **Unit File Generation**: Detects binary path via `os.Executable()`, generates user and system-wide configurations
- **Features**: Type=notify, RestartSec=5s, TimeoutStartSec=60s, security hardening (NoNewPrivileges, PrivateTmp)

#### launchd Integration (`cmd/daemon/subcommands/launchctl.go`)
- **Command**: `daemon launchctl` - Generates launchd plist files
- **Features**: KeepAlive with SuccessfulExit=false, RunAtLoad, ThrottleInterval=30s
- **Plist Generation**: Uses config.GetConfig() to read log file path, detects binary path
- **Label Format**: `com.<username>.agentic-memorizer` for proper macOS service identification

#### Testing Commands
```bash
# Test systemd unit generation
./agentic-memorizer daemon systemctl

# Test launchd plist generation
./agentic-memorizer daemon launchctl

# Test Type=notify integration (requires systemd)
systemd-notify --status="Testing notification"
```

**Development Workflow:**
1. Daemon runs normally in foreground (`daemon start`)
2. Service files are generated on-demand via subcommands
3. Users install generated files to appropriate system locations
4. Service managers handle backgrounding, restarts, logging

**Files Modified:**
- `internal/daemon/daemon.go` - Added SdNotify call after health server initialization
- `cmd/daemon/subcommands/systemctl.go` - New command for systemd unit generation
- `cmd/daemon/subcommands/launchctl.go` - New command for launchd plist generation
- `cmd/daemon/daemon.go` - Registered new subcommands
- `go.mod` - Added github.com/coreos/go-systemd/v22 dependency

**Testing:**
- All existing tests pass (no regressions)
- Commands generate valid service files
- systemd Type=notify integration verified on Linux
- See README.md for end-user setup instructions

## High-Level Architecture

### Three-Phase Processing Pipeline

The system processes files through three distinct phases:

1. **Metadata Extraction** (`internal/metadata/`) - Fast, deterministic extraction of file-specific metadata (word counts, dimensions, page counts) using a handler pattern with specialized extractors for 9 file type categories
2. **Semantic Analysis** (`internal/semantic/`) - AI-powered content understanding via Claude API with content-based routing (text, vision for images, document blocks for PDFs, extraction for Office files) and entity extraction
3. **Knowledge Graph Storage** (`internal/graph/`) - FalkorDB graph database stores files, tags, topics, entities, and their relationships for semantic search and discovery

### Background Daemon Architecture

The daemon (`internal/daemon/`) orchestrates two complementary file discovery mechanisms:

- **Walker** (`internal/walker/`) - Performs full directory scans during rebuilds using callback-based visitor pattern with two-tier filtering (directory pruning and file filtering)
- **File Watcher** (`internal/watcher/`) - Real-time monitoring with fsnotify, implements debouncing (default 500ms) to batch rapid file changes

Jobs flow through a worker pool with priority calculation (recent files first) where each worker:
1. Extracts metadata via metadata extractor
2. Computes SHA-256 content hash for cache key
3. Checks cache (if hit, skip analysis)
4. On cache miss: waits for rate limiter token, performs semantic analysis, stores result
5. Returns index entry with metadata + semantic analysis
6. Stores entry in FalkorDB knowledge graph with relationships

### FalkorDB Knowledge Graph

The graph subsystem (`internal/graph/`) provides persistent storage and relationship-based queries:

**Graph Architecture:**
- **FalkorDB Backend** - Redis-compatible graph database running in Docker container
- **Manager** (`internal/graph/manager.go`) - Connection pooling, health checks, graceful degradation
- **Queries** (`internal/graph/queries.go`) - Cypher query execution for CRUD operations
- **Schema** (`internal/graph/schema.go`) - Node/edge type definitions and constraint management

**Node Types:**
- `File` - Indexed files with metadata (path, hash, size, category, summary)
- `Tag` - Semantic tags extracted during analysis
- `Topic` - Key topics identified from content
- `Entity` - Named entities (people, organizations, concepts)
- `Category` - File categories (documents, images, code, data, other)

**Relationship Types:**
- `HAS_TAG` - File → Tag relationships
- `COVERS_TOPIC` - File → Topic relationships
- `MENTIONS` - File → Entity relationships
- `IN_CATEGORY` - File → Category relationships

**Graph Commands** (`cmd/graph/`):
- `graph start` - Start FalkorDB Docker container
- `graph stop` - Stop container
- `graph status` - Show connection status and node counts

To rebuild the graph, use `daemon rebuild [--force]`.

**HTTP API** (`internal/daemon/api/`):
- `GET /health` - Daemon health with graph metrics
- `POST /api/v1/search` - Graph-powered semantic search
- `GET /api/v1/files/{path}` - File metadata with connections
- `GET /api/v1/files/recent` - Recently modified files
- `GET /api/v1/files/related` - Related files by shared tags/topics
- `GET /api/v1/entities/search` - Files mentioning an entity
- `POST /api/v1/rebuild` - Trigger index rebuild
- `GET /sse` - Server-Sent Events for real-time updates

**Graceful Degradation:**
When FalkorDB is unavailable, the daemon logs warnings but continues operating. Graph queries return empty results rather than errors.

**Data Persistence:**
FalkorDB stores data at `/data` inside the container, which is bind-mounted to `~/.agentic-memorizer/falkordb/`. Persistence files (`dump.rdb`) appear in this directory after data is saved.

**Clearing Graph Data:**
```bash
# Option A: Delete persistence files and restart (simplest)
rm -rf ~/.agentic-memorizer/falkordb/*
docker restart memorizer-falkordb

# Option B: Clear and rebuild via daemon
agentic-memorizer daemon rebuild --force

# Option C: Remove and recreate container (also clears data)
docker stop memorizer-falkordb && docker rm memorizer-falkordb
agentic-memorizer graph start
```

### Semantic Search

The graph manager (`internal/graph/`) provides graph-powered semantic search via Cypher queries:
- **Multi-Signal Search** - Queries across filename, tags, topics, and summary using OR logic
- **Tag-Based Search** - Matches files connected to tags containing search terms
- **Topic-Based Search** - Matches files connected to topics containing search terms
- **Summary Text Search** - Full-text search on file summary fields
- **Category Filtering** - Optional filtering by file category (documents, images, code, etc.)
- **Related Files** - Find files sharing tags/topics with a given file
- **Entity Search** - Find files mentioning specific entities
- **Recent Files** - Query by modification time with configurable window

The legacy in-memory search engine (`internal/search/`) remains available for fallback when graph is unavailable.

### Index Management

The Index Manager (`internal/index/`) maintains the precomputed index with:
- **Thread-safe operations** using sync.RWMutex for concurrent reads
- **Atomic writes** via temp file + rename pattern to prevent corruption
- **Two-level versioning**: schema version (index format) and daemon version (application release)

Index structure:
```
GraphIndex → []FileEntry
FileEntry → path, name, hash, type, category, size, modified + semantic fields (summary, tags, topics, entities)
Internal types (processing pipeline): FileMetadata, SemanticAnalysis, IndexEntry
```

### Integration Framework

The Integration Registry (`internal/integrations/`) provides framework-agnostic integration through:

- **Adapter Pattern** - Common Integration interface with specialized implementations (Claude Code, Gemini CLI, Codex CLI)
- **Registry Pattern** - Thread-safe singleton managing adapter registration and lookup
- **Output Processors** - Independent formatters (XML, Markdown, JSON) separate from integration wrapping
- **Auto-registration** - Adapters register via init() functions

Claude Code integration uses two methods:
1. **SessionStart hooks** (`claude-code-hook`) - Inject full index at session start via hooks with matchers (startup, resume, clear, compact)
2. **MCP server** (`claude-code-mcp`) - Provide on-demand tools for semantic search and metadata retrieval

Gemini CLI integration uses:
- **MCP server** (`gemini-cli-mcp`) - Provide on-demand tools via stdio transport configured in `~/.gemini/settings.json`

Codex CLI integration uses:
- **MCP server** (`codex-cli-mcp`) - Provide on-demand tools via stdio transport configured in `~/.codex/config.toml` (TOML format)

All integrations wrap output appropriately: SessionStart hooks use JSON envelope with systemMessage and additionalContext fields; MCP uses JSON-RPC 2.0 protocol responses.

**Configuration Formats:**
- Claude Code & Gemini CLI use JSON configuration files
- Codex CLI uses TOML configuration (`github.com/pelletier/go-toml/v2` library)

### MCP Server

The MCP server (`internal/mcp/`) implements Model Context Protocol for tool-based integration:
- **Protocol Layer** (`internal/mcp/protocol/`) - JSON-RPC 2.0 message types and handlers
- **Transport Layer** (`internal/mcp/transport/`) - Stdio transport for MCP communication
- **Server Lifecycle** - Initialize, initialized, shutdown sequence following MCP spec
- **Daemon Integration** - Connects to daemon HTTP API for graph-powered queries
- **Tool Integration** - Exposes five tools via daemon API:
  - `search_files` - Graph-powered semantic search across tags, topics, summary
  - `get_file_metadata` - File metadata with graph connections (related files, tags, topics)
  - `list_recent_files` - Recently modified files within configurable time window
  - `get_related_files` - Find files connected through shared tags, topics, or entities with ranked connection strength
  - `search_entities` - Find files mentioning specific entities (people, organizations, concepts) with optional type filtering
- **Logging** - Separate log file and level control via `mcp.log_file` and `mcp.log_level` config

### Configuration System

The Config Manager (`internal/config/`) implements layered configuration with precedence: defaults → YAML file → environment variables (MEMORIZER_* prefix). Supports hot-reload via `config reload` command for non-structural settings.

Configuration is organized into tiers:
- **Minimal tier** (shown in initialized config): Core settings users typically need to change
- **Advanced tier** (documented, not shown by default): Rarely changed settings with sensible defaults
- **Hardcoded** (documented, not configurable): Internal conventions that should not change

Use `config show-schema` to discover all available settings:
```bash
agentic-memorizer config show-schema                  # Show all settings
agentic-memorizer config show-schema --advanced-only  # Show advanced settings
agentic-memorizer config show-schema --hardcoded-only # Show hardcoded conventions
agentic-memorizer config show-schema --format yaml    # Output as YAML
```

Key configuration sections:
- `claude` - API credentials, model selection, timeout (5-300s), enable_vision toggle
- `analysis` - File size limits, skip patterns (`skip_files`, `skip_extensions`), cache directory (enable is derived from API key presence)
- `daemon` - Workers, debounce timing, rate limits, rebuild intervals, health check port
- `integrations` - Per-framework settings with type and custom settings
- `mcp` - MCP server log file, log level, and daemon connectivity (`daemon_host`, `daemon_port`)
- `graph` - FalkorDB host, port, database name, and password
- `embeddings` - API key, provider, model, and dimensions (enabled is derived from API key presence)

Settings like `analysis.enabled` and `embeddings.enabled` are automatically derived from API key presence.

Settings requiring daemon restart: `memory_root`, `analysis.cache_dir`, `daemon.log_file`.

Validation uses error accumulation pattern (collects all errors before failing) with structured ValidationError providing field, rule, message, suggestion, and value.

## Code Organization Principles

### Subsystem Independence

Each major subsystem (`internal/daemon/`, `internal/metadata/`, `internal/semantic/`, `internal/cache/`, `internal/index/`, `internal/config/`, `internal/integrations/`, `internal/watcher/`, `internal/walker/`, `internal/mcp/`, `internal/search/`, `internal/graph/`, `internal/version/`) operates independently with clean boundaries. The daemon orchestrates but doesn't tightly couple to implementation details.

### Separation of Metadata and Semantics

Metadata extraction is fast and deterministic; semantic analysis is slow and AI-powered. This separation enables efficient caching (metadata extraction always happens to compute hashes; semantic analysis is cached) and parallel processing (metadata is CPU-bound; semantic analysis is I/O-bound).

### Content-Addressable Caching

Cache keys are SHA-256 hashes of file content (not paths), enabling cache hits across file renames/moves and automatic invalidation on content changes. No explicit cache invalidation logic needed.

### Cache Versioning

The semantic analysis cache uses three-tier versioning to detect stale entries after application upgrades:

- **SchemaVersion** - CachedAnalysis struct format changes (bump when adding/removing/renaming fields)
- **MetadataVersion** - Metadata extraction logic changes (bump when metadata output changes)
- **SemanticVersion** - Semantic analysis logic changes (bump when prompts or analysis changes)

**Version Constants Location:** `internal/cache/version.go`

**Cache Key Format:** `{content-hash[:16]}-v{schema}-{metadata}-{semantic}.json`

Example: `sha256:abc12345def67890-v1-1-1.json`

**Staleness Detection:**
- Schema mismatch (any direction) = always stale
- Metadata/semantic version behind current = stale
- Metadata/semantic version ahead (future) = not stale (forward compatible)

**When Modifying Cache-Related Code:**
1. Identify scope of changes (struct format, metadata, or semantic)
2. Bump appropriate version constant in `internal/cache/version.go`
3. Commit message should mention cache version bump

**CLI Commands:**
- `cache status` - Show version distribution and entry counts
- `cache clear --old-versions` - Remove stale entries
- `cache clear --all` - Remove all entries
- `daemon rebuild --clear-old-cache` - Clear stale entries before rebuild

### Handler/Adapter Patterns

Both metadata extraction and integration systems use handler/adapter patterns with registries. New file types or frameworks can be added by implementing interfaces and registering—no changes to core logic required.

## Testing Approach

The project uses Go's standard testing package with table-based tests where appropriate. Key testing patterns:

- **Metadata extractors** - Each handler has dedicated test file with test data in `testdata/`
- **Integration adapters** - Test registration, detection, and output formatting
- **Worker pool** - Tests parallel processing, cache integration, priority ordering
- **Configuration** - Tests validation rules, error accumulation, path safety
- **MCP server** - Tests JSON-RPC 2.0 protocol implementation, tool handlers
- **Semantic search** - Tests substring matching, relevance scoring, tag/topic filtering

**End-to-End Testing:**

The project includes comprehensive E2E tests (`e2e/tests/`) covering complete workflows across all major subsystems:

- **Test Infrastructure** - Test harness framework (`e2e/harness/`) provides isolated environments, daemon management, HTTP/MCP/graph clients, and automatic cleanup
- **Test Suites** - 18 test files covering CLI commands, daemon lifecycle, filesystem integration, HTTP API, SSE, configuration, graph operations, integrations, and output formats
- **Build Tags** - E2E tests use `//go:build e2e` to separate from unit tests
- **Docker Integration** - FalkorDB runs in Docker container for realistic graph testing
- **Isolation** - Each test runs in temporary directory with unique daemon instance
- **Automatic Volume Cleanup** - All E2E test targets automatically remove Docker volumes after completion to prevent stale data from interfering with local development

Run E2E tests with `make test-e2e` or specific suites with `-run` flag. See `docs/subsystems/e2e-tests/` for architecture details.

**E2E Test Commands:**
```bash
# Run all E2E tests (with automatic volume cleanup)
make test-e2e

# Run quick smoke tests
make test-e2e-quick

# Run specific test suites
cd e2e && make test-cli          # CLI command tests
cd e2e && make test-daemon       # Daemon lifecycle tests
cd e2e && make test-mcp          # MCP server tests
cd e2e && make test-graph        # Graph operations tests

# Preserve volumes for debugging (use sparingly)
cd e2e && ./scripts/teardown.sh --keep-volumes

# Manual volume cleanup
cd e2e && docker-compose down -v
```

When writing tests:
- Use `t.Run()` for subtests within table-driven tests
- Place test data files in `testdata/` directory
- Mock external dependencies (Claude API, file system) where appropriate
- Test error paths explicitly (validation failures, missing files, malformed data)

## Key File Locations

**CLI Commands:**
- Root command: `cmd/root.go`
- Command packages: `cmd/{initialize,daemon,integrations,config,read,mcp,graph,cache}/`
- Daemon commands: `cmd/daemon/daemon.go` (parent) + `cmd/daemon/subcommands/` (8 subcommands: start, stop, status, restart, logs, rebuild, systemctl, launchctl)
- Graph commands: `cmd/graph/graph.go` (parent) + `cmd/graph/subcommands/` (3 subcommands)
- Cache commands: `cmd/cache/cache.go` (parent) + `cmd/cache/subcommands/` (2 subcommands: status, clear)
- Integration commands: `cmd/integrations/integrations.go` (parent) + `cmd/integrations/subcommands/` (6 subcommands + helpers)
- Config commands: `cmd/config/config.go` (parent) + `cmd/config/subcommands/` (2 subcommands)

**Core Subsystems:**
- Main subsystems: `internal/{daemon,metadata,semantic,cache,index,config,integrations,watcher,walker,mcp,search,graph,version}/`
- Graph package: `internal/graph/` - FalkorDB client, queries, schema, exporter
- Daemon API: `internal/daemon/api/` - HTTP server, SSE hub, request handlers
- Type definitions: `pkg/types/types.go`

**Documentation & Resources:**
- Subsystem documentation: `docs/subsystems/` - comprehensive technical documentation
- Test data: `testdata/` - files for testing metadata extraction
- Docker Compose: `docker-compose.yml` - FalkorDB container configuration

## Development Notes

### Go Standards

- Follow the Go Style Guide (https://google.github.io/styleguide/go/guide)
- Use `log/slog` for logging, not fmt.Printf
- Use `any` instead of `interface{}` where possible
- Generated code must pass all tests before being considered complete

### Git Workflow

- Commit messages use conventional commit format, lowercase, single line
- Do not mention Claude Code coauthoring in commit messages
- Current version: v0.12.1 (semantic versioning)

### API Rate Limiting

The daemon implements token bucket rate limiting (default 20 calls/minute) to respect Claude API quotas. Workers call `rateLimiter.Wait(ctx)` before semantic analysis. Adjust `daemon.rate_limit_per_min` in config as needed for your API tier.

### Configuration Hot-Reload

The daemon supports hot-reloading non-structural configuration changes via `config reload` command without requiring a restart. The `config reload` command sends SIGHUP to the daemon process, which detects changes and updates affected components.

**Important:** The daemon and MCP server are separate processes. Hot-reload only affects the daemon. MCP settings (`mcp.*`) require the MCP client to disconnect and reconnect to spawn a new MCP server instance.

**Settings requiring daemon restart:** `memory_root`, `analysis.cache_dir`, `daemon.log_file`

**Settings requiring MCP server restart:** `mcp.log_file`, `mcp.log_level`, `mcp.daemon_host`, `mcp.daemon_port`

**Hot-reloadable daemon settings:** Claude API settings, `daemon.workers`, `daemon.rate_limit_per_min`, `daemon.debounce_ms`, `daemon.log_level`, `daemon.http_port`, `daemon.full_rebuild_interval_minutes`, `analysis.skip_extensions`, `analysis.skip_files`

### Binary Path in Integrations

Integration setup commands automatically detect and configure the correct binary path. When modifying integration setup logic, ensure binary path detection works for both `go install` (in $GOPATH/bin) and `make install` (in ~/.local/bin) installations.

### Integration Adapter Versioning

Integration adapters use semantic versioning (MAJOR.MINOR.PATCH) to track changes. When modifying adapter code, update the version constant appropriately.

**Version Constants Location:**
- Claude Code Hook: `internal/integrations/adapters/claude/hook_adapter.go` → `IntegrationVersion`
- Claude Code MCP: `internal/integrations/adapters/claude/mcp_adapter.go` → `MCPIntegrationVersion`
- Gemini CLI MCP: `internal/integrations/adapters/gemini/mcp_adapter.go` → `MCPIntegrationVersion`
- Codex CLI MCP: `internal/integrations/adapters/codex/mcp_adapter.go` → `MCPIntegrationVersion`

**When to Bump Versions:**

- **PATCH** (0.0.X): Bug fixes, internal refactoring, documentation updates
  - Fix incorrect config file parsing
  - Improve error messages
  - Refactor internal helper functions

- **MINOR** (0.X.0): New features, backward-compatible changes
  - Add new optional configuration options
  - Support additional matchers or settings
  - Add new validation checks

- **MAJOR** (X.0.0): Breaking changes affecting user configuration
  - Change config file format or location
  - Rename or remove existing settings
  - Change default behavior in incompatible ways

**When modifying adapter code:**
1. Identify the scope of changes (bug fix, new feature, or breaking change)
2. Update the appropriate version constant in the adapter file
3. Mention the version bump in the commit message (e.g., `fix(integrations): improve error handling in claude-code-hook adapter (v1.0.2)`)

### CLI Error Handling Pattern

All CLI commands (except root) implement input validation using the `PreRunE` hook to distinguish between user input errors and runtime errors:

- **Input validation errors** (invalid flags, missing arguments) trigger BEFORE `cmd.SilenceUsage = true` is set, causing the command to display usage help
- **Runtime errors** (file not found, daemon not running, API failures) occur AFTER `cmd.SilenceUsage = true`, suppressing usage display

This pattern ensures users see helpful usage information only when they've made an input mistake, not when they've encountered a system error during correct command execution.

**Implementation:**
- Every command has a named `validateXxx` function assigned to `PreRunE`
- Validation functions check all user input (flags, arguments)
- Only after validation passes does the function set `cmd.SilenceUsage = true`
- The root command's `Execute()` function checks `cmd.SilenceUsage` to determine whether to show usage

**When adding new commands:**
- Always add a `PreRunE: validateCommandName` attribute
- Create a named validation function
- Validate all user-provided input before setting `cmd.SilenceUsage = true`
- Set `cmd.SilenceUsage = true` as the final step before returning nil

### Error Message Formatting

All error messages use **semicolons** (`;`) instead of colons (`:`) to separate error stanzas when wrapping errors with `%w`:

```go
// CORRECT - use semicolon
return fmt.Errorf("failed to initialize config; %w", err)

// INCORRECT - don't use colon
return fmt.Errorf("failed to initialize config: %w", err)
```

**Rationale:** Since the root command's `Execute()` function already prefixes all errors with "Error: ", using semicolons for internal error separators creates cleaner, more readable error output:

```
Error: failed to initialize config; configuration file not found
```

Instead of:
```
Error: failed to initialize config: configuration file not found
```

This pattern provides consistent punctuation with only one colon in the entire error message chain.

### CLI Output Formatting

All CLI commands use the `internal/format` package for consistent, structured output. **Never use `fmt.Printf` or `fmt.Println` for command output** - always use format package builders.

**Location:** `internal/format/` and `internal/format/formatters/`

**Philosophy:**
- Separate content from presentation
- Structured data with multiple output formats (text, JSON, YAML, XML, markdown)
- Reusable builders for common output patterns
- Consistent styling across all commands

**Available Builders:**

1. **Status** - Success/error/warning/info messages
2. **Section** - Hierarchical key-value data with subsections
3. **Table** - Tabular data with headers and alignment
4. **List** - Ordered/unordered lists with nesting
5. **Progress** - Progress bars and percentages
6. **Error** - Detailed error messages with suggestions
7. **GraphContent** - Graph index output (special case)

**When to Use Each Builder:**

- **Status** - Command completion messages, state changes, notifications
- **Section** - Configuration display, daemon status, structured information
- **Table** - File lists, statistics, version distributions
- **List** - Simple enumerations, steps, feature lists
- **Progress** - Long-running operations (currently used in tests)
- **Error** - Validation failures with field-level details
- **GraphContent** - Graph index output in read/integration commands

**Basic Usage Pattern:**

```go
import (
    "github.com/leefowlercu/agentic-memorizer/internal/format"
    _ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
)

// Helper function (usually in helpers.go or at package level)
func outputStatus(status *format.Status) error {
    formatter, err := format.GetFormatter("text")
    if err != nil {
        return fmt.Errorf("failed to get formatter; %w", err)
    }
    output, err := formatter.Format(status)
    if err != nil {
        return fmt.Errorf("failed to format status; %w", err)
    }
    fmt.Println(output)
    return nil
}

// In command RunE function
func runMyCommand(cmd *cobra.Command, args []string) error {
    // ... command logic ...

    status := format.NewStatus(format.StatusSuccess, "Operation completed successfully")
    status.AddDetail("Files processed: 42")
    status.AddDetail("Errors: 0")
    return outputStatus(status)
}
```

**Status Builder Examples:**

```go
// Success message
status := format.NewStatus(format.StatusSuccess, "Cache cleared successfully")
status.AddDetail("Run 'agentic-memorizer daemon rebuild' to regenerate")

// Error message
status := format.NewStatus(format.StatusError, "Configuration validation failed")
status.AddDetail(err.Error())

// Info message
status := format.NewStatus(format.StatusInfo, "Daemon is not running")
status.AddDetail("New configuration will be used on next daemon start")

// Running/in-progress message
status := format.NewStatus(format.StatusRunning, "Clearing 42 cache entries")

// Warning message
status := format.NewStatus(format.StatusWarning, "Cache is outdated")
status.AddDetail("Run cache clear --old-versions")
```

**Section Builder Examples:**

```go
// Simple section with key-value pairs
section := format.NewSection("Cache Status").AddDivider()
section.AddKeyValue("Total Entries", format.FormatNumber(stats.TotalEntries))
section.AddKeyValue("Total Size", format.FormatBytes(stats.TotalSize))

// Hierarchical sections
mainSection := format.NewSection("Configuration Schema")
configSection := format.NewSection("Configurable Settings").SetLevel(1)
configSection.AddKeyValue("claude.api_key", "API key for Claude")
mainSection.AddSubsection(configSection)
```

**Table Builder Examples:**

```go
// Create table with headers
table := format.NewTable([]string{"Version", "Count", "Size"})

// Add rows
for version, count := range versionCounts {
    table.AddRow([]string{
        version,
        format.FormatNumber(int64(count)),
        format.FormatBytes(sizes[version]),
    })
}

// Set alignment (left, center, right)
table.SetAlignments([]format.Alignment{
    format.AlignLeft,
    format.AlignRight,
    format.AlignRight,
})
```

**List Builder Examples:**

```go
// Unordered list
list := format.NewList(format.ListTypeUnordered)
list.AddItem("Documents: 10 files")
list.AddItem("Images: 5 files")

// Ordered list
list := format.NewList(format.ListTypeOrdered)
list.AddItem("Initialize configuration")
list.AddItem("Start daemon")
list.AddItem("Verify status")

// Nested list
mainList := format.NewList(format.ListTypeUnordered)
item := mainList.AddItem("Features")
nested := format.NewList(format.ListTypeUnordered)
nested.AddItem("Semantic search")
nested.AddItem("Graph relationships")
item.Nested = nested
```

**GraphContent for Integration Output:**

```go
// Used in read command and integration adapters
func formatGraph(index *types.GraphIndex, formatStr string) error {
    formatter, err := format.GetFormatter(formatStr)
    if err != nil {
        return fmt.Errorf("failed to get formatter; %w", err)
    }

    graphContent := format.NewGraphContent(index)
    content, err := formatter.Format(graphContent)
    if err != nil {
        return fmt.Errorf("failed to format output; %w", err)
    }

    fmt.Print(content)
    return nil
}
```

**Multiple Output Formats:**

All builders support multiple formatters:
- `text` - Plain text with symbols (default for CLI)
- `json` - Structured JSON output
- `yaml` - YAML format
- `xml` - XML format
- `markdown` - Rich markdown with emojis

Most commands use `text` formatter. JSON/YAML/XML are available via flags in some commands (e.g., `config show-schema --format json`).

**Helper Utilities:**

```go
format.FormatBytes(1024)           // "1.0 KB"
format.FormatNumber(1000000)       // "1,000,000"
format.Bold("Important")           // Bold text (if formatter supports colors)
format.Green("Success")            // Green text (if formatter supports colors)
format.Red("Error")                // Red text (if formatter supports colors)
```

**Migration from fmt.Printf:**

```go
// BEFORE (❌ Don't do this)
fmt.Printf("✓ Configuration is valid\n")
fmt.Printf("Cache cleared: %d entries\n", count)

// AFTER (✅ Do this)
status := format.NewStatus(format.StatusSuccess, "Configuration is valid")
outputStatus(status)

status := format.NewStatus(format.StatusSuccess, fmt.Sprintf("Cache cleared: %d entries", count))
outputStatus(status)
```

**When Direct fmt.Print is Acceptable:**

1. **Interactive prompts** in `cmd/initialize/` - User input/output during setup
2. **File generation** in `systemctl.go`/`launchctl.go` - Writing service files
3. **Helper functions** - Internal utilities that return formatted strings
4. **Debug output** - Temporary debugging (should be removed before commit)

**Testing with Format Package:**

```go
import (
    "testing"
    _ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
)

func TestMyCommand(t *testing.T) {
    // Format package requires formatters to be registered
    // The blank import above handles this

    // ... test code ...
}
```

**Common Patterns:**

```go
// Shared helper in subcommands package (see cmd/daemon/subcommands/helpers.go)
func outputStatus(status *format.Status) error {
    formatter, err := format.GetFormatter("text")
    if err != nil {
        return fmt.Errorf("failed to get formatter; %w", err)
    }
    output, err := formatter.Format(status)
    if err != nil {
        return fmt.Errorf("failed to format status; %w", err)
    }
    fmt.Println(output)
    return nil
}

// Use in command
status := format.NewStatus(format.StatusSuccess, "Operation completed")
return outputStatus(status)
```

**Key Files:**

- `internal/format/interface.go` - Core interfaces and types
- `internal/format/builders.go` - Builder constructors (NewStatus, NewSection, etc.)
- `internal/format/section.go` - Section builder implementation
- `internal/format/table.go` - Table builder implementation
- `internal/format/list.go` - List builder implementation
- `internal/format/status.go` - Status builder implementation
- `internal/format/formatters/text.go` - Text formatter (default)
- `internal/format/formatters/json.go` - JSON formatter
- `internal/format/formatters/xml.go` - XML formatter
- `internal/format/formatters/markdown.go` - Markdown formatter
- `internal/format/formatters/yaml.go` - YAML formatter

### Releasing

The project uses automated release tooling via Goreleaser. All release commands are in the Makefile.

**Release Workflow:**

```bash
# Bump version and prepare release (choose one)
make release-patch  # 0.10.0 → 0.10.1 (bug fixes)
make release-minor  # 0.10.0 → 0.11.0 (new features)
make release-major  # 0.10.0 → 1.0.0 (breaking changes)

# Or specify version manually
make release-prep VERSION=v0.11.0
```

**What the release command does:**

1. Validates prerequisites (clean git status, goreleaser installed, GITHUB_TOKEN set)
2. Calculates next version from `internal/version/VERSION`
3. Updates VERSION file with new version
4. Creates commit: `release: prepare vX.Y.Z`
5. Creates temporary git tag
6. Runs GoReleaser:
   - Builds multi-platform binaries (Linux/macOS, amd64/arm64)
   - Generates checksums and tar.gz archives
   - Creates dist/CHANGELOG.md from conventional commits
   - Creates draft GitHub release with binaries
7. Merges dist/CHANGELOG.md into root CHANGELOG.md
8. Amends commit to include changelog changes
9. Recreates tag on amended commit

**After successful release preparation:**

1. Review changes:
   ```bash
   git show HEAD:CHANGELOG.md
   git diff HEAD~1 CHANGELOG.md
   ```

2. Review draft release on GitHub:
   https://github.com/leefowlercu/agentic-memorizer/releases

3. If changes needed:
   - Edit CHANGELOG.md locally and/or release notes on GitHub
   - Amend and update tag:
     ```bash
     git add CHANGELOG.md && git commit --amend --no-edit
     git tag -fa vX.Y.Z -m "Agentic Memorizer vX.Y.Z"
     ```

4. When ready to publish:
   ```bash
   git push && git push --tags
   ```
   Then publish the draft release on GitHub

**Undoing a release (before pushing):**

```bash
git tag -d vX.Y.Z && git reset --hard HEAD~1
```

**Testing Goreleaser:**

```bash
# Validate configuration
make goreleaser-check

# Test local build without publishing
make goreleaser-snapshot
```

**Prerequisites:**

- GoReleaser installed: `go install github.com/goreleaser/goreleaser/v2@latest`
- GITHUB_TOKEN environment variable set with `repo` scope
- Clean git working directory
- Conventional commit messages for changelog generation

**Conventional Commits:**

Use conventional commit format for automatic changelog generation:
- `feat:` → Added section
- `fix:`, `bug:` → Fixed section
- `refactor:` → Changed section
- `docs:` → Documentation section
- `build:`, `chore:` → Build section
- `test:`, `style:` → Tests section

The release scripts filter out merge commits and release preparation commits from the changelog.
