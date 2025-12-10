# Subsystems Documentation

This directory contains detailed technical documentation for each major subsystem of Agentic Memorizer. Each subsystem is documented in its own subdirectory with comprehensive information about architecture, implementation, usage, and troubleshooting.

## Purpose

The subsystem documentation provides:
- **Architecture Details** - Component design and interactions
- **Implementation Guide** - Key data structures, algorithms, and code locations
- **Integration Points** - How subsystems interact with each other
- **Operational Guide** - Configuration, monitoring, and troubleshooting
- **API Reference** - Public interfaces and usage patterns

## Audience

This documentation is intended for:
- **Contributors** - Understanding the codebase for development
- **Maintainers** - System administration and troubleshooting
- **Advanced Users** - Deep configuration and optimization

## Available Subsystems

**Table of Contents:**
- **Core Subsystems**
  - [Daemon](#daemon)
- **Index Management**
  - [Index Management](#index-management)
  - [File Watcher](#file-watcher)
- **File Processing**
  - [Metadata Extractor](#metadata-extractor)
  - [Semantic Analyzer](#semantic-analyzer)
- **Caching System**
  - [Cache Manager](#cache-manager)
- **Configuration**
  - [Config Manager](#config-manager)
- **Integration Framework**
  - [Integration Registry](#integration-registry)
- **Knowledge Graph**
  - [FalkorDB Graph](#falkordb-graph)
- **External Integration**
  - [MCP Server](#mcp-server)
  - [Semantic Search](#semantic-search)
- **Utilities**
  - [Walker](#walker)
  - [Format](#format)
  - [Version](#version)
- **Testing**
  - [E2E Tests](#e2e-tests)

---

### Core Subsystems

#### [Daemon](./daemon/)
**Status:** ✅ Documented

The background indexing daemon that maintains a FalkorDB knowledge graph through continuous file monitoring.

**Key Features:**
- File system watching with fsnotify
- Parallel file processing with worker pools
- Real-time graph updates via FalkorDB
- Health monitoring and metrics
- System service integration (systemd/launchd)

**Primary Components:**
- `internal/daemon/daemon.go` - Core orchestrator
- `internal/daemon/worker_pool.go` - Parallel processing
- `internal/daemon/health.go` - Health monitoring
- `cmd/daemon/daemon.go` - Parent CLI command
- `cmd/daemon/subcommands/` - Daemon subcommands (start, stop, status, restart, logs, rebuild, systemctl, launchctl)

**See:** [daemon/README.md](./daemon/README.md)

---

### Index Management

#### [Index Management](./index-management/)
**Status:** ✅ Documented

Graph-native storage architecture with on-demand export capabilities for SessionStart hooks and external integrations.

**Key Features:**
- FalkorDB-backed persistent storage (nodes and relationships)
- Real-time graph updates via Graph Manager
- On-demand index export from live graph data
- Thread-safe graph operations with connection pooling
- GraphIndex generation for SessionStart hooks and read command
- No file-based persistence (fully graph-native)

**Primary Components:**
- `internal/graph/manager.go` - Graph CRUD operations and queries
- `internal/graph/export.go` - GraphIndex export from graph
- `pkg/types/types.go` - GraphIndex and FileEntry structures

**See:** [index-management/README.md](./index-management/README.md)

---

#### [File Watcher](./file-watcher/)
**Status:** ✅ Documented

Monitors the file system for changes using fsnotify with intelligent event debouncing.

**Key Features:**
- Real-time file change detection
- Event debouncing and batching
- Configuration hot-reload (debounce interval)
- Recursive directory watching
- Skip pattern support
- Automatic new directory monitoring

**Primary Components:**
- `internal/watcher/watcher.go` - File system watcher
- `internal/watcher/watcher_test.go` - Test suite

**See:** [file-watcher/README.md](./file-watcher/README.md)

---

### File Processing

#### [Metadata Extractor](./metadata-extractor/)
**Status:** ✅ Documented

Extracts file-specific metadata from various file types without AI analysis.

**Key Features:**
- Multi-format support (markdown, images, documents, code, etc.)
- Type-specific metadata extraction (word count, dimensions, page count)
- Extensible handler system
- Error handling and fallbacks

**Supported File Types:**
- Text: Markdown, plain text, JSON/YAML
- Code: Go, Python, JavaScript, TypeScript, etc.
- Images: PNG, JPG, GIF, WebP
- Documents: DOCX, PPTX, PDF
- Transcripts: VTT, SRT

**Primary Components:**
- `internal/metadata/extractor.go` - Handler orchestration
- `internal/metadata/markdown.go` - Markdown handler
- `internal/metadata/image.go` - Image handler
- `internal/metadata/docx.go` - DOCX handler
- `internal/metadata/pptx.go` - PPTX handler
- `internal/metadata/pdf.go` - PDF handler
- `internal/metadata/code.go` - Code file handler
- `internal/metadata/json.go` - JSON/YAML handler
- `internal/metadata/vtt.go` - VTT transcript handler

**See:** [metadata-extractor/README.md](./metadata-extractor/README.md)

---

#### [Semantic Analyzer](./semantic-analyzer/)
**Status:** ✅ Documented

AI-powered semantic understanding using the Claude API.

**Key Features:**
- Claude API integration
- Vision support for images
- Structured analysis output (summary, tags, topics, document type)
- Rate limiting
- Error handling and retries

**Primary Components:**
- `internal/semantic/analyzer.go` - Analysis orchestration
- `internal/semantic/client.go` - Claude API client

**See:** [semantic-analyzer/README.md](./semantic-analyzer/README.md)

---

### Caching System

#### [Cache Manager](./cache-manager/)
**Status:** ✅ Documented

Stores and retrieves semantic analysis results keyed by file content hash.

**Key Features:**
- Hash-based caching (SHA-256)
- File-based persistence
- Cache invalidation on content change
- Cache statistics tracking
- High cache hit rates (>95%)

**Primary Components:**
- `internal/cache/manager.go` - Cache operations

**See:** [cache-manager/README.md](./cache-manager/README.md)

---

### Configuration

#### [Config Manager](./config-manager/)
**Status:** ✅ Documented

Loads, validates, and manages application configuration.

**Key Features:**
- YAML configuration files
- Environment variable overrides
- Schema validation
- Path resolution and expansion
- Default value management
- Validation with actionable error messages

**Primary Components:**
- `internal/config/config.go` - Configuration loading
- `internal/config/types.go` - Configuration structures
- `internal/config/validate.go` - Validation logic
- `internal/config/constants.go` - Default values

**See:** [config-manager/README.md](./config-manager/README.md)

---

### Integration Framework

#### [Integration Registry](./integration-registry/)
**Status:** ✅ Documented

Framework-agnostic integration system for connecting with AI agent platforms.

**Key Features:**
- Pluggable adapter pattern
- Automatic framework detection
- Integration lifecycle management
- Thread-safe registry
- Output format processors

**Supported Integrations:**
- Claude Code Hook (claude-code-hook) - SessionStart hooks for context injection
- Claude Code MCP (claude-code-mcp) - MCP server for on-demand tools
- Gemini CLI Hook (gemini-cli-hook) - SessionStart hooks for context injection
- Gemini CLI MCP (gemini-cli-mcp) - MCP server for Google Gemini CLI
- Codex CLI MCP (codex-cli-mcp) - MCP server for Codex CLI

**Primary Components:**
- `internal/integrations/registry.go` - Integration registry
- `internal/integrations/interface.go` - Integration interface
- `internal/integrations/adapters/claude/` - Claude Code adapters (hook & MCP)
- `internal/integrations/adapters/gemini/` - Gemini CLI MCP adapter
- `internal/integrations/adapters/codex/` - Codex CLI MCP adapter
- `internal/integrations/output/` - Output processors (XML, Markdown, JSON)

**See:** [integration-registry/README.md](./integration-registry/README.md)

---

### Knowledge Graph

#### [FalkorDB Graph](./falkordb-graph/)
**Status:** ✅ Documented

FalkorDB-backed knowledge graph for persistent storage and relationship-based queries.

**Key Features:**
- Graph-based storage for files, tags, topics, entities
- Cypher query language for semantic search
- Relationship traversal for related file discovery
- Docker container management via CLI commands
- Graceful degradation when graph unavailable
- HTTP API for graph-powered queries

**Node Types:**
- File - Indexed files with metadata
- Tag - Semantic tags from analysis
- Topic - Key topics from content
- Entity - Named entities (people, organizations)
- Category - File categories (documents, code, images)

**Relationship Types:**
- HAS_TAG - File → Tag
- COVERS_TOPIC - File → Topic
- MENTIONS - File → Entity
- IN_CATEGORY - File → Category

**Primary Components:**
- `internal/graph/manager.go` - Connection management and health checks
- `internal/graph/queries.go` - Cypher query execution
- `internal/graph/schema.go` - Node/edge type definitions
- `internal/graph/exporter.go` - Index export from graph
- `internal/daemon/api/` - HTTP API handlers
- `cmd/graph/subcommands/` - CLI commands (start, stop, status)

**See:** [falkordb-graph/README.md](./falkordb-graph/README.md)

---

### External Integration

#### [MCP Server](./mcp/)
**Status:** ✅ Documented

Exposes the knowledge graph through the Model Context Protocol (MCP) as a standardized server interface for universal integration with AI development tools.

**Key Features:**
- JSON-RPC 2.0 protocol implementation
- Static context delivery via GraphIndex export in multiple formats (XML, Markdown, JSON)
- Dynamic graph-powered semantic search across knowledge base
- Metadata retrieval and time-based filtering via daemon HTTP API
- Graph query tools (search, related files, entity search, recent files)
- Support for Claude Code, Gemini CLI, Codex CLI, and any MCP-compliant client

**Primary Components:**
- `internal/mcp/server.go` - MCP server orchestrator
- `internal/mcp/protocol/` - JSON-RPC message types and protocol definitions
- `internal/mcp/transport/` - Transport abstraction (stdio implementation)
- `cmd/mcp/` - CLI command for running the MCP server

**See:** [mcp/README.md](./mcp/README.md)

---

#### [Semantic Search](./semantic-search/)
**Status:** ✅ Documented

Provides graph-powered, relationship-aware search capabilities with automatic fallback to in-memory search.

**Key Features:**
- **Primary Mode**: Graph-based search using Cypher queries against FalkorDB
  - Multi-signal search across filenames, tags, topics, entities, and summaries
  - Relationship traversal (HAS_TAG, COVERS_TOPIC, MENTIONS edges)
  - Related files discovery through shared connections
  - Entity search with normalized name matching
  - Vector similarity search (when embeddings enabled)
  - Full-text search on summaries
- **Fallback Mode**: Token-based in-memory search when graph unavailable
  - Weighted proportional scoring across seven fields
  - Stop word filtering and case-insensitive matching
- Graceful degradation with automatic mode switching
- Category filtering and configurable result limits

**Primary Components:**
- `internal/graph/manager.go` - Graph-powered search queries
- `internal/search/semantic.go` - Fallback in-memory searcher
- `internal/daemon/api/` - HTTP API search endpoints

**See:** [semantic-search/README.md](./semantic-search/README.md)

---

### Utilities

#### [Walker](./walker/)
**Status:** ✅ Documented

Directory tree traversal with filtering and relative path computation.

**Key Features:**
- Recursive directory walking
- Skip pattern support
- Relative path computation
- Callback-based processing

**Primary Components:**
- `internal/walker/walker.go` - Directory traversal

**See:** [walker/README.md](./walker/README.md)

---

#### [Format](./format/)
**Status:** ✅ Documented

Centralized CLI output formatting with multi-format support and structured builders.

**Key Features:**
- Three-tier architecture (builders, formatters, writers)
- Seven builder types for common CLI output patterns (status, section, table, list, progress, error, graph content)
- Five output formats (text, JSON, YAML, XML, markdown)
- Thread-safe formatter registry with extensible registration
- Consistent styling with shared utilities (symbols, colors, number formatting)
- Type-safe validation before rendering

**Primary Components:**
- `internal/format/builder.go` - Buildable interface and builder types
- `internal/format/formatter.go` - Formatter interface and registry
- `internal/format/writer.go` - Buffered I/O abstraction
- `internal/format/status.go` - Status message builder
- `internal/format/section.go` - Section builder with hierarchical key-value pairs
- `internal/format/table.go` - Table builder with alignment support
- `internal/format/list.go` - List builder with nesting
- `internal/format/progress.go` - Progress indicator builder
- `internal/format/error.go` - Structured error message builder
- `internal/format/graph.go` - GraphContent builder for integration output
- `internal/format/utils.go` - Shared formatting utilities
- `internal/format/formatters/` - Format-specific renderers (text, JSON, YAML, XML, markdown)

**See:** [format/README.md](./format/README.md)

---

#### [Version](./version/)
**Status:** ✅ Documented

Version information management for build-time metadata.

**Key Features:**
- Version string
- Git commit hash
- Build date
- Build-time variable injection

**Primary Components:**
- `internal/version/version.go` - Version information

**See:** [version/README.md](./version/README.md)

---

### Testing

#### [E2E Tests](./e2e-tests/)
**Status:** ✅ Documented

Comprehensive end-to-end testing framework for validating complete workflows across the full application stack.

**Key Features:**
- Isolated test environments with temporary directories
- Test harness framework for daemon management
- HTTP, MCP, and graph client abstractions
- Automatic cleanup and resource management
- Build tag separation from unit tests (`//go:build e2e`)
- Docker integration for FalkorDB testing

**Test Coverage:**
- CLI command execution and output validation
- Daemon lifecycle management
- File system watching and processing pipelines
- HTTP API endpoints and SSE notifications
- Graph database operations and queries
- Configuration validation and hot-reload
- Integration framework setup and teardown
- Error handling and edge cases

**Primary Components:**
- `e2e/harness/` - Test harness framework
- `e2e/tests/` - Test suites (18 test files)
- `e2e/fixtures/` - Test data and fixtures

**See:** [e2e-tests/README.md](./e2e-tests/README.md)

---

## Subsystem Interactions

```
┌──────────────────────────────────────────────────────────────────────┐
│                        CLI Commands (cmd/)                           │
│  ┌──────┐ ┌────────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────────────┐     │
│  │ init │ │ daemon │ │ read │ │ mcp  │ │graph │ │ integrations │     │
│  └───┬──┘ └───┬────┘ └───┬──┘ └───┬──┘ └───┬──┘ └──────┬───────┘     │
└──────┼────────┼──────────┼────────┼────────┼───────────┼─────────────┘
       │        │          │        │        │           │
       v        v          v        v        v           v
┌──────────────────────────────────────────────────────────────────────┐
│                 Configuration (internal/config)                      │
│            Loads settings, validates, manages paths                  │
└──────────────────────────────┬───────────────────────────────────────┘
                               │
        ┌──────────────────────┼─────────────────────────────┐
        │                      │                             │
        v                      v                             v
┌──────────────┐        ┌──────────────────┐       ┌──────────────────┐
│   Daemon     │───────>│  Graph Manager   │       │   Integration    │
│  Subsystem   │        │  (FalkorDB ops)  │       │    Registry      │
└──────┬───────┘        └────────┬─────────┘       └────────┬─────────┘
       │                         │                          │
       │                ┌────────┼───────┐                  │
   ┌───┴───────┐        │                │                  │
   │           │        │                │                  │
   v           v        v                v                  v
┌────────┐ ┌──────┐ ┌───────┐  ┌──────────────────┐  ┌──────────────┐
│Watcher │ │Worker│ │ Read  │  │   MCP Server     │  │   Output     │
│        │ │ Pool │ │Command│  │                  │  │  Processors  │
└────────┘ └───┬──┘ └───┬───┘  └─────────┬────────┘  └──────────────┘
               │        │                │
       ┌───────┴────┐   │                │
       │            │   │                │
       v            v   v                v
  ┌─────────┐ ┌──────────┐       ┌───────────────┐
  │Metadata │ │ Semantic │       │  Daemon HTTP  │
  │Extractor│ │ Analyzer │       │     API       │
  └────┬────┘ └────┬─────┘       └───────┬───────┘
       │           │                     │
       │           v                     │
       │      ┌─────────┐                │
       │      │ Claude  │                │
       │      │   API   │                │
       │      └─────────┘                │
       │           │                     │
       └───────┬───┘                     │
               v                         v
        ┌────────────┐           ┌──────────────┐
        │   Cache    │           │   FalkorDB   │
        │  Manager   │           │    Graph     │
        └────────────┘           └──────┬───────┘
                                        │
                                        v
                                 ┌──────────────┐
                                 │ External MCP │
                                 │   Clients    │
                                 └──────────────┘
```

## Documentation Standards

Each subsystem documentation **MUST** include:

1. **Table of Contents**: An organized list of the main sections and subsections within the documentation for easy navigation.
2. **Overview**: A brief introduction to the subsystem, its purpose, and its role within the larger system.
3. **Design Principles**: An explanation of the core design principles and architectural patterns that guide the development of the subsystem.
4. **Key Components**: A high-level description of the main components or modules that make up the subsystem.
5. **Integration Points**: An outline of how the subsystem integrates with other subsystems or components within the codebase.
6. **Glossary**: Definitions of any specialized terms or concepts relevant to the subsystem.

## Contributing Documentation

When adding or updating subsystem documentation:

1. **Create Subdirectory**
   ```bash
   mkdir -p docs/subsystems/your-subsystem
   ```

2. **Write README.md**
   Use the daemon subsystem as a template.

3. **Update This Index**
   Add your subsystem to the "Available Subsystems" section above.

4. **Follow Standards**
   Ensure all required sections are included.

5. **Keep It Updated**
   Update documentation when code changes.

## Documentation Checklist

Before marking subsystem documentation as complete, verify:

- [ ] All required sections included
- [ ] Technical accuracy verified
- [ ] Professional tone maintained

## Related Documentation

### Project Documentation
- [README](../../README.md) - User-facing documentation
- [CHANGELOG](../../CHANGELOG.md) - Version history

## Getting Help

If you're looking for specific subsystem information:

1. **Check this index** for available documentation
2. **Read the code** in `internal/`
3. **Ask questions** via GitHub issues
4. **Contribute docs** for undocumented subsystems

## Documentation Status Legend

- ✅ **Documented** - Complete documentation available
- 🚧 **Planned** - Documentation planned but not yet written
- 📝 **Draft** - Documentation in progress
- 🔄 **Needs Update** - Documentation exists but may be outdated

---

**Last Updated:** 2025-12-06

**Recent Updates:**
- Added Format subsystem documentation (2025-12-06)
- Comprehensive subsystems index accuracy review (2025-12-05)
  - Corrected Daemon description: "maintains FalkorDB knowledge graph" (not "precomputed memory index")
  - Updated Daemon key features: "Real-time graph updates via FalkorDB" (not "Atomic index updates")
  - Completely rewrote Index Management section to reflect graph-native architecture:
    - Changed title from "Index Manager" to "Index Management" throughout
    - Updated description: "Graph-native storage with on-demand export" (not "precomputed index file with atomic writes")
    - Replaced key features to reflect FalkorDB storage, GraphIndex export, no file persistence
    - Updated primary components: `internal/graph/manager.go` and `export.go` (not `internal/index/`)
  - Updated MCP Server description: "Exposes knowledge graph" (not "precomputed index")
  - Enhanced MCP Server key features to clarify GraphIndex export and daemon HTTP API integration
  - Updated Subsystem Interactions diagram: "Graph Manager (FalkorDB ops)" replacing "Index Manager (atomic I/O)"
  - Added missing daemon subcommands: systemctl, launchctl
  - Fixed broken link: index-manager → index-management
  - Added E2E Tests subsystem to index with comprehensive description
  - Enhanced Integration Registry: added Gemini CLI MCP and Codex CLI MCP to supported integrations
  - Updated Semantic Search description to reflect graph-powered architecture with in-memory fallback
- Version subsystem documentation accuracy review (2025-12-05)
  - Updated version references from 0.11.0 to 0.12.1
  - Added MCP Server integration point
  - Enhanced version command documentation
- Added FalkorDB Graph subsystem documentation (2025-11-30)
- Updated architecture diagram to include graph subsystem
- Comprehensive accuracy review of all subsystem documentation (2025-11-22)
- 47 inaccuracies corrected across 9 subsystems
- MCP Server documentation completed and verified
