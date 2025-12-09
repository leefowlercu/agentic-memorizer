# Config Manager Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Configuration Tiers](#configuration-tiers)
   - [Layered Configuration Priority](#layered-configuration-priority)
   - [Error Accumulation Pattern](#error-accumulation-pattern)
   - [Path Expansion and Safety](#path-expansion-and-safety)
   - [Separation of Concerns](#separation-of-concerns)
3. [Key Components](#key-components)
   - [Configuration Loading](#configuration-loading)
   - [Configuration Types](#configuration-types)
   - [Configuration Schema](#configuration-schema)
   - [Schema Generator](#schema-generator)
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

### Configuration Tiers

The Config Manager organizes settings into three distinct tiers that balance flexibility with simplicity, making configuration approachable for new users while providing full control for advanced users.

**Tier 1: Minimal (shown in initialized config.yaml)**
These settings appear in the default configuration file and are commonly adjusted:
- `memory_root` - Directory to index
- `claude.api_key` - Claude API credentials
- `claude.model` - Claude model for semantic analysis
- `daemon.http_port` - HTTP API port (0 = disabled)
- `daemon.log_level` - Logging verbosity for daemon operations
- `mcp.log_level` - Logging verbosity for MCP server
- `mcp.daemon_host` - Daemon HTTP host for MCP connectivity (conditional)
- `mcp.daemon_port` - Daemon HTTP port for MCP connectivity (conditional)
- `graph.host`, `graph.port` - FalkorDB connection
- `graph.password` - FalkorDB authentication password
- `embeddings.api_key` - OpenAI API key for embeddings (conditional)
- `integrations.enabled` - List of enabled integrations

**Tier 2: Advanced (documented, not shown by default)**
These settings can be added to `config.yaml` to override sensible defaults:
- `claude.max_tokens` - Maximum tokens per request (default: 1500)
- `claude.timeout` - API request timeout in seconds (default: 30)
- `claude.enable_vision` - Enable image analysis (default: true)
- `analysis.max_file_size` - Maximum file size for analysis (default: 10MB)
- `analysis.skip_extensions`, `analysis.skip_files` - File filtering
- `daemon.workers`, `daemon.debounce_ms`, `daemon.rate_limit_per_min` - Worker pool tuning
- `embeddings.provider` - Embedding provider (default: "openai")
- `embeddings.model` - Embedding model (default: "text-embedding-3-small")
- `embeddings.dimensions` - Vector dimensions (default: 1536)
- `graph.similarity_threshold`, `graph.max_similar_files` - Graph tuning

**Tier 3: Hardcoded (documented, not configurable)**
These values are hardcoded for consistency and operational correctness:
- Environment variable names: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `FALKORDB_PASSWORD`
- Application directory name: `.agentic-memorizer`
- Configuration file name: `config.yaml`
- Internal batch sizes and retry logic

**Discovering Configuration Options:**
Users can explore all available settings using the `config show-schema` command:
```bash
# Show all settings with defaults and descriptions
memorizer config show-schema

# Show only advanced settings
memorizer config show-schema --advanced-only

# Show only hardcoded conventions
memorizer config show-schema --hardcoded-only

# Output as YAML or JSON
memorizer config show-schema --format yaml
```

### Layered Configuration Priority

The Config Manager implements a multi-layer configuration system where settings can come from multiple sources with clear precedence rules. This design enables flexibility while maintaining predictability about which values take effect.

**Configuration Layers (lowest to highest priority):**
1. **Default Values**: Hardcoded defaults from constants ensuring zero-configuration operation
2. **Configuration File**: YAML file in `~/.memorizer/config.yaml` or current directory
3. **Environment Variables**: Variables prefixed with `MEMORIZER_` override file settings
4. **Command-Line Flags**: Explicit flags take highest precedence (when bound via Viper)

**Search Path for Configuration Files:**
The subsystem searches for configuration files in multiple locations, using the first one found:
1. `./config.yaml` - Project-specific configuration in current directory (searched first)
2. `$HOME/.agentic-memorizer/config.yaml` - User-specific configuration (searched second)

This search strategy prioritizes project-specific configurations over user defaults, enabling per-project overrides while maintaining global user preferences as fallback. Projects can provide recommended configurations that take precedence over personal defaults.

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

**Special Environment Variables:**
Note that `MEMORIZER_APP_DIR` is a special system variable that affects configuration file search paths rather than overriding configuration values. It controls where the application looks for its own files (config, index, PID, logs) but does not override the `memory_root` or `analysis.cache_dir` settings.

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
The subsystem automatically expands tilde (`~`) prefix in paths to the user's home directory. This expansion enables portable configuration files that work across different user accounts and operating systems without hardcoding absolute paths. Paths like `~/.memorizer/memory` work consistently whether the user is `/home/alice` or `/Users/bob`.

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
2. Adds search paths (`~/.memorizer` and current directory)
3. Loads all default values from `DefaultConfig` constant
4. Configures environment variable prefix (`MEMORIZER_`)
5. Enables automatic environment variable binding with dot-to-underscore transformation
6. Attempts to read configuration file (non-fatal if absent)
7. Binds environment variables to configuration structure

**Configuration Retrieval:**
The `GetConfig()` function provides access to loaded configuration:
1. Unmarshals Viper configuration into strongly-typed `Config` struct
2. Expands tilde notation in paths to full home directory paths
3. Resolves API keys from hardcoded environment variable names (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `FALKORDB_PASSWORD`)
4. Derives `analysis.enabled` and `embeddings.enabled` from API key presence
5. Returns complete configuration ready for consumption by other subsystems

**Configuration Writing:**
The `WriteConfig()` function enables programmatic configuration creation:
1. Marshals configuration structure to YAML format
2. Writes to specified file path with appropriate permissions
3. Used by initialize command to create default configuration files

The `WriteMinimalConfig()` function writes only user-facing settings:
1. Converts full `Config` to `MinimalConfig` via `ToMinimalConfig()` method
2. Filters out advanced/internal settings not needed for typical usage
3. Marshals minimal configuration to YAML format
4. Used by initialize command to create approachable default configuration files

**Path Helper Functions:**
- `GetAppDir()` - Returns app directory path (respects `MEMORIZER_APP_DIR` environment variable, defaults to `~/.memorizer`)
- `GetPIDPath()` - Returns path to daemon PID file (uses app directory)
- `ExpandHome()` - Expands tilde in arbitrary paths
- `GetConfigPath()` - Returns path to loaded configuration file

**App Directory Environment Variable:**
The `MEMORIZER_APP_DIR` environment variable allows customization of where the application stores its own files (config, index, PID, logs). This enables:
- **Testing**: Isolated test environments without affecting production
- **Multi-instance**: Multiple independent instances for different projects
- **Containers**: Custom paths in Docker or other containerized deployments
- **CI/CD**: Isolated build/test environments

When `MEMORIZER_APP_DIR` is set, it overrides the default `~/.memorizer` location. The path undergoes:
1. **Home expansion**: Tilde (`~`) prefix expanded to user's home directory
2. **Security validation**: Path safety checks prevent directory traversal
3. **Search path update**: Configuration file search uses custom app directory

Note that `MEMORIZER_APP_DIR` only affects the application's own files. The memory directory and cache directory locations are still controlled by `memory_root` and `analysis.cache_dir` settings in the configuration file.

**Configuration Hot-Reload:**
The subsystem supports hot-reloading configuration changes without requiring a daemon restart, enabling dynamic operational adjustments for running deployments.

**Important: Daemon vs MCP Server:**
The daemon and MCP server are separate processes. The hot-reload mechanism (via `config reload` command) only affects the running daemon process. MCP server settings (`mcp.*`) take effect when the MCP server is spawned by an MCP client (e.g., Claude Code). To apply MCP configuration changes, the MCP client must disconnect and reconnect to spawn a new MCP server instance.

**Reload Mechanism:**
Configuration reload is triggered via the `config reload` CLI command, which:
1. Loads the new configuration from file and environment variables
2. Validates the configuration using standard validation rules
3. Checks reload compatibility via `ValidateReload()` function
4. Sends a SIGHUP signal to the running daemon process
5. Daemon's signal handler executes `ReloadConfig()` to apply changes

**Immutable Settings (Require Restart):**

*Daemon Process (require `daemon stop` + `daemon start`):*
- `memory_root` - Changing the watched directory requires full restart
- `analysis.cache_dir` - Cache storage location is initialized at startup
- `daemon.log_file` - Log file handle is opened at startup

*MCP Server Process (require MCP client reconnection):*
- `mcp.log_file` - MCP log file handle is opened at startup
- `mcp.daemon_host` - Daemon API host established at startup
- `mcp.daemon_port` - Daemon API port established at startup
- `mcp.log_level` - Log level configured at startup

**Hot-Reloadable Settings (Daemon):**
These settings can be changed dynamically while the daemon is running:
- **Claude API Settings**: `api_key`, `model`, `max_tokens`
- **Worker Configuration**: `daemon.workers` (concurrent processing capacity)
- **Rate Limiting**: `daemon.rate_limit_per_min` (API call throttling)
- **Debounce Timing**: `daemon.debounce_ms` (file change batching delay)
- **Logging**: `daemon.log_level` (verbosity control)
- **HTTP Server**: `daemon.http_port` (enables/disables health check and SSE endpoints)
- **Rebuild Timing**: `daemon.full_rebuild_interval_minutes` (periodic rebuild schedule)
- **Skip Patterns**: `analysis.skip_extensions`, `analysis.skip_files` (file filtering)

**Component Update Logic:**
When reload is triggered, the daemon detects which settings changed and updates only affected components:
- **Claude API changes**: Creates new semantic analyzer with updated client
- **Worker/rate limit changes**: Signals worker pool to adjust concurrency
- **Debounce changes**: Updates file watcher debounce interval
- **Log level changes**: Reconfigures logger verbosity
- **HTTP port changes**: Restarts HTTP server on new port (SSE clients auto-reconnect)
- **Rebuild interval changes**: Adjusts rebuild timer
- **Skip pattern changes**: Updates walker file filters

**Validation:**
The `ValidateReload()` function (`internal/config/reload.go`) compares old and new configurations to ensure only hot-reloadable fields have changed. If immutable fields differ, the reload is rejected with an error message indicating which settings require a full restart.

**Usage Example:**
```bash
# Edit configuration file
vim ~/.memorizer/config.yaml

# Validate and reload without restarting daemon
memorizer config reload
```

### Configuration Types

The Configuration Types component (`internal/config/types.go`) defines the schema for all configuration using strongly-typed Go structures with comprehensive tag annotations for serialization.

**Root Configuration Structure:**
The `Config` struct serves as the top-level container with eight major sections:
- `MemoryRoot` - Directory path where user files are stored
- `Claude` - Claude API configuration (credentials, model)
- `Analysis` - Semantic analysis configuration
- `Daemon` - Background daemon settings
- `MCP` - Model Context Protocol server configuration
- `Graph` - FalkorDB knowledge graph configuration
- `Embeddings` - Vector embeddings configuration
- `Integrations` - Integration framework configuration

**ClaudeConfig Structure:**
Configures Claude API integration:
- `APIKey` - Direct API key (or empty to use ANTHROPIC_API_KEY environment variable)
- `Model` - Claude model identifier (default: claude-sonnet-4-5-20250929)
- `MaxTokens` - Maximum response length in tokens (default: 1500)
- `Timeout` - API request timeout in seconds (default: 30, valid range: 5-300)
- `EnableVision` - Enable vision API for image analysis (default: true)

**Hardcoded Settings:**
The following settings are hardcoded constants in `internal/config/constants.go`:
- `ClaudeAPIKeyEnv` = "ANTHROPIC_API_KEY" - Environment variable for Claude API key
- `GraphPasswordEnv` = "FALKORDB_PASSWORD" - Environment variable for FalkorDB password
- `EmbeddingsAPIKeyEnv` = "OPENAI_API_KEY" - Environment variable for embeddings API key
- `EmbeddingsCacheEnabled` = true - Embedding caching is always enabled
- `EmbeddingsBatchSize` = 100 - Batch size for embedding generation
- `OutputShowRecentDays` = 7 - Days for recent file display

Use `config show-schema --hardcoded-only` to view all hardcoded settings with their reasons.

**AnalysisConfig Structure:**
Configures semantic analysis behavior:
- `Enabled` - Toggle semantic analysis entirely (default: true)
- `MaxFileSize` - Maximum file size for analysis in bytes (default: 10MB)
- `SkipExtensions` - File extensions to exclude from analysis
- `SkipFiles` - Specific filenames to exclude
- `CacheDir` - Cache storage location (default: `~/.memorizer/.cache`)

Note: The `Parallel` field has been deprecated. Worker concurrency is now controlled by `daemon.workers` configuration.

**DaemonConfig Structure:**
Configures background daemon operation:
- `DebounceMs` - File change debounce delay in milliseconds (default: 500)
- `Workers` - Number of concurrent processing workers (default: 3)
- `RateLimitPerMin` - Maximum API calls per minute (default: 20)
- `FullRebuildIntervalMinutes` - Periodic complete rebuild interval (default: 60)
- `HTTPPort` - Unified HTTP server port for health check and SSE endpoints (default: 0 for disabled)
- `LogFile` - Daemon log file path (default: `~/.memorizer/daemon.log`)
- `LogLevel` - Logging verbosity: debug, info, warn, error (default: info)

Note: Daemon operation is controlled via CLI commands (`daemon start`, `daemon stop`) or service managers, not through configuration.

**MCPConfig Structure:**
Configures Model Context Protocol server:
- `LogFile` - MCP server log file path (default: `~/.memorizer/mcp.log`)
- `LogLevel` - Logging verbosity: debug, info, warn, error (default: info)
- `DaemonHost` - Host for daemon's HTTP API (default: "localhost")
- `DaemonPort` - Port for daemon's HTTP API (default: 0 for disabled)

The MCP configuration is separate from daemon logging, enabling independent logging control for MCP integrations. These settings are applied when the MCP server is initialized. When `DaemonPort` is configured with a non-zero value, the MCP server connects to the daemon's HTTP API (constructed as `http://{DaemonHost}:{DaemonPort}`) for graph-powered queries and real-time index updates. The `GetDaemonURL()` helper method constructs the full URL from host and port.

During `initialize`, if the user enables the daemon HTTP server (`daemon.http_port`), the same port is automatically copied to `mcp.daemon_port` to ensure MCP servers can connect to the daemon.

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

### Configuration Schema

The Configuration Schema component (`internal/config/schema.go` and `internal/config/schema_generator.go`) provides programmatic access to the complete configuration schema through automatic reflection-based generation.

**Architecture:**

The schema is generated automatically via reflection on `Config` and `MinimalConfig` structs, ensuring schema accuracy and eliminating manual maintenance drift. The `GetConfigSchema()` function delegates to `generateConfigSchema()` which:

1. **Introspects MinimalConfig structs** - Walks the MinimalConfig type hierarchy to identify which fields are minimal tier
2. **Introspects Config structs** - Walks the full Config type hierarchy to generate field definitions
3. **Auto-derives tier classification** - Fields present in MinimalConfig are "minimal", all others are "advanced"
4. **Applies metadata** - Enriches fields with descriptions and hot-reload flags from metadata maps
5. **Filters derived fields** - Excludes computed fields like `analysis.enabled` and `embeddings.enabled`

**Key Innovation: Automatic Tier Derivation**

Tier classification is determined by struct presence, not manual maintenance:
- If a field exists in `MinimalConfig` structs → tier = "minimal"
- If a field exists only in `Config` struct → tier = "advanced"
- Schema generation cannot drift from MinimalConfig definitions (single source of truth)

**Metadata Storage:**

Since reflection cannot derive human-readable content, three metadata maps provide descriptions and operational characteristics:
- `fieldDescriptions` - Maps field paths to documentation strings
- `hotReloadSettings` - Maps field paths to hot-reload capability flags
- `sectionDescriptions` - Maps section names to overview text

These maps are maintained in `schema_generator.go` and are the only components requiring manual updates when adding fields.

**Schema Types:**

**ConfigSchema:**
Root schema structure containing:
- `Sections` - List of configuration sections (claude, daemon, analysis, etc.)
- `Hardcoded` - List of non-configurable settings with reasons

**SchemaSection:**
Represents a configuration section:
- `Name` - Section identifier (e.g., "claude", "daemon")
- `Description` - Human-readable section description
- `Fields` - List of fields in this section

**SchemaField:**
Describes a single configuration field:
- `Name` - Field identifier (e.g., "timeout")
- `Type` - Data type (string, int, bool, float64, []string)
- `Default` - Default value from `DefaultConfig` constant
- `Tier` - Configuration tier (minimal or advanced), auto-derived from MinimalConfig presence
- `HotReload` - Whether the setting can be changed without restart
- `Description` - Human-readable field description

**HardcodedSetting:**
Describes a non-configurable constant:
- `Name` - Constant identifier
- `Value` - Hardcoded value
- `Reason` - Explanation of why this is hardcoded

**Usage:**
The `GetConfigSchema()` function returns the complete schema, used by:
- `config show-schema` command for schema discovery
- Documentation generation tools
- Configuration validation

### Schema Generator

The Schema Generator component (`internal/config/schema_generator.go`) implements the reflection-based schema generation system that automatically derives configuration schema from struct definitions.

**Core Functions:**

**generateConfigSchema():**
Main entry point that orchestrates schema generation:
1. Builds map of minimal-tier field paths via `buildMinimalFieldMap()`
2. Generates configuration sections via `generateConfigSections()`
3. Retrieves hardcoded settings via `getHardcodedSettings()`
4. Returns complete `ConfigSchema` with all metadata

**buildMinimalFieldMap():**
Reflects on `MinimalConfig` struct hierarchy to identify minimal-tier fields:
- Walks `MinimalConfig` struct tree recursively
- Extracts field names from `yaml` struct tags
- Builds dot-notation path map (e.g., "daemon.log_level", "claude.api_key")
- Returns map used for automatic tier determination

**generateConfigSections():**
Walks `Config` struct to generate schema sections:
- Iterates through top-level Config fields
- Calls `generateSection()` for each configuration section
- Uses reflection to extract field types and default values from `DefaultConfig`

**generateSection():**
Generates individual section with field definitions:
- Handles special case for top-level `memory_root` string field
- Walks struct fields using reflection to build SchemaField list
- Calls `determineTier()` to auto-derive tier classification
- Applies metadata from description and hot-reload maps
- Filters out derived fields via `isDerivedField()`

**determineTier():**
The key innovation - automatic tier classification:
- Takes field path and minimal fields map
- Returns "minimal" if path exists in map, otherwise "advanced"
- Makes tier drift impossible since MinimalConfig is source of truth

**isDerivedField():**
Filters computed fields from schema output:
- Returns true for `analysis.enabled` (derived from Claude API key presence)
- Returns true for `embeddings.enabled` (derived from embeddings API key presence)
- Prevents user confusion about non-configurable derived state

**Metadata Maps:**

Three maps provide information that cannot be derived via reflection:

- **fieldDescriptions** - Human-readable field documentation (e.g., "API request timeout in seconds (5-300)")
- **hotReloadSettings** - Hot-reload capability flags (true = can reload, false = requires restart)
- **sectionDescriptions** - Configuration section overview text

**Adding New Configuration Fields:**

When adding a new configuration option:
1. Add field to appropriate struct in `types.go` (e.g., `ClaudeConfig`, `DaemonConfig`)
2. Add default value to `DefaultConfig` constant in `constants.go`
3. Add viper default in `InitConfig()` function in `config.go`
4. Add metadata entries in `schema_generator.go`:
   - Field description in `fieldDescriptions` map
   - Hot-reload flag in `hotReloadSettings` map
5. If the field should be minimal tier, add it to corresponding `MinimalConfig` struct in `config.go`
6. Schema generation automatically includes the field with correct tier classification

The reflection-based system ensures tier classification stays synchronized with MinimalConfig definitions, eliminating manual schema maintenance.

### Constants and Defaults

The Constants and Defaults component (`internal/config/constants.go`) centralizes all default values, application constants, and fallback behaviors, ensuring consistent behavior across the system.

**Application Constants:**
- `AppDirName` = ".agentic-memorizer" - User configuration directory name
- `MemoryDirName` = "memory" - Default memory storage directory name
- `CacheDirName` = ".cache" - Cache directory name
- `ConfigFile` = "config.yaml" - Configuration filename
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
- Memory root: `~/.memorizer/memory`
- Claude model: `claude-sonnet-4-5-20250929`
- Claude max tokens: 1500
- Claude timeout: 30 seconds
- Claude enable vision: true
- Max file size: 10485760 (10 MB)
- Cache directory: `~/.memorizer/.cache`
- Daemon workers: 3
- Debounce delay: 500 ms
- Rate limit: 20 calls/minute
- Rebuild interval: 60 minutes
- HTTP port: 0 (disabled)
- Daemon log file: `~/.memorizer/daemon.log`
- Daemon log level: info
- MCP log file: `~/.memorizer/mcp.log`
- MCP log level: info
- MCP daemon host: localhost
- MCP daemon port: 0 (disabled)
- Graph host: localhost
- Graph port: 6379
- Graph similarity threshold: 0.7
- Graph max similar files: 10
- Embeddings provider: openai
- Embeddings model: text-embedding-3-small
- Embeddings dimensions: 1536

**Derived Settings:**
These settings are automatically computed:
- `analysis.enabled` - Derived from Claude API key presence
- `embeddings.enabled` - Derived from embeddings API key presence

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
- API key required (directly or via ANTHROPIC_API_KEY environment variable)
- Model name required (cannot be empty)
- Max tokens range enforcement (1-8192 tokens)
- Timeout range enforcement (5-300 seconds)

**Embeddings Validation:**
- Provider enumeration (only "openai" currently supported)
- Model enumeration (text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002)
- Dimensions must match model (1536 for small/ada-002, 3072 for large)

**Analysis Validation:**
- Max file size non-negative check
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
- `SafePath()` function prevents directory traversal attacks using parent references (`..` in paths)
- `ValidateBinaryPath()` ensures executable paths are safe and accessible
- Applied to all user-specified paths (memory root, cache directory, log files)
- Verifies executables exist, are files (not directories), and have execute permissions

**Deprecated Configuration Detection:**
The validator detects and warns about deprecated configuration keys that have been removed from the system:
- `analysis.parallel` - Removed in favor of `daemon.workers` for worker pool sizing
- Warning messages provide migration guidance to help users update their configuration

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
- **HTTP Server**: `cfg.Daemon.HTTPPort` enables optional HTTP server for health check and SSE endpoints
- **Logging**: `cfg.Daemon.LogFile` and `cfg.Daemon.LogLevel` control log output

**Path Resolution:**
The daemon uses configuration helper functions to locate system files:
- `config.GetPIDPath()` - Location for process ID file
- Path expansion ensures portable configuration across user accounts

**Dynamic Behavior:**
The daemon's operational mode is entirely determined by configuration. Users control whether the daemon runs by starting or stopping the process (via `daemon start`/`daemon stop` or service managers). Adjusting worker counts or rate limits changes concurrency characteristics. All behavior is externalized to configuration rather than hardcoded.

**Configuration Hot-Reload:**
The daemon supports dynamic configuration updates without requiring a full restart, implemented through SIGHUP signal handling.

**Thread-Safe Configuration Storage:**
- Daemon stores configuration with `sync.RWMutex` for concurrent access protection
- `GetConfig()` method provides safe read access to current configuration
- `SetConfig()` method enables atomic configuration replacement

**Reload Process:**
When a SIGHUP signal is received (triggered by `config reload` command):
1. Daemon's signal handler invokes `ReloadConfig()` method
2. Loads new configuration via `config.InitConfig()` and `config.GetConfig()`
3. Validates with `config.ValidateReload()` to ensure compatibility
4. Detects which configuration settings have changed
5. Updates only affected components atomically

**Component-Specific Updates:**
- **Claude API changes**: Replaces semantic analyzer with new instance (uses `atomic.Value` for lock-free replacement)
- **Worker/rate limit changes**: Signals worker pool manager to adjust concurrency
- **Debounce changes**: Updates file watcher debounce interval
- **Log level changes**: Reconfigures logger with new verbosity setting
- **Health port changes**: Stops old health server and starts new one on different port
- **Rebuild interval changes**: Sends signal to rebuild scheduler via channel
- **Skip pattern changes**: Updates walker file filters for future scans

**Immutable Settings:**
Attempts to change `memory_root`, `analysis.cache_dir`, `daemon.log_file`, or `mcp.log_file` are rejected during validation, requiring a full daemon restart.

### Semantic Analyzer

The Semantic Analyzer subsystem relies on the Config Manager for Claude API credentials, model selection, and analysis behavior configuration.

**API Client Configuration:**
The semantic analyzer creates its Claude API client using configuration values:
- `cfg.Claude.APIKey` or resolved from `ANTHROPIC_API_KEY` environment variable
- `cfg.Claude.Model` specifies which Claude model to use for analysis
- `cfg.Claude.MaxTokens` limits response length
- `cfg.Claude.Timeout` configures request timeout in seconds

**Analysis Behavior:**
- `cfg.Claude.EnableVision` enables/disables image analysis using vision capabilities
- `cfg.Analysis.Enabled` is derived from API key presence (enabled when Claude API key is set)
- `cfg.Analysis.MaxFileSize` limits files sent for analysis

**Environment Variable Resolution:**
The analyzer benefits from automatic API key resolution from the `ANTHROPIC_API_KEY` environment variable (hardcoded in `config.ClaudeAPIKeyEnv`), enabling secure credential management without storing keys in configuration files.

**Optional Component Pattern:**
The analyzer is only created when `cfg.Analysis.Enabled` is true. This configuration-driven instantiation allows the system to operate in metadata-only mode without Claude API access.

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
The cache manager is only created when semantic analysis is enabled. The daemon checks `cfg.Analysis.Enabled` before instantiating the cache manager, ensuring resources aren't allocated for unused functionality.

### CLI Commands

All CLI commands depend on the Config Manager to load configuration before executing their operations, establishing configuration as a foundational concern.

**Universal Initialization:**
The root command defines a `PersistentPreRunE` hook that calls `config.InitConfig()` before any command executes (except `initialize` command which creates new configuration). This hook ensures configuration is loaded, validated, and available to all subcommands.

**Command-Specific Usage:**

**Read Command:**
- Uses `--format` flag to determine output format (xml, markdown, json) with hardcoded default of "xml"
- Uses `cfg.MemoryRoot` to locate files for reading
- Connects to FalkorDB using `cfg.Graph` settings (host, port, database name from `cfg.Graph.Database`)

**Init Command:**
- Creates default configuration file using `WriteConfig()`
- Uses `DefaultConfig` as template for generated configuration
- Validates target path before writing

**Config Validate Command:**
- Loads configuration via `InitConfig()`
- Retrieves config via `GetConfig()`
- Runs full validation and reports all errors

**Config Reload Command:**
- Loads new configuration from file and environment variables
- Validates configuration with standard validation rules
- Checks reload compatibility via `ValidateReload()` to ensure only hot-reloadable fields changed
- Locates running daemon process via PID file (`config.GetPIDPath()`)
- Sends SIGHUP signal to daemon to trigger reload
- Reports which settings will be updated and which require restart
- Fails with clear error if immutable settings (memory_root, cache_dir, log files) have changed

**Daemon Command:**
- Reads complete daemon configuration section
- Validates daemon-specific settings
- Uses configuration to control all daemon behavior

**Integration Commands:**
- Read integration-specific configuration
- Validate integration settings
- Configure output formats per integration

## Glossary

**Memory Root**: The directory where users store files they want indexed and analyzed, separate from the application configuration directory (typically `~/.memorizer/memory`).

**Cache Directory**: Storage location for semantic analysis results keyed by file content hash to avoid redundant API calls (typically `~/.memorizer/.cache`).

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

**App Directory**: Location where the application stores its own files (configuration, index, PID file, logs), defaulting to `~/.memorizer` but configurable via `MEMORIZER_APP_DIR` environment variable for testing and multi-instance deployments.

**MEMORIZER_APP_DIR**: Environment variable that overrides the default app directory location, enabling isolated test environments, multiple instances, and custom paths in containerized deployments.

**MCP Log File**: Path to the Model Context Protocol (MCP) server log output, separate from daemon logs, enabling independent logging configuration for MCP integrations (typically `~/.memorizer/mcp.log`).

**MCP Log Level**: Logging verbosity control for MCP server operations (debug, info, warn, error), allows independent configuration from daemon logging for troubleshooting MCP integration issues.

**Configuration Tier**: Categorization of settings as minimal (shown in default config), advanced (documented but hidden by default), or hardcoded (not configurable). Enables approachable initial configuration while providing full control for power users.

**Config Show-Schema**: CLI command (`config show-schema`) that displays the complete configuration schema including all fields, types, defaults, tiers, and hardcoded settings. Supports table, YAML, and JSON output formats.
