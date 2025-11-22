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
  - [Index Management](#index-management)
  - [Integration Framework](#integration-framework)
  - [Configuration System](#configuration-system)
- [Code Organization Principles](#code-organization-principles)
  - [Subsystem Independence](#subsystem-independence)
  - [Separation of Metadata and Semantics](#separation-of-metadata-and-semantics)
  - [Content-Addressable Caching](#content-addressable-caching)
  - [Handler/Adapter Patterns](#handleradapter-patterns)
- [Testing Approach](#testing-approach)
- [Key File Locations](#key-file-locations)
- [Development Notes](#development-notes)
  - [Go Standards](#go-standards)
  - [Git Workflow](#git-workflow)
  - [API Rate Limiting](#api-rate-limiting)
  - [Configuration Hot-Reload](#configuration-hot-reload)
  - [Binary Path in Integrations](#binary-path-in-integrations)
  - [CLI Error Handling Pattern](#cli-error-handling-pattern)
  - [Error Message Formatting](#error-message-formatting)
  - [Releasing](#releasing)

## Project Overview

Agentic Memorizer is a local file memorizer for Claude Code that provides automatic awareness and understanding of files through AI-powered semantic analysis. A background daemon watches a memory directory, extracts metadata, performs semantic analysis via Claude API, and maintains a precomputed index that loads into Claude's context via SessionStart hooks.

## Development Commands

### Building and Testing

```bash
# Build the binary (development - version shows as "dev")
make build

# Build with version information from git
make build-release

# Install to ~/.local/bin (development)
make install

# Install with version information (recommended for production)
make install-release

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

# Start the daemon
./agentic-memorizer daemon start

# Check daemon status
./agentic-memorizer daemon status

# Stop the daemon
./agentic-memorizer daemon stop

# Read the precomputed index
./agentic-memorizer read

# Validate configuration
./agentic-memorizer config validate

# Hot-reload configuration (non-structural settings)
./agentic-memorizer config reload

# Start MCP server (for Claude Code integration)
./agentic-memorizer mcp start

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

1. **Metadata Extraction** (`internal/metadata/`) - Fast, deterministic extraction of file-specific metadata (word counts, dimensions, page counts) using a handler pattern with specialized extractors for 10 file type categories
2. **Semantic Analysis** (`internal/semantic/`) - AI-powered content understanding via Claude API with content-based routing (text, vision for images, document blocks for PDFs, extraction for Office files)
3. **Caching** (`internal/cache/`) - Content-hash-based storage of analysis results that achieves >95% hit rates, dramatically reducing API costs

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

### Semantic Search

The search engine (`internal/search/`) provides advanced file discovery for MCP tools:
- **Fuzzy Filename Matching** - Configurable similarity threshold (default 0.3) for approximate filename searches
- **Tag-Based Search** - Partial matching against indexed tags
- **Topic-Based Search** - Search across key topics identified during semantic analysis
- **Summary Text Search** - Content-based discovery via summary text matching
- **Relevance Scoring** - Multi-signal ranking algorithm combining match quality across dimensions
- **Category Filtering** - Optional filtering by file category (documents, images, code, etc.)
- **Result Ranking** - Returns top N results sorted by relevance score

### Index Management

The Index Manager (`internal/index/`) maintains the precomputed index with:
- **Thread-safe operations** using sync.RWMutex for concurrent reads
- **Atomic writes** via temp file + rename pattern to prevent corruption
- **Two-level versioning**: schema version (index format) and daemon version (application release)

Index structure:
```
Index → []IndexEntry
IndexEntry → FileMetadata + SemanticAnalysis + Error
FileMetadata → path, type, category, size, hash, type-specific fields
SemanticAnalysis → summary, tags, key_topics, document_type, confidence
```

### Integration Framework

The Integration Registry (`internal/integrations/`) provides framework-agnostic integration through:

- **Adapter Pattern** - Common Integration interface with specialized (Claude Code) and generic (Continue, Cline, Aider, Cursor) implementations
- **Registry Pattern** - Thread-safe singleton managing adapter registration and lookup
- **Output Processors** - Independent formatters (XML, Markdown, JSON) separate from integration wrapping
- **Auto-registration** - Adapters register via init() functions

Claude Code integration uses two methods:
1. **SessionStart hooks** (`claude-code-hook`) - Inject full index at session start via hooks with matchers (startup, resume, clear, compact)
2. **MCP server** (`claude-code-mcp`) - Provide on-demand tools for semantic search and metadata retrieval

Both wrap output appropriately: SessionStart hooks use JSON envelope with systemMessage and additionalContext fields; MCP uses JSON-RPC 2.0 protocol responses.

### MCP Server

The MCP server (`internal/mcp/`) implements Model Context Protocol for tool-based integration:
- **Protocol Layer** (`internal/mcp/protocol/`) - JSON-RPC 2.0 message types and handlers
- **Transport Layer** (`internal/mcp/transport/`) - Stdio transport for MCP communication
- **Server Lifecycle** - Initialize, initialized, shutdown sequence following MCP spec
- **Tool Integration** - Exposes three tools integrated with index and search:
  - `search_files` - Semantic search with fuzzy matching, tag/topic/summary search
  - `get_file_metadata` - Complete file metadata and semantic analysis retrieval
  - `list_recent_files` - Recently modified files within configurable time window
- **Logging** - Separate log file and level control via `mcp.log_file` and `mcp.log_level` config

### Configuration System

The Config Manager (`internal/config/`) implements layered configuration with precedence: defaults → YAML file → environment variables (MEMORIZER_* prefix). Supports hot-reload via `config reload` command for non-structural settings.

Key configuration sections:
- `claude` - API credentials, model selection, vision toggle, timeouts
- `analysis` - Enable flag, file size limits, skip patterns, cache directory
- `daemon` - Workers, debounce timing, rate limits, rebuild intervals, health check port
- `integrations` - Per-framework settings with type, output format, custom settings
- `mcp` - MCP server log file path, log level (separate from daemon logging)

Settings requiring daemon restart: `memory_root`, `analysis.cache_dir`, `daemon.log_file`, `mcp.log_file`.

Validation uses error accumulation pattern (collects all errors before failing) with structured ValidationError providing field, rule, message, suggestion, and value.

## Code Organization Principles

### Subsystem Independence

Each major subsystem (`internal/daemon/`, `internal/metadata/`, `internal/semantic/`, `internal/cache/`, `internal/index/`, `internal/config/`, `internal/integrations/`, `internal/watcher/`, `internal/walker/`, `internal/mcp/`, `internal/search/`, `internal/version/`) operates independently with clean boundaries. The daemon orchestrates but doesn't tightly couple to implementation details.

### Separation of Metadata and Semantics

Metadata extraction is fast and deterministic; semantic analysis is slow and AI-powered. This separation enables efficient caching (metadata extraction always happens to compute hashes; semantic analysis is cached) and parallel processing (metadata is CPU-bound; semantic analysis is I/O-bound).

### Content-Addressable Caching

Cache keys are SHA-256 hashes of file content (not paths), enabling cache hits across file renames/moves and automatic invalidation on content changes. No explicit cache invalidation logic needed.

### Handler/Adapter Patterns

Both metadata extraction and integration systems use handler/adapter patterns with registries. New file types or frameworks can be added by implementing interfaces and registering—no changes to core logic required.

## Testing Approach

The project uses Go's standard testing package with table-based tests where appropriate. Key testing patterns:

- **Metadata extractors** - Each handler has dedicated test file with test data in `testdata/`
- **Integration adapters** - Test registration, detection, and output formatting
- **Worker pool** - Tests parallel processing, cache integration, priority ordering
- **Configuration** - Tests validation rules, error accumulation, path safety
- **MCP server** - Tests JSON-RPC 2.0 protocol implementation, tool handlers
- **Semantic search** - Tests fuzzy matching, relevance scoring, tag/topic filtering

When writing tests:
- Use `t.Run()` for subtests within table-driven tests
- Place test data files in `testdata/` directory
- Mock external dependencies (Claude API, file system) where appropriate
- Test error paths explicitly (validation failures, missing files, malformed data)

## Key File Locations

**CLI Commands:**
- Root command: `cmd/root.go`
- Command packages: `cmd/{initialize,daemon,integrations,config,read,mcp}/`
- Daemon commands: `cmd/daemon/daemon.go` (parent) + `cmd/daemon/subcommands/` (6 subcommands)
- Integration commands: `cmd/integrations/integrations.go` (parent) + `cmd/integrations/subcommands/` (6 subcommands + helpers)
- Config commands: `cmd/config/config.go` (parent) + `cmd/config/subcommands/` (2 subcommands)

**Core Subsystems:**
- Main subsystems: `internal/{daemon,metadata,semantic,cache,index,config,integrations,watcher,walker,mcp,search,version}/`
- Type definitions: `pkg/types/types.go`

**Documentation & Resources:**
- Subsystem documentation: `docs/subsystems/` - comprehensive technical documentation
- Test data: `testdata/` - files for testing metadata extraction

## Development Notes

### Go Standards

- Follow the Go Style Guide (https://google.github.io/styleguide/go/guide)
- Use `log/slog` for logging, not fmt.Printf
- Use `any` instead of `interface{}` where possible
- Generated code must pass all tests before being considered complete

### Git Workflow

- Commit messages use conventional commit format, lowercase, single line
- Do not mention Claude Code coauthoring in commit messages
- Current version: v0.9.0 (semantic versioning)

### API Rate Limiting

The daemon implements token bucket rate limiting (default 20 calls/minute) to respect Claude API quotas. Workers call `rateLimiter.Wait(ctx)` before semantic analysis. Adjust `daemon.rate_limit_per_min` in config as needed for your API tier.

### Configuration Hot-Reload

The daemon supports hot-reloading non-structural configuration changes via `config reload` command without requiring a restart. Use the daemon's health check endpoint to trigger reloads. Settings requiring daemon restart: `memory_root`, `analysis.cache_dir`, `daemon.log_file`, `mcp.log_file`.

### Binary Path in Integrations

Integration setup commands automatically detect and configure the correct binary path. When modifying integration setup logic, ensure binary path detection works for both `go install` (in $GOPATH/bin) and `make install` (in ~/.local/bin) installations.

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
- GPG_FINGERPRINT environment variable set (get with `gpg --list-secret-keys --keyid-format=long`)
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
