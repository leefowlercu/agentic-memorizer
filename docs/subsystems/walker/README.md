# Directory Walker

Recursive directory scanning with configurable filtering for full directory scans during initialization and rebuilds.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-31

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Directory Walker subsystem provides recursive directory scanning for discovering files during daemon initialization and rebuild operations. While the Watcher subsystem handles real-time monitoring of file changes, the Walker subsystem performs full directory traversals to build the initial file inventory or rebuild after configuration changes. The subsystem wraps Go's filepath.Walk with configurable filtering for directories, files, and extensions.

The subsystem is intentionally minimal and focused: a single Walk function with visitor pattern, plus a helper for relative path computation. Filtering happens at two levels: directory pruning (skipping entire directory trees) and file filtering (skipping individual files by name or extension). Hidden files and directories (those starting with a dot) are automatically excluded.

Key capabilities include:

- **Recursive traversal** - Walks entire directory trees using filepath.Walk
- **Directory pruning** - Skip entire directory subtrees for performance (e.g., node_modules, .git)
- **File filtering** - Skip files by exact name or extension match
- **Hidden path exclusion** - Automatic exclusion of dot-prefixed files and directories
- **Visitor pattern** - Callback function for flexible file processing
- **Error tolerance** - Continues walking after access errors with stderr warnings

## Design Principles

### Visitor Pattern for Flexibility

Rather than returning a list of files, the Walk function accepts a FileVisitor callback that is invoked for each discovered file. This pattern enables the caller to process files during traversal, reducing memory usage for large directories. The visitor receives both the absolute path and os.FileInfo, providing all metadata needed for processing decisions.

### Two-Tier Filtering

Filtering operates at two distinct levels for optimal performance. Directory filtering prunes entire subtrees, preventing descent into directories like node_modules or .git that could contain thousands of irrelevant files. File filtering removes individual files that don't match criteria but doesn't affect directory traversal. This separation ensures maximum pruning benefit while maintaining file-level control.

### Map-Based Lookup

Skip patterns are converted to maps at the start of Walk for O(1) lookup during traversal. Skip directories become absolute path keys for exact matching. Skip files and extensions use basename/extension keys for name matching. This optimization matters for directories with many files where linear search would be costly.

### Graceful Error Handling

Access errors (permission denied, broken symlinks) are logged to stderr but don't stop the walk. This ensures that one inaccessible file or directory doesn't prevent discovery of all other files. The Walk function returns nil for access errors, allowing traversal to continue. Visitor errors, however, propagate immediately to stop processing.

### Path Cleaning

The root path is cleaned via filepath.Clean at entry to handle malformed paths with extra slashes, dots, or other anomalies. This ensures consistent path handling regardless of how the caller specifies the root directory.

### Minimal Surface Area

The subsystem exports only two functions: Walk for traversal and GetRelPath for path computation. This minimal API prevents misuse and keeps the subsystem focused on its core responsibility. All internal details (map construction, skip logic) remain unexported.

## Key Components

### Walk Function

The primary entry point for directory traversal. Walk accepts a root path, three skip lists (directories, files, extensions), and a FileVisitor callback. It constructs lookup maps from skip lists, wraps filepath.Walk with filtering logic, and invokes the visitor for each non-skipped file. The function returns an error only if the visitor returns an error or filepath.Walk encounters a fatal error.

### FileVisitor Type

A function type defining the callback signature: `func(path string, info os.FileInfo) error`. The visitor receives the absolute file path and file info. Returning nil continues the walk; returning an error stops immediately. This type is exported for caller use.

### Directory Skip Logic

When encountering a directory, Walk checks two conditions. Hidden directories (names starting with dot) trigger filepath.SkipDir to prune the subtree. Directories in the skip list (matched by absolute path) also trigger SkipDir. All other directories are entered for further traversal.

### File Skip Logic

When encountering a file, Walk checks three conditions. Hidden files (names starting with dot) are silently skipped. Files whose basename matches the skip files list are skipped. Files whose extension matches the skip extensions list are skipped. Only files passing all checks reach the visitor.

### GetRelPath Function

A helper function computing the relative path from a root to a target path. Wraps filepath.Rel with error handling using the project's semicolon-style error formatting. Used by callers to convert absolute paths from the visitor into relative paths for storage and display.

### Error Handling

Access errors during traversal print a warning to stderr and return nil to continue. This non-fatal handling ensures maximum file discovery despite access issues. Visitor errors propagate unchanged, giving callers control over stopping conditions.

## Integration Points

### Daemon Subsystem

The daemon uses Walk during initialization and rebuild operations. At startup, Walk scans the memory root to discover all files for initial indexing. During rebuilds triggered by `daemon rebuild`, Walk provides the complete file inventory for reprocessing. The daemon passes its skip configuration (directories, files, extensions) directly to Walk.

### Watcher Subsystem

Walker and Watcher serve complementary purposes. Walker provides point-in-time snapshots for initialization and rebuilds. Watcher provides continuous monitoring between snapshots. Both use the same skip configuration to ensure consistent file discovery. The daemon coordinates between them: Walker for bulk operations, Watcher for incremental updates.

### Configuration Subsystem

Skip patterns come from the configuration subsystem. The daemon reads skip_directories, skip_files, and skip_extensions from config and passes them to Walk. Configuration changes to skip patterns require daemon restart as they affect the static Walk parameters.

### Worker Pool

Files discovered by Walk flow to the daemon's worker pool for processing. The daemon's visitor callback creates processing jobs for each discovered file. The worker pool then handles metadata extraction, semantic analysis, and graph storage in parallel.

## Glossary

**Directory Pruning**
Skipping an entire directory and all its contents during traversal. Implemented via filepath.SkipDir return value. Critical for performance with large excluded directories.

**Extension Filter**
A file extension (e.g., ".tmp", ".bak") used to skip files during traversal. Extensions include the leading dot and are matched exactly.

**File Filter**
A filename (e.g., ".DS_Store", "Thumbs.db") used to skip files during traversal. Matched against the file's basename exactly.

**FileVisitor**
A callback function invoked for each discovered file. Receives path and FileInfo, returns error to stop or nil to continue.

**Hidden Path**
A file or directory whose name starts with a dot (.). Hidden paths are automatically excluded from traversal without explicit skip configuration.

**Relative Path**
A file path expressed relative to the memory root directory. Computed from absolute paths using GetRelPath for storage and display.

**Skip List**
A collection of patterns (directories, files, or extensions) to exclude from traversal. Converted to maps for O(1) lookup during walking.

**Walker**
The subsystem performing full directory scans. Complements the Watcher subsystem which handles real-time monitoring.

**Watcher**
The complementary subsystem providing real-time file change detection. Uses similar skip configuration as Walker for consistent discovery.
