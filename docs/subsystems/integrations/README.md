# Integration Framework

Framework-agnostic integration system with dual-hook architecture, MCP servers, and adapter pattern for Claude Code, Gemini CLI, and Codex CLI.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Integration subsystem provides a unified framework for integrating Agentic Memorizer with AI assistant tools. Built around an adapter pattern with a thread-safe registry, it supports multiple integration strategies: hook-based injection for automatic context delivery and MCP servers for on-demand tool access. Each supported tool receives a specialized adapter that handles detection, setup, configuration management, and removal.

The subsystem implements a dual-hook architecture for Claude Code and Gemini CLI: a SessionStart hook injects the file index at session start, while a UserPromptSubmit (Claude) or BeforeAgent (Gemini) hook injects user facts before each prompt. Codex CLI uses MCP-only integration. All adapters share common utilities for file operations, path resolution, and configuration preservation.

Key capabilities include:

- **Adapter pattern** - Common Integration interface with specialized implementations per tool
- **Registry pattern** - Thread-safe singleton with automatic registration via init functions
- **Dual-hook architecture** - SessionStart for files, UserPromptSubmit/BeforeAgent for facts
- **MCP integration** - On-demand tools for search, metadata, related files, and entity lookup
- **Transactional setup** - Rollback on failure with original configuration preservation
- **Configuration preservation** - Existing settings in target files are maintained during setup

## Design Principles

### Interface-Driven Adapter Pattern

All integrations implement a common Integration interface with 13 methods covering the complete lifecycle: Name, Version, Description for identity; Detect for checking existing installations; Setup for configuration; Remove for cleanup; Health for runtime status; and getters for paths, commands, and output format preferences. This enables the CLI commands to work uniformly across all integrations without tool-specific logic.

### Thread-Safe Registry with Auto-Registration

The Registry uses a sync.RWMutex for concurrent access and provides List, Get, and Register operations. Adapters register themselves via init functions, enabling automatic discovery when their packages are imported. The registry follows the same singleton pattern used by semantic providers and metadata handlers.

### Dual Integration Strategy

Each supported tool (except Codex) uses two complementary integration modes. Hook integrations inject context automatically at defined trigger points (session start, before prompts). MCP integrations provide on-demand tools that assistants can invoke when needed. This dual approach ensures context is available proactively while also supporting interactive discovery.

### Transactional Setup with Rollback

Setup operations preserve original configuration before modification. If any step fails, the original file is restored. This prevents partial configurations that could break the target tool. The setup process: read original → merge settings → write new → verify → commit (or rollback on error).

### Configuration Preservation

When modifying existing configuration files (settings.json for Claude, settings.json for Gemini, .codex/config.toml for Codex), adapters preserve all existing settings and only add or update Memorizer-specific entries. JSON adapters merge at the key level; TOML adapters operate on section level.

### Separate Adapters for Hooks and MCP

Rather than combining hook and MCP logic in a single adapter, each integration strategy gets its own adapter file. This separation enables independent versioning, clearer testing, and the ability to enable only hooks or only MCP for a given tool. The naming convention is `{tool}_hook_adapter.go` and `{tool}_mcp_adapter.go`.

## Key Components

### Integration Interface (`interface.go`)

The Integration interface defines the contract all adapters must implement. Identity methods (Name, Version, Description) provide metadata for display. Lifecycle methods (Detect, Setup, Remove, Health) manage the integration state. Path methods (ConfigPath, BinaryPath, ExecutionCommand) return tool-specific locations. Format methods (OutputFormat) indicate preferred output format (XML or Markdown). The interface enables uniform handling by CLI commands.

### Registry (`registry.go`)

The Registry struct holds a map of integrations keyed by name, protected by RWMutex. GlobalRegistry returns the singleton instance via sync.Once. Register adds adapters (typically called from init functions). Get retrieves a specific adapter by name. List returns all registered adapters sorted by name. The registry enables discovery without hardcoded adapter lists.

### Types (`types.go`)

OutputFormat is a string enum with XMLFormat and MarkdownFormat values. The types file defines constants and type aliases used across adapters. Additional types include IntegrationHealth for health check results and SetupOptions for configuration parameters passed to Setup.

### Utilities (`utils.go`)

Shared helper functions used by multiple adapters. FindBinaryPath searches common installation locations ($GOPATH/bin, ~/.local/bin, /usr/local/bin). ReadJSONConfig and WriteJSONConfig handle JSON configuration files with indentation preservation. BackupFile creates timestamped backups before modification. MergeSettings combines existing and new configuration maps.

### Claude Code Hook Adapter (`adapters/claude_code/hook_adapter.go`)

Implements dual-hook integration for Claude Code. SessionStart hook triggers on conversation start and injects the file index via a shell command that calls `memorizer read files --format xml --compact`. UserPromptSubmit hook triggers before each prompt and injects user facts via `memorizer read facts --format xml`. Configuration targets `~/.claude/settings.json` with hooks as shell commands.

### Claude Code MCP Adapter (`adapters/claude_code/mcp_adapter.go`)

Implements MCP server integration for Claude Code. Configures `~/.claude/settings.json` to include the memorizer MCP server in the mcpServers section. The server provides five tools: search_files, get_file_metadata, list_recent_files, get_related_files, and search_entities. MCP configuration includes the binary path and "mcp start" arguments.

### Claude Code Helpers (`adapters/claude_code/helpers.go`)

Shared functions for Claude Code adapters. LoadSettings reads and parses settings.json. SaveSettings writes with proper formatting. EnsureHooksSection and EnsureMCPSection create required parent objects if missing. GetBinaryPath resolves the memorizer binary location. ValidateSettings checks for required fields.

### Gemini CLI Hook Adapter (`adapters/gemini_cli/hook_adapter.go`)

Implements dual-hook integration for Gemini CLI. SessionStart hook injects file index at session start. BeforeAgent hook (Gemini's equivalent to UserPromptSubmit) injects user facts before each agent interaction. Configuration targets `~/.gemini/settings.json` with hook definitions matching Gemini's expected format.

### Gemini CLI MCP Adapter (`adapters/gemini_cli/mcp_adapter.go`)

Implements MCP server integration for Gemini CLI. Configures `~/.gemini/settings.json` to include the memorizer MCP server. Uses the same five tools as Claude Code. Gemini's MCP configuration format differs slightly from Claude's but provides equivalent functionality.

### Codex CLI MCP Adapter (`adapters/codex_cli/mcp_adapter.go`)

Implements MCP-only integration for Codex CLI. Codex does not support hooks, so only MCP integration is available. Configuration targets `~/.codex/config.toml` using TOML format. The adapter uses `github.com/pelletier/go-toml/v2` for parsing and writing. MCP server configuration is added under the `[mcp]` section.

### Codex CLI Helpers (`adapters/codex_cli/helpers.go`)

Shared functions for Codex adapter. LoadConfig reads and parses config.toml. SaveConfig writes with TOML formatting. EnsureMCPSection creates the mcp section if missing. GetBinaryPath resolves the memorizer binary location. The TOML handling differs significantly from JSON adapters.

## Integration Points

### CLI Commands

The integrations CLI commands (`cmd/integrations/`) interact with the subsystem through the Registry. `integrations list` calls Registry.List() to display all available adapters with version and description. `integrations detect` iterates adapters calling Detect() on each. `integrations setup <name>` retrieves an adapter via Get() and calls Setup(). `integrations remove <name>` calls Remove(). `integrations health <name>` calls Health().

### Daemon HTTP API

The daemon exposes endpoints that integrations invoke via their hooks. Hook commands execute `memorizer read files` and `memorizer read facts`, which internally call the daemon's HTTP API (`/api/v1/files` and `/api/v1/facts`) to retrieve current data. MCP tools similarly proxy to daemon endpoints for search, metadata, and relationship queries.

### MCP Server

The MCP server (`internal/mcp/`) is the backend for MCP integrations. When Claude Code, Gemini, or Codex invoke an MCP tool, the request flows to the memorizer MCP server (started via `memorizer mcp start`), which connects to the daemon HTTP API to execute the operation and returns results to the client.

### Configuration System

Integrations respect the global configuration for default paths and settings. The memory_root path, output format preferences, and daemon HTTP port come from the config subsystem. Hook commands construct shell invocations using the configured binary path.

### Format Subsystem

Hook output uses the format subsystem for structured rendering. The `--format xml` and `--format markdown` flags invoke formatters from `internal/format/formatters/`. FilesContent and FactsContent builders wrap domain types for formatting. XML output includes schema metadata; Markdown provides human-readable context.

### Graph Storage

Integration hooks indirectly use graph storage through the read commands. `memorizer read files` calls the graph's GetAll method to retrieve all indexed files. `memorizer read facts` calls the graph's GetAllFacts method. MCP tools like search_files and get_related_files execute graph queries.

## Glossary

**Adapter**
An implementation of the Integration interface for a specific tool (Claude Code, Gemini CLI, Codex CLI). Adapters handle tool-specific configuration formats and hook mechanisms.

**BeforeAgent**
Gemini CLI's hook trigger equivalent to Claude's UserPromptSubmit. Fires before each agent interaction, used to inject user facts.

**Dual-Hook Architecture**
The pattern of using two complementary hooks: one for session start (file index injection) and one for prompt submission (facts injection). Provides both proactive and per-prompt context.

**Hook**
A shell command configured in an AI tool's settings that executes at specific trigger points. Hooks enable automatic context injection without manual invocation.

**Integration**
A configured connection between Memorizer and an AI assistant tool. Can include hooks, MCP servers, or both.

**MCP (Model Context Protocol)**
A protocol for AI assistants to access external tools and resources on demand. Memorizer's MCP server provides five tools for file discovery and search.

**OutputFormat**
An enum (XMLFormat, MarkdownFormat) indicating the preferred output format for hook content. Claude uses XML; other tools may prefer Markdown.

**Registry**
The thread-safe singleton that manages adapter registration and lookup. Uses sync.RWMutex for concurrent access.

**SessionStart**
A hook trigger that fires when a new conversation or session begins. Used to inject the file index for immediate context.

**Setup**
The process of configuring an AI tool to use Memorizer. Includes modifying configuration files, registering hooks, and configuring MCP servers.

**Transactional Setup**
A setup pattern that preserves original configuration and rolls back on failure. Prevents partial configurations that could break the target tool.

**UserPromptSubmit**
Claude Code's hook trigger that fires before each user prompt is sent. Used to inject user facts for per-prompt context.
