# Version Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Build-Time Injection Pattern](#build-time-injection-pattern)
   - [Default Values for Development](#default-values-for-development)
   - [Separation of Concerns](#separation-of-concerns)
   - [Two-Level Version Tracking](#two-level-version-tracking)
3. [Key Components](#key-components)
   - [Package Variables](#package-variables)
   - [VERSION File](#version-file)
   - [Version Functions](#version-functions)
   - [Build System Integration](#build-system-integration)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Index Manager](#index-manager)
   - [MCP Server](#mcp-server)
   - [CLI Commands](#cli-commands)
5. [Glossary](#glossary)

## Overview

The Version subsystem is a lightweight build-time metadata injection system that provides version, commit, and build date information throughout the agentic-memorizer application. It serves as a tracking mechanism for operational visibility, debugging, and compatibility verification between different versions of the daemon and index files.

The subsystem uses Go's linker flags mechanism to inject version information at build time rather than hardcoding it in source files. This approach ensures version information is automatically synchronized with git tags and commits, providing accurate build provenance without requiring manual code updates.

The Version subsystem provides three key pieces of information: application semantic version (identifying releases), git commit hash (enabling exact source identification), and build timestamp (tracking when binaries were compiled). These components combine to create comprehensive version strings that appear in logs, index files, and status displays throughout the application.

## Design Principles

### Build-Time Injection Pattern

The Version subsystem implements build-time variable injection using Go's linker flags, enabling version information to be set during compilation without modifying source code.

**Mechanism:**
The subsystem defines package-level variables with safe default values. During compilation, the build system uses `-ldflags` with the `-X` flag to set these variables to actual values derived from git tags, commit hashes, and current timestamps. This injection happens at link time, after compilation but before the final binary is produced.

**Benefits:**
Build-time injection eliminates manual version management in source code, preventing version strings from becoming stale or inconsistent. Version information automatically synchronizes with git tags, ensuring the binary version matches the repository state. The single source of truth is the git repository itself, with tags defining releases and commits providing exact provenance.

**Implementation Pattern:**
The Go linker's `-X` flag accepts a full import path to a string variable and a value to assign. For the Version subsystem, the full path is `github.com/leefowlercu/agentic-memorizer/internal/version.Version` (or GitCommit, BuildDate). Build commands construct ldflags with shell commands that extract git information, passing these values to the linker for injection.

**Development vs Production:**
Simple builds using `go build` without ldflags rely on automatic fallbacks (embedded VERSION file and VCS metadata from `runtime/debug.ReadBuildInfo()`). Makefile-driven builds using `make build` explicitly set all version metadata via ldflags for complete control. This flexibility enables developers to work with simple commands while allowing the build system to provide precise version information when needed.

### Default Values and Fallback Behavior

The Version subsystem prioritizes developer experience by providing meaningful defaults and automatic fallback mechanisms that enable builds without requiring explicit build-time injection setup.

**Default Values:**
- Version: `""` (empty string) - Falls back to embedded VERSION file content (currently "0.12.1")
- GitCommit: "unknown" - Falls back to VCS metadata from `runtime/debug.ReadBuildInfo()` when available
- BuildDate: "unknown" - Falls back to VCS commit time from `runtime/debug.ReadBuildInfo()` when available

**Fallback Mechanisms:**
The subsystem implements intelligent fallback behavior using Go's build metadata when ldflags are not provided:
1. **Version Fallback**: When the Version variable remains empty, `getVersion()` returns the content of the embedded VERSION file, providing the current release version
2. **Commit Fallback**: When GitCommit is "unknown", `getGitCommit()` queries `runtime/debug.ReadBuildInfo()` for VCS revision information, extracting the first 7 characters of the commit hash with a `-dirty` suffix if uncommitted changes exist
3. **Build Date Fallback**: When BuildDate is "unknown", `getBuildDate()` queries build metadata for VCS commit timestamp

**Automatic Version Information:**
These fallbacks ensure that binaries built with standard commands like `go build` or `go install` still receive version information automatically from Go's build system. The version comes from the embedded VERSION file, while commit and date information are extracted from VCS metadata when the build occurs within a git repository.

**Development vs Production:**
While fallback values provide reasonable defaults, using the Makefile's `build` or `install` targets provides more control by explicitly setting all version metadata via ldflags. The ldflags approach allows customization of version strings (e.g., using `git describe --tags --always --dirty`) and ensures consistent version formatting across builds.

### Three-Tier Fallback System

The Version subsystem implements a sophisticated three-tier fallback mechanism that ensures binaries always have valid version information regardless of how they're built.

**Priority Hierarchy:**
Each version metadata component (Version, GitCommit, BuildDate) uses a priority-based fallback chain:
1. **Priority 1 - Explicit ldflags**: Values set via `-ldflags "-X ..."` during build (highest priority)
2. **Priority 2 - Go Build Metadata**: Automatic extraction from `runtime/debug.ReadBuildInfo()` VCS information
3. **Priority 3 - Defaults**: Hardcoded fallback values ("unknown" for commit/date, embedded VERSION file for version)

**Version String Fallback:**
The `getVersion()` function (`internal/version/version.go:29-34`) implements version fallback:
- Checks if `Version` variable is non-empty (set via ldflags)
- If empty, returns `strings.TrimSpace(embeddedVersion)` from the `//go:embed VERSION` directive
- This ensures all binaries have the current release version even without ldflags

**Git Commit Fallback:**
The `getGitCommit()` function (`internal/version/version.go:37-75`) implements commit hash fallback:
- Checks if `GitCommit` variable is not "unknown" (set via ldflags) - if so, returns immediately
- Calls `debug.ReadBuildInfo()` to access VCS metadata embedded by Go's build system
- Extracts `vcs.revision` (commit hash) and `vcs.modified` (dirty flag) from build settings
- Truncates commit hash to first 7 characters for brevity
- Appends `-dirty` suffix if `vcs.modified` is true
- Returns "unknown" only if build metadata is unavailable

**Build Date Fallback:**
The `getBuildDate()` function (`internal/version/version.go:78-97`) implements build date fallback:
- Checks if `BuildDate` variable is not "unknown" (set via ldflags) - if so, returns immediately
- Calls `debug.ReadBuildInfo()` to access VCS metadata
- Extracts `vcs.time` (commit timestamp) from build settings
- Returns "unknown" only if build metadata is unavailable

**When BuildInfo Is Available:**
Go's build system automatically embeds VCS information when building from a git repository using `go build` or `go install` with Go 1.18+. This means most development builds automatically receive commit and date information without requiring any special flags or Makefile usage.

**Fallback Benefits:**
This three-tier system provides several advantages:
- Developers can use simple `go build` commands and still get version information
- Binaries built with `go install` automatically include VCS metadata
- Makefile builds can override with custom version strings when needed
- No build ever produces a binary with completely missing version information
- Version tracking works across different build methods (direct Go commands, Makefile, CI/CD pipelines)

### Separation of Concerns

The Version subsystem centralizes all version management logic in a single package, providing a clean interface for version information consumption throughout the application.

**Single Definition Point:**
All version-related variables and functions reside in `internal/version/version.go`. This centralization prevents duplication of version logic and ensures consistent version string formatting across all consumers. Changes to version handling require updates in only one location.

**Multiple Consumption Points:**
The daemon uses version information for startup logging and index file metadata. The index manager stores version information in computed index files. Status commands display version information to users. Each consumer imports the version package and calls its public functions, maintaining loose coupling.

**Clean Interface:**
The subsystem exposes only two public functions: `GetVersion()` for complete version information and `GetShortVersion()` for simple version display. Package-level variables remain accessible for advanced use cases but are not the primary interface. This design hides implementation details while providing convenient access patterns.

### Two-Level Version Tracking

The application distinguishes between schema versions (data format versions) and application versions (software release versions), enabling independent evolution of data structures and application logic.

**Schema Versions:**
Schema versions track the format of data structures like index files. The `ComputedIndex.Version` field stores the index format version (currently "1.0"), indicating which fields and structures the file contains. Schema versions change when index format evolves, such as adding new fields or changing data representations.

**Application Versions:**
Application versions track software releases. The `ComputedIndex.DaemonVersion` field stores the application version from the Version subsystem (e.g., "v0.6.0"), indicating which daemon release created the index. Application versions change with each release, following semantic versioning conventions.

**Independent Evolution:**
This two-level approach allows index format to remain stable (schema version "1.0") across multiple application releases (v0.5.0, v0.6.0, etc.). Conversely, index format can evolve (schema version "2.0") without requiring application version changes. The separation provides flexibility for maintenance and compatibility management.

**Compatibility Tracking:**
Storing both versions in index files enables sophisticated compatibility checking. The daemon can verify whether it understands the schema version before reading an index. Operators can identify which daemon version created problematic indexes by examining the daemon version field. This dual tracking supports graceful upgrades and debugging.

## Key Components

### Package Variables

The Version subsystem defines three package-level string variables that serve as injection targets for build-time metadata.

**Version Variable:**
Stores the application semantic version following semver conventions (e.g., "v0.12.1"). The default value is `""` (empty string). When empty, the `getVersion()` function returns the content of the embedded VERSION file (via `//go:embed VERSION` directive). Build systems can inject version values from git tags using `git describe --tags --always` or explicit tag values through ldflags. This variable identifies release versions and appears in user-facing output.

**GitCommit Variable:**
Stores the git commit hash that identifies the exact source code state. The default value is "unknown". When not set via ldflags, the `getGitCommit()` function attempts to extract commit information from Go's build metadata using `runtime/debug.ReadBuildInfo()`. This automatic extraction provides a 7-character short hash when available. When set via ldflags using `git rev-parse HEAD`, this variable contains the full 40-character hexadecimal string. A `-dirty` suffix is appended when the workspace has uncommitted changes. This variable enables tracing binaries back to their precise source code, supporting debugging and security audits.

**BuildDate Variable:**
Stores the ISO 8601 UTC timestamp when the binary was compiled. The default value is "unknown". When not set via ldflags, the `getBuildDate()` function attempts to extract build time from Go's build metadata using `runtime/debug.ReadBuildInfo()`. This automatic extraction provides the VCS commit timestamp when available. When set via ldflags, the value comes from `date -u +%Y-%m-%dT%H:%M:%SZ`, producing timestamps like "2025-11-01T17:39:22Z". This variable helps track binary age and deployment timing.

**Injection Mechanism:**
All three variables are string types, matching the `-X` flag's requirements. The variables are package-level (not const) to allow linker modification. Full import paths are required for injection: `github.com/leefowlercu/agentic-memorizer/internal/version.Variable`.

### VERSION File

The VERSION file serves as the canonical source of truth for the application's semantic version number, providing a stable version identifier that persists across builds.

**File Location:**
The VERSION file resides at `internal/version/VERSION` and contains only the semantic version number without the `v` prefix (e.g., `0.12.1`). This plain text file is committed to version control and updated by release automation scripts.

**Multiple Consumers:**
The VERSION file serves three distinct purposes in the build system:
1. **Binary Embedding**: The file is embedded into compiled binaries via `//go:embed VERSION` directive, providing a fallback version when ldflags are not used
2. **Makefile Integration**: The Makefile reads this file to set the `CURRENT_VERSION` variable for display and validation purposes
3. **Release Automation**: Release scripts update this file as part of the version bump process, ensuring version consistency across the system

**Fallback Mechanism:**
When the `Version` package variable is not set via ldflags (i.e., remains empty string), the `getVersion()` function returns the embedded VERSION file content. This ensures binaries always have a valid version identifier even when built with simple `go build` commands without custom ldflags.

**Version Bumping:**
The VERSION file is updated programmatically by the `scripts/bump-version.sh` script during the release preparation process. The script reads the current version, calculates the next version based on semver rules (major, minor, or patch bump), and writes the new version back to the file. This automation prevents manual version management errors and ensures consistency.

**Design Rationale:**
Storing the version in a dedicated file rather than a constant in code enables version updates without modifying Go source files. This separation supports clean release automation where version bumps can be performed by scripts without parsing or editing Go code. The embedded file approach provides a reliable fallback while still allowing ldflags to override for more detailed versioning.

### Version Functions

The Version subsystem exposes four public functions that provide version information for different use cases and display contexts.

**GetVersion Function:**
Returns a comprehensive version string combining all three metadata components in the format: `"<version> (commit: <hash>, built: <date>)"`. For example: `"v0.12.1 (commit: a942b5c, built: 2025-11-01T17:39:22Z)"`. This function provides complete version information suitable for logging, debugging output, and detailed status displays.

**Use Cases:**
- Daemon startup logging for operational visibility
- Index file metadata for compatibility tracking
- Error reports that need full build context
- Support requests requiring exact binary identification

**GetShortVersion Function:**
Returns just the version number without commit or build metadata, in the format: `"<version>"`. For example: `"v0.12.1"` or `"0.12.1"`. This function internally calls `getVersion()`, which returns either the ldflags-injected version or the embedded VERSION file content.

**Use Cases:**
- Brief version displays in CLI output
- API responses that include version information
- Log messages where full metadata is excessive
- User-facing version indicators in help text

**GetGitCommit Function:**
Returns just the git commit hash without other metadata. The return value depends on the fallback tier: ldflags-injected value (full 40-char hash), BuildInfo extraction (7-char hash with optional `-dirty` suffix), or "unknown". For example: `"a942b5c"`, `"d147fda-dirty"`, or `"a942b5c1234567890abcdef1234567890abcdef"`.

**Use Cases:**
- Individual component display in version command (`cmd/version/version.go:38`)
- Detailed debugging output where commit needs separate formatting
- Systems that track commit hashes independently from full version strings

**GetBuildDate Function:**
Returns just the build timestamp without other metadata. The return value depends on the fallback tier: ldflags-injected ISO 8601 timestamp, BuildInfo VCS commit time, or "unknown". For example: `"2025-11-22T14:30:00Z"` or `"2025-11-20T20:00:00Z"`.

**Use Cases:**
- Individual component display in version command (`cmd/version/version.go:39`)
- Build age calculations and deployment tracking
- Systems that need to compare build timestamps independently

**Function Design:**
All four functions are simple accessors that call internal getter functions or format package variables. They contain minimal logic, ensuring reliability and minimal performance overhead. The functions can be called from any goroutine safely since they only read package variables or call thread-safe getter functions.

### Build System Integration

The Version subsystem integrates with the build system through linker flags that inject metadata during compilation. The project's Makefile provides build targets that automatically inject version information.

**Makefile Integration:**
The Makefile defines version variables and ldflags automatically (`Makefile:7-15`):
```makefile
VERSION_FILE=internal/version/VERSION
CURRENT_VERSION=$(shell cat $(VERSION_FILE) 2>/dev/null || echo "0.0.0")
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/leefowlercu/agentic-memorizer/internal/version.Version=$(VERSION) \
           -X github.com/leefowlercu/agentic-memorizer/internal/version.GitCommit=$(GIT_COMMIT) \
           -X github.com/leefowlercu/agentic-memorizer/internal/version.BuildDate=$(BUILD_DATE)
```

**Build Targets:**
- `make build` - Build binary with version information injected via ldflags
- `make install` - Install binary with version information to `~/.local/bin`

Both targets use the same ldflags to inject version, commit, and build date. There is no distinction between "development" and "release" builds at the Makefile level—all Makefile builds inject version information.

**Injection Commands:**
The ldflags contain `-X` flags for each variable:
- Version: `-X github.com/leefowlercu/agentic-memorizer/internal/version.Version=$(VERSION)`
  - Derives from `git describe --tags --always --dirty`
  - Includes tag name, commits since tag, short commit hash, and dirty flag if uncommitted changes exist
  - Falls back to "dev" if git is unavailable
- GitCommit: `-X github.com/leefowlercu/agentic-memorizer/internal/version.GitCommit=$(GIT_COMMIT)`
  - Derives from `git rev-parse HEAD`
  - Provides full 40-character commit SHA
  - Falls back to "unknown" if git is unavailable
- BuildDate: `-X github.com/leefowlercu/agentic-memorizer/internal/version.BuildDate=$(BUILD_DATE)`
  - Derives from `date -u +%Y-%m-%dT%H:%M:%SZ`
  - Produces UTC timestamp in ISO 8601 format

**Git Integration:**
Version information derives from git commands executed during build:
- `git describe --tags --always --dirty` produces descriptive version string (e.g., "v0.12.1" for tagged release, "v0.12.0-5-ga942b5c-dirty" for development)
- `git rev-parse HEAD` produces full commit SHA
- `date -u +%Y-%m-%dT%H:%M:%SZ` produces UTC timestamp in ISO 8601 format

**VERSION File Integration:**
The Makefile reads `internal/version/VERSION` to set `CURRENT_VERSION` variable for display purposes (used in build output messages). This file provides the canonical version number that will be used for the next release and serves as the embedded version fallback in binaries built without ldflags.

**Build Automation:**
The Makefile automates ldflags construction, extracting git information and constructing injection commands. This automation ensures version information is always current and accurate without manual intervention. The fallback values ("dev", "unknown") ensure builds succeed even in non-git environments.

## Integration Points

### Daemon Subsystem

The Daemon subsystem uses version information for operational logging and index file metadata, providing visibility into daemon version during operation and enabling compatibility tracking.

**Startup Logging:**
When the daemon starts, it logs the full version information using `version.GetVersion()`. This log entry appears in the daemon log file and provides critical context for troubleshooting. Operators can identify which daemon version is running, when it was built, and from which commit. This information is essential for correlating daemon behavior with code changes.

**Index File Metadata:**
The daemon passes its version to the index manager when writing index files. The index manager stores this version in the `ComputedIndex.DaemonVersion` field, creating a permanent record of which daemon version generated each index. This metadata enables compatibility checking, debugging, and version migration strategies.

**Operational Visibility:**
Version information in logs and index files provides operational visibility into the system state. Operators can verify deployed versions, track version consistency across index files, and identify version-related issues. This visibility is crucial for production environments with multiple daemon instances or frequent updates.

### Index Manager

The Index Manager subsystem stores daemon version information in computed index files, enabling version tracking and compatibility verification.

**Version Storage:**
When writing index files, the index manager includes the daemon version in the `ComputedIndex` structure. This structure contains:
- `Version` field: Index schema version (e.g., "1.0")
- `DaemonVersion` field: Application version from Version subsystem (e.g., "v0.12.1 (commit: a942b5c, built: 2025-11-01T17:39:22Z)")

**Separation from Schema Version:**
The daemon version is distinct from the index schema version. Schema version tracks the format of the index file structure itself. Daemon version tracks which application release created the index. This separation enables independent versioning of data format and application logic.

**Compatibility Checking:**
Storing daemon version in index files enables future compatibility checking. If index format requirements change, newer daemons can examine the daemon version of existing indexes to determine whether migration is needed. Operators can identify which daemon version created problematic indexes for debugging.

**Persistence:**
Daemon version information persists in index files across daemon restarts and system reboots. This persistence creates an audit trail of index creation, supporting forensic analysis and version migration planning.

### MCP Server

The MCP Server subsystem advertises version information in Model Context Protocol initialize responses, enabling MCP clients to identify the server version.

**Initialize Response:**
When an MCP client connects, the server responds to the `initialize` request with server metadata (`internal/mcp/server.go:225-228`). This metadata includes:
- `ServerInfo.Name`: "agentic-memorizer"
- `ServerInfo.Version`: Short version from `version.GetShortVersion()` (e.g., "v0.12.1")

This enables MCP clients (like Claude Code, Gemini CLI, or Codex CLI) to identify which version of agentic-memorizer they're communicating with, supporting compatibility checks and debugging.

### CLI Commands

CLI commands consume version information for status display and operational reporting, providing users with visibility into system version state.

**Status Command:**
The `daemon status` command reads the precomputed index file and displays its metadata, including the daemon version that created it. This display shows users which daemon version is active and enables verification that the expected version is deployed. The status output includes both schema version and daemon version for complete context.

**Version Command:**
The `version` command (`cmd/version/version.go`) displays detailed version information for the agentic-memorizer binary. The command calls `PrintVersion()` which outputs a multi-line format showing Version, Commit, and Built fields separately using `GetShortVersion()`, `GetGitCommit()`, and `GetBuildDate()` respectively. This enables users to query version without starting the daemon or examining index files. Example output:
```
Version: v0.12.1
Commit:  a942b5c
Built:   2025-11-22T14:30:00Z
```

**Usage:**
```bash
memorizer version
agentic-memorizer --version  # Alternative using root command flag
```

Both forms display the same multi-line format. The `--version` flag is implemented through a custom version template on the root command that calls the same version functions.

This command provides clean, parseable output suitable for scripts, CI/CD pipelines, and support ticket reporting.

## Glossary

**ldflags**: Linker flags passed to the Go build system that modify binary construction. The `-X` flag specifically sets string variable values at link time, enabling build-time metadata injection without code changes.

**Build-Time Injection**: The practice of setting variable values during compilation rather than in source code. This enables version information to be derived from git state and build environment without hardcoding in files.

**Build Provenance**: The ability to trace a compiled binary back to its exact source code state. Git commit hashes provide provenance by identifying the specific code version that produced a binary.

**Semantic Versioning (SemVer)**: Version numbering scheme following MAJOR.MINOR.PATCH format (e.g., v0.12.1). Major version increments indicate breaking changes, minor increments add features, patch increments fix bugs.

**Schema Version**: Version number tracking the format of data structures like index files. Schema versions change when data format evolves, independent of application version changes.

**Application Version**: Version number tracking software release state. Application versions change with each release following semantic versioning conventions.

**Daemon Version**: The specific application version that created an index file. Stored in `ComputedIndex.DaemonVersion` field, sourced from the Version subsystem.

**VERSION File**: Plain text file at `internal/version/VERSION` containing the canonical semantic version number (e.g., "0.12.1"). Embedded into binaries via `//go:embed` directive and read by the Makefile. Updated by release automation scripts during version bumps.

**Git Tag**: A named reference to a specific git commit, typically used for releases (e.g., v0.12.1). Tags provide stable version identifiers that don't change as development continues.

**Git Commit SHA**: A hexadecimal hash uniquely identifying a specific code state. Can be full 40-character hash (from ldflags) or short 7-character hash (from BuildInfo). Commit hashes enable precise source code identification for any binary.

**Dirty Flag**: A `-dirty` suffix appended to commit hashes when the workspace has uncommitted changes. Indicates the binary was built from a modified codebase that doesn't exactly match the commit hash. Applied by both `git describe` and BuildInfo extraction.

**ISO 8601**: International standard for date and time representation. The Version subsystem uses UTC timestamps in format YYYY-MM-DDTHH:MM:SSZ (e.g., 2025-11-01T17:39:22Z).

**Development Build**: Binary compiled with simple `go build` command. Uses fallback mechanisms: embedded VERSION file for version, `runtime/debug.ReadBuildInfo()` for commit and date if available, or defaults to "unknown". Suitable for local testing and development.

**Makefile Build**: Binary compiled using `make build` or `make install`. Uses ldflags to explicitly inject version metadata from git commands, providing full control over version strings. Suitable for deployment and production use.

**go:embed Directive**: Go compiler directive (`//go:embed VERSION`) that embeds file contents directly into the compiled binary at build time. Enables the VERSION file content to be accessed as a string variable without file I/O.

**runtime/debug.ReadBuildInfo()**: Go standard library function that extracts build metadata embedded by the Go build system. Provides access to VCS information (commit hash, modification status, commit time) when building from a git repository with Go 1.18+.

**VCS Metadata**: Version Control System information automatically embedded by Go's build system. Includes revision (commit hash), modified status (dirty flag), and commit time. Available through `runtime/debug.ReadBuildInfo()` when building from git repositories.

**Version String**: Formatted combination of version metadata, typically in format "version (commit: hash, built: date)". Version strings appear in logs, index files, and status output.

**ComputedIndex**: Data structure representing a precomputed index file. Contains index data plus metadata including schema version, daemon version, generation timestamp, and statistics.

**Operational Visibility**: The ability to observe system state and behavior through logs, metrics, and status information. Version information provides critical operational visibility into deployed software state.

**Compatibility Tracking**: Recording version information to enable future compatibility verification. Stored versions in index files support migration, debugging, and upgrade planning.

**Two-Level Versioning**: Strategy of tracking both schema versions (data format) and application versions (software releases) independently. Enables flexible evolution of format and application.
