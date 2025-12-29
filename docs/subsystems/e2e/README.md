# End-to-End Testing

Comprehensive integration testing with isolated environments, Docker-based FalkorDB, and full-stack validation.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The End-to-End (E2E) testing subsystem provides a comprehensive integration testing framework for validating the complete Agentic Memorizer system. Unlike unit tests that verify individual components in isolation, E2E tests exercise full workflows from CLI commands through daemon processing to graph storage, ensuring all subsystems work together correctly.

The subsystem consists of three main layers: a test harness framework that orchestrates isolated test environments, a collection of 21 test suites covering all major features, and Docker Compose infrastructure for running FalkorDB in a controlled environment. Tests use Go's standard testing package with build tags (`//go:build e2e`) to separate them from unit tests.

Key capabilities include:

- **Environment isolation** - Each test receives a unique temporary directory, configuration, and graph database namespace
- **Multi-layer client abstractions** - Dedicated clients for HTTP API, MCP protocol, and FalkorDB graph queries
- **Automatic cleanup** - LIFO cleanup stack ensures resources are released even when tests fail
- **Docker integration** - FalkorDB runs in a container with health checks, automatic startup, and volume cleanup
- **Parallel execution** - Tests can run in parallel with isolated graph names preventing cross-test contamination

## Design Principles

### Test Isolation

Every test operates in complete isolation from other tests. The harness creates a unique temporary directory via `t.TempDir()`, generates a timestamp-based graph name (`e2e_test_{nanoseconds}`), and sets environment variables to point the binary at this isolated environment. This prevents any state leakage between tests, enabling reliable parallel execution and deterministic results.

### Layered Client Abstractions

Rather than directly calling system commands or APIs, tests use specialized client abstractions that encapsulate protocol details and provide clean interfaces:

- **HTTPClient** - Wraps daemon HTTP API calls with automatic timeout, retry logic, and structured responses
- **MCPClient** - Manages stdio-based JSON-RPC 2.0 communication with the MCP server subprocess
- **GraphClient** - Provides Cypher query execution against FalkorDB with connection pooling

This abstraction makes tests more readable, reduces duplication, and centralizes protocol handling.

### Idempotent Cleanup

The cleanup system guarantees resource release through multiple mechanisms. A `done` flag prevents double cleanup, cleanup functions execute in LIFO order to respect dependencies, and `t.Cleanup()` registration ensures execution even on test failure. The daemon shutdown sequence includes graceful termination (20s timeout) with force-kill fallback for Docker environment slowness.

### Build Tag Separation

All E2E tests use `//go:build e2e` to exclude them from regular `go test` runs. This separation is important because E2E tests require Docker infrastructure, take longer to execute, and may modify external state. Developers run unit tests frequently during development while E2E tests run in CI or before releases.

### Graceful Degradation Testing

Tests verify the system handles unavailable dependencies gracefully. When FalkorDB cannot be reached, tests skip with `t.Skipf()` rather than fail, and daemon behavior under graph unavailability is explicitly validated to ensure the system logs warnings but continues operating with fallback search.

## Key Components

### Test Harness (`e2e/harness/`)

The harness package provides the core testing infrastructure:

**E2EHarness** orchestrates test environments with isolated directories, configuration files, and client instances. Key methods include `Setup()` for environment initialization, `StartDaemon()` and `StopDaemon()` for process lifecycle, `AddMemoryFile()` for test data creation, and `RunCommand()` for CLI execution with captured output.

**Cleanup** manages resource teardown through a registered function stack. `MustCleanup()` registers with `t.Cleanup()` for automatic execution, while `CleanupAll()` executes the full sequence: graceful daemon stop, force kill if needed, graph clearing, file removal, and custom cleanup functions.

**Assertions** provides test-specific helpers like `AssertExitCode()`, `AssertContains()`, `AssertNoError()`, and `AssertCommandSuccess()` that integrate with `testing.T` for proper error attribution.

### Test Clients

**HTTPClient** (`http_client.go`) connects to the daemon HTTP API with a 5-second timeout. Methods include `Health()`, `WaitForHealthy()`, `SearchFiles()`, `GetFileMetadata()`, `ListRecentFiles()`, `GetRelatedFiles()`, `SearchEntities()`, and `TriggerRebuild()`.

**MCPClient** (`mcp_client.go`) manages an MCP server subprocess via stdio pipes. It handles JSON-RPC 2.0 protocol with sequential request IDs, implements `Initialize()`, `ListResources()`, `ListTools()`, `CallTool()`, and `Shutdown()` for complete protocol coverage.

**GraphClient** (`graph_client.go`) provides direct FalkorDB access for verification queries. `Query()` executes arbitrary Cypher, while specialized methods like `FileExists()`, `CountNodes()`, and `GetRelatedFiles()` simplify common assertions.

### Test Suites (`e2e/tests/`)

Twenty-one test files organized by subsystem:

- **smoke_test.go** - Quick sanity checks (harness, version, help)
- **cli_test.go** - All CLI commands with output validation
- **daemon_test.go** - Lifecycle operations (start, stop, restart, rebuild)
- **config_test.go** - Validation, hot-reload, environment overrides
- **graph_test.go** / **graph_advanced_test.go** - Graph operations and queries
- **http_api_test.go** - All HTTP endpoints with status codes
- **mcp_test.go** - Protocol handshake, resources, tools
- **integration_test.go** - Claude Code, Gemini, Codex adapter testing
- **facts_test.go** / **integrations_facts_test.go** - User facts CRUD and hook injection
- **filesystem_test.go** - File watching, modification, deletion
- **cache_test.go** - Hit/miss behavior, versioning, provider isolation
- **semantic_providers_test.go** - Provider configuration and routing
- **metadata_test.go** - Extraction accuracy by file type
- **walker_test.go** - Directory scanning and filtering
- **sse_test.go** - Server-Sent Events streaming
- **output_formats_test.go** - JSON, YAML, XML, Markdown output
- **error_handling_test.go** / **edge_cases_test.go** - Error conditions and boundaries
- **e2e_test.go** - Complete workflow tests

### Test Fixtures (`e2e/fixtures/`)

Pre-created test data organized by purpose:

- **memory/** - Sample files for indexing (markdown, Go code, JSON config)
- **configs/** - YAML configurations (minimal, valid, invalid)
- **integrations/** - Integration adapter configuration samples

### Docker Infrastructure

**docker-compose.yml** defines two services: `falkordb` (graph database on port 16379 with health checks) and `test-runner` (Go test execution with volume mounts for code, test data, and Go module cache).

**Dockerfile.test** builds a Debian-based image with Go 1.25.1, Docker CLI, and the compiled binary placed at `/usr/local/bin/memorizer`.

**Makefile** provides 28 targets for test execution, including `test` (full suite), `test-quick` (smoke tests), and individual suite targets like `test-cli`, `test-daemon`, `test-mcp`, and `test-graph`.

## Integration Points

### Daemon Subsystem

E2E tests validate daemon lifecycle through the harness's `StartDaemon()` and `StopDaemon()` methods, which execute the actual binary. Tests verify PID file creation, health endpoint availability, graceful shutdown on SIGTERM, and proper cleanup of resources. The daemon tests also validate SSE event delivery and configuration hot-reload via SIGHUP.

### HTTP API

The `HTTPClient` directly exercises all daemon API endpoints, validating response formats, status codes, and error handling. Tests cover health checks with graph metrics, semantic search queries, file metadata retrieval, recent files listing, related files discovery, entity search, and rebuild triggers.

### MCP Protocol

The `MCPClient` spawns an actual MCP server process and communicates via stdio, testing the complete JSON-RPC 2.0 protocol implementation. Tests verify initialization handshake (protocol version negotiation), resource listing and reading (file index in multiple formats), tool execution (all five MCP tools), and graceful shutdown.

### Graph Storage

The `GraphClient` connects directly to FalkorDB for verification queries independent of the daemon. Tests validate node creation, relationship edges, Cypher query execution, data persistence across daemon restarts, and proper cleanup. Graph isolation via unique names enables parallel test execution.

### Integration Adapters

Integration tests exercise the complete adapter workflow: detection of existing integration configurations, setup with proper binary path resolution, configuration file generation (JSON for Claude/Gemini, TOML for Codex), removal, and idempotency verification for repeated setup calls.

### Configuration System

Config tests validate the full configuration lifecycle: loading from YAML, environment variable overrides, validation error accumulation, hot-reload compatibility checking, and runtime updates. Tests use fixture configurations to verify both valid and invalid scenarios.

### Cache System

Cache tests verify content-addressable caching behavior through daemon processing. Tests validate cache hits on unchanged content, cache misses on modifications, provider isolation between semantic providers, version-based staleness detection, and cache statistics accuracy.

## Glossary

**Build Tag**
A Go compiler directive (`//go:build e2e`) that includes or excludes source files from compilation based on tags specified during build or test commands.

**Content-Addressable**
A storage scheme where cache keys derive from content hashes (SHA-256) rather than file paths, enabling cache hits when files are renamed or moved without content changes.

**Cypher**
A declarative graph query language used by FalkorDB (and Neo4j) for creating, reading, updating, and deleting graph data through pattern matching expressions.

**E2E Test**
End-to-end test that exercises the complete system from entry point (CLI command) through all layers (daemon, API, graph) to verify integrated behavior rather than isolated component functionality.

**FalkorDB**
A Redis-compatible graph database that stores the knowledge graph of files, tags, topics, entities, and their relationships. E2E tests run FalkorDB in Docker with port 16379.

**Graph Isolation**
The practice of using unique graph names per test (based on nanosecond timestamps) to prevent data interference between tests running in parallel or sequence.

**Harness**
The test orchestration framework (`e2e/harness/`) that manages environment setup, client creation, command execution, and resource cleanup for E2E tests.

**JSON-RPC 2.0**
A stateless, light-weight remote procedure call protocol using JSON encoding. The MCP protocol uses JSON-RPC 2.0 over stdio for tool invocation and resource access.

**LIFO Cleanup**
Last-In-First-Out ordering for cleanup functions, ensuring that resources are released in reverse order of acquisition to respect dependencies (e.g., stop daemon before clearing graph).

**MCP (Model Context Protocol)**
A protocol for AI assistants to access external tools and resources. The MCP client in E2E tests validates the complete protocol implementation including initialization, resource listing, and tool execution.

**Smoke Test**
A minimal test suite that quickly validates basic functionality is working before running the full test suite. The `smoke_test.go` file provides quick sanity checks for CI pipelines.

**Test Fixture**
Pre-created test data (files, configurations) stored in `e2e/fixtures/` and used across multiple tests to provide consistent, known inputs for validation.
