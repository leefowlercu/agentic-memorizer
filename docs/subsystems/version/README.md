# Version Management

Build-time version injection with embedded fallback and Go build info integration.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Version Management subsystem provides version, commit, and build date information for the application. It supports three sources of version data with clear precedence: ldflags injection at build time (highest priority), embedded VERSION file (fallback for version), and Go's runtime/debug build info (fallback for commit and date). This design ensures version information is always available regardless of build method.

The subsystem exposes a minimal API: four getter functions for different version representations. GetVersion returns a formatted string with all components. GetShortVersion returns just the version number. GetGitCommit and GetBuildDate return individual components. The internal getters handle source selection and formatting.

Key capabilities include:

- **Ldflags injection** - Build-time version, commit, and date via -X flags
- **Embedded fallback** - VERSION file embedded via go:embed for version number
- **Build info integration** - runtime/debug provides commit and date when ldflags not set
- **Dirty detection** - Marks commit with -dirty suffix when workspace has uncommitted changes
- **Short commit hash** - Automatically truncates commit to 7 characters for display
- **Formatted output** - GetVersion combines all components in standard format

## Design Principles

### Three-Tier Source Precedence

Version information uses a clear priority chain. Ldflags values (set via -X flags during build) take highest priority, enabling release builds to inject exact version strings. When ldflags are not set, the embedded VERSION file provides the version number. For commit and date, runtime/debug build info serves as secondary fallback, extracting VCS information from the Go build system.

### Embedded Version File

The VERSION file is embedded at compile time via go:embed directive. This ensures a version number is always available even for simple `go build` without ldflags. The file contains just the version number (e.g., "0.13.0"), trimmed of whitespace. This file is the source of truth for version during development.

### Build Info Extraction

Go's runtime/debug.ReadBuildInfo provides VCS metadata when the binary was built with Go modules. The subsystem extracts vcs.revision (commit hash) and vcs.time (build timestamp) from the build settings. This enables version information for `go install` users who don't use the Makefile's ldflags.

### Dirty State Detection

When using build info, the subsystem checks vcs.modified to detect uncommitted changes. If modified is true, the commit hash is suffixed with "-dirty" to indicate the build doesn't represent a clean commit. This helps debugging by identifying development builds.

### Minimal API Surface

The subsystem exports only four getter functions, keeping the interface simple. Internal functions (lowercase) handle source selection and formatting. Package variables are exported for ldflags injection but not for direct consumer use. This separation keeps the public API clean while enabling build-time customization.

### Graceful Unknown Handling

When no source provides a value, the subsystem returns "unknown" rather than empty string or panicking. This ensures version output is always printable and parseable. The full version format "v0.13.0 (commit: unknown, built: unknown)" remains valid even with missing data.

## Key Components

### Package Variables

Three exported variables receive ldflags injection: Version for the version string, GitCommit for the commit hash, and BuildDate for the build timestamp. These start empty or "unknown" and are overwritten at link time when using `-ldflags "-X ..."`. The variables must be exported for ldflags to work.

### Embedded VERSION File

The VERSION file in the package directory contains the current version number. The go:embed directive includes it in the binary at compile time. The embeddedVersion variable holds the file contents, accessed via getVersion() when the Version variable is empty.

### getVersion Function

The internal getVersion function implements version source selection. If the Version variable is non-empty (ldflags set), it returns that value. Otherwise, it returns the embedded version with whitespace trimmed. This provides the version number without metadata.

### getGitCommit Function

The internal getGitCommit function implements commit source selection. If GitCommit is not "unknown" (ldflags set), it returns that value. Otherwise, it reads build info and extracts vcs.revision. The commit is truncated to 7 characters for display. If vcs.modified is true, "-dirty" is appended. Returns "unknown" if no source available.

### getBuildDate Function

The internal getBuildDate function implements date source selection. If BuildDate is not "unknown" (ldflags set), it returns that value. Otherwise, it reads build info and extracts vcs.time. Returns "unknown" if no source available.

### GetVersion Function

The public GetVersion function returns the formatted full version string: "{version} (commit: {commit}, built: {date})". It calls the three internal getters and combines them. This is the primary function for version command output.

### GetShortVersion Function

The public GetShortVersion function returns just the version number without metadata. Calls getVersion() internally. Used where a compact version string is needed.

### GetGitCommit and GetBuildDate Functions

Public wrappers around the internal getters. Provide individual component access for specialized use cases like logging or API responses.

## Integration Points

### CLI Version Command

The root command's version flag and version subcommand use GetVersion() for output. The formatted string appears in `memorizer version` and `memorizer --version` output. This is the primary consumer of version information.

### Daemon and MCP Logging

The daemon and MCP server log version information at startup. GetShortVersion() provides compact version for log lines. GetGitCommit() enables log correlation with specific commits during debugging.

### Integration Adapters

Integration adapters include version in their metadata. The version appears in adapter Version() methods used for compatibility checking. Adapters can embed version in configuration files they generate.

### HTTP API Headers

The daemon HTTP API can include version in response headers. X-Version headers enable clients to detect server version for compatibility. GetShortVersion() provides the header value.

### Makefile Build Process

The Makefile's build-release target sets ldflags from git tags and commit info. Version comes from git describe or VERSION file. Commit comes from git rev-parse HEAD. Date comes from current timestamp. These flow to the package variables at link time.

### GoReleaser Integration

GoReleaser builds set ldflags automatically during release. The goreleaser.yml configuration defines ldflags matching the package variable paths. This ensures release binaries have exact version information matching git tags.

## Glossary

**Build Info**
Runtime information available via runtime/debug.ReadBuildInfo. Contains VCS metadata when built with Go modules, including revision, time, and modified state.

**Dirty**
A suffix indicating uncommitted changes in the working directory at build time. Appended to commit hash as "-dirty" to distinguish development builds.

**Embed**
Go's compile-time file inclusion via //go:embed directive. The VERSION file is embedded into the binary, available without external file access.

**GitCommit**
The git commit hash identifying the source code version. Set via ldflags or extracted from build info.

**Ldflags**
Linker flags used to set package variables at build time. The -X flag syntax enables injecting version strings during go build.

**Short Version**
The version number without additional metadata. Just "0.13.0" versus "0.13.0 (commit: abc123, built: 2024-01-01)".

**VCS Settings**
Key-value pairs in build info containing version control information. Keys include vcs.revision, vcs.time, and vcs.modified.

**VERSION File**
A text file containing the current version number. Embedded in the binary as fallback when ldflags not provided.
