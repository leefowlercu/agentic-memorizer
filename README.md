# Agentic Memorizer

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25.5-00ADD8.svg)](https://go.dev/)

A knowledge graph-based memorization tool for AI agents.

**Current Version:** N/A

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [CLI Commands](#cli-commands)
- [Configuration](#configuration)
- [Integrations](#integrations)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [License](#license)

## Overview

Agentic Memorizer is an automated knowledge graph builder designed to give AI assistants persistent, queryable memory of filesystem content. Users register directories they want the system to "remember," and a background daemon takes over from there.

The daemon continuously watches registered directories for file changes and periodically walks them to ensure completeness. When files are added, modified, or removed, the system automatically:

1. **Filters** content based on configurable skip/include rules (extensions, directories, hidden files)
2. **Chunks** files using format-specific parsers that preserve semantic structure
3. **Analyzes** chunks via AI providers to extract topics, entities, summaries, and tags
4. **Generates embeddings** for semantic similarity search
5. **Persists** everything to a FalkorDB knowledge graph with typed relationships

The resulting knowledge graph is exposed to AI coding assistants through multiple integration methods: the Model Context Protocol (MCP) for standards-based access, hooks for injecting context at session start, and plugins for native tool integration. This enables AI assistants to understand and query any content you point it at—codebases, documentation, research notes, configuration repositories, or any other file-based knowledge.

Key capabilities:

- **Intelligent Chunking** - 20+ format-specific chunkers with language-aware semantic splitting using Tree-sitter AST parsing for code and structure-preserving chunking for documents
- **Semantic Analysis** - Pluggable providers (Anthropic, OpenAI, Google) extract topics, entities, and summaries from content
- **Vector Embeddings** - OpenAI, Voyage AI, and Google providers generate embeddings for semantic similarity search
- **Knowledge Graph** - FalkorDB (Redis Graph) backend stores files, chunks, metadata, and relationships
- **Real-time Monitoring** - Filesystem watcher with event coalescing detects changes and triggers analysis
- **MCP Integration** - Standards-based protocol exposes knowledge graph to AI tools

## Quick Start

1. **Build and install the binary**
   ```bash
   git clone https://github.com/leefowlercu/agentic-memorizer.git
   cd agentic-memorizer
   make install
   ```

2. **Run the setup wizard**
   ```bash
   memorizer initialize
   ```

3. **Start the daemon**
   ```bash
   memorizer daemon start
   ```

4. **Register a directory to monitor**
   ```bash
   memorizer remember ~/projects/my-codebase
   ```

5. **List remembered directories**
   ```bash
   memorizer list
   ```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLI Layer (Cobra)                           │
│  [version] [initialize] [daemon] [remember] [forget] [list] [read]  │
│  [integrations] [providers] [config]                                │
└──────────────────┬──────────────────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────────────────┐
│                       Daemon Core                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │  Component   │  │  Health      │  │  HTTP Server │               │
│  │  Lifecycle   │  │  Manager     │  │  (7600)      │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
└──────────────────┬──────────────────────────────────────────────────┘
                   │
        ┌──────────┴──────────┬──────────────┬──────────────┐
        │                     │              │              │
┌───────▼────────┐  ┌─────────▼─────┐ ┌──────▼──────┐ ┌─────▼────────┐
│  Filesystem    │  │   Analysis    │ │   Semantic  │ │ Embeddings   │
│  Watcher       │  │   Pipeline    │ │   Providers │ │   Providers  │
└────────────────┘  └───────┬───────┘ └─────────────┘ └──────────────┘
                            │
                    ┌───────▼───────┐
                    │  Knowledge    │
                    │  Graph        │
                    │  (FalkorDB)   │
                    └───────────────┘
```

**Data Flow:**

1. Filesystem watcher detects changes in registered directories
2. Events are debounced and queued for analysis
3. Format-specific chunkers split files and extract metadata
4. Semantic providers analyze content for topics, entities, and summaries
5. Embeddings providers generate vector representations
6. Results are stored in the FalkorDB knowledge graph
7. CLI and MCP server provide query interfaces

## CLI Commands

| Command | Description |
|---------|-------------|
| `version` | Display build information |
| `initialize` | Run the interactive setup wizard |
| `daemon start` | Start the daemon in foreground mode |
| `daemon stop` | Stop the running daemon gracefully |
| `daemon status` | Show daemon status and health metrics |
| `daemon rebuild` | Rebuild the knowledge graph |
| `remember <path>` | Register a directory for tracking |
| `forget <path>` | Unregister a directory |
| `list` | List all remembered directories |
| `read` | Export the knowledge graph |
| `integrations list` | List available integrations |
| `integrations setup <name>` | Configure an integration |
| `integrations status` | Show integration status |
| `integrations remove <name>` | Remove an integration |
| `providers list` | List semantic/embeddings providers |
| `providers test <name>` | Test provider connectivity |
| `config show` | Display current configuration |
| `config edit` | Open configuration in editor |
| `config validate` | Validate configuration file |
| `config reset` | Reset to default configuration |

## Configuration

Configuration is stored at `~/.config/memorizer/config.yaml` with environment variable overrides using the `MEMORIZER_` prefix.

```yaml
log_level: info
log_file: ~/.config/memorizer/memorizer.log

daemon:
  http_port: 7600
  http_bind: 127.0.0.1
  shutdown_timeout: 30
  pid_file: ~/.config/memorizer/daemon.pid

graph:
  host: localhost
  port: 6379

semantic:
  provider: anthropic
  rate_limit: 10

embeddings:
  enabled: true
  provider: openai
```

Environment variable examples:
- `MEMORIZER_DAEMON_HTTP_PORT=9000`
- `MEMORIZER_GRAPH_HOST=redis.local`
- `MEMORIZER_SEMANTIC_PROVIDER=google`

## Integrations

Agentic Memorizer integrates with AI coding assistants via hooks and MCP (Model Context Protocol):

| Harness | Integrations |
|---------|--------------|
| claude-code | `claude-code-hook`, `claude-code-mcp` |
| gemini-cli | `gemini-cli-hook`, `gemini-cli-mcp` |
| codex-cli | `codex-cli-mcp` |
| opencode | `opencode-mcp`, `opencode-plugin` |

Setup an integration:
```bash
memorizer integrations setup claude-code-mcp
```

## Prerequisites

- Go 1.25.5 or later
- FalkorDB (Redis Graph) instance
- API keys for semantic/embeddings providers (as needed)

## Installation

**From source:**
```bash
git clone https://github.com/leefowlercu/agentic-memorizer.git
cd agentic-memorizer
make build
```

**Install to ~/.local/bin:**
```bash
make install
```

**Platform service registration:**

macOS (launchd):
```bash
memorizer daemon start --register
```

Linux (systemd):
```bash
memorizer daemon start --register
```

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
