# File Watcher Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Event Debouncing](#event-debouncing)
   - [Recursive Directory Monitoring](#recursive-directory-monitoring)
   - [Filtering Strategy](#filtering-strategy)
   - [Graceful Shutdown](#graceful-shutdown)
3. [Key Components](#key-components)
   - [Watcher Struct](#watcher-struct)
   - [Event Types](#event-types)
   - [Core Methods](#core-methods)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Index Manager](#index-manager)
   - [Configuration System](#configuration-system)
5. [Glossary](#glossary)

## Overview

The File Watcher subsystem provides real-time file system monitoring for the Agentic Memorizer daemon. It watches the memory directory for file changes (creates, modifications, deletions) and emits debounced events to trigger incremental index updates. This enables the daemon to maintain an always-current index without requiring full rebuilds for every file change.

The subsystem follows an event-driven, debounced observer pattern and wraps the fsnotify library to provide platform-native file system notifications. It implements intelligent event batching to prevent redundant processing of rapid file changes, making it efficient for scenarios where files are frequently saved or modified in quick succession.

## Design Principles

### Event Debouncing

The File Watcher implements a last-write-wins debouncing strategy to prevent redundant processing when files are saved multiple times rapidly (such as during auto-save in editors or when build tools write many files in quick succession).

Events accumulate in an internal map keyed by file path, with a ticker that fires at a configurable interval (default: 500ms). When the debounce period expires, only the last event for each path is sent to consumers. This approach collapses multiple rapid saves into a single event, preventing API quota exhaustion and reducing CPU/disk thrashing.

For example, if a file is modified at t=0ms, t=100ms, t=200ms, and t=300ms, only one EventModify is emitted at t=500ms when the debounce window closes.

### Recursive Directory Monitoring

Since fsnotify only watches explicitly registered directories (not recursively), the File Watcher implements automatic recursive monitoring. During initialization, it walks the entire directory tree and registers all subdirectories with fsnotify. When a Create event is detected for a directory, the watcher spawns a goroutine to recursively add the new directory and its descendants, ensuring that newly created subdirectories are automatically monitored without requiring a restart.

### Filtering Strategy

The File Watcher implements a three-level filtering strategy to reduce system overhead and prevent index pollution:

1. **Directory Skip**: Prevents descending into specified directories like `.git`, `.cache`, and other hidden directories
2. **File Skip**: Ignores specific files configured by the user, such as binaries or temporary files
3. **Hidden Filter**: Automatically skips any file or directory starting with `.`

This filtering happens before events are added to the batch, minimizing the memory footprint and processing overhead of the watcher.

### Graceful Shutdown

The File Watcher implements a coordinated shutdown pattern that ensures all in-flight events are processed, no goroutines are leaked, and all resources are cleanly released. The shutdown sequence:

1. Signals shutdown via a close channel
2. Waits for all goroutines to complete using a WaitGroup
3. Closes the underlying fsnotify watcher
4. Closes the event output channel

This pattern ensures that the daemon can cleanly restart or terminate without losing events or leaving system resources in an inconsistent state.

## Key Components

### Watcher Struct

The Watcher struct (`internal/watcher/watcher.go`) is the core component that manages file system monitoring state and coordinates event processing.

**Core State Fields:**
- `rootPath`: The base directory being monitored
- `fsWatcher`: The underlying fsnotify watcher instance
- `eventChan`: Buffered output channel (capacity: 100) for debounced events
- `batchedEvents`: Map accumulating events during the debounce period
- `skipDirs`: Directories to ignore during recursive watching
- `skipFiles`: Specific files to ignore
- `debounceMs`: Debounce period in milliseconds
- `debounceIntervalCh`: Buffered channel (capacity 1) for runtime debounce interval updates

**Concurrency Controls:**
- `eventMu`: Mutex protecting the batchedEvents map
- `stopChan`: Signal channel for graceful shutdown coordination
- `wg`: WaitGroup for goroutine lifecycle management

### Event Types

The File Watcher defines a simplified event model that abstracts fsnotify's platform-specific details:

- **EventCreate**: Emitted when a file or directory is created
- **EventModify**: Emitted when a file's contents are modified
- **EventDelete**: Emitted when a file is deleted or renamed (renames are treated as delete+create pairs)

Each event includes the event type and the absolute path to the affected file.

### Core Methods

**Lifecycle Management:**
- `New()`: Creates a new watcher with configuration (root path, skip patterns, debounce period, logger)
- `Start()`: Begins watching by walking the directory tree and launching processing goroutines
- `Stop()`: Initiates graceful shutdown
- `Events()`: Returns a read-only channel for consuming debounced events

**Runtime Configuration:**
- `UpdateDebounceInterval(intervalMs int)`: Updates the debounce interval without restarting the watcher, enabling hot-reload of daemon configuration

**Internal Event Processing:**
- `processEvents()`: Goroutine that reads from fsnotify and delegates to handleEvent
- `handleEvent()`: Translates fsnotify events to Watcher events, filters based on skip patterns, and handles dynamic directory registration
- `debounceBatch()`: Ticker goroutine that periodically flushes batched events; monitors `debounceIntervalCh` to recreate ticker when interval changes
- `sendBatchedEvents()`: Flushes accumulated events to the output channel

**Directory Management:**
- `addRecursive()`: Recursively registers directories with fsnotify
- `shouldSkip()`: Checks if a file should be ignored based on configuration
- `shouldSkipDir()`: Checks if a directory should be skipped during recursive watching

## Integration Points

### Daemon Subsystem

The Daemon subsystem creates and manages the File Watcher lifecycle. During daemon initialization (`internal/daemon/daemon.go:140-156`), it instantiates a watcher with configuration-driven parameters:

- Root path from `cfg.MemoryRoot`
- Skip directories hardcoded as `.cache` and `.git`
- Skip files from `cfg.Analysis.SkipFiles`
- Debounce period from `cfg.Daemon.DebounceMs`

The daemon runs a dedicated goroutine (`processWatcherEvents()`) that consumes events from the watcher's channel and delegates to `handleFileEvent()` for processing. Event handling differs by type:

- **Create/Modify**: Extracts metadata, computes file hash, checks cache (recording cache hit metric via `RecordCacheHit()` if found), performs semantic analysis if needed (recording API call metric via `RecordAPICall()`), updates the index, records file processed metric via `RecordFileProcessed()`, and increments index file count via `IncrementIndexFileCount()` when a new file is added
- **Delete**: Removes the entry from the index and decrements index file count via `DecrementIndexFileCount()` if removal was successful

All changes are immediately persisted via atomic write operations to ensure index consistency. Health metrics are recorded throughout the event processing flow to accurately track cache hits, API calls, files processed, and index file count changes during incremental updates.

The daemon also tracks watcher health via `HealthMetrics.WatcherActive`, which is exposed through an optional HTTP health check endpoint.

**Configuration Hot-Reload:**
The daemon supports runtime debounce interval updates without restarting via the `config reload` command. When configuration is reloaded (`daemon.go:747`), the daemon calls `watcher.UpdateDebounceInterval()` with the new value. The watcher uses a buffered channel (capacity 1) to signal the debounce goroutine, which then recreates the ticker with the new interval. If the channel is full (a previous update hasn't been processed), a warning is logged and the update is skipped. This design enables non-blocking configuration updates while the watcher continues processing events.

### Index Manager

The File Watcher indirectly integrates with the Index Manager through the daemon. The event flow is:

1. Watcher emits events
2. Daemon processes events and performs semantic analysis
3. Daemon calls Index Manager methods with tracking information:
   - `UpdateSingle(entry types.IndexEntry, info UpdateInfo) (UpdateResult, error)` - For creates/modifies, passing UpdateInfo to track what happened during processing (cache hit, API call, error)
   - `RemoveFile(path string) (RemoveResult, error)` - For deletes, returning RemoveResult with removal status and file size
4. Index Manager updates the in-memory index and relevant statistics
5. Daemon uses the result to update health metrics (increment/decrement file count)
6. Daemon persists the updated index to disk

This separation of concerns allows the watcher to focus solely on file system monitoring while the daemon orchestrates the complete update workflow.

### Configuration System

The File Watcher's behavior is controlled by configuration parameters defined in `internal/config/types.go`:

```yaml
daemon:
  debounce_ms: 500              # Event batching period (default: 500ms)

analysis:
  skip_files:                   # User-configured files to ignore
    - agentic-memorizer
```

The watcher also uses hardcoded skip patterns (`.cache`, `.git`) and automatically filters hidden files and directories. This configuration-driven approach allows users to customize the watcher's behavior without modifying code.

## Glossary

**Debouncing**: A technique for delaying action until a burst of events settles. The File Watcher accumulates events in a map during a time window, then processes only the final state of each file.

**fsnotify**: A cross-platform Go library for file system notifications. It uses platform-native APIs such as inotify (Linux), FSEvents (macOS), or ReadDirectoryChangesW (Windows).

**Event Batching**: The process of accumulating multiple events before processing. The watcher uses a map keyed by file path, so multiple events for the same file collapse into one.

**Recursive Watching**: Monitoring a directory tree including all subdirectories and their descendants. Since fsnotify requires explicit registration of each directory, the watcher maintains this automatically.

**Channel Buffering**: The event output channel has a capacity of 100 to prevent blocking the debounce goroutine if the daemon temporarily cannot keep up with event processing.

**Last-Write-Wins**: When multiple events occur for the same path during the debounce window, only the final event is preserved and emitted. This is semantically correct for file watching since only the final file state matters for indexing.

**Skip Patterns**: Configuration-driven filters that prevent the watcher from monitoring irrelevant files or directories. This reduces system overhead and prevents index pollution.

**Graceful Shutdown**: A coordinated shutdown pattern that ensures all in-flight events are processed, no goroutines are leaked, and all resources (file descriptors, channels) are properly closed.
