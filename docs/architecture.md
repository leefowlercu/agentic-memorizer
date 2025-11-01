# Agentic Memorizer Architecture

**Version**: 1.0
**Last Updated**: 2025-11-01

## Overview

Agentic Memorizer is a framework-agnostic memory indexing system for AI agents. It provides automatic file discovery, semantic analysis, and integration with multiple agent frameworks through a pluggable adapter architecture.

## Core Architecture

### High-Level Design

```
┌─────────────────────────────────────────────────────────────────┐
│                     CORE PIPELINE                               │
│  ┌──────┐   ┌──────────┐   ┌──────────┐   ┌───────┐             │
│  │Daemon│──>│ Metadata │──>│ Semantic │──>│ Index │             │
│  │      │   │ Extract  │   │ Analysis │   │ Mgr   │             │
│  └──────┘   └──────────┘   └──────────┘   └───────┘             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│              INTEGRATION LAYER                                  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │            Integration Registry & Manager                │   │
│  │  - Discovers available integrations                      │   │
│  │  - Manages lifecycle (setup/update/remove)               │   │
│  │  - Thread-safe global registry                           │   │
│  └──────────────────────────────────────────────────────────┘   │
│                          │                                      │
│            ┌─────────────┴─────────────┬──────────────┐         │
│            ▼                           ▼              ▼         │
│  ┌──────────────────┐     ┌──────────────────┐   ┌────────┐     │
│  │  ClaudeAdapter   │     │ GenericAdapter   │   │  ...   │     │
│  │  (Automatic)     │     │ (Manual Setup)   │   │        │     │
│  │                  │     │                  │   │        │     │
│  │ - Setup hooks    │     │ - Continue       │   │        │     │
│  │ - Validate       │     │ - Cline          │   │        │     │
│  │ - Format output  │     │ - Aider          │   │        │     │
│  │                  │     │ - Cursor         │   │        │     │
│  └──────────────────┘     └──────────────────┘   └────────┘     │
│            │                           │              │         │
│            ▼                           ▼              ▼         │
│  ┌──────────────┐         ┌──────────────┐   ┌──────────┐       │
│  │ Claude Code  │         │   Continue   │   │  Cline   │       │
│  │.claude/      │         │.continue/    │   │ .cline/  │       │
│  │settings.json │         │config.json   │   │config.ts │       │
│  └──────────────┘         └──────────────┘   └──────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

## Component Architecture

### 1. Core Pipeline

The core pipeline is framework-agnostic and handles all file processing:

#### Daemon (`internal/daemon/`)
- Watches memory directory for file changes using fsnotify
- Maintains worker pool for parallel processing
- Implements rate limiting for API calls
- Triggers periodic full rebuilds
- Manages precomputed index updates

#### Metadata Extractor (`internal/metadata/`)
- Extracts file metadata (size, modified time, type)
- Handles document-specific metadata:
  - PDF: page count
  - DOCX/PPTX: word count, slide count
  - Images: dimensions, format
  - Transcripts: duration
- Determines file categories (documents, presentations, images, etc.)

#### Semantic Analyzer (`internal/semantic/`)
- Uses Claude API for AI-powered content analysis
- Generates summaries and semantic tags
- Performs vision analysis for images
- Caches analysis results keyed by file hash
- Handles retry logic and error recovery

#### Index Manager (`internal/index/`)
- Maintains precomputed index file (`~/.agentic-memorizer/index.json`)
- Provides atomic updates with backup/rollback
- Tracks index statistics (total files, sizes, cached vs. analyzed)
- Handles concurrent access safely

### 2. Integration Layer

The integration layer provides framework-specific adapters:

#### Integration Interface (`internal/integrations/interface.go`)

All integrations implement this interface:

```go
type Integration interface {
    // Metadata
    GetName() string
    GetDescription() string
    GetVersion() string

    // Detection
    Detect() (bool, error)           // Can we find this framework?
    IsEnabled() (bool, error)         // Is it currently configured?

    // Lifecycle
    Setup(binaryPath string) error   // Configure the framework
    Update(binaryPath string) error  // Update existing config
    Remove() error                    // Remove configuration

    // Command Generation
    GetCommand(binaryPath string, format OutputFormat) string

    // Output Formatting
    FormatOutput(index *types.Index, format OutputFormat) (string, error)

    // Validation
    Validate() error                  // Check configuration health
    Reload(newConfig IntegrationConfig) error
}
```

#### Registry Pattern (`internal/integrations/registry.go`)

Thread-safe global registry for managing integrations:

```go
type Registry struct {
    integrations map[string]Integration
    mu           sync.RWMutex
}

// Key methods:
func (r *Registry) Register(integration Integration)
func (r *Registry) Get(name string) (Integration, error)
func (r *Registry) List() []Integration
func (r *Registry) DetectAvailable() []Integration
func (r *Registry) DetectEnabled() []Integration
```

- **Singleton pattern**: `GlobalRegistry()` provides global instance
- **Auto-registration**: Adapters register via `init()` functions
- **Lazy loading**: Imported via blank imports only when needed
- **Thread-safe**: All operations protected by RWMutex

#### Output Processors (`internal/integrations/output/`)

Separate formatters for different output formats:

**Important Design Principle**: Output formatting is **separate** from integration wrapping:

- **Output Format** = How the index is rendered (XML, Markdown, JSON)
- **Integration Wrapper** = Framework-specific envelope around formatted content

Three processors:
1. **XMLProcessor** - Structured XML with semantic hierarchy
2. **MarkdownProcessor** - Human-readable markdown
3. **JSONProcessor** - Pretty-printed JSON representation

All implement:
```go
type OutputProcessor interface {
    Format(index *types.Index) (string, error)
}
```

### 3. Adapter Implementations

#### Claude Code Adapter (`internal/integrations/adapters/claude/`)

**Automatic Setup**: Modifies `~/.claude/settings.json` directly

Components:
- **adapter.go**: Main integration implementation
- **settings.go**: JSON settings file management with atomic writes
- **output.go**: SessionStart JSON wrapper implementation
- **register.go**: Auto-registers with global registry

Key features:
- Detects `~/.claude/` directory
- Configures all 4 SessionStart matchers (startup, resume, clear, compact)
- Generates commands: `agentic-memorizer read --format xml --integration claude-code`
- Wraps output in SessionStart JSON envelope with:
  - `continue: true`
  - `suppressOutput: true`
  - `systemMessage`: Concise summary
  - `hookSpecificOutput.additionalContext`: Full formatted index

#### Generic Adapter (`internal/integrations/adapters/generic/`)

**Manual Setup**: Provides instructions, doesn't modify configs

Registered for:
- Continue.dev
- Cline
- Aider
- Cursor AI
- Custom frameworks

Key features:
- `Detect()` always returns false (can't auto-detect)
- `Setup()` returns error with manual instructions and exact command
- `FormatOutput()` returns plain formatted content (no wrapper)
- Users manually add commands to their framework's configuration

### 4. Command Layer

#### Commands (`cmd/`)

1. **init** - Initial setup
   - Creates config and directories
   - Detects and optionally sets up integrations
   - Optionally starts daemon

2. **daemon** - Background processing
   - start, stop, status, restart, rebuild
   - logs, health monitoring

3. **read** - Output precomputed index
   - `--format`: xml, markdown, json
   - `--integration`: Format for specific framework
   - `--verbose`: Detailed output

4. **integrations** - Manage integrations
   - list: Show all available integrations
   - detect: Find installed frameworks
   - setup: Configure an integration
   - remove: Remove an integration
   - validate: Check configurations

## Data Flow

### Indexing Flow

```
1. File Change Detected (fsnotify)
        │
        ▼
2. Debounce (500ms default)
        │
        ▼
3. Worker Pool Processes File
        │
        ├─> Metadata Extraction
        │   - File stats
        │   - Document metadata
        │
        ├─> Semantic Analysis
        │   - Check cache (by hash)
        │   - Call Claude API if needed
        │   - Cache result
        │
        └─> Index Update
            - Load current index
            - Merge changes
            - Atomic write with backup
```

### Read Flow

```
1. User/Hook calls: agentic-memorizer read --integration claude-code
        │
        ▼
2. Load Precomputed Index
        │
        ▼
3. Get Integration Adapter
        │
        ▼
4. Adapter.FormatOutput()
        │
        ├─> Select Output Processor (XML/Markdown/JSON)
        │   - Generate formatted content
        │
        └─> Apply Integration Wrapper (if needed)
            - Claude: SessionStart JSON
            - Generic: No wrapper
        │
        ▼
5. Output to stdout (captured by framework hook)
```

## Key Design Patterns

### 1. Adapter Pattern

Each framework gets a dedicated adapter implementing the common `Integration` interface. This allows:
- Framework-specific setup logic
- Framework-specific output formatting
- Clean separation of concerns
- Easy addition of new frameworks

### 2. Registry Pattern

Global registry manages all integrations:
- Single source of truth for available integrations
- Thread-safe concurrent access
- Easy discovery and enumeration
- Lazy loading via blank imports

### 3. Strategy Pattern

Output processors implement a common interface, allowing runtime selection of formatting strategy:
- XML for structured data
- Markdown for readability
- JSON for programmatic access

### 4. Template Method Pattern

Base integration lifecycle:
1. Detect framework
2. Check if already configured
3. Setup configuration
4. Validate setup
5. Generate commands

Each adapter implements these steps differently.

### 5. Singleton Pattern

Global registry instance:
- Single global instance via `GlobalRegistry()`
- Thread-safe initialization
- Consistent access point

## Configuration Management

### Configuration Files

1. **Application Config**: `~/.agentic-memorizer/config.yaml`
   - Memory directory location
   - Claude API settings
   - Output preferences
   - Analysis settings
   - Daemon configuration
   - Integration settings

2. **Precomputed Index**: `~/.agentic-memorizer/index.json`
   - Full index with metadata and semantic analysis
   - Updated by daemon automatically
   - Loaded by read command

3. **Framework Configs**: Managed by adapters
   - Claude Code: `~/.claude/settings.json`
   - Continue: `~/.continue/config.json` (manual)
   - Cline: `.cline/config.ts` (manual)
   - Aider: `.aider.conf.yml` (manual)

### Configuration Schema

```yaml
# Memory location
memory_root: ~/.agentic-memorizer/memory

# Claude API
claude:
  api_key_env: ANTHROPIC_API_KEY
  model: claude-sonnet-4-5-20250929
  max_tokens: 1500
  enable_vision: true

# Output format
output:
  format: xml              # xml, markdown, or json
  verbose: false
  show_recent_days: 7

# File analysis
analysis:
  enable: true
  max_file_size: 10485760  # 10 MB
  parallel: 3
  skip_extensions: [.zip, .tar, .gz, .exe]
  cache_dir: ~/.agentic-memorizer/.cache

# Background daemon
daemon:
  enabled: false
  debounce_ms: 500
  workers: 3
  rate_limit_per_min: 20
  full_rebuild_interval_minutes: 60
  log_file: ~/.agentic-memorizer/daemon.log

# Integration configurations (future)
integrations:
  enabled: []
  configs: {}
```

## Performance Characteristics

### Startup Performance

- **Precomputed index load**: 10-50ms (typical)
- **SessionStart hook execution**: <100ms total (including framework overhead)
- **No analysis overhead**: All analysis done in background

### Background Processing

- **Worker pool**: 3 workers by default (configurable)
- **Rate limiting**: 20 API calls/min default (configurable)
- **Debounce**: 500ms for rapid file changes
- **Cache hit rate**: 95%+ (only new/modified files analyzed)

### Scalability

- **100 files**: ~2-5 seconds initial indexing
- **1,000 files**: ~20-50 seconds initial indexing
- **10,000 files**: ~3-8 minutes initial indexing

Actual performance depends on:
- File sizes
- Number of files requiring semantic analysis
- API rate limits
- Network latency

## Security Considerations

### File Permissions

- Config file: `0644` (readable by all, writable by owner)
- Cache directory: `0755` (standard directory permissions)
- Sensitive files: Settings files preserved with original permissions
- Backup files: Created with same permissions as originals

### API Key Management

- Prefer environment variable (`ANTHROPIC_API_KEY`)
- Config file storage supported but discouraged
- Never logged or output
- Handled only by semantic analysis module

### Settings File Safety

- Atomic writes with backup
- Rollback on failure
- Preserve existing settings when adding hooks
- No destructive operations without user confirmation

## Extension Points

### Adding New Integrations

1. Create adapter package in `internal/integrations/adapters/yourframework/`
2. Implement `Integration` interface
3. Add register.go with `init()` function
4. Import in `cmd/integrations/integrations.go`
5. Test with actual framework
6. Document in `docs/integrations/yourframework.md`

### Adding New Output Formats

1. Create processor in `internal/integrations/output/`
2. Implement `OutputProcessor` interface
3. Add format constant to `types.go`
4. Update `ParseOutputFormat()` utility
5. Update adapters to handle new format
6. Document usage

### Adding New Metadata Extractors

1. Add handler to `internal/metadata/`
2. Register for file extensions
3. Extract relevant metadata
4. Update `IndexEntry.Metadata` if needed

## Future Architecture Considerations

### Potential Enhancements

1. **Plugin System**
   - Load integrations from external packages
   - Runtime registration
   - Versioned plugin API

2. **Remote Index**
   - Shared index across machines
   - Cloud storage backends
   - Sync protocol

3. **Advanced Caching**
   - Multi-tier cache (memory + disk)
   - Distributed cache
   - Cache warming strategies

4. **Integration Features**
   - Auto-update detection for frameworks
   - Migration helpers for breaking changes
   - Integration health monitoring

5. **Analysis Pipeline**
   - Custom analysis plugins
   - Multiple LLM providers
   - Embeddings support for similarity search

## References

- [Implementation Plan](wip/agent-framework-decoupling.md)
- [Integration Interface](../internal/integrations/interface.go)
- [Claude Adapter](../internal/integrations/adapters/claude/)
- [Generic Adapter](../internal/integrations/adapters/generic/)
