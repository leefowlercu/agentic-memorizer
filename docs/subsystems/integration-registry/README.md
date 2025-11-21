# Integration Registry Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Adapter Pattern](#adapter-pattern)
   - [Registry Pattern](#registry-pattern)
   - [Separation of Concerns](#separation-of-concerns)
   - [Safety and Reliability](#safety-and-reliability)
3. [Key Components](#key-components)
   - [Integration Interface](#integration-interface)
   - [Registry Component](#registry-component)
   - [Adapter Implementations](#adapter-implementations)
   - [Output Processors](#output-processors)
4. [Integration Points](#integration-points)
   - [CLI Commands](#cli-commands)
   - [Config Manager](#config-manager)
   - [Index Manager](#index-manager)
5. [Glossary](#glossary)

## Overview

The Integration Registry subsystem provides a framework-agnostic integration system that connects agentic-memorizer with various AI agent platforms including Claude Code, Continue.dev, Cline, Aider, and Cursor AI. It enables automatic or manual setup of memory index integration, allowing agent frameworks to access the precomputed memory index during their sessions through framework-specific output formatting and command generation.

The subsystem follows a plugin-based architecture with three main layers: a thread-safe Registry that manages adapter registration and lookup, Adapter implementations that provide framework-specific integration logic, and Output Processors that handle rendering the index in different formats (XML, Markdown, JSON). This layered design separates concerns cleanly, enabling independent evolution of output formatting, integration wrapping, and registry management.

The Integration Registry distinguishes between specialized adapters (like Claude Code) that provide automatic detection, setup, and framework-specific output wrapping, and generic adapters (for Continue, Cline, Aider, Cursor, and custom integrations) that provide manual setup instructions and plain output formatting. This dual approach balances sophisticated automation for supported platforms with extensibility for emerging frameworks.

## Design Principles

### Adapter Pattern

The Integration Registry implements the classic Adapter pattern to provide a uniform interface for diverse AI agent frameworks with different integration mechanisms, configuration approaches, and output requirements.

**Common Interface:**
All integrations implement a shared `Integration` interface that defines metadata methods (name, description, version), detection methods (checking if framework is installed), lifecycle methods (setup, update, remove), command generation, output formatting, and validation. This common interface enables the registry to manage integrations polymorphically without knowing implementation details.

**Framework-Specific Implementations:**
Each adapter encapsulates the specific requirements of its target framework. The Claude Code adapter understands Claude's settings.json format and SessionStart hook mechanism. Generic adapters provide manual setup instructions tailored to their framework's configuration approach. This encapsulation shields the rest of the system from framework-specific complexity.

**Pluggable Architecture:**
New integrations can be added by implementing the Integration interface and registering with the global registry, typically through an init() function. The registry handles discovery, validation, and lifecycle management automatically. This pluggability enables the system to support new frameworks without modifying existing code or the registry implementation.

**Separation of Integration Logic:**
The adapter pattern separates integration logic from core functionality. The Index Manager, Semantic Analyzer, and other subsystems have no knowledge of specific frameworks. Integration concerns are isolated in the adapters, enabling the core system to evolve independently from integration requirements.

### Registry Pattern

The Integration Registry implements a centralized registry pattern that manages the collection of available integrations and provides discovery, validation, and access capabilities.

**Singleton Global Registry:**
The subsystem uses a singleton pattern with lazy initialization via `sync.Once` to ensure a single global registry instance exists throughout the application lifecycle. This singleton is accessible via `GlobalRegistry()`, providing a consistent access point for all subsystem interactions with integrations.

**Thread-Safe Operations:**
The registry protects its internal adapter map with a `sync.RWMutex`, enabling concurrent read operations while serializing writes. This thread safety ensures correct behavior when multiple goroutines query or register integrations during initialization or command execution.

**Auto-Registration Mechanism:**
Adapters register themselves with the global registry through init() functions that execute during package initialization. This auto-registration eliminates manual wiring and ensures all compiled-in adapters are automatically available. Adding a new adapter simply requires implementing the interface and adding an init() registration call.

**Discovery Operations:**
The registry provides comprehensive discovery capabilities including listing all registered integrations, checking for existence by name, counting available integrations, and detecting which frameworks are installed on the system. These operations enable commands like `integrations list` and `integrations detect` to provide visibility into available options.

### Separation of Concerns

The Integration Registry maintains clear boundaries between output formatting, integration wrapping, and registry management, enabling independent evolution of each concern.

**Output Processors Independence:**
Output processors (XML, Markdown, JSON) are completely independent from integration adapters. They implement a separate `OutputProcessor` interface and have no knowledge of frameworks or integration wrapping. This separation enables output format evolution without affecting integration logic and vice versa.

**Base Format vs. Integration Wrapping:**
The architecture distinguishes between base output formats (rendered by processors) and integration-specific wrapping (applied by adapters). For example, Claude Code uses XML base format but wraps it in a SessionStart JSON envelope with system message and additional context. This separation enables reusing output processors across multiple integrations.

**Detection Logic Separation:**
Framework detection logic (checking for config files/directories) is isolated in adapter implementations. The registry provides detection operations but delegates to adapters. This separation allows each adapter to implement detection appropriate for its framework without coupling detection logic to registry management.

**Configuration Independence:**
Integration configurations are stored in the Config Manager, but the registry and adapters don't depend on configuration for operation. Configuration provides persistence and user preferences, but the runtime integration system operates independently, enabling programmatic integration management without configuration files.

### Safety and Reliability

The Integration Registry implements several safety and reliability patterns to ensure correct operation even when modifying system configuration files or handling concurrent access.

**Atomic File Operations:**
When modifying framework configuration files (like Claude's settings.json), the subsystem uses atomic write patterns: create temporary file, write content, rename to target. This atomic rename prevents file corruption if the process is interrupted. The write operation either completes entirely or has no effect.

**Backup Creation:**
Before modifying configuration files, the subsystem creates backups with .backup suffix. If errors occur during modification, the backup enables recovery. Users can manually restore from backup if automatic recovery fails. This defensive approach prevents data loss from integration setup failures.

**Error Handling:**
All operations return detailed errors with context about what failed and why. Integration setup errors include suggestions for manual resolution. Validation errors enumerate specific problems with configuration. This comprehensive error reporting helps users diagnose and fix integration issues.

**Thread Safety:**
The registry's mutex-protected operations ensure correct behavior under concurrent access. Multiple commands or goroutines can safely query the registry simultaneously. Registration operations (typically during init) are serialized to prevent race conditions in the adapter map.

## Key Components

### Integration Interface

The Integration interface (`internal/integrations/interface.go`) defines the contract that all framework adapters must implement, providing a uniform API for integration management regardless of framework specifics.

**Metadata Methods:**
- `GetName()` - Returns unique integration identifier (e.g., "claude-code-hook", "continue-dev")
- `GetDescription()` - Provides human-readable description of the integration's purpose
- `GetVersion()` - Indicates adapter version for compatibility tracking

**Detection Methods:**
- `Detect()` - Checks if the framework is installed by looking for configuration directories or files
- `IsEnabled()` - Determines if the integration is currently configured and active

**Lifecycle Methods:**
- `Setup()` - Performs initial integration configuration, potentially modifying framework config files
- `Update()` - Updates existing integration configuration when settings change
- `Remove()` - Removes integration configuration, cleaning up hooks and settings
- `Validate()` - Checks configuration health and returns detailed validation results
- `Reload()` - Reloads configuration from disk after external changes

**Command Generation:**
- `GetCommand()` - Generates the shell command that frameworks should invoke to access the index (e.g., `agentic-memorizer read --format xml --integration claude-code-hook`)

**Output Formatting:**
- `FormatOutput()` - Transforms base index format into framework-specific output, applying any necessary wrapping or envelope structures

### Registry Component

The Registry component (`internal/integrations/registry.go`) manages the collection of available integrations and provides thread-safe operations for registration, lookup, and discovery.

**Core Structure:**
The registry maintains a map from integration name (string) to Integration interface, protected by a read-write mutex. This structure enables fast lookup by name while ensuring thread safety through the mutex.

**Registration Operations:**
- `Register(integration Integration)` - Adds an integration to the registry, typically called from init() functions during package initialization
- Auto-registration pattern eliminates manual wiring and ensures all compiled-in integrations are available

**Lookup Operations:**
- `Get(name string)` - Retrieves a specific integration by name, returning nil if not found
- `Exists(name string)` - Checks if an integration is registered without retrieving it
- `List()` - Returns slice of all registered integrations for iteration
- `Count()` - Returns number of registered integrations
- `Names()` - Returns slice of registered integration names

**Discovery Operations:**
- `DetectAvailable()` - Scans all registered integrations and returns those whose frameworks are installed (Detect() returns true)
- `DetectEnabled()` - Returns integrations that are both installed and configured (IsEnabled() returns true)

**Singleton Access:**
- `GlobalRegistry()` - Returns the global singleton registry instance, initializing it lazily on first access via sync.Once

### Adapter Implementations

The Integration Registry includes two categories of adapter implementations: specialized adapters for frameworks with deep integration support, and generic adapters for frameworks requiring manual setup.

#### Claude Code Adapter

The Claude Code adapter (`internal/integrations/adapters/claude/`) provides sophisticated automatic integration with Claude Code through settings file manipulation and SessionStart hook installation.

**Detection:**
Checks for the existence of the `~/.claude` directory, indicating Claude Code is installed. If the directory exists but no settings file is present, the adapter creates a minimal settings file during setup.

**Setup Process:**
1. Locates or creates `~/.claude/settings.json` file
2. Reads existing settings, preserving all unknown fields
3. Adds or updates SessionStart hooks with matchers (startup, resume, clear, compact)
4. Configures hook to run `agentic-memorizer read --format xml --integration claude-code-hook`
5. Writes modified settings atomically with backup creation
6. Returns detailed success/failure information

**Output Wrapping:**
The adapter wraps XML base format in a SessionStart JSON envelope structure containing:
- `systemMessage` field with usage instructions and file statistics
- `additionalContext` field with the complete XML index
- This envelope conforms to Claude Code's expected SessionStart output format

**Settings Management:**
Uses atomic file operations with temporary files and renames to prevent corruption. Creates backups before modification. Preserves unknown settings fields to maintain compatibility when Claude Code adds new features.

#### Generic Adapters

Generic adapters (`internal/integrations/adapters/generic/`) provide basic integration support for frameworks without specialized implementation, returning manual setup instructions rather than performing automatic configuration.

**Supported Frameworks:**
- Continue.dev (Markdown output default)
- Cline (Markdown output default)
- Aider (Markdown output default)
- Cursor AI (Markdown output default)
- Custom (XML output default)

**Behavior:**
- `Detect()` always returns false (no automatic detection)
- `IsEnabled()` always returns false (no automatic enablement check)
- `Setup()` returns error with detailed manual setup instructions specific to the framework
- `FormatOutput()` uses plain output processors without integration-specific wrapping

**Manual Setup Instructions:**
Each generic adapter provides tailored instructions for configuring its target framework. These instructions guide users through adding the memory index command to their framework's configuration, typically in settings files or command palettes.

### Output Processors

Output Processors (`internal/integrations/output/`) are independent formatters that render the memory index into different base formats before integration-specific wrapping is applied.

**Processor Interface:**
All processors implement the `OutputProcessor` interface with a single `Format()` method that accepts an Index and returns formatted output as a string. Processors can be configured with Options (e.g., ShowRecentDays filter) at creation time.

#### XML Processor

Produces structured XML output with comprehensive metadata and category organization suitable for programmatic parsing.

**Structure:**
- Root `<memory_index>` element containing `<metadata>` and `<categories>` sections
- Metadata includes file count, category count, oldest/newest file timestamps
- Categories group files by semantic classification with individual file entries
- Optional `<recent_activity>` section when filtering by recent days

**File Entries:**
Each file includes path, relative path, modification time, type, category, size, hash, and semantic analysis (summary, tags, topics, document type). Type-specific metadata like dimensions, word count, or page count appears when available.

**XML Safety:**
All text content undergoes XML entity escaping to prevent malformed output from special characters in file paths or summaries.

#### Markdown Processor

Generates human-readable output with emoji indicators, visual formatting, and natural language descriptions designed for direct human consumption.

**Organization:**
- Title section with overall statistics
- Category sections with emoji headers and file counts
- Individual file cards with inline metadata
- Usage guide section with query examples

**Visual Elements:**
- Category emojis (documents, images, code, etc.)
- File type indicators
- Inline metadata badges (pages/slides/words/dimensions)
- Readable/extraction status indicators

**Formatting:**
Uses Markdown heading levels, bold text, inline code, and emoji to create visually organized output that's pleasant to read in terminal or rendered form.

#### JSON Processor

Provides direct JSON serialization of the Index structure with pretty-printing for both programmatic access and human inspection.

**Output:**
- Complete Index structure with metadata, entries, and statistics
- Pretty-printed with 2-space indentation for readability
- Optional recent_entries field when filtering by recent days
- Preserves all data fields without transformation

**Use Cases:**
Ideal for programmatic integration, automated processing, or when consumers need full access to structured index data without parsing XML or Markdown.

## Integration Points

### CLI Commands

The Integration Registry integrates deeply with CLI commands to provide user-facing integration management and framework-specific output generation.

#### Integrations Command Group

The `integrations` command group (`cmd/integrations/integrations.go` parent command with subcommands in `cmd/integrations/subcommands/`) provides comprehensive integration management:

**`integrations list`:**
Lists all registered integrations with their names, descriptions, versions, and detection status. Shows whether each framework is installed and enabled, helping users understand available options.

**`integrations detect`:**
Scans the system for installed frameworks and reports which ones are detected. Useful for discovering what integrations are possible on the current system.

**`integrations setup <name>`:**
Configures the specified integration. For specialized adapters like Claude Code, performs automatic setup including config file modification. For generic adapters, displays detailed manual setup instructions. Updates `integrations.enabled` list in config.yaml to track configured integrations.

**`integrations remove <name>`:**
Removes the specified integration configuration, cleaning up hooks and settings modifications. Restores frameworks to their pre-integration state. Removes integration from `integrations.enabled` list in config.yaml.

**`integrations validate`:**
Checks configuration health for all enabled integrations, reporting any issues with binary paths, settings files, or configuration validity.

**`integrations health`:**
Performs comprehensive health checks including framework detection, configuration validation, binary path verification, and settings file accessibility.

#### Read Command Integration

The `read` command (`cmd/read/read.go`) produces integration-specific output when the `--integration` flag is provided:

1. Loads precomputed index from Index Manager
2. Accepts `--integration <name>` flag to specify target framework
3. Looks up integration adapter from registry
4. Calls adapter's `FormatOutput()` to apply integration-specific wrapping
5. Falls back to plain output processors when no integration specified

**Format Selection:**
The `--format` flag (xml, markdown, json) selects the base output format. The integration adapter then wraps this base format appropriately for its framework.

#### Initialize Command Integration

The `initialize` command (`cmd/initialize/initialize.go`) offers optional integration setup during initial configuration:

1. Detects available frameworks using registry's `DetectAvailable()`
2. Prompts user to select integrations to configure
3. Calls `Setup()` on selected integrations
4. Configures binary path automatically in integration settings
5. Validates setup success before completing initialization

### Config Manager

The Config Manager stores persistent integration configuration that survives across command invocations and daemon restarts.

**Configuration Structure:**
The `IntegrationsConfig` type maps integration names to `IntegrationConfig` structures containing:
- `Type` - Integration type identifier
- `Enabled` - Whether the integration is active
- `OutputFormat` - Preferred output format (xml, markdown, json)
- `Settings` - Framework-specific settings as flexible key-value map

**Settings Examples:**
- Claude Code: `settings_path`, `matchers` for hook configuration
- Generic integrations: `timeout`, custom format preferences
- Settings are adapter-specific and extensible

**Configuration Lifecycle:**
1. Config Manager loads configurations during initialization
2. Integration commands read/write configurations
3. Adapters use configurations to guide setup and validation
4. Changes persist to YAML configuration file

### Index Manager

The Integration Registry collaborates with the Index Manager to access precomputed index data for output formatting.

**Integration Flow:**
1. Read command invokes Index Manager to load precomputed index from disk
2. Index Manager returns complete Index structure with metadata and entries
3. Read command passes Index to selected output processor
4. Processor formats Index into base format (XML, Markdown, JSON)
5. Integration adapter wraps base format with framework-specific envelope
6. Final output goes to stdout for framework consumption

**No Direct Coupling:**
The Integration Registry doesn't directly depend on Index Manager. Instead, commands mediate between the two subsystems, passing Index data from manager to registry for formatting. This loose coupling enables independent evolution of indexing and integration concerns.

**Type System Integration:**
Both subsystems use the shared `Index` type from `pkg/types/types.go`, providing a stable contract for data exchange without direct dependency between subsystems.

## Glossary

**Integration**: A plugin adapter that connects agentic-memorizer with a specific AI agent framework, handling framework-specific configuration and output formatting.

**Adapter**: Concrete implementation of the Integration interface for a particular framework, encapsulating framework-specific setup logic and configuration requirements.

**Output Format**: Base rendering format (XML, Markdown, JSON) produced by output processors before any integration-specific wrapping is applied.

**Integration Wrapping**: Framework-specific envelope or transformation applied to base format to conform to framework expectations (e.g., SessionStart JSON for Claude Code).

**Output Processor**: Independent formatter implementing the OutputProcessor interface that renders Index data into a specific base format without knowledge of integrations.

**SessionStart Hook**: Claude Code's mechanism for running commands at session initialization, triggered by matchers like "startup", "resume", "clear", or "compact".

**Matchers**: Keywords or patterns that trigger SessionStart hooks in Claude Code, determining when the memory index command executes.

**Framework Detection**: Process of checking if an AI agent framework is installed by verifying the existence of configuration directories, files, or other markers.

**Integration Lifecycle**: Sequence of operations from initial setup through updates, validation, reloading, and eventual removal of an integration configuration.

**Atomic Write**: File write pattern using temporary file creation followed by atomic rename to prevent corruption if process is interrupted during write.

**Global Registry**: Singleton registry instance accessible throughout the application via `GlobalRegistry()`, managing all registered integration adapters.

**Generic Adapter**: Fallback adapter implementation for frameworks without specialized support, providing manual setup instructions instead of automatic configuration.

**Specialized Adapter**: Full-featured adapter implementation with automatic detection, setup, and framework-specific output wrapping (e.g., Claude Code adapter).

**Auto-Registration**: Pattern where adapters register themselves with the global registry through init() functions during package initialization.

**Thread-Safe Registry**: Registry implementation using sync.RWMutex to enable concurrent read operations while serializing write operations.

**Base Format**: The underlying output format (XML, Markdown, JSON) that serves as input to integration-specific wrapping or transformation.

**Settings Preservation**: Practice of reading complete settings files, modifying only specific sections, and writing back all fields to maintain compatibility with unknown features.

**Integration Health Check**: Comprehensive validation process checking framework installation, configuration validity, binary accessibility, and settings file integrity.

**Manual Setup Instructions**: Detailed guidance provided by generic adapters for configuring frameworks that don't support automatic integration setup.
