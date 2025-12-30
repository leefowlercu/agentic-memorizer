# Daemon

Background file monitoring and knowledge graph maintenance with parallel processing, hot-reload, and HTTP API.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Daemon subsystem orchestrates continuous file monitoring and knowledge graph maintenance. It watches a memory directory for changes, extracts metadata, performs AI-powered semantic analysis, and maintains a FalkorDB knowledge graph with files, tags, topics, and entities.

The daemon runs in foreground mode, delegating process supervision to external service managers (systemd on Linux, launchd on macOS). This follows modern Go best practices by avoiding fork/exec complexity while leveraging production-grade process supervision.

Key capabilities include parallel file processing through a worker pool, content-addressable caching to minimize API calls, hot-reload for configuration changes without restart, real-time Server-Sent Events for client notifications, and a comprehensive HTTP API for health monitoring and graph queries.

## Design Principles

### External Process Supervision

The daemon runs as a foreground process, delegating lifecycle management to platform service managers:

- **systemd** (Linux) - Type=notify integration with SdNotify readiness signal
- **launchd** (macOS) - KeepAlive with SuccessfulExit=false for crash recovery

This approach avoids fork() complexity with goroutines and provides native logging integration (journald, Console.app).

### Worker Pool with Rate Limiting

File processing uses a configurable worker pool with token bucket rate limiting:

- Parallel workers (default 3) for concurrent file processing
- Per-provider rate limits (Claude 20/min, OpenAI 60/min, Gemini 100/min)
- Priority ordering processes recently modified files first
- Batch submission for rebuild operations

### Graceful Degradation

The daemon employs graceful degradation strategies:

- If semantic API is unavailable, continues with metadata-only processing
- If initial rebuild fails but existing graph data exists, continues in degraded mode
- If graph connection is lost, continues monitoring and queues updates

### Atomic Configuration Updates

Configuration hot-reload uses atomic value replacement:

- Configuration swap is protected by mutex
- Semantic provider replacement uses atomic.Value for lock-free access
- Logger replacement is thread-safe for runtime log level changes
- Structural changes (memory_root, cache_dir) require restart

### Ordered Shutdown

Shutdown follows a strict ordering to prevent data loss:

1. Stop inbound requests (HTTP server)
2. Stop event sources (file watcher)
3. Drain workers (wait group)
4. Close external connections (graph manager)
5. Cleanup state (PID file)

## Key Components

### Daemon

The Daemon struct (`internal/daemon/daemon.go`) is the central orchestrator:

- **New** - Creates daemon with all subsystem dependencies
- **Start** - Performs initial build, starts watcher, HTTP server, and periodic rebuild
- **Stop** - Initiates graceful shutdown
- **ReloadConfig** - Hot-reloads configuration via SIGHUP
- **Rebuild** - Forces immediate index rebuild

### Worker Pool

The worker pool (`internal/daemon/worker/`) provides parallel file processing:

- **Pool** - Manages worker goroutines with job and result channels
- **Job** - Represents a file to process with priority
- **JobResult** - Contains processed entry with metadata, semantics, and optional embedding
- **CalculatePriority** - Assigns priority based on file modification time

### HTTP Server

The HTTP server (`internal/daemon/api/server.go`) provides a unified API:

- **Health endpoint** (`GET /health`) - Returns daemon status, metrics, and semantic provider info
- **Files query endpoint** (`GET /api/v1/files`) - Unified query supporting semantic search and filtering with parameters: `q` (search query), `entity`, `tag`, `topic`, `category`, `days` (recently modified), and `limit`
- **Files index endpoint** (`GET /api/v1/files/index`) - Exports complete FileIndex from graph
- **File metadata endpoint** (`GET /api/v1/files/{path}`) - File metadata with related files, supports `related_limit` parameter
- **Facts index endpoint** (`GET /api/v1/facts/index`) - Lists all stored facts with statistics
- **Fact metadata endpoint** (`GET /api/v1/facts/{id}`) - Individual fact retrieval by ID
- **Rebuild endpoint** (`POST /api/v1/rebuild`) - Trigger rebuild via API, supports `force=true` parameter
- **SSE endpoint** (`GET /sse`) - Real-time index update notifications

API endpoints have 60-second timeouts; health and SSE endpoints have no timeout.

### SSE Hub

The SSE hub (`internal/daemon/api/sse.go`) provides real-time notifications:

- **HandleSSE** - Handles SSE client connections
- **BroadcastIndexUpdate** - Notifies clients of index changes
- **ClientCount** - Returns connected client count for health metrics

### Health Metrics

The HealthMetrics tracker (`internal/daemon/health.go`) provides observability:

- Uptime and start time tracking
- Files processed, API calls, and cache hits counters
- Error tracking with degraded/unhealthy status thresholds
- Cache version and entry statistics

### Signal Handler

Signal handling (`internal/daemon/signals.go`) provides graceful control:

- **SIGINT/SIGTERM** - Graceful shutdown
- **SIGUSR1** - Trigger manual rebuild
- **SIGHUP** - Hot-reload configuration

### PID Management

PID file handling (`internal/daemon/pid.go`) prevents duplicate instances:

- Checks for existing PID file and running process
- Writes PID file on start
- Removes PID file on clean shutdown

## Integration Points

### File Watcher

The file watcher subsystem provides real-time change detection. The daemon receives events through a channel and processes them via handleFileEvent, which extracts metadata, performs semantic analysis, and updates the graph.

### Walker

The walker subsystem provides directory traversal for rebuild operations. The daemon uses it to collect all files for batch processing through the worker pool.

### Graph Manager

The graph manager provides persistent storage via FalkorDB. The daemon writes all file entries and relationships to the graph, and the HTTP API queries the graph for search and retrieval operations.

### Cache Manager

The cache manager provides content-addressable caching. The daemon checks cache before semantic analysis and stores results keyed by content hash and provider.

### Semantic Providers

Semantic providers (Claude, OpenAI, Gemini) perform AI-powered analysis. The daemon loads the configured provider from the registry and performs atomic provider replacement during hot-reload.

### Metadata Extractor

The metadata extractor provides fast, deterministic file metadata extraction. The daemon always extracts metadata regardless of semantic analysis availability.

### Configuration

The config subsystem provides layered configuration. The daemon loads configuration at startup and responds to SIGHUP for hot-reload, validating changes before applying them atomically.

### CLI Commands

The daemon CLI commands (`cmd/daemon/subcommands/`) provide user-facing control:

- `daemon start` - Start the daemon
- `daemon stop` - Stop the daemon via PID file
- `daemon status` - Check daemon status
- `daemon restart` - Restart the daemon
- `daemon logs` - Tail daemon logs
- `daemon rebuild` - Trigger manual rebuild via SIGUSR1
- `daemon systemctl` - Generate systemd unit file
- `daemon launchctl` - Generate launchd plist file

## Glossary

| Term | Definition |
|------|------------|
| Degraded Mode | State where daemon continues operating despite partial failures |
| Hot-Reload | Ability to change configuration without restarting the daemon |
| Job Priority | Numeric value determining processing order; lower values processed first |
| PID File | File containing process ID to prevent duplicate daemon instances |
| Rate Limiter | Token bucket algorithm enforcing API call limits per provider |
| Rebuild | Full directory scan and graph update operation |
| SdNotify | systemd notification mechanism for daemon readiness |
| SSE | Server-Sent Events for real-time client notifications |
| Worker Pool | Set of goroutines processing files in parallel |
