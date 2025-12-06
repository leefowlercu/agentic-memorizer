# E2E Tests Subsystem

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
  - [Isolation and Reproducibility](#isolation-and-reproducibility)
  - [Test Organization](#test-organization)
  - [Harness-Based Architecture](#harness-based-architecture)
- [Key Components](#key-components)
  - [Test Harness Framework](#test-harness-framework)
  - [Test Fixtures](#test-fixtures)
  - [Test Suites](#test-suites)
  - [Cleanup Infrastructure](#cleanup-infrastructure)
- [Integration Points](#integration-points)
  - [Daemon Subsystem](#daemon-subsystem)
  - [Configuration Subsystem](#configuration-subsystem)
  - [FalkorDB Graph Database](#falkordb-graph-database)
  - [Integration Framework](#integration-framework)
  - [Walker and Watcher Subsystems](#walker-and-watcher-subsystems)
  - [Output Processors](#output-processors)
- [Test Categories](#test-categories)
  - [CLI Command Tests](#cli-command-tests)
  - [Daemon Lifecycle Tests](#daemon-lifecycle-tests)
  - [File System Integration Tests](#file-system-integration-tests)
  - [HTTP API Tests](#http-api-tests)
  - [SSE Notification Tests](#sse-notification-tests)
  - [Configuration Tests](#configuration-tests)
  - [Graph Integration Tests](#graph-integration-tests)
  - [Integration Framework Tests](#integration-framework-tests)
  - [Metadata and Cache Tests](#metadata-and-cache-tests)
  - [Output Format Tests](#output-format-tests)
  - [Error Handling Tests](#error-handling-tests)
  - [Walker Configuration Tests](#walker-configuration-tests)
- [Execution Model](#execution-model)
  - [Build Tags](#build-tags)
  - [Test Execution Lifecycle](#test-execution-lifecycle)
  - [Timeout Management](#timeout-management)
- [Glossary](#glossary)

---

## Overview

The E2E Tests subsystem provides comprehensive end-to-end testing for Agentic Memorizer. Unlike unit tests that verify individual components in isolation, E2E tests validate complete workflows by executing the actual binary, spawning real daemon processes, interacting with FalkorDB containers, and exercising the full application stack.

The subsystem ensures that all major features work correctly when integrated together, including:
- CLI command execution and output validation
- Daemon lifecycle management and health monitoring
- File system watching and processing pipelines
- HTTP API endpoints and Server-Sent Events
- Configuration validation and hot-reload functionality
- Knowledge graph operations and semantic search
- Integration framework adapters and output processors
- Error handling and graceful degradation

E2E tests serve as acceptance tests for release validation, regression detection in CI/CD pipelines, and confidence checks during development.

---

## Design Principles

### Isolation and Reproducibility

Each test operates in complete isolation using temporary directories for application state, ensuring no interference between tests and no dependency on external state. The test harness creates fresh environments with unique app directories, daemon instances, and configuration files for every test execution.

Tests are fully reproducible across different machines and environments through consistent setup, deterministic test data, and proper cleanup of all resources. The harness tracks all spawned processes and temporary directories, guaranteeing cleanup even when tests fail or panic.

### Test Organization

Test suites are organized by functional area (CLI, daemon, filesystem, HTTP API, graph, configuration, integrations) with clear naming conventions that make it easy to identify test scope. Each test file focuses on a specific subsystem or feature area, following Go's standard testing conventions with descriptive test names using underscore notation.

Table-driven tests are used where appropriate to cover multiple scenarios with the same test logic, reducing duplication and making it easy to add new test cases.

### Harness-Based Architecture

The test harness provides reusable infrastructure for common operations like spawning daemons, creating test files, waiting for health checks, and cleaning up resources. This abstraction layer separates test logic from environment management, making tests more readable and maintainable.

The harness handles the complexity of process management, temporary directory creation, daemon communication, and graph database interaction, allowing test authors to focus on validating behavior rather than managing infrastructure.

---

## Key Components

### Test Harness Framework

The harness framework (`e2e/harness/`) provides the foundation for all E2E tests through five specialized components:

- **E2EHarness** (`harness.go`): Core orchestration managing test environments with isolated app directories, unique graph names for test isolation, and environment variable configuration. Provides daemon lifecycle methods (start, stop, health polling), binary execution with captured output, memory file creation helpers, and automatic cleanup registration.

- **HTTP Client** (`http_client.go`): Type-safe HTTP client providing methods for all daemon API endpoints including health checks, semantic search, file metadata retrieval, recent files queries, related file discovery, entity search, rebuild triggers, and full index retrieval. Uses 5-second request timeouts and supports dynamic port configuration.

- **MCP Client** (`mcp_client.go`): Full JSON-RPC 2.0 MCP protocol implementation spawning MCP server as subprocess via stdio transport. Provides initialize/shutdown lifecycle, resource listing/reading, tool listing/calling, and thread-safe request ID management with line-delimited message handling.

- **Graph Client** (`graph_client.go`): FalkorDB test client with connection management, Cypher query execution, graph clearing, node counting by label, file existence checks, tag/topic retrieval, and related file discovery through shared graph relationships.

- **Cleanup Infrastructure** (`cleanup.go`): Comprehensive resource management with LIFO cleanup function execution, graceful daemon shutdown with timeout fallback, graph clearing with context deadlines, and selective file removal preserving cache directories.

- **Assertions Helper** (`assertions.go`): 28 assertion functions covering command execution (exit codes, success/failure), string matching (contains, empty checks), data structures (map keys, list lengths), error validation, equality comparisons, and test utilities (retry loops, log dumping).

### Test Fixtures

Test fixtures (`e2e/fixtures/`) provide consistent test data across all test suites:

- **Memory Files**: Sample files for testing indexing (markdown, JSON, images, code files)
- **Configuration Files**: Valid, invalid, and edge-case configuration examples
- **Integration Configs**: Mock integration settings for testing adapters

Fixtures are immutable reference data that tests can copy or reference without modification, ensuring consistent test inputs across runs.

### Test Suites

Test suites (`e2e/tests/`) comprise 18 test files with 171 test functions covering 9,259 lines of test code:

| Test Suite | Lines | Coverage |
|------------|-------|----------|
| **smoke_test.go** | 76 | 3 smoke tests validating harness setup, version command, and help text |
| **cli_test.go** | 384 | 13 CLI command tests covering version, help, invalid commands, daemon operations, config management, graph commands, integrations, and read command |
| **daemon_test.go** | 301 | 4 daemon lifecycle tests including start/stop, health endpoint validation, and multiple start prevention |
| **filesystem_test.go** | 385 | File system integration tests verifying new file detection, modification tracking, deletion handling, rename operations, debouncing, and subdirectory scanning |
| **graph_test.go** | 443 | 7 graph tests validating status checks, connection establishment, query execution, node creation, and relationship management |
| **graph_advanced_test.go** | 892 | Advanced graph operations including schema creation, file upsert logic, semantic search, query optimization, and data persistence |
| **http_api_test.go** | 580 | HTTP API endpoint tests covering search, metadata retrieval, recent files, index queries, and comprehensive error handling |
| **sse_test.go** | 304 | SSE notification tests with single/multiple client handling, event delivery validation, and disconnection recovery |
| **mcp_test.go** | 762 | 11 MCP protocol tests including initialize/shutdown lifecycle, resource operations, tool calls, protocol compliance, and error scenarios |
| **integration_test.go** | 857 | 15 integration adapter tests for claude-code-hook, claude-code-mcp, gemini-cli-mcp, and codex-cli-mcp including setup, validation, and removal |
| **config_test.go** | 876 | 12 configuration tests covering validation, hot-reload, environment variable overrides, immutable field detection, and error reporting |
| **cache_test.go** | 497 | 9 cache tests validating directory creation, file processing, modification detection, cache status reporting, and cache clearing operations |
| **metadata_test.go** | 335 | Metadata extraction tests for different file types and formats |
| **walker_test.go** | 492 | File discovery and walker tests validating skip patterns, directory traversal, and filtering logic |
| **edge_cases_test.go** | 624 | Edge case handling for unusual file types, special characters, and boundary conditions |
| **error_handling_test.go** | 390 | Error scenario tests including graceful degradation, recovery mechanisms, and user-facing error messages |
| **output_formats_test.go** | 436 | Output format validation for JSON, XML, and Markdown with schema compliance and integration wrapping |
| **e2e_test.go** | 625 | 6 comprehensive end-to-end workflows including fresh installation, integration setup, daemon lifecycle, file processing pipeline, MCP tools, graph operations, error recovery, and complete system validation |

Each suite uses the test harness to establish isolated environments and validate complete workflows from user input through system processing to final output.

### Cleanup Infrastructure

The cleanup subsystem (`e2e/harness/cleanup.go`) ensures proper resource cleanup even when tests fail:

- **Resource Registry**: Tracks all temporary directories, processes, and connections
- **Deferred Cleanup**: Uses Go's defer mechanism to guarantee cleanup execution
- **Process Termination**: Sends SIGTERM to daemon processes and waits for graceful shutdown
- **Timeout Handling**: Falls back to SIGKILL if graceful shutdown exceeds timeout threshold
- **Error Reporting**: Logs cleanup failures without failing the test

Cleanup runs after every test via `t.Cleanup()` callbacks, ensuring no resource leaks regardless of test outcome.

---

## Integration Points

### Daemon Subsystem

E2E tests interact with the daemon subsystem by:
- Spawning real daemon processes with isolated configurations
- Monitoring health check endpoints to verify operational status
- Sending HTTP API requests to validate endpoint behavior
- Subscribing to SSE streams for real-time event testing
- Sending SIGHUP signals to trigger configuration reloads
- Verifying log output for expected events and errors

Tests validate that the daemon correctly orchestrates file processing, maintains index state, serves HTTP requests, and handles lifecycle operations.

### Configuration Subsystem

E2E tests validate configuration handling by:
- Creating test configuration files with various settings
- Testing validation rules with valid and invalid configurations
- Verifying hot-reload functionality with non-structural changes
- Ensuring structural changes are rejected during reload
- Testing environment variable overrides
- Validating error messages for configuration problems

Tests ensure the configuration subsystem correctly loads settings, applies validation rules, and supports runtime updates.

### FalkorDB Graph Database

E2E tests interact with FalkorDB through:
- Using Docker Compose to manage graph container lifecycle
- Verifying graph schema creation and constraint management
- Testing node creation, relationship establishment, and query execution
- Validating semantic search across tags, topics, and entities
- Testing related file discovery through graph connections
- Verifying graceful degradation when graph is unavailable

Tests ensure the graph subsystem correctly models file relationships and supports semantic queries.

### Integration Framework

E2E tests validate integration framework behavior by:
- Testing adapter registration and discovery mechanisms
- Validating setup operations that modify configuration files
- Testing remove operations that clean up integration state
- Verifying output processor behavior (XML, JSON, Markdown)
- Testing integration wrapping with framework-specific envelopes
- Validating health check and validation operations

Tests ensure adapters correctly detect frameworks, modify configurations safely, and produce properly formatted output.

### Walker and Watcher Subsystems

E2E tests exercise file discovery through:
- Creating test files and verifying they appear in the index
- Testing skip patterns for directories, files, and extensions
- Validating file watching for create, modify, and delete events
- Testing debouncing behavior with rapid file changes
- Verifying that hidden files and directories are skipped
- Testing walker behavior during full rebuilds

Tests ensure the walker and watcher correctly discover and monitor files according to configuration rules.

### Output Processors

E2E tests validate output formatting by:
- Testing XML structure with schema validation
- Verifying JSON formatting and structure
- Testing Markdown table generation
- Validating integration-specific wrapping (SessionStart hooks, MCP responses)
- Testing empty index handling
- Verifying metadata accuracy in output formats

Tests ensure output processors produce correctly formatted and valid output for different consumption contexts.

---

## Test Categories

The 171 test functions are organized into 12 functional categories providing comprehensive system validation:

### CLI Command Tests (13 tests, 384 lines)

Validate all CLI commands execute correctly with proper argument parsing, formatted output, and appropriate exit codes. Tests verify version display, help text generation, invalid command handling, daemon operations (start/stop/status/restart/rebuild), configuration management (validate/reload), graph commands (start/stop/status), integration operations (list/setup/remove), and read command functionality. Coverage includes both success paths and error conditions with user-facing error message validation.

### Daemon Lifecycle Tests (4 tests, 301 lines)

Verify daemon start/stop/status/restart/rebuild operations through complete lifecycle workflows. Tests ensure PID files are created and removed correctly, health check endpoints respond with accurate status, log files contain expected events, graceful shutdown occurs on SIGTERM/SIGHUP signals, and duplicate start attempts are prevented with clear error messages.

### File System Integration Tests (385 lines)

Validate the complete file processing pipeline from file creation through indexing with real-time monitoring. Tests verify new file detection within debounce windows, modification tracking with content hash updates, deletion event handling, rename operation processing, rapid change debouncing (multiple edits → single processing), subdirectory scanning, and skip pattern enforcement for hidden files and excluded extensions.

### HTTP API Tests (580 lines)

Validate all REST endpoints with comprehensive request/response validation. Tests cover health checks with daemon status, semantic search across tags/topics/summary, file metadata retrieval with graph connections, recent files queries with time windows, related file discovery through shared relationships, entity search with type filtering, rebuild triggers with force flag support, and full index retrieval. Includes error handling validation for malformed requests, missing resources, and invalid parameters.

### SSE Notification Tests (4 tests, 304 lines)

Verify Server-Sent Events deliver real-time notifications for index changes. Tests validate single client event reception, multiple concurrent client handling, event format compliance (file_added, file_modified, file_deleted, rebuild_complete events), connection lifecycle management, automatic reconnection on temporary failures, and proper cleanup on client disconnect.

### Configuration Tests (12 tests, 876 lines)

Validate configuration loading, validation, and runtime updates. Tests ensure YAML parsing correctness, validation rule enforcement (type checking, range validation, path safety), hot-reload functionality for non-structural settings (workers, rate limits, log levels), immutable field detection (memory_root, cache_dir, log files require restart), environment variable override precedence (MEMORIZER_* prefix), and helpful error messages with suggestions for common mistakes.

### Graph Integration Tests (7 basic + advanced, 1,335 lines)

Verify FalkorDB operations from connection management through complex semantic queries. Basic tests validate status checks, connection establishment with authentication, query execution with result parsing, node creation with properties, and relationship management. Advanced tests cover schema creation with constraints, file upsert logic (create vs update), semantic search across multiple signals (tags, topics, entities), related file discovery with connection strength ranking, query optimization for large graphs, and data persistence across daemon restarts. Includes graceful degradation testing when FalkorDB is unavailable.

### Integration Framework Tests (15 tests, 857 lines)

Validate adapter behavior for all supported frameworks (Claude Code hook/MCP, Gemini CLI MCP, Codex CLI MCP). Tests verify framework detection via config file presence, setup operations that safely modify configuration files (JSON for Claude/Gemini, TOML for Codex), validation of integration health and configuration correctness, remove operations that cleanly uninstall integrations, output processor behavior (XML, JSON, Markdown), integration wrapping with framework-specific envelopes (SessionStart hook JSON, MCP JSON-RPC), binary path detection and configuration, and version management for adapter compatibility.

### Metadata and Cache Tests (9 tests + metadata extraction, 832 lines)

Verify metadata extraction handlers for different file types and cache operations. Tests validate cache directory creation with proper permissions, cache key computation from content hashes (SHA-256), cache hit behavior avoiding redundant API calls, cache invalidation on content changes, metadata extraction for text files (word/line counts), images (dimensions, format), PDFs (page counts), office documents (extraction methods), version-based staleness detection (schema/metadata/semantic versions), cache status reporting with entry counts and size statistics, and selective cache clearing (old versions vs. all entries).

### Output Format Tests (436 lines)

Validate output processors produce correctly formatted data for different consumption contexts. Tests verify XML structure with proper schema compliance, JSON formatting with accurate type representation, Markdown table generation with aligned columns, integration-specific wrapping (SessionStart hook JSON envelope with systemMessage/additionalContext fields, MCP JSON-RPC responses), empty index handling with appropriate null/empty values, and metadata accuracy preservation across format transformations.

### Error Handling Tests (390 lines)

Verify graceful degradation and error recovery across failure scenarios. Tests validate daemon behavior when FalkorDB is unavailable (warnings logged, operations continue with reduced functionality), recovery from transient API failures (retry with backoff), handling of corrupted cache files (skip and rebuild), invalid configuration detection with helpful error messages, missing binary path handling during integration setup, permission errors with actionable suggestions, and network timeout handling with appropriate fallback behavior.

### Walker Configuration Tests (492 lines)

Validate file discovery subsystem correctly applies configuration rules. Tests verify skip pattern enforcement for directories (.git, node_modules, .cache), file exclusion by name (.DS_Store, Thumbs.db), extension-based filtering (.log, .tmp, .swp), hidden file/directory handling (leading dot), symlink traversal prevention, directory depth limits, and real-time watcher integration ensuring consistent behavior between full scans and incremental monitoring.

---

## Execution Model

### Build Tags

E2E tests use the `e2e` build tag (`//go:build e2e`) at the top of all 18 test files to separate them from unit tests. This separation enables:
- Fast unit test execution during development (no Docker overhead, no daemon spawning)
- Isolated E2E test execution in CI/CD pipelines with FalkorDB container orchestration
- Prevention of accidental long-running test execution during standard `go test ./...` runs

Tests are compiled only when explicitly requested via `go test -tags=e2e` or `make test-e2e`.

### Test Execution Lifecycle

Each test follows a consistent four-phase lifecycle managed by the test harness:

1. **Setup Phase**: Harness creates isolated test environment
   - Generate unique temporary directory via `t.TempDir()`
   - Create memory root, cache directory, and required subdirectories
   - Write minimal valid configuration with unique graph name
   - Set `MEMORIZER_APP_DIR` environment variable
   - Initialize HTTP, MCP, and Graph clients with test-specific configurations
   - Register cleanup callbacks via `t.Cleanup()`

2. **Execution Phase**: Test performs operations and interactions
   - Run CLI commands via `h.RunCommand()` with captured stdout/stderr/exit codes
   - Start daemon process and wait for health check via `h.StartDaemon()` + `h.WaitForHealthy()`
   - Create test files in memory directory using `h.AddMemoryFile()`
   - Call HTTP API endpoints through type-safe client methods
   - Execute MCP protocol operations (initialize, resource/tool calls, shutdown)
   - Query FalkorDB graph database via Cypher query helpers

3. **Validation Phase**: Test asserts expected behavior
   - Use 28 assertion helpers from `assertions.go` for consistent validation
   - Verify command exit codes, output content, and error messages
   - Validate API response structure, status codes, and data accuracy
   - Check graph node/edge existence and relationship correctness
   - Confirm file processing pipeline produces expected index entries
   - Assert configuration changes applied correctly or rejected appropriately

4. **Cleanup Phase**: Harness releases all resources (guaranteed execution)
   - Stop daemon process with graceful shutdown (SIGTERM with timeout fallback)
   - Clear graph database to prevent cross-test contamination
   - Close HTTP, MCP, and Graph client connections
   - Remove temporary directories (automatic via `t.TempDir()`)
   - Execute registered cleanup functions in LIFO order
   - Cleanup runs even when tests fail or panic (via defer mechanisms)

### Timeout Management

E2E tests employ environment-specific timeout strategies:

**Full Test Suite**: 30-minute timeout (`-timeout 30m`) accounting for:
- FalkorDB Docker container startup (health polling up to 30 seconds)
- Binary compilation if not pre-built
- Daemon initialization across all 171 tests
- Cumulative file processing and semantic analysis operations
- Graph query execution for complex relationship searches

**Individual Test Suites**: 10-minute timeout per suite via Makefile targets:
- `make test-cli` - CLI command tests (10m)
- `make test-daemon` - Daemon lifecycle tests (10m)
- `make test-mcp` - MCP protocol tests (10m)
- `make test-integrations` - Integration framework tests (10m)
- `make test-config` - Configuration tests (10m)
- `make test-graph` - Graph integration tests (10m)
- `make test-e2e` - Comprehensive workflow tests (10m)

**Quick Smoke Tests**: 5-minute timeout (`make test-quick`) for rapid validation during development with only 3 essential tests (harness setup, version command, help text).

**Component-Specific Timeouts**:
- Health endpoint polling: 2-second wait after PID file creation
- HTTP client requests: 5-second timeout per API call
- Graph query operations: 10-second context deadline for complex queries
- Daemon graceful shutdown: 5-second timeout before SIGKILL fallback
- FalkorDB health check: 30-second polling with 1-second intervals

**Makefile Test Targets** (`e2e/Makefile`):
```makefile
test                    # Full suite (30m timeout, all 18 files)
test-quick             # Smoke tests only (5m timeout, 3 tests)
test-verbose           # All tests with verbose output (-count=1 prevents caching)
test-cli               # CLI command tests (10m)
test-daemon            # Daemon lifecycle tests (10m)
test-mcp               # MCP protocol tests (10m)
test-integrations      # Integration framework tests (10m)
test-config            # Configuration tests (10m)
test-graph             # Graph integration tests (10m)
test-e2e               # Comprehensive workflow tests (10m)
```

All targets use `-tags=e2e` flag and run tests from `./tests/...` directory. Environment setup includes FalkorDB container startup with health polling before test execution begins.

---

## Glossary

**E2E Test**: End-to-end test that validates complete workflows by executing the real application binary and exercising multiple integrated subsystems.

**Test Harness**: Reusable infrastructure that provides environment setup, daemon management, HTTP client, graph client, and cleanup functionality for E2E tests.

**Isolation**: Property ensuring each test runs in its own environment without affecting or being affected by other tests.

**Fixture**: Immutable test data file used as input for tests, stored in the `e2e/fixtures/` directory.

**Build Tag**: Go compilation directive (`//go:build e2e`) that controls which files are compiled based on command-line flags.

**Cleanup Callback**: Function registered via `t.Cleanup()` that runs after test completion to release resources, even if the test fails.

**SSE (Server-Sent Events)**: HTTP-based protocol for real-time server-to-client notifications used by the daemon to broadcast index changes.

**Graceful Degradation**: Property where the system continues operating with reduced functionality when dependencies are unavailable.

**Hot-Reload**: Capability to apply configuration changes to a running daemon without restarting the process.

**Semantic Search**: Graph-powered search that finds files by matching tags, topics, entities, and summary text.

**Integration Adapter**: Component that provides framework-specific integration by detecting the framework, modifying configuration files, and wrapping output appropriately.

**Output Processor**: Component that formats index data into specific output formats (XML, JSON, Markdown) for different consumption contexts.
