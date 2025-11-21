# Config Manager Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Layered Configuration Priority](#layered-configuration-priority)
   - [Error Accumulation Pattern](#error-accumulation-pattern)
   - [Path Expansion and Safety](#path-expansion-and-safety)
   - [Separation of Concerns](#separation-of-concerns)
3. [Key Components](#key-components)
   - [Configuration Loading](#configuration-loading)
   - [Configuration Types](#configuration-types)
   - [Constants and Defaults](#constants-and-defaults)
   - [Validation System](#validation-system)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Semantic Analyzer](#semantic-analyzer)
   - [Cache Manager](#cache-manager)
   - [CLI Commands](#cli-commands)
5. [Glossary](#glossary)

## Overview

The Config Manager subsystem is responsible for loading, validating, and managing all application configuration for the agentic-memorizer tool. It serves as the central configuration authority that all other subsystems depend on for their settings and operational parameters. The subsystem provides a comprehensive configuration system with multi-source loading (defaults, YAML files, environment variables), extensive validation with actionable error messages, and security features including path sanitization.

The Config Manager implements a layered configuration approach where default values can be overridden by configuration files, which can in turn be overridden by environment variables. This flexibility enables the system to work with zero configuration for quick starts while supporting sophisticated production deployments with fine-tuned settings. The subsystem enforces strong validation rules that catch configuration errors early with detailed feedback, including field-specific error messages and suggestions for correcting issues.

By centralizing all configuration management, the Config Manager provides a single source of truth for system behavior. All subsystems query configuration through strongly-typed structures, eliminating magic strings and enabling compile-time type checking. The subsystem also handles cross-cutting concerns like path expansion (tilde to home directory), security validation (preventing directory traversal), and environment variable resolution for sensitive values like API keys.

## Design Principles

### Layered Configuration Priority

The Config Manager implements a multi-layer configuration system where settings can come from multiple sources with clear precedence rules. This design enables flexibility while maintaining predictability about which values take effect.

**Configuration Layers (lowest to highest priority):**
1. **Default Values**: Hardcoded defaults from constants ensuring zero-configuration operation
2. **Configuration File**: YAML file in `~/.agentic-memorizer/config.yaml` or current directory
3. **Environment Variables**: Variables prefixed with `MEMORIZER_` override file settings
4. **Command-Line Flags**: Explicit flags take highest precedence (when bound via Viper)

**Search Path for Configuration Files:**
The subsystem searches for configuration files in multiple locations, using the first one found:
1. `$HOME/.agentic-memorizer/config.yaml` - User-specific configuration
2. `./config.yaml` - Project-specific configuration in current directory

This search strategy enables both global user preferences and per-project overrides. Users can maintain personal defaults while projects can provide recommended configurations.

**Environment Variable Override Pattern:**
Environment variables use automatic transformation from configuration structure to variable names:
- Prefix: `MEMORIZER_` is prepended to all variables
- Dot-to-underscore: Nested fields use underscores (e.g., `claude.api_key` becomes `MEMORIZER_CLAUDE_API_KEY`)
- Case-insensitive: Variable names are case-insensitive for flexibility

Examples:
- `claude.api_key` becomes `MEMORIZER_CLAUDE_API_KEY`
- `mcp.log_level` becomes `MEMORIZER_MCP_LOG_LEVEL`
- `daemon.log_file` becomes `MEMORIZER_DAEMON_LOG_FILE`

This pattern enables containerized deployments and CI/CD pipelines to inject configuration without modifying files. Sensitive values like API keys can be provided through environment variables, avoiding storage in version control.

### Error Accumulation Pattern

Rather than failing on the first validation error, the Config Manager collects all validation errors and presents them together. This design significantly improves user experience by allowing users to fix all issues in a single iteration rather than discovering problems one at a time.

**Accumulation Process:**
The validation system uses a `Validator` struct that maintains a list of `ValidationError` objects. Each validation function adds errors to this list rather than returning immediately. After all validations complete, the system checks if any errors were accumulated and returns the complete set.

**Structured Error Information:**
Each validation error includes:
- **Field**: The configuration field that failed validation
- **Rule**: The specific validation rule that was violated
- **Message**: Human-readable description of the problem
- **Suggestion**: Actionable recommendation for fixing the issue
- **Value**: The problematic value that triggered the error

**User Experience Impact:**
Consider a configuration with three invalid fields. Traditional validation would report one error, require a fix, then report the second error, require another fix, and finally report the third. Error accumulation reports all three errors immediately, allowing the user to fix everything in one pass. This approach respects user time and reduces frustration.

### Path Expansion and Safety

The Config Manager implements comprehensive path handling that balances user convenience with security. Path expansion enables portable configurations using tilde notation, while safety validation prevents directory traversal attacks.

**Home Directory Expansion:**
The subsystem automatically expands tilde (`~`) prefix in paths to the user's home directory. This expansion enables portable configuration files that work across different user accounts and operating systems without hardcoding absolute paths. Paths like `~/.agentic-memorizer/memory` work consistently whether the user is `/home/alice` or `/Users/bob`.

**Security Validation:**
All paths undergo security validation to prevent directory traversal attacks. The `SafePath()` function checks for parent directory references (`..`) that could enable accessing files outside intended boundaries. Configuration containing paths like `~/memory/../../etc/passwd` would be rejected with a clear security-focused error message.

**Path Types Supported:**
- **Absolute paths**: Paths starting with `/` (Unix) or drive letters (Windows)
- **Relative paths**: Paths relative to current directory
- **Home-relative paths**: Paths starting with `~` for portability

**Applied to Critical Paths:**
Path expansion and safety validation apply to:
- `memory_root` - User file storage directory
- `analysis.cache_dir` - Cache storage location
- `daemon.log_file` - Daemon log output location
- `mcp.log_file` - MCP server log output location

### Separation of Concerns

The Config Manager organizes its functionality into distinct modules with clear responsibilities, enabling independent evolution and simplifying testing and maintenance.

**Module Organization:**

**config.go (Loading and I/O):**
Handles interaction with external systems including reading configuration files, unmarshaling YAML, expanding environment variables, and providing configuration access methods. This module encapsulates all Viper integration and file system operations.

**types.go (Data Structures):**
Defines the schema for configuration using strongly-typed Go structures with tags for both Viper unmarshaling (`mapstructure`) and YAML serialization (`yaml`). This module provides the contract between configuration sources and consuming code.

**constants.go (Defaults):**
Centralizes all default values, application constants, and fallback behaviors. This module ensures the system can operate without configuration and provides reference values for documentation.

**validate.go (Business Logic):**
Implements all validation rules including range checks, enumeration validation, path safety, and cross-field constraints. This module enforces correctness and provides actionable error feedback.

**Benefits of Separation:**
- **Independent Testing**: Each module can be tested in isolation
- **Clear Ownership**: Bugs and enhancements have obvious module homes
- **Reduced Coupling**: Changes to loading logic don't affect validation
- **Simplified Understanding**: Developers can focus on relevant module without understanding entire subsystem

## Key Components

### Configuration Loading

The Configuration Loading component (`internal/config/config.go`) manages the lifecycle of configuration from initialization through access, handling interaction with the Viper configuration library and external sources.

**Initialization Process:**
The `InitConfig()` function performs the complete configuration bootstrap sequence:
1. Sets configuration name to "config" and type to "yaml"
2. Adds search paths (`~/.agentic-memorizer` and current directory)
3. Loads all default values from `DefaultConfig` constant
4. Configures environment variable prefix (`MEMORIZER_`)
5. Enables automatic environment variable binding with dot-to-underscore transformation
6. Attempts to read configuration file (non-fatal if absent)
7. Binds environment variables to configuration structure

**Configuration Retrieval:**
The `GetConfig()` function provides access to loaded configuration:
1. Unmarshals Viper configuration into strongly-typed `Config` struct
2. Expands tilde notation in paths to full home directory paths
3. Resolves API keys from environment variables if `api_key_env` is specified
4. Returns complete configuration ready for consumption by other subsystems

**Configuration Writing:**
The `WriteConfig()` function enables programmatic configuration creation:
1. Marshals configuration structure to YAML format
2. Writes to specified file path with appropriate permissions
3. Used by initialize command to create default configuration files

**Path Helper Functions:**
- `GetAppDir()` - Returns application directory path (respects `MEMORIZER_APP_DIR` environment variable, defaults to `~/.agentic-memorizer`)
- `GetIndexPath()` - Returns path to precomputed index file (uses app directory)
- `GetPIDPath()` - Returns path to daemon PID file (uses app directory)
- `ExpandHome()` - Expands tilde in arbitrary paths
- `GetConfigPath()` - Returns path to loaded configuration file

**App Directory Environment Variable:**
The `MEMORIZER_APP_DIR` environment variable allows customization of where the application stores its own files (config, index, PID, logs). This enables:
- **Testing**: Isolated test environments without affecting production
- **Multi-instance**: Multiple independent instances for different projects
- **Containers**: Custom paths in Docker or other containerized deployments
- **CI/CD**: Isolated build/test environments

When `MEMORIZER_APP_DIR` is set, it overrides the default `~/.agentic-memorizer` location. The path undergoes:
1. **Home expansion**: Tilde (`~`) prefix expanded to user's home directory
2. **Security validation**: Path safety checks prevent directory traversal
3. **Search path update**: Configuration file search uses custom app directory

Note that `MEMORIZER_APP_DIR` only affects the application's own files. The memory directory and cache directory locations are still controlled by `memory_root` and `analysis.cache_dir` settings in the configuration file.

### Configuration Types

The Configuration Types component (`internal/config/types.go`) defines the schema for all configuration using strongly-typed Go structures with comprehensive tag annotations for serialization.

**Root Configuration Structure:**
The `Config` struct serves as the top-level container with seven major sections:
- `MemoryRoot` - Directory path where user files are stored
- `Claude` - Claude API configuration (credentials, model, settings)
- `Output` - Output formatting preferences
- `Analysis` - Semantic analysis configuration
- `Daemon` - Background daemon settings
- `MCP` - Model Context Protocol server configuration
- `Integrations` - Integration framework configuration

**ClaudeConfig Structure:**
Configures Claude API integration:
- `APIKey` - Direct API key (or empty to use environment variable)
- `APIKeyEnv` - Environment variable name containing API key (default: ANTHROPIC_API_KEY)
- `Model` - Claude model identifier (default: claude-sonnet-4-5-20250929)
- `MaxTokens` - Maximum response length in tokens (default: 1500)
- `EnableVision` - Toggle for image analysis capabilities (default: true)
- `TimeoutSeconds` - API request timeout (default: 30 seconds)

**OutputConfig Structure:**
Controls output format and recent activity:
- `Format` - Output format: xml, markdown, or json (default: xml)
- `ShowRecentDays` - Show files modified in last N days in recent activity section (default: 7)

**AnalysisConfig Structure:**
Configures semantic analysis behavior:
- `Enable` - Toggle semantic analysis entirely (default: true)
- `MaxFileSize` - Maximum file size for analysis in bytes (default: 10MB)
- `Parallel` - Number of concurrent analysis workers (default: 3)
- `SkipExtensions` - File extensions to exclude from analysis
- `SkipFiles` - Specific filenames to exclude
- `CacheDir` - Cache storage location (default: `~/.agentic-memorizer/.cache`)

**DaemonConfig Structure:**
Configures background daemon operation:
- `Enabled` - Toggle daemon mode (default: false for on-demand operation)
- `DebounceMs` - File change debounce delay in milliseconds (default: 500)
- `Workers` - Number of concurrent processing workers (default: 3)
- `RateLimitPerMin` - Maximum API calls per minute (default: 20)
- `FullRebuildIntervalMinutes` - Periodic complete rebuild interval (default: 60)
- `HealthCheckPort` - HTTP health endpoint port (default: 0 for disabled)
- `LogFile` - Daemon log file path (default: `~/.agentic-memorizer/daemon.log`)
- `LogLevel` - Logging verbosity: debug, info, warn, error (default: info)

**MCPConfig Structure:**
Configures Model Context Protocol server logging:
- `LogFile` - MCP server log file path (default: `~/.agentic-memorizer/mcp.log`)
- `LogLevel` - Logging verbosity: debug, info, warn, error (default: info)

The MCP configuration is separate from daemon logging, enabling independent logging control for MCP integrations. These settings are applied when the MCP server is initialized.

**IntegrationsConfig Structure:**
Manages integration framework settings:
- `Enabled` - List of enabled integration names (automatically populated by init/setup/remove commands)

Integration-specific configuration (hooks, tools, server settings) is stored in framework-specific files:
- Claude Code SessionStart hooks: `~/.claude/settings.json`
- Claude Code MCP server: `~/.claude.json`
- Continue.dev: `~/.continue/config.json`
- Other frameworks: respective configuration files

**Tag Annotations:**
Each field includes three tags:
- `mapstructure` - For Viper unmarshaling from configuration sources
- `yaml` - For YAML serialization when writing configuration
- `json` - For JSON serialization in API responses

### Constants and Defaults

The Constants and Defaults component (`internal/config/constants.go`) centralizes all default values, application constants, and fallback behaviors, ensuring consistent behavior across the system.

**Application Constants:**
- `AppDirName` = ".agentic-memorizer" - User configuration directory name
- `MemoryDirName` = "memory" - Default memory storage directory name
- `CacheDirName` = ".cache" - Cache directory name
- `ConfigFile` = "config.yaml" - Configuration filename
- `IndexFile` = "index.json" - Precomputed index filename
- `DaemonLogFile` = "daemon.log" - Daemon log filename
- `DaemonPIDFile` = "daemon.pid" - Daemon process ID filename
- `MCPLogFile` = "mcp.log" - MCP server log filename

**Default Skip Patterns:**
The system ships with sensible defaults for files to exclude from indexing:
- **Skip Extensions**: Binary and archive formats that don't benefit from semantic analysis (.zip, .tar, .gz, .exe, .bin, .dmg, .iso)
- **Skip Files**: The agentic-memorizer binary itself to prevent self-indexing

**DefaultConfig Constant:**
A complete `Config` instance with all fields populated with production-ready defaults. This constant enables:
- Zero-configuration operation for quick starts
- Reference documentation showing example values
- Initialization of Viper with sensible defaults
- Basis for generating default configuration files

**Default Values Summary:**
- Memory root: `~/.agentic-memorizer/memory`
- Claude model: claude-sonnet-4-5-20250929
- Max tokens: 1500
- Vision enabled: true
- API timeout: 30 seconds
- Output format: xml
- Cache directory: `~/.agentic-memorizer/.cache`
- Analysis workers: 3
- Daemon disabled by default (on-demand operation)
- Rate limit: 20 calls/minute
- Rebuild interval: 60 minutes
- MCP log file: `~/.agentic-memorizer/mcp.log`
- MCP log level: info

### Validation System

The Validation System component (`internal/config/validate.go`) implements comprehensive validation logic that enforces correctness constraints and provides actionable error feedback.

**Validation Architecture:**
The system uses structured validation with the `Validator` type accumulating errors and the `ValidationError` type capturing detailed information about each violation. This architecture enables comprehensive error reporting where users see all problems at once rather than discovering issues iteratively.

**Validation Categories:**

**Memory Root Validation:**
- Required field check (cannot be empty)
- Path safety validation (no parent directory traversal)
- Directory existence verification (must exist before indexing)
- Type verification (must be directory, not regular file)

**Claude API Validation:**
- API key or environment variable required (one must be specified)
- Model name required (cannot be empty)
- Max tokens range enforcement (1-8192 tokens)
- Timeout range enforcement (1-300 seconds)

**Output Validation:**
- Format enumeration check (must be xml, markdown, or json)
- Recent days range enforcement (0-365 days)

**Analysis Validation:**
- Max file size non-negative check
- Parallel workers range enforcement (1-20 workers)
- Cache directory required and safe path validation
- Skip patterns validity (proper format and safe paths)

**Daemon Validation:**
- Debounce range enforcement (0-10000 milliseconds)
- Workers range enforcement (1-20 workers)
- Rate limit range enforcement (1-200 calls per minute)
- Full rebuild interval non-negative check
- Health check port range validation (0-65535)
- Log level enumeration check (debug, info, warn, error)
- Log file path required and safety validation

**MCP Validation:**
- Log level enumeration check (must be debug, info, warn, or error)
- Log file path required and safety validation (no parent directory traversal)

**Integration Validation:**
- Type field required for each integration
- Output format enumeration validation
- Integration-specific settings validation

**Security Validation:**
- `SafePath()` function prevents directory traversal attacks using parent references
- `ValidateBinaryPath()` ensures executable paths are safe and accessible
- Applied to all user-specified paths (memory root, cache directory, log files)

**Error Structure:**
Each `ValidationError` contains:
- Field name for precise identification
- Validation rule that was violated
- Human-readable message explaining the problem
- Actionable suggestion for correcting the issue
- The problematic value for context

## Integration Points

### Daemon Subsystem

The Daemon subsystem depends on the Config Manager for all operational parameters that control its behavior, from worker pool sizing to logging configuration.

**Initialization:**
The daemon reads its configuration during startup via `config.GetConfig()`, receiving a strongly-typed `Config` struct with all settings validated and ready for use. The daemon stores this configuration and uses it throughout its lifecycle to guide operational decisions.

**Configuration Usage:**
- **Worker Pool Size**: `cfg.Daemon.Workers` determines parallel processing capacity
- **Debounce Timing**: `cfg.Daemon.DebounceMs` controls file change batching
- **Rate Limiting**: `cfg.Daemon.RateLimitPerMin` configures API call throttling
- **Rebuild Interval**: `cfg.Daemon.FullRebuildIntervalMinutes` schedules periodic complete rebuilds
- **Health Monitoring**: `cfg.Daemon.HealthCheckPort` enables optional HTTP health endpoint
- **Logging**: `cfg.Daemon.LogFile` and `cfg.Daemon.LogLevel` control log output

**Path Resolution:**
The daemon uses configuration helper functions to locate system files:
- `config.GetIndexPath()` - Location for index storage
- `config.GetPIDPath()` - Location for process ID file
- Path expansion ensures portable configuration across user accounts

**Dynamic Behavior:**
The daemon's operational mode is entirely determined by configuration. Setting `daemon.enabled = false` switches to on-demand operation. Adjusting worker counts or rate limits changes concurrency characteristics. All behavior is externalized to configuration rather than hardcoded.

### Semantic Analyzer

The Semantic Analyzer subsystem relies on the Config Manager for Claude API credentials, model selection, and analysis behavior configuration.

**API Client Configuration:**
The semantic analyzer creates its Claude API client using configuration values:
- `cfg.Claude.APIKey` or resolved from `cfg.Claude.APIKeyEnv` environment variable
- `cfg.Claude.Model` specifies which Claude model to use for analysis
- `cfg.Claude.MaxTokens` limits response length
- `cfg.Claude.TimeoutSeconds` configures request timeout

**Analysis Behavior:**
- `cfg.Claude.EnableVision` toggles image analysis using vision capabilities
- `cfg.Analysis.Enable` provides a master switch for semantic analysis
- `cfg.Analysis.MaxFileSize` limits files sent for analysis

**Environment Variable Resolution:**
The analyzer benefits from automatic API key resolution from environment variables. When `api_key_env` is set (default: ANTHROPIC_API_KEY), the Config Manager resolves the key from the environment, enabling secure credential management without storing keys in configuration files.

**Optional Component Pattern:**
The analyzer is only created when `cfg.Analysis.Enable` is true. This configuration-driven instantiation allows the system to operate in metadata-only mode without Claude API access.

### Cache Manager

The Cache Manager subsystem depends on the Config Manager to determine cache storage location and behavior.

**Cache Directory Configuration:**
The cache manager is initialized with `cfg.Analysis.CacheDir`, which specifies where cached semantic analysis results should be stored. This path undergoes home expansion and safety validation by the Config Manager before being passed to the cache manager.

**Integration Pattern:**
```
daemon initialization:
  config = GetConfig()
  cacheManager = cache.NewManager(config.Analysis.CacheDir)
  pass cacheManager to worker pool
```

**Path Validation:**
The Config Manager ensures the cache directory path is safe (no directory traversal) and properly expanded (tilde to home directory) before the cache manager uses it. This validation prevents security issues and ensures portable configuration.

**Conditional Creation:**
The cache manager is only created when semantic analysis is enabled. The daemon checks `cfg.Analysis.Enable` before instantiating the cache manager, ensuring resources aren't allocated for unused functionality.

### CLI Commands

All CLI commands depend on the Config Manager to load configuration before executing their operations, establishing configuration as a foundational concern.

**Universal Initialization:**
The root command defines a `PersistentPreRunE` hook that calls `config.InitConfig()` before any command executes (except `initialize` command which creates new configuration). This hook ensures configuration is loaded, validated, and available to all subcommands.

**Command-Specific Usage:**

**Read Command:**
- Uses `cfg.Output.Format` to determine output format (xml, markdown, json)
- Respects `cfg.Output.ShowRecentDays` for recent file filtering
- Uses `cfg.MemoryRoot` to locate files for reading

**Init Command:**
- Creates default configuration file using `WriteConfig()`
- Uses `DefaultConfig` as template for generated configuration
- Validates target path before writing

**Config Validate Command:**
- Loads configuration via `InitConfig()`
- Retrieves config via `GetConfig()`
- Runs full validation and reports all errors

**Daemon Command:**
- Reads complete daemon configuration section
- Validates daemon-specific settings
- Uses configuration to control all daemon behavior

**Integration Commands:**
- Read integration-specific configuration
- Validate integration settings
- Configure output formats per integration

## Glossary

**Memory Root**: The directory where users store files they want indexed and analyzed, separate from the application configuration directory (typically `~/.agentic-memorizer/memory`).

**Cache Directory**: Storage location for semantic analysis results keyed by file content hash to avoid redundant API calls (typically `~/.agentic-memorizer/.cache`).

**Debounce**: Time delay in milliseconds to batch rapid file changes together, preventing excessive rebuilds during bulk file operations like git checkouts or mass edits.

**Full Rebuild Interval**: Periodic complete re-indexing of all files in minutes, even if no changes detected, to ensure index consistency and recover from missed file events.

**Rate Limit Per Minute**: Maximum number of Claude API calls allowed per minute to prevent hitting API quota limits and to control costs.

**Skip Patterns**: File extensions and specific filenames to exclude from indexing, typically binary files, archives, and system files that don't benefit from semantic analysis.

**Vision Support**: Configuration flag enabling semantic analysis of images using Claude's multimodal capabilities, allowing understanding of diagrams, screenshots, and visual content.

**Environment Variable Override**: Configuration values can be overridden via environment variables with `MEMORIZER_` prefix, enabling container deployments and secure credential management.

**Home Expansion**: Automatic conversion of tilde (`~`) prefix in paths to the user's home directory for portable configuration across user accounts and operating systems.

**Path Safety Validation**: Security checks that prevent directory traversal attacks using parent directory references (`..`) in configuration paths.

**Validation Error Accumulation**: Pattern of collecting all validation errors before reporting, providing comprehensive feedback that enables fixing multiple issues in one iteration.

**Layered Configuration**: Multi-source configuration system where defaults can be overridden by files, which can be overridden by environment variables, with clear precedence rules.

**API Key Environment Variable**: Configuration pattern where API keys are stored in environment variables rather than configuration files, referenced by variable name (default: ANTHROPIC_API_KEY).

**Configuration Schema**: Strongly-typed structure definitions using Go structs with tags for YAML/JSON serialization and Viper unmarshaling, providing compile-time type safety.

**Zero-Configuration Operation**: Ability to run the system with sensible defaults without requiring configuration file creation, enabled by comprehensive default values.

**Validation Suggestion**: Actionable recommendation provided with each validation error to guide users toward correct configuration values.

**Integration Configuration**: Framework-specific settings controlling output format and behavior for different AI agent platforms like Claude Code, Continue.dev, and Cursor.

**Health Check Port**: Optional HTTP endpoint port for daemon monitoring, exposing metrics and health status for operational observability (0 disables the endpoint).

**App Directory**: Location where the application stores its own files (configuration, index, PID file, logs), defaulting to `~/.agentic-memorizer` but configurable via `MEMORIZER_APP_DIR` environment variable for testing and multi-instance deployments.

**MEMORIZER_APP_DIR**: Environment variable that overrides the default app directory location, enabling isolated test environments, multiple instances, and custom paths in containerized deployments.

**MCP Log File**: Path to the Model Context Protocol (MCP) server log output, separate from daemon logs, enabling independent logging configuration for MCP integrations (typically `~/.agentic-memorizer/mcp.log`).

**MCP Log Level**: Logging verbosity control for MCP server operations (debug, info, warn, error), allows independent configuration from daemon logging for troubleshooting MCP integration issues.
