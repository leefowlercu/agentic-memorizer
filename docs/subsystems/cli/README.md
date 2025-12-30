# Command Line Interface

Cobra-based CLI with hierarchical command structure, input validation via PreRunE hooks, and consistent output formatting for daemon management, integration setup, and memory operations.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Command Line Interface subsystem provides the primary user interaction layer for Agentic Memorizer. Built on Cobra, it implements a hierarchical command structure with ten top-level commands that organize functionality into logical groups. The CLI handles application initialization, daemon lifecycle management, knowledge graph operations, integration configuration, memory access, and configuration management.

The command structure follows the pattern of parent commands containing subcommands in dedicated packages. Input validation occurs in PreRunE hooks to distinguish user input errors (which show usage) from runtime errors (which suppress usage). All output flows through the format subsystem for consistent, structured display across text, JSON, YAML, XML, and Markdown formats.

Key capabilities include:

- **Hierarchical command structure** - Ten parent commands with subcommands organized by functional area
- **Input validation hooks** - PreRunE functions validate input before SilenceUsage is set
- **Consistent output formatting** - All commands use format subsystem builders instead of raw printf
- **Interactive and unattended modes** - Initialize command supports both TUI wizard and scripted setup
- **Shared utilities** - Common config loading and output helpers in cmd/shared
- **Integration auto-detection** - Binary path detection for hook and MCP server setup

## Design Principles

### Cobra Command Organization

Commands follow a consistent folder structure: parent commands in `cmd/{command}/` with subcommands in `cmd/{command}/subcommands/`. Each parent command file registers its subcommands in an init() function. Subcommand variables use PascalCase exports (StartCmd, StopCmd) enabling the parent to import and register them. This separation prevents monolithic command files while maintaining clear ownership.

### PreRunE Input Validation Pattern

Every command (except root) implements a PreRunE hook for input validation. The hook validates all user-provided input (flags, arguments) before setting `cmd.SilenceUsage = true`. This creates a clear distinction: validation errors show usage to help the user correct their command, while runtime errors (network failures, permission issues) suppress usage since the command was invoked correctly. Named validation functions (validateStart, validateSetup) improve readability over inline closures.

### Long Description Convention

Command Long descriptions follow a specific pattern: opening newline, single concise sentence, double newline, detailed paragraph(s). String concatenation with the + operator breaks long strings at natural phrase boundaries. This produces clean help output with proper paragraph separation while keeping source code readable.

### Format Subsystem Integration

Commands never use fmt.Printf for user output. Instead, they construct format.Status, format.Section, or format.Table builders and pass them through formatters. This ensures consistent styling, supports multiple output formats, and enables structured data extraction. The cmd/shared/output.go package provides helper functions that reduce boilerplate.

### Config Loading via Shared Package

The cmd/shared/config.go package provides GetConfig() which combines config.InitConfig() and config.GetConfig() into a single call. Commands that need configuration call this helper rather than duplicating initialization logic. The root command's PersistentPreRunE handles config initialization for all commands except initialize (which creates the config).

### Error Message Formatting

Error wrapping uses semicolons instead of colons (`fmt.Errorf("failed to connect; %w", err)`). Since the root command prefixes all errors with "Error: ", semicolons create cleaner output with only one colon in the error chain. This convention applies to all error returns throughout the CLI.

## Key Components

### Root Command

The memorizerCmd in cmd/root.go defines the application entry point. It registers all ten parent commands, sets up PersistentPreRunE for config initialization, configures custom version output, and implements the Execute() function that handles error display and usage suppression based on SilenceUsage state.

### Execute Function

The Execute function in cmd/root.go provides the main error handling loop. It silences default Cobra error handling, calls Execute() on the root command, then checks if an error occurred. For errors, it finds the actual command that failed and displays the error message, only showing usage if the command has not set SilenceUsage = true.

### Initialize Command

The initialize command in cmd/initialize/ handles first-time setup. It supports two modes: interactive (TUI wizard via internal/tui/initialize) and unattended (flag-based configuration). The command creates the config file, memory directory, cache directory, optionally starts FalkorDB, and configures integrations. Extensive flag handling supports semantic provider selection, API key configuration, embeddings enablement, and integration setup.

### Daemon Command

The daemon command in cmd/daemon/ provides subcommands for lifecycle management: start (foreground mode), stop, status, restart, rebuild, logs. Additional subcommands systemctl and launchctl generate service manager unit files for background operation. The start subcommand creates the daemon instance and blocks until termination.

### Graph Command

The graph command in cmd/graph/ manages the FalkorDB Docker container with subcommands: start, stop, status. These wrap the internal/docker helper functions to provide user-friendly container lifecycle management.

### Cache Command

The cache command in cmd/cache/ provides subcommands for semantic analysis cache management: status (displays cache statistics) and clear (removes cache entries with --stale or --all flags).

### Read Command

The read command in cmd/read/ exports data from the knowledge graph. The files subcommand outputs the file index in XML, Markdown, or JSON format, optionally wrapped for specific integrations. The facts subcommand outputs user-defined facts. These commands are typically called by integration hooks.

### Remember and Forget Commands

The remember command in cmd/remember/ adds items to memory via two subcommands: file (copy files or directories into the memory directory with conflict resolution and batch support) and fact (store user-defined facts in the knowledge graph). The forget command in cmd/forget/ removes items via two subcommands: file (move files from memory to the `.forgotten` directory for non-destructive removal) and fact (remove a stored fact by ID). File operations use the fileops package for cross-filesystem moves, conflict resolution with `-N` suffix pattern, and batch processing. Both commands interact with the daemon watcher for automatic reindexing and the graph subsystem for fact storage.

### Integrations Command

The integrations command in cmd/integrations/ manages framework integrations. Subcommands include: list (available integrations), detect (framework installation status), setup (configure integration with binary path), remove (unconfigure integration), and health (check integration status). The command registers integration adapters via init() imports.

### Config Command

The config command in cmd/config/ provides configuration utilities: validate (check config file), reload (send SIGHUP to daemon for hot-reload), and show-schema (display all available configuration settings).

### MCP Command

The mcp command in cmd/mcp/ manages the Model Context Protocol server. Currently provides the start subcommand which launches the MCP server process for AI tool integration.

### Version Command

The version command in cmd/version/ displays build information using the version subsystem: semantic version, git commit hash, and build timestamp. Output uses the format subsystem for consistent display.

### Shared Config Helper

The shared.GetConfig() function in cmd/shared/config.go provides a single-call pattern for loading configuration, combining InitConfig and GetConfig. Commands use this instead of duplicating config initialization.

### Shared Output Helpers

The cmd/shared/output.go file provides OutputStatus(), OutputSection(), and OutputError() helpers that obtain the text formatter and format content. These reduce boilerplate in commands that need simple status or section output.

## Integration Points

### Configuration Subsystem

The root command's PersistentPreRunE calls config.InitConfig() for all commands except initialize. Commands access configuration via shared.GetConfig() or directly through config.GetConfig(). The initialize command uses config.WriteMinimalConfig() and config.DefaultConfig for initial setup.

### Daemon Subsystem

The daemon start command creates a daemon.New() instance with configuration and logger, then calls Start() which blocks. Status commands check PID files and process existence. Restart sends SIGHUP for hot-reload operations.

### Format Subsystem

All commands import the format package and format/formatters for output. Commands construct builders (Status, Section, Table) and pass them to formatters obtained via format.GetFormatter(). The shared/output.go helpers wrap common patterns.

### TUI Subsystem

The initialize command in interactive mode calls tuiinit.RunWizard() with initial configuration. The wizard returns WizardResult containing final config, confirmation status, selected integrations, and startup step choices. The command handles post-wizard actions based on these choices.

### Integrations Subsystem

The integrations command imports integration adapters via blank imports to trigger init() registration. Commands call registry.GlobalRegistry() to access the singleton, then use Get(), List(), Detect(), Setup(), Remove(), and IsEnabled() methods.

### Graph Subsystem

The read files command creates a graph.Manager to connect to FalkorDB and uses graph.Exporter to convert the knowledge graph to FileIndex format. The read facts command uses graph.Manager.GetAllFacts() for fact retrieval.

### Docker Subsystem

The graph start/stop/status commands and initialize command use internal/docker helpers for container management: docker.IsAvailable(), docker.IsFalkorDBRunning(), docker.StartFalkorDB(), docker.StopFalkorDB().

### Logging Subsystem

The daemon start command creates a logger via logging.NewLogger() with file, level, and handler options from configuration. This logger is passed to the daemon instance for operation logging.

### Version Subsystem

The version command and root command's version template call version.GetShortVersion(), version.GetGitCommit(), and version.GetBuildDate() for display.

### Fileops Subsystem

The remember file and forget file commands use internal/fileops for filesystem operations. Functions include Copy() and CopyBatch() for file copying, Move() and MoveBatch() for file moving, ResolveConflict() for automatic renaming with `-N` suffix pattern, IsInDirectory() for path validation, and EnsureDir() for directory creation.

## Glossary

**Cobra**
A popular Go library for creating command-line applications. Provides command hierarchy, flag parsing, help generation, and shell completion.

**Parent Command**
A command that groups related subcommands. For example, daemon is a parent command with start, stop, status subcommands.

**PersistentPreRunE**
A Cobra hook that runs before any command or subcommand in the hierarchy. Used on the root command for global initialization.

**PreRunE**
A Cobra hook that runs before a specific command's RunE. Used for input validation before setting SilenceUsage.

**RunE**
A Cobra command function that executes the command logic. Returns an error that Cobra propagates to the Execute caller.

**SilenceUsage**
A Cobra command field that, when true, prevents usage output on error. Set after input validation to distinguish user errors from runtime errors.

**Subcommand**
A command nested under a parent command. For example, daemon start where start is a subcommand of daemon.

**Unattended Mode**
A mode of operation that requires no user interaction, using flags and environment variables for all input. Used for scripted or automated setup.
