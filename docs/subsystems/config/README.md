# Config

Layered configuration management with YAML files, environment variable overrides, validation, and hot-reload support.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Config subsystem provides centralized configuration management for all application components. It implements a layered configuration approach where settings can come from multiple sources with clear precedence: defaults, YAML configuration files, and environment variables.

The subsystem supports three tiers of configuration settings: minimal settings shown in initialized configs that users typically customize, advanced settings with sensible defaults that power users can override, and hardcoded conventions that are not configurable. This tiered approach keeps the default configuration simple while providing flexibility for advanced use cases.

Configuration validation catches errors early with actionable error messages and suggestions. Hot-reload support allows many settings to be changed without restarting the daemon, while structural changes that require restart are clearly identified.

## Design Principles

### Layered Configuration

Configuration values are resolved with clear precedence:

1. **Defaults** - Sensible values defined in code (DefaultConfig)
2. **YAML File** - User configuration in config.yaml
3. **Environment Variables** - Runtime overrides with MEMORIZER_ prefix

Environment variables use underscore replacement for nested keys (e.g., `MEMORIZER_SEMANTIC_PROVIDER` maps to `semantic.provider`).

### Three-Tier Settings

Settings are classified into three tiers based on user exposure:

- **Minimal** - Included in initialized config files; settings users typically need to customize (memory.root, semantic provider/key/model, ports, log levels)
- **Advanced** - Use defaults but can be overridden; power-user settings (workers, debounce, rate limits, timeouts)
- **Hardcoded** - Not configurable; conventions and internal constants (environment variable names, cache behavior, batch sizes)

### Error Accumulation

Validation accumulates all errors rather than failing on the first issue. Each error includes:

- Field name identifying the problematic setting
- Rule that was violated (required, range, enum, security)
- Clear error message
- Actionable suggestion for resolution

### Path Safety

All configurable paths undergo security validation to prevent directory traversal attacks. Paths containing `..` are rejected. Home directory expansion (`~`) is applied after safety checks.

### Credential Resolution

API keys follow a consistent resolution pattern:

1. Check config file value
2. If empty, check provider-specific environment variable (ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY)
3. Features requiring credentials are automatically disabled if no key is found

### Hot-Reload Safety

Settings are classified as either hot-reloadable or requiring restart:

- **Hot-reloadable** - provider, api_key, model, workers, debounce, log_level, rate limits
- **Requires Restart** - memory_root, cache_dir, log_file (structural changes)

The ValidateReload function enforces this distinction, preventing unsafe changes during runtime.

## Key Components

### Config and Types

The Config struct (`internal/config/types.go`) defines the complete configuration structure with sections for:

- **MemoryConfig** - Memory directory root path (`memory.root`)
- **SemanticConfig** - AI provider selection, credentials, analysis constraints, caching, rate limiting
- **DaemonConfig** - File watching skip patterns (`skip_hidden`, `skip_dirs`, `skip_files`, `skip_extensions`), worker pool, debouncing, HTTP server, logging
- **MCPConfig** - MCP server logging and daemon connectivity
- **GraphConfig** - FalkorDB connection and similarity search settings
- **EmbeddingsConfig** - Embedding provider configuration

### InitConfig and GetConfig

Configuration loading (`internal/config/config.go`) follows a two-phase pattern:

- **InitConfig** - Initializes viper with defaults, config paths, and environment binding; supports hot-reload via viper.Reset()
- **GetConfig** - Unmarshals configuration, applies path safety checks, expands home directories, resolves credentials from environment

### Validator

The Validator type (`internal/config/validate.go`) provides structured validation:

- Accumulates multiple errors for comprehensive feedback
- Per-section validation functions (validateMemoryRoot, validateSemantic, validateDaemon, validateMCP, validateGraph, validateEmbeddings)
- Deprecated key detection with migration suggestions

### ValidateReload

Reload validation (`internal/config/reload.go`) checks if configuration changes are compatible with hot-reload, identifying fields that require daemon restart.

### DefaultConfig and Constants

Default values (`internal/config/constants.go`) include:

- Provider-specific defaults (models, rate limits)
- Standard skip patterns for analysis
- Hardcoded environment variable names
- Internal settings not exposed to configuration

### ConfigSchema

The schema system (`internal/config/schema.go`) enables programmatic introspection of configuration settings for the `config show-schema` command, including tier classification and hot-reload status.

## Integration Points

### CLI Commands

The config CLI commands (`cmd/config/`) provide user-facing configuration management:

- `config validate` - Validates configuration and reports errors
- `config reload` - Sends SIGHUP to daemon for hot-reload
- `config show-schema` - Displays all available settings with metadata

### Daemon

The daemon consumes configuration at startup and responds to SIGHUP for hot-reload. Configuration changes trigger atomic replacement of affected components (semantic provider, rate limiter, logger).

### MCP Server

The MCP server loads configuration independently (separate process from daemon). MCP settings changes require client disconnect/reconnect rather than SIGHUP.

### Initialization Wizard

The TUI initialization wizard (`cmd/initialize/`) creates minimal configuration files using ToMinimalConfig() to include only user-facing settings.

### Integration Framework

Integration adapters read configuration to determine binary paths, host settings, and port numbers for generating integration configuration files.

## Glossary

| Term | Definition |
|------|------------|
| Advanced Setting | Configuration setting with sensible defaults that power users can override |
| Credential Resolution | Process of checking config file, then environment variables for API keys |
| Error Accumulation | Validation pattern that collects all errors before reporting |
| Hardcoded Setting | Non-configurable value defined as a constant in code |
| Hot-Reload | Ability to change configuration without restarting the daemon |
| Layered Configuration | Resolution of settings from multiple sources with defined precedence |
| Minimal Setting | Configuration setting included in initialized config files for user customization |
| Path Safety | Validation that prevents directory traversal attacks in configurable paths |
| Tier | Classification of settings by user exposure level (minimal, advanced, hardcoded) |
