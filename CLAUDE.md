# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Table of Contents

- [Project Overview](#project-overview)
- [Development Commands](#development-commands)
- [High-Level Architecture](#high-level-architecture)
- [Code Organization Principles](#code-organization-principles)
- [Testing Approach](#testing-approach)
- [Key File Locations](#key-file-locations)
- [Development Notes](#development-notes)

## Project Overview

Agentic Memorizer is a local file memorizer for Claude Code that provides automatic awareness and understanding of files through AI-powered semantic analysis, plus user-defined facts that inject persistent context into every conversation. A background daemon watches a memory directory, extracts metadata, performs semantic analysis via multiple AI providers (Claude, OpenAI, Gemini), and maintains a knowledge graph in FalkorDB that integrates with Claude via hooks (SessionStart for files, UserPromptSubmit for facts) and MCP tools.

## Development Commands

### Building and Testing

```bash
# Build, install, and test
make build                    # Build binary with git version info
make install                  # Install to ~/.local/bin
make test                     # Run all tests
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
make deps                     # Download and tidy dependencies
```

### Daemon Development

```bash
# Quick daemon workflow
make daemon-start             # Build and start daemon
make daemon-stop              # Stop daemon
make daemon-status            # Check daemon status
make daemon-logs              # Tail daemon logs
make validate-config          # Validate configuration
```

### Running the Application

```bash
# Initialization
./memorizer initialize                        # Interactive setup
./memorizer initialize --setup-integrations   # Automated setup

# Graph database
./memorizer graph start                       # Start FalkorDB (required before daemon)
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

# Service manager integration (Linux/macOS)
./memorizer daemon systemctl                  # Generate systemd unit file
./memorizer daemon launchctl                  # Generate launchd plist file
```

### Service Manager Integration

The daemon runs in foreground mode, delegating process supervision to external service managers (systemd, launchd). This follows modern Go best practices and avoids self-daemonization anti-patterns.

**Why External Process Managers:**
- Idiomatic Go (avoids fork() complexity with goroutines)
- Production-grade supervision and automatic restarts
- Native logging integration (journald, Console.app)
- Platform portability without fork logic

**Implementation:**
- systemd: Type=notify integration via `daemon.SdNotify()` after health server starts
- launchd: KeepAlive with SuccessfulExit=false for crash recovery
- Generate service files on-demand with `daemon systemctl`/`daemon launchctl`

## High-Level Architecture

### Three-Phase Processing Pipeline

Files are processed through three distinct phases:

1. **Metadata Extraction** (`internal/metadata/`) - Fast, deterministic extraction using specialized handlers for 9 file type categories (documents, images, code, data, media, archives, fonts, models, other)
2. **Semantic Analysis** (`internal/semantic/`) - AI-powered content understanding via provider abstraction supporting Claude, OpenAI, and Gemini with content-based routing (text, vision for images, document blocks for PDFs, extraction for Office files)
3. **Knowledge Graph Storage** (`internal/graph/`) - FalkorDB stores files, tags, topics, entities, and relationships for semantic search

### Semantic Analysis Provider Architecture

The semantic analysis subsystem uses a provider abstraction pattern for multi-provider support:

**Provider Interface** (`internal/semantic/provider.go`):
```go
type Provider interface {
    Analyze(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error)
    Name() string           // "claude", "openai", "gemini"
    Model() string          // e.g., "claude-sonnet-4-5-20250929"
    SupportsVision() bool   // Image analysis capability
    SupportsDocuments() bool // Native PDF/document blocks
}
```

**Provider Registry** (`internal/semantic/registry.go`):
- Thread-safe singleton pattern (like integrations)
- Providers register via `init()` functions
- Factory pattern for provider instantiation

**Provider Implementations** (`internal/semantic/providers/`):
- **Claude** (`providers/claude/`) - Anthropic API with vision, PDF document blocks, PPTX/DOCX text extraction
- **OpenAI** (`providers/openai/`) - OpenAI API with GPT-4o/5.x vision support
- **Gemini** (`providers/gemini/`) - Google Gemini API with native multimodal support

**Content Routing by Provider:**
| Content Type | Claude | OpenAI | Gemini |
|-------------|--------|--------|--------|
| Text files | ✓ | ✓ | ✓ |
| Images (vision) | ✓ | ✓ (GPT-4o/5.x) | ✓ |
| PDFs | Document blocks | Text extraction | Native multimodal |
| Office docs | Text extraction | Metadata only | Metadata only |

**Hot-Reload Support:**
- Provider can be changed at runtime via `config reload`
- Atomic value replacement in daemon
- Cache isolation by provider (separate subdirectories)

**Cache Provider Isolation:**
```
~/.memorizer/cache/summaries/
├── claude/   # Claude provider cache
├── openai/   # OpenAI provider cache
└── gemini/   # Gemini provider cache
```

### Background Daemon Architecture

The daemon (`internal/daemon/`) orchestrates file discovery through:

- **Walker** (`internal/walker/`) - Full directory scans during rebuilds with two-tier filtering (directory pruning, file filtering)
- **File Watcher** (`internal/watcher/`) - Real-time monitoring with fsnotify and debouncing (default 500ms)

Jobs flow through a worker pool with priority calculation (recent files first):
1. Extract metadata and compute SHA-256 content hash
2. Check cache (if hit, skip analysis)
3. On cache miss: acquire rate limiter token, perform semantic analysis, store result
4. Return entry with metadata + semantic analysis
5. Store in FalkorDB knowledge graph with relationships

### FalkorDB Knowledge Graph

The graph subsystem (`internal/graph/`) provides persistent storage and relationship-based queries.

**Architecture:**
- **FalkorDB Backend** - Redis-compatible graph database in Docker
- **Manager** (`manager.go`) - Connection pooling, health checks, graceful degradation
- **Queries** (`queries.go`) - Cypher query execution for CRUD operations
- **Schema** (`schema.go`) - Node/edge type definitions and constraint management
- **Exporter** (`exporter.go`) - Converts graph storage to FileIndex output format

**Graph Schema:**

Nodes: File, Tag, Topic, Entity, Category, Fact

Edges: HAS_TAG, COVERS_TOPIC, MENTIONS, IN_CATEGORY

**Facts Storage** (`internal/graph/facts.go`):
- CRUD operations for user-defined facts
- Up to 50 facts, 10-500 characters each
- Facts injected via UserPromptSubmit (Claude) / BeforeAgent (Gemini) hooks

**Graph Commands:**
- `graph start/stop/status` - Manage FalkorDB Docker container
- `daemon rebuild [--force]` - Rebuild graph (--force clears existing data)

**HTTP API** (`internal/daemon/api/`):
- `GET /health` - Health with graph metrics
- `POST /api/v1/search` - Graph-powered semantic search
- `GET /api/v1/files/{path}` - File metadata with connections
- `GET /api/v1/files/recent` - Recently modified files
- `GET /api/v1/files/related` - Related files by shared tags/topics
- `GET /api/v1/entities/search` - Files mentioning an entity
- `POST /api/v1/rebuild` - Trigger rebuild
- `GET /sse` - Server-Sent Events for real-time updates

**HTTP API Timeouts:**
- SSE endpoint (`/sse`): No timeout (long-lived streaming connection with 30s keepalive)
- Health endpoint (`/health`): No timeout (monitoring endpoint, always fast)
- API endpoints (`/api/v1/*`): 60s timeout via `http.TimeoutHandler` middleware

**Data Persistence:**
FalkorDB stores data at `~/.memorizer/falkordb/`. To clear: `rm -rf ~/.memorizer/falkordb/* && docker restart memorizer-falkordb`

**Graceful Degradation:**
When FalkorDB is unavailable, daemon logs warnings but continues. Graph queries return empty results. Legacy in-memory search (`internal/search/`) provides fallback.

### Integration Framework

The Integration Registry (`internal/integrations/`) provides framework-agnostic integration:

- **Adapter Pattern** - Common Integration interface with specialized implementations
- **Registry Pattern** - Thread-safe singleton managing adapter registration and lookup
- **Output Processors** - Independent formatters (XML, Markdown, JSON)
- **Auto-registration** - Adapters register via init() functions

**Active Integrations:**
- **Claude Code** - Dual hooks (`claude-code-hook`) + MCP server (`claude-code-mcp`)
- **Gemini CLI** - Dual hooks (`gemini-cli-hook`) + MCP server (`gemini-cli-mcp`)
- **Codex CLI** - MCP server only (`codex-cli-mcp`)

**Dual-Hook Architecture:**
- SessionStart hooks inject file index at session start
- UserPromptSubmit (Claude) / BeforeAgent (Gemini) hooks inject user facts before each prompt
- MCP servers provide on-demand tools (search, metadata, related files, entity search)

**Configuration Formats:**
- Claude Code & Gemini CLI: JSON configuration files
- Codex CLI: TOML configuration (`github.com/pelletier/go-toml/v2`)

### MCP Server

The MCP server (`internal/mcp/`) implements Model Context Protocol:
- **Protocol Layer** (`protocol/`) - JSON-RPC 2.0 message types and handlers
- **Transport Layer** (`transport/`) - Stdio transport
- **Server Lifecycle** - Initialize, initialized, shutdown sequence
- **Daemon Integration** - Connects to daemon HTTP API
- **Tools** - Five graph-powered tools: search_files, get_file_metadata, list_recent_files, get_related_files, search_entities
- **Logging** - Separate log file and level control via `mcp.log_file` and `mcp.log_level`

### Configuration System

The Config Manager (`internal/config/`) implements layered configuration with precedence: defaults → YAML file → environment variables (MEMORIZER_* prefix). Supports hot-reload via `config reload` for non-structural settings.

**Configuration Tiers:**
- **Minimal** - Shown in initialized config (core settings users typically change)
- **Advanced** - Documented but not shown by default (sensible defaults)
- **Hardcoded** - Internal conventions not configurable

Use `config show-schema` to discover all settings.

**Key Configuration Sections:**
- `semantic` - Provider selection, API credentials, model, timeout, vision, rate limits
- `daemon` - Workers, debounce, rebuild intervals, health port
- `mcp` - Log file, log level, daemon connectivity
- `graph` - FalkorDB host, port, database, password
- `embeddings` - API key, provider, model, dimensions

**Semantic Configuration (`semantic.*`):**
```yaml
semantic:
  provider: claude           # claude, openai, gemini
  api_key: ""                # Or use env var (ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY)
  model: claude-sonnet-4-5-20250929
  max_tokens: 4096
  timeout: 30                # Seconds (5-300)
  enable_vision: true
  max_file_size: 10485760    # 10MB
  skip_extensions: [...]
  skip_files: [...]
  cache_dir: ~/.memorizer/cache
  rate_limit_per_min: 20     # Provider-specific defaults: Claude=20, OpenAI=60, Gemini=100
```

**Environment Variables for API Keys:**
- Claude: `ANTHROPIC_API_KEY`
- OpenAI: `OPENAI_API_KEY`
- Gemini: `GOOGLE_API_KEY`

**Settings Requiring Daemon Restart:** `memory_root`, `semantic.cache_dir`, `daemon.log_file`

**Hot-reloadable Settings:** `semantic.provider`, `semantic.api_key`, `semantic.model`, `semantic.max_tokens`, `semantic.timeout`, `semantic.enable_vision`, `semantic.rate_limit_per_min`, `daemon.workers`, `daemon.debounce_ms`, `daemon.log_level`, `daemon.http_port`

## Code Organization Principles

### Subsystem Independence

Each major subsystem operates independently with clean boundaries:
- `internal/daemon/` - Daemon orchestration and worker pool
- `internal/metadata/` - Metadata extraction handlers
- `internal/semantic/` - Multi-provider semantic analysis (Claude, OpenAI, Gemini)
- `internal/semantic/providers/` - Provider implementations
- `internal/cache/` - Content-addressable caching with provider isolation
- `internal/graph/` - FalkorDB graph storage and queries
- `internal/config/` - Configuration management
- `internal/integrations/` - Integration adapters
- `internal/watcher/` - Real-time file monitoring
- `internal/walker/` - Directory scanning
- `internal/mcp/` - MCP server protocol and transport
- `internal/search/` - Fallback in-memory search
- `internal/format/` - Structured output formatting
- `internal/embeddings/` - Embeddings provider
- `internal/servicemanager/` - systemd/launchd integration
- `internal/tui/` - Terminal UI for initialization
- `internal/version/` - Version management

### Separation of Metadata and Semantics

Metadata extraction is fast and deterministic; semantic analysis is slow and AI-powered. This separation enables efficient caching (metadata extraction always happens to compute hashes; semantic analysis is cached) and parallel processing.

### Content-Addressable Caching

Cache keys are SHA-256 hashes of file content (not paths), enabling cache hits across file renames/moves and automatic invalidation on content changes.

### Cache Versioning

The semantic analysis cache uses three-tier versioning (`internal/cache/version.go`):

- **SchemaVersion** - CachedAnalysis struct format (bump when adding/removing/renaming fields)
- **MetadataVersion** - Metadata extraction logic (bump when metadata output changes)
- **SemanticVersion** - Semantic analysis logic (bump when prompts/analysis changes)

**Cache Key Format:** `{content-hash[:16]}-v{schema}-{metadata}-{semantic}.json`

**Staleness Detection:**
- Schema mismatch = always stale
- Metadata/semantic version behind = stale
- Version ahead (future) = not stale (forward compatible)

**When Modifying Cache-Related Code:**
1. Identify scope (struct format, metadata, or semantic)
2. Bump appropriate version in `internal/cache/version.go`
3. Mention cache version bump in commit message

### Handler/Adapter Patterns

Both metadata extraction and integration systems use handler/adapter patterns with registries. New file types or frameworks can be added by implementing interfaces and registering—no changes to core logic required.

## Testing Approach

The project uses Go's standard testing package with table-based tests.

**Unit Testing Patterns:**
- Metadata extractors: Dedicated test files with test data in `testdata/`
- Integration adapters: Test registration, detection, output formatting
- Worker pool: Test parallel processing, cache integration, priority ordering
- Configuration: Test validation rules, error accumulation, path safety
- MCP server: Test JSON-RPC 2.0 protocol, tool handlers

**End-to-End Testing:**
Comprehensive E2E tests (`e2e/tests/`) cover complete workflows:
- Test harness (`e2e/harness/`) provides isolated environments, daemon management, HTTP/MCP/graph clients
- 18 test files covering CLI, daemon lifecycle, filesystem, HTTP API, SSE, config, graph, integrations
- Build tags: `//go:build e2e` separates from unit tests
- Docker integration for realistic FalkorDB testing
- Automatic volume cleanup after tests

**E2E Commands:**
```bash
make test-e2e                 # All E2E tests with volume cleanup
make test-e2e-quick           # Quick smoke tests
cd e2e && make test-cli       # Specific test suite
```

**Testing Guidelines:**
- Use `t.Run()` for subtests in table-driven tests
- Place test data in `testdata/` directory
- Mock external dependencies (Claude API, filesystem)
- Test error paths explicitly

## Key File Locations

**CLI Commands:**
- Root: `cmd/root.go`
- Commands: `cmd/{initialize,daemon,integrations,config,read,remember,forget,mcp,graph,cache}/`
- Daemon subcommands (8): `cmd/daemon/subcommands/` - start, stop, status, restart, logs, rebuild, systemctl, launchctl
- Graph subcommands (3): `cmd/graph/subcommands/` - start, stop, status
- Cache subcommands (2): `cmd/cache/subcommands/` - status, clear
- Read subcommands (2): `cmd/read/subcommands/` - files, facts
- Remember subcommands (1): `cmd/remember/subcommands/` - fact
- Forget subcommands (1): `cmd/forget/subcommands/` - fact
- Integration subcommands (5): `cmd/integrations/subcommands/` - detect, list, setup, remove, health
- Config subcommands (3): `cmd/config/subcommands/` - validate, reload, show-schema

**Core Subsystems:**
- Main: `internal/{daemon,metadata,semantic,cache,graph,config,integrations,watcher,walker,mcp,search,format,embeddings,servicemanager,tui,version}/`
- Semantic Providers: `internal/semantic/providers/{claude,openai,gemini}/` - Provider implementations
- Graph: `internal/graph/` - FalkorDB client, queries, schema, exporter, facts
- Daemon API: `internal/daemon/api/` - HTTP server, SSE hub, handlers
- Types: `pkg/types/` - Core type definitions (FileIndex, FileEntry, Fact, FactsIndex)

**Documentation:**
- Subsystems: `docs/subsystems/` - Technical documentation
- Test data: `testdata/` - Metadata extraction test files
- Docker: `docker-compose.yml` - FalkorDB container

## Development Notes

### Go Standards

- Follow Go Style Guide (https://google.github.io/styleguide/go/guide)
- Use `log/slog` for logging
- Use `any` instead of `interface{}`
- Code must pass all tests before completion

### Logging Standards

All log messages must be entirely lowercase with no exceptions. This follows Go error string conventions and ensures consistency across the codebase.

**Rules:**
- Log messages MUST be entirely lowercase (no exceptions)
- Follow Go error string conventions: lowercase messages, structured data in fields
- Use structured logging with key-value pairs via `log/slog`
- Put proper nouns, acronyms, and important data in structured fields, not in the message string
- Avoid string formatting in log messages; use structured fields instead

**Examples:**
```go
// CORRECT - lowercase messages with structured fields
logger.Info("starting daemon", "version", v)
logger.Error("failed to connect", "host", host, "error", err)
logger.Info("connection established", "database", "FalkorDB")
logger.Info("server started", "protocol", "HTTP", "port", 8080)
logger.Info("sse client connected", "total_clients", count)

// INCORRECT - uppercase messages
logger.Info("Starting daemon")
logger.Error("Failed to connect")
logger.Info("FalkorDB connection established")  // Put "FalkorDB" in field
logger.Info("HTTP server started")              // Put "HTTP" in field
logger.Info("SSE client connected")             // Put "SSE" in field
```

**Rationale:**
- Simpler rule (no judgment calls needed about what counts as a proper noun)
- Easier to enforce programmatically with linters
- Messages are context; structured fields carry important data
- Maximum consistency across entire codebase
- Aligns perfectly with Go error string conventions

### Git Workflow

- Commit messages: conventional commit format, lowercase, single line
- No Claude Code coauthoring mentions
- Current version: v0.13.0

### API Rate Limiting

Token bucket rate limiting respects provider API quotas. Workers call `rateLimiter.Wait(ctx)` before semantic analysis.

**Provider-Specific Defaults:**
- Claude: 20 calls/minute (conservative)
- OpenAI: 60 calls/minute (GPT-4o tier 1: 500 RPM)
- Gemini: 100 calls/minute (paid tier: 1000-2000 RPM)

Adjust via `semantic.rate_limit_per_min` in config. Rate limit is hot-reloadable.

### Configuration Hot-Reload

Hot-reload non-structural config changes via `config reload` without daemon restart. Command sends SIGHUP to daemon, which detects changes and updates components.

**Note:** Daemon and MCP server are separate processes. Hot-reload only affects daemon. MCP settings require client disconnect/reconnect.

### Binary Path in Integrations

Integration setup commands auto-detect binary path. Ensure detection works for both `go install` ($GOPATH/bin) and `make install` (~/.local/bin) installations.

### Integration Adapter Versioning

Adapters use semantic versioning (MAJOR.MINOR.PATCH). Version constants in adapter files track changes. Bump version appropriately when modifying adapters.

**When to Bump:**
- PATCH: Bug fixes, refactoring, docs
- MINOR: New features, backward-compatible changes
- MAJOR: Breaking changes affecting user configuration

### CLI Error Handling Pattern

All CLI commands (except root) implement `PreRunE` hook for input validation to distinguish user input errors from runtime errors.

**Pattern:**
- Input validation errors (before `cmd.SilenceUsage = true`) → show usage
- Runtime errors (after `cmd.SilenceUsage = true`) → suppress usage

**Implementation:**
- Every command has named `validateXxx` function in `PreRunE`
- Validate all input before setting `cmd.SilenceUsage = true`
- Root command's `Execute()` checks `cmd.SilenceUsage` to determine usage display

### Error Message Formatting

Use semicolons (`;`) instead of colons (`:`) when wrapping errors with `%w`:

```go
// CORRECT
return fmt.Errorf("failed to initialize config; %w", err)

// INCORRECT
return fmt.Errorf("failed to initialize config: %w", err)
```

**Rationale:** Root command prefixes errors with "Error: ". Using semicolons creates cleaner output: `Error: failed to initialize config; configuration file not found`

### CLI Output Formatting

All CLI commands use `internal/format` package for consistent, structured output. **Never use `fmt.Printf` for command output** - always use format builders.

**Available Builders:**
- **Status** - Success/error/warning/info messages
- **Section** - Hierarchical key-value data with subsections
- **Table** - Tabular data with headers and alignment
- **List** - Ordered/unordered lists with nesting
- **Progress** - Progress bars and percentages
- **Error** - Detailed error messages with suggestions
- **FilesContent** - File index output

**Usage Pattern:**
```go
import (
    "github.com/leefowlercu/agentic-memorizer/internal/format"
    _ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters"
)

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

// In command
status := format.NewStatus(format.StatusSuccess, "Operation completed")
return outputStatus(status)
```

**Output Formats:** text (default), json, yaml, xml, markdown

**When Direct fmt.Print is Acceptable:**
- Interactive prompts in `cmd/initialize/`
- Service file generation in `systemctl.go`/`launchctl.go`
- Internal helper functions returning formatted strings

### Releasing

The project uses automated release tooling via Goreleaser.

**Release Commands:**
```bash
make release-patch            # 0.10.0 → 0.10.1 (bug fixes)
make release-minor            # 0.10.0 → 0.11.0 (new features)
make release-major            # 0.10.0 → 1.0.0 (breaking changes)
make release-prep VERSION=v0.11.0  # Specify version manually
```

**Release Process:**
1. Validates prerequisites (clean git, goreleaser installed, GITHUB_TOKEN set)
2. Updates VERSION file
3. Creates commit: `release: prepare vX.Y.Z`
4. Creates git tag
5. Runs GoReleaser (builds binaries, generates changelog, creates draft release)
6. Merges changelog into root CHANGELOG.md
7. Amends commit and recreates tag

**After Release:**
1. Review: `git show HEAD:CHANGELOG.md`
2. Review draft on GitHub
3. Edit if needed: `git add CHANGELOG.md && git commit --amend --no-edit && git tag -fa vX.Y.Z`
4. Publish: `git push && git push --tags` + publish draft release

**Undo (before push):** `git tag -d vX.Y.Z && git reset --hard HEAD~1`

**Testing:** `make goreleaser-check` (validate config), `make goreleaser-snapshot` (test build)

**Prerequisites:**
- GoReleaser: `go install github.com/goreleaser/goreleaser/v2@latest`
- GITHUB_TOKEN with `repo` scope
- Clean git working directory
- Conventional commit messages for changelog generation
