# Agentic Memorizer

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25.1-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/leefowlercu/agentic-memorizer)](https://goreportcard.com/report/github.com/leefowlercu/agentic-memorizer)
[![GitHub Release](https://img.shields.io/github/v/release/leefowlercu/agentic-memorizer?include_prereleases)](https://github.com/leefowlercu/agentic-memorizer/releases)
[![CI](https://github.com/leefowlercu/agentic-memorizer/workflows/CI/badge.svg)](https://github.com/leefowlercu/agentic-memorizer/actions/workflows/ci.yml)
[![E2E Tests](https://github.com/leefowlercu/agentic-memorizer/workflows/E2E%20Tests/badge.svg)](https://github.com/leefowlercu/agentic-memorizer/actions/workflows/e2e-tests.yml)

A framework-agnostic AI agent memory system that provides automatic awareness and understanding of files in your memory directory through AI-powered semantic analysis, plus user-defined facts that inject persistent context into every conversation. Features native automatic integration for Claude Code (hooks + MCP), Gemini CLI (hooks + MCP), and OpenAI Codex CLI (MCP).

**Current Version**: v0.13.0 ([CHANGELOG.md](CHANGELOG.md))

## Table of Contents

- [Overview](#overview)
  - [How It Works](#how-it-works)
  - [Key Capabilities](#key-capabilities)
  - [Facts Management](#facts-management)
- [Why Use This?](#why-use-this)
- [Supported AI Agent Frameworks](#supported-ai-agent-frameworks)
  - [Automatic Integration](#automatic-integration)
  - [Framework Comparison](#framework-comparison)
- [Architecture](#architecture)
- [FalkorDB Knowledge Graph](#falkordb-knowledge-graph)
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
  - [Gemini CLI SessionStart Hook Integration (Automatic)](#gemini-cli-sessionstart-hook-integration-automatic)
  - [Gemini CLI MCP Integration (Automatic)](#gemini-cli-mcp-integration-automatic)
  - [OpenAI Codex CLI Integration (Automatic)](#openai-codex-cli-integration-automatic)
- [Managing Integrations](#managing-integrations)
- [Usage](#usage)
  - [Background Daemon (Required)](#background-daemon-required)
  - [Running as a Service](#running-as-a-service)
  - [Upgrading](#upgrading)
  - [Adding Files to Memory](#adding-files-to-memory-1)
  - [Using the MCP Server](#using-the-mcp-server)
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

Works seamlessly with Claude Code, Gemini CLI, and Codex CLI with automatic setup.

### How It Works

A background daemon continuously watches your designated memory directory (`~/.memorizer/memory/` by default), automatically discovering and analyzing files as they're added or modified. Each file is processed to extract metadata (word counts, dimensions, page counts, etc.) and—using configurable AI providers (Claude, OpenAI, or Gemini)—semantically analyzed to understand its content, purpose, and key topics. 

When you launch your AI agent, context is automatically injected via hooks:
- **File Index**: SessionStart hooks load the precomputed file index at session start
- **User Facts**: UserPromptSubmit (Claude) / BeforeAgent (Gemini) hooks inject user-defined facts before each prompt
- **Other frameworks**: Configure your agent to run `memorizer read files` and `memorizer read facts` on startup

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
- Vision analysis for images using multimodal capabilities (Claude, OpenAI, Gemini)

**Efficiency:**
- Background daemon handles all processing asynchronously
- Smart caching only re-analyzes changed files
- Precomputed index enables quick Claude Code startup
- Minimal API usage—only new/modified files are analyzed

**Wide Format Support:**
- **Direct reading**: Markdown, text, JSON/YAML, code files, images, VTT transcripts
- **Extraction supported**: Word documents (DOCX), PowerPoint (PPTX), PDFs
- Automatic metadata extraction for all file types

**Facts Management:**
- Store persistent facts that inject into every AI conversation
- User-defined context (preferences, project info, reminders)
- Facts delivered via per-prompt hooks (UserPromptSubmit/BeforeAgent)
- Simple CRUD operations: remember, read, forget
- Up to 50 facts, 10-500 characters each

**Integration:**
- Framework-agnostic with native support for multiple AI agent frameworks
- Automatic setup for Claude Code, Gemini CLI, and Codex CLI
- Dual-hook architecture: files at session start, facts before each prompt
- Configurable output formats (XML, Markdown, JSON)
- Integration management commands for detection, setup, validation, and health checks
- Optional health monitoring and logging

### Facts Management

Facts are user-defined pieces of context that persist across AI sessions and inject automatically into every conversation. Unlike files (which provide document awareness), facts provide personalized context about you, your projects, and your preferences.

**Example facts:**
- "I prefer TypeScript over JavaScript for new projects"
- "The current sprint focuses on authentication improvements"
- "Always use conventional commit format for commit messages"
- "I work on a MacBook Pro M2 running macOS Sonoma"

**How facts are delivered:**

Facts are injected before each prompt via framework-specific hooks:
- **Claude Code**: UserPromptSubmit hook
- **Gemini CLI**: BeforeAgent hook

This ensures your AI agent always has your context, even in long sessions where the SessionStart context may be summarized.

**Managing facts:**

```bash
# Add a fact
memorizer remember fact "I prefer dark mode in all applications"

# View all facts
memorizer read facts

# Remove a fact by ID
memorizer forget fact <fact-id>
```

Facts are stored in FalkorDB and support multiple output formats (XML, Markdown, JSON).

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

Agentic Memorizer integrates with multiple AI agent frameworks, providing automatic setup for all supported frameworks.

### Automatic Integration

**Claude Code** - Full automatic integration with one-command setup
- Automatic framework detection and configuration
- One-command setup: `memorizer integrations setup claude-code-hook`
- SessionStart hook configuration with all matchers (startup, resume, clear, compact)
- Default XML output with JSON envelope wrapping for proper hook formatting
- Full lifecycle management (setup, update, remove, validate)

**Gemini CLI** - Full automatic integration with SessionStart hooks and MCP server
- Automatic framework detection and configuration
- SessionStart hook setup: `memorizer integrations setup gemini-cli-hook`
- MCP server setup: `memorizer integrations setup gemini-cli-mcp`
- Hook configuration with matchers (startup, resume, clear)
- MCP server provides five on-demand tools: `search_files`, `get_file_metadata`, `list_recent_files`, `get_related_files`, `search_entities`
- Full lifecycle management (setup, update, remove, validate)
- Works with both user and project-level Gemini CLI configurations

**OpenAI Codex CLI** - MCP server integration with automatic setup
- Automatic framework detection and configuration
- One-command setup: `memorizer integrations setup codex-cli-mcp`
- MCP server configuration in `~/.codex/config.toml` (TOML format)
- Provides five on-demand tools: `search_files`, `get_file_metadata`, `list_recent_files`, `get_related_files`, `search_entities`
- Full lifecycle management (setup, update, remove, validate)
- Verification via `/mcp` command in Codex TUI

### Framework Comparison

| Feature | Claude Code (Hook) | Claude Code (MCP) | Gemini CLI (Hook) | Gemini CLI (MCP) | Codex CLI (MCP) |
|---------|-------------------|-------------------|-------------------|------------------|-----------------|
| **Setup Type** | Automatic | Automatic | Automatic | Automatic | Automatic |
| **Delivery** | SessionStart injection | On-demand tools | SessionStart injection | On-demand tools | On-demand tools |
| **Output Format** | XML (JSON-wrapped) | N/A (tool-based) | XML (JSON-wrapped) | N/A (tool-based) | N/A (tool-based) |
| **Best For** | Complete awareness | Large directories | Complete awareness | Large directories | Large directories |
| **Validation** | Automatic | Automatic | Automatic | Automatic | Automatic |

## Architecture

**Three-Phase Processing Pipeline:**

1. **Metadata Extraction** (`internal/metadata/`) - Fast, deterministic extraction using specialized handlers for 9 file type categories
2. **Semantic Analysis** (`internal/semantic/`) - AI-powered content understanding with multi-provider support (Claude, OpenAI, Gemini) and entity extraction
3. **Knowledge Graph Storage** (`internal/graph/`) - FalkorDB graph database for relationships and semantic search

**Background Daemon** (`internal/daemon/`):
- **Walker** (`internal/walker/`) - Full directory scans during rebuilds
- **File Watcher** (`internal/watcher/`) - Real-time monitoring with fsnotify
- **Worker Pool** - Parallel processing with rate limiting (default 3 workers, provider-specific rate limits: Claude 20/min, OpenAI 60/min, Gemini 100/min)
- **HTTP API** (`internal/daemon/api/`) - RESTful endpoints and SSE for real-time updates:
  - `GET /health` - Health check with metrics
  - `GET /sse` - Server-Sent Events stream
  - `GET /api/v1/index` - Full memory index
  - `POST /api/v1/search` - Semantic search
  - `GET /api/v1/files/{path}` - File metadata
  - `GET /api/v1/files/recent` - Recent files
  - `GET /api/v1/files/related` - Related files
  - `GET /api/v1/entities/search` - Entity search
  - `POST /api/v1/rebuild` - Trigger rebuild

**Knowledge Graph** (`internal/graph/`):
- FalkorDB (Redis-compatible graph database)
- Node types: File, Tag, Topic, Entity, Category, Directory, Fact
- Relationship types: HAS_TAG, COVERS_TOPIC, MENTIONS, IN_CATEGORY, REFERENCES, SIMILAR_TO, IN_DIRECTORY, PARENT_OF
- Vector embeddings for semantic similarity (optional, requires OpenAI API)

**Facts Storage** (`internal/graph/facts.go`):
- CRUD operations for user-defined facts
- Up to 50 facts, 10-500 characters each
- Facts injected via UserPromptSubmit (Claude) / BeforeAgent (Gemini) hooks

**Semantic Search** (`internal/search/`):
- Graph-powered Cypher queries
- Full-text search on summaries
- Entity-based file discovery
- Related file traversal
- Tag and topic filtering

**Integration Framework** (`internal/integrations/`):
- Adapter pattern for Claude Code (hook + MCP), Gemini CLI, Codex CLI
- Independent output processors (XML, Markdown, JSON)

**MCP Server** (`internal/mcp/`):
- JSON-RPC 2.0 stdio transport
- Five tools: `search_files`, `get_file_metadata`, `list_recent_files`, `get_related_files`, `search_entities`
- Connects to daemon HTTP API for graph queries

**Configuration** (`internal/config/`):
- Layered: defaults → YAML → environment variables
- Hot-reload support via `config reload` command

The daemon handles all processing in the background, so AI agent startup remains quick regardless of file count.

## FalkorDB Knowledge Graph

Agentic Memorizer uses FalkorDB as its storage backend, providing a knowledge graph that captures relationships between files, tags, topics, and entities.

### Why a Knowledge Graph?

Unlike flat file indexes, a knowledge graph enables:
- **Relationship Discovery**: Find files that share tags, topics, or mention the same entities
- **Semantic Search**: Query by meaning, not just keywords
- **Entity-Based Navigation**: "Find all files mentioning Terraform" or "What files reference this API?"
- **Related File Suggestions**: Discover files connected through shared concepts

### Starting FalkorDB

FalkorDB runs as a Docker container. Start it before the daemon:

```bash
# Start FalkorDB container (pulls image on first run)
memorizer graph start

# Check status
memorizer graph status

# Stop when done
memorizer graph stop
```

Or use docker-compose:

```bash
docker-compose up -d      # Start FalkorDB
docker-compose down       # Stop FalkorDB
```

### Graph Commands

```bash
# Start FalkorDB Docker container
memorizer graph start [--detach]

# Stop FalkorDB container
memorizer graph stop [--remove]

# Check FalkorDB status and graph statistics
memorizer graph status
```

To rebuild the graph, use `memorizer daemon rebuild [--force]`.

### Graph Configuration

In `~/.memorizer/config.yaml`:

```yaml
graph:
  enabled: true              # Enable graph storage
  host: localhost            # FalkorDB host
  port: 6379                 # FalkorDB port (Redis protocol)
  database: memorizer        # Graph database name
```

### Browser UI

FalkorDB includes a browser-based UI for exploring the graph:

```
http://localhost:3000
```

### Data Persistence

FalkorDB stores data at `/data` inside the container, which is bind-mounted to `~/.memorizer/falkordb/`. Persistence files (`dump.rdb`) appear in this directory after data is saved.

**Clearing graph data:**

```bash
# Option A: Delete persistence files and restart (simplest)
rm -rf ~/.memorizer/falkordb/*
docker restart memorizer-falkordb

# Option B: Clear and rebuild via daemon
memorizer daemon rebuild --force

# Option C: Remove and recreate container
docker stop memorizer-falkordb && docker rm memorizer-falkordb
memorizer graph start
```

### FalkorDB Availability

**IMPORTANT:** The daemon requires FalkorDB to be running at startup and cannot operate without it.

If FalkorDB is unavailable:
- Daemon initialization will fail with "failed to initialize graph"
- You must start FalkorDB before starting the daemon
- Use `memorizer graph start` to launch the FalkorDB container

If an index rebuild fails but existing graph data is present, the daemon will continue running with the existing data (degraded mode). However, this does not apply to FalkorDB connection failures.

## Quick Start

Get up and running quickly with your AI agent:

### 1. Install

```bash
go install github.com/leefowlercu/agentic-memorizer@latest
```

### 2. Set API Key

Set the API key for your chosen provider:

```bash
# Claude (Anthropic)
export ANTHROPIC_API_KEY="your-key-here"

# OpenAI
export OPENAI_API_KEY="your-key-here"

# Google Gemini
export GOOGLE_API_KEY="your-key-here"
```

You only need to set the key for your chosen semantic analysis provider. The initialize wizard will prompt you to select a provider.

### 3. Start FalkorDB

```bash
# Start the knowledge graph database (requires Docker)
memorizer graph start
```

### 4. Choose Your Integration Path

#### Path A: Claude Code (Automatic Integration)

For Claude Code users, automatic setup configures everything for you:

```bash
memorizer initialize --integrations claude-code-hook,claude-code-mcp
```

This will:
- Create config at `~/.memorizer/config.yaml`
- Create memory directory at `~/.memorizer/memory/`
- **Automatically configure Claude Code SessionStart hooks and MCP Server integration** (no manual editing required)

Then start the daemon:
```bash
memorizer daemon start
# OR set up as system service (recommended):
memorizer daemon systemctl  # Linux
memorizer daemon launchctl  # macOS
```

#### Path B: Gemini CLI (Automatic Integration)

For Gemini CLI users, automatic setup works the same way:

```bash
memorizer initialize --integrations gemini-cli-hook,gemini-cli-mcp
```

This will:
- Create config at `~/.memorizer/config.yaml`
- Create memory directory at `~/.memorizer/memory/`
- **Automatically configure Gemini CLI SessionStart hooks and MCP Server integration** (no manual editing required)

Then start the daemon:
```bash
memorizer daemon start
```

#### Path C: Interactive Setup (All Frameworks)

For any framework, use interactive setup which will prompt you to select integrations:

```bash
memorizer initialize
# Interactive TUI will prompt for integration setup
# Or skip integration prompts with: memorizer initialize --skip-integrations
```

This will:
- Create config at `~/.memorizer/config.yaml`
- Create memory directory at `~/.memorizer/memory/`
- **Prompt you to select which integrations to configure** (Claude Code, Gemini CLI, Codex CLI, or skip)
- Automatically configure selected integrations (no manual editing required)

### 5. Add Files to Memory

```bash
# Add any files you want your AI agent to be aware of
cp ~/important-notes.md ~/.memorizer/memory/
cp ~/project-docs/*.pdf ~/.memorizer/memory/documents/
```

The daemon will automatically detect and index these files.

### 6. Start Your AI Agent

**Claude Code:**
```bash
claude
```

**Gemini CLI:**
```bash
gemini
```

Both frameworks automatically load the memory index via SessionStart hooks.

**Other Frameworks:**

Start your agent normally. The memory index will load based on the configuration you set up in step 4.

---

Your AI agent now automatically knows about all files in your memory directory!

For detailed installation options, configuration, and advanced usage, see the sections below.

## Installation

### Prerequisites

- Go 1.25.1 or later
- Docker (for FalkorDB knowledge graph)
- AI provider API key: [Claude](https://console.anthropic.com/), [OpenAI](https://platform.openai.com/), or [Google Gemini](https://aistudio.google.com/)
- An AI agent framework: Claude Code, Gemini CLI, or Codex CLI

### Build and Install

#### Option 1: Using go install (Recommended)

```bash
go install github.com/leefowlercu/agentic-memorizer@latest
```

Then run the initialize command to set up configuration:

```bash
# Interactive setup (prompts for integrations)
memorizer initialize

# Or with flags for automated setup
memorizer initialize --integrations claude-code-hook,claude-code-mcp
```

This creates:
- Config file at `~/.memorizer/config.yaml`
- Memory directory at `~/.memorizer/memory/`
- Cache directory at `~/.memorizer/.cache/` (for semantic analysis cache)
- Graph database (populated by daemon on first run in FalkorDB)

The initialize command can optionally configure AI agent integrations automatically with `--integrations <integration-name>`.

After initialization, start the daemon:
```bash
memorizer daemon start
# OR set up as system service (recommended for production):
memorizer daemon systemctl  # Linux
memorizer daemon launchctl  # macOS
```

#### Option 2: Using Makefile

```bash
# Build and install
make install
```

This will:
- Build the `memorizer` binary with version info from git
- Install it to `~/.local/bin/memorizer`

The build automatically injects version information from git tags and commits, providing accurate version tracking in logs and index files.

### Configuration

Set your provider API key via environment variable (recommended):

```bash
# Claude (default provider)
export ANTHROPIC_API_KEY="your-key-here"

# OpenAI
export OPENAI_API_KEY="your-key-here"

# Google Gemini
export GOOGLE_API_KEY="your-key-here"
```

Or edit `~/.memorizer/config.yaml` to configure provider and credentials:

```yaml
semantic:
  provider: claude  # Options: claude, openai, gemini
  api_key: "your-api-key-here"
  model: claude-sonnet-4-5-20250929  # Provider-specific model
```

**Custom Setup:**

```bash
# Custom memory directory
memorizer initialize --memory-root ~/my-memory

# Custom cache directory
memorizer initialize --cache-dir ~/my-memory/.cache

# Force overwrite existing config
memorizer initialize --force
```

## Integration Setup

### Claude Code Integration (Automatic)

Claude Code enjoys full automatic integration support with one-command setup.

#### Automatic Setup (Recommended)

```bash
memorizer integrations setup claude-code-hook
```

This command automatically:
1. Detects your Claude Code installation (`~/.claude/` directory)
2. Creates or updates `~/.claude/settings.json`
3. Preserves existing settings (won't overwrite other configurations)
4. Adds **two hook types**:
   - **SessionStart hooks** (startup, resume, clear, compact) - load file index at session start
   - **UserPromptSubmit hook** - inject user facts before each prompt
5. Configures the commands:
   - `memorizer read files --format xml --integration claude-code-hook`
   - `memorizer read facts --format xml --integration claude-code-hook`
6. Creates backup at `~/.claude/settings.json.backup`

You can also use the `--integrations` flag during initialization:

```bash
memorizer initialize --integrations
memorizer daemon start
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
            "command": "/path/to/memorizer read files --format xml --integration claude-code-hook"
          }
        ]
      }
      // Repeat for "resume", "clear", and "compact" matchers
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/memorizer read facts --format xml --integration claude-code-hook"
          }
        ]
      }
    ]
  }
}
```

**Note**: SessionStart hooks require matchers (startup, resume, clear, compact). UserPromptSubmit hooks do NOT use matchers - they fire on every prompt submission.

#### Validation

Verify your setup:

```bash
memorizer integrations health
```

#### Removal

Remove the integration:

```bash
memorizer integrations remove claude-code-hook
```

### Claude Code MCP Integration (Automatic)

Claude Code also supports integration via the Model Context Protocol (MCP), providing advanced semantic search capabilities through MCP tools.

#### Automatic Setup (Recommended)

```bash
memorizer integrations setup claude-code-mcp
```

This command automatically:
1. Detects your Claude Code installation (`~/.claude/` directory)
2. Creates or updates `~/.claude.json` (MCP server configuration)
3. Registers the `memorizer` MCP server
4. Configures environment variables (`MEMORIZER_MEMORY_ROOT`)
5. Sets the binary command path
6. Creates backup at `~/.claude.json.backup`

#### MCP Tools and Prompts

The MCP server exposes five tools and three prompt templates for interacting with your memory index. For detailed information on each tool, prompt, and how to use them, see the [Using the MCP Server](#using-the-mcp-server) section.

**Available MCP Tools:**
- `search_files` - Semantic search across indexed files
- `get_file_metadata` - Complete metadata for a specific file
- `list_recent_files` - Recently modified files
- `get_related_files` - Files connected through shared tags/topics/entities (requires FalkorDB)
- `search_entities` - Files mentioning specific entities (requires FalkorDB)

**Available MCP Prompts:**
- `analyze-file` - Generate detailed file analysis
- `search-context` - Build effective search queries
- `explain-summary` - Understand semantic analysis results

#### MCP Configuration

The MCP server has dedicated configuration in `config.yaml`:

```yaml
mcp:
  log_file: ~/.memorizer/mcp.log  # MCP server logs
  log_level: info                          # Log level (debug/info/warn/error)
```

#### Running the MCP Server

The MCP server is automatically started by Claude Code when configured. You can also run it manually for testing:

```bash
# Start MCP server in stdio mode
memorizer mcp start

# Start with debug logging
memorizer mcp start --log-level debug

# View MCP logs
tail -f ~/.memorizer/mcp.log
```

The server communicates via stdin/stdout using JSON-RPC 2.0 protocol.

#### MCP vs Hook Integration

You can use one or both Claude Code integration methods:

- **Hook Integration** (`claude-code-hook`): Automatic context injection via hooks
  - SessionStart hooks inject file index at session start
  - UserPromptSubmit hook injects user facts before each prompt
  - Best for: Always-available context, complete file and facts awareness
  - Trade-off: Larger initial context, all files loaded upfront

- **MCP Server** (`claude-code-mcp`): Provides on-demand tools for semantic search
  - Best for: Large memory directories, selective file discovery
  - Trade-off: Requires explicit tool use, context fetched on demand

Many users enable both for maximum flexibility.

#### Validation

Verify your MCP setup:

```bash
memorizer integrations health
memorizer integrations health
```

#### Removal

Remove the MCP integration:

```bash
memorizer integrations remove claude-code-mcp
```

### Gemini CLI SessionStart Hook Integration (Automatic)

Gemini CLI supports SessionStart hook integration for automatic memory index loading, similar to Claude Code.

#### Automatic Setup (Recommended)

```bash
memorizer integrations setup gemini-cli-hook
```

This command automatically:
1. Detects your Gemini CLI installation (`~/.gemini/` directory)
2. Creates or updates `~/.gemini/settings.json`
3. Preserves existing settings (won't overwrite other configurations)
4. Adds **two hook types**:
   - **SessionStart hooks** (startup, resume, clear) - load file index at session start
   - **BeforeAgent hook** - inject user facts before each agent invocation
5. Configures the commands:
   - `memorizer read files --format xml --integration gemini-cli-hook`
   - `memorizer read facts --format xml --integration gemini-cli-hook`
6. Creates backup at `~/.gemini/settings.json.backup`

You can also use the `--integrations` flag during initialization:

```bash
memorizer initialize --integrations gemini-cli-hook
memorizer daemon start
```

#### Manual Setup (Alternative)

If you prefer manual configuration, add to `~/.gemini/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "name": "memorizer-hook",
            "type": "command",
            "command": "/path/to/memorizer read files --format xml --integration gemini-cli-hook",
            "description": "Load agentic memory index"
          }
        ]
      }
      // Repeat for "resume" and "clear" matchers
    ],
    "BeforeAgent": [
      {
        "hooks": [
          {
            "name": "memorizer-facts-hook",
            "type": "command",
            "command": "/path/to/memorizer read facts --format xml --integration gemini-cli-hook",
            "description": "Load user-defined facts"
          }
        ]
      }
    ]
  }
}
```

**Note**: SessionStart hooks require matchers (`startup`, `resume`, `clear`). BeforeAgent hooks do NOT use matchers - they fire before every agent invocation.

#### Hook Output Format

The Gemini CLI hook integration uses JSON envelopes with hook-specific event names:

**SessionStart (file index):**
```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "<memory_index>...</memory_index>"
  }
}
```

**BeforeAgent (facts):**
```json
{
  "hookSpecificOutput": {
    "hookEventName": "BeforeAgent",
    "additionalContext": "<facts_index>...</facts_index>"
  }
}
```

- **hookEventName**: Indicates hook type ("SessionStart" for files, "BeforeAgent" for facts)
- **additionalContext**: Contains the formatted content (XML, Markdown, or JSON) that Gemini CLI adds to context

#### Validation

Verify your setup:

```bash
memorizer integrations health
```

#### Removal

Remove the integration:

```bash
memorizer integrations remove gemini-cli-hook
```

### OpenAI Codex CLI Integration (Automatic)

OpenAI Codex CLI supports integration via the Model Context Protocol (MCP), providing semantic search and metadata retrieval tools.

#### Setup

One-command automatic setup:

```bash
memorizer integrations setup codex-cli-mcp
```

**What it does:**

1. Detects your Codex CLI installation (`~/.codex/` directory)
2. Creates/updates `~/.codex/config.toml` with MCP server configuration
3. Configures the binary path and memory root environment variable
4. Enables the MCP server by default

**Configuration:**

The setup command adds an MCP server entry to your Codex CLI configuration:

```toml
[mcp_servers.memorizer]
command = "/path/to/memorizer"
args = ["mcp", "start"]
enabled = true

[mcp_servers.memorizer.env]
MEMORIZER_MEMORY_ROOT = "/path/to/memory"
```

**MCP Tools:**

The MCP server exposes five tools to Codex CLI:

- **`search_files`**: Semantic search across indexed files
  - Query by filename, tags, topics, or summary content
  - Returns ranked results with relevance scores
  - Optional category filtering

- **`get_file_metadata`**: Retrieve complete metadata for specific files
  - Full semantic analysis (summary, tags, topics, document type)
  - File metadata (size, type, category, modification date)
  - Confidence scores and analysis results

- **`list_recent_files`**: List recently modified files
  - Configurable time window (1-365 days)
  - Sorted by modification date
  - Optional result limit

- **`get_related_files`**: Find files connected through shared concepts
  - Discovers files with shared tags, topics, or entities
  - Ranks by connection strength
  - Enables knowledge graph traversal

- **`search_entities`**: Search for files mentioning specific entities
  - Find files referencing people, organizations, concepts
  - Supports entity type filtering
  - Returns files with entity mention details

**Verification:**

Run Codex CLI and use the `/mcp` command to verify the integration:

```bash
codex
# In Codex TUI, type:
/mcp
```

You should see `memorizer` listed as an active MCP server.

Alternatively, validate via CLI:

```bash
memorizer integrations health
```

**Removal:**

Remove the MCP integration:

```bash
memorizer integrations remove codex-cli-mcp
```

### Gemini CLI MCP Integration (Automatic)

Gemini CLI supports integration via the Model Context Protocol (MCP), providing semantic search and metadata retrieval tools.

#### Setup

One-command automatic setup:

```bash
memorizer integrations setup gemini-cli-mcp
```

**What it does:**

1. Detects your Gemini CLI installation (`~/.gemini/` directory)
2. Creates/updates `~/.gemini/settings.json` with MCP server configuration
3. Configures the binary path and memory root environment variable
4. Enables the MCP server by default

**Configuration:**

The setup command adds an MCP server entry to your Gemini CLI configuration:

```json
{
  "mcpServers": {
    "memorizer": {
      "command": "/path/to/memorizer",
      "args": ["mcp", "start"],
      "env": {
        "MEMORIZER_MEMORY_ROOT": "/path/to/memory"
      }
    }
  }
}
```

**MCP Tools:**

The MCP server exposes five tools to Gemini CLI:

- **`search_files`**: Semantic search across indexed files
  - Query by filename, tags, topics, or summary content
  - Returns ranked results with relevance scores
  - Optional category filtering

- **`get_file_metadata`**: Retrieve complete metadata for specific files
  - Full semantic analysis (summary, tags, topics, document type)
  - File metadata (size, type, category, modification date)
  - Confidence scores and analysis results

- **`list_recent_files`**: List recently modified files
  - Configurable time window (1-365 days)
  - Sorted by modification date
  - Optional result limit

- **`get_related_files`**: Find files connected through shared concepts
  - Discovers files with shared tags, topics, or entities
  - Ranks by connection strength
  - Enables knowledge graph traversal

- **`search_entities`**: Search for files mentioning specific entities
  - Find files referencing people, organizations, concepts
  - Supports entity type filtering
  - Returns files with entity mention details

**Verification:**

Validate via CLI:

```bash
memorizer integrations health
```

**Removal:**

Remove the MCP integration:

```bash
memorizer integrations remove gemini-cli-mcp
```

## Managing Integrations

The `integrations` command group provides comprehensive tools for managing integrations with various AI agent frameworks.

### List Available Integrations

```bash
memorizer integrations list
```

Shows all registered integrations with their status and configuration:

**Example Output:**

```
✓ claude-code-hook
  Description: Claude Code SessionStart hooks integration
  Version:     2.0.0
  Status:      configured

✓ claude-code-mcp
  Description: Claude Code MCP server integration
  Version:     2.0.0
  Status:      configured

✓ gemini-cli-hook
  Description: Gemini CLI SessionStart hook integration
  Version:     1.0.0
  Status:      configured

✓ gemini-cli-mcp
  Description: Gemini CLI MCP server integration
  Version:     2.0.0
  Status:      configured

✓ codex-cli-mcp
  Description: OpenAI Codex CLI MCP server integration
  Version:     2.0.0
  Status:      configured
```

### Detect Installed Frameworks

Automatically detect which agent frameworks are installed on your system:

```bash
memorizer integrations detect
```

**Example Output:**

```
Detected Frameworks:
  ✓ claude-code-hook (installed at ~/.claude)
```

Checks for framework-specific configuration directories and files.

### Setup an Integration

#### Automatic Setup

All supported integrations offer automatic setup:

```bash
# Claude Code SessionStart hooks
memorizer integrations setup claude-code-hook

# Claude Code MCP server
memorizer integrations setup claude-code-mcp

# Gemini CLI SessionStart hooks
memorizer integrations setup gemini-cli-hook

# Gemini CLI MCP server
memorizer integrations setup gemini-cli-mcp

# Codex CLI MCP server
memorizer integrations setup codex-cli-mcp

# With custom binary path
memorizer integrations setup claude-code-hook --binary-path /custom/path/memorizer
```

Setup automatically:
- Detects the framework's configuration file
- Adds appropriate integration configuration
- Preserves existing settings and creates backup
- Validates the configuration

### Remove an Integration

```bash
memorizer integrations remove claude-code-hook
memorizer integrations remove claude-code-mcp
memorizer integrations remove gemini-cli-hook
memorizer integrations remove gemini-cli-mcp
```

Removes the integration configuration from the framework's settings file. For hook integrations, this:
- Removes SessionStart hooks added by memorizer
- Preserves other hooks and settings
- Creates backup before modification

### Validate Configurations

Check that all configured integrations are properly set up:

```bash
memorizer integrations health
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
memorizer integrations health
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
memorizer daemon start

# OR set up as system service for automatic management (recommended):
memorizer daemon systemctl  # Linux
memorizer daemon launchctl  # macOS
```

**Note**: If you used `initialize --integrations`, the integration is already configured. Otherwise, configure your AI agent framework to call `memorizer read` (see Integration Setup section above).

#### Daemon Commands

```bash
# Start daemon (runs in foreground - press Ctrl+C to stop)
memorizer daemon start

# Check daemon status
memorizer daemon status

# Stop daemon
memorizer daemon stop

# Restart daemon
memorizer daemon restart

# Force immediate rebuild
memorizer daemon rebuild                    # Rebuild index
memorizer daemon rebuild --force            # Clear graph first, then rebuild
memorizer daemon rebuild --clear-stale      # Clear stale cache entries before rebuild

# View daemon logs
memorizer daemon logs              # Last 50 lines
memorizer daemon logs -f           # Follow logs
memorizer daemon logs -n 100       # Last 100 lines

# Hot-reload configuration without daemon restart
memorizer config reload
```

#### How It Works

The daemon:
1. **Watches** your memory directory for file changes using fsnotify
2. **Processes** files in parallel using a worker pool (3 workers by default)
3. **Rate limits** API calls to respect provider limits (default: Claude 20/min, OpenAI 60/min, Gemini 100/min)
4. **Maintains** a precomputed index in FalkorDB with all metadata and semantic analysis
5. **Updates** the index automatically when files are added/modified/deleted
6. **Supports** hot-reload of most configuration settings via `config reload` command

When you run `memorizer read`, it simply loads the precomputed index from FalkorDB instead of analyzing all files.

#### Daemon Configuration

In `~/.memorizer/config.yaml`:

```yaml
daemon:
  enabled: true                          # Enable daemon mode
  debounce_ms: 500                       # Debounce file events (milliseconds)
  workers: 3                             # Parallel worker count
  rate_limit_per_min: 20                 # API rate limit
  full_rebuild_interval_minutes: 60      # Periodic full rebuild interval
  http_port: 0                           # HTTP server for health + SSE (0 = disabled)
  log_file: ~/.memorizer/daemon.log
  log_level: info                        # debug, info, warn, error
```

**Hot-Reloading**: Most settings can be hot-reloaded using `memorizer config reload` without restarting the daemon:
- ✓ `daemon.workers`, `daemon.rate_limit_per_min`, `daemon.debounce_ms`
- ✓ `daemon.full_rebuild_interval_minutes`, `daemon.http_port`
- ✓ `semantic.*` settings (provider, model, vision, rate limits)
- ✗ `memory_root`, `semantic.cache_dir`, `daemon.log_file`, `mcp.log_file` (require restart)

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
memorizer daemon systemctl
```

This command outputs a complete systemd unit file. To install:

**Option A: User Service (Recommended - No root required)**

```bash
# Create directory
mkdir -p ~/.config/systemd/user

# Generate and save unit file
memorizer daemon systemctl > ~/.config/systemd/user/memorizer.service

# Reload systemd
systemctl --user daemon-reload

# Enable autostart
systemctl --user enable memorizer

# Start service
systemctl --user start memorizer

# Check status
systemctl --user status memorizer

# View logs
journalctl --user -u memorizer -f
```

**Option B: System-Wide Service (Requires root)**

```bash
# Generate and save unit file (requires sudo)
memorizer daemon systemctl | sudo tee /etc/systemd/system/memorizer.service

# Reload systemd
sudo systemctl daemon-reload

# Enable autostart
sudo systemctl enable memorizer

# Start service
sudo systemctl start memorizer

# Check status
systemctl status memorizer

# View logs
journalctl -u memorizer -f
```

**Managing the service:**

```bash
# Stop service
systemctl --user stop memorizer

# Restart service
systemctl --user restart memorizer

# Disable autostart
systemctl --user disable memorizer

# Remove service
systemctl --user stop memorizer
systemctl --user disable memorizer
rm ~/.config/systemd/user/memorizer.service
systemctl --user daemon-reload
```

##### launchd (macOS)

Generate a launchd property list:

```bash
memorizer daemon launchctl
```

This command outputs a complete launchd plist file. To install:

```bash
# Create directory
mkdir -p ~/Library/LaunchAgents

# Generate and save plist
memorizer daemon launchctl > ~/Library/LaunchAgents/com.$(whoami).memorizer.plist

# Load service
launchctl load ~/Library/LaunchAgents/com.$(whoami).memorizer.plist

# Start service (if not running)
launchctl start com.$(whoami).memorizer

# Check if running
launchctl list | grep memorizer
```

**Managing the service:**

```bash
# Stop service
launchctl stop com.$(whoami).memorizer

# Restart service
launchctl stop com.$(whoami).memorizer
launchctl start com.$(whoami).memorizer

# Disable autostart (unload)
launchctl unload ~/Library/LaunchAgents/com.$(whoami).memorizer.plist

# Remove service
launchctl unload ~/Library/LaunchAgents/com.$(whoami).memorizer.plist
rm ~/Library/LaunchAgents/com.$(whoami).memorizer.plist
```

**View logs:**

```bash
# Tail daemon log
tail -f ~/.memorizer/daemon.log

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

Create `/etc/supervisor/conf.d/memorizer.conf`:

```ini
[program:memorizer]
command=/home/youruser/.local/bin/memorizer daemon start
directory=/home/youruser
autostart=true
autorestart=true
startretries=3
user=youruser
redirect_stderr=true
stdout_logfile=/var/log/memorizer/daemon.log
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
sudo supervisorctl start memorizer

# Check status
sudo supervisorctl status memorizer

# Stop service
sudo supervisorctl stop memorizer

# Restart service
sudo supervisorctl restart memorizer

# View logs
sudo supervisorctl tail -f memorizer
```

### Upgrading

When upgrading to a new version, the upgrade process depends on how you're running the daemon.

#### Upgrading with Service Managers (Recommended)

**systemd (Linux):**

```bash
# Stop service
systemctl --user stop memorizer

# Upgrade binary
go install github.com/leefowlercu/agentic-memorizer@latest
# OR: cd /path/to/repo && make install

# Start service
systemctl --user start memorizer

# Verify
systemctl --user status memorizer
memorizer version
```

**Or use restart for one command:**
```bash
# Stop and upgrade (Makefile handles daemon stop)
make install

# Restart service
systemctl --user restart memorizer
```

**launchd (macOS):**

```bash
# Stop service
launchctl stop com.$(whoami).memorizer

# Upgrade binary
go install github.com/leefowlercu/agentic-memorizer@latest
# OR: cd /path/to/repo && make install

# Start service
launchctl start com.$(whoami).memorizer

# Verify
launchctl list | grep memorizer
memorizer version
```

**Supervisor:**

```bash
# Stop service
sudo supervisorctl stop memorizer

# Upgrade binary
go install github.com/leefowlercu/agentic-memorizer@latest

# Start service
sudo supervisorctl start memorizer

# Verify
sudo supervisorctl status memorizer
```

#### Upgrading Manual Daemon

If running daemon manually (not as service):

```bash
# Stop daemon
memorizer daemon stop

# Upgrade
go install github.com/leefowlercu/agentic-memorizer@latest
# OR: cd /path/to/repo && make install

# Start daemon
memorizer daemon start

# Verify
memorizer version
```

**Note:** The Makefile install target automatically stops the daemon before replacing the binary:
```bash
# This command handles daemon shutdown automatically
make install
```

#### Service File Updates

Service files typically **do not need to be regenerated** when upgrading unless:
- The binary path changed
- New configuration options require service file changes
- Release notes explicitly mention service file updates

Service files reference the binary by path, not version:
```ini
ExecStart=/home/user/.local/bin/memorizer daemon start
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

Enable HTTP server for health checks and SSE notifications:

```yaml
daemon:
  http_port: 8080
```

Then check health at: `http://localhost:8080/health`

Response includes uptime, files processed, API calls, errors, and build status.

#### Troubleshooting

**Check daemon status:**
```bash
./memorizer daemon status
```

**Common issues:**

1. **Daemon won't start - "daemon already running"**
   - Check if daemon is actually running: `./memorizer daemon status`
   - If not running but PID file exists: `rm ~/.memorizer/daemon.pid`
   - Try starting again

2. **Daemon crashes or exits immediately**
   - Check logs: `tail -f ~/.memorizer/daemon.log`
   - Verify config file: `cat ~/.memorizer/config.yaml`
   - Ensure API key is set for your configured provider (in config or provider-specific env var)
   - Check file permissions on cache directory

3. **Index not updating after file changes**
   - Verify daemon is running: `./memorizer daemon status`
   - Check watcher is active in status output
   - Review daemon logs for file watcher errors
   - Ensure files aren't in skipped directories (`.cache`, `.git`)

4. **High API usage**
   - Reduce workers: `daemon.workers: 1` in config
   - Lower rate limit: `daemon.rate_limit_per_min: 10`
   - Increase rebuild interval: `daemon.full_rebuild_interval_minutes: 120`
   - Add files to skip list: `analysis.skip_files` in config

5. **Graph corruption after crash**
   - FalkorDB persists data to `~/.memorizer/falkordb/`
   - Force rebuild: `./memorizer daemon rebuild --force`
   - If still corrupted: Clear graph data and rebuild:
     ```bash
     memorizer daemon stop
     rm -rf ~/.memorizer/falkordb/*
     docker restart memorizer-falkordb
     memorizer daemon start
     ```

6. **Service won't start (macOS/Linux)**
   - **macOS**: Check Console.app for launchd errors
   - **Linux**: Check systemd logs: `journalctl -u memorizer.service -n 50`
   - Verify binary path in service config matches installation location
   - Check user permissions on config and cache directories

**Debug logging:**
```yaml
daemon:
  log_level: debug
```

### Adding Files to Memory

Simply add files to `~/.memorizer/memory/` (or the directory you've configured as the `memory_root` in `config.yaml`):

```bash
# Organize however you like
~/.memorizer/memory/
├── documents/
│   └── project-plan.md
├── presentations/
│   └── quarterly-review.pptx
└── images/
    └── architecture-diagram.png
```

On your next Claude Code session, these files will be automatically analyzed and indexed.

### Using the MCP Server

The MCP (Model Context Protocol) server provides AI agents with tools, prompts, and resources to interact with your memory index. This section covers how to use the MCP server regardless of which AI agent framework you're using (Claude Code, Gemini CLI, Codex CLI, etc.).

The MCP server provides three types of capabilities: **Tools** for performing operations, **Prompts** for generating contextual messages, and **Resources** for accessing the memory index with real-time updates.

#### MCP Tools

The MCP server exposes five tools that AI agents can invoke to interact with your memory index:

**1. search_files**

Search for files using semantic search across filenames, summaries, tags, and topics.

**Parameters:**
- `query` (required): Search query text
- `categories` (optional): Array of categories to filter by (e.g., `["documents", "code"]`)
- `max_results` (optional): Maximum results to return (default: 10, max: 100)

**Example prompts that trigger this tool:**
- "Search my memory for files about authentication"
- "Find documents related to API design"
- "Show me code files that mention database migrations"
- "What files do I have about FalkorDB?"

**2. get_file_metadata**

Retrieve complete metadata and semantic analysis for a specific file.

**Parameters:**
- `path` (required): Absolute path to the file

**Example prompts that trigger this tool:**
- "Show me details about ~/.memorizer/memory/docs/api-guide.md"
- "What's in my architecture diagram file?"
- "Get metadata for /Users/me/.memorizer/memory/notes.md"

**3. list_recent_files**

List recently modified files within a specified time period.

**Parameters:**
- `days` (optional): Number of days to look back (default: 7, max: 365)
- `limit` (optional): Maximum number of files (default: 20, max: 100)

**Example prompts that trigger this tool:**
- "What files did I add this week?"
- "Show me files modified in the last 3 days"
- "List my recent documents"

**4. get_related_files**

Find files connected through shared tags, topics, or entities in the knowledge graph.

**Parameters:**
- `path` (required): Path to the source file
- `limit` (optional): Maximum related files to return (default: 10, max: 50)

**Requirements:** FalkorDB must be running

**Example prompts that trigger this tool:**
- "What files are related to my API documentation?"
- "Find files similar to ~/.memorizer/memory/architecture.md"
- "Show me documents connected to this design proposal"

**5. search_entities**

Search for files that mention specific entities (technologies, people, concepts, organizations).

**Parameters:**
- `entity` (required): Entity name to search for
- `entity_type` (optional): Filter by type (`technology`, `person`, `concept`, `organization`)
- `max_results` (optional): Maximum results (default: 10, max: 100)

**Requirements:** FalkorDB must be running

**Example prompts that trigger this tool:**
- "Which files mention Terraform?"
- "Find documents about authentication"
- "Show me files that reference Docker"
- "What mentions Go programming language?"

#### MCP Prompts

The MCP server provides three pre-configured prompt templates that generate contextual messages for analysis. These are currently available in Claude Code and may be supported by other MCP clients in the future.

**1. analyze-file**

Generates a detailed analysis request using the file's semantic metadata.

**Arguments:**
- `file_path` (required): Path to the file to analyze
- `focus` (optional): Specific aspect to focus on (e.g., "security", "performance", "architecture")

**What it does:**
Creates a prompt that asks the AI to analyze the file's purpose, main concepts, relationships to other files, and notable patterns based on its semantic summary, tags, and topics.

**Usage:**
If your MCP client supports prompts, select "analyze-file" from the prompt selector, provide the file path, and optionally specify a focus area like "security implications" or "architectural patterns".

**2. search-context**

Helps construct effective search queries by identifying related terms and strategies.

**Arguments:**
- `topic` (required): Topic or concept to search for
- `category` (optional): File category to focus on (e.g., "documents", "code")

**What it does:**
Generates suggestions for key terms, related tags, file types to focus on, and alternative search terms based on the specified topic.

**Usage:**
Use this prompt when you know what you're looking for conceptually but need help formulating an effective search query. Provide a topic like "API authentication" and get back ranked search strategies.

**3. explain-summary**

Generates a detailed explanation of how a file's semantic analysis was derived.

**Arguments:**
- `file_path` (required): Path to the file whose summary to explain

**What it does:**
Creates a prompt asking the AI to explain what the summary reveals about the file, how tags and topics were determined, the significance of the document type classification, and how to interpret the information.

**Usage:**
Use this prompt when you want to understand why a file was analyzed and tagged in a particular way. Useful for validating or understanding the semantic analysis results.

#### MCP Resources

The MCP server exposes the memory index as three resources in different formats:

**Available Resources:**

1. **memorizer://index**
   - Format: XML
   - MIME Type: `application/xml`
   - Description: Complete semantic index with hierarchical structure optimized for AI consumption

2. **memorizer://index/markdown**
   - Format: Markdown
   - MIME Type: `text/markdown`
   - Description: Human-readable format with rich formatting and emojis

3. **memorizer://index/json**
   - Format: JSON
   - MIME Type: `application/json`
   - Description: Structured data format for programmatic access

**Reading Resources:**

MCP clients can read these resources directly to access the full memory index. This is useful when you want complete context about all indexed files rather than querying specific files or searching.

**Resource Subscriptions:**

The MCP server supports resource subscriptions for real-time updates:

**How it works:**

1. **Subscribe:** MCP client subscribes to one or more resource URIs (e.g., `memorizer://index`)
2. **Daemon Updates:** When files are added, modified, or deleted, the daemon rebuilds the index
3. **SSE Notification:** Daemon sends Server-Sent Event (SSE) to connected MCP servers
4. **Resource Updated:** MCP server sends `notifications/resources/updated` to subscribed clients
5. **Client Refresh:** AI agent automatically knows the index has changed and can re-fetch

**Benefits:**
- AI agents stay synchronized with your latest files without manual refresh
- Real-time awareness of newly added documents, images, or code
- Automatic context updates during long-running sessions

**Configuration:**

The MCP server connects to the daemon's SSE endpoint automatically when `daemon.http_port` is configured:

```yaml
daemon:
  http_port: 8080  # Enable HTTP API and SSE notifications
```

**Subscription Workflow Example:**

1. AI agent starts and connects to MCP server
2. Agent subscribes to `memorizer://index/markdown` resource
3. You add a new document: `~/.memorizer/memory/new-design.md`
4. Daemon detects the file, analyzes it, rebuilds index
5. Daemon sends SSE event: `{"type": "index_updated", ...}`
6. MCP server receives event and checks subscriptions
7. MCP server sends notification to agent: `{"method": "notifications/resources/updated", "params": {"uri": "memorizer://index/markdown"}}`
8. Agent re-fetches the resource and now knows about `new-design.md`

This creates a seamless experience where your AI agent automatically becomes aware of new files as you add them to memory.

### Manual Testing

View the precomputed index and facts:

```bash
# Start daemon if not already running
memorizer daemon start

# In another terminal, read the file index
memorizer read files

# Read user facts
memorizer read facts
```

This outputs the index (XML by default) that AI agents receive. The daemon must be running (or have completed at least one indexing cycle) for the index file to exist.

### CLI Usage

**Commands:**

```bash
# Initialize config and memory directory
memorizer initialize [flags]

# Manage background daemon
memorizer daemon start
memorizer daemon stop
memorizer daemon status
memorizer daemon systemctl      # Generate systemd unit file
memorizer daemon launchctl      # Generate launchd plist

# Manage FalkorDB knowledge graph
memorizer graph start           # Start FalkorDB container
memorizer graph stop            # Stop FalkorDB container
memorizer graph status          # Check graph health and stats
memorizer daemon rebuild        # Rebuild index/graph (use --force to clear first)

# Manage semantic analysis cache
memorizer cache status          # Show cache statistics and version info
memorizer cache clear --stale   # Clear stale cache entries
memorizer cache clear --all     # Clear all cache entries

# Read precomputed index and facts
memorizer read files [flags]    # Read file index (SessionStart hooks)
memorizer read facts [flags]    # Read user facts (UserPromptSubmit/BeforeAgent hooks)

# Manage user-defined facts
memorizer remember fact "fact content"   # Add a new fact
memorizer forget fact <fact-id>          # Remove a fact by ID

# Manage agent framework integrations
memorizer integrations list
memorizer integrations detect
memorizer integrations setup <integration-name>
memorizer integrations remove <integration-name>
memorizer integrations health

# MCP server
memorizer mcp start

# Manage configuration
memorizer config validate
memorizer config reload
memorizer config show-schema

# Get help
memorizer --help
memorizer initialize --help
memorizer daemon --help
memorizer read --help
memorizer remember --help
memorizer forget --help
memorizer integrations --help
memorizer config --help
```

**Common Flags:**

```bash
# Read files/facts command flags
--format <xml|markdown|json>        # Output format
--integration <name>                # Format for specific integration (claude-code-hook, etc)

# Remember fact command flags
--id <uuid>                         # Update existing fact by ID

# Init command flags
--memory-root <dir>                 # Custom memory directory
--cache-dir <dir>                   # Custom cache directory
--force                             # Overwrite existing config
--integrations                # Configure agent framework integrations
--skip-integrations                 # Skip integration setup prompt
--http-port <port>                  # HTTP API port (0=disable, -1=interactive prompt)
```

**Examples:**

```bash
# Initialize (interactive prompts for API key, HTTP port, integrations)
memorizer initialize

# Initialize with HTTP API enabled on port 7600 (scripted, no prompt)
memorizer initialize --http-port 7600 --integrations

# Read file index (XML format - default)
memorizer read files

# Read file index (Markdown format)
memorizer read files --format markdown

# Read file index (JSON format)
memorizer read files --format json

# Read file index with Claude Code hook integration (SessionStart)
memorizer read files --format xml --integration claude-code-hook

# Read user facts (for UserPromptSubmit/BeforeAgent hooks)
memorizer read facts

# Read facts with integration-specific formatting
memorizer read facts --format xml --integration claude-code-hook
memorizer read facts --format xml --integration gemini-cli-hook

# Add a new fact
memorizer remember fact "I prefer TypeScript over JavaScript"

# View all facts
memorizer read facts

# Update an existing fact
memorizer remember fact "I prefer Go over TypeScript" --id <fact-id>

# Remove a fact
memorizer forget fact <fact-id>

# Note: MCP integration uses tools, not read command

# Start daemon
memorizer daemon start

# Check daemon status
memorizer daemon status

# Force rebuild index
memorizer daemon rebuild

# List available integrations
memorizer integrations list

# Detect installed agent frameworks
memorizer integrations detect

# Setup Claude Code hooks (SessionStart + UserPromptSubmit)
memorizer integrations setup claude-code-hook

# Setup Claude Code MCP server
memorizer integrations setup claude-code-mcp

# Setup Gemini CLI hooks (SessionStart + BeforeAgent)
memorizer integrations setup gemini-cli-hook

# Setup Gemini CLI MCP server
memorizer integrations setup gemini-cli-mcp

# Remove integrations
memorizer integrations remove claude-code-hook
memorizer integrations remove claude-code-mcp
memorizer integrations remove gemini-cli-hook

# Validate integration configurations
memorizer integrations health
```

### Controlling Semantic Analysis

Semantic analysis is automatically enabled when an API key is configured for any provider. To disable semantic analysis, remove the API key configuration from `config.yaml`:

```yaml
semantic:
  provider: claude  # Provider still required for configuration
  api_key: ""       # Empty or missing = semantic analysis disabled
```

When disabled, the daemon will only extract file metadata without semantic analysis.

## Supported File Types

### Directly Readable by Claude Code
- Markdown (`.md`)
- Text files (`.txt`)
- Configuration files (`.json`, `.yaml`, `.yml`, `.toml`)
- Images (`.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`)
- Code files (`.go`, `.py`, `.js`, `.ts`, `.java`, `.c`, `.cpp`, `.rs`, `.rb`, `.php`)
- Transcripts (`.vtt`, `.srt`)

### Requires Extraction
- Word documents (`.docx`)
- PowerPoint (`.pptx`)
- PDFs (`.pdf`)

The index tells your AI agent which method to use for each file.

## Configuration Options

The configuration system follows "convention over configuration" principles. Most settings have optimal defaults, so you only need to configure what you want to customize.

### Configuration Tiers

**User-Facing Settings** (shown after `initialize`):
- `memory_root` - Directory containing your memory files
- `semantic.provider` - Semantic analysis provider (claude, openai, or gemini)
- `semantic.api_key` - Provider API key (or use env var: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, or `GOOGLE_API_KEY`)
- `semantic.model` - Model for analysis (provider-specific, e.g., `claude-sonnet-4-5-20250929`, `gpt-5.2-chat-latest`, `gemini-2.5-flash`)
- `daemon.http_port` - HTTP API port for MCP integration (0 to disable)
- `daemon.log_level` - Daemon log verbosity (debug/info/warn/error)
- `mcp.log_level` - MCP server log verbosity
- `graph.host` / `graph.port` - FalkorDB connection settings
- `graph.password` - FalkorDB password (or set `FALKORDB_PASSWORD` env var)
- `embeddings.api_key` - OpenAI API key for embeddings (or set `OPENAI_API_KEY` env var)

**Advanced Settings** (available but not in initialized config):

These settings have optimal defaults but can be customized by adding them to your `config.yaml`:

```yaml
# Semantic analysis provider tuning
semantic:
  provider: claude           # Provider: claude, openai, or gemini
  max_tokens: 1500           # Response length limit per analysis (1-8192)
  timeout: 30                # API request timeout in seconds (5-300)
  enable_vision: true        # Enable vision API for image analysis
  rate_limit_per_min: 20     # Provider-specific (Claude: 20, OpenAI: 60, Gemini: 100)

# Analysis tuning
analysis:
  max_file_size: 10485760    # 10MB - files larger than this skip semantic analysis
  skip_extensions: [.zip, .tar, .gz, .exe, .bin, .dmg, .iso]
  skip_files: [memorizer]
  cache_dir: ~/.memorizer/.cache

# Daemon performance tuning
daemon:
  debounce_ms: 500           # Wait time before processing file changes
  workers: 3                 # Parallel processing workers
  rate_limit_per_min: 20     # Provider API rate limit
  full_rebuild_interval_minutes: 60
  log_file: ~/.memorizer/daemon.log

# MCP server settings
mcp:
  log_file: ~/.memorizer/mcp.log
  daemon_host: localhost
  daemon_port: 0             # Set to match daemon.http_port for MCP integration

# Embeddings tuning
embeddings:
  provider: openai           # Embedding provider (only 'openai' currently supported)
  model: text-embedding-3-small  # Embedding model
  dimensions: 1536           # Vector dimensions (must match model)

# Graph tuning
graph:
  similarity_threshold: 0.7  # Minimum similarity for related files (0.0-1.0)
  max_similar_files: 10      # Max similar files per query
```

To discover all available settings:

```bash
memorizer config show-schema --advanced-only
```

**Derived Settings** (computed automatically):
- `semantic.enabled` - Automatically enabled when provider API key is set
- `embeddings.enabled` - Automatically enabled when OpenAI API key is set

See `config.yaml.example` for a complete reference with all available options

### File Exclusions

The indexer automatically excludes:
- Hidden files and directories (starting with `.`)
- The `.cache/` directory (where analyses are cached)
- The `memorizer` binary itself (if located in the memory directory)

You can exclude additional files by name or extension in `config.yaml`:

```yaml
analysis:
  skip_files:
    - memorizer  # Default
    - my-private-notes.md
    - temp-file.txt
  skip_extensions:
    - .log
    - .tmp
    - .bak
    - .swp
```

Files matching skip patterns are completely ignored during indexing and won't appear in the generated index.

### Environment Variables

#### Configuration Override Pattern

All configuration settings can be overridden using environment variables with the `MEMORIZER_` prefix. Configuration keys use dot notation (e.g., `semantic.model`), which maps to environment variables by replacing dots with underscores and adding the prefix.

**Examples:**

```bash
# Override memory_root
export MEMORIZER_MEMORY_ROOT=/custom/memory/path

# Override semantic.provider
export MEMORIZER_SEMANTIC_PROVIDER=openai

# Override semantic.model
export MEMORIZER_SEMANTIC_MODEL=gpt-5.2-chat-latest

# Override daemon.workers
export MEMORIZER_DAEMON_WORKERS=5

# Override daemon.http_port
export MEMORIZER_DAEMON_HTTP_PORT=8080
```

**Priority:** Environment variables take precedence over `config.yaml` settings.

#### Credential Environment Variables

API keys and passwords have dedicated environment variables that are checked before falling back to config file values. The semantic analysis provider determines which API key is required:

**ANTHROPIC_API_KEY**

Claude API key for semantic analysis when `semantic.provider: claude`. If not set, falls back to `semantic.api_key` in config.

```bash
export ANTHROPIC_API_KEY="your-claude-api-key"
```

**OPENAI_API_KEY**

OpenAI API key for semantic analysis when `semantic.provider: openai`, or for vector embeddings (optional). Falls back to `semantic.api_key` or `embeddings.api_key` in config.

```bash
export OPENAI_API_KEY="your-openai-api-key"
```

**GOOGLE_API_KEY**

Google API key for semantic analysis when `semantic.provider: gemini`. If not set, falls back to `semantic.api_key` in config.

```bash
export GOOGLE_API_KEY="your-google-api-key"
```

**FALKORDB_PASSWORD**

FalkorDB password for graph database authentication (optional). If not set, falls back to `graph.password` in config.

```bash
export FALKORDB_PASSWORD="your-falkordb-password"
```

**Best Practice:** Use these credential-specific environment variables instead of storing API keys in the config file.

#### MEMORIZER_APP_DIR

Customizes the application directory location. By default, configuration and data files are stored in `~/.memorizer/`.

```bash
# Use a custom app directory
export MEMORIZER_APP_DIR=/path/to/custom/location
memorizer initialize

# Or for a single command
MEMORIZER_APP_DIR=/tmp/test-instance memorizer daemon start
```

**Files stored in the app directory:**
- `config.yaml` - Configuration file
- `daemon.pid` - Daemon process ID
- `daemon.log` - Daemon logs (if configured)
- `mcp.log` - MCP server logs (if configured)
- `falkordb/` - FalkorDB data persistence directory

**Use cases:**
- **Testing**: Run isolated test instances without affecting your main instance
- **Multi-instance**: Run multiple independent instances for different projects
- **Containers**: Use custom paths in Docker or other containerized environments
- **CI/CD**: Isolate build/test environments

**Note**: The memory directory and cache directory locations are controlled by `config.yaml` settings (or their corresponding `MEMORIZER_MEMORY_ROOT` and `MEMORIZER_ANALYSIS_CACHE_DIR` environment variables), not `MEMORIZER_APP_DIR`. Only the application's own files (config, PID, logs, FalkorDB data) use the app directory.

### Output Formats

The memorizer supports three output formats for both files and facts:

#### XML (Default)

Highly structured XML following Anthropic's recommendations for Claude [prompt engineering](https://docs.claude.com/en/docs/build-with-claude/prompt-engineering/use-xml-tags):

```bash
memorizer read files
# or explicitly:
memorizer read files --format xml

# Facts also use XML by default
memorizer read facts
```

#### Markdown

Human-readable markdown, formatted for direct viewing:

```bash
memorizer read files --format markdown
memorizer read facts --format markdown
```

#### JSON Format

Pretty-printed JSON representation of the index:

```bash
memorizer read files --format json
memorizer read facts --format json
```

#### Integration-Specific Output

Use the `--integration` flag to format output for specific agent frameworks. This wraps the content in the appropriate hook structure:

```bash
# Claude Code integration
memorizer read files --format xml --integration claude-code-hook  # SessionStart hook
memorizer read facts --format xml --integration claude-code-hook  # UserPromptSubmit hook

# Gemini CLI integration
memorizer read files --format xml --integration gemini-cli-hook  # SessionStart hook
memorizer read facts --format xml --integration gemini-cli-hook  # BeforeAgent hook

# Can also use markdown or json formats
memorizer read files --format markdown --integration claude-code-hook
memorizer read facts --format json --integration gemini-cli-hook

# Note: MCP integration doesn't use read - uses tools instead
```

**Claude Code Integration Output Structure:**

The Claude Code integration uses different hook structures for files and facts:

**SessionStart (file index):**
```json
{
  "continue": true,
  "suppressOutput": true,
  "systemMessage": "Memory index updated: 15 files (5 documents, 3 images, 2 presentations, 5 code files), 2.3 MB total",
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "<memory_index>...</memory_index>"
  }
}
```

**UserPromptSubmit (facts):**
```json
{
  "continue": true,
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "<facts_index>...</facts_index>"
  }
}
```

- **continue**: Always `true` - allows session/prompt to proceed
- **suppressOutput**: `true` for SessionStart to keep verbose index out of transcript
- **systemMessage**: Concise summary visible to user in UI (SessionStart only)
- **hookSpecificOutput**: Contains the formatted content in `additionalContext`

**More Info**

[Claude Hook JSON Common Fields](https://docs.claude.com/en/docs/claude-code/hooks#common-json-fields)

[Claude SessionStart Hook Fields](https://docs.claude.com/en/docs/claude-code/hooks#sessionstart-decision-control)

[Claude UserPromptSubmit Hook Fields](https://docs.claude.com/en/docs/claude-code/hooks#userpromptsubmit-decision-control)

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
          <sections>
            <section>Introduction</section>
            <section>RESTful Principles</section>
            <section>Versioning Strategies</section>
            <section>Authentication</section>
            <section>Error Handling</section>
            <section>Rate Limiting</section>
            <section>Documentation</section>
            <section>Best Practices</section>
          </sections>
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
**Modified**: 2025-10-04 | **Size**: 45.2 KB | **Words**: 4,520
**Sections**: Introduction • RESTful Principles • Versioning Strategies • Authentication • Error Handling • Rate Limiting • Documentation • Best Practices
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
├── docker-compose.yml        # FalkorDB Docker configuration
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
│   ├── graph/                # FalkorDB graph management commands
│   │   ├── graph.go          # Parent graph command
│   │   └── subcommands/      # Graph subcommands (3 total)
│   │       ├── start.go      # Start FalkorDB container
│   │       ├── stop.go       # Stop FalkorDB container
│   │       └── status.go     # Check graph health
│   ├── cache/                # Cache management commands
│   │   ├── cache.go          # Parent cache command
│   │   └── subcommands/      # Cache subcommands (2 total)
│   │       ├── status.go     # Show cache statistics
│   │       └── clear.go      # Clear cache entries
│   ├── mcp/                  # MCP server commands
│   │   ├── mcp.go            # Parent mcp command
│   │   └── subcommands/
│   │       └── start.go      # Start MCP server
│   ├── integrations/         # Integration management commands
│   │   ├── integrations.go   # Parent integrations command
│   │   └── subcommands/      # Integration subcommands (5 total)
│   │       ├── list.go
│   │       ├── detect.go
│   │       ├── setup.go
│   │       ├── remove.go
│   │       ├── health.go     # Health check and validation
│   │       └── helpers.go
│   ├── config/               # Configuration commands
│   │   ├── config.go         # Parent config command
│   │   └── subcommands/      # Config subcommands (3 total)
│   │       ├── validate.go
│   │       ├── reload.go
│   │       └── show_schema.go
│   ├── read/                 # Read file index and facts
│   │   ├── read.go           # Parent read command
│   │   └── subcommands/      # Read subcommands (2 total)
│   │       ├── files.go      # Read file index
│   │       └── facts.go      # Read user facts
│   ├── remember/             # Remember (create) commands
│   │   ├── remember.go       # Parent remember command
│   │   └── subcommands/
│   │       └── fact.go       # Remember a fact
│   ├── forget/               # Forget (delete) commands
│   │   ├── forget.go         # Parent forget command
│   │   └── subcommands/
│   │       └── fact.go       # Forget a fact
│   └── version/              # Version command
│       └── version.go
├── internal/
│   ├── config/               # Configuration loading, validation, and hot-reload
│   ├── daemon/               # Background daemon implementation
│   │   ├── api/              # HTTP API server, handlers, SSE
│   │   └── worker/           # Worker pool for file processing
│   ├── graph/                # FalkorDB knowledge graph
│   │   ├── client.go         # FalkorDB connection management
│   │   ├── manager.go        # Graph operations (CRUD, search)
│   │   ├── queries.go        # Cypher query patterns
│   │   ├── schema.go         # Node/edge types and constraints
│   │   ├── export.go         # Graph to index export
│   │   └── facts.go          # Facts CRUD operations
│   ├── embeddings/           # Vector embeddings (optional)
│   ├── watcher/              # File system watching (fsnotify)
│   ├── walker/               # File system traversal with filtering
│   ├── logging/              # Structured logging with slog, rotation, and context
│   ├── document/             # Office document extraction (DOCX, PPTX)
│   ├── metadata/             # File metadata extraction (9 category handlers)
│   ├── semantic/             # Multi-provider semantic analysis (Claude, OpenAI, Gemini)
│   ├── cache/                # Content-addressable analysis caching
│   ├── search/               # Semantic search engine (graph-powered)
│   ├── format/               # Output formatting system
│   │   ├── formatters/       # Individual formatters (text, JSON, XML, YAML, markdown)
│   │   └── testdata/         # Test data for formatters
│   ├── mcp/                  # MCP server implementation
│   │   ├── protocol/         # JSON-RPC 2.0 protocol messages
│   │   └── transport/        # Stdio transport layer
│   ├── integrations/         # Integration framework and adapters
│   │   └── adapters/         # Framework-specific adapters
│   │       ├── claude/       # Hook and MCP adapters for Claude Code
│   │       ├── gemini/       # Hook and MCP adapters for Gemini CLI
│   │       └── codex/        # MCP adapter for Codex CLI
│   ├── docker/               # Docker container management utilities
│   ├── servicemanager/       # Service manager integration (systemd, launchd)
│   ├── tui/                  # Terminal UI components
│   │   ├── initialize/       # Initialization wizard
│   │   └── styles/           # TUI styling
│   └── version/              # Version information and embedding
│       ├── VERSION           # Semantic version file (embedded)
│       └── version.go        # Version getters with buildinfo fallback
├── scripts/                  # Release automation scripts
│   ├── bump-version.sh       # Semantic version bumping
│   └── prepare-release.sh    # Release preparation and automation
├── pkg/types/                # Shared types and data structures
├── docs/                     # Documentation
│   ├── subsystems/           # Comprehensive subsystem documentation
│   ├── migration/            # Migration guides
│   └── wip/                  # Work in progress documentation
├── e2e/                      # End-to-end testing framework
│   ├── harness/              # Test harness and utilities
│   ├── tests/                # Test suites
│   ├── fixtures/             # Test fixtures and data
│   ├── scripts/              # Test automation scripts
│   ├── docker-compose.yml    # Test environment setup
│   └── Dockerfile.test       # Test container image
└── testdata/                 # Unit test files
```

### Building and Testing

```bash
# Building
make build             # Build binary with version info from git
make install           # Build and install to ~/.local/bin

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
- **E2E tests** (`make test-e2e`) - Complete workflows with Docker-based FalkorDB
- Integration tests use `MEMORIZER_APP_DIR` for isolated environments
- Test data in `testdata/` directory

### End-to-End Testing

The project includes comprehensive E2E tests covering complete workflows across all major subsystems:

```bash
# Run all E2E tests (requires Docker for FalkorDB)
make test-e2e

# Run specific E2E test suite
go test -tags=e2e -v ./e2e/tests/ -run TestCLI        # CLI commands
go test -tags=e2e -v ./e2e/tests/ -run TestDaemon     # Daemon lifecycle
go test -tags=e2e -v ./e2e/tests/ -run TestHTTPAPI    # HTTP endpoints
go test -tags=e2e -v ./e2e/tests/ -run TestMCP        # MCP server
go test -tags=e2e -v ./e2e/tests/ -run TestGraph      # FalkorDB operations
```

**E2E Test Coverage:**
- **CLI Tests** - All commands with argument parsing and output validation
- **Daemon Tests** - Start, stop, status, restart, rebuild operations
- **Filesystem Tests** - File watching, processing pipelines, cache behavior
- **HTTP API Tests** - All REST endpoints with request/response validation
- **SSE Tests** - Real-time event delivery and connection management
- **Configuration Tests** - Loading, validation, hot-reload, error handling
- **Graph Tests** - FalkorDB CRUD, schema, queries, and graceful degradation
- **Facts Tests** - Remember, read, forget commands with validation
- **Integration Tests** - All framework adapters (Claude Code, Gemini, Codex, etc.)
- **Integration Facts Tests** - Dual-hook setup (SessionStart + UserPromptSubmit/BeforeAgent)
- **Output Format Tests** - XML, JSON, Markdown processors with schema validation
- **Walker Tests** - Directory/file/extension skip patterns

The test harness (`e2e/harness/`) provides isolated environments, daemon management, and automatic cleanup. See `docs/subsystems/e2e-tests/` for architecture details.

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

- **API Costs**: Semantic analysis uses API calls to your configured provider (costs apply)
  - Mitigated by caching (only analyzes new/modified files)
  - Can disable semantic analysis by removing API key for metadata-only mode

- **File Size Limit**: Default 10MB max for semantic analysis
  - Configurable via `analysis.max_file_size` in config
  - Larger files are indexed with metadata only

- **Internet Required**: Needs connection for provider API calls
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

Set the API key for your configured provider:

```bash
# For Claude (default provider)
export ANTHROPIC_API_KEY="your-key-here"

# For OpenAI
export OPENAI_API_KEY="your-key-here"

# For Google Gemini
export GOOGLE_API_KEY="your-key-here"
```

Check your configured provider in `~/.memorizer/config.yaml` under `semantic.provider`.

### Index not appearing in AI agent

1. Verify daemon is running: `memorizer daemon status`
2. Check your framework's integration configuration:
   - **Claude Code**: Check `~/.claude/settings.json` has SessionStart hooks configured
   - **Other frameworks**: Verify you followed the setup instructions from `memorizer integrations setup <framework-name>`
3. Verify binary path is correct (`~/.local/bin/memorizer` or `~/go/bin/memorizer`)
4. Test manually: `memorizer read`
5. Check your AI agent's output/logs for errors

### Config reload not applying changes

1. Some settings require daemon restart (see Daemon Configuration section)
2. Validate config syntax: `memorizer config validate`
3. Check daemon logs: `tail -f ~/.memorizer/daemon.log`
4. If reload fails, restart: `memorizer daemon restart`

### Reducing resource usage

When indexing many files:
- Reduce daemon workers: `daemon.workers: 1` in config
- Lower rate limit: `daemon.rate_limit_per_min: 10` in config
- Disable semantic analysis temporarily by removing API key from config

### Cache issues

The semantic analysis cache uses versioning to detect stale entries after application upgrades.

**Check cache status:**

```bash
memorizer cache status
```

This shows:
- Current cache version
- Total entries and size
- Version distribution (current, legacy, stale)
- Number of entries that need re-analysis

**Clear stale entries (recommended after upgrade):**

```bash
# Clear only stale/legacy entries
memorizer cache clear --stale

# Or include with rebuild
memorizer daemon rebuild --clear-stale
```

**Force re-analysis of all files:**

```bash
# Clear all cache entries
memorizer cache clear --all
memorizer daemon restart
```

**Legacy entries (v0.0.0):** Entries from before cache versioning was implemented. They will be re-analyzed automatically on next daemon rebuild.

### Graph data issues

If you need to reset the knowledge graph (e.g., seeing stale data, want to start fresh):

**Clear graph data:**

```bash
# Stop daemon first
memorizer daemon stop

# Delete persistence files
rm -rf ~/.memorizer/falkordb/*

# Restart FalkorDB container
docker restart memorizer-falkordb

# Start daemon (will rebuild from memory files)
memorizer daemon start
```

**Verify graph was cleared:**

```bash
memorizer graph status
```

This shows node/relationship counts. After clearing, you should see 5 nodes (category nodes) and 0 files.

## Contributing

Contributions are welcome! To contribute:

1. **Report Issues**: Open an issue on GitHub describing the problem
2. **Suggest Features**: Propose new features via GitHub issues
3. **Submit Pull Requests**: Fork the repo, make changes, and submit a PR
4. **Follow Standards**: Use Go conventions, add tests, update docs

See existing code for examples and patterns.

## License

MIT License - see [LICENSE](LICENSE) file for details.
