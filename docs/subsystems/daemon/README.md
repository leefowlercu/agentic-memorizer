# Daemon Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
3. [Key Components](#key-components)
   - [Core Daemon](#core-daemon)
   - [Worker Pool](#worker-pool)
   - [File Watcher](#file-watcher)
   - [Index Manager](#index-manager)
   - [PID Management](#pid-management)
   - [Signal Handling](#signal-handling)
   - [Health Monitoring](#health-monitoring)
   - [SSE Notification Hub](#sse-notification-hub)
4. [Integration Points](#integration-points)
   - [Configuration System](#configuration-system)
   - [Cache System](#cache-system)
   - [Metadata Extractor](#metadata-extractor)
   - [Semantic Analyzer](#semantic-analyzer)
   - [Directory Walker](#directory-walker)
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

The worker pool provides parallel file processing with rate limiting and priority-based scheduling. It manages a configurable number of worker goroutines that process files concurrently, implements token bucket rate limiting for API calls, and prioritizes recently modified files for processing.

Each worker follows a consistent processing flow: extract metadata from the file, compute a content hash, check the cache using that hash, and either use the cached result or perform semantic analysis via the Claude API. The worker pool tracks detailed statistics including jobs processed, cache hit rates, API calls made, and error counts.

Priority calculation considers file modification time, giving highest priority to files modified in the last hour, followed by files modified in the last day, week, and older files respectively. This ensures that recent changes are reflected in the index quickly while still processing all files eventually.

### File Watcher

The file watcher component provides real-time file system monitoring using the fsnotify library. It recursively watches all directories within the memory path, debounces rapid event sequences to avoid redundant processing, and dynamically adds watchers for newly created directories.

Event debouncing uses a map-based batching mechanism where events accumulate for a configurable period before being sent for processing. This batching ensures that the last write wins for any given path, preventing redundant work when files are modified multiple times in quick succession. The debounce interval can be changed at runtime during configuration reload by signaling the watcher through a channel, causing it to recreate its internal ticker with the new interval.

The watcher filters events intelligently, skipping hidden files and directories, ignoring configurable file patterns, and handling special cases like the cache directory and version control directories. It translates file system events into appropriate actions: Create and Modify events trigger analysis, Delete events trigger index removal, and Rename events are handled as deletion followed by creation.

### Index Manager

The index manager provides thread-safe management of the precomputed index file. It maintains an in-memory representation of the index, coordinates atomic writes to disk, enables incremental updates for individual files, and supports full index replacement during rebuilds.

The atomic write pattern ensures index integrity by writing to a temporary file, syncing to disk to ensure durability, and atomically renaming to the final location. This guarantees that the index file is never in a partially-written state, even if the daemon crashes during a write operation.

The index manager supports both bulk operations during full rebuilds and incremental operations for individual file changes. Incremental updates modify the in-memory index and immediately persist the change atomically, ensuring that file changes are reflected in the index with minimal delay.

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

The hub exposes two HTTP endpoints: `/notifications/stream` for the SSE event stream and `/health` for monitoring connected clients. The SSE stream endpoint delivers notifications in standard SSE format with event type "notification" and JSON-formatted data containing the notification type (`index_updated` for single file changes, `index_rebuilt` for full rebuilds), timestamp, and optional metadata (file path for updates, total file count for rebuilds).

The hub supports unlimited concurrent MCP server connections. Each connection receives all broadcast notifications until it disconnects. Client connections are tracked in a thread-safe map, and the `/health` endpoint reports the current count of connected clients for operational monitoring.

Keepalive comments are sent every 30 seconds to prevent connection timeouts and enable detection of disconnected clients. The hub uses a non-blocking broadcast strategy where slow clients that cannot keep up with the notification rate will have notifications dropped rather than blocking other clients.

The SSE hub is exposed via the daemon's unified HTTP server, controlled by the `http_port` setting. Setting the port to 0 disables the HTTP server entirely. The server supports hot-reload: when the port changes during configuration reload, the server shuts down gracefully and restarts on the updated port.

Lifecycle management follows a unified approach. The HTTP server (which provides both health check and SSE endpoints) starts during daemon initialization if enabled. During shutdown, the server stops gracefully, allowing clients to disconnect cleanly.

Notifications are broadcast by the daemon after successful index writes. The `BroadcastIndexUpdate()` method is called immediately following `WriteAtomic()` in both the event processing loop (single file updates) and periodic rebuild loop (full rebuilds). This ensures MCP servers receive notifications for all index changes, enabling real-time synchronization.

Implementation is located in `internal/daemon/sse_hub.go`, with integration points in `internal/daemon/daemon.go` for lifecycle management and broadcast calls.

### Service Manager Integration

The daemon follows modern Go best practices by running in foreground mode and delegating process supervision to external service managers. This design avoids self-daemonization anti-patterns in favor of battle-tested tools like systemd and launchd.

The daemon provides two commands for generating service manager configuration files. The `systemctl` command generates systemd unit files for Linux systems, while the `launchctl` command generates launchd plist files for macOS. Both commands detect the binary path automatically and create user-level and system-level configuration templates.

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

### CLI Commands

CLI commands provide the user interface for daemon control. The daemon supports eight subcommands: start, stop, status, restart, rebuild, logs, systemctl, and launchctl.

The start command launches the daemon process. The stop command sends SIGTERM to gracefully shut down. The status command checks daemon state and displays index information. The restart command performs stop followed by start. The rebuild command triggers an immediate rebuild via the HTTP API (`POST /api/v1/rebuild`), with an optional `--force` flag to clear the graph before rebuilding. The logs command displays daemon log output with optional follow mode.

The systemctl command generates systemd unit files for Linux service manager integration. The launchctl command generates launchd plist files for macOS service manager integration. Both commands detect the binary path automatically and create user-level and system-level configuration templates.

The config reload command validates and applies configuration changes by sending SIGHUP to the running daemon. Most commands interact with the daemon through the PID file and signal-based communication, while the rebuild command uses the HTTP API for richer feedback and options.

### Read Command

The read command represents the primary consumption interface for the daemon's output. It loads the precomputed index file, formats it according to the requested output format and integration wrapper, and outputs the result to stdout for consumption by agent frameworks.

Critically, the read command is daemon-independent. It directly reads the index file without any communication with the daemon process. This design provides fast read operations, resilience to daemon unavailability, and simplicity in the integration interface.

### MCP Server Integration

The daemon integrates with MCP server instances through the SSE notification hub, enabling real-time index synchronization for MCP-enabled AI tools. MCP servers connect to the daemon's SSE stream endpoint and receive notifications whenever the index changes.

When an MCP server starts, it connects to `http://localhost:{http_port}/notifications/stream` if the HTTP server is enabled. The connection uses standard HTTP with `Accept: text/event-stream` header and remains open for the duration of the MCP server's lifecycle. Multiple MCP servers can connect simultaneously, each maintaining its own independent stream.

The daemon broadcasts notifications to all connected MCP servers immediately after successful index writes. After processing a file event through the worker pool, the daemon calls `index.Manager.WriteAtomic()` to persist the updated index, then calls `sseHub.BroadcastIndexUpdate()` with notification type `index_updated` and the affected file path. Similarly, after completing a full rebuild, the daemon broadcasts type `index_rebuilt` with the total file count.

MCP servers receive these notifications as SSE events with JSON payloads containing `type`, `timestamp`, and optional metadata fields. Upon receiving a notification, an MCP server reloads its in-memory index from disk via `index.Manager.LoadComputed()`, then sends JSON-RPC notifications to its connected clients (e.g., Claude Code) informing them that resources have changed. This cascading notification chain ensures AI tools can react immediately to index changes.

The integration supports configuration hot-reload. When the `http_port` setting changes during daemon configuration reload, the HTTP server shuts down gracefully (disconnecting all MCP servers from the SSE stream), and restarts on the updated port. MCP servers detect the disconnection, wait with exponential backoff, and automatically reconnect when the server becomes available again.

Error handling follows a resilient design. If no MCP servers are connected, broadcasts are no-ops with no performance impact. If a broadcast fails to a specific client (slow consumer, network issue), that client is removed from the active connection pool without affecting other clients. The daemon continues operating normally whether MCP servers are connected or not, maintaining the principle that MCP integration is an optional enhancement rather than a core dependency.

Implementation touchpoints include: `internal/daemon/daemon.go` (lifecycle management, broadcast calls after index writes), `internal/daemon/sse_hub.go` (SSE server and client management), `internal/mcp/server.go` (MCP server integration), and `internal/mcp/sse_client.go` (SSE client with auto-reconnect).

## Lifecycle Management

### Startup Sequence

Daemon startup follows a carefully orchestrated sequence to ensure proper initialization. Configuration is loaded and validated, logging is configured with appropriate output destinations and levels, and all component instances are created in dependency order.

Process management begins with checking for an existing daemon through PID file validation. If no daemon is running, a new PID file is written. Signal handlers are registered for graceful shutdown and operational commands. Crash recovery is attempted by loading any existing index file.

The daemon then performs an initial full rebuild to ensure the index is current, starts the file watcher to begin monitoring for changes, launches background goroutines for event processing and periodic rebuilds, and optionally starts the HTTP server (if `http_port > 0`) which provides both health check and SSE notification endpoints. Finally, it logs successful startup and blocks waiting for the context to be cancelled.

### Runtime Operations

During normal operation, the daemon runs several concurrent processes. The event processing loop receives events from the file watcher, submits them to the worker pool, collects results, updates the in-memory index, atomically writes the updated index to disk, and broadcasts an `index_updated` notification to connected MCP servers via the SSE hub (if enabled).

Throughout event processing, health metrics are recorded to track operational activity. When a file is processed, the daemon records cache hits (`RecordCacheHit()`) when cached analysis is reused, API calls (`RecordAPICall()`) when semantic analysis is performed, file processing (`RecordFileProcessed()`) after successful index update, and index file count changes (`IncrementIndexFileCount()` for new files, `DecrementIndexFileCount()` for deletions). This granular tracking ensures metrics accurately reflect both full rebuilds and incremental updates.

The periodic rebuild loop triggers full directory walks at configured intervals, submits all discovered files to the worker pool with priority sorting, collects results, builds a complete index structure, calculates statistics, atomically replaces the index file, and broadcasts an `index_rebuilt` notification to connected MCP servers.

Both loops operate concurrently and handle context cancellation to enable clean shutdown. The worker pool processes jobs continuously, respecting rate limits and updating statistics as work completes. The SSE hub maintains connections to MCP servers, delivering notifications and keepalive messages independently of the main processing loops.

### Shutdown Sequence

Graceful shutdown is triggered by receiving SIGINT or SIGTERM signals. The daemon cancels its context to signal all goroutines to exit, stops the file watcher to prevent new events from being generated, stops the SSE notification hub (if enabled) to disconnect MCP servers gracefully, and waits for all worker goroutines to complete their current jobs using a WaitGroup.

After all background work completes, the daemon stops the health check HTTP server and removes its PID file to indicate it is no longer running. The daemon logs the shutdown completion and exits. This sequence ensures no work is lost, all network connections are closed properly, resources are released, and the system is left in a clean state.

## Future Enhancements

No major enhancements are currently planned for the daemon subsystem. The core architecture is stable and feature-complete for the intended use cases.

Potential minor improvements that could be considered in the future include file-based configuration watching (automatic reload on config file change without manual signal), metrics export to Prometheus or similar systems, and distributed operation for multi-machine knowledge bases.

## Glossary

**Atomic Operation**: An operation that either completes fully or has no effect, preventing partial or inconsistent state. Used extensively in index writes and PID file management.

**Content Hash**: A SHA-256 hash of file contents used as a cache key. Enables cache hits even when files are moved or renamed as long as content is unchanged.

**Debouncing**: The practice of delaying action until a burst of events has settled. Prevents redundant processing when files are modified multiple times in rapid succession.

**Event Loop**: A programming construct that continuously waits for and processes events. The daemon uses event loops for file system events and periodic rebuild triggers.

**Framework Integration**: An adapter that formats the precomputed index for consumption by a specific agent framework like Claude Code or Claude Agents.

**Health Metrics**: Operational statistics tracked by the daemon including uptime, processing counts, error rates, and system state.

**Index Manager**: The component responsible for maintaining and persisting the precomputed index with thread-safety and atomicity guarantees.

**PID File**: A file containing the process ID of a running daemon, used for process tracking and duplicate prevention.

**Precomputed Index**: An index file that contains all semantic analysis results and metadata, maintained continuously by the daemon for instant loading by agent frameworks.

**Priority Queue**: A queue where items are processed in priority order rather than insertion order. The worker pool uses priorities based on file modification time.

**Rate Limiting**: Controlling the rate at which operations occur to stay within quotas or avoid overwhelming external services. The daemon rate-limits Claude API calls.

**Semantic Analysis**: AI-powered understanding of file contents including summarization, tagging, and topic extraction.

**Service Manager Integration**: Integration with operating system service managers like systemd and launchd for process supervision. The daemon generates service configuration files and implements readiness protocols.

**Signal Handler**: Code that executes in response to UNIX signals like SIGINT or SIGTERM. Enables graceful shutdown and operational commands.

**Token Bucket**: A rate limiting algorithm that allows bursts up to a capacity while maintaining an average rate. Used for Claude API call throttling.

**Type=notify**: A systemd service type where the daemon signals readiness after initialization. The daemon sends a notification to systemd when fully operational, ensuring proper service ordering and health tracking.

**Worker Pool**: A collection of goroutines that process jobs concurrently with a shared job queue and result collection.
