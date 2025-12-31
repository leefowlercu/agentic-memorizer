# File Watcher

Real-time filesystem monitoring with debounced event batching and intelligent filtering.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-31

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The File Watcher subsystem provides real-time monitoring of the memory directory for file changes. Built on fsnotify, it detects file creation, modification, deletion, and rename events, then delivers them to consumers through a buffered channel. The watcher implements debouncing to coalesce rapid changes into single events, preventing redundant processing when files are saved multiple times in quick succession.

The subsystem handles recursive directory watching, automatically adding new subdirectories to the watch list as they are created. It supports configurable filtering by directory names, file names, and extensions, plus automatic exclusion of hidden files and directories (those starting with a dot).

Key capabilities include:

- **Event detection** - Monitors for Create, Modify, Delete, and Rename operations via fsnotify
- **Debounced batching** - Coalesces rapid changes within a configurable window (default 500ms)
- **Event priority** - DELETE overwrites all, CREATE preserved over MODIFY, ensuring correct final state
- **Recursive watching** - Automatically watches new subdirectories as they appear
- **Configurable filtering** - Skip directories, files, and extensions based on configuration
- **Hot-reload support** - Debounce interval can be updated at runtime without restart

## Design Principles

### Event Batching with Priority

Raw filesystem events arrive at high frequency, especially during editor saves or bulk operations. The watcher batches events by path within a debounce window, but with priority rules that ensure the final event accurately represents the file's state. DELETE events always take precedence (the file is gone), CREATE events are preserved over MODIFY (a new file was created), and MODIFY events can overwrite other MODIFY events. This prevents processing stale intermediate states.

### Recursive Watch Management

Rather than requiring callers to manage directory watches, the watcher automatically handles subdirectory discovery. During startup, it recursively walks the root path and adds all non-excluded directories. When a new directory is created, the watcher detects the CREATE event and asynchronously adds the directory and its contents to the watch list. This ensures complete coverage without explicit management.

### Two-Tier Filtering

Filtering operates at two levels: directory pruning and file filtering. Directory pruning (shouldSkipDir) prevents entire directory trees from being watched, which is critical for performance with large excluded directories like .git or node_modules. File filtering (shouldSkip) prevents events for individual files from being processed. Both tiers check against skip lists and automatically exclude hidden paths.

### Graceful Lifecycle Management

The watcher uses a WaitGroup to coordinate its two goroutines (event processing and debounce batching) and a stop channel for clean shutdown. The Stop method closes the stop channel, waits for goroutines to complete, closes the fsnotify watcher, and finally closes the event channel. This ordering prevents panics from writing to closed channels.

### Non-Blocking Updates

The UpdateDebounceInterval method uses a buffered channel with select-default to avoid blocking. If an update is already pending (channel full), the call silently skips rather than blocking the caller. This enables safe calls from configuration hot-reload handlers without risk of deadlock.

## Key Components

### Watcher

The main Watcher struct encapsulates all watching functionality. It holds the root path, skip lists (directories, files, extensions), the debounce interval in milliseconds, the underlying fsnotify watcher, and the event delivery channel. Internal state includes a batched events map protected by a mutex, a stop channel for shutdown coordination, a WaitGroup for goroutine management, and a channel for debounce interval updates.

### Event Types

Three event types represent filesystem changes: EventCreate for new files, EventModify for content changes, and EventDelete for removed files. Rename events from fsnotify are translated to EventDelete for the source path, with the destination receiving a separate EventCreate. This simplifies consumer logic to handle just three cases.

### Event Processing Goroutine

The processEvents goroutine reads from the fsnotify event channel and calls handleEvent for each. It runs until the stop channel is closed or the fsnotify channel closes. The handler determines event type, applies skip filtering, handles new directory detection with recursive add, and stores the event in the batched events map with priority rules.

### Debounce Goroutine

The debounceBatch goroutine uses a ticker at the configured interval. On each tick, it swaps out the batched events map (under lock), clears it, and sends all accumulated events to the output channel. It also listens for debounce interval updates, replacing the ticker when the interval changes. The ticker-based approach means events are delivered in batches at regular intervals rather than after a quiet period.

### Skip Logic

Two filtering functions implement the skip behavior. shouldSkipDir checks if a directory should be excluded from watching based on hidden status (dot prefix) or presence in the skip directories list. shouldSkip checks files against hidden status, skip files list, and skip extensions list. Both use the path's basename for comparison.

## Integration Points

### Daemon Subsystem

The daemon creates and manages the watcher as part of its startup sequence. It passes the memory root path, skip configurations from the config subsystem, and the debounce interval. The daemon reads from the watcher's Events channel and routes events to handleFileEvent, which dispatches to appropriate processing based on event type. Create and Modify events trigger file processing through the worker pool; Delete events trigger index removal.

### Configuration Subsystem

The watcher receives its initial configuration (skip patterns, debounce interval) from the config subsystem via the daemon. When configuration hot-reload occurs, the daemon calls UpdateDebounceInterval to adjust the watcher's batching behavior without restart. Skip pattern changes require daemon restart as they affect directory watching.

### Worker Pool

The daemon's worker pool receives jobs generated from watcher events. When the watcher emits Create or Modify events, the daemon creates processing jobs that flow through the worker pool for metadata extraction and semantic analysis. The debounced batching ensures the worker pool processes files at a sustainable rate.

### Walker Subsystem

The watcher and walker serve complementary purposes: the walker performs full directory scans during rebuilds, while the watcher provides real-time change detection between rebuilds. Both use similar skip configurations to ensure consistent file discovery. The walker is used for initial population and periodic rebuilds; the watcher handles incremental updates.

## Glossary

**Debouncing**
The practice of coalescing multiple rapid events into a single event. When a file is saved multiple times in quick succession, debouncing ensures only one processing job is created for the final state.

**Event Batching**
Accumulating events over a time window before delivering them. The watcher collects events into a map keyed by path, then sends all accumulated events when the debounce timer fires.

**Event Priority**
Rules determining which event type wins when multiple events occur for the same path within a batch window. DELETE has highest priority, then CREATE, then MODIFY.

**fsnotify**
A Go library providing cross-platform filesystem notification support. It uses inotify on Linux, FSEvents on macOS, and ReadDirectoryChangesW on Windows.

**Hidden File**
A file or directory whose name starts with a dot (.). The watcher automatically excludes hidden paths from both watching and event processing.

**Recursive Watching**
Automatically adding subdirectories to the watch list. When a new directory is created, the watcher detects it and begins watching its contents without explicit registration.

**Skip Pattern**
Configuration specifying directories, files, or extensions to exclude from watching. Common examples include .git, .cache, and node_modules directories.

**WaitGroup**
A Go synchronization primitive used to wait for multiple goroutines to complete. The watcher uses a WaitGroup to ensure clean shutdown of its processing and batching goroutines.
