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

### Core Subsystems

#### [Daemon](./daemon/)
**Status:** ✅ Documented

The background indexing daemon that maintains a precomputed memory index through continuous file monitoring.

**Key Features:**
- File system watching with fsnotify
- Parallel file processing with worker pools
- Atomic index updates
- Health monitoring and metrics
- System service integration (systemd/launchd)

**Primary Components:**
- `internal/daemon/daemon.go` - Core orchestrator
- `internal/daemon/worker_pool.go` - Parallel processing
- `internal/daemon/health.go` - Health monitoring
- `cmd/daemon/` - CLI commands

**See:** [daemon/README.md](./daemon/README.md)

---

### Index Management

#### [Index Manager](./index-manager/)
**Status:** ✅ Documented

Manages the precomputed index file with thread-safe operations and atomic writes.

**Key Features:**
- Thread-safe read/write operations
- Atomic file updates (temp + rename)
- Incremental index updates
- Corruption recovery
- Index versioning

**Primary Components:**
- `internal/index/manager.go` - Index management
- `internal/index/computed.go` - Index structure
- `pkg/types/types.go` - Type definitions

**See:** [index-manager/README.md](./index-manager/README.md)

---

#### [File Watcher](./file-watcher/)
**Status:** ✅ Documented

Monitors the file system for changes using fsnotify with intelligent event debouncing.

**Key Features:**
- Real-time file change detection
- Event debouncing and batching
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

#### Integration Registry
**Status:** 🚧 Planned

Framework-agnostic integration system for connecting with AI agent platforms.

**Key Features:**
- Pluggable adapter pattern
- Automatic framework detection
- Integration lifecycle management
- Thread-safe registry
- Output format processors

**Supported Integrations:**
- Claude Code (automatic setup)
- Continue.dev (manual setup)
- Cline (manual setup)
- Aider (manual setup)
- Cursor AI (manual setup)
- Custom integrations

**Primary Components:**
- `internal/integrations/registry.go` - Integration registry
- `internal/integrations/interface.go` - Integration interface
- `internal/integrations/adapters/claude/` - Claude Code adapter
- `internal/integrations/adapters/generic/` - Generic adapter
- `internal/integrations/output/` - Output processors

**Documentation:** Coming soon

---

### Utilities

#### Walker
**Status:** 🚧 Planned

Directory tree traversal with filtering and relative path computation.

**Key Features:**
- Recursive directory walking
- Skip pattern support
- Relative path computation
- Callback-based processing

**Primary Components:**
- `internal/walker/walker.go` - Directory traversal

**Documentation:** Coming soon

---

#### Version
**Status:** 🚧 Planned

Version information management for build-time metadata.

**Key Features:**
- Version string
- Git commit hash
- Build date
- Build-time variable injection

**Primary Components:**
- `internal/version/version.go` - Version information

**Documentation:** Coming soon

---

## Subsystem Interactions

```
┌─────────────────────────────────────────────────────────────┐
│                    CLI Commands (cmd/)                      │
│  ┌──────────┐ ┌──────────┐ ┌────────────┐ ┌──────────────┐  │
│  │   init   │ │  daemon  │ │    read    │ │ integrations │  │
│  └────┬─────┘ └────┬─────┘ └─────┬──────┘ └──────┬───────┘  │
└───────┼────────────┼─────────────┼───────────────┼──────────┘
        │            │             │               │
        v            v             v               v
┌─────────────────────────────────────────────────────────────┐
│              Configuration (internal/config)                │
│         Loads settings, validates, manages paths            │
└────────────────────────────┬────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        v                    v                    v
┌──────────────┐    ┌─────────────────┐   ┌──────────────────┐
│   Daemon     │───>│ Index Manager   │   │   Integration    │
│  Subsystem   │    │  (atomic I/O)   │   │    Registry      │
└──────┬───────┘    └────────┬────────┘   └────────┬─────────┘
       │                     │                     │
       │                     │                     │
   ┌───┴───────┬─────────────┤                     │
   │           │             │                     │
   v           v             v                     v
┌────────┐ ┌─────────┐ ┌────────────┐       ┌──────────────┐
│Watcher │ │ Worker  │ │   Read     │       │   Output     │
│        │ │  Pool   │ │  Command   │       │  Processors  │
└────────┘ └────┬────┘ └────────────┘       └──────────────┘
                │
        ┌───────┴───────┐
        │               │
        v               v
   ┌─────────┐    ┌──────────┐
   │Metadata │    │ Semantic │
   │Extractor│    │ Analyzer │
   └────┬────┘    └────┬─────┘
        │              │
        │              v
        │         ┌─────────┐
        │         │ Claude  │
        │         │   API   │
        │         └─────────┘
        │              │
        └──────┬───────┘
               v
        ┌────────────┐
        │   Cache    │
        │  Manager   │
        └────────────┘
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

### Configuration Examples
- [Examples Directory](../../examples/) - Configuration templates
- [Examples README](../../examples/README.md) - Example documentation

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

**Last Updated:** 2025-11-01
