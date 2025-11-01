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
4. [Integration Points](#integration-points)
   - [Configuration System](#configuration-system)
   - [Cache System](#cache-system)
   - [Metadata Extractor](#metadata-extractor)
   - [Semantic Analyzer](#semantic-analyzer)
   - [Directory Walker](#directory-walker)
   - [CLI Commands](#cli-commands)
   - [Read Command](#read-command)
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

Event debouncing uses a map-based batching mechanism where events accumulate for a configurable period before being sent for processing. This batching ensures that the last write wins for any given path, preventing redundant work when files are modified multiple times in quick succession.

The watcher filters events intelligently, skipping hidden files and directories, ignoring configurable file patterns, and handling special cases like the cache directory and version control directories. It translates file system events into appropriate actions: Create and Modify events trigger analysis, Delete events trigger index removal, and Rename events are handled as deletion followed by creation.

### Index Manager

The index manager provides thread-safe management of the precomputed index file. It maintains an in-memory representation of the index, coordinates atomic writes to disk, enables incremental updates for individual files, and supports full index replacement during rebuilds.

The atomic write pattern ensures index integrity by writing to a temporary file, syncing to disk to ensure durability, and atomically renaming to the final location. This guarantees that the index file is never in a partially-written state, even if the daemon crashes during a write operation.

The index manager supports both bulk operations during full rebuilds and incremental operations for individual file changes. Incremental updates modify the in-memory index and immediately persist the change atomically, ensuring that file changes are reflected in the index with minimal delay.

### PID Management

PID management provides process tracking and duplicate prevention through PID files. On daemon startup, it checks for existing PID files and validates whether the referenced process is still running, preventing multiple daemon instances from running simultaneously.

The PID file mechanism also enables CLI commands to interact with the running daemon by reading the PID and sending signals to that process. On shutdown, the daemon removes its PID file to indicate that it is no longer running. Stale PID files from crashed processes are automatically detected and removed during the validation check.

### Signal Handling

Signal handling enables graceful shutdown and operational commands through UNIX signals. The daemon registers handlers for SIGINT and SIGTERM to trigger graceful shutdown, and SIGUSR1 to trigger an immediate index rebuild.

When a shutdown signal is received, the daemon cancels its context to trigger goroutine exits, stops the file watcher, waits for all worker goroutines to complete their current work, and performs cleanup including PID file removal. This ensures that no work is lost and resources are properly released.

### Health Monitoring

Health monitoring provides operational visibility through metrics tracking and optional HTTP endpoints. The daemon tracks uptime, files processed, API calls made, cache hit rates, error counts, last build time and success status, index file count, and watcher active status.

An optional HTTP health check endpoint serves this information in JSON format, enabling integration with monitoring systems. The health status is computed based on recent build success and error rates, allowing automated detection of degraded daemon operation.

## Integration Points

### Configuration System

The daemon reads configuration at startup from the system configuration file. Configuration options control all aspects of daemon behavior including whether the daemon is enabled, event debounce timing, worker pool size, API rate limits, full rebuild interval, health check endpoint port, log file location, and log level.

Changes to configuration require a daemon restart to take effect. This design choice simplifies configuration management and avoids complex runtime reconfiguration logic while still providing comprehensive control over daemon behavior.

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

CLI commands provide the user interface for daemon control. The start command launches the daemon process, the stop command sends SIGTERM to gracefully shut down, the status command checks daemon state and displays index information, the restart command performs stop followed by start, the rebuild command sends SIGUSR1 to trigger an immediate rebuild, and the logs command displays daemon log output with optional follow mode.

These commands interact with the daemon through the PID file and signal-based communication rather than direct API calls. This design keeps the CLI lightweight and ensures that commands can execute quickly without waiting for the daemon to process requests.

### Read Command

The read command represents the primary consumption interface for the daemon's output. It loads the precomputed index file, formats it according to the requested output format and integration wrapper, and outputs the result to stdout for consumption by agent frameworks.

Critically, the read command is daemon-independent. It directly reads the index file without any communication with the daemon process. This design provides fast read operations, resilience to daemon unavailability, and simplicity in the integration interface.

## Lifecycle Management

### Startup Sequence

Daemon startup follows a carefully orchestrated sequence to ensure proper initialization. Configuration is loaded and validated, logging is configured with appropriate output destinations and levels, and all component instances are created in dependency order.

Process management begins with checking for an existing daemon through PID file validation. If no daemon is running, a new PID file is written. Signal handlers are registered for graceful shutdown and operational commands. Crash recovery is attempted by loading any existing index file.

The daemon then performs an initial full rebuild to ensure the index is current, starts the file watcher to begin monitoring for changes, launches background goroutines for event processing and periodic rebuilds, and optionally starts the health check HTTP server. Finally, it logs successful startup and blocks waiting for the context to be cancelled.

### Runtime Operations

During normal operation, the daemon runs several concurrent processes. The event processing loop receives events from the file watcher, submits them to the worker pool, collects results, updates the in-memory index, and atomically writes the updated index to disk.

The periodic rebuild loop triggers full directory walks at configured intervals, submits all discovered files to the worker pool with priority sorting, collects results, builds a complete index structure, calculates statistics, and atomically replaces the index file.

Both loops operate concurrently and handle context cancellation to enable clean shutdown. The worker pool processes jobs continuously, respecting rate limits and updating statistics as work completes.

### Shutdown Sequence

Graceful shutdown is triggered by receiving SIGINT or SIGTERM signals. The daemon cancels its context to signal all goroutines to exit, stops the file watcher to prevent new events from being generated, and waits for all worker goroutines to complete their current jobs using a WaitGroup.

After all background work completes, the daemon removes its PID file to indicate it is no longer running, logs the shutdown completion, and exits. This sequence ensures no work is lost, resources are properly released, and the system is left in a clean state.

## Future Enhancements

### Configuration Reload via SIGHUP

A planned enhancement to the daemon subsystem is runtime configuration reloading through the SIGHUP signal. This feature would enable operators to modify daemon configuration without requiring a full restart, reducing downtime and improving operational flexibility.

#### Proposed Behavior

When the daemon receives a SIGHUP signal, it would reload its configuration file and apply changes to runtime parameters where possible. This would allow tuning of operational settings in response to changing workload characteristics or resource availability without interrupting the daemon's operation.

#### Reloadable Configuration

Not all configuration parameters can be safely changed at runtime. The proposed implementation would support reloading of operational parameters while requiring restart for structural changes:

**Safely Reloadable**:
- Worker pool size (dynamically adjust concurrency)
- API rate limits (tune based on quota usage)
- Full rebuild interval (adjust rebuild frequency)
- Debounce timing (tune event batching)
- Log level (increase verbosity for debugging)
- Health check port (enable/disable monitoring endpoint)

**Requires Restart**:
- Memory directory path (structural change)
- Daemon enable/disable (lifecycle change)
- Log file path (resource change)
- Integration settings (not used by daemon)

#### Implementation Considerations

The implementation would need to address several technical challenges:

**Worker Pool Adjustment**: Dynamically adding or removing workers requires careful coordination to avoid disrupting in-flight jobs. New workers can be added immediately, while worker reduction should wait for current jobs to complete.

**Rate Limit Updates**: The token bucket rate limiter can be updated atomically by replacing the limiter instance, with worker goroutines transparently using the new limits for subsequent API calls.

**Rebuild Interval Changes**: The periodic rebuild ticker can be stopped and recreated with new intervals. Any in-progress rebuild should complete before the new interval takes effect.

**Configuration Validation**: Before applying configuration changes, the new configuration must be validated to ensure it contains valid values. If validation fails, the daemon should log an error and continue operating with the existing configuration.

**Atomic Updates**: Configuration changes should be applied atomically to avoid inconsistent state where some components use old configuration while others use new configuration. This typically requires a configuration object protected by a read-write mutex.

#### Operational Benefits

Runtime configuration reload provides several operational advantages:

**Reduced Downtime**: Tuning parameters can be adjusted without daemon restart, maintaining continuous index availability for agent frameworks.

**Performance Tuning**: Worker count and rate limits can be adjusted in response to observed performance characteristics without interrupting service.

**Debug Support**: Log levels can be increased temporarily for troubleshooting without requiring restart and losing in-memory state.

**Resource Management**: Concurrency and rate limits can be adjusted based on current system load or API quota consumption.

#### Alternative Approaches

An alternative to signal-based reload would be file-based configuration watching, where the daemon monitors its configuration file for changes and automatically reloads when modifications are detected. This approach provides automatic reloading without requiring manual signal sending but adds complexity around file watching and determining when a configuration file is fully written.

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

**Signal Handler**: Code that executes in response to UNIX signals like SIGINT or SIGTERM. Enables graceful shutdown and operational commands.

**Token Bucket**: A rate limiting algorithm that allows bursts up to a capacity while maintaining an average rate. Used for Claude API call throttling.

**Worker Pool**: A collection of goroutines that process jobs concurrently with a shared job queue and result collection.
