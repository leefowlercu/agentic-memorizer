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

The harness framework (`e2e/harness/`) provides the foundation for all E2E tests:

- **E2EHarness**: Core orchestration structure managing test environments, daemon processes, and cleanup
- **Environment Management**: Creates isolated application directories with unique configurations
- **Daemon Control**: Methods for starting, stopping, and health-checking daemon processes
- **HTTP Client**: Provides typed methods for calling daemon HTTP API endpoints
- **Graph Client**: Manages FalkorDB connections and provides graph query helpers
- **Binary Execution**: Runs CLI commands with captured stdout/stderr and exit codes
- **Cleanup Tracking**: Maintains registry of all resources for guaranteed cleanup

### Test Fixtures

Test fixtures (`e2e/fixtures/`) provide consistent test data across all test suites:

- **Memory Files**: Sample files for testing indexing (markdown, JSON, images, code files)
- **Configuration Files**: Valid, invalid, and edge-case configuration examples
- **Integration Configs**: Mock integration settings for testing adapters

Fixtures are immutable reference data that tests can copy or reference without modification, ensuring consistent test inputs across runs.

### Test Suites

Test suites (`e2e/tests/`) are organized by functional area:

- **CLI Tests**: Validate command execution, argument parsing, output formatting, and error handling
- **Daemon Tests**: Test lifecycle operations (start, stop, status, restart, rebuild)
- **Filesystem Tests**: Verify file watching, processing pipelines, and cache behavior
- **HTTP API Tests**: Validate REST endpoints, request/response formats, and error responses
- **SSE Tests**: Test real-time notification delivery and connection management
- **Configuration Tests**: Verify loading, validation, hot-reload, and error reporting
- **Graph Tests**: Validate FalkorDB operations, schema management, and semantic queries
- **Integration Tests**: Test adapter behavior, setup/remove operations, and output wrapping
- **Output Format Tests**: Verify XML, JSON, and Markdown output processors
- **Error Handling Tests**: Test graceful degradation and error recovery scenarios

Each suite uses the test harness to set up isolated environments and validate complete workflows from user input to system output.

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

### CLI Command Tests

Validate all CLI commands execute correctly, handle arguments properly, display appropriate output, and return correct exit codes. Tests cover both success paths and error conditions, ensuring helpful error messages guide users toward correct usage.

### Daemon Lifecycle Tests

Verify daemon start, stop, status, restart, and rebuild operations work correctly. Tests ensure PID files are managed properly, health checks respond correctly, log files are created, and graceful shutdown occurs on SIGTERM.

### File System Integration Tests

Validate the complete file processing pipeline from file creation through indexing. Tests verify that files are discovered, metadata extracted, semantic analysis performed, cache hits occur on repeated processing, and index entries contain expected data.

### HTTP API Tests

Validate all REST endpoints including health checks, semantic search, file metadata retrieval, related file queries, entity search, and rebuild operations. Tests verify request parsing, response formatting, error handling, and status codes.

### SSE Notification Tests

Verify Server-Sent Events deliver real-time notifications for file additions, modifications, deletions, and rebuilds. Tests validate event format, connection management, and proper cleanup on disconnect.

### Configuration Tests

Validate configuration loading, validation rules, hot-reload functionality, and error reporting. Tests ensure invalid configurations are rejected with helpful messages, structural changes are blocked during reload, and non-structural changes apply correctly.

### Graph Integration Tests

Verify FalkorDB operations including schema creation, node/edge management, semantic queries, related file discovery, and graceful degradation. Tests ensure the graph correctly models file relationships and supports complex queries.

### Integration Framework Tests

Validate adapter behavior for all supported frameworks including detection, setup operations, configuration file modifications, remove operations, and output wrapping. Tests ensure adapters safely modify configuration files and produce correctly formatted output.

### Metadata and Cache Tests

Verify metadata extraction for different file types, cache key computation, cache hit behavior, and cache invalidation on content changes. Tests ensure the cache subsystem correctly identifies identical content and avoids redundant processing.

### Output Format Tests

Validate XML, JSON, and Markdown output formatting including structure, schema compliance, integration wrapping, and empty index handling. Tests ensure output processors produce valid, well-formed output for all consumption contexts.

### Error Handling Tests

Verify graceful degradation when external dependencies are unavailable, proper error messages for user mistakes, recovery from transient failures, and continued operation despite component failures.

### Walker Configuration Tests

Validate that walker correctly applies skip patterns for directories, files, and extensions. Tests ensure configuration rules are honored during both full scans and real-time watching.

---

## Execution Model

### Build Tags

E2E tests use the `e2e` build tag to separate them from unit tests. This allows:
- Running unit tests quickly during development without E2E overhead
- Running E2E tests separately in CI/CD pipelines
- Avoiding accidental execution of long-running E2E tests during unit test runs

Tests are compiled only when explicitly requested via `go test -tags=e2e`.

### Test Execution Lifecycle

Each test follows this lifecycle:
1. **Setup**: Harness creates isolated application directory and configuration
2. **Execution**: Test runs CLI commands, spawns daemon, or calls HTTP endpoints
3. **Validation**: Test asserts expected behavior using standard Go testing assertions
4. **Cleanup**: Harness terminates processes, removes temporary directories, and closes connections

The harness ensures cleanup always runs via `t.Cleanup()` callbacks, even when tests fail or panic.

### Timeout Management

E2E tests use generous timeouts to account for:
- Docker container startup latency
- Daemon initialization time
- File processing delays
- Graph query execution time

Tests that spawn long-running operations include explicit timeout parameters to prevent hanging, typically set to 5-30 minutes depending on test complexity.

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
