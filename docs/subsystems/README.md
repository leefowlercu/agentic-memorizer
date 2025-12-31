# Subsystems Documentation

This directory contains detailed technical documentation for each major subsystem of Agentic Memorizer. Each subsystem is documented in its own subdirectory with comprehensive information about architecture, design principles, key components, and integration points.

**Last Updated:** 2025-12-31

## Purpose

The subsystem documentation provides:

- **Overview** - A brief, high-level description of the subsystem, its purpose, goals, and key features
- **Design Principles** - Core design principles and architectural patterns that guide development
- **Key Components** - High-level description of the main components or modules
- **Integration Points** - How the subsystem integrates with other subsystems or components
- **Glossary** - Definitions of specialized terms or concepts relevant to the subsystem

## Available Subsystems

**Table of Contents:**

- **Core**
  - [CLI](#cli)
  - [Daemon](#daemon)
  - [Fileops](#fileops)
  - [Skip](#skip)
  - [Walker](#walker)
  - [Watcher](#watcher)
- **Processing**
  - [Document](#document)
  - [Embeddings](#embeddings)
  - [Metadata](#metadata)
  - [Semantic](#semantic)
- **Storage**
  - [Cache](#cache)
  - [Graph](#graph)
- **Configuration**
  - [Config](#config)
- **Observability**
  - [Logging](#logging)
- **Integration**
  - [Integrations](#integrations)
  - [MCP](#mcp)
- **Output**
  - [Format](#format)
- **User Interface**
  - [TUI](#tui)
- **Build**
  - [Version](#version)
- **Infrastructure**
  - [Container](#container)
  - [Servicemanager](#servicemanager)
- **Testing**
  - [E2E](#e2e)

---

### [CLI](./cli/)

**Status:** ✅ Documented

Cobra-based CLI with hierarchical command structure, input validation via PreRunE hooks, and consistent output formatting for daemon management, integration setup, file management, and memory operations.

**Key Features:**

- Eleven parent commands with subcommands organized by functional area
- PreRunE input validation distinguishing user errors from runtime errors
- Consistent output through format subsystem builders
- File management with copy/move, conflict resolution, and batch processing
- Interactive TUI wizard and unattended scripted modes for initialization
- Shared config loading and output helpers
- Integration auto-detection for binary path resolution

**Primary Components:**

- `cmd/root.go` - Root command, Execute function, and global PersistentPreRunE
- `cmd/initialize/initialize.go` - Interactive and unattended setup modes
- `cmd/daemon/` - Daemon lifecycle: start, stop, status, restart, rebuild, logs
- `cmd/integrations/` - Integration management: list, detect, setup, remove, health
- `cmd/remember/` - Memory addition: file (copy to memory), fact (store in graph)
- `cmd/forget/` - Memory removal: file (move to .forgotten), fact (delete from graph)
- `cmd/read/` - Memory access: files, facts
- `cmd/shared/` - Common config and output helpers

**See:** [cli/README.md](./cli/README.md)

---

### [Daemon](./daemon/)

**Status:** ✅ Documented

Background file monitoring and knowledge graph maintenance with parallel processing, hot-reload, and HTTP API.

**Key Features:**

- Background file monitoring with real-time change detection via fsnotify
- Parallel file processing through configurable worker pool with rate limiting
- FalkorDB knowledge graph maintenance for files, tags, topics, entities, and facts
- Configuration hot-reload via SIGHUP without daemon restart
- Unified HTTP API with query parameters for search, filtering, and facts
- Server-Sent Events for real-time client notifications
- External process supervision integration (systemd, launchd)

**Primary Components:**

- `internal/daemon/daemon.go` - Core orchestrator and lifecycle management
- `internal/daemon/worker/pool.go` - Parallel processing with rate limiting
- `internal/daemon/api/server.go` - Unified HTTP API server
- `internal/daemon/api/sse.go` - Server-Sent Events hub
- `internal/daemon/health.go` - Health metrics tracking
- `internal/daemon/signals.go` - Signal handling (SIGINT, SIGHUP, SIGUSR1)

**See:** [daemon/README.md](./daemon/README.md)

---

### [Fileops](./fileops/)

**Status:** ✅ Documented

Filesystem utilities for copying and moving files with automatic conflict resolution, cross-filesystem support, and batch operations.

**Key Features:**

- Copy and move operations for files and directories with recursive support
- Automatic conflict resolution via `-N` suffix pattern (file.md → file-1.md)
- Cross-filesystem move fallback using copy+delete when rename fails
- Batch processing with continue-on-failure semantics
- Path validation to prevent directory traversal attacks
- Permission preservation for copied files

**Primary Components:**

- `internal/fileops/copy.go` - Copy, CopyBatch, CopyToDir with CopyResult tracking
- `internal/fileops/move.go` - Move, MoveBatch, MoveToDir with cross-filesystem fallback
- `internal/fileops/conflict.go` - ResolveConflict with compound extension handling
- `internal/fileops/paths.go` - IsInDirectory, ValidatePath, EnsureDir, PathExists utilities

**See:** [fileops/README.md](./fileops/README.md)

---

### [Skip](./skip/)

**Status:** ✅ Documented

Configurable file and directory filtering for consistent skip behavior across the walker and watcher subsystems.

**Key Features:**

- Two-tier filtering with hardcoded always-skip directories (.git, .cache, .forgotten)
- Configurable hidden file handling via SkipHidden toggle
- Directory, file name, and extension-based pattern matching
- Unified ShouldSkip interface for files and directories
- Consistent behavior across walker and watcher subsystems

**Primary Components:**

- `internal/skip/skip.go` - Config struct, AlwaysSkipDirs, ShouldSkipDir, ShouldSkipFile, ShouldSkip functions

**See:** [skip/README.md](./skip/README.md)

---

### [Walker](./walker/)

**Status:** ✅ Documented

Recursive directory scanning with configurable filtering for full directory scans during initialization and rebuilds.

**Key Features:**

- Recursive traversal of directory trees using filepath.Walk
- Directory pruning to skip entire subtrees (e.g., node_modules, .git)
- File filtering by exact name or extension match
- Automatic exclusion of hidden files and directories (dot-prefixed)
- Visitor pattern for flexible file processing during traversal
- Error tolerance continuing after access errors with warnings

**Primary Components:**

- `internal/walker/walker.go` - Walk function with filtering and FileVisitor type

**See:** [walker/README.md](./walker/README.md)

---

### [Watcher](./watcher/)

**Status:** ✅ Documented

Real-time filesystem monitoring with debounced event batching and intelligent filtering.

**Key Features:**

- Real-time detection of Create, Modify, Delete, and Rename events via fsnotify
- Debounced event batching to coalesce rapid changes within configurable window
- Event priority system ensuring correct final state (DELETE > CREATE > MODIFY)
- Recursive directory watching with automatic subdirectory registration
- Two-tier filtering for directories and files with hidden path exclusion
- Hot-reload support for debounce interval updates without restart

**Primary Components:**

- `internal/watcher/watcher.go` - Core watcher with fsnotify integration and event batching

**See:** [watcher/README.md](./watcher/README.md)

---

### [Document](./document/)

**Status:** ✅ Documented

Shared utilities for Microsoft Office file extraction including ZIP archive handling, XML text extraction, and format-specific metadata parsing.

**Key Features:**

- ZIP archive handling for Office Open XML format files
- XML text extraction from Word (`<w:t>`) and PowerPoint (`<a:t>`) text runs
- DOCX processing for word count and author extraction
- PPTX processing for slide counting and text aggregation
- Core properties parsing from docProps/core.xml

**Primary Components:**

- `internal/document/office.go` - Shared ZIP and XML utilities
- `internal/document/docx.go` - Word document text and metadata extraction
- `internal/document/pptx.go` - PowerPoint text and metadata extraction

**See:** [document/README.md](./document/README.md)

---

### [Embeddings](./embeddings/)

**Status:** ✅ Documented

Provider-based text embedding generation with content-addressable caching for semantic similarity search in the knowledge graph.

**Key Features:**

- Provider interface pattern enabling future multi-provider support
- OpenAI integration using text-embedding-3-small (1536 dimensions)
- Binary caching with efficient float32 serialization
- Batch processing for multi-text embedding in single API call
- Separate rate limiter (500 RPM) for embedding API calls
- HNSW vector index in FalkorDB for similarity search

**Primary Components:**

- `internal/embeddings/provider.go` - Provider interface and EmbeddingResult struct
- `internal/embeddings/openai.go` - OpenAI embedding provider implementation
- `internal/embeddings/cache.go` - Content-addressable embedding cache with binary format

**See:** [embeddings/README.md](./embeddings/README.md)

---

### [Metadata](./metadata/)

**Status:** ✅ Documented

Fast, deterministic file metadata extraction with handler pattern for 9 file type categories and 26+ file extensions.

**Key Features:**

- 9 file categories: documents, presentations, images, transcripts, data, code, videos, audio, archives
- Handler/adapter pattern with FileHandler interface for type-specific extraction
- Registry pattern with O(1) extension-to-handler lookup via hash map
- Graceful degradation returning base metadata when handlers fail
- Readability classification for Claude Code direct access indication
- Content hash computation for cache key generation

**Primary Components:**

- `internal/metadata/extractor.go` - Main orchestration, registry, and public API
- `internal/metadata/markdown.go` - Markdown word count and section extraction
- `internal/metadata/code.go` - Programming language detection and line counting
- `internal/metadata/image.go` - Image dimension extraction via DecodeConfig
- `internal/metadata/docx.go` - DOCX word count and author extraction
- `internal/metadata/pptx.go` - PPTX slide count and author extraction
- `internal/document/` - Shared Office file utilities for ZIP/XML handling

**See:** [metadata/README.md](./metadata/README.md)

---

### [Semantic](./semantic/)

**Status:** ✅ Documented

Multi-provider AI-powered content understanding with intelligent content routing, shared prompts, and graceful fallbacks.

**Key Features:**

- Multi-provider support for Claude, OpenAI, and Gemini with provider-specific optimizations
- Provider interface with Analyze, SupportsVision, and SupportsDocuments capability detection
- Content routing based on file type: text analysis, vision API, document blocks, text extraction
- Shared prompt templates for consistent output structure across providers
- Native PDF handling via document blocks (Claude) and multimodal blobs (Gemini)
- Graceful fallback producing metadata-only analysis with 0.5 confidence
- Rate limiting integration respecting provider-specific API quotas

**Primary Components:**

- `internal/semantic/provider.go` - Provider interface definition
- `internal/semantic/registry.go` - Thread-safe singleton registry with factory pattern
- `internal/semantic/providers/claude/` - Claude provider with native document blocks
- `internal/semantic/providers/openai/` - OpenAI provider with go-openai client
- `internal/semantic/providers/gemini/` - Gemini provider with native multimodal support
- `internal/semantic/common/` - Shared prompts, response parsing, media types, fallback logic

**See:** [semantic/README.md](./semantic/README.md)

---

### [Cache](./cache/)

**Status:** ✅ Documented

Content-addressable caching for semantic analysis results with three-tier versioning, provider isolation, and directory sharding.

**Key Features:**

- SHA-256 content-addressable storage enabling cache hits across file renames and moves
- Three-tier versioning (schema, metadata, semantic) for intelligent cache invalidation
- Provider isolation with separate subdirectories for Claude, OpenAI, and Gemini
- Two-level directory sharding for filesystem performance at scale
- Forward compatibility for rollback scenarios
- Cache statistics and stale entry cleanup

**Primary Components:**

- `internal/cache/manager.go` - Core cache operations (Get, Set, Clear, Stats)
- `internal/cache/version.go` - Version management and staleness detection
- `internal/cache/shard.go` - Shared directory sharding utilities

**See:** [cache/README.md](./cache/README.md)

---

### [Graph](./graph/)

**Status:** ✅ Documented

FalkorDB-powered knowledge graph for files, semantic relationships, and intelligent discovery.

**Key Features:**

- Persistent storage of files, tags, topics, entities, categories, and directories in FalkorDB
- HNSW vector indexes for embedding-based semantic similarity search
- Entity disambiguation with 100+ built-in alias mappings and automatic normalization
- Knowledge analytics including clustering, recommendations, temporal analysis, and gap detection
- User facts CRUD for persistent context injection into AI conversations
- Full-text search on summaries plus relationship-based discovery

**Primary Components:**

- `internal/graph/manager.go` - Facade orchestrating all graph operations
- `internal/graph/client.go` - FalkorDB connection and query execution
- `internal/graph/schema.go` - Node labels, edge types, and index definitions
- `internal/graph/nodes.go` - Node CRUD operations with MERGE-based upserts
- `internal/graph/edges.go` - Relationship creation with entity normalization
- `internal/graph/queries.go` - Search and traversal operations
- `internal/graph/facts.go` - User facts storage and retrieval
- `internal/graph/export.go` - Data transformation for output formats

**See:** [graph/README.md](./graph/README.md)

---

### [Config](./config/)

**Status:** ✅ Documented

Layered configuration management with YAML files, environment variable overrides, validation, and hot-reload support.

**Key Features:**

- Layered configuration with clear precedence (defaults, YAML, environment variables)
- Hierarchical structure with memory.root and daemon skip patterns
- Three-tier settings (minimal, advanced, hardcoded) for appropriate user exposure
- Error accumulation with actionable suggestions during validation
- Hot-reload support for non-structural configuration changes
- Path safety validation preventing directory traversal attacks
- Automatic credential resolution from provider-specific environment variables

**Primary Components:**

- `internal/config/config.go` - Configuration loading and initialization
- `internal/config/types.go` - Configuration structure definitions
- `internal/config/validate.go` - Validation with error accumulation
- `internal/config/reload.go` - Hot-reload compatibility checking
- `internal/config/constants.go` - Default values and hardcoded settings

**See:** [config/README.md](./config/README.md)

---

### [Logging](./logging/)

**Status:** ✅ Documented

Centralized logging infrastructure with slog integration, file rotation, context propagation, and standardized field names for consistent observability.

**Key Features:**

- Logger factory with functional options pattern for flexible configuration
- Automatic file rotation via lumberjack with size, backup, and age limits
- JSON and text handler types for machine and human readability
- Standardized field names aligned with OpenTelemetry conventions
- Context-based logger propagation through request handling
- UUIDv7 identifiers for time-ordered process, session, and client correlation

**Primary Components:**

- `internal/logging/factory.go` - Logger factory with functional options
- `internal/logging/rotation.go` - Lumberjack rotation configuration
- `internal/logging/fields.go` - Standardized field name constants
- `internal/logging/identifiers.go` - UUIDv7 identifier generation
- `internal/logging/context.go` - Context-based logger propagation
- `internal/logging/logging.go` - Logger enrichment helpers

**See:** [logging/README.md](./logging/README.md)

---

### [Integrations](./integrations/)

**Status:** ✅ Documented

Framework-agnostic integration system with dual-hook architecture, MCP servers, and adapter pattern for Claude Code, Gemini CLI, and Codex CLI.

**Key Features:**

- Adapter pattern with common Integration interface and specialized implementations
- Thread-safe registry with automatic registration via init functions
- Dual-hook architecture: SessionStart for files, UserPromptSubmit/BeforeAgent for facts
- MCP server integration for on-demand tools (search, metadata, related files)
- Transactional setup with rollback on failure and configuration preservation
- Support for JSON (Claude/Gemini) and TOML (Codex) configuration formats

**Primary Components:**

- `internal/integrations/interface.go` - Integration interface with 13 lifecycle methods
- `internal/integrations/registry.go` - Thread-safe singleton registry
- `internal/integrations/adapters/claude_code/` - Claude Code hook and MCP adapters
- `internal/integrations/adapters/gemini_cli/` - Gemini CLI hook and MCP adapters
- `internal/integrations/adapters/codex_cli/` - Codex CLI MCP adapter (hooks not supported)

**See:** [integrations/README.md](./integrations/README.md)

---

### [MCP](./mcp/)

**Status:** ✅ Documented

Model Context Protocol implementation with JSON-RPC 2.0 messaging, stdio transport, and graph-powered tools for AI assistant integration.

**Key Features:**

- JSON-RPC 2.0 protocol with standard error codes and MCP-specific extensions
- Stdio transport with line-delimited JSON for subprocess communication
- Five graph-powered tools using unified daemon API with query parameters
- Three resources: file index in XML, JSON, and Markdown formats with subscription support
- Three built-in prompts: analyze-file, search-context, explain-summary
- Partial fallback for two tools (get_file_metadata, list_recent_files) when daemon unavailable
- Real-time updates via SSE client for subscribed resource notifications

**Primary Components:**

- `internal/mcp/server.go` - Main server orchestrator and message routing
- `internal/mcp/transport/stdio.go` - Line-delimited JSON over stdin/stdout
- `internal/mcp/protocol/` - JSON-RPC 2.0 message types and MCP capabilities
- `internal/mcp/handlers/` - Tool implementations with daemon API integration
- `internal/mcp/sse_client.go` - Real-time index updates from daemon
- `internal/mcp/subscriptions.go` - Thread-safe resource subscription tracking
- `internal/mcp/prompts.go` - Prompt registry and message generation

**See:** [mcp/README.md](./mcp/README.md)

---

### [Format](./format/)

**Status:** ✅ Documented

Structured CLI output with multiple format support through a builder pattern and pluggable formatters.

**Key Features:**

- Builder pattern with fluent API for constructing structured content with validation
- Five output formats: text (with ANSI colors), JSON, YAML, XML, and Markdown
- Status messaging with six severity levels and consistent symbols/colors
- Hierarchical sections with key-value pairs and subsections up to 5 levels deep
- Table formatting with alignment control, compact mode, and header hiding
- Thread-safe formatter registry with auto-registration via init functions

**Primary Components:**

- `internal/format/formatter.go` - Formatter interface and thread-safe registry
- `internal/format/builder.go` - Buildable interface and type definitions
- `internal/format/section.go` - Hierarchical key-value builder
- `internal/format/table.go` - Columnar data builder with alignment
- `internal/format/status.go` - Severity-based status message builder
- `internal/format/formatters/text.go` - Text formatter with ANSI color support
- `internal/format/formatters/json.go` - JSON formatter
- `internal/format/formatters/yaml.go` - YAML formatter

**See:** [format/README.md](./format/README.md)

---

### [TUI](./tui/)

**Status:** ✅ Documented

Interactive setup wizard built on Bubble Tea with multi-step navigation, reusable components, and consistent styling for guided configuration.

**Key Features:**

- Seven-step configuration wizard with forward/backward navigation
- Bubble Tea integration for event-driven terminal UI
- Reusable components (RadioGroup, TextInput, Checkbox, Progress)
- Centralized styling via lipgloss color palette and theme
- Environment detection for API keys, services, and integrations
- Full-screen alternate buffer for clean terminal experience

**Primary Components:**

- `internal/tui/initialize/wizard.go` - Wizard orchestrator implementing tea.Model
- `internal/tui/initialize/steps/step.go` - Step interface definition
- `internal/tui/initialize/steps/*.go` - Individual step implementations
- `internal/tui/initialize/components/*.go` - Reusable UI components
- `internal/tui/styles/styles.go` - Centralized color palette and styling

**See:** [tui/README.md](./tui/README.md)

---

### [Version](./version/)

**Status:** ✅ Documented

Build-time version injection with embedded fallback and Go build info integration.

**Key Features:**

- Ldflags injection for version, commit, and build date at compile time
- Embedded VERSION file fallback via go:embed directive
- Go runtime/debug build info extraction for commit and date
- Dirty state detection for uncommitted workspace changes
- Short commit hash truncation (7 characters) for display
- Formatted output combining all version components

**Primary Components:**

- `internal/version/version.go` - Version getters with source priority and formatting
- `internal/version/VERSION` - Embedded version number file

**See:** [version/README.md](./version/README.md)

---

### [Container](./container/)

**Status:** ✅ Documented

Container runtime abstraction for FalkorDB lifecycle management supporting both Docker and Podman with availability detection and readiness polling.

**Key Features:**

- Multi-runtime support for Docker and Podman with runtime-specific configurations
- Runtime detection via `docker info` and `podman info` commands
- Container state inspection for running and existence status
- Container lifecycle operations with timeout protection
- Readiness polling via Redis PING until FalkorDB responds
- Podman uses `--network=host` for reliable localhost connectivity
- Persistent storage support via bind-mounted data directories

**Primary Components:**

- `internal/container/container.go` - Runtime abstraction with Docker/Podman support

**See:** [container/README.md](./container/README.md)

---

### [Servicemanager](./servicemanager/)

**Status:** ✅ Documented

Platform-specific service integration for systemd (Linux) and launchd (macOS) enabling automatic daemon startup and system-level supervision.

**Key Features:**

- Platform detection for Linux (systemd) and macOS (launchd)
- Binary path resolution via executable, common paths, and PATH lookup
- User-level and system-level service installation options
- Security hardening (NoNewPrivileges, PrivateTmp for systemd)
- Automatic restart on failure with configurable delays
- Complete installation instructions for each platform and mode

**Primary Components:**

- `internal/servicemanager/servicemanager.go` - Platform detection and binary path resolution
- `internal/servicemanager/systemd.go` - Systemd unit file generation with user/system modes
- `internal/servicemanager/launchd.go` - Launchd plist generation with agent/daemon modes

**See:** [servicemanager/README.md](./servicemanager/README.md)

---

### [E2E](./e2e/)

**Status:** ✅ Documented

Comprehensive integration testing with isolated environments, Docker-based FalkorDB, and full-stack validation.

**Key Features:**

- Environment isolation with unique temp directories and graph namespaces per test
- Multi-layer client abstractions for HTTP API, MCP protocol, and FalkorDB queries
- LIFO cleanup stack ensuring resource release even on test failure
- Docker Compose infrastructure with FalkorDB health checks and volume cleanup
- 25 test suites covering CLI, daemon, graph, HTTP API, MCP, integrations, and more
- Build tag separation (`//go:build e2e`) from unit tests

**Primary Components:**

- `e2e/harness/harness.go` - Core orchestrator for test environment setup and teardown
- `e2e/harness/cleanup.go` - LIFO cleanup stack with graceful daemon shutdown
- `e2e/harness/http_client.go` - Daemon HTTP API client with timeout and retry
- `e2e/harness/mcp_client.go` - JSON-RPC 2.0 MCP protocol client via stdio
- `e2e/harness/graph_client.go` - Direct FalkorDB Cypher query execution
- `e2e/harness/assertions.go` - Test-specific assertion helpers

**See:** [e2e/README.md](./e2e/README.md)

---

## Documentation Status Legend

- ✅ **Documented** - Complete documentation available
- 🚧 **Planned** - Documentation planned but not yet written
- 📝 **Draft** - Documentation in progress
- 🔄 **Needs Update** - Documentation exists but may be outdated

---

**Recent Updates:**

- Reconciled all subsystems to v0.14.0 for release (2025-12-31)
- Updated e2e documentation - 25 test files (was 21), added new test files (2025-12-31)
- Updated format documentation - removed markdown formatter (2025-12-31)
- Updated graph documentation - multi-provider embedding indexes (2025-12-31)
- Updated cli subsystem documentation - removed systemctl/launchctl, corrected command count (2025-12-31)
- Updated cache subsystem documentation - directory sharding, shared shard utilities (2025-12-31)
- Renamed docker to container subsystem with Docker/Podman support (2025-12-30)
- Created skip subsystem documentation (2025-12-29)
- Created servicemanager subsystem documentation (2025-12-29)
- Created fileops subsystem documentation (2025-12-29)
- Updated daemon subsystem documentation - unified HTTP API endpoints, facts API (2025-12-29)
- Updated config subsystem documentation - memory.root, daemon skip patterns (2025-12-29)
- Updated mcp subsystem documentation - unified daemon API endpoints (2025-12-29)
- Updated cli subsystem documentation - added remember/forget file commands (2025-12-29)
- Created cli subsystem documentation (2025-12-29)
- Created tui subsystem documentation (2025-12-29)
- Created logging subsystem documentation (2025-12-29)
- Created embeddings subsystem documentation (2025-12-29)
- Created document subsystem documentation (2025-12-29)
- Created container subsystem documentation (2025-12-29)
- Created version subsystem documentation (2025-12-29)
- Created walker subsystem documentation (2025-12-29)
- Created semantic subsystem documentation (2025-12-29)
- Created metadata subsystem documentation (2025-12-29)
- Created mcp subsystem documentation (2025-12-29)
- Created integrations subsystem documentation (2025-12-29)
- Created format subsystem documentation (2025-12-29)
- Created watcher subsystem documentation (2025-12-29)
- Created graph subsystem documentation (2025-12-29)
- Created e2e subsystem documentation (2025-12-29)
- Created daemon subsystem documentation (2025-12-29)
- Created config subsystem documentation (2025-12-29)
- Created cache subsystem documentation (2025-12-29)
- Created subsystems documentation index (2025-12-29)
