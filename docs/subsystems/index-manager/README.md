# Index Manager Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Thread Safety](#thread-safety)
   - [Atomic Operations](#atomic-operations)
   - [Crash Recovery](#crash-recovery)
   - [Separation of Concerns](#separation-of-concerns)
   - [Versioning and Metadata](#versioning-and-metadata)
3. [Key Components](#key-components)
   - [Index Manager](#index-manager)
   - [Computed Index Structure](#computed-index-structure)
   - [Build Metadata](#build-metadata)
   - [Type Definitions](#type-definitions)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Read Command](#read-command)
   - [Type System](#type-system)
   - [File System](#file-system)
5. [Operational Patterns](#operational-patterns)
   - [Full Index Rebuild](#full-index-rebuild)
   - [Incremental Updates](#incremental-updates)
   - [Concurrent Access](#concurrent-access)
   - [Loading Existing Index](#loading-existing-index)
6. [Glossary](#glossary)

## Overview

The Index Manager subsystem is responsible for managing the precomputed index file that serves as the central data structure for Agentic Memorizer. It provides thread-safe operations for reading, writing, and updating the index while ensuring data integrity through atomic file operations and crash recovery mechanisms.

### Purpose

The Index Manager provides several critical capabilities:

- **Thread-Safe Access**: Manages concurrent read and write operations to the index using reader-writer locks
- **Atomic Persistence**: Ensures index writes are atomic using temporary files and rename operations to prevent corruption
- **Incremental Updates**: Supports both full index rebuilds and single-file updates for efficient index maintenance
- **Crash Recovery**: Validates and loads existing index files on startup to recover from unexpected termination
- **Data Integrity**: Maintains consistent index state through validation and atomic state transitions

### Role in the System

The Index Manager acts as the persistence layer for the memory index, sitting between the daemon's processing logic and the file system. It abstracts the complexities of thread-safe index management and atomic file operations, allowing other subsystems to work with a simple, high-level API. The Index Manager ensures that the precomputed index is always in a valid state, even in the face of crashes or concurrent access patterns.

## Design Principles

### Thread Safety

All index operations are protected by a reader-writer mutex to enable safe concurrent access. Read operations acquire read locks, allowing multiple concurrent readers without blocking. Write operations acquire exclusive write locks, ensuring that only one writer can modify the index at a time. This design maximizes concurrency while preventing data races and ensuring consistency.

The Index Manager stores the current index in memory and protects it with a RWMutex, enabling the daemon to update the index frequently while still allowing concurrent reads. This pattern is essential for supporting high-throughput indexing operations without sacrificing access speed.

**Important Safety Caveat:** The `GetCurrent()` method returns a pointer to the shared index structure, not a deep copy. Callers must treat the returned index as read-only and must not mutate any fields or nested structures. Mutations would create data races with concurrent updates. The Index Manager prioritizes performance (avoiding expensive deep copies) over defensive programming, placing the burden of correct usage on callers.

### Atomic Operations

All writes to the index file use atomic operations to prevent corruption. The atomic write pattern follows these steps: marshal the index to JSON, write to a temporary file with a `.tmp` extension, sync the temporary file to disk to ensure durability, and atomically rename the temporary file to the final index path.

This pattern guarantees that the index file is never in a partially-written state. If the process crashes during a write, either the old index remains intact or the new index is complete. There is no intermediate state where the index file contains partial or corrupted data.

### Crash Recovery

The Index Manager supports crash recovery by validating and loading existing index files on daemon startup. When the daemon initializes, it attempts to load any existing index file. If the file exists and passes validation, it becomes the starting point for incremental updates. If the file is missing or corrupt, the daemon performs a full rebuild.

Index validation ensures that loaded indexes contain all required fields including version information, index data structures, and entry arrays. If validation fails, an error is returned and the daemon knows it must rebuild from scratch. This design ensures that the system can always recover to a consistent state after unexpected termination.

### Separation of Concerns

The Index Manager focuses solely on index persistence and does not concern itself with how indexes are built or consumed. It provides a clean API for setting, updating, and retrieving indexes, while the daemon handles the logic of when to build, update, or rebuild. The read command consumes the index file directly without involving the Index Manager, maintaining independence between index production and consumption.

This separation enables each component to be tested, understood, and modified independently. The Index Manager can guarantee atomicity and thread safety without knowing about worker pools or file watchers. The daemon can implement complex scheduling logic without worrying about file corruption.

### Versioning and Metadata

The Index Manager wraps the core index structure with versioning and metadata to enable future evolution and operational visibility. The ComputedIndex structure includes a version field for schema versioning, generation timestamp for staleness detection, daemon version for compatibility tracking, and build metadata for performance analysis.

This metadata enables future enhancements like schema migrations, index format changes, and performance monitoring. Storing the daemon version allows the read command to detect version mismatches and handle them appropriately.

## Key Components

### Index Manager

The Manager type provides the primary interface for index operations. It maintains an in-memory representation of the current index, stores the file system path where the index should be persisted, protects access with a reader-writer mutex, and tracks build metadata from the most recent index generation.

**Constructor:**
- `NewManager(indexPath string) *Manager` - Creates a new index manager with the specified file path for persistence

**Core Operations:**
The Manager exposes several key operations:
- `LoadComputed()` - Reads and validates an existing index file from disk
- `SetIndex(*types.Index, BuildMetadata)` - Updates the in-memory index from a full rebuild
- `WriteAtomic(daemonVersion string)` - Persists the current index atomically to disk
- `GetCurrent() *types.Index` - Returns a thread-safe read of the current index (**Returns pointer, not copy - callers must not mutate**)
- `UpdateSingle(entry types.IndexEntry, info UpdateInfo) (UpdateResult, error)` - Modifies a single entry with tracking info (**O(n) linear search** through entries)
- `RemoveFile(path string) (RemoveResult, error)` - Deletes an entry from the index with result tracking (**O(n) linear search** through entries)

Each operation is designed to be composable and safe. The daemon can call SetIndex followed by WriteAtomic to persist a full rebuild, or call UpdateSingle followed by WriteAtomic to persist an incremental change. Thread safety is maintained regardless of the operation sequence.

**Performance Considerations:**
- UpdateSingle and RemoveFile perform linear searches through the entries array (O(n) complexity)
- For large indexes with thousands of files, these operations may take milliseconds
- Full rebuilds replace the entire entries array, avoiding repeated linear searches

### Computed Index Structure

The ComputedIndex type wraps the core Index type with additional metadata for persistence. It includes a version string for schema versioning starting at "1.0", a generation timestamp recording when the index was built, a daemon version string identifying which version of the daemon created the index, a pointer to the core Index containing all file entries and statistics, and BuildMetadata containing performance metrics from the build process.

This structure separates the conceptual index content from the file format concerns. The core Index type represents the semantic content, while ComputedIndex adds the envelope information needed for file-based persistence and versioning.

### Build Metadata

The BuildMetadata type captures performance metrics from index generation. It tracks build duration in milliseconds for performance monitoring, the total number of files processed, cache hits indicating how many files used cached analysis, and API calls showing how many new semantic analyses were performed.

This metadata enables operational visibility into daemon performance. Operators can monitor cache hit rates to ensure caching is effective, track build durations to detect performance degradation, and analyze API call patterns to manage costs.

### Type Definitions

The Index Manager relies on type definitions from the pkg/types package including Index as the core structure containing entries and statistics, IndexEntry representing a single file with metadata and optional semantic analysis, FileMetadata containing file-specific properties, and IndexStats providing aggregate statistics.

These types are defined separately from the Index Manager to enable reuse across subsystems. The daemon, cache system, metadata extractor, and output formatters all work with the same type definitions, ensuring consistency and enabling composition.

### Type Definitions for Incremental Operations

The Index Manager defines several types specifically for tracking incremental update operations:

**UpdateInfo** - Provides context about what happened during file processing:
- `WasAnalyzed bool` - True if semantic analysis was performed via Claude API (cache miss)
- `WasCached bool` - True if cached analysis was reused (cache hit)
- `HadError bool` - True if an error occurred during processing

The daemon populates UpdateInfo when calling UpdateSingle to communicate what happened during file processing. This enables the Index Manager to properly update index statistics (AnalyzedFiles, CachedFiles, ErrorFiles) during incremental operations.

**UpdateResult** - Returns information about what the update operation did:
- `Added bool` - True if a new entry was added to the index
- `Updated bool` - True if an existing entry was modified

UpdateSingle returns UpdateResult to inform the caller whether this was a new file addition or an existing file modification. The daemon uses this to determine whether to increment the index file count metric.

**RemoveResult** - Returns information about what was removed:
- `Removed bool` - True if an entry was actually removed (false if file wasn't in index)
- `Size int64` - Size in bytes of the removed file

RemoveFile returns RemoveResult to provide feedback about the operation. The daemon uses the Removed flag to determine whether to decrement the index file count metric.

## Integration Points

### Daemon Subsystem

The daemon is the primary consumer of the Index Manager. During startup, the daemon calls LoadComputed to attempt crash recovery by loading any existing index. If loading succeeds, it calls SetIndex to initialize the in-memory index with the recovered data.

During full rebuilds, the daemon builds a complete Index structure with all entries and statistics, calls SetIndex to update the in-memory index, and calls WriteAtomic to persist the index to disk atomically. The daemon version string is passed to WriteAtomic for tracking purposes.

For incremental updates triggered by file system events, the daemon calls UpdateSingle to modify a single entry in the index and calls WriteAtomic to persist the change immediately. For file deletions, it calls RemoveFile to remove the entry and calls WriteAtomic to persist the deletion.

The daemon never directly manipulates the index file. All file operations go through the Index Manager API, ensuring atomicity and thread safety are maintained.

### Read Command

The read command is a consumer of the index file but not the Index Manager itself. It directly reads the index.json file from disk, unmarshals the ComputedIndex structure, validates the loaded data, and formats it for output according to the requested format and integration.

This independence is intentional. The read command must be fast and must not depend on the daemon running. By reading the file directly, the read command can load the index in milliseconds and can succeed even if the daemon has crashed or is being restarted.

The read command trusts that the index file is valid because the Index Manager guarantees atomicity. There is never a partially-written index file, so the read command either reads a complete valid index or receives a file-not-found error.

### Type System

The Index Manager depends on the pkg/types package for core data structures. This dependency is one-way: the Index Manager imports and uses types but does not expose any Index Manager specific types to other packages except for ComputedIndex and BuildMetadata, which are defined in the internal/index package.

The separation allows the type system to evolve independently. New metadata fields, semantic analysis properties, or statistics can be added to pkg/types without modifying the Index Manager, as long as the core structure remains compatible.

### File System

The Index Manager interacts with the file system through standard library functions from the os package. It uses os.ReadFile for reading the index file during LoadComputed, os.WriteFile for writing the temporary file during atomic writes, os.Open and os.File.Sync for ensuring data durability, and os.Rename for atomic file replacement.

The choice to use standard library functions rather than abstractions keeps the implementation simple and predictable. File system operations are well-defined and easily tested using temporary directories.

## Operational Patterns

### Full Index Rebuild

A full index rebuild follows a specific sequence orchestrated by the daemon. First, the daemon walks the entire memory directory collecting all file paths. These paths are submitted to the worker pool for parallel processing. As results are collected, they are accumulated into a new Index structure with entries for each file and statistics aggregated across all files.

Once the complete index is built, the daemon calls SetIndex to replace the in-memory index atomically. It then calls WriteAtomic to persist the new index to disk. The entire index file is replaced atomically, ensuring that readers never see a partially-updated index.

Full rebuilds occur during daemon startup to ensure the index is current and periodically during runtime to catch any inconsistencies that might arise from missed file system events. The rebuild frequency is configurable through the daemon's full_rebuild_interval_minutes setting.

### Incremental Updates

Incremental updates optimize common cases where only one or a few files change. When the file watcher detects a file modification or creation, the daemon processes that single file through the worker pool and calls UpdateSingle with the resulting IndexEntry and an UpdateInfo struct describing what happened during processing.

UpdateSingle accepts an UpdateInfo parameter that tracks whether semantic analysis was performed (WasAnalyzed), cached analysis was used (WasCached), or an error occurred (HadError). This information enables proper index statistics updates during incremental operations.

UpdateSingle acquires a write lock, searches for an existing entry with the same file path, and either updates the existing entry in place or appends a new entry to the entries slice. It updates the generation timestamp to reflect that the index has changed. For new files, it increments the total files count and updates AnalyzedFiles, CachedFiles, or ErrorFiles counts based on the UpdateInfo flags. For existing file updates, it recalculates TotalSize by removing the old file's size contribution and adding the new size.

UpdateSingle returns an UpdateResult indicating whether the operation added a new entry or updated an existing one. The daemon uses this result to determine whether to increment the index file count health metric.

After UpdateSingle completes, the daemon calls WriteAtomic to persist the updated index immediately. This ensures that the on-disk index reflects the file system state as quickly as possible.

For file deletions, the daemon calls RemoveFile which locates the entry by path, removes it from the entries slice, decrements TotalFiles and updates TotalSize, and updates the generation timestamp. RemoveFile returns a RemoveResult indicating whether an entry was actually removed and the size of the removed file. The daemon uses this to determine whether to decrement the index file count health metric. The updated index is then persisted via WriteAtomic.

Note that AnalyzedFiles and CachedFiles counts are not decremented during file removal, as these represent historical counts of analysis operations performed rather than current index state.

### Concurrent Access

The Index Manager uses a reader-writer mutex to support concurrent access patterns. The daemon's event processing loop may be updating individual entries while the periodic rebuild goroutine is replacing the entire index. Meanwhile, health monitoring code may be reading the current index to report statistics.

GetCurrent acquires a read lock, allowing multiple readers to access the index simultaneously without blocking each other. SetIndex, UpdateSingle, and RemoveFile acquire write locks, ensuring exclusive access during modifications. WriteAtomic acquires a read lock during the marshal and write operations, allowing the index to be read while it is being persisted.

This locking strategy balances performance and safety. Reads are fast and non-blocking in the common case. Writes are protected but do not block readers any longer than necessary. The atomic rename at the end of WriteAtomic occurs outside the lock, minimizing the critical section.

### Loading Existing Index

During daemon startup, LoadComputed attempts to load an existing index file for crash recovery. It acquires a read lock, reads the file contents with os.ReadFile, unmarshals the JSON into a ComputedIndex structure, and validates that the index contains required fields including version, index pointer, and entries array.

If validation succeeds, LoadComputed updates the internal currentIndex and buildMetadata fields to initialize the Index Manager state. It then returns the loaded ComputedIndex to the caller. The daemon uses this to resume from a previous state rather than rebuilding from scratch.

If the file does not exist or validation fails, LoadComputed returns an error. The daemon interprets this as requiring a full rebuild. This design ensures that the Index Manager never operates with invalid data, even if the index file has been corrupted or manually edited.

## Glossary

**Atomic Operation**: An operation that either completes fully or has no effect, preventing partial or inconsistent state. The Index Manager uses atomic file operations to ensure the index file is never corrupted.

**Build Metadata**: Performance metrics captured during index generation including build duration, files processed, cache hits, and API calls.

**Computed Index**: The file format wrapper around the core Index type, adding versioning and metadata for persistence and future evolution.

**Content Hash**: A SHA-256 hash of file contents used as a cache key and stored in index entries for change detection.

**Crash Recovery**: The ability to detect and recover from unexpected process termination by loading and validating existing index files.

**Incremental Update**: Modifying a single entry in the index rather than rebuilding the entire index, used for handling individual file changes efficiently.

**Index Entry**: A single record in the index representing one file with its metadata and optional semantic analysis.

**Index Manager**: The component responsible for thread-safe persistence and management of the precomputed index file.

**Precomputed Index**: An index file maintained continuously by the daemon containing all file metadata and semantic analysis, enabling instant loading by agent frameworks.

**Reader-Writer Lock**: A synchronization primitive that allows multiple concurrent readers or a single exclusive writer, used to protect index access.

**Schema Versioning**: The practice of including version information in persisted data structures to enable future format changes and migrations.

**Temporary File Pattern**: A technique for atomic file writes where data is written to a temporary file and then atomically renamed to the final location.

**Thread Safety**: The property that a component can be safely accessed from multiple concurrent threads without data races or corruption.

**Validation**: The process of checking that loaded data structures contain all required fields and relationships before using them.
