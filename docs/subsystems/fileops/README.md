# File Operations

Filesystem utilities for copying and moving files with automatic conflict resolution, cross-filesystem support, and batch operations.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The File Operations subsystem provides safe, reliable file manipulation utilities for the `remember file` and `forget file` CLI commands. It handles the complexities of copying and moving files across different filesystems, automatically resolving naming conflicts, and supporting batch operations that continue on individual failures.

The subsystem operates independently of the daemon, performing direct filesystem operations. The daemon's file watcher then detects changes and updates the knowledge graph accordingly. This separation ensures that file mutations remain simple and predictable while the daemon handles indexing concerns asynchronously.

Key capabilities include:

- **Copy and Move Operations** - File and directory copy/move with recursive directory support
- **Automatic Conflict Resolution** - Appends `-N` suffix to avoid overwriting existing files
- **Cross-Filesystem Moves** - Transparent fallback to copy+delete when rename fails across mount points
- **Batch Processing** - Process multiple files with continue-on-failure semantics
- **Path Validation** - Security checks to prevent directory traversal attacks
- **Permission Preservation** - Maintains source file permissions on copy

## Design Principles

### Conflict Resolution via Suffix Pattern

When a destination path already exists, the subsystem automatically finds a non-conflicting name by appending a `-N` suffix (where N starts at 1 and increments). For files with extensions, the suffix is inserted before the extension: `file.md` becomes `file-1.md`. For compound extensions like `.tar.gz`, the entire compound extension is preserved: `archive.tar.gz` becomes `archive-1.tar.gz`. This predictable pattern enables users to identify renamed files easily.

### Cross-Filesystem Transparency

Move operations first attempt a direct `os.Rename()`, which is atomic and efficient when source and destination are on the same filesystem. When this fails with `EXDEV` (cross-device link error), the operation transparently falls back to copy+delete. The result struct indicates whether cross-filesystem fallback occurred, enabling callers to log or handle this case differently if needed.

### Continue-on-Failure Batch Semantics

Batch operations (`CopyBatch`, `MoveBatch`) process all items regardless of individual failures. Results and errors are returned as parallel slices indexed to match input items. This allows callers to report all errors at once rather than failing on the first error, providing a better user experience for multi-file operations.

### Path Security Validation

Path validation functions reject dangerous patterns before filesystem operations occur. Paths containing parent directory references (`..`) or null bytes are rejected. Subdirectory validation additionally rejects absolute paths when relative paths are expected. These checks prevent directory traversal attacks when constructing paths from user input.

### Permission Preservation

File copies preserve the source file's permission mode. Destination directories are created with 0755 permissions. This ensures that executable files remain executable and read-only files remain read-only after copying.

## Key Components

### Copy Module

The copy module (`copy.go`) provides file and directory copying. The `Copy` function handles both files and directories, automatically creating destination directories and resolving conflicts. The `CopyBatch` function processes multiple copy operations with continue-on-failure semantics. The `CopyToDir` convenience function copies a file into a directory while preserving its original name.

Result types (`CopyItem`, `CopyResult`) track source and destination paths, bytes copied, file counts for directories, and whether conflict resolution renamed the destination.

### Move Module

The move module (`move.go`) provides file and directory moving with cross-filesystem support. The `Move` function first attempts `os.Rename()` for efficiency, then falls back to copy+delete when crossing filesystem boundaries. The `MoveBatch` function processes multiple moves with continue-on-failure semantics. The `MoveToDir` convenience function moves a file into a directory while preserving its name.

Result types (`MoveItem`, `MoveResult`) track source and destination paths, file counts, whether conflict resolution occurred, and whether cross-filesystem fallback was used.

### Conflict Resolution Module

The conflict module (`conflict.go`) handles naming conflicts. The `ResolveConflict` function finds a non-conflicting name by incrementing the `-N` suffix until an available name is found, with an upper limit of 10,000 attempts. The `splitNameAndExtensions` function correctly handles compound extensions like `.tar.gz` and hidden files like `.gitignore`. Helper functions `HasConflictSuffix` and `GetConflictNumber` allow inspection of conflict suffixes.

### Path Utilities Module

The paths module (`paths.go`) provides path validation and manipulation. `IsInDirectory` checks if a path is within a directory tree. `ValidatePath` and `ValidateSubdirectory` reject dangerous patterns. `EnsureDir` creates directories idempotently. `RelativePath` computes relative paths with escape detection. `ExpandHome` handles tilde expansion. Predicate functions `PathExists`, `IsFile`, and `IsDir` provide common path checks.

## Integration Points

### CLI Commands

The `remember file` command uses `Copy` and `CopyBatch` to copy files into the memory directory. The `forget file` command uses `Move` and `MoveBatch` to move files from the memory directory to the `.forgotten` directory. Both commands use path validation functions to sanitize user input.

### Daemon Watcher

The fileops subsystem does not interact directly with the daemon. After file operations complete, the daemon's file watcher (fsnotify) detects the changes and triggers graph updates. This loose coupling keeps file operations simple and allows the daemon to batch related changes.

### Configuration Subsystem

The fileops subsystem reads `memory.root` from configuration to determine the memory directory path. The CLI commands handle configuration loading; fileops receives paths directly.

## Glossary

**Conflict Resolution**
The process of finding a non-conflicting filename when the destination already exists. Uses `-N` suffix pattern where N increments until an available name is found.

**Compound Extension**
A multi-part file extension like `.tar.gz` or `.tar.bz2` that is treated as a single unit during conflict resolution to preserve archive type information.

**Cross-Filesystem Move**
A move operation where source and destination are on different mount points. Requires copy+delete fallback since `os.Rename()` only works within a single filesystem.

**EXDEV**
The Unix error code for "cross-device link" returned when attempting to rename a file across filesystem boundaries.
