# Agentic Memorizer

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25.1-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/leefowlercu/agentic-memorizer)](https://goreportcard.com/report/github.com/leefowlercu/agentic-memorizer)
[![GitHub Release](https://img.shields.io/github/v/release/leefowlercu/agentic-memorizer)](https://github.com/leefowlercu/agentic-memorizer/releases)

A framework-agnostic AI agent memory system that provides automatic awareness and understanding of files in your memory directory through AI-powered semantic analysis. Features native automatic integration for Claude Code and manual integration support for Cursor AI, Continue.dev, Aider, Cline, and custom frameworks.

**Current Version**: v0.11.0 ([CHANGELOG.md](CHANGELOG.md))

## Table of Contents

- [Overview](#overview)
  - [How It Works](#how-it-works)
  - [Key Capabilities](#key-capabilities)
- [Why Use This?](#why-use-this)
- [Supported AI Agent Frameworks](#supported-ai-agent-frameworks)
  - [Automatic Integration (Native Support)](#automatic-integration-native-support)
  - [Manual Integration (Configuration Required)](#manual-integration-configuration-required)
  - [Framework Comparison](#framework-comparison)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
  - [Installation](#installation)
  - [Integration Setup](#integration-setup)
  - [Adding Files to Memory](#adding-files-to-memory)
- [Installation](#installation)
  - [Prerequisites](#prerequisites)
  - [Build and Install](#build-and-install)
  - [Configuration](#configuration)
- [Integration Setup](#integration-setup)
  - [Claude Code Integration (Automatic)](#claude-code-integration-automatic)
  - [Claude Code MCP Integration (Automatic)](#claude-code-mcp-integration-automatic)
  - [Cursor AI Integration (Manual)](#cursor-ai-integration-manual)
  - [Continue.dev Integration (Manual)](#continuedev-integration-manual)
  - [Aider Integration (Manual)](#aider-integration-manual)
  - [Cline Integration (Manual)](#cline-integration-manual)
  - [Custom Framework Integration](#custom-framework-integration)
- [Managing Integrations](#managing-integrations)
- [Usage](#usage)
  - [Background Daemon (Required)](#background-daemon-required)
  - [Running as a Service](#running-as-a-service)
  - [Upgrading](#upgrading)
  - [Adding Files to Memory](#adding-files-to-memory-1)
  - [Manual Testing](#manual-testing)
  - [CLI Usage](#cli-usage)
  - [Controlling Semantic Analysis](#controlling-semantic-analysis)
- [Supported File Types](#supported-file-types)
- [Configuration Options](#configuration-options)
  - [File Exclusions](#file-exclusions)
  - [Environment Variables](#environment-variables)
  - [Output Formats](#output-formats)
- [Example Outputs](#example-outputs)
- [Development](#development)
  - [Project Structure](#project-structure)
  - [Building and Testing](#building-and-testing)
  - [Adding New File Type Handlers](#adding-new-file-type-handlers)
- [Limitations & Known Issues](#limitations--known-issues)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Overview

Agentic Memorizer provides AI agents with persistent, semantic awareness of your local files. Instead of manually managing which files to include in context or repeatedly explaining what files exist, your AI agent automatically receives a comprehensive, AI-powered index showing what files you have, what they contain, their purpose, and how to access them.

Works seamlessly with Claude Code (automatic setup), Cursor AI, Continue.dev, Aider, Cline, and any custom framework that can execute shell commands.

### How It Works

A background daemon continuously watches your designated memory directory (`~/.agentic-memorizer/memory/` by default), automatically discovering and analyzing files as they're added or modified. Each file is processed to extract metadata (word counts, dimensions, page counts, etc.) and—using the Claude API—semantically analyzed to understand its content, purpose, and key topics. This information is maintained in a precomputed index that loads quickly when your AI agent starts.

When you launch your AI agent, the precomputed index is loaded into its context:
- **Claude Code**: SessionStart hooks automatically load the index
- **Other frameworks**: Configure your agent to run the read command on startup

Your AI agent can then:
- **Discover** what files exist without you listing them
- **Understand** file content and purpose before reading them
- **Decide** which files to access based on semantic relevance
- **Access** files efficiently using the appropriate method (Read tool for text/code/images, extraction for PDFs/docs)

### Key Capabilities

**Automatic File Management:**
- Discovers files as you add them to the memory directory
- Updates the index automatically when files are modified or deleted
- Maintains a complete catalog without manual intervention

**Semantic Understanding:**
- AI-powered summaries of file content and purpose
- Semantic tags and key topics for each file
- Document type classification (e.g., "technical-guide", "architecture-diagram")
- Vision analysis for images using Claude's multimodal capabilities

**Efficiency:**
- Background daemon handles all processing asynchronously
- Smart caching only re-analyzes changed files
- Precomputed index enables quick Claude Code startup
- Minimal API usage—only new/modified files are analyzed

**Wide Format Support:**
- **Direct reading**: Markdown, text, JSON/YAML, code files, images, VTT transcripts
- **Extraction supported**: Word documents (DOCX), PowerPoint (PPTX), PDFs
- Automatic metadata extraction for all file types

**Integration:**
- Framework-agnostic with native support for multiple AI agent frameworks
- Automatic setup for Claude Code via SessionStart hooks
- Manual integration for Cursor AI, Continue.dev, Aider, Cline, and custom frameworks
- Configurable output formats (XML, Markdown, JSON)
- Integration management commands for detection, setup, validation, and health checks
- Optional health monitoring and logging

## Why Use This?

**Instead of:**
- ✗ Manually copying file contents into prompts
- ✗ Pre-loading all files into context (wasting tokens)
- ✗ Repeatedly explaining what files exist to Claude
- ✗ Managing which files to include/exclude manually

**You get:**
- ✓ Automatic file awareness on every session
- ✓ Smart, on-demand file access (AI agent decides what to read)
- ✓ Semantic understanding of content before reading
- ✓ Efficient token usage (only index, not full content)
- ✓ Works across sessions with persistent cache

## Supported AI Agent Frameworks

Agentic Memorizer integrates with multiple AI agent frameworks, providing either automatic setup or manual integration instructions.

### Automatic Integration (Native Support)

**Claude Code** - Full automatic integration with one-command setup
- Automatic framework detection and configuration
- One-command setup: `agentic-memorizer integrations setup claude-code-hook`
- SessionStart hook configuration with all matchers (startup, resume, clear, compact)
- Default XML output with JSON envelope wrapping for proper hook formatting
- Full lifecycle management (setup, update, remove, validate)

### Manual Integration (Configuration Required)

The following frameworks require manual configuration file editing. The `integrations setup` command provides detailed, framework-specific instructions:

**Cursor AI** - Manual configuration with context file or custom commands
- Command: `agentic-memorizer integrations setup cursor`
- Provides setup instructions for context files or custom commands
- Markdown output (optimized for readability)

**Continue.dev** - Manual configuration via context providers
- Command: `agentic-memorizer integrations setup continue`
- Provides setup instructions for Continue config file (`~/.continue/config.json`)
- Markdown output (optimized for readability)

**Aider** - Manual integration via shell alias or command execution
- Command: `agentic-memorizer integrations setup aider`
- Provides setup instructions for shell aliases or pre-session commands
- Markdown output (optimized for readability)

**Cline** - Manual configuration via VS Code settings
- Command: `agentic-memorizer integrations setup cline`
- Provides setup instructions for Cline extension settings
- Markdown output (optimized for readability)

**Custom Frameworks** - Generic integration for any framework
- Works with any agent that can execute shell commands
- XML, Markdown, or JSON output formats available
- Just needs ability to execute commands and capture output

### Framework Comparison

| Feature | Claude Code (Hook) | Claude Code (MCP) | Other Frameworks |
|---------|-------------------|-------------------|------------------|
| **Setup Type** | Automatic | Automatic | Manual |
| **Delivery** | SessionStart injection | On-demand tools | User-configured |
| **Output Format** | XML (JSON-wrapped) | N/A (tool-based) | Markdown |
| **Best For** | Complete awareness | Large directories | Flexibility |
| **Validation** | Automatic | Automatic | Manual |

## Architecture

**Three-Phase Processing Pipeline:**

1. **Metadata Extraction** (`internal/metadata/`) - Fast, deterministic extraction using specialized handlers for 10 file type categories
2. **Semantic Analysis** (`internal/semantic/`) - AI-powered content understanding via Claude API
3. **Caching** (`internal/cache/`) - Content-hash-based storage achieving >95% cache hit rates

**Background Daemon** (`internal/daemon/`):
- **Walker** (`internal/walker/`) - Full directory scans during rebuilds
- **File Watcher** (`internal/watcher/`) - Real-time monitoring with fsnotify
- **Worker Pool** - Parallel processing with rate limiting (default 3 workers, 20 calls/min)

**Semantic Search** (`internal/search/`):
- Fuzzy filename matching
- Tag and topic search
- Summary text search
- Relevance-ranked results

**Index Management** (`internal/index/`):
- Thread-safe operations
- Atomic writes via temp file + rename
- Two-level versioning

**Integration Framework** (`internal/integrations/`):
- Adapter pattern for Claude Code (hook + MCP), Cursor, Continue, Aider, Cline, custom
- Independent output processors (XML, Markdown, JSON)

**MCP Server** (`internal/mcp/`):
- JSON-RPC 2.0 stdio transport
- Three tools: `search_files`, `get_file_metadata`, `list_recent_files`
- Integrates with semantic search

**Configuration** (`internal/config/`):
- Layered: defaults → YAML → environment variables
- Hot-reload support via `config reload` command

The daemon handles all processing in the background, so AI agent startup remains quick regardless of file count.

## Quick Start

Get up and running quickly with your AI agent:

### 1. Install

```bash
go install github.com/leefowlercu/agentic-memorizer@latest
```

### 2. Set API Key

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

### 3. Choose Your Integration Path

#### Path A: Claude Code (Automatic Integration)

For Claude Code users, automatic setup configures everything for you:

```bash
agentic-memorizer initialize --setup-integrations
```

This will:
- Create config at `~/.agentic-memorizer/config.yaml`
- Create memory directory at `~/.agentic-memorizer/memory/`
- **Automatically configure Claude Code SessionStart hooks and MCP Server integration** (no manual editing required)

Then start the daemon:
```bash
agentic-memorizer daemon start
# OR set up as system service (recommended):
agentic-memorizer daemon systemctl  # Linux
agentic-memorizer daemon launchctl  # macOS
```

**Skip to step 4 below.**

#### Path B: Other Frameworks (Manual Integration)

For Cursor AI, Continue.dev, Aider, or Cline:

```bash
# Initialize
agentic-memorizer initialize

# Start daemon
agentic-memorizer daemon start
```

Then get framework-specific setup instructions:

```bash
# Get setup instructions for your framework
agentic-memorizer integrations setup cursor      # For Cursor AI
agentic-memorizer integrations setup continue    # For Continue.dev
agentic-memorizer integrations setup aider       # For Aider
agentic-memorizer integrations setup cline       # For Cline
```

Each command provides detailed instructions on where to add the memory index command in your framework's configuration. **Follow those instructions before proceeding to step 4.**

### 4. Add Files to Memory

```bash
# Add any files you want your AI agent to be aware of
cp ~/important-notes.md ~/.agentic-memorizer/memory/
cp ~/project-docs/*.pdf ~/.agentic-memorizer/memory/documents/
```

The daemon will automatically detect and index these files.

### 5. Start Your AI Agent

**Claude Code:**
```bash
claude
```

Claude automatically loads the memory index via SessionStart hooks.

**Other Frameworks:**

Start your agent normally. The memory index will load based on the configuration you set up in step 3.

---

Your AI agent now automatically knows about all files in your memory directory!

For detailed installation options, configuration, and advanced usage, see the sections below.

## Installation

### Prerequisites

- Go 1.25.1 or later
- Claude API key ([get one here](https://console.anthropic.com/))
- An AI agent framework: Claude Code, Cursor AI, Continue.dev, Aider, or Cline

### Build and Install

#### Option 1: Using go install (Recommended)

```bash
go install github.com/leefowlercu/agentic-memorizer@latest
```

Then run the initialize command to set up configuration:

```bash
# Interactive setup (prompts for integrations)
agentic-memorizer initialize

# Or with flags for automated setup
agentic-memorizer initialize --setup-integrations
```

This creates:
- Config file at `~/.agentic-memorizer/config.yaml`
- Memory directory at `~/.agentic-memorizer/memory/`
- Cache directory at `~/.agentic-memorizer/.cache/` (for semantic analysis cache)
- Index file at `~/.agentic-memorizer/index.json` (created by daemon on first run)

The initialize command can optionally configure Claude Code SessionStart hooks automatically with `--setup-integrations`.

After initialization, start the daemon:
```bash
agentic-memorizer daemon start
# OR set up as system service (recommended for production):
agentic-memorizer daemon systemctl  # Linux
agentic-memorizer daemon launchctl  # macOS
```

#### Option 2: Using Makefile

```bash
# Development build (version shows as "dev")
make install

# Production build with version information
make install-release
```

This will:
- Build the `agentic-memorizer` binary (with version info for release builds)
- Install it to `~/.local/bin/agentic-memorizer`

The `install-release` target injects version information from git tags and commits, providing accurate version tracking in logs and index files.

### Configuration

Set your API key via environment variable (recommended):

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

Or edit `~/.agentic-memorizer/config.yaml`:

```yaml
claude:
  api_key: "your-api-key-here"
```

**Custom Setup:**

```bash
# Custom memory directory
agentic-memorizer initialize --memory-root ~/my-memory

# Custom cache directory
agentic-memorizer initialize --cache-dir ~/my-memory/.cache

# Force overwrite existing config
agentic-memorizer initialize --force
```

## Integration Setup

### Claude Code Integration (Automatic)

Claude Code enjoys full automatic integration support with one-command setup.

#### Automatic Setup (Recommended)

```bash
agentic-memorizer integrations setup claude-code-hook
```

This command automatically:
1. Detects your Claude Code installation (`~/.claude/` directory)
2. Creates or updates `~/.claude/settings.json`
3. Preserves existing settings (won't overwrite other configurations)
4. Adds SessionStart hooks for all matchers (startup, resume, clear, compact)
5. Configures the command: `agentic-memorizer read --format xml --integration claude-code-hook`
6. Creates backup at `~/.claude/settings.json.backup`

You can also use the `--setup-integrations` flag during initialization:

```bash
agentic-memorizer initialize --setup-integrations
agentic-memorizer daemon start
```

#### Manual Setup (Alternative)

If you prefer manual configuration, add to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer read --format xml --integration claude-code-hook"
          }
        ]
      }
      // Repeat for "resume", "clear", and "compact" matchers
    ]
  }
}
```

**Note**: Include all four SessionStart matchers to ensure the memory index loads throughout your session lifecycle.

#### Validation

Verify your setup:

```bash
agentic-memorizer integrations validate
```

#### Removal

Remove the integration:

```bash
agentic-memorizer integrations remove claude-code-hook
```

### Claude Code MCP Integration (Automatic)

Claude Code also supports integration via the Model Context Protocol (MCP), providing advanced semantic search capabilities through MCP tools.

#### Automatic Setup (Recommended)

```bash
agentic-memorizer integrations setup claude-code-mcp
```

This command automatically:
1. Detects your Claude Code installation (`~/.claude/` directory)
2. Creates or updates `~/.claude.json` (MCP server configuration)
3. Registers the `agentic-memorizer` MCP server
4. Configures environment variables (`MEMORIZER_MEMORY_ROOT`)
5. Sets the binary command path
6. Creates backup at `~/.claude.json.backup`

#### MCP Tools Provided

The MCP server exposes three tools to Claude Code:

- **`search_files`** - Semantic search across indexed files
  - Fuzzy filename matching
  - Tag and topic search
  - Summary text search
  - Relevance-ranked results
  - Category filtering

- **`get_file_metadata`** - Retrieve complete metadata for a specific file
  - File metadata (size, type, category)
  - Semantic analysis (summary, tags, topics)
  - Document-specific fields (word count, page count, dimensions)

- **`list_recent_files`** - List recently modified files
  - Configurable lookback period (days)
  - Sorted by modification date
  - Includes full metadata

#### MCP Configuration

The MCP server has dedicated configuration in `config.yaml`:

```yaml
mcp:
  log_file: ~/.agentic-memorizer/mcp.log  # MCP server logs
  log_level: info                          # Log level (debug/info/warn/error)
```

#### Running the MCP Server

The MCP server is automatically started by Claude Code when configured. You can also run it manually for testing:

```bash
# Start MCP server in stdio mode
agentic-memorizer mcp start

# Start with debug logging
agentic-memorizer mcp start --log-level debug

# View MCP logs
tail -f ~/.agentic-memorizer/mcp.log
```

The server communicates via stdin/stdout using JSON-RPC 2.0 protocol.

#### MCP vs SessionStart Hooks

You can use one or both Claude Code integration methods:

- **SessionStart Hooks** (`claude-code-hook`): Injects full memory index at session start
  - Best for: Always-available context, complete file awareness
  - Trade-off: Larger initial context, all files loaded upfront

- **MCP Server** (`claude-code-mcp`): Provides on-demand tools for semantic search
  - Best for: Large memory directories, selective file discovery
  - Trade-off: Requires explicit tool use, context fetched on demand

Many users enable both for maximum flexibility.

#### Validation

Verify your MCP setup:

```bash
agentic-memorizer integrations validate
agentic-memorizer integrations health
```

#### Removal

Remove the MCP integration:

```bash
agentic-memorizer integrations remove claude-code-mcp
```

### Cursor AI Integration (Manual)

Cursor AI requires manual configuration.

#### Setup Instructions

Get framework-specific instructions:

```bash
agentic-memorizer integrations setup cursor
```

#### Option A: Context File Generation

Configure a script to regenerate context file:

```bash
#!/bin/bash
agentic-memorizer read --format markdown > ~/.cursor/memory-context.md
```

Add to Cursor's context sources:
- File path: `~/.cursor/memory-context.md`
- Auto-reload: Enabled

#### Option B: Custom Command

If Cursor supports custom commands, add:

```json
{
  "customCommands": [
    {
      "name": "Load Memory",
      "command": "agentic-memorizer read --format markdown"
    }
  ]
}
```

**Output Format**: Cursor works best with **Markdown** output (default for this integration).

### Continue.dev Integration (Manual)

Continue.dev requires manual configuration file editing.

#### Setup Instructions

Get framework-specific instructions:

```bash
agentic-memorizer integrations setup continue
```

#### Configuration

1. Locate your Continue configuration (typically `~/.continue/config.json` or `.continue/config.json` in your workspace)

2. Add a context provider for agentic-memorizer:

```json
{
  "contextProviders": [
    {
      "name": "agentic-memory",
      "params": {
        "command": "agentic-memorizer read --format markdown"
      }
    }
  ]
}
```

3. Restart Continue.dev

#### Usage in Continue

In Continue chat, reference the memory index:

```
@agentic-memory What files do I have about API design?
```

**Output Format**: Continue works best with **Markdown** output (default for this integration).

### Aider Integration (Manual)

Aider can access the memory index via shell integration.

#### Setup Instructions

Get framework-specific instructions:

```bash
agentic-memorizer integrations setup aider
```

#### Option A: Shell Alias (Recommended)

Add to your shell profile (`~/.bashrc`, `~/.zshrc`):

```bash
alias aider-with-memory='agentic-memorizer read --format markdown && aider'
```

Usage:

```bash
aider-with-memory
```

#### Option B: Manual Command

Before starting Aider, run:

```bash
agentic-memorizer read --format markdown > /tmp/memory-index.md
```

Then in Aider:

```
/add /tmp/memory-index.md
```

**Output Format**: Aider works best with **Markdown** output (default for this integration).

### Cline Integration (Manual)

Cline requires manual configuration via VS Code settings.

#### Setup Instructions

Get framework-specific instructions:

```bash
agentic-memorizer integrations setup cline
```

#### Configuration

1. Locate your Cline configuration (typically in VS Code settings under Cline extension)

2. Add a startup command or context provider:

```json
{
  "cline.contextCommands": [
    {
      "name": "Load Memory Index",
      "command": "agentic-memorizer read --format markdown"
    }
  ]
}
```

3. Configure Cline to run this command on session start or manually trigger it

#### Usage in Cline

Run the memory index command via Cline's command palette or configure it to run automatically on startup.

**Output Format**: Cline works best with **Markdown** output (default for this integration).

### Custom Framework Integration

Any AI agent framework that can execute shell commands can integrate with agentic-memorizer.

#### Requirements

Your framework must be able to:
1. Execute shell commands
2. Capture command output (stdout)
3. Add output to the agent's context

#### Setup

1. Determine how your framework executes commands at startup or on-demand

2. Configure it to run:

```bash
agentic-memorizer read --format <xml|markdown|json>
```

3. Choose output format based on your framework:
   - **XML**: Best for programmatic parsing, structured data
   - **Markdown**: Best for human readability, LLM consumption
   - **JSON**: Best for programmatic integration, custom parsing

#### Example Integration Patterns

**Pattern 1: Startup Hook**

```json
{
  "onStartup": {
    "commands": [
      "agentic-memorizer read --format markdown"
    ]
  }
}
```

**Pattern 2: Context Provider**

```json
{
  "contextProviders": [
    {
      "name": "memory",
      "command": "agentic-memorizer read --format markdown"
    }
  ]
}
```

**Pattern 3: Manual Command**

Add a custom agent command that runs:

```bash
agentic-memorizer read --format markdown
```

Then invoke it when needed:

```
@memory What files do I have?
```

#### Testing Your Integration

1. Ensure daemon is running:

```bash
agentic-memorizer daemon status
```

2. Test command output:

```bash
agentic-memorizer read --format markdown
```

3. Verify your framework captures and displays the output

## Managing Integrations

The `integrations` command group provides comprehensive tools for managing integrations with various AI agent frameworks.

### List Available Integrations

```bash
agentic-memorizer integrations list
```

Shows all registered integrations with their status and configuration:

**Example Output:**

```
✓ claude-code-hook
  Description: Claude Code SessionStart hooks integration
  Version:     1.0.0
  Status:      configured

✓ claude-code-mcp
  Description: Claude Code MCP server integration
  Version:     1.0.0
  Status:      configured

○ cursor
  Description: Cursor AI integration (manual setup required)
  Version:     1.0.0
  Status:      not configured

○ continue
  Description: Continue.dev integration (manual setup required)
  Version:     1.0.0
  Status:      not configured

○ aider
  Description: Aider integration (manual setup required)
  Version:     1.0.0
  Status:      not configured

○ cline
  Description: Cline integration (manual setup required)
  Version:     1.0.0
  Status:      not configured
```

### Detect Installed Frameworks

Automatically detect which agent frameworks are installed on your system:

```bash
agentic-memorizer integrations detect
```

**Example Output:**

```
Detected Frameworks:
  ✓ claude-code-hook (installed at ~/.claude)
```

Checks for framework-specific configuration directories and files.

### Setup an Integration

#### Claude Code (Automatic)

```bash
# SessionStart hooks setup
agentic-memorizer integrations setup claude-code-hook

# MCP server setup
agentic-memorizer integrations setup claude-code-mcp

# With custom binary path
agentic-memorizer integrations setup claude-code-hook --binary-path /custom/path/agentic-memorizer
```

For Claude Code, this automatically:
- Detects the Claude settings file (`~/.claude/settings.json`)
- Adds SessionStart hooks for all matchers (startup, resume, clear, compact)
- Configures the correct command with `--integration claude-code-hook` flag
- Preserves existing settings and creates backup

#### Other Frameworks (Manual Instructions)

```bash
# Get framework-specific setup instructions
agentic-memorizer integrations setup cursor      # Cursor AI
agentic-memorizer integrations setup continue    # Continue.dev
agentic-memorizer integrations setup aider       # Aider
agentic-memorizer integrations setup cline       # Cline
```

Each command provides detailed manual setup instructions specific to that framework, including:
- Where to find the framework's configuration file
- What configuration to add
- Example configuration snippets
- Recommended output format

### Remove an Integration

```bash
agentic-memorizer integrations remove claude-code-hook
agentic-memorizer integrations remove claude-code-mcp
```

Removes the integration configuration from the framework's settings file. For Claude Code, this:
- Removes SessionStart hooks added by agentic-memorizer
- Preserves other hooks and settings
- Creates backup before modification

### Validate Configurations

Check that all configured integrations are properly set up:

```bash
agentic-memorizer integrations validate
```

**Example Output:**

```
Validating integrations...
  ✓ claude-code-hook: Valid (settings file exists, hooks configured)
```

Validates:
- Configuration file exists and is readable
- Integration-specific settings are properly formatted
- Required commands are configured

### Health Check

Comprehensive health check including both detection and validation:

```bash
agentic-memorizer integrations health
```

**Example Output:**

```
Framework Detection:
  ✓ claude-code-hook (installed at ~/.claude)

Configuration Validation:
  ✓ claude-code-hook: Valid (settings file exists, hooks configured)

Overall Status: Healthy (1/1 configured integrations valid)
```

Performs:
- Framework installation detection
- Configuration file validation
- Integration setup verification
- Overall health status summary

## Usage

### Background Daemon (Required)

The background daemon is the core of Agentic Memorizer. It maintains a precomputed index for quick startup, watching your memory directory and automatically updating the index as files change.

#### Quick Start

```bash
# Start the daemon (run in foreground - use Ctrl+C to stop)
agentic-memorizer daemon start

# OR set up as system service for automatic management (recommended):
agentic-memorizer daemon systemctl  # Linux
agentic-memorizer daemon launchctl  # macOS
```

**Note**: If you used `initialize --setup-integrations`, the integration is already configured. Otherwise, configure your AI agent framework to call `agentic-memorizer read` (see Integration Setup section above).

#### Daemon Commands

```bash
# Start daemon (runs in foreground - press Ctrl+C to stop)
agentic-memorizer daemon start

# Check daemon status
agentic-memorizer daemon status

# Stop daemon
agentic-memorizer daemon stop

# Restart daemon
agentic-memorizer daemon restart

# Force immediate rebuild
agentic-memorizer daemon rebuild

# View daemon logs
agentic-memorizer daemon logs              # Last 50 lines
agentic-memorizer daemon logs -f           # Follow logs
agentic-memorizer daemon logs -n 100       # Last 100 lines

# Hot-reload configuration without daemon restart
agentic-memorizer config reload
```

#### How It Works

The daemon:
1. **Watches** your memory directory for file changes using fsnotify
2. **Processes** files in parallel using a worker pool (3 workers by default)
3. **Rate limits** API calls to respect Claude API limits (20/min default)
4. **Maintains** a precomputed `index.json` file with all metadata and semantic analysis
5. **Updates** the index automatically when files are added/modified/deleted
6. **Supports** hot-reload of most configuration settings via `config reload` command

When you run `agentic-memorizer read`, it simply loads the precomputed index from disk instead of analyzing all files.

#### Daemon Configuration

In `~/.agentic-memorizer/config.yaml`:

```yaml
daemon:
  enabled: true                          # Enable daemon mode
  debounce_ms: 500                       # Debounce file events (milliseconds)
  workers: 3                             # Parallel worker count
  rate_limit_per_min: 20                 # API rate limit
  full_rebuild_interval_minutes: 60      # Periodic full rebuild interval
  health_check_port: 0                   # HTTP health check (0 = disabled)
  log_file: ~/.agentic-memorizer/daemon.log
  log_level: info                        # debug, info, warn, error
```

**Hot-Reloading**: Most settings can be hot-reloaded using `agentic-memorizer config reload` without restarting the daemon:
- ✓ `daemon.workers`, `daemon.rate_limit_per_min`, `daemon.debounce_ms`
- ✓ `daemon.full_rebuild_interval_minutes`, `daemon.health_check_port`
- ✓ `analysis.*` settings, `claude.*` settings
- ✗ `memory_root`, `analysis.cache_dir`, `daemon.log_file` (require restart)

#### Running as a Service

For production use, run the daemon as a system service that starts automatically and restarts on failure. The application provides commands to generate service configuration files for systemd (Linux) and launchd (macOS).

**Benefits of running as a service:**
- Automatic start on system boot or user login
- Automatic restart if daemon crashes
- Centralized log management
- Health monitoring and status checking
- No manual terminal session required

##### systemd (Linux)

Generate a systemd unit file:

```bash
agentic-memorizer daemon systemctl
```

This command outputs a complete systemd unit file. To install:

**Option A: User Service (Recommended - No root required)**

```bash
# Create directory
mkdir -p ~/.config/systemd/user

# Generate and save unit file
agentic-memorizer daemon systemctl > ~/.config/systemd/user/agentic-memorizer.service

# Reload systemd
systemctl --user daemon-reload

# Enable autostart
systemctl --user enable agentic-memorizer

# Start service
systemctl --user start agentic-memorizer

# Check status
systemctl --user status agentic-memorizer

# View logs
journalctl --user -u agentic-memorizer -f
```

**Option B: System-Wide Service (Requires root)**

```bash
# Generate and save unit file (requires sudo)
agentic-memorizer daemon systemctl | sudo tee /etc/systemd/system/agentic-memorizer.service

# Reload systemd
sudo systemctl daemon-reload

# Enable autostart
sudo systemctl enable agentic-memorizer

# Start service
sudo systemctl start agentic-memorizer

# Check status
systemctl status agentic-memorizer

# View logs
journalctl -u agentic-memorizer -f
```

**Managing the service:**

```bash
# Stop service
systemctl --user stop agentic-memorizer

# Restart service
systemctl --user restart agentic-memorizer

# Disable autostart
systemctl --user disable agentic-memorizer

# Remove service
systemctl --user stop agentic-memorizer
systemctl --user disable agentic-memorizer
rm ~/.config/systemd/user/agentic-memorizer.service
systemctl --user daemon-reload
```

##### launchd (macOS)

Generate a launchd property list:

```bash
agentic-memorizer daemon launchctl
```

This command outputs a complete launchd plist file. To install:

```bash
# Create directory
mkdir -p ~/Library/LaunchAgents

# Generate and save plist
agentic-memorizer daemon launchctl > ~/Library/LaunchAgents/com.$(whoami).agentic-memorizer.plist

# Load service
launchctl load ~/Library/LaunchAgents/com.$(whoami).agentic-memorizer.plist

# Start service (if not running)
launchctl start com.$(whoami).agentic-memorizer

# Check if running
launchctl list | grep agentic-memorizer
```

**Managing the service:**

```bash
# Stop service
launchctl stop com.$(whoami).agentic-memorizer

# Restart service
launchctl stop com.$(whoami).agentic-memorizer
launchctl start com.$(whoami).agentic-memorizer

# Disable autostart (unload)
launchctl unload ~/Library/LaunchAgents/com.$(whoami).agentic-memorizer.plist

# Remove service
launchctl unload ~/Library/LaunchAgents/com.$(whoami).agentic-memorizer.plist
rm ~/Library/LaunchAgents/com.$(whoami).agentic-memorizer.plist
```

**View logs:**

```bash
# Tail daemon log
tail -f ~/.agentic-memorizer/daemon.log

# Check Console.app for system messages (macOS)
open /Applications/Utilities/Console.app
```

##### Supervisor (Cross-Platform Alternative)

For development environments or servers without systemd, use Supervisor:

**Install Supervisor:**

```bash
# Ubuntu/Debian
sudo apt-get install supervisor

# macOS
brew install supervisor

# Or via pip
pip install supervisor
```

**Configure:**

Create `/etc/supervisor/conf.d/agentic-memorizer.conf`:

```ini
[program:agentic-memorizer]
command=/home/youruser/.local/bin/agentic-memorizer daemon start
directory=/home/youruser
autostart=true
autorestart=true
startretries=3
user=youruser
redirect_stderr=true
stdout_logfile=/var/log/agentic-memorizer/daemon.log
stdout_logfile_maxbytes=10MB
stdout_logfile_backups=3
environment=HOME="/home/youruser"
```

Replace `youruser` with your username and adjust paths as needed.

**Manage with supervisorctl:**

```bash
# Reload config
sudo supervisorctl reread
sudo supervisorctl update

# Start service
sudo supervisorctl start agentic-memorizer

# Check status
sudo supervisorctl status agentic-memorizer

# Stop service
sudo supervisorctl stop agentic-memorizer

# Restart service
sudo supervisorctl restart agentic-memorizer

# View logs
sudo supervisorctl tail -f agentic-memorizer
```

### Upgrading

When upgrading to a new version, the upgrade process depends on how you're running the daemon.

#### Upgrading with Service Managers (Recommended)

**systemd (Linux):**

```bash
# Stop service
systemctl --user stop agentic-memorizer

# Upgrade binary
go install github.com/leefowlercu/agentic-memorizer@latest
# OR: cd /path/to/repo && make install-release

# Start service
systemctl --user start agentic-memorizer

# Verify
systemctl --user status agentic-memorizer
agentic-memorizer version
```

**Or use restart for one command:**
```bash
# Stop and upgrade (Makefile handles daemon stop)
make install-release

# Restart service
systemctl --user restart agentic-memorizer
```

**launchd (macOS):**

```bash
# Stop service
launchctl stop com.$(whoami).agentic-memorizer

# Upgrade binary
go install github.com/leefowlercu/agentic-memorizer@latest
# OR: cd /path/to/repo && make install-release

# Start service
launchctl start com.$(whoami).agentic-memorizer

# Verify
launchctl list | grep agentic-memorizer
agentic-memorizer version
```

**Supervisor:**

```bash
# Stop service
sudo supervisorctl stop agentic-memorizer

# Upgrade binary
go install github.com/leefowlercu/agentic-memorizer@latest

# Start service
sudo supervisorctl start agentic-memorizer

# Verify
sudo supervisorctl status agentic-memorizer
```

#### Upgrading Manual Daemon

If running daemon manually (not as service):

```bash
# Stop daemon
agentic-memorizer daemon stop

# Upgrade
go install github.com/leefowlercu/agentic-memorizer@latest
# OR: cd /path/to/repo && make install-release

# Start daemon
agentic-memorizer daemon start

# Verify
agentic-memorizer version
```

**Note:** The Makefile install targets automatically stop the daemon before replacing the binary:
```bash
# These commands handle daemon shutdown automatically
make install          # Development build
make install-release  # Production build with version info
```

#### Service File Updates

Service files typically **do not need to be regenerated** when upgrading unless:
- The binary path changed
- New configuration options require service file changes
- Release notes explicitly mention service file updates

Service files reference the binary by path, not version:
```ini
ExecStart=/home/user/.local/bin/agentic-memorizer daemon start
```

The service manager automatically uses whatever binary exists at that path after upgrade.

#### Why Service Managers Handle Upgrades Better

**Manual daemon process:**
- ✗ Must manually stop before upgrade
- ✗ Must manually restart after upgrade
- ✗ On macOS, replacing running binary triggers security warnings
- ✗ Old process may continue running from deleted inode

**Service managers:**
- ✓ Orchestrated shutdown and restart
- ✓ No security warnings
- ✓ One-command upgrade with restart
- ✓ Rollback capability if new version fails
- ✓ Health monitoring during upgrade

#### Health Monitoring

Enable health checks for monitoring:

```yaml
daemon:
  health_check_port: 8080
```

Then check health at: `http://localhost:8080`

Response includes uptime, files processed, API calls, errors, and build status.

#### Troubleshooting

**Check daemon status:**
```bash
./agentic-memorizer daemon status
```

**Common issues:**

1. **Daemon won't start - "daemon already running"**
   - Check if daemon is actually running: `./agentic-memorizer daemon status`
   - If not running but PID file exists: `rm ~/.agentic-memorizer/daemon.pid`
   - Try starting again

2. **Daemon crashes or exits immediately**
   - Check logs: `tail -f ~/.agentic-memorizer/daemon.log`
   - Verify config file: `cat ~/.agentic-memorizer/config.yaml`
   - Ensure Claude API key is set (in config or `ANTHROPIC_API_KEY` env var)
   - Check file permissions on cache directory

3. **Index not updating after file changes**
   - Verify daemon is running: `./agentic-memorizer daemon status`
   - Check watcher is active in status output
   - Review daemon logs for file watcher errors
   - Ensure files aren't in skipped directories (`.cache`, `.git`)

4. **High API usage**
   - Reduce workers: `daemon.workers: 1` in config
   - Lower rate limit: `daemon.rate_limit_per_min: 10`
   - Increase rebuild interval: `daemon.full_rebuild_interval_minutes: 120`
   - Add files to skip list: `analysis.skip_files` in config

5. **Index corruption after crash**
   - Daemon automatically loads last good index on startup
   - Force rebuild: `./agentic-memorizer daemon stop && ./agentic-memorizer daemon start`
   - If still corrupted: `rm ~/.agentic-memorizer/index.json` and restart

6. **Service won't start (macOS/Linux)**
   - **macOS**: Check Console.app for launchd errors
   - **Linux**: Check systemd logs: `journalctl -u agentic-memorizer.service -n 50`
   - Verify binary path in service config matches installation location
   - Check user permissions on config and cache directories

**Debug logging:**
```yaml
daemon:
  log_level: debug
```

### Adding Files to Memory

Simply add files to `~/.agentic-memorizer/memory/` (or the directory you've configured as the `memory_root` in `config.yaml`):

```bash
# Organize however you like
~/.agentic-memorizer/memory/
├── documents/
│   └── project-plan.md
├── presentations/
│   └── quarterly-review.pptx
└── images/
    └── architecture-diagram.png
```

On your next Claude Code session, these files will be automatically analyzed and indexed.

### Manual Testing

View the precomputed index:

```bash
# Start daemon if not already running
agentic-memorizer daemon start

# In another terminal, read the index
agentic-memorizer read
```

This outputs the index (XML by default) that AI agents receive. The daemon must be running (or have completed at least one indexing cycle) for the index file to exist.

### CLI Usage

**Commands:**

```bash
# Initialize config and memory directory
agentic-memorizer initialize [flags]

# Manage background daemon
agentic-memorizer daemon start
agentic-memorizer daemon stop
agentic-memorizer daemon status
agentic-memorizer daemon systemctl      # Generate systemd unit file
agentic-memorizer daemon launchctl      # Generate launchd plist

# Read precomputed index (for SessionStart hooks)
agentic-memorizer read [flags]

# Manage agent framework integrations
agentic-memorizer integrations list
agentic-memorizer integrations detect
agentic-memorizer integrations setup <integration-name>
agentic-memorizer integrations remove <integration-name>
agentic-memorizer integrations validate

# Manage configuration
agentic-memorizer config validate
agentic-memorizer config reload

# Get help
agentic-memorizer --help
agentic-memorizer initialize --help
agentic-memorizer daemon --help
agentic-memorizer read --help
agentic-memorizer integrations --help
agentic-memorizer config --help
```

**Common Flags:**

```bash
# Read command flags
--format <xml|markdown|json>        # Output format
--integration <name>                # Format for specific integration (claude-code-hook, etc)

# Init command flags
--memory-root <dir>                 # Custom memory directory
--cache-dir <dir>                   # Custom cache directory
--force                             # Overwrite existing config
--setup-integrations                # Configure agent framework integrations
```

**Examples:**

```bash
# Initialize
agentic-memorizer initialize

# Read index (XML format)
agentic-memorizer read

# Read index (Markdown format)
agentic-memorizer read --format markdown

# Read index (JSON format)
agentic-memorizer read --format json

# Read with Claude Code hook integration (SessionStart)
agentic-memorizer read --format xml --integration claude-code-hook

# Note: MCP integration uses tools, not read command

# Start daemon
agentic-memorizer daemon start

# Check daemon status
agentic-memorizer daemon status

# Force rebuild index
agentic-memorizer daemon rebuild

# List available integrations
agentic-memorizer integrations list

# Detect installed agent frameworks
agentic-memorizer integrations detect

# Setup Claude Code SessionStart hooks
agentic-memorizer integrations setup claude-code-hook

# Setup Claude Code MCP server
agentic-memorizer integrations setup claude-code-mcp

# Remove integrations
agentic-memorizer integrations remove claude-code-hook
agentic-memorizer integrations remove claude-code-mcp

# Validate integration configurations
agentic-memorizer integrations validate
```

### Controlling Semantic Analysis

Semantic analysis can be enabled or disabled in `config.yaml`:

```yaml
analysis:
  enable: true  # Set to false to skip Claude API calls
```

When disabled, the daemon will only extract file metadata without semantic analysis.

## Supported File Types

### Directly Readable by Claude Code
- Markdown (`.md`)
- Text files (`.txt`)
- JSON/YAML (`.json`, `.yaml`)
- Images (`.png`, `.jpg`, `.gif`, `.webp`)
- Code files (`.go`, `.py`, `.js`, `.ts`, etc.)
- Transcripts (`.vtt`, `.srt`)

### Requires Extraction
- Word documents (`.docx`)
- PowerPoint (`.pptx`)
- PDFs (`.pdf`)

The index tells your AI agent which method to use for each file.

## Configuration Options

See `config.yaml.example` for all options:

- **API Settings**: Model, tokens, timeout, vision enable
- **Analysis**: Enable/disable, file size limits, file exclusions, cache directory
- **Daemon**: Worker count, rate limits, rebuild intervals, health check, debounce timing
- **Output**: Format (xml/markdown/json), recent activity days
- **MCP**: Log file path, log level
- **Integrations**: Tracks enabled integrations (auto-managed by CLI)

### File Exclusions

The indexer automatically excludes:
- Hidden files and directories (starting with `.`)
- The `.cache/` directory (where analyses are cached)
- The `agentic-memorizer` binary itself (if located in the memory directory)

You can exclude additional files by name in `config.yaml`:

```yaml
analysis:
  skip_files:
    - agentic-memorizer  # Default
    - my-private-notes.md
    - temp-file.txt
```

Files in the skip list are completely ignored during indexing and won't appear in the generated index.

### Environment Variables

#### MEMORIZER_APP_DIR

By default, configuration and data files are stored in `~/.agentic-memorizer/`. You can customize this location by setting the `MEMORIZER_APP_DIR` environment variable:

```bash
# Use a custom app directory
export MEMORIZER_APP_DIR=/path/to/custom/location
agentic-memorizer initialize

# Or for a single command
MEMORIZER_APP_DIR=/tmp/test-instance agentic-memorizer daemon start
```

Files stored in the app directory:
- `config.yaml` - Configuration file
- `index.json` - Precomputed index
- `daemon.pid` - Daemon process ID
- `daemon.log` - Daemon logs (if configured)

**Use cases:**
- **Testing**: Run isolated test instances without affecting your main instance
- **Multi-instance**: Run multiple independent instances for different projects
- **Containers**: Use custom paths in Docker or other containerized environments
- **CI/CD**: Isolate build/test environments

**Note**: The memory directory and cache directory locations are still controlled by `config.yaml` settings, not `MEMORIZER_APP_DIR`. Only the application's own files (config, index, PID, logs) use the app directory.

### Output Formats

The memorizer supports three output formats:

#### XML (Default)

Highly structured XML following Anthropic's recommendations for Claude [prompt engineering](https://docs.claude.com/en/docs/build-with-claude/prompt-engineering/use-xml-tags):

```bash
agentic-memorizer read
# or explicitly:
agentic-memorizer read --format xml
```

#### Markdown

Human-readable markdown, formatted for direct viewing:

```bash
agentic-memorizer read --format markdown
```

#### JSON Format

Pretty-printed JSON representation of the index:

```bash
agentic-memorizer read --format json
```

#### Integration-Specific Output

Use the `--integration` flag to format output for specific agent frameworks. This wraps the index in the appropriate structure for that framework:

```bash
# Claude Code hook integration (SessionStart injection)
agentic-memorizer read --format xml --integration claude-code-hook

# Can also use markdown or json formats
agentic-memorizer read --format markdown --integration claude-code-hook

# Note: MCP integration doesn't use read - uses tools instead
```

**Claude Code Integration Output Structure:**

The Claude Code integration wraps the formatted index in a SessionStart hook JSON envelope:

```json
{
  "continue": true,
  "suppressOutput": true,
  "systemMessage": "Memory index updated: 15 files (5 documents, 3 images, 2 presentations, 5 code files), 2.3 MB total — 12 cached, 3 analyzed",
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "<memory_index>...</memory_index>"
  }
}
```

- **continue**: Always `true` - allows session to proceed
- **suppressOutput**: Always `true` - keeps verbose index out of transcript
- **systemMessage**: Concise summary visible to user in UI
- **hookSpecificOutput**: Contains the full index (XML, Markdown, or JSON) in `additionalContext`, which Claude Code adds to the context window

**More Info**

[Claude Hook JSON Common Fields](https://docs.claude.com/en/docs/claude-code/hooks#common-json-fields)

[Claude SessionStart Hook Fields](https://docs.claude.com/en/docs/claude-code/hooks#sessionstart-decision-control)

## Example Outputs

Here are examples of what the memory index looks like in each format:

### XML Output Example

Abbreviated example showing structure (actual output includes all files):

```xml
<memory_index>
  <metadata>
    <generated>2025-10-05T14:30:22-04:00</generated>
    <file_count>3</file_count>
    <total_size_human>2.0 MB</total_size_human>
    <root_path>/Users/username/.agentic-memorizer/memory</root_path>
    <cache_stats>
      <cached_files>2</cached_files>
      <analyzed_files>1</analyzed_files>
    </cache_stats>
  </metadata>

  <recent_activity days="7">
    <file><path>documents/api-design-guide.md</path><modified>2025-10-04</modified></file>
  </recent_activity>

  <categories>
    <category name="documents" count="1" total_size="45.2 KB">
      <file>
        <name>api-design-guide.md</name>
        <path>/Users/username/.agentic-memorizer/memory/documents/api-design-guide.md</path>
        <modified>2025-10-04</modified>
        <size_human>45.2 KB</size_human>
        <file_type>markdown</file_type>
        <readable>true</readable>
        <metadata>
          <word_count>4520</word_count>
          <section_count>8</section_count>
        </metadata>
        <semantic>
          <summary>Comprehensive API design guidelines covering RESTful principles, versioning strategies, authentication patterns, and best practices for building scalable microservices.</summary>
          <document_type>technical-guide</document_type>
          <topics>
            <topic>RESTful API design principles and conventions</topic>
            <topic>API versioning and backward compatibility</topic>
            <!-- Additional topics -->
          </topics>
          <tags>
            <tag>api-design</tag>
            <tag>rest</tag>
            <tag>microservices</tag>
          </tags>
        </semantic>
      </file>
      <!-- Additional files in this category -->
    </category>
    <!-- Additional categories: code, images, presentations, etc. -->
  </categories>

  <usage_guide>
    <direct_read_extensions>md, txt, json, yaml, vtt, go, py, js, ts, png, jpg</direct_read_extensions>
    <direct_read_tool>Read tool</direct_read_tool>
    <extraction_required_extensions>docx, pptx, pdf</extraction_required_extensions>
    <extraction_required_tool>Bash + conversion tools</extraction_required_tool>
  </usage_guide>
</memory_index>
```

### Markdown Output Example

Abbreviated example showing structure (actual output includes all files):

```markdown
# Claude Code Agentic Memory Index
📅 Generated: 2025-10-05 14:30:24
📁 Files: 3 | 💾 Total Size: 2.0 MB
📂 Root: /Users/username/.agentic-memorizer/memory

## 🕐 Recent Activity (Last 7 Days)
- 2025-10-04: `documents/api-design-guide.md`

---

## 📄 Documents (1 files, 45.2 KB)

### api-design-guide.md
**Path**: `/Users/username/.agentic-memorizer/memory/documents/api-design-guide.md`
**Modified**: 2025-10-04 | **Size**: 45.2 KB | **Words**: 4,520 | **Sections**: 8
**Type**: Markdown • Technical-Guide

**Summary**: Comprehensive API design guidelines covering RESTful principles, versioning strategies, authentication patterns, and best practices for building scalable microservices.

**Topics**: RESTful API design principles, API versioning, Authentication patterns, Rate limiting
**Tags**: `api-design` `rest` `microservices` `best-practices`

✓ Use Read tool directly

## 💻 Code (1 files, 12.8 KB)
[... similar structure for code files ...]

## 🖼️ Images (1 files, 1.4 MB)
[... similar structure for images ...]

## Usage Guide
**Reading Files**:
- ✅ **Direct**: Markdown, text, VTT, JSON, YAML, images, code → Use Read tool
- ⚠️ **Extraction needed**: DOCX, PPTX, PDF → Use Bash + conversion tools
```

## Development

### Project Structure

```
agentic-memorizer/
├── main.go                   # Main entry point
├── LICENSE                   # MIT License
├── .goreleaser.yaml          # GoReleaser configuration for multi-platform releases
├── cmd/
│   ├── root.go               # Root command
│   ├── initialize/           # Initialization command
│   │   └── initialize.go
│   ├── daemon/               # Daemon management commands
│   │   ├── daemon.go         # Parent daemon command
│   │   └── subcommands/      # Daemon subcommands (8 total)
│   │       ├── start.go
│   │       ├── stop.go
│   │       ├── status.go
│   │       ├── restart.go
│   │       ├── rebuild.go
│   │       ├── logs.go
│   │       ├── systemctl.go  # Generate systemd unit files
│   │       └── launchctl.go  # Generate launchd plist files
│   ├── mcp/                  # MCP server commands
│   │   ├── mcp.go            # Parent mcp command
│   │   └── subcommands/
│   │       └── start.go      # Start MCP server
│   ├── integrations/         # Integration management commands
│   │   ├── integrations.go   # Parent integrations command
│   │   └── subcommands/      # Integration subcommands (6 total)
│   │       ├── list.go
│   │       ├── detect.go
│   │       ├── setup.go
│   │       ├── remove.go
│   │       ├── validate.go
│   │       ├── health.go
│   │       └── helpers.go
│   ├── config/               # Configuration commands
│   │   ├── config.go         # Parent config command
│   │   └── subcommands/      # Config subcommands (2 total)
│   │       ├── validate.go
│   │       └── reload.go
│   ├── read/                 # Read precomputed index
│   │   └── read.go
│   └── version/              # Version command
│       └── version.go
├── internal/
│   ├── config/               # Configuration loading, validation, and hot-reload
│   ├── daemon/               # Background daemon implementation and worker pool
│   ├── index/                # Index management and atomic writes
│   ├── watcher/              # File system watching (fsnotify)
│   ├── walker/               # File system traversal with filtering
│   ├── metadata/             # File metadata extraction (10 category handlers)
│   ├── semantic/             # Claude API integration for semantic analysis
│   ├── cache/                # Content-addressable analysis caching
│   ├── search/               # Semantic search engine (fuzzy, tag, topic, summary)
│   ├── mcp/                  # MCP server implementation
│   │   ├── protocol/         # JSON-RPC 2.0 protocol messages
│   │   └── transport/        # Stdio transport layer
│   ├── integrations/         # Integration framework and adapters
│   │   ├── output/           # Output formatting (XML/Markdown/JSON)
│   │   └── adapters/         # Framework-specific adapters
│   │       └── claude/       # Hook and MCP adapters for Claude Code
│   └── version/              # Version information and embedding
│       ├── VERSION           # Semantic version file (embedded)
│       ├── version.go        # Version getters with buildinfo fallback
│       └── version_test.go
├── scripts/                  # Release automation scripts
│   ├── bump-version.sh       # Semantic version bumping
│   └── prepare-release.sh    # Release preparation and automation
├── pkg/types/                # Shared types and data structures
├── docs/subsystems/          # Comprehensive subsystem documentation
└── testdata/                 # Test files
```

### Building and Testing

```bash
# Building
make build             # Build binary (development)
make build-release     # Build with version information
make install           # Build and install to ~/.local/bin
make install-release   # Install release build with version info

# Testing
make test              # Run unit tests only (fast, no external dependencies)
make test-integration  # Run integration tests only (requires daemon, slower)
make test-all          # Run all tests (unit + integration)
make test-race         # Run tests with race detector (important for concurrent code)
make coverage          # Generate coverage report
make coverage-html     # Generate and view HTML coverage report

# Code Quality
make fmt               # Format code with gofmt
make vet               # Run go vet
make lint              # Run golangci-lint (if installed)
make check             # Run all checks (fmt, vet, test-all)

# Utilities
make clean             # Remove build artifacts
make clean-cache       # Remove cache files only
make deps              # Update dependencies
```

**Test Types:**
- **Unit tests** (`make test`) - Fast, no external dependencies
- **Integration tests** (`make test-integration`) - Full daemon lifecycle, requires `-tags=integration`
- Integration tests use `MEMORIZER_APP_DIR` for isolated environments
- Test data in `testdata/` directory

### Adding New File Type Handlers

1. Create handler in `internal/metadata/`
2. Implement `FileHandler` interface:
   ```go
   type FileHandler interface {
       Extract(path string, info os.FileInfo) (*types.FileMetadata, error)
       CanHandle(ext string) bool
   }
   ```
3. Register in `internal/metadata/extractor.go`

See existing handlers for examples.

## Limitations & Known Issues

### Current Limitations

- **API Costs**: Semantic analysis uses Claude API calls (costs apply)
  - Mitigated by caching (only analyzes new/modified files)
  - Can disable semantic analysis in config: `analysis.enable: false` for metadata-only mode

- **File Size Limit**: Default 10MB max for semantic analysis
  - Configurable via `analysis.max_file_size` in config
  - Larger files are indexed with metadata only

- **Internet Required**: Needs connection for Claude API calls
  - Cached analyses work offline
  - Metadata extraction works offline

- **File Format Support**: Limited to formats with extractors
  - Common formats covered (docs, images, code, etc.)
  - Binary files without extractors get basic metadata only

### Known Issues

- Some PPTX files with complex formatting may have incomplete extraction
- PDF page count detection is basic (stub implementation)
- Very large directories (1000+ files) may take time on first run

## Troubleshooting

### "API key is required" error

Set your API key in config or environment:

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

### Index not appearing in AI agent

1. Verify daemon is running: `agentic-memorizer daemon status`
2. Check your framework's integration configuration:
   - **Claude Code**: Check `~/.claude/settings.json` has SessionStart hooks configured
   - **Other frameworks**: Verify you followed the setup instructions from `agentic-memorizer integrations setup <framework-name>`
3. Verify binary path is correct (`~/.local/bin/agentic-memorizer` or `~/go/bin/agentic-memorizer`)
4. Test manually: `agentic-memorizer read`
5. Check your AI agent's output/logs for errors

### Config reload not applying changes

1. Some settings require daemon restart (see Daemon Configuration section)
2. Validate config syntax: `agentic-memorizer config validate`
3. Check daemon logs: `tail -f ~/.agentic-memorizer/daemon.log`
4. If reload fails, restart: `agentic-memorizer daemon restart`

### Reducing resource usage

When indexing many files:
- Reduce daemon workers: `daemon.workers: 1` in config
- Lower rate limit: `daemon.rate_limit_per_min: 10` in config
- Disable semantic analysis temporarily: `analysis.enable: false` in config

### Cache issues

Clear cache to force re-analysis of all files:

```bash
rm -rf ~/.agentic-memorizer/.cache
agentic-memorizer daemon restart
```

## Contributing

Contributions are welcome! To contribute:

1. **Report Issues**: Open an issue on GitHub describing the problem
2. **Suggest Features**: Propose new features via GitHub issues
3. **Submit Pull Requests**: Fork the repo, make changes, and submit a PR
4. **Follow Standards**: Use Go conventions, add tests, update docs

See existing code for examples and patterns.

## License

MIT License - see [LICENSE](LICENSE) file for details.
