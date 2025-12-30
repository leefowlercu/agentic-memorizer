# Skip Patterns

Configurable file and directory filtering for consistent skip behavior across the walker and watcher subsystems.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Skip Patterns subsystem provides centralized filtering logic that determines which files and directories should be excluded from indexing. Both the walker (full directory scans) and watcher (real-time monitoring) use this subsystem to ensure consistent skip behavior. The subsystem implements a two-tier filtering approach: hardcoded always-skip directories that protect system-critical paths, and configurable patterns for user customization.

The subsystem operates on path names only, not file contents, enabling fast decisions during directory traversal. It supports hidden file filtering (dot-prefixed), exact name matching for directories and files, and extension-based filtering for file types.

Key capabilities include:

- **Two-Tier Filtering** - Hardcoded always-skip directories plus configurable patterns
- **Hidden File Control** - Toggle for dot-prefixed files and directories
- **Directory Filtering** - Skip entire directory trees by name
- **File Name Filtering** - Skip specific files by exact name
- **Extension Filtering** - Skip files by extension for binary and archive types
- **Unified Interface** - Single `ShouldSkip` function for both files and directories

## Design Principles

### Always-Skip Protection

Certain directories must never be indexed regardless of configuration. The `.git` directory contains version control data that is huge and constantly changing. The `.cache` directory is the application's own cache. The `.forgotten` directory contains soft-deleted files that should not be re-indexed. These are hardcoded in `AlwaysSkipDirs` and checked before any configurable patterns.

### Configurable Hidden File Handling

Hidden files and directories (dot-prefixed) can be optionally skipped via the `SkipHidden` configuration flag. When enabled, all dot-prefixed paths are skipped except those in `AlwaysSkipDirs` (which are always skipped anyway). When disabled, users can index directories like `.github/` and `.vscode/` while the always-skip directories remain protected.

### Name-Based Matching

All skip checks use exact name matching on the final path component, not the full path. This keeps the logic simple and fast. Directory skipping uses directory names like `node_modules` or `vendor`. File skipping uses file names like `LICENSE` or `Makefile`. Extension skipping uses extensions like `.zip` or `.exe`. Pattern matching and glob syntax are not supported to maintain predictable behavior.

### Separation of Concerns

The skip subsystem only determines whether a path should be skipped. It does not perform the actual traversal, file reading, or indexing. The walker and watcher subsystems call skip functions and handle the result appropriately: the walker prunes entire directory trees when a directory is skipped, while the watcher ignores events from skipped paths.

## Key Components

### Config Struct

The `Config` struct holds all configurable skip patterns. The `SkipHidden` boolean controls whether dot-prefixed paths are skipped. The `SkipDirs` slice contains directory names to skip. The `SkipFiles` slice contains file names to skip. The `SkipExtensions` slice contains extensions (with leading dot) to skip. This struct is passed to walker and watcher for consistent behavior.

### AlwaysSkipDirs

The `AlwaysSkipDirs` variable is a hardcoded slice containing directories that are always skipped: `.git`, `.cache`, and `.forgotten`. These are checked first in `ShouldSkipDir` before any configurable patterns, ensuring they cannot be accidentally indexed even if `SkipHidden` is disabled.

### ShouldSkipDir Function

The `ShouldSkipDir` function takes a directory name and config, returning true if the directory should be skipped. It first checks against `AlwaysSkipDirs`, then checks `SkipHidden` for dot-prefixed names, and finally checks against configured `SkipDirs`. Used by both walker and watcher when encountering directories.

### ShouldSkipFile Function

The `ShouldSkipFile` function takes a file name and config, returning true if the file should be skipped. It checks `SkipHidden` for dot-prefixed names, then `SkipFiles` for exact name matches, and finally `SkipExtensions` for extension matches. Does not check the containing directory path.

### ShouldSkip Function

The `ShouldSkip` function provides a unified interface taking a name, `isDir` boolean, and config. It delegates to `ShouldSkipDir` or `ShouldSkipFile` based on the type flag. This simplifies caller code that needs to handle both files and directories.

## Integration Points

### Walker Subsystem

The walker uses skip functions during recursive directory traversal. When `ShouldSkipDir` returns true, the walker prunes the entire directory tree by returning `filepath.SkipDir`. When `ShouldSkipFile` returns true, the walker simply skips that file without affecting traversal. The walker receives a `Config` from daemon configuration.

### Watcher Subsystem

The watcher uses skip functions when receiving filesystem events. When a create or modify event occurs for a path that matches skip patterns, the event is ignored. When a new directory is detected, `ShouldSkipDir` determines whether to add recursive watches. The watcher receives a `Config` from daemon configuration.

### Configuration Subsystem

The config subsystem provides skip pattern configuration via `daemon.skip_hidden`, `daemon.skip_dirs`, `daemon.skip_files`, and `daemon.skip_extensions` settings. The config subsystem constructs a `skip.Config` struct that is passed to walker and watcher at initialization and can be updated via hot-reload.

### Daemon Subsystem

The daemon orchestrates skip pattern usage by loading configuration, constructing a `skip.Config`, and passing it to both walker (for rebuilds) and watcher (for real-time monitoring). Configuration changes trigger updates to both subsystems for consistent behavior.

## Glossary

**Always-Skip Directories**
Hardcoded directories that are never indexed regardless of configuration: `.git`, `.cache`, and `.forgotten`. Protected to prevent indexing of version control data, cache files, and soft-deleted content.

**Extension Filtering**
Skipping files based on their extension (e.g., `.zip`, `.exe`). Extensions must include the leading dot and are matched exactly.

**Hidden Files**
Files or directories whose names begin with a dot (e.g., `.gitignore`, `.config/`). Commonly used in Unix systems for configuration and system files. Controlled by the `SkipHidden` setting.

**Skip Pattern**
A name or extension used to determine whether a file or directory should be excluded from indexing. Patterns use exact matching, not glob or regex syntax.

**Two-Tier Filtering**
The filtering approach combining hardcoded always-skip directories (first tier) with configurable patterns (second tier). The first tier provides protection that cannot be overridden.
