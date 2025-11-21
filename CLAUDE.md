# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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
./agentic-memorizer initialize --setup-integrations --with-daemon

# Start the background daemon
./agentic-memorizer daemon start

# Check daemon status
./agentic-memorizer daemon status

# Stop the daemon
./agentic-memorizer daemon stop

# Read the precomputed index
./agentic-memorizer read

# Validate configuration
./agentic-memorizer config validate
```

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

Claude Code integration uses SessionStart hooks with matchers (startup, resume, clear, compact) that wrap XML output in JSON envelope with systemMessage and additionalContext fields.

### Configuration System

The Config Manager (`internal/config/`) implements layered configuration with precedence: defaults → YAML file → environment variables (MEMORIZER_* prefix).

Key configuration sections:
- `claude` - API credentials, model selection, vision toggle, timeouts
- `analysis` - Enable flag, file size limits, skip patterns, cache directory
- `daemon` - Workers, debounce timing, rate limits, rebuild intervals, health check port
- `integrations` - Per-framework settings with type, output format, custom settings

Validation uses error accumulation pattern (collects all errors before failing) with structured ValidationError providing field, rule, message, suggestion, and value.

## Code Organization Principles

### Subsystem Independence

Each major subsystem (`internal/daemon/`, `internal/metadata/`, `internal/semantic/`, `internal/cache/`, `internal/index/`, `internal/config/`, `internal/integrations/`) operates independently with clean boundaries. The daemon orchestrates but doesn't tightly couple to implementation details.

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

When writing tests:
- Use `t.Run()` for subtests within table-driven tests
- Place test data files in `testdata/` directory
- Mock external dependencies (Claude API, file system) where appropriate
- Test error paths explicitly (validation failures, missing files, malformed data)

## Key File Locations

**CLI Commands:**
- Root command: `cmd/root.go`
- Command packages: `cmd/{initialize,daemon,integrations,config,read}/`
- Daemon commands: `cmd/daemon/daemon.go` (parent) + `cmd/daemon/subcommands/` (6 subcommands)
- Integration commands: `cmd/integrations/integrations.go` (parent) + `cmd/integrations/subcommands/` (6 subcommands + helpers)
- Config commands: `cmd/config/config.go` (parent) + `cmd/config/subcommands/` (1 subcommand)

**Core Subsystems:**
- Main subsystems: `internal/{daemon,metadata,semantic,cache,index,config,integrations,watcher,walker}/`
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
- Current version: v0.6.0 (semantic versioning)

### API Rate Limiting

The daemon implements token bucket rate limiting (default 20 calls/minute) to respect Claude API quotas. Workers call `rateLimiter.Wait(ctx)` before semantic analysis. Adjust `daemon.rate_limit_per_min` in config as needed for your API tier.

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
