# End-to-End Test Suite

Comprehensive end-to-end testing framework for Agentic Memorizer.

## Overview

This test suite provides isolated, containerized testing of all major subsystems:
- CLI command execution
- Daemon lifecycle and file processing
- MCP server/client protocol
- Integration framework adapters
- Configuration and hot-reload
- FalkorDB graph operations
- Complete user workflows

## Architecture

The test suite uses Docker Compose to orchestrate:
- **FalkorDB**: Graph database for testing (port 16379)
- **Test Runner**: Container with Go toolchain and binary

Test harness framework (`harness/`):
- `harness.go` - Main E2E test orchestrator
- `mcp_client.go` - MCP JSON-RPC 2.0 client simulation
- `graph_client.go` - FalkorDB test client
- `http_client.go` - Daemon HTTP API client
- `assertions.go` - Custom test assertions
- `cleanup.go` - Resource cleanup utilities

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.25+
- Make

### Run All Tests

```bash
cd e2e
make test
```

### Run Quick Smoke Tests

```bash
make test-quick
```

### Run Specific Test Suite

```bash
make test-cli          # CLI command tests
make test-daemon       # Daemon lifecycle tests
make test-mcp          # MCP server/client tests
make test-integrations # Integration adapter tests
make test-config       # Configuration tests
make test-graph        # FalkorDB graph tests
make test-e2e          # End-to-end workflow tests
```

## Test Suites

### 1. CLI Tests (`tests/cli_test.go`)

Tests all 30 CLI commands with real binary execution.

**Coverage:**
- `initialize` - Interactive and non-interactive modes
- `daemon` - start, stop, status, restart, rebuild, logs, systemctl, launchctl
- `graph` - start, stop, status
- `integrations` - list, detect, setup, remove, validate, health
- `config` - validate, reload
- `mcp` - start
- `read`, `version`

### 2. Daemon Lifecycle Tests (`tests/daemon_test.go`)

Tests daemon process management and health monitoring.

**Coverage:**
- Start/stop/restart operations
- PID file management
- Health endpoint responses
- Graceful shutdown (SIGTERM)
- SSE event delivery
- FalkorDB graceful degradation

### 3. File System Tests (`tests/filesystem_test.go`)

Tests file watching, processing, and indexing.

**Coverage:**
- New file detection and processing
- File modification and re-analysis
- File deletion and index updates
- File rename handling
- Debouncing (rapid changes)
- Skip patterns (.cache, .git, extensions)
- Subdirectory scanning

### 4. MCP Server Tests (`tests/mcp_test.go`)

Tests MCP JSON-RPC 2.0 protocol implementation.

**Coverage:**
- Initialize handshake
- Resources: list, read (XML/JSON)
- Tools: list, call (search_files, get_file_metadata, list_recent_files, get_related_files, search_entities)
- Error handling
- Shutdown sequence

### 5. Integration Tests (`tests/integration_test.go`)

Tests all integration framework adapters.

**Coverage per framework:**
- Claude Code Hook: detect, setup, validate, remove
- Claude Code MCP: setup, validate, binary path
- Gemini CLI MCP: setup, validate, TOML format
- Codex CLI MCP: setup, validate, TOML parsing
- Idempotency of setup operations

### 6. Configuration Tests (`tests/config_test.go`)

Tests configuration loading, validation, and hot-reload.

**Coverage:**
- Valid config loading
- Invalid config rejection
- Environment variable overrides
- Hot-reload via SIGHUP
- Immutable field protection (memory_root)
- Mutable field updates (workers, rate_limit)
- Validation error handling

### 7. FalkorDB Graph Tests (`tests/graph_test.go`)

Tests knowledge graph operations.

**Coverage:**
- Connection and schema creation
- File node upsert
- Tag/topic/entity relationships
- Search by tag/topic/entity
- Related files queries
- Recent files filtering
- Daemon restart persistence
- Graceful degradation

### 8. End-to-End Workflow Tests (`tests/e2e_test.go`)

Tests complete user workflows.

**Workflows:**
- Fresh installation (initialize → start → add files → query → stop)
- Integration setup (detect → setup → validate → read → remove)
- Configuration changes (modify → reload → verify)
- MCP tool usage (connect → search → metadata → related → disconnect)

## Test Harness Usage

```go
func TestExample(t *testing.T) {
    // Create harness
    h := harness.New(t)

    // Setup environment
    if err := h.Setup(); err != nil {
        t.Fatalf("Setup failed: %v", err)
    }

    // Register cleanup
    cleanup := harness.MustCleanup(t, h)
    defer cleanup.CleanupAll()

    // Start daemon
    if err := h.StartDaemon(); err != nil {
        t.Fatalf("Failed to start daemon: %v", err)
    }

    // Wait for healthy
    if err := h.WaitForHealthy(30 * time.Second); err != nil {
        t.Fatalf("Daemon not healthy: %v", err)
    }

    // Add test file
    if err := h.AddMemoryFile("test.md", "# Test"); err != nil {
        t.Fatalf("Failed to add file: %v", err)
    }

    // Run command
    stdout, stderr, exitCode := h.RunCommand("read")
    harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
    harness.AssertContains(t, stdout, "test.md")
}
```

## MCP Client Usage

```go
func TestMCP(t *testing.T) {
    h := harness.New(t)
    h.Setup()
    defer h.Teardown()

    // Create MCP client
    client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
    if err != nil {
        t.Fatalf("Failed to create MCP client: %v", err)
    }
    defer client.Close()

    // Initialize
    initResp, err := client.Initialize()
    harness.AssertNoError(t, err, "Initialize")
    harness.AssertEqual(t, "agentic-memorizer", initResp.ServerInfo.Name, "Server name")

    // List resources
    resources, err := client.ListResources()
    harness.AssertNoError(t, err, "ListResources")
    harness.AssertListLength(t, resources, 3, "Resources")

    // Call tool
    result, err := client.CallTool("search_files", map[string]any{
        "query": "test",
        "max_results": 10,
    })
    harness.AssertNoError(t, err, "CallTool")
}
```

## Graph Client Usage

```go
func TestGraph(t *testing.T) {
    h := harness.New(t)
    h.Setup()
    defer h.Teardown()

    ctx := context.Background()

    // Connect to graph
    if err := h.GraphClient.Connect(); err != nil {
        t.Fatalf("Failed to connect to graph: %v", err)
    }

    // Clear graph
    if err := h.GraphClient.Clear(ctx); err != nil {
        t.Fatalf("Failed to clear graph: %v", err)
    }

    // Check file exists
    exists, err := h.GraphClient.FileExists(ctx, "/path/to/file")
    harness.AssertNoError(t, err, "FileExists")
    harness.AssertTrue(t, exists, "File should exist in graph")

    // Count nodes
    count, err := h.GraphClient.CountNodes(ctx, "File")
    harness.AssertNoError(t, err, "CountNodes")
    harness.AssertEqual(t, 1, count, "File count")
}
```

## Development

### Adding New Test Suite

1. Create test file in `tests/` (e.g., `tests/newsuite_test.go`)
2. Add build tag: `//go:build e2e`
3. Import harness: `import "github.com/leefowlercu/agentic-memorizer/e2e/harness"`
4. Write tests using harness framework
5. Add Makefile target in `e2e/Makefile`:
   ```makefile
   test-newsuite: setup
       docker compose run --rm test-runner go test -tags=e2e -v ./e2e/tests/newsuite_test.go
   ```

### Adding Test Fixtures

Add files to appropriate fixture directory:
- `fixtures/memory/` - Sample files for indexing
- `fixtures/configs/` - Test configuration files
- `fixtures/integrations/` - Mock integration configs

### Debugging Tests

```bash
# Run with verbose output
make test-verbose

# Open shell in test environment
make shell

# View FalkorDB logs
make logs

# Clean environment and start fresh
make clean-volumes
make test
```

## CI/CD Integration

See `.github/workflows/e2e-tests.yml` for CI/CD pipeline configuration.

The test suite is designed to run in CI with:
- Automatic Docker Compose orchestration
- Test result artifacts
- 30-minute timeout
- Parallel test execution where possible

## Troubleshooting

### Tests fail to start

```bash
# Ensure Docker is running
docker info

# Rebuild images
make clean
make setup
```

### FalkorDB connection errors

```bash
# Check FalkorDB status
docker compose ps falkordb

# View logs
docker compose logs falkordb

# Restart FalkorDB
docker compose restart falkordb
```

### Binary not found

```bash
# Build binary
cd ..
make build
```

### Tests timeout

Increase timeout in test command:
```bash
docker compose run --rm test-runner go test -tags=e2e -v -timeout 60m ./e2e/tests/...
```

## Test Coverage

Current coverage (as of v0.12.0):
- Unit tests: 364 test functions
- Integration tests: 30+ scenarios
- E2E tests: 8 comprehensive suites

Target: 80%+ code coverage across all subsystems.
