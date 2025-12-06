# Walker Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Callback-Based Processing](#callback-based-processing)
   - [Skip Pattern Strategy](#skip-pattern-strategy)
   - [Recursive Traversal](#recursive-traversal)
   - [Error Handling Strategy](#error-handling-strategy)
3. [Key Components](#key-components)
   - [Walk Function](#walk-function)
   - [Relative Path Utility](#relative-path-utility)
   - [Filtering System](#filtering-system)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Worker Pool](#worker-pool)
   - [File Watcher](#file-watcher)
   - [Configuration System](#configuration-system)
5. [Glossary](#glossary)

## Overview

The Walker subsystem is a lightweight directory traversal utility that recursively walks a file system tree, applying filtering rules and invoking callbacks for each discovered file. It serves as the file discovery mechanism during full index rebuilds, collecting all files that should be processed by the daemon's worker pool. The walker is a focused, single-purpose component that bridges file system navigation with the daemon's processing pipeline.

The subsystem implements a visitor pattern through callback functions, allowing the caller to define what happens with each discovered file while the walker handles the mechanics of directory traversal and filtering. This separation of concerns enables the walker to remain generic and reusable while supporting specialized use cases like job queue construction for the daemon's worker pool.

The Walker provides two core capabilities: recursive directory traversal with configurable skip patterns, and relative path computation for creating portable index entries. These capabilities combine to enable efficient file discovery while maintaining clean boundaries between file system navigation and file processing logic.

## Design Principles

### Callback-Based Processing

The Walker implements the Visitor Pattern through function callbacks, separating the traversal algorithm from processing logic and enabling flexible, caller-controlled behavior for each discovered file.

**Visitor Function Type:**
The walker defines a `FileVisitor` function type that receives a file path and its `os.FileInfo` structure. This callback is invoked for every file that passes the filtering rules, allowing the caller to perform arbitrary operations like creating job structures, collecting statistics, or validating file characteristics.

**Decoupling Discovery from Processing:**
By using callbacks, the walker completely decouples file discovery from file processing. The walker's responsibility ends at identifying files and providing their metadata. The callback decides what to do with each file, whether that's adding to a job queue, performing immediate processing, or gathering statistics.

**Caller-Controlled Behavior:**
The callback pattern gives the caller complete control over processing logic without requiring the walker to understand specific use cases. Different callers can use the same walker with different callbacks for entirely different purposes. The daemon uses it for job creation, but the same walker could support validation, cleanup, or analysis operations through different callbacks.

**Error Propagation Control:**
The callback returns an error, giving the caller the power to halt traversal at any point. If the callback returns a non-nil error, the walk immediately terminates and propagates that error to the caller. This enables early termination when errors occur or when sufficient data has been collected, avoiding unnecessary traversal of remaining directories.

### Skip Pattern Strategy

The Walker implements a three-tier filtering strategy that distinguishes between directory-level, file-level, and extension-level exclusions, optimizing performance by preventing traversal of entire subtrees.

**Directory-Level Skipping:**
When a directory matches a skip pattern, the walker returns `filepath.SkipDir`, preventing descent into that directory and its entire subtree. This early pruning provides massive performance benefits by avoiding traversal of potentially thousands of files within excluded directories like `.git` or `.cache`. The walker maintains a map of absolute skip paths built by joining the root with each skip directory name, enabling O(1) lookup during traversal.

**File-Level Skipping:**
After a directory passes filtering, individual files within that directory are checked against file skip patterns. Files whose basenames match the skip list are silently ignored without callback invocation. This file-level filtering has lower performance impact since it only affects individual files rather than entire directory trees.

**Extension-Level Skipping:**
After file-level filtering, files are checked against the skip extensions list. Files whose extensions (including the dot, e.g., `.zip`, `.tar`) match the skip list are silently ignored without callback invocation. This provides an efficient way to exclude entire categories of files (archives, binaries, etc.) without explicitly listing each filename.

**Hidden File Filtering:**
The walker automatically excludes any file or directory whose name starts with a dot (`.`), treating them as hidden system files. This built-in filter prevents accidental processing of hidden configuration files, temporary files, and system directories. The hidden file filter applies before explicit skip patterns and cannot be disabled.

**Performance Optimization:**
The three-tier strategy recognizes that directory skipping has exponentially greater impact than file or extension skipping. A single directory skip can eliminate thousands of file checks. The walker prioritizes directory filtering, checking it first and using `filepath.SkipDir` to short-circuit traversal efficiently. File and extension filtering occur sequentially after directory filtering passes.

### Recursive Traversal

The Walker leverages Go's standard `filepath.Walk()` function to provide platform-independent recursive directory traversal with built-in handling of symbolic links, permission errors, and path normalization.

**Standard Library Foundation:**
By building on `filepath.Walk()`, the walker inherits robust, well-tested traversal logic that handles edge cases like circular symbolic links, permission denied errors, and platform-specific path separators. This foundation ensures correct behavior across Unix, Windows, and macOS without platform-specific code.

**Pre-Order Traversal:**
The walk visits nodes in pre-order (parent before children), meaning directories are processed before their contents. This ordering enables directory-level filtering decisions to affect subtree processing, supporting the skip pattern strategy's performance optimization.

**Path Normalization:**
Before traversal begins, the walker normalizes the root path using `filepath.Clean()`, removing redundant separators and resolving relative references like `.` and `..`. This normalization ensures consistent path representation throughout the traversal, preventing duplicate visits or missed paths due to path string variations.

**Symbolic Link Handling:**
The walker does NOT follow symbolic links. Go's `filepath.Walk()` explicitly does not follow symbolic links, treating them as regular files instead. This means symbolic links to directories will appear as files in the walker's traversal and will not trigger recursion into the linked directories. This behavior prevents potential infinite loops from circular symlinks and ensures the walker only processes files directly within the memory root directory tree.

### Error Handling Strategy

The Walker implements a graceful degradation strategy for file system errors while respecting callback errors, balancing robustness with caller control over error semantics.

**Graceful File System Error Handling:**
When `filepath.Walk()` encounters file system errors (permission denied, file not found, I/O errors), the walker logs these errors to stderr but continues traversal. This graceful degradation ensures that temporarily inaccessible files or permission issues don't prevent processing of remaining files. The walker aims for partial success rather than all-or-nothing failure.

**Strict Callback Error Handling:**
In contrast to file system errors, callback errors halt traversal immediately. When the visitor function returns a non-nil error, the walker propagates that error and terminates the walk. This strict handling respects the caller's error semantics, allowing the callback to signal fatal conditions that should stop processing.

**Error Logging vs. Propagation:**
The distinction between logged and propagated errors reflects their severity and recoverability. File system errors are typically transient or localized (one inaccessible file shouldn't prevent processing thousands of others). Callback errors represent business logic failures that may require complete operation failure (job creation error might invalidate the entire rebuild).

**Caller Error Control:**
The daemon's callback always returns nil, effectively making all walks complete successfully from the walker's perspective. This design choice moves error handling to the job processing phase rather than the discovery phase, allowing file discovery to always succeed while deferring processing errors to the worker pool.

## Key Components

### Walk Function

The Walk function (`internal/walker/walker.go`) provides recursive directory traversal with filtering and callback invocation for each discovered file.

**Function Signature:**
```
Walk(root string, skipDirs []string, skipFiles []string, skipExtensions []string, visitor FileVisitor) error
```

**Parameters:**
- `root` - Starting directory for traversal (typically the memory root)
- `skipDirs` - List of directory names to exclude from traversal (triggers subtree pruning)
- `skipFiles` - List of file basenames to exclude from processing (individual file filtering)
- `skipExtensions` - List of file extensions to exclude from processing (e.g., `.zip`, `.tar`, `.exe`)
- `visitor` - Callback function invoked for each qualifying file

**Traversal Process:**
1. Normalizes the root path using `filepath.Clean()`
2. Converts relative skip directories to absolute paths by joining with root
3. Builds map data structures for efficient skip pattern lookup (directories, files, and extensions)
4. Initiates recursive traversal using `filepath.Walk()`
5. For each path encountered, applies filtering rules hierarchically
6. Invokes callback for files that pass all filters
7. Returns errors from callback or underlying traversal

**Filtering Hierarchy:**
The walk applies filters in this order:
1. Root directory check (skip callback for root itself)
2. Hidden directory filter (name starts with `.` triggers `filepath.SkipDir`)
3. Explicit directory skip (matches `skipDirs` list triggers `filepath.SkipDir`)
4. Directory pass-through (directories that pass return `nil` to continue traversal)
5. Hidden file filter (name starts with `.` skips file without callback)
6. Explicit file skip (basename matches `skipFiles` list skips without callback)
7. Extension filter (file extension matches `skipExtensions` list skips without callback)
8. File callback (qualifying files invoke the visitor function)

**Return Value:**
Returns `nil` on successful completion or an error if the callback returns an error. File system errors during traversal are logged but don't cause Walk to return an error.

### Relative Path Utility

The `GetRelPath` function provides path normalization for creating portable index entries that store paths relative to the memory root.

**Function Signature:**
```
GetRelPath(root, path string) (string, error)
```

**Purpose:**
Computes the relative path from root to path, enabling index entries to store portable paths that remain valid when the memory directory moves or is shared across systems.

**Implementation:**
Wraps Go's `filepath.Rel()` function with error handling, converting absolute paths within the memory tree to relative paths suitable for index storage.

**Usage Pattern:**
Called by the daemon after file processing completes to populate the `RelPath` field in index entry metadata:
1. Worker pool processes file using absolute path
2. Worker returns result with absolute path in metadata
3. Daemon calls `GetRelPath(memoryRoot, absolutePath)`
4. Daemon stores relative path in `IndexEntry.Metadata.RelPath`
5. Index file contains portable relative paths

**Portability Benefits:**
- Index files can be shared across systems with different absolute paths
- Memory directory can be moved without invalidating the index
- Human-readable paths in index output
- Security through avoiding absolute path exposure

### Filtering System

The Walker's filtering system implements a hierarchical approach that distinguishes between directory-level and file-level exclusions, optimizing performance through early pruning of directory subtrees.

**Directory Filtering:**
Directory filtering operates at the directory node level, making pruning decisions before examining directory contents. The walker maintains an absolute path map for skip directories, enabling O(1) lookup during traversal. When a directory matches a skip pattern, the walker returns `filepath.SkipDir` to prevent descending into that subtree.

**Skip Directory Resolution:**
Skip directories should be specified as basenames or paths relative to the root directory. The walker resolves these by joining with the root path, producing absolute paths for matching during traversal (`skipPaths[filepath.Join(root, dir)] = true`). This means absolute paths passed to `skipDirs` will be incorrectly joined with the root path, resulting in paths like `/root/path/absolute/path` that won't match any actual directories. For correct behavior, use directory basenames (e.g., ".cache", ".git") or relative paths (e.g., "subdir/nested"). This resolution happens once at walk initialization rather than repeatedly during traversal.

**File Filtering:**
File filtering operates after a directory has passed filtering and its contents are being examined. Files are checked against the skip files list by basename comparison (not full path). Matching files are silently skipped without callback invocation.

**Extension Filtering:**
Extension filtering operates after file filtering, checking the file extension against the skip extensions list. Extensions are matched including the dot separator (e.g., `.zip`, `.tar`, `.exe`). Files with matching extensions are silently skipped without callback invocation. This provides a convenient way to exclude binary files, archives, and other non-analyzable file types.

**Hidden Item Filtering:**
The walker automatically filters any file or directory whose basename starts with `.` (dot character), treating them as hidden system items. This filter applies before explicit skip patterns and cannot be disabled, preventing accidental processing of configuration files, temporary files, and system directories.

**Filter Data Structures:**
The walker uses Go maps for skip pattern storage, providing O(1) lookup performance:
- `skipPaths map[string]bool` - Absolute paths of directories to skip
- `skipFileNames map[string]bool` - Basenames of files to skip
- `skipExts map[string]bool` - File extensions to skip (including dot)

These maps are built once at walk initialization and reused throughout traversal for efficient filtering.

## Integration Points

### Daemon Subsystem

The Daemon subsystem is the primary consumer of the Walker, using it exclusively during full index rebuilds to discover all files that require processing.

**Integration Context:**
When the daemon performs a full rebuild (periodic, startup, or manual), it invokes the walker to discover all files within the memory directory. The walker traverses the tree and invokes a callback for each discovered file, allowing the daemon to create Job structures for the worker pool.

**Job Creation Callback:**
The daemon provides a closure as the visitor callback that captures a job slice:
1. Walker invokes callback with file path and FileInfo
2. Callback calculates priority based on modification time (recent files get higher priority)
3. Callback creates Job struct with path, FileInfo, and priority
4. Job is appended to the captured slice
5. Callback returns nil to continue traversal

**Skip Pattern Configuration:**
The daemon configures skip patterns from three sources:
- **Hardcoded Skip Directories**: `.cache` (prevents indexing the cache) and `.git` (prevents indexing version control data)
- **Configured Skip Files**: From `config.Analysis.SkipFiles`, allowing users to exclude specific files like binaries or build artifacts
- **Configured Skip Extensions**: From `config.Analysis.SkipExtensions`, allowing users to exclude entire categories of files by extension (default: `.zip`, `.tar`, `.gz`, `.exe`, `.bin`, `.dmg`, `.iso`)

**Post-Traversal Processing:**
After the walk completes, the daemon:
1. Submits collected jobs to worker pool via `SubmitBatch()`
2. Worker pool sorts jobs by priority (recent files first)
3. Workers process each job (extract metadata, analyze, cache)
4. Results collected into index entries
5. Daemon uses `GetRelPath()` to populate relative paths in metadata

**Error Handling:**
The daemon's callback always returns nil, ensuring the walk completes even if individual job creation encounters issues. This design defers error handling to the processing phase, separating discovery from processing concerns.

### Worker Pool

The Walker and Worker Pool collaborate indirectly through the Job structures that the walker's callback creates during traversal.

**Data Flow:**
1. Walker discovers files and invokes callback
2. Callback creates Job structs with path, FileInfo, and priority
3. Jobs are collected in a slice during traversal
4. After walk completes, jobs are submitted to worker pool as a batch
5. Worker pool sorts jobs by priority using FileInfo.ModTime()
6. Workers process each job through the full processing pipeline

**Priority System:**
The walker provides `os.FileInfo` to the callback, which includes modification timestamp. The daemon uses this timestamp to calculate priority:
- Files modified within last hour: Priority 100 (highest)
- Files modified within last day: Priority 50
- Files modified within last week: Priority 25
- Older files: Priority 10 (lowest)

This priority system ensures recently modified files are processed first, providing responsive updates for active work.

**Separation of Concerns:**
The walker handles file discovery while the worker pool handles file processing. The Job struct serves as the boundary between these concerns, containing just enough information (path, FileInfo, priority) for the worker pool to operate independently.

### File Watcher

The Walker and File Watcher serve complementary roles in the daemon's file discovery strategy, with the walker handling full scans and the watcher handling incremental updates.

**Complementary Responsibilities:**
- **Walker**: Performs complete directory scans during full rebuilds (periodic, startup, manual)
- **Watcher**: Monitors individual file changes in real-time (create, modify, delete events)

**Shared Filtering Approach:**
Both components implement similar skip pattern logic but with independent implementations and different matching strategies:
- **Walker**: Uses `skipDirs`, `skipFiles`, and `skipExtensions` parameters with map-based lookup. Directory filtering compares **full absolute paths** against skip patterns.
- **Watcher**: Uses `shouldSkip()` and `shouldSkipDir()` methods with slice-based checking. Directory filtering compares only **basenames** against skip patterns. Both use identical extension filtering logic.

This difference means the walker's directory skip patterns must be specified as basenames or relative paths (which get joined with root to form absolute paths), while the watcher matches skip directories by basename only. This duplication is intentional - the components operate independently with different operational characteristics (walker is one-shot, watcher is continuous) and different performance requirements (walker optimizes for batch processing, watcher optimizes for real-time response).

**Operational Distinction:**
The walker runs when:
- Daemon starts up (initial index build)
- Full rebuild interval expires (periodic consistency check)
- User manually triggers rebuild
- Index corruption is detected and rebuild is required

The watcher runs continuously, detecting file changes between full rebuilds and enabling incremental index updates without complete rebuilds.

### Configuration System

The Walker receives configuration indirectly through the daemon, which reads skip patterns from the Config Manager and passes them to the walker during traversal.

**Configuration Flow:**
1. Config Manager loads `config.yaml` with `Analysis.SkipFiles` and `Analysis.SkipExtensions` lists
2. Daemon reads skip configuration during initialization
3. Daemon adds hardcoded skip directories (`.cache`, `.git`)
4. Daemon passes combined skip patterns to walker during rebuild

**Hardcoded vs. Configurable:**
- **Skip Directories**: Hardcoded in daemon (`.cache`, `.git`) - not user-configurable
- **Skip Files**: Configurable via `config.Analysis.SkipFiles` - user-definable list
- **Skip Extensions**: Configurable via `config.Analysis.SkipExtensions` - user-definable list with sensible defaults

**Design Rationale:**
Skip directories are hardcoded to prevent users from accidentally indexing cache data or version control history, which would be counterproductive. Skip files and extensions are configurable to allow users to exclude project-specific files and file types.

**Default Skip Files:**
The default configuration includes:
- `agentic-memorizer` - The binary itself to prevent self-indexing

**Default Skip Extensions:**
The default configuration includes:
- `.zip`, `.tar`, `.gz` - Archive formats
- `.exe`, `.bin` - Binary executables
- `.dmg`, `.iso` - Disk images

Users can extend or override these lists with additional patterns specific to their workflows or projects. Both `skip_files` and `skip_extensions` are hot-reloadable configuration settings.

## Glossary

**FileVisitor**: A callback function type that receives each discovered file's path and `os.FileInfo` structure. The visitor pattern allows the walker to remain generic while enabling caller-specific processing logic for each file.

**Skip Patterns**: Lists of directory names, file names, and file extensions to exclude from traversal. Directory skip patterns trigger early termination of subtree traversal (performance optimization), while file and extension skip patterns filter individual files after directories pass filtering.

**Absolute Skip Paths**: The walker converts relative skip directory names to absolute paths by joining with the root directory. This conversion happens once at initialization and enables efficient map-based lookup during traversal.

**Recursive Traversal**: Depth-first directory tree walking that visits every node (file or directory) in the tree. The walker uses Go's `filepath.Walk()` which implements pre-order traversal (parent directories visited before their contents).

**Directory Pruning**: The act of returning `filepath.SkipDir` from the walk function to prevent descending into a directory and its children. This is a critical performance optimization that avoids processing entire subtrees.

**Relative Path Normalization**: Converting absolute file paths to paths relative to the memory root directory. This makes index entries portable and enables moving the memory directory without invalidating the index.

**Job Creation**: The walker's callback creates Job structs that encapsulate file path, `os.FileInfo`, and calculated priority. These jobs are then submitted to the worker pool for parallel processing.

**Priority Calculation**: A time-based priority system where files modified recently receive higher priority for processing. The walker provides `FileInfo.ModTime()` which the daemon uses to calculate priority scores.

**Hidden File Filter**: Automatic exclusion of any file or directory whose basename starts with `.` (dot). This is built into the walker and cannot be disabled, preventing accidental processing of hidden system files.

**Callback Error Propagation**: When the visitor function returns an error, the walk immediately terminates and propagates that error to the caller. This enables early termination when errors occur or sufficient data has been collected.

**Pre-Order Traversal**: Visiting nodes in parent-before-children order, meaning directories are processed before their contents. This ordering enables directory-level filtering decisions to affect subtree processing.

**Path Normalization**: Cleaning and standardizing path representations using `filepath.Clean()` to remove redundant separators and resolve relative references like `.` and `..`.

**Graceful Degradation**: Error handling strategy where file system access errors are logged but don't stop traversal, ensuring partial success rather than all-or-nothing failure.

**Subtree Exclusion**: Preventing traversal of an entire directory tree by returning `filepath.SkipDir` when a directory matches a skip pattern, avoiding examination of potentially thousands of files.

**Extension Matching**: File extension comparison including the dot separator (e.g., `.zip`, `.tar`). Extension filtering provides efficient exclusion of entire categories of files by type rather than individual filenames.

**Visitor Pattern**: Design pattern where an algorithm (traversal) is separated from the operations performed on elements (file processing), enabling the same traversal code to support different operations through callbacks.

**Portability**: Property of index entries where relative paths remain valid across different systems, users, or directory locations, achieved through relative path computation rather than absolute path storage.
