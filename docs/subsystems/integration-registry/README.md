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
      - [Claude Code Hook Adapter](#claude-code-hook-adapter)
      - [Claude Code MCP Adapter](#claude-code-mcp-adapter)
      - [Gemini CLI MCP Adapter](#gemini-cli-mcp-adapter)
      - [Codex CLI MCP Adapter](#codex-cli-mcp-adapter)
   - [Output Processors](#output-processors)
4. [Integration Points](#integration-points)
   - [CLI Commands](#cli-commands)
   - [Config Manager](#config-manager)
   - [Index Manager](#index-manager)
5. [Glossary](#glossary)

## Overview

The Integration Registry subsystem provides a framework-agnostic integration system that connects agentic-memorizer with various AI agent platforms including Claude Code, Gemini CLI, and Codex CLI. It enables automatic setup of memory index integration, allowing agent frameworks to access the precomputed memory index during their sessions through framework-specific output formatting and command generation.

The subsystem follows a plugin-based architecture with three main layers: a thread-safe Registry that manages adapter registration and lookup, Adapter implementations that provide framework-specific integration logic, and Output Processors that handle rendering the index in different formats (XML, Markdown, JSON). This layered design separates concerns cleanly, enabling independent evolution of output formatting, integration wrapping, and registry management.

The Integration Registry provides specialized adapters (Claude Code, Gemini CLI, Codex CLI) that offer automatic detection, setup, and framework-specific output wrapping. This approach provides sophisticated automation for all supported platforms.

## Design Principles

### Adapter Pattern

The Integration Registry implements the classic Adapter pattern to provide a uniform interface for diverse AI agent frameworks with different integration mechanisms, configuration approaches, and output requirements.

**Common Interface:**
All integrations implement a shared `Integration` interface that defines metadata methods (name, description, version), detection methods (checking if framework is installed), lifecycle methods (setup, update, remove), command generation, output formatting, and validation. This common interface enables the registry to manage integrations polymorphically without knowing implementation details.

**Framework-Specific Implementations:**
Each adapter encapsulates the specific requirements of its target framework. The Claude Code adapter understands Claude's settings.json format and SessionStart hook mechanism, while MCP adapters configure their respective frameworks' MCP server settings. This encapsulation shields the rest of the system from framework-specific complexity.

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
Before modifying configuration files, the subsystem creates temporary backups with .backup suffix. On successful write, backups are automatically removed via deferred cleanup. Only when errors occur during modification do backups persist, enabling manual recovery. This defensive approach prevents data loss from integration setup failures while avoiding clutter from successful operations.

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
- `GetCommand()` - Generates the shell command that frameworks should invoke to access the index (e.g., `memorizer read --format xml --integration claude-code-hook`)

**Output Formatting:**
- `FormatOutput()` - Transforms base index format into framework-specific output, applying any necessary wrapping or envelope structures (optional for MCP-style integrations that provide tools/resources instead of formatted output)

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

The Integration Registry includes specialized adapter implementations for frameworks with deep integration support and automatic setup capabilities.

#### Claude Code Hook Adapter

The Claude Code Hook adapter (`internal/integrations/adapters/claude/hook_adapter.go`) provides automatic integration with Claude Code through SessionStart hooks that inject the memory index at session initialization.

**Integration Name:** `claude-code-hook`

**Detection:**
Checks for the existence of the `~/.claude` directory and `~/.claude/settings.json` file, indicating Claude Code is installed. If the directory exists but no settings file is present, the adapter creates a minimal settings file during setup.

**Setup Process:**
1. Locates or creates `~/.claude/settings.json` file
2. Reads existing settings, preserving all unknown fields
3. Adds or updates SessionStart hooks with default matchers (startup, resume, clear, compact)
4. Configures hook to run `memorizer read --format xml --integration claude-code-hook`
5. Writes modified settings atomically with temporary backup creation
6. Returns detailed success/failure information

**Output Wrapping:**
The adapter wraps the base format (XML, Markdown, or JSON) in a SessionStart JSON envelope structure:
```json
{
  "continue": true,
  "suppressOutput": true,
  "systemMessage": "Memory index updated: 15 files...",
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "<memory_index>...</memory_index>"
  }
}
```

The `hookSpecificOutput.additionalContext` field contains the complete formatted index, which Claude Code adds to the context window without displaying in the transcript.

**Settings Management:**
Uses atomic file operations with temporary files and renames to prevent corruption. Creates temporary backups before modification that are automatically removed on successful write. Only on write failure does the backup persist for manual recovery. Preserves unknown settings fields to maintain compatibility when Claude Code adds new features.

#### Claude Code MCP Adapter

The Claude Code MCP adapter (`internal/integrations/adapters/claude/mcp_adapter.go`) provides integration through the Model Context Protocol, exposing on-demand tools for semantic search rather than injecting the full index at startup.

**Integration Name:** `claude-code-mcp`

**Detection:**
Checks for TWO requirements:
1. Existence of the `~/.claude.json` file (MCP server configuration)
2. Availability of the `claude` CLI command via PATH

Both must be present for successful detection.

**Setup Process:**
1. Locates or creates `~/.claude.json` file (different from hook adapter's settings.json)
2. Reads existing MCP server configurations
3. Registers the `agentic-memorizer` MCP server with:
   - Command: `memorizer mcp start`
   - Environment variables: `MEMORIZER_MEMORY_ROOT` (path to memory directory)
4. Writes modified configuration atomically with temporary backup creation
5. Verifies registration using `claude mcp get agentic-memorizer`

**Output Behavior:**
The MCP adapter **does not use `FormatOutput()`**. Instead of formatting the entire index for injection, it exposes five MCP tools that Claude Code can invoke on-demand:
- `search_files` - Semantic search across indexed files
- `get_file_metadata` - Retrieve complete metadata for specific files
- `list_recent_files` - List recently modified files
- `get_related_files` - Find files connected through shared tags, topics, or entities
- `search_entities` - Find files mentioning specific entities

**When to Use:**
- **Hook adapter (claude-code-hook)**: Best for complete file awareness, smaller memory directories, always-available context
- **MCP adapter (claude-code-mcp)**: Best for large directories, selective file discovery, reduced initial context size

Many users enable both integrations for maximum flexibility.

#### Gemini CLI MCP Adapter

The Gemini CLI MCP adapter (`internal/integrations/adapters/gemini/mcp_adapter.go`) provides integration with Google's Gemini CLI through the Model Context Protocol, exposing on-demand tools for semantic search.

**Integration Name:** `gemini-cli-mcp`

**Detection:**
Checks for TWO requirements:
1. Existence of the `~/.gemini/` directory
2. Availability of the `gemini` CLI command via PATH

Both must be present for successful detection.

**Setup Process:**
1. Locates or creates `~/.gemini/settings.json` file
2. Reads existing MCP server configurations
3. Registers the `agentic-memorizer` MCP server with:
   - Command: `memorizer mcp start`
   - Environment variables: `MEMORIZER_MEMORY_ROOT` (path to memory directory)
   - Note: No explicit `type` field needed (Gemini defaults to stdio transport)
4. Writes modified configuration atomically with temporary backup creation

**Output Behavior:**
Like the Claude Code MCP adapter, the Gemini CLI MCP adapter **does not use `FormatOutput()`**. It exposes five MCP tools that Gemini CLI can invoke on-demand:
- `search_files` - Semantic search across indexed files
- `get_file_metadata` - Retrieve complete metadata for specific files
- `list_recent_files` - List recently modified files
- `get_related_files` - Find files connected through shared tags, topics, or entities
- `search_entities` - Find files mentioning specific entities

**When to Use:**
The Gemini CLI MCP adapter is the only integration option for Gemini CLI users. It provides on-demand file discovery and metadata retrieval through MCP tools during Gemini CLI chat sessions.

#### Codex CLI MCP Adapter

The Codex CLI MCP adapter (`internal/integrations/adapters/codex/mcp_adapter.go`) provides integration with OpenAI's Codex CLI through the Model Context Protocol, exposing on-demand tools for semantic search.

**Integration Name:** `codex-cli-mcp`

**Detection:**
Checks for TWO requirements:
1. Existence of the `~/.codex/` directory
2. Availability of the `codex` CLI command via PATH

Both must be present for successful detection.

**Setup Process:**
1. Locates or creates `~/.codex/config.toml` file
2. Reads existing MCP server configurations (TOML format)
3. Registers the `agentic-memorizer` MCP server with:
   - Command: `memorizer mcp start`
   - Args: `["mcp", "start"]`
   - Environment variables: `MEMORIZER_MEMORY_ROOT` (path to memory directory)
   - Enabled: `true` (explicit flag to activate the server)
4. Writes modified configuration atomically with temporary backup creation

**Output Behavior:**
Like other MCP adapters, the Codex CLI MCP adapter **does not use `FormatOutput()`**. It exposes five MCP tools that Codex CLI can invoke on-demand:
- `search_files` - Semantic search across indexed files
- `get_file_metadata` - Retrieve complete metadata for specific files
- `list_recent_files` - List recently modified files
- `get_related_files` - Find files connected through shared tags, topics, or entities
- `search_entities` - Find files mentioning specific entities

**Configuration Format:**
Unlike Claude Code and Gemini CLI which use JSON, Codex CLI uses TOML format (`~/.codex/config.toml`). The adapter uses `github.com/pelletier/go-toml/v2` for TOML parsing and generation, handling optional fields with pointer types (`*bool`, `*int`) and preserving non-MCP configuration sections during updates.

**When to Use:**
The Codex CLI MCP adapter is the only integration option for Codex CLI users. It provides on-demand file discovery and metadata retrieval through MCP tools during Codex CLI chat sessions.

**Configuration Files Modified by Integrations:**

| Integration | Config File | Section |
|-------------|-------------|---------|
| `claude-code-hook` | `~/.claude/settings.json` | `hooks.SessionStart` |
| `claude-code-mcp` | `~/.claude.json` | `mcpServers.agentic-memorizer` |
| `gemini-cli-mcp` | `~/.gemini/settings.json` | `mcpServers.agentic-memorizer` |
| `codex-cli-mcp` | `~/.codex/config.toml` | `mcp_servers.agentic-memorizer` |

All config file operations use atomic writes with backup pattern for safety.

### Output Processors

Output Processors (`internal/integrations/output/`) are independent formatters that render the memory index into different base formats before integration-specific wrapping is applied.

**Processor Interface:**
All processors implement the `GraphOutputProcessor` interface with `FormatGraph()` and `GetFormat()` methods. The `FormatGraph()` method accepts a GraphIndex and returns formatted output as a string. Processors can be configured with Options (e.g., ShowRecentDays filter) at creation time.

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
Configures the specified integration. Performs automatic setup including config file modification and validation. Updates `integrations.enabled` list in config.yaml to track configured integrations.

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
The `IntegrationsConfig` type (`internal/integrations/types.go`) contains a simple list tracking which integrations are enabled:
- `Enabled` ([]string) - Array of enabled integration names

This lightweight configuration tracks which integrations have been set up without storing detailed framework-specific settings. Detailed settings like SessionStart matchers or MCP environment variables are stored in framework-specific configuration files (`~/.claude/settings.json`, `~/.claude.json`, etc.) rather than in agentic-memorizer's config.yaml.

**Configuration Lifecycle:**
1. Config Manager loads enabled integrations list during initialization
2. `integrations setup` command adds integration names to the Enabled list
3. `integrations remove` command removes integration names from the Enabled list
4. Adapters read their detailed configuration from framework-specific files
5. Changes persist to config.yaml's `integrations.enabled` array

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

**Output Processor**: Independent formatter implementing the GraphOutputProcessor interface that renders GraphIndex data into a specific base format without knowledge of integrations.

**SessionStart Hook**: Claude Code's mechanism for running commands at session initialization, triggered by matchers like "startup", "resume", "clear", or "compact".

**Matchers**: Keywords or patterns that trigger SessionStart hooks in Claude Code, determining when the memory index command executes.

**Framework Detection**: Process of checking if an AI agent framework is installed by verifying the existence of configuration directories, files, or other markers.

**Integration Lifecycle**: Sequence of operations from initial setup through updates, validation, reloading, and eventual removal of an integration configuration.

**Atomic Write**: File write pattern using temporary file creation followed by atomic rename to prevent corruption if process is interrupted during write.

**Global Registry**: Singleton registry instance accessible throughout the application via `GlobalRegistry()`, managing all registered integration adapters.

**Gemini CLI Integration**: MCP-based integration with Google's Gemini CLI tool through stdio transport configuration in `~/.gemini/settings.json`, providing on-demand file search and metadata retrieval via MCP tools.

**Codex CLI Integration**: MCP-based integration with OpenAI's Codex CLI tool through stdio transport configuration in `~/.codex/config.toml` (TOML format), providing on-demand file search and metadata retrieval via MCP tools.

**Specialized Adapter**: Full-featured adapter implementation with automatic detection, setup, and framework-specific output wrapping (e.g., Claude Code adapter, Gemini CLI adapter, Codex CLI adapter).

**Auto-Registration**: Pattern where adapters register themselves with the global registry through init() functions during package initialization.

**Thread-Safe Registry**: Registry implementation using sync.RWMutex to enable concurrent read operations while serializing write operations.

**Base Format**: The underlying output format (XML, Markdown, JSON) that serves as input to integration-specific wrapping or transformation.

**Settings Preservation**: Practice of reading complete settings files, modifying only specific sections, and writing back all fields to maintain compatibility with unknown features.

**Integration Health Check**: Comprehensive validation process checking framework installation, configuration validity, binary accessibility, and settings file integrity.
