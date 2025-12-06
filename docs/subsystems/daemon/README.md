# Daemon Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
3. [Key Components](#key-components)
   - [Core Daemon](#core-daemon)
   - [Worker Pool](#worker-pool)
   - [File Watcher](#file-watcher)
   - [Graph Manager](#graph-manager)
   - [PID Management](#pid-management)
   - [Signal Handling](#signal-handling)
   - [Health Monitoring](#health-monitoring)
   - [SSE Notification Hub](#sse-notification-hub)
   - [Embeddings System](#embeddings-system)
   - [Service Manager Integration](#service-manager-integration)
4. [Integration Points](#integration-points)
   - [Configuration System](#configuration-system)
   - [Cache System](#cache-system)
   - [Metadata Extractor](#metadata-extractor)
   - [Semantic Analyzer](#semantic-analyzer)
   - [Directory Walker](#directory-walker)
   - [Graph System](#graph-system)
   - [CLI Commands](#cli-commands)
   - [Read Command](#read-command)
   - [MCP Server Integration](#mcp-server-integration)
5. [Lifecycle Management](#lifecycle-management)
   - [Startup Sequence](#startup-sequence)
   - [Runtime Operations](#runtime-operations)
   - [Shutdown Sequence](#shutdown-sequence)
6. [Future Enhancements](#future-enhancements)
7. [Glossary](#glossary)

## Overview

The daemon subsystem is the core engine of Agentic Memorizer, serving as the foundation for automatic file awareness and semantic understanding. It operates as a background process that continuously monitors a designated memory directory, processes changes in real-time, and maintains a precomputed index for instant access by AI agent frameworks.

### Purpose

The daemon provides several critical capabilities:

- **Continuous Monitoring**: Watches the memory directory for file system changes using platform-native file system events
- **Parallel Processing**: Processes files concurrently using a worker pool architecture with configurable concurrency limits
- **Precomputed Indexes**: Maintains an always-up-to-date index file that can be loaded in milliseconds
- **Intelligent Caching**: Reduces API costs and processing time by caching semantic analysis results keyed by content hash
- **Real-Time Notifications**: Broadcasts index update events to connected MCP servers via Server-Sent Events (SSE)
- **Resilient Operation**: Recovers gracefully from crashes, handles errors without stopping, and uses atomic operations to prevent data corruption

### Role in the System

The daemon acts as the bridge between raw file storage and intelligent semantic awareness. While other subsystems handle specific tasks like metadata extraction or semantic analysis, the daemon orchestrates these components into a cohesive, continuously-running system. It enables the core value proposition of Agentic Memorizer: AI agents that start with instant, comprehensive awareness of a curated knowledge base.

## Design Principles

### Separation of Concerns

The daemon strictly separates background processing from user interaction and index consumption. The daemon handles all expensive operations asynchronously, CLI commands provide user control interfaces, and the read command consumes the precomputed index independently. This separation enables each component to be optimized for its specific role and allows the read operation to succeed even if the daemon is temporarily unavailable.

### Precomputation Over On-Demand

All computationally expensive operations happen in the background during idle time rather than during agent session initialization. File analysis, semantic understanding, and index generation occur continuously as files change. This design choice ensures that agent frameworks experience sub-100ms startup times regardless of the size of the knowledge base.

### Crash Recovery and Atomicity

The daemon uses atomic file operations and crash recovery mechanisms throughout. Index writes use a temporary file plus atomic rename pattern to prevent corruption. On startup, the daemon attempts to load any existing index, providing continuity even after unexpected termination. PID files are validated to detect and clean up stale state automatically.

### Configurable Behavior

All operational parameters are exposed through configuration rather than hard-coded. Worker count, rate limits, rebuild intervals, debounce timings, and logging levels can all be adjusted without code changes. This enables tuning for different use cases, from small personal knowledge bases to large team repositories.

### Observability and Monitoring

The daemon exposes comprehensive health metrics and structured logging to enable operational visibility. Health checks provide real-time status information including uptime, processing statistics, error counts, and system state. Structured logging captures all significant events with appropriate context for debugging and auditing.

### Framework Agnosticism

The daemon produces a framework-agnostic index file that serves as a clean integration boundary. The daemon has no knowledge of Claude Code, Claude Agents, or any other consuming framework. This decoupling allows the same daemon to serve multiple frameworks simultaneously and simplifies the addition of new integrations.

## Key Components

### Core Daemon

The core daemon component serves as the primary orchestrator for the entire subsystem. It initializes all supporting components, manages their lifecycle, coordinates event processing between the file watcher and worker pool, handles signal-based control, and exposes health monitoring interfaces.

During initialization, the daemon loads configuration, creates instances of all supporting components (index manager, cache manager, metadata extractor, semantic analyzer, file watcher), and establishes signal handlers. At startup, it writes a PID file for process tracking, attempts to load any existing index for crash recovery, and performs an initial full rebuild of the index.

During runtime, the daemon processes file system events from the watcher by submitting them to the worker pool for analysis and updating the index with results. It periodically triggers full rebuilds to ensure consistency and captures comprehensive health metrics for monitoring. On shutdown, it stops the file watcher, waits for all goroutines to complete, removes the PID file, and exits cleanly.

### Worker Pool

The worker pool provides parallel file processing with rate limiting and batch priority sorting. It manages a configurable number of worker goroutines that process files concurrently, implements dual token bucket rate limiting for Claude API and embeddings API calls, and processes files based on initial priority sorting.

Each worker follows a consistent processing flow: extract metadata from the file, compute a content hash, check the cache using that hash, and either use the cached result or perform semantic analysis via the Claude API. If embeddings are enabled and analysis succeeds, an embedding vector is generated and cached separately. The worker pool tracks detailed statistics including jobs processed, cache hit rates, API calls made, embedding calls, and error counts.

Priority sorting occurs at batch submission time, with jobs sorted by modification time before entering the processing queue. Files modified within the last hour receive highest priority (100), followed by files modified in the last day (50), last week (25), and older files (10). Once sorted, jobs are processed FIFO by available workers. This ensures recent changes are analyzed first during rebuilds while maintaining efficient throughput.

### File Watcher

The file watcher component provides real-time file system monitoring using the fsnotify library. It recursively watches all directories within the memory path, debounces rapid event sequences to avoid redundant processing, and dynamically adds watchers for newly created directories.

Event debouncing uses a map-based batching mechanism where events accumulate for a configurable period before being sent for processing. This batching ensures that the last write wins for any given path, preventing redundant work when files are modified multiple times in quick succession. The debounce interval can be changed at runtime during configuration reload by signaling the watcher through a channel, causing it to recreate its internal ticker with the new interval.

The watcher filters events intelligently, skipping hidden files and directories (dot-prefixed), ignoring configurable file patterns and extensions, and handling special cases like the cache directory and `.git` version control directories. It translates file system events into appropriate actions: Create and Modify events trigger analysis, Delete events trigger index removal, and Rename events are handled as deletion of the source path (with destination handled separately as Create).

### Graph Manager

The graph manager provides persistent storage and relationship-based querying through FalkorDB, a graph database backend. It stores files as nodes with metadata and semantic analysis, creates relationship edges for tags, topics, entities, and categories, and enables semantic search across the knowledge graph.

The graph manager is initialized during daemon startup and required for daemon operation. It maintains connections to the FalkorDB Docker container, creates schema constraints for node types and relationships, and provides graceful degradation when the database is temporarily unavailable.

File operations in the graph support both single-file updates and full rebuilds. UpdateSingle adds or modifies a single file node with its relationships, returning information about whether the file was newly added or updated. RemoveFile deletes a file node and all its relationships. During rebuilds, all discovered files are written to the graph with their metadata, semantic analysis, and optional embedding vectors.

The graph manager exposes query operations for semantic search, related file discovery, entity search, and recent file queries. These operations power both the HTTP API endpoints and MCP server tools, enabling rich semantic exploration of the indexed knowledge base.

### PID Management

PID management provides process tracking and duplicate prevention through PID files. On daemon startup, it checks for existing PID files and validates whether the referenced process is still running, preventing multiple daemon instances from running simultaneously.

The PID file mechanism also enables CLI commands to interact with the running daemon by reading the PID and sending signals to that process. On shutdown, the daemon removes its PID file to indicate that it is no longer running. Stale PID files from crashed processes are automatically detected and removed during the validation check.

### Signal Handling

Signal handling enables graceful shutdown and configuration reloading through UNIX signals. The daemon registers handlers for multiple signals: SIGINT and SIGTERM trigger graceful shutdown, and SIGHUP triggers configuration reload. Index rebuilds are triggered via the HTTP API (`POST /api/v1/rebuild`) rather than signals.

When a shutdown signal is received, the daemon cancels its context to trigger goroutine exits, stops the file watcher, waits for all worker goroutines to complete their current work, and performs cleanup including PID file removal. This ensures that no work is lost and resources are properly released.

Configuration reload via SIGHUP allows hot-reloading of most operational parameters without daemon restart. When SIGHUP is received, the daemon validates the new configuration and checks for immutable field changes. Three fields are immutable during reload: memory_root (the watched directory), analysis.cache_dir (the cache storage location), and daemon.log_file (the log output path). If any immutable field has changed, the reload fails with an error message indicating a daemon restart is required.

After validation succeeds, the daemon applies hot-reloadable settings including Claude API configuration, worker pool parameters, rate limits, debounce intervals, rebuild intervals, log level, and health check port. Settings take effect immediately without requiring any downtime.

### Health Monitoring

Health monitoring provides operational visibility through metrics tracking and optional HTTP endpoints. The daemon tracks uptime, files processed, API calls made, cache hit rates, error counts, last build time and success status, index file count, and watcher active status.

Metrics are recorded during both full index rebuilds and incremental file operations. The HealthMetrics component provides thread-safe granular recording methods that are called throughout the daemon's runtime:

- `RecordFileProcessed()` - Increments the files processed counter. Called after successfully updating the index with a file entry (both new files and modifications).
- `RecordAPICall()` - Increments the API calls counter. Called after performing semantic analysis via the Claude API (cache miss).
- `RecordCacheHit()` - Increments the cache hits counter. Called when cached analysis is reused instead of making an API call (cache hit).
- `RecordError()` - Increments the error counter. Called when file processing or analysis fails.
- `IncrementIndexFileCount()` - Increments the index file count by 1. Called when a new file is added to the index (not when updating existing entries).
- `DecrementIndexFileCount()` - Decrements the index file count by 1. Called when a file is removed from the index, with protection against underflow (won't decrement below 0).

An optional HTTP health check endpoint serves this information in JSON format, enabling integration with monitoring systems. The health status is computed based on recent build success and error rates, allowing automated detection of degraded daemon operation.

The health server can be disabled by setting the health check port to 0 in configuration. When the port changes during configuration reload, the daemon automatically stops the existing health server and starts a new one on the updated port. This hot-restart capability allows operational teams to reconfigure monitoring endpoints without full daemon restarts.

### SSE Notification Hub

The SSE notification hub provides real-time index update notifications to connected MCP server instances via Server-Sent Events. This enables MCP servers to reload their indexes immediately when changes occur, rather than polling or waiting for manual refresh.

The hub exposes SSE event streams at the `/sse` endpoint. The SSE stream delivers notifications in standard SSE format with JSON-formatted event data containing the event type, timestamp, and complete graph index. Two event types are sent: `index_snapshot` on initial connection and `index_updated` for all subsequent changes (both single-file updates and full rebuilds send the same event type with the complete updated index).

The hub supports unlimited concurrent MCP server connections. Each connection receives all broadcast notifications until it disconnects. Client connections are tracked in a thread-safe map, and the `/health` endpoint reports the current count of connected clients for operational monitoring.

Keepalive comments are sent every 30 seconds to prevent connection timeouts and enable detection of disconnected clients. The hub uses a non-blocking broadcast strategy where slow clients that cannot keep up with the notification rate will have notifications dropped rather than blocking other clients.

The SSE hub is exposed via the daemon's unified HTTP server, controlled by the `http_port` setting. Setting the port to 0 disables the HTTP server entirely. The server supports hot-reload: when the port changes during configuration reload, the server shuts down gracefully and restarts on the updated port.

Lifecycle management follows a unified approach. The HTTP server (which provides both health check and SSE endpoints) starts during daemon initialization if enabled. During shutdown, the server stops gracefully, allowing clients to disconnect cleanly.

Notifications are broadcast by the daemon after successful index writes. The `BroadcastIndexUpdate()` method is called immediately following `WriteAtomic()` in both the event processing loop (single file updates) and periodic rebuild loop (full rebuilds). This ensures MCP servers receive notifications for all index changes, enabling real-time synchronization.

Implementation is located in `internal/daemon/sse_hub.go`, with integration points in `internal/daemon/daemon.go` for lifecycle management and broadcast calls.

### Embeddings System

The embeddings system provides optional vector embedding generation for file content, enabling future vector similarity search capabilities. When enabled via configuration, the system generates embedding vectors for successfully analyzed files and caches them separately from semantic analysis results.

The embeddings provider supports OpenAI's text embedding models with configurable dimensions. Embedding generation occurs after semantic analysis succeeds and only if the file has a non-empty summary. The worker pool maintains a separate rate limiter for embedding API calls (hardcoded at 500 RPM) to prevent embedding generation from affecting Claude API quota.

Embeddings are cached independently using content hash as the key, stored in a separate cache directory. During graph updates, embeddings are included if available via UpdateSingleWithEmbedding. The embedding system is designed for future extensibility but currently stores vectors without utilizing them for search operations.

### Service Manager Integration

The daemon follows modern Go best practices by running in foreground mode and delegating process supervision to external service managers. This design avoids self-daemonization anti-patterns in favor of battle-tested tools like systemd and launchd.

The daemon provides two commands for generating service manager configuration files. The `daemon systemctl` command generates systemd unit files for Linux systems, while the `daemon launchctl` command generates launchd plist files for macOS. Both commands detect the binary path automatically and create user-level and system-level configuration templates.

When running under systemd, the daemon implements the Type=notify protocol by sending a readiness notification after the health server starts. This integration ensures systemd knows when the daemon is fully operational and ready to handle requests. The notification occurs via the go-systemd library's SdNotify function.

Service managers handle backgrounding, automatic restarts on failure, and logging integration with system facilities like journald or Console.app. This approach provides production-grade process supervision without complex daemonization code in the application itself.

## Integration Points

### Configuration System

The daemon reads configuration at startup from the system configuration file. Configuration options control all aspects of daemon behavior including whether the daemon is enabled, event debounce timing, worker pool size, API rate limits, full rebuild interval, health check endpoint port, log file location, and log level.

Most configuration changes can be applied without daemon restart using the `config reload` command or by sending SIGHUP to the daemon process. Hot-reloadable settings include Claude API parameters, worker pool size, rate limits, debounce intervals, log level, health check port, rebuild intervals, and skip patterns. Structural settings that determine process architecture (memory_root, cache_dir, log_file) require a daemon restart.

When the rebuild interval changes during configuration reload, the daemon signals the periodic rebuild loop through a channel. The loop responds by stopping its current ticker and creating a new one with the updated interval. If the interval is set to 0 or negative, periodic rebuilds are disabled entirely, and the loop exits until the daemon restarts or receives a new non-zero interval. This design provides operational flexibility while maintaining system integrity.

### Cache System

The worker pool integrates with the cache system to avoid redundant semantic analysis. Before analyzing a file, the worker computes a content hash and queries the cache. If a cached analysis exists and is still fresh, it is used directly without calling the Claude API.

Cache entries are keyed by content hash rather than file path, meaning that if a file's content is unchanged, the cache remains valid even if the file is moved or renamed. This approach maximizes cache effectiveness and minimizes unnecessary API usage.

### Metadata Extractor

The worker pool uses the metadata extractor to gather structured information about files before semantic analysis. The metadata extractor supports a wide variety of file types including text formats, programming languages, images, office documents, and media files.

Metadata extraction always succeeds even if semantic analysis fails, ensuring that basic file information is always available in the index. The extracted metadata provides context for semantic analysis and enables filtering and searching based on structural properties.

### Semantic Analyzer

The worker pool invokes the semantic analyzer to generate AI-powered summaries and semantic tags for files. The semantic analyzer uses the Claude API with model-specific capabilities including text analysis for text files, vision analysis for images using multimodal capabilities, document analysis for office formats, and PDF analysis using document content blocks.

Semantic analysis is rate-limited to stay within API quotas and avoid excessive costs. The analyzer produces structured output including a concise summary, semantic tags, key topics, document type classification, and confidence scores.

### Directory Walker

The daemon uses the directory walker during full index rebuilds to traverse the entire memory directory tree. The walker recursively collects all file paths while respecting skip patterns for hidden directories, version control directories, and the cache directory.

The walker returns relative paths from the memory directory root, which are then submitted to the worker pool for analysis. This separation allows the walker to focus purely on file system traversal while the worker pool handles the more complex processing logic.

### Graph System

The daemon integrates deeply with the graph system for persistent storage and semantic querying. All processed files are stored as nodes in FalkorDB with their metadata, semantic analysis, and relationship edges.

The graph manager is initialized during daemon startup and must be available for the daemon to operate. If initialization fails, the daemon will not start. During runtime, the graph provides graceful degradation when temporarily unavailable, logging warnings but continuing to accept new files.

File updates flow through UpdateSingle for incremental changes and UpdateSingleWithEmbedding when embedding vectors are available. Full rebuilds write all discovered files to the graph in batch. The daemon queries the graph for statistics during startup to detect existing data and determine whether to continue with stale data if the initial rebuild fails.

### CLI Commands

CLI commands provide the user interface for daemon control. The daemon supports eight subcommands: start, stop, status, restart, rebuild, logs, systemctl, and launchctl.

The start command launches the daemon process. The stop command sends SIGTERM to gracefully shut down. The status command checks daemon state and displays index information. The restart command performs stop followed by start. The rebuild command triggers an immediate rebuild via the HTTP API (`POST /api/v1/rebuild`), with an optional `--force` flag to clear the graph before rebuilding. The logs command displays daemon log output with optional follow mode.

The systemctl command generates systemd unit files for Linux service manager integration. The launchctl command generates launchd plist files for macOS service manager integration. Both commands detect the binary path automatically and create user-level and system-level configuration templates.

The config reload command validates and applies configuration changes by sending SIGHUP to the running daemon. Most commands interact with the daemon through the PID file and signal-based communication, while the rebuild command uses the HTTP API for richer feedback and options.

### Read Command

The read command provides backwards compatibility for integration adapters that use precomputed index files. It queries the graph database to export the complete index, formats it according to the requested output format and integration wrapper, and outputs the result to stdout for consumption by agent frameworks.

The read command requires the graph database to be available. It exports the complete index from FalkorDB using the graph manager's export functionality, then applies formatting and integration-specific wrapping. This design maintains the same interface as the legacy index file approach while leveraging the graph's persistent storage.

### MCP Server Integration

The daemon integrates with MCP server instances through the SSE notification hub, enabling real-time index synchronization for MCP-enabled AI tools. MCP servers connect to the daemon's SSE stream endpoint and receive notifications whenever the index changes.

When an MCP server starts, it connects to `http://localhost:{http_port}/sse` if the HTTP server is enabled. The connection uses standard HTTP with `Accept: text/event-stream` header and remains open for the duration of the MCP server's lifecycle. Multiple MCP servers can connect simultaneously, each maintaining its own independent stream.

The daemon broadcasts notifications to all connected MCP servers immediately after successful graph updates. After processing a file event through the worker pool, the daemon calls `graphManager.UpdateSingle()` to persist the updated file node and relationships, then calls `sseHub.BroadcastIndexUpdate()` which sends an `index_updated` event with the complete graph index. Full rebuilds trigger the same `index_updated` event type after all files are written to the graph.

MCP servers receive these notifications as SSE events with JSON payloads containing `type` (either `index_snapshot` on initial connection or `index_updated` for changes), `timestamp`, and the complete `GraphIndex` structure in the `data` field. Upon receiving an `index_updated` notification, an MCP server reloads its in-memory index by querying the graph manager's export functionality, then sends JSON-RPC notifications to its connected clients (e.g., Claude Code) informing them that resources have changed. This cascading notification chain ensures AI tools can react immediately to index changes.

The integration supports configuration hot-reload. When the `http_port` setting changes during daemon configuration reload, the HTTP server shuts down gracefully (disconnecting all MCP servers from the SSE stream), and restarts on the updated port. MCP servers detect the disconnection, wait with exponential backoff, and automatically reconnect when the server becomes available again.

Error handling follows a resilient design. If no MCP servers are connected, broadcasts are no-ops with no performance impact. If a broadcast fails to a specific client (slow consumer, network issue), that client is removed from the active connection pool without affecting other clients. The daemon continues operating normally whether MCP servers are connected or not, maintaining the principle that MCP integration is an optional enhancement rather than a core dependency.

Implementation touchpoints include: `internal/daemon/daemon.go` (lifecycle management, broadcast calls after index writes), `internal/daemon/sse_hub.go` (SSE server and client management), `internal/mcp/server.go` (MCP server integration), and `internal/mcp/sse_client.go` (SSE client with auto-reconnect).

## Lifecycle Management

### Startup Sequence

Daemon startup follows a carefully orchestrated sequence to ensure proper initialization. Configuration is loaded and validated, logging is configured with appropriate output destinations and levels, and all component instances are created in dependency order.

The graph manager is initialized early in the startup sequence, establishing a connection to FalkorDB and creating schema constraints. If graph initialization fails, the daemon will not start. Once the graph is initialized, the daemon checks for existing data in the graph to determine whether to continue with stale data if the initial rebuild encounters errors.

Process management begins with checking for an existing daemon through PID file validation. If no daemon is running, a new PID file is written. Signal handlers are registered for graceful shutdown and operational commands.

The daemon then performs an initial full rebuild to ensure the graph is current, writing all discovered files as nodes with their metadata, semantic analysis, and relationships. After the rebuild completes, the file watcher starts monitoring for changes, background goroutines launch for event processing and periodic rebuilds, and the HTTP server starts (if `http_port > 0`) providing health check and SSE notification endpoints. For systemd environments, a readiness notification is sent after the HTTP server starts. Finally, the daemon logs successful startup and blocks waiting for context cancellation.

### Runtime Operations

During normal operation, the daemon runs several concurrent processes. The event processing loop receives events from the file watcher, submits them to the worker pool for metadata extraction and semantic analysis, collects results including optional embeddings, writes file nodes and relationships to the graph via `UpdateSingle()` or `UpdateSingleWithEmbedding()`, and broadcasts an `index_updated` notification to connected MCP servers via the SSE hub (if enabled).

Throughout event processing, health metrics are recorded to track operational activity. When a file is processed, the daemon records cache hits (`RecordCacheHit()`) when cached analysis is reused, API calls (`RecordAPICall()`) when semantic analysis is performed, file processing (`RecordFileProcessed()`) after successful graph update, and index file count changes (`IncrementIndexFileCount()` for new files, `DecrementIndexFileCount()` for deletions). This granular tracking ensures metrics accurately reflect both full rebuilds and incremental updates.

The periodic rebuild loop triggers full directory walks at configured intervals, submits all discovered files to the worker pool with priority sorting (recent files first), collects results with metadata and semantic analysis, writes all file nodes to the graph in batch, calculates statistics from worker pool and graph, and broadcasts an `index_updated` notification to connected MCP servers.

Both loops operate concurrently and handle context cancellation to enable clean shutdown. The worker pool processes jobs continuously, respecting dual rate limits for Claude API and embeddings API, updating statistics as work completes. The SSE hub maintains connections to MCP servers, delivering notifications with the complete graph index and keepalive messages independently of the main processing loops.

### Shutdown Sequence

Graceful shutdown is triggered by receiving SIGINT or SIGTERM signals. The daemon cancels its context to signal all goroutines to exit, stops the HTTP server (which includes both health check and SSE endpoints) to disconnect MCP servers gracefully, stops the file watcher to prevent new events from being generated, and waits for all worker goroutines to complete their current jobs using a WaitGroup.

After all background work completes, the daemon closes the graph manager connection to FalkorDB and removes its PID file to indicate it is no longer running. The daemon logs the shutdown completion and exits. This sequence ensures no work is lost, all network connections (HTTP and graph) are closed properly, resources are released, and the system is left in a clean state.

## Future Enhancements

No major enhancements are currently planned for the daemon subsystem. The core architecture is stable and feature-complete for the intended use cases.

Potential minor improvements that could be considered in the future include file-based configuration watching (automatic reload on config file change without manual signal), metrics export to Prometheus or similar systems, and distributed operation for multi-machine knowledge bases.

## Glossary

**Atomic Operation**: An operation that either completes fully or has no effect, preventing partial or inconsistent state. Used extensively in index writes and PID file management.

**Content Hash**: A SHA-256 hash of file contents used as a cache key. Enables cache hits even when files are moved or renamed as long as content is unchanged.

**Debouncing**: The practice of delaying action until a burst of events has settled. Prevents redundant processing when files are modified multiple times in rapid succession.

**Event Loop**: A programming construct that continuously waits for and processes events. The daemon uses event loops for file system events and periodic rebuild triggers.

**Embedding Vector**: A high-dimensional numerical representation of file content generated by embedding models, enabling future vector similarity search capabilities.

**Framework Integration**: An adapter that formats the graph index export for consumption by a specific agent framework like Claude Code or Gemini CLI.

**Graph Node**: A vertex in the FalkorDB knowledge graph representing a file, tag, topic, entity, or category with associated properties.

**Graph Relationship**: An edge in the FalkorDB knowledge graph connecting nodes (e.g., HAS_TAG, COVERS_TOPIC, MENTIONS, IN_CATEGORY).

**Health Metrics**: Operational statistics tracked by the daemon including uptime, processing counts, error rates, cache statistics, and system state.

**PID File**: A file containing the process ID of a running daemon, used for process tracking and duplicate prevention.

**Priority Sorting**: Ordering jobs by priority before submission to workers. The worker pool sorts batches by file modification time, then processes jobs FIFO.

**Rate Limiting**: Controlling the rate at which operations occur to stay within quotas or avoid overwhelming external services. The daemon uses dual rate limiters for Claude API calls and embeddings API calls.

**Semantic Analysis**: AI-powered understanding of file contents including summarization, tagging, and topic extraction.

**Service Manager Integration**: Integration with operating system service managers like systemd and launchd for process supervision. The daemon generates service configuration files and implements readiness protocols.

**Signal Handler**: Code that executes in response to UNIX signals like SIGINT or SIGTERM. Enables graceful shutdown and operational commands.

**Token Bucket**: A rate limiting algorithm that allows bursts up to a capacity while maintaining an average rate. Used for Claude API call throttling.

**Type=notify**: A systemd service type where the daemon signals readiness after initialization. The daemon sends a notification to systemd when fully operational, ensuring proper service ordering and health tracking.

**Worker Pool**: A collection of goroutines that process jobs concurrently with a shared job queue and result collection.
