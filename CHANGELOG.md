# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.14.0] - 2026-01-01

### Added
- Feat(cache): add directory sharding to semantic analysis cache
- Feat(cli): improve initialize command flags for unattended mode
- Feat(container): add podman support alongside docker for falkordb management
- Feat(embeddings): add multi-provider support for openai, voyage, and gemini
- Feat(facts): add facts management system for persistent user context
- Feat(format): add consistent usage guide to file index output formats
- Feat(mcp): add support for protocol version 2025-11-25
- Feat(semantic): add multi-provider support for semantic analysis
- Feat: add file remember/forget commands and graph sync functionality
- Feat: add logging for semantic analysis status
- Feat: add logging subsystem with structured logging and factory pattern
- Feat: write API keys to config file during initialization

### Fixed
- Fix(cli): align connection and database values in graph status output
- Fix(e2e): update output_formats tests to use read files subcommand
- Fix(e2e): update tests for unified api endpoint and correct config format
- Fix(integrations): prevent duplicate hooks during initialization
- Fix(test): update reload integration test for current config api
- Fix(test): update xml formatter test to not expect xml declaration
- Fix: remove broken env var option when not detected in TUI
- Fix: sse stream timeout causing disconnections every 60 seconds

### Changed
- Refactor(cli): add example fields and helpers.go files to cobra commands
- Refactor(cli): remove daemon systemctl and launchctl subcommands
- Refactor(cli): remove docker-gating from graph status command
- Refactor(cli): remove graph start and stop commands
- Refactor(cli): standardize cobra flag storage to variable-based pattern
- Refactor(config): move embeddings.provider to minimal config tier
- Refactor(format): remove markdown formatter from output options
- Refactor(integrations): consolidate adapter code and improve organization
- Refactor(mcp): require daemon for search_files and remove internal/search package
- Refactor: consolidate shared code and optimize metadata handler lookup
- Refactor: remove integrations config section
- Refactor: rename graphindex to fileindex for terminology consistency
- Refactor: standardize all log messages to lowercase

### Documentation
- Docs: add logging standards section to claude.md
- Docs: reconcile project documentation ahead of v0.14.0 release
- Docs: restructure and update project documentation
- Docs: update latest version ref in readme
- Docs: update quick start section in readme
- Docs: update readme.md
- Docs: update releases badge to include prereleases

### Build
- Chore: update makefile test-integration target

### Tests
- Style: go fmt updates
- Style: gofmt updates/fixes
- Style: replace emoji with ansi color codes in makefile
- Style: update wording in api key step in initialization wizard
- Test: add required client headers to sse tests

## [0.13.0] - 2025-12-10

### Added
- Feat(cache): add three-tier versioning system for cache staleness detection
- Feat(cache): rename --old-versions to --stale and add context-aware notes
- Feat(cli): add tui wizard for initialization and consolidate docker helpers
- Feat(cli): update commands for graph-native architecture
- Feat(config): add skipextensions and database configuration fields
- Feat(config): update config schema generator and show-schema command
- Feat(config): update configuration subsystem, add config subcommands
- Feat(daemon): integrate skipextensions filtering and database config
- Feat(embeddings): add openai embeddings provider with caching
- Feat(format): add GraphContent builder type for Phase 5
- Feat(format): add GraphContent support to JSON and YAML formatters
- Feat(format): add GraphContent support to Markdown formatter
- Feat(format): add GraphContent support to XML formatter
- Feat(format): add GraphContent support to text formatter
- Feat(format): add core builder types and utilities
- Feat(format): add text line support and improve text formatter
- Feat(format): implement all formatters (text, json, yaml, markdown, xml)
- Feat(format): migrate MCP server to use format package
- Feat(format): migrate cache status to use format package
- Feat(format): migrate claude hook adapter to use format package
- Feat(format): migrate config show-schema to use format package
- Feat(format): migrate config validate to use format package
- Feat(format): migrate daemon status to use format package
- Feat(format): migrate generic adapter to use format package
- Feat(format): migrate graph status to use format package
- Feat(format): migrate integrations detect to use format package
- Feat(format): migrate integrations health to use format package
- Feat(format): migrate integrations list to use format package
- Feat(format): migrate read command to use format package
- Feat(format): migrate version command to use format package
- Feat(graph): add configuration section to status command
- Feat(graph): add falkordb knowledge graph subsystem
- Feat(graph): add getfilebyfilename method for flexible file lookup
- Feat(integrations): add gemini cli sessionstart hook integration
- Feat(integrations): update output formatters for graph-native types
- Feat(mcp): use daemon api for graph-powered queries
- Feat(search): update semantic search for graph-native types
- Feat(semantic): add entity and reference extraction to analyzer
- Feat(tui): add automatic startup configuration to initialize wizard
- Feat(version): add divider to version command output
- Feat(walker): add skipextensions parameter for file filtering
- Feat(watcher): add skipextensions parameter for file filtering
- Feat: add codex cli mcp integration adapter
- Feat: add gemini cli mcp integration
- Feat: reduce configuration surface area, add minimal configuration key set

### Fixed
- Fix!: update hardcoded paths in TUI initialize steps
- Fix(cmd): replace precomputed index terminology with knowledge graph in command descriptions
- Fix(config): add format flag to validate command for full config output
- Fix(config): add missing viper default for claude.timeout
- Fix(config): add missing viper defaults for embeddings provider, model, and dimensions
- Fix(config): remove outdated index file references
- Fix(e2e): improve daemon cleanup timeout handling
- Fix(e2e): remove obsolete version attribute from docker-compose.yml
- Fix(e2e): update Dockerfile and harness for new binary name
- Fix(e2e): update cache clear tests to use --stale flag
- Fix(graph): configure falkordb to use bind mount for data persistence
- Fix(graph): disable maintenance notifications for falkordb compatibility
- Fix(graph): suppress initialization logs in status command
- Fix(init): suppress daemon start instructions after service manager setup
- Fix(integrations): update xml output processor for empty index handling
- Fix(tests): add formatters import to config tests for init registration
- Fix(tests): add formatters import to integration and MCP tests
- Fix(tui): add intelligent language for integration configuration steps
- Fix(tui): correct wizard confirmation tracking and daemon startup behavior
- Fix(tui): remove vertical centering from initialization wizard
- Fix: various fixes

### Changed
- Refactor(api): remove legacy /notifications/stream sse endpoint
- Refactor(cache): migrate clear command to use format package
- Refactor(cli)!: update all commands for new binary name
- Refactor(config): implement reflection-based schema generation with auto-derived tier classifications
- Refactor(config): migrate reload command to use format package
- Refactor(core)!: rename binary to memorizer and app dir to .memorizer
- Refactor(daemon)!: update service manager for new binary name
- Refactor(daemon): migrate stop/restart commands to use format package
- Refactor(daemon): rename rebuild flag from --clear-old-cache to --clear-stale for consistency
- Refactor(daemon): restructure with worker/api packages and graph integration
- Refactor(daemon): simplify status command and use switch statements in helpers
- Refactor(format): remove deprecated output package
- Refactor(integrations)!: update adapters with old binary detection (v2.0.0)
- Refactor(integrations): remove generic integrations (continue, cline, aider, cursor, custom)
- Refactor(integrations): remove redundant validate command
- Refactor: use idiomatic go patterns for validation and conditionals

### Documentation
- Docs!: add migration guide for v0.12 to v0.13 rename
- Docs!: update CLAUDE.md for new binary name
- Docs!: update README.md for new binary name
- Docs!: update subsystem docs and config files for new binary name
- Docs(comments): improve comment accuracy and add missing rationale
- Docs(config): update config manager documentation to reflect reflection-based schema generation
- Docs(e2e): add end-to-end testing subsystem documentation
- Docs(format): add comprehensive CLI output formatting guidelines to CLAUDE.md
- Docs(format): add format subsytem documentation
- Docs(readme): update version, badges, and feature documentation
- Docs: add codex cli integration documentation
- Docs: add falkordb persistence and e2e testing documentation
- Docs: clarify format subsystem integration adapter formatter usage and add last updated date
- Docs: comprehensive readme accuracy review and corrections
- Docs: remove assertion count and line count from e2e tests documentation
- Docs: remove references to --hardcoded-only flag from config show-schema
- Docs: rename index-manager to index-management and update subsystem documentation
- Docs: update cache clear flag from --old-versions to --stale
- Docs: update claude.md to add instructions for integration adapter version bumping during adapter updates
- Docs: update documentation for graph-native architecture
- Docs: update environment variables section in readme
- Docs: update falkordb-graph subsystem documentation to reflect current implementation
- Docs: update integration registry subsystem documentation with codex cli adapter addition
- Docs: update integration registry subsytem documentation with gemini cli adapter addition

### Build
- Build(makefile): add e2e test and ci targets
- Build: clean dist directory in make clean target
- Chore(gitignore): update gitignore
- Chore: update .gitignore with new binary name

### Tests
- Style(config): complete database config and align formatting
- Style(graph): align struct field formatting across graph subsystem
- Style: apply gofmt formatting
- Style: change root command long text description
- Style: gofmt updates/fixes
- Test!: fix test expectations for new binary name
- Test!: update test files for new binary name (partial)
- Test(e2e): add automatic volume cleanup to all test targets
- Test(e2e): add comprehensive end-to-end test suite
- Test(e2e): update graceful degradation test to expect daemon failure without falkordb
- Test(integration): add missing timeout to reload test config
- Test(integration): fix daemon reload test config structure and api imports
- Test(integration): fix mcp server test signatures and response parsing
- Test(integrations): update output and registry tests for new patterns
- Test: fix various test failures, update make targets

## [0.12.1] - 2025-11-25

### Fixed
- Fix: bugfix for daemon http health endpoint metrics accumulation

### Build
- Build: update prepare-release script to properly handle unreleased section

### Tests
- Style: go fmt
- Test: update tests, remove brittle version test, update daemon integration tests

## [0.12.0] - 2025-11-24

### Added
- Feat: add http api configuration to initialize command
- Feat: update mcp server functionality with prompts and resource change subscription, update daemon to run sse server that informs mcp servers of index updates

### Fixed
- Fix: update styling of initialize command text

### Changed
- Refactor: consolidate health and sse endpoints into unified http server

### Documentation
- Docs: comprehensive update to subsystem documentation
- Docs: update inaccuracy in readme
- Docs: update project structure diagram in readme
- Docs: update readme.md and claude.md for accuracy

### Build
- Build: automate version reference updates in release workflow

## [0.11.0] - 2025-11-22

### Added
- Interactive release tagline support in Goreleaser workflow - Makefile prompts for custom release taglines that appear in GitHub release titles
- Hybrid version embedding with VERSION file and buildinfo fallback - go install users get version from embedded VERSION file, Make builds use git describe

### Changed
- Simplified daemon status command implementation - Cleaner code structure and improved maintainability

### Documentation
- Added GitHub badges to README (Go Version, MIT License, Go Report Card, GitHub Release)
- Added MIT License to project
- Updated README for accuracy

### Build
- Implemented Goreleaser for multi-platform releases - Automated builds for Linux/macOS (amd64/arm64) with checksums and draft GitHub releases
- Moved systemd dependency to direct requirements in go.mod

### Tests
- Aligned struct field declarations in test files for consistency
- Fixed reload integration test for config schema changes

## [0.10.0] - 2025-11-21

### Added
- **Service manager integration** - Comprehensive support for systemd and launchd service managers
  - Service file generation commands: `daemon systemctl` (systemd) and `daemon launchctl` (launchd)
  - Automatic service manager detection on Linux and macOS platforms
  - Generated service files include proper paths, environment variables, and security configurations
  - systemd integration uses Type=notify for precise readiness signaling via SdNotify
  - launchd integration includes KeepAlive, ThrottleInterval, and proper environment setup
  - Helper utilities for platform detection and service manager availability checks
- **Enhanced daemon commands** - Context-aware help and suggestions based on detected service manager
  - `daemon start` detects service-managed vs foreground mode and provides appropriate startup messages
  - `daemon start` provides platform-specific setup hints for non-service-managed starts
  - `daemon start` wraps "already running" errors with troubleshooting steps and service manager commands
  - `daemon status` shows service-managed vs foreground status and adds "Service Management" section
  - `daemon status` suggests checking service manager when daemon not running
  - `daemon stop` provides platform-specific service manager commands when errors occur
  - Improved user experience with actionable next steps for all daemon lifecycle commands
- **Comprehensive documentation** - Service manager integration guide (430 lines added to README.md and CLAUDE.md)
  - Step-by-step setup instructions for systemd (Linux) with user-level and system-wide options
  - Step-by-step setup instructions for launchd (macOS) with complete plist generation
  - Service file generation examples and customization guidance
  - Platform-specific troubleshooting and common issues resolution
  - Upgrade procedures for service-managed and manual daemon installations
  - Updated CLAUDE.md with service manager workflow and development notes

### Changed
- **BREAKING**: Removed `daemon.enabled` configuration field
  - This field was only used to control whether daemon started during initialization
  - Daemon behavior is now controlled entirely by whether it's running (PID file check)
  - Users with `daemon.enabled` in config files should remove it (no functional impact)
  - `daemon status` command enhanced to show detailed status without relying on config field
  - Simplified daemon management: start/stop/restart commands work regardless of config
- **Initialize command output** - Simplified and streamlined formatting (149 lines removed)
  - Clearer, more concise progress indicators and section headers
  - Improved readability with better visual hierarchy
  - Added blank lines for better section separation
  - Enhanced integration setup output with clearer success messages
  - Improved "Next steps" guidance directing users to service manager setup

### Removed
- **daemon.enabled config field** - No longer needed for daemon behavior control
  - Removed from config types (`internal/config/types.go`)
  - Removed from config validation (`cmd/config/subcommands/validate.go`)
  - Removed from default config constants (`internal/config/constants.go`)
  - Removed from example configuration (`config.yaml.example`)
  - Removed from status command output (`cmd/daemon/subcommands/status.go`)
  - Removed from subsystem documentation (`docs/subsystems/config-manager/README.md`)
  - Status checks now rely on PID file and process state instead of config flag

## [0.9.1] - 2025-11-21

### Added
- **Comprehensive test suite expansion** (1,966 lines of new tests)
  - Cache manager unit tests (`internal/cache/manager_test.go` - 386 lines)
  - Integration types unit tests (`internal/integrations/types_test.go` - 84 lines)
  - Integration utilities unit tests (`internal/integrations/utils_test.go` - 453 lines)
  - MCP server lifecycle unit tests (`internal/mcp/server_lifecycle_test.go` - 324 lines)
  - Version package unit tests (`internal/version/version_test.go` - 312 lines)
  - Walker package unit tests (`internal/walker/walker_test.go` - 407 lines)
  - Improved test coverage across core subsystems

### Changed
- **Enhanced documentation** - CLAUDE.md and README.md updated to fully reflect v0.9.0 features
  - Added MCP server architecture and tools documentation
  - Added semantic search capabilities description
  - Added configuration hot-reload instructions
  - Updated command examples for both hook and MCP integrations
  - Enhanced architecture overview with all subsystems
  - Added troubleshooting section for config reload
  - Clarified integration types (claude-code-hook vs claude-code-mcp)
  - Updated framework comparison table with MCP column
  - Added table of contents to both documentation files

### Fixed
- **JSON output HTML escaping** - Prevented HTML entity encoding in JSON formatted output
  - JSON output now preserves angle brackets, ampersands, and quotes as-is
  - Ensures clean JSON parsing for programmatic consumption
  - Created `marshalIndentNoEscape()` helper using `json.NewEncoder()` with `SetEscapeHTML(false)`
  - Added comprehensive tests verifying no escape sequences in output
- **Initialize command output formatting** - Improved readability and structure of initialization output
  - Clearer section headers and progress indicators
  - Better visual hierarchy in command output

### Removed
- **Dead configuration key** - Removed unused `analysis.parallel` configuration field
  - This field was defined, validated, and documented but never actually used by any application code
  - The daemon uses `daemon.workers` to control parallel processing via worker pool
  - Users should use `daemon.workers` (default: 3) to configure parallel API calls
  - Existing configs with `analysis.parallel` will log a deprecation warning but continue working
- **Dead configuration and flag** - Removed unused `output.verbose` configuration field and `--verbose` flag
  - Infrastructure existed (config field, flag, Options struct) but no implementation ever used the value
  - Setting `--verbose` or `output.verbose: true` had no effect on application behavior
  - Removed incomplete/abandoned feature to reduce code complexity

### Chore
- **Dependency organization** - Moved direct dependencies from indirect section in go.mod
  - Improved dependency management clarity

## [0.9.0] - 2025-11-20

### Added
- **MCP Server Implementation** (`cmd/mcp/`, `internal/mcp/`) - Complete Model Context Protocol server for Claude Code integration
  - New `mcp` command with stdio transport for MCP protocol communication
  - Three MCP tools exposed: `search_files`, `get_file_metadata`, `list_recent_files`
  - Semantic search capabilities integrated into MCP tool responses
  - Protocol-compliant request/response handling with JSON-RPC 2.0
  - Server lifecycle management (initialize, initialized, shutdown)
  - Comprehensive logging with configurable log levels
  - Integration with existing index and metadata subsystems
- **Semantic Search Subsystem** (`internal/search/`) - Advanced file discovery and ranking
  - Fuzzy filename matching with configurable threshold (default 0.3)
  - Tag-based search with partial matching support
  - Topic-based search across indexed file analysis
  - Summary text search for content-based discovery
  - Relevance scoring algorithm combining multiple signals
  - File category filtering (documents, images, code, etc.)
  - Results limited and ranked by relevance score
- **Claude Code MCP Integration** (`internal/integrations/adapters/claude/mcp_adapter.go`)
  - Dedicated MCP server adapter separate from SessionStart hooks
  - Automatic `~/.claude.json` configuration management
  - Environment variable setup for memory root path
  - Binary path detection and configuration
  - Health check verification for MCP server connectivity
- **MCP Configuration Section** - New `mcp` section in config.yaml
  - `mcp.log_file` - Dedicated MCP server log file path
  - `mcp.log_level` - Independent log level control (debug, info, warn, error)
  - Separate from daemon logging for clear operational visibility

### Fixed
- **MCP integration setup** - Fixed bug where `MEMORIZER_MEMORY_ROOT` environment variable was blank in `~/.claude.json` during `initialize` command
  - Config is now initialized immediately after writing so integration setup can read config values
  - MCP server now correctly receives memory root path from freshly initialized configuration
  - Applies to both `initialize --setup-integrations` and standalone `integrations setup` commands
- **Stdio transport test** - Fixed newline handling mismatch in MCP transport layer
  - Tests now correctly expect trimmed output matching implementation behavior
  - Read() method intentionally strips whitespace for clean JSON parsing

### Changed
- **Claude Code integration split** - Separated into two distinct integration types
  - `claude-code-hook` - SessionStart hooks integration (existing functionality)
  - `claude-code-mcp` - MCP server integration (new functionality)
  - Users can choose one or both integration methods
  - Each integration independently configurable and removable
- **Integration tracking** - Config file now tracks which integrations are configured
  - `integrations.enabled` list in config.yaml automatically updated by `initialize`, `setup`, and `remove` commands
  - Provides visibility into configured integrations without checking framework-specific files
  - Integration-specific settings remain in framework files (~/.claude.json, ~/.claude/settings.json, etc.)

### Removed
- **integrations.configs section** - Removed unused `configs` map from IntegrationsConfig structure
  - The `configs` field was never populated and all integration-specific configuration lives in framework files
  - Streamlines config file structure and eliminates vestigial code
  - Only `integrations.enabled` list remains for tracking configured integrations
- **Dead code cleanup** - Removed unused IntegrationConfig type and validation function from config package
  - Deleted duplicate `IntegrationConfig` type definition from `internal/config/types.go` (integrations package version remains)
  - Deleted unused `ValidateIntegrationConfig` function from `internal/config/validate.go`
  - Eliminates confusion about which type definition to use and reduces dead code

## [0.8.2] - 2025-11-05

### Fixed
- **Makefile installation targets** for macOS code signing compatibility
  - Added automatic daemon stop before installation to prevent "Killed: 9" errors
  - Changed from `cp` to `mv` with temporary file pattern to force new inode creation
  - Prevents macOS code signing cache invalidation issues when replacing running binaries
  - Added 1-second sleep after daemon stop for clean shutdown
  - Added user prompt to restart daemon after installation completes
  - Applied to both `install` and `install-release` targets

### Changed
- **Interactive initialization command** API key configuration workflow
  - Added interactive prompt for Claude API key during `initialize` command
  - Detects existing ANTHROPIC_API_KEY environment variable and offers to use it
  - Allows users to enter API key directly (stored in config) or reference environment variable
  - Supports secure password-style input (hidden characters) when entering API key
  - Enhanced "Next steps" output to conditionally show API key setup based on configuration
  - Improved user experience with clear options for API key management

## [0.8.1] - 2025-11-03

### Fixed
- **Initialization command daemon startup** - Fixed bug where selecting "yes" to start daemon during `initialize` didn't actually start or enable it
  - Daemon is now properly enabled in configuration (`daemon.enabled: true`) before startup
  - Improved startup verification with retry logic to confirm daemon started successfully
  - Better error visibility - startup errors are now shown to users instead of silently suppressed
  - Success message displays daemon PID and verification command

## [0.8.0] - 2025-11-03

### Added
- **Version command and flag** for displaying version information
  - `version` subcommand (`agentic-memorizer version`) for standalone version display
  - `--version` persistent flag available at root and all command levels
  - `-v` short flag alias for convenient access
  - Multi-line detailed format showing version, commit hash, and build timestamp
  - Enhanced version package with `GetGitCommit()` and `GetBuildDate()` getter functions
  - Custom version template for Cobra integration with consistent formatting
  - Updated Makefile `install-release` target to verify installation with version command

### Fixed
- **Integration test configuration** missing required fields
  - Added `Output` section with `Format: "xml"` to test environment configuration
  - Added `Analysis.Parallel: 2` field to match daemon workers in test setup
  - Prevents validation errors during integration test execution
- **Mutex copy bug** in health metrics system
  - Created separate `HealthSnapshot` struct without mutex for safe copying and serialization
  - Changed `GetSnapshot()` return type from `HealthMetrics` to `HealthSnapshot`
  - Eliminates `go vet` warning about copying `sync.RWMutex` which violates concurrency semantics
- **Context leak** in daemon initialization
  - Moved `context.WithCancel()` creation to after all fallible operations
  - Ensures `cancel()` function is only created when initialization will succeed
  - Prevents resource leaks on early returns during daemon startup
- **Concurrent map writes** in viper configuration system
  - Added package-level mutex to protect `InitConfig()` from concurrent access
  - Added `viper.Reset()` to clear cached config state during reload
  - Prevents fatal concurrent map writes when multiple goroutines reload configuration
- **Type assertion panic** in integration tests
  - Changed from direct `atomic.Value.Store()` to proper `SetSemanticAnalyzer()` API method
  - Ensures type safety with atomic.Value requiring correct concrete types
- **Immutable field validation** in reload integration test
  - Fixed test to create config copy before writing to file
  - Prevents unintended modification of daemon's in-memory config reference
  - Now correctly detects and rejects immutable field changes during reload

## [0.7.0] - 2025-11-03

### Added
- **Configuration hot-reload mechanism** for live updates without daemon restart
  - `config reload` command for manual configuration reload with validation
  - SIGHUP signal handler for graceful reload (compatible with systemd/launchd)
  - Atomic reload with multi-stage validation and automatic rollback on errors
  - Dynamic worker pool reconfiguration (rate limiter, worker count changes)
  - File watcher reconfiguration with automatic directory re-monitoring
  - Thread-safe reload implementation using RWMutex and atomic.Value
  - Best-effort component updates with detailed logging
  - Hot-reloadable settings: workers, rate limit, log level, health port, debounce interval, rebuild interval, Claude API settings
  - Restart-required settings (validated against changes): memory root, cache directory, log file path
- **MEMORIZER_APP_DIR environment variable** for custom application directory
  - Override default `~/.agentic-memorizer/` location for config, index, PID, logs
  - Path safety validation preventing directory traversal attacks
  - Home directory (`~`) expansion support for portable configurations
  - Use cases: testing isolation, multi-instance deployments, Docker containers, CI/CD pipelines
  - Documented in README with usage examples and init system integration
- **Comprehensive test suite** (1,951+ lines of new tests)
  - Configuration loading and validation unit tests (`internal/config/config_test.go` - 247 lines)
  - Configuration reload validation tests (`internal/config/reload_test.go` - 291 lines with 13 test scenarios)
  - Daemon lifecycle and thread-safety tests (`internal/daemon/daemon_test.go` - 689 lines with 23 test scenarios)
  - Full reload integration tests (`internal/daemon/reload_integration_test.go` - 603 lines with 11 test scenarios)
  - File watcher debounce update tests (`internal/watcher/watcher_test.go` - 119 additional lines)
  - Build tag separation (`//go:build !integration` for unit tests, `//go:build integration` for integration tests)
  - Test isolation using MEMORIZER_APP_DIR with isolated temp directories per test
  - TestEnv helper providing comprehensive test environment setup
  - All tests pass with race detector (`go test -race ./...`)
- **Subsystem documentation** (11 comprehensive architectural guides, ~3,500 lines)
  - `docs/subsystems/README.md` - Index of all subsystems with architectural overview
  - `docs/subsystems/daemon/README.md` - Daemon orchestration, signal handling, health monitoring, reload mechanism
  - `docs/subsystems/index-manager/README.md` - Index structure, atomic updates, versioning, schema evolution
  - `docs/subsystems/file-watcher/README.md` - fsnotify integration, debouncing, event handling, directory monitoring
  - `docs/subsystems/metadata-extractor/README.md` - Handler pattern, 10 file type extractors, content-specific metadata
  - `docs/subsystems/cache-manager/README.md` - SHA-256 content hashing, LRU eviction, cache hit optimization
  - `docs/subsystems/config-manager/README.md` - Layered configuration, validation system, reload mechanism, MEMORIZER_APP_DIR
  - `docs/subsystems/semantic-analyzer/README.md` - Claude API integration, content routing, vision support
  - `docs/subsystems/integration-registry/README.md` - Adapter pattern, framework detection, output processors
  - `docs/subsystems/version/README.md` - Build-time version injection, Makefile integration, semantic versioning
  - `docs/subsystems/walker/README.md` - Directory traversal, callback pattern, two-tier filtering
- **Enhanced Makefile targets** for release management and testing
  - `build-release` - Build with version information (VERSION, GitCommit, BuildDate via ldflags)
  - `install-release` - Install release build with version info and verification
  - `test-integration` - Run integration tests separately with `-tags=integration`
  - `test-all` - Run both unit and integration test suites
  - `clean-cache` - Remove cache files without cleaning build artifacts
  - Improved help documentation with clear target descriptions
  - Version injection pattern: `-ldflags "-X internal/version.Version=$(VERSION)"`
- **PreRunE input validation pattern** across all CLI commands
  - Distinguishes user input errors (shows usage) from runtime errors (no usage)
  - Named validation functions (`validateXxx`) for consistency and maintainability
  - Sets `cmd.SilenceUsage = true` only after input validation passes
  - Applied to all daemon subcommands (start, stop, restart, status, rebuild, logs)
  - Applied to all integration subcommands (list, detect, setup, remove, validate, health)
  - Applied to all config subcommands (validate, reload)
  - Improved user experience with contextual help only when appropriate
- **Init system integration enhancements**
  - systemd service support with `Environment="MEMORIZER_APP_DIR=/custom/path"` directive
  - launchd plist support with `EnvironmentVariables` key for MEMORIZER_APP_DIR
  - SIGHUP reload support documented for both init systems

### Changed
- **BREAKING**: Renamed `init` command to `initialize` (avoid Go reserved keyword conflict)
  - Command-line usage: `agentic-memorizer initialize` (was `agentic-memorizer init`)
  - Package path: `cmd/initialize/` (was `cmd/init/`)
  - Variable name: `InitializeCmd` (was `InitCmd`)
  - **Migration path**: Update any scripts, documentation, or automation referencing the old `init` command
- **Command structure reorganization** following Cobra subcommands pattern
  - Daemon commands moved from inline to `cmd/daemon/subcommands/` package
  - Integration commands moved from inline to `cmd/integrations/subcommands/` package
  - Config commands separated into `cmd/config/` parent and `cmd/config/subcommands/` package
  - Removed `cmd` prefix aliases in package imports (was `cmddaemon`, now just `daemon`)
  - Each subcommand in its own file with exported command variable
  - Parent commands define structure, subcommands implement functionality
  - Improved code maintainability, discoverability, and organization
- **Error message formatting standardized** with semicolon separators
  - Pattern changed: `fmt.Errorf("context; %w", err)` instead of `fmt.Errorf("context: %w", err)`
  - Rationale: Root command already prefixes all errors with "Error:", semicolon provides cleaner output
  - Example: `Error: failed to load config; file not found` (was `Error: failed to load config: file not found`)
  - Applied consistently across entire codebase (~50+ error wrapping sites)
- **Command help text standardized** with consistent Long description format
  - Pattern: `"\n[introductory sentence]\n\n[detailed explanation paragraph]"`
  - Opening newline for clean visual separation in help output
  - Double newline between introduction and detailed description
  - String concatenation with `+` operator for natural line breaks
  - Applied to root, initialize, daemon, config, and integration commands
  - Professional, consistent appearance across all CLI help output
- **Daemon signal handling enhanced** with comprehensive reload support
  - SIGHUP signal triggers configuration reload in dedicated goroutine
  - Improved graceful shutdown coordination for SIGINT/SIGTERM
  - SIGUSR1 triggers manual rebuild as before
  - Better error propagation during signal-triggered operations
  - Signal handler logs all received signals for debugging
- **Configuration system** respects MEMORIZER_APP_DIR throughout
  - `GetAppDir()` checks environment variable first, then defaults to `~/.agentic-memorizer`
  - Path safety validation (`SafePath()`) applied to custom app directories
  - Home directory expansion (`~`) supported in MEMORIZER_APP_DIR values
  - `InitConfig()` adds custom app directory to viper config search path
  - All path helper functions (`GetIndexPath()`, `GetPIDPath()`) use `GetAppDir()`
- **README documentation significantly expanded**
  - New "Environment Variables" section explaining MEMORIZER_APP_DIR with use cases
  - Updated "Building and Testing" section distinguishing unit vs integration tests
  - Enhanced init system integration examples showing environment variable usage
  - Clarified daemon reload mechanism and hot-reloadable vs restart-required settings
  - Updated development workflow with new Makefile targets
  - Added note about integration test isolation via MEMORIZER_APP_DIR

### Removed
- **Outdated and WIP documentation** (7 files, ~7,000+ lines removed)
  - `docs/wip/agent-framework-decoupling.md` (3,518 lines) - superseded by integration registry subsystem docs
  - `docs/wip/background-index-computation-plan.md` (1,551 lines) - superseded by daemon subsystem docs
  - `docs/wip/init-system-integration.md` (1,719 lines) - superseded by examples and daemon docs
  - `docs/architecture.md` (506 lines) - superseded by comprehensive subsystem documentation
  - `docs/integrations/claude-code.md` (406 lines) - superseded by integration registry docs
  - `docs/integrations/custom.md` (643 lines) - superseded by integration registry docs
  - `docs/integrations/generic.md` (441 lines) - superseded by integration registry docs
  - Replaced with structured, comprehensive subsystem documentation in `docs/subsystems/`

### Fixed
- Configuration reload safety with proper validation and rollback
  - Worker pool correctly reconfigures rate limiter during reload without dropping jobs
  - File watcher successfully reconfigures debounce timing and re-monitors directories
  - No race conditions during concurrent reload attempts (protected by config RWMutex)
  - Validation errors prevent partial configuration application (atomic swap pattern)
- Test isolation prevents interference between integration test runs
  - Each test gets isolated temporary directory via `t.TempDir()`
  - MEMORIZER_APP_DIR ensures no collision with production daemon or other tests
  - Proper cleanup of test resources (temp dirs, goroutines, HTTP servers)
- Init system service examples properly support SIGHUP reload
  - systemd service includes commented Environment directive
  - launchd plist includes commented EnvironmentVariables key
  - Both examples document reload signal handling

## [0.6.0] - 2025-11-01

### Added
- **Framework-agnostic integration system** with pluggable adapter pattern
  - Integration registry with automatic framework detection
  - `Integration` interface for lifecycle management (Detect, Setup, Remove, Validate, etc.)
  - Thread-safe global registry with RWMutex protection
- **Claude Code adapter** (`internal/integrations/adapters/claude/`)
  - Automatic setup of SessionStart hooks in `~/.claude/settings.json`
  - Atomic settings file updates with backup/rollback
  - SessionStart JSON envelope wrapping for proper hook format
  - Support for all 4 matchers (startup, resume, clear, compact)
  - Configuration validation for existing setups
- **Generic adapter** for frameworks without automatic setup
  - Manual setup instructions with framework-specific examples
  - Registered for: Continue.dev, Cline, Aider, Cursor, Custom integrations
  - Plain output without framework-specific wrapping
- **JSON output format** (in addition to XML and Markdown)
  - Clean JSON structure for programmatic consumption
  - Integration-agnostic format
- **Output processor architecture** (`internal/integrations/output/`)
  - Separate XML, Markdown, and JSON processors
  - Clean separation between format rendering and integration wrapping
  - Migrated from legacy formatter with improved structure
- **Integration management commands** (`cmd/integrations/`)
  - `integrations list` - List all available integrations with status
  - `integrations detect` - Detect installed agent frameworks on system
  - `integrations setup <name>` - Configure integration automatically
  - `integrations remove <name>` - Remove integration configuration
  - `integrations validate` - Validate all configured integrations
  - `integrations health` - Comprehensive health checks (detection + validation)
- **Configuration validation system** (`internal/config/validate.go`)
  - Schema validation (required fields, types, structure)
  - Value validation (numeric ranges, enums, path formats)
  - Path safety validation (directory traversal protection)
  - Cross-field dependency validation
  - Structured error reporting with actionable suggestions
  - `Validator` type with error accumulation pattern
- **Configuration management command** (`cmd/config/`)
  - `config validate` - Validate configuration file for errors
  - Comprehensive validation reporting with fix suggestions
- **Integration utilities** (`internal/integrations/utils.go`)
  - Path expansion (`ExpandPath` for ~ home directory)
  - File/directory existence checks
  - Binary path validation with permission verification
  - Output format parsing and validation
- **Comprehensive documentation**
  - Detailed subsystem documentation in `docs/subsystems/`
  - Integration setup guides and usage instructions

### Changed
- **Init command** now uses integration registry
  - Automatic detection of installed frameworks
  - Interactive prompts for integration setup selection
  - `--setup-integrations` flag for automated configuration
  - `--skip-integrations` flag for non-interactive mode
  - Improved user experience with progress feedback
- **Read command** supports `--integration <name>` flag
  - Format output specifically for target integration
  - Applies integration-specific wrapping (e.g., SessionStart JSON for Claude Code)
  - Maintains backwards compatibility with format-only output
- Configuration schema expanded with `integrations` section
  - Per-integration settings (type, enabled, output_format)
  - Integration-specific configuration options
  - Settings path customization
- Improved help text and examples across all commands
- Error messages now include actionable guidance and suggestions
- README updated with multi-framework support and integration management

### Removed
- **BREAKING**: Removed `--wrap-json` flag (replaced by integration-specific formatting via `--integration` flag)
- **BREAKING**: Removed legacy hook management system (`internal/hooks/` - 779 lines)
  - `hooks/manager.go` (178 lines)
  - `hooks/manager_test.go` (584 lines)
  - `hooks/types.go` (17 lines)
- **BREAKING**: Removed legacy output formatter (`internal/output/formatter.go` - 491 lines)
- Removed legacy formatter tests (`internal/output/formatter_test.go` - 364 lines)
- Removed `WrapJSON` configuration option from config types

### Fixed
- Integration-specific output wrapping now properly handled by adapters
- Settings file updates are atomic with proper error handling
- Binary path detection improved with multiple fallback strategies

## [0.5.0] - 2025-10-31

### Added
- **Background daemon architecture** for continuous index maintenance
  - `daemon start` - Start daemon in foreground mode
  - `daemon stop` - Stop running daemon gracefully
  - `daemon restart` - Restart daemon with new configuration
  - `daemon status` - Check daemon status and index information
  - `daemon logs` - View daemon output (with follow mode)
  - `daemon rebuild` - Force full index rebuild
  - Health check HTTP endpoint (configurable port)
  - PID file management (`~/.agentic-memorizer/daemon.pid`)
  - Signal handling (SIGINT/SIGTERM for shutdown, SIGUSR1 for rebuild)
- **File system watcher** (`internal/watcher/`)
  - Real-time file change detection using fsnotify
  - Automatic incremental index updates on file create/modify/delete
  - Debouncing to avoid excessive processing
  - Support for new directory monitoring
- **Worker pool** for parallel semantic analysis (`internal/daemon/`)
  - Configurable concurrency (default: 3 workers)
  - Rate limiting for API calls (default: 20/min)
  - Retry logic with exponential backoff
  - Job queue with priority handling
- **Precomputed index system** (`internal/index/`)
  - Index stored at `~/.agentic-memorizer/index.json`
  - Atomic updates with backup/rollback mechanism
  - Index statistics tracking (build time, file counts, errors)
  - Version information in index metadata
- **New `read` command** (`cmd/read/read.go`)
  - Fast, read-only operation (<50ms typical, <10ms measured)
  - Reads precomputed index instead of scanning files
  - Outputs formatted index for agent frameworks
  - Multiple output formats (XML, Markdown)
- **System service integration support**
  - Compatible with systemd (Linux) and launchd (macOS)
  - Service configuration is platform-specific
- **Version information system** (`internal/version/`)
  - Version, git commit, and build date tracking
  - Build-time variable injection support

### Changed
- **BREAKING**: File scanning and analysis moved to background daemon
- **BREAKING**: SessionStart hooks must call `read` command instead of performing direct file scanning
- **BREAKING**: Daemon must be running for automatic index updates (manual `read` still works without daemon)
- Configuration structure expanded with daemon settings:
  - `daemon.enabled` - Enable/disable daemon mode
  - `daemon.debounce_ms` - File change debouncing (default: 500ms)
  - `daemon.workers` - Parallel worker count (default: 3)
  - `daemon.rate_limit_per_min` - API rate limit (default: 20)
  - `daemon.full_rebuild_interval_minutes` - Periodic rebuild interval (default: 60)
  - `daemon.health_check_port` - HTTP health endpoint port (default: 8080)
  - `daemon.log_file` - Daemon log file path
  - `daemon.log_level` - Logging verbosity (debug, info, warn, error)
- README updated with daemon architecture details and usage examples

### Fixed
- CLI command usage strings updated for improved clarity

## [0.4.3] - 2025-10-06

### Fixed
- Config YAML key mismatch preventing API key from being loaded from configuration file (added `yaml` struct tags to match `mapstructure` tags)
- Image semantic analysis failing with media type validation error (image handler now sets specific file extension instead of generic "image" type)

## [0.4.2] - 2025-10-06

### Added
- CHANGELOG.md following Keep a Changelog specification

## [0.4.1] - 2025-10-05

### Added
- Quick Start section in README for 3-minute setup guide
- Example Outputs section showing XML and Markdown format examples with realistic data

### Changed
- Removed XML declaration (`<?xml version="1.0" encoding="UTF-8"?>`) from XML output for cleaner AI consumption
- Updated documentation to reflect XML as default output format

## [0.4.0] - 2025-10-05

### Changed
- **BREAKING**: Default output format changed from Markdown to XML
- Hook commands now use `--format xml --wrap-json` by default
- Updated all documentation and examples to reflect XML as default

### Fixed
- Updated configuration examples to use XML format

## [0.3.0] - 2025-10-05

### Added
- XML output format for structured AI-friendly prompting following Anthropic guidelines
- `--wrap-json` explicit flag for JSON wrapping (replaces implicit behavior)
- Comprehensive unit test suite for output formatter
- Test coverage for hooks manager (table-driven tests)

### Changed
- JSON wrapping now requires explicit `--wrap-json` flag instead of `--format json`
- Improved hook setup code with settings preservation
- Code cleanup: removed redundant comments across codebase

### Fixed
- Bug #1: Hook setup no longer deletes other Claude Code settings (awsCredentialExport, permissions, etc.)
- Bug #2: Hook commands now update correctly using index-based access instead of range variables
- Settings preservation now uses `map[string]any` to maintain all JSON fields

## [0.2.0] - 2025-10-05

### Added
- Comprehensive unit testing suite for metadata package (91.3% coverage)
- Table-based tests for all file type handlers
- Test coverage for metadata extraction, caching, and error handling

## [0.1.0] - 2025-10-04

### Added
- Initial release of Agentic Memorizer
- Semantic file indexing with Claude API integration
- Support for multiple file types (Markdown, DOCX, PPTX, PDF, images, code files, VTT transcripts)
- Hash-based caching system (SHA-256) for efficient re-analysis
- SessionStart hook integration with Claude Code
- `init` subcommand with `--setup-hooks` for automatic configuration
- Metadata extraction for file-specific attributes
- Vision analysis for images using Claude's vision capabilities
- Configurable parallel processing for semantic analysis
- File exclusion system (hidden files, skip lists)
- Markdown output format with emoji-rich formatting
- Cache management with automatic invalidation
- Configuration via YAML file and environment variables
- Command-line interface with Cobra + Viper
- Automatic hook configuration for Claude Code (startup, resume, clear, compact matchers)

[unreleased]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.11.0...HEAD
[0.14.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.12.1...v0.13.0
[0.12.1]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.8.2...v0.9.0
[0.8.2]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.4.3...v0.5.0
[0.4.3]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/leefowlercu/agentic-memorizer/releases/tag/v0.1.0
