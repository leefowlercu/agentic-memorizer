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
   - [Version Functions](#version-functions)
   - [Build System Integration](#build-system-integration)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Index Manager](#index-manager)
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
Development builds omit ldflags, resulting in binaries with default values ("dev", "unknown", "unknown"). Production builds include ldflags that inject real values from git tags and commands. This distinction enables developers to work without special build requirements while ensuring production binaries have accurate metadata.

### Default Values for Development

The Version subsystem prioritizes developer experience by providing meaningful defaults that enable development builds without requiring build-time injection setup.

**Default Values:**
- Version: "dev" - Clearly indicates a development build
- GitCommit: "unknown" - Honest indication that commit info wasn't injected
- BuildDate: "unknown" - Honest indication that build date wasn't injected

**Purpose:**
These defaults enable the standard `go build` command to produce working binaries without special flags. Developers can compile, test, and run the application normally during development. The "dev" and "unknown" values make it immediately obvious that a binary is not a production release, preventing confusion about version identity.

**Safety:**
Default values never masquerade as production versions. A binary with "dev" or "unknown" cannot be mistaken for a release build. This safety prevents development binaries from being deployed to production accidentally, as status checks and logs will clearly show non-production version information.

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
Stores the application semantic version following semver conventions (e.g., "v0.6.0"). The default value is "dev" for development builds. Production builds inject the version from git tags, typically using `git describe --tags --always` or explicit tag values. This variable identifies release versions and appears in user-facing output.

**GitCommit Variable:**
Stores the full git commit SHA that identifies the exact source code state. The default value is "unknown" for development builds. Production builds inject the commit hash from `git rev-parse HEAD`, providing a 40-character hexadecimal string. This variable enables tracing binaries back to their precise source code, supporting debugging and security audits.

**BuildDate Variable:**
Stores the ISO 8601 UTC timestamp when the binary was compiled. The default value is "unknown" for development builds. Production builds inject the current timestamp from `date -u +%Y-%m-%dT%H:%M:%SZ`, producing timestamps like "2025-11-01T17:39:22Z". This variable helps track binary age and deployment timing.

**Injection Mechanism:**
All three variables are string types, matching the `-X` flag's requirements. The variables are package-level (not const) to allow linker modification. Full import paths are required for injection: `github.com/leefowlercu/agentic-memorizer/internal/version.Variable`.

### Version Functions

The Version subsystem exposes two public functions that format version information for different display contexts.

**GetVersion Function:**
Returns a comprehensive version string combining all three metadata components in the format: `"<version> (commit: <hash>, built: <date>)"`. For example: `"v0.6.0 (commit: a942b5c1234..., built: 2025-11-01T17:39:22Z)"`. This function provides complete version information suitable for logging, debugging output, and detailed status displays.

**Use Cases:**
- Daemon startup logging for operational visibility
- Index file metadata for compatibility tracking
- Error reports that need full build context
- Support requests requiring exact binary identification

**GetShortVersion Function:**
Returns just the version number without commit or build metadata, in the format: `"<version>"`. For example: `"v0.6.0"`. This function provides simple version display suitable for brief status output, version checks, and user-facing displays where full metadata would be verbose.

**Use Cases:**
- Brief version displays in CLI output
- API responses that include version information
- Log messages where full metadata is excessive
- User-facing version indicators

**Function Design:**
Both functions are simple accessors that format package variables. They contain no complex logic, ensuring reliability and minimal performance overhead. The functions can be called from any goroutine safely since they only read package variables.

### Build System Integration

The Version subsystem integrates with the build system through linker flags that inject metadata during compilation. The project's Makefile provides both development and release build targets.

**Makefile Integration:**
The Makefile defines version variables and ldflags automatically:
```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/leefowlercu/agentic-memorizer/internal/version.Version=$(VERSION) \
           -X github.com/leefowlercu/agentic-memorizer/internal/version.GitCommit=$(GIT_COMMIT) \
           -X github.com/leefowlercu/agentic-memorizer/internal/version.BuildDate=$(BUILD_DATE)
```

**Build Targets:**
- `make build` - Development build with default values (version: "dev")
- `make build-release` - Production build with injected version information
- `make install` - Install development build
- `make install-release` - Install release build with version info

**Injection Commands:**
The ldflags contain `-X` flags for each variable:
- Version: `-X github.com/leefowlercu/agentic-memorizer/internal/version.Version=$(git describe --tags --always --dirty)`
- GitCommit: `-X github.com/leefowlercu/agentic-memorizer/internal/version.GitCommit=$(git rev-parse HEAD)`
- BuildDate: `-X github.com/leefowlercu/agentic-memorizer/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)`

**Git Integration:**
Version information derives from git commands executed during build:
- `git describe --tags --always --dirty` produces version string with tag, commit info, and dirty flag
- `git rev-parse HEAD` produces full commit SHA
- `date -u +%Y-%m-%dT%H:%M:%SZ` produces UTC timestamp in ISO 8601 format

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
- `DaemonVersion` field: Application version from Version subsystem (e.g., "v0.6.0 (commit: a942b5c, built: 2025-11-01T17:39:22Z)")

**Separation from Schema Version:**
The daemon version is distinct from the index schema version. Schema version tracks the format of the index file structure itself. Daemon version tracks which application release created the index. This separation enables independent versioning of data format and application logic.

**Compatibility Checking:**
Storing daemon version in index files enables future compatibility checking. If index format requirements change, newer daemons can examine the daemon version of existing indexes to determine whether migration is needed. Operators can identify which daemon version created problematic indexes for debugging.

**Persistence:**
Daemon version information persists in index files across daemon restarts and system reboots. This persistence creates an audit trail of index creation, supporting forensic analysis and version migration planning.

### CLI Commands

CLI commands consume version information for status display and operational reporting, providing users with visibility into system version state.

**Status Command:**
The `daemon status` command reads the precomputed index file and displays its metadata, including the daemon version that created it. This display shows users which daemon version is active and enables verification that the expected version is deployed. The status output includes both schema version and daemon version for complete context.

**Version Command (Future):**
While not currently implemented, a dedicated version command could use `version.GetVersion()` or `version.GetShortVersion()` to display the binary's version information. This would enable users to query version without starting the daemon or examining index files. The command could be implemented as either a subcommand (`agentic-memorizer version`) or a root flag (`agentic-memorizer --version`).

**Help and Error Messages:**
Version information could be included in help text, error messages, or debug output to provide context. While not currently implemented, this integration would help users report issues with accurate version information and enable better support workflows.

## Glossary

**ldflags**: Linker flags passed to the Go build system that modify binary construction. The `-X` flag specifically sets string variable values at link time, enabling build-time metadata injection without code changes.

**Build-Time Injection**: The practice of setting variable values during compilation rather than in source code. This enables version information to be derived from git state and build environment without hardcoding in files.

**Build Provenance**: The ability to trace a compiled binary back to its exact source code state. Git commit hashes provide provenance by identifying the specific code version that produced a binary.

**Semantic Versioning (SemVer)**: Version numbering scheme following MAJOR.MINOR.PATCH format (e.g., v0.6.0). Major version increments indicate breaking changes, minor increments add features, patch increments fix bugs.

**Schema Version**: Version number tracking the format of data structures like index files. Schema versions change when data format evolves, independent of application version changes.

**Application Version**: Version number tracking software release state. Application versions change with each release following semantic versioning conventions.

**Daemon Version**: The specific application version that created an index file. Stored in `ComputedIndex.DaemonVersion` field, sourced from the Version subsystem.

**Git Tag**: A named reference to a specific git commit, typically used for releases (e.g., v0.6.0). Tags provide stable version identifiers that don't change as development continues.

**Git Commit SHA**: A 40-character hexadecimal hash uniquely identifying a specific code state. Commit hashes enable precise source code identification for any binary.

**ISO 8601**: International standard for date and time representation. The Version subsystem uses UTC timestamps in format YYYY-MM-DDTHH:MM:SSZ (e.g., 2025-11-01T17:39:22Z).

**Development Build**: Binary compiled without version injection, resulting in default values ("dev", "unknown"). Development builds are for local testing and should not be deployed to production.

**Production Build**: Binary compiled with version injection, resulting in accurate version, commit, and build date. Production builds are intended for deployment and operation.

**Version String**: Formatted combination of version metadata, typically in format "version (commit: hash, built: date)". Version strings appear in logs, index files, and status output.

**ComputedIndex**: Data structure representing a precomputed index file. Contains index data plus metadata including schema version, daemon version, generation timestamp, and statistics.

**Operational Visibility**: The ability to observe system state and behavior through logs, metrics, and status information. Version information provides critical operational visibility into deployed software state.

**Compatibility Tracking**: Recording version information to enable future compatibility verification. Stored versions in index files support migration, debugging, and upgrade planning.

**Two-Level Versioning**: Strategy of tracking both schema versions (data format) and application versions (software releases) independently. Enables flexible evolution of format and application.
