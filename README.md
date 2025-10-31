# Agentic Memorizer

A local file 'memorizer' for Claude Code and Claude Agents that provides automatic awareness and understanding of files in your memory directory through AI-powered semantic analysis.

## Overview

Agentic Memorizer provides Claude Code and Claude Agents with persistent, semantic awareness of your local files. Instead of manually managing which files to include in context or repeatedly explaining what files exist, Claude automatically receives a comprehensive, AI-powered index showing what files you have, what they contain, their purpose, and how to access them.

### How It Works

A background daemon continuously watches your designated memory directory (`~/.agentic-memorizer/memory/` by default), automatically discovering and analyzing files as they're added or modified. Each file is processed to extract metadata (word counts, dimensions, page counts, etc.) and—using the Claude API—semantically analyzed to understand its content, purpose, and key topics. This information is maintained in a precomputed index that loads instantly (<50ms) when Claude Code starts.

When you launch Claude Code, a SessionStart hook loads the precomputed index into Claude's context. Claude can then:
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

**Performance & Efficiency:**
- Background daemon handles all processing asynchronously
- Smart caching only re-analyzes changed files (95%+ cache hit rate)
- Precomputed index loads in 10-50ms for instant Claude Code startup
- Minimal API usage—only new/modified files are analyzed

**Wide Format Support:**
- **Direct reading**: Markdown, text, JSON/YAML, code files, images, VTT transcripts
- **Extraction supported**: Word documents (DOCX), PowerPoint (PPTX), PDFs
- Automatic metadata extraction for all file types

**Integration:**
- Seamless Claude Code integration via SessionStart hooks
- Compatible with Claude Agents and other AI systems
- Configurable output formats (XML, Markdown)
- Optional health monitoring and logging

## Why Use This?

**Instead of:**
- ✗ Manually copying file contents into prompts
- ✗ Pre-loading all files into context (wasting tokens)
- ✗ Repeatedly explaining what files exist to Claude
- ✗ Managing which files to include/exclude manually

**You get:**
- ✓ Automatic file awareness on every session
- ✓ Smart, on-demand file access (Claude decides what to read)
- ✓ Semantic understanding of content before reading
- ✓ Efficient token usage (only index, not full content)
- ✓ Works across sessions with persistent cache

## Architecture

**Background Daemon Architecture:**

1. **Background Daemon** continuously watches `~/.agentic-memorizer/memory/` for file changes
2. **File Processing** automatically extracts metadata and performs semantic analysis via Claude API
3. **Smart Caching** stores analyses keyed by file hash (only re-analyzes when files change)
4. **Precomputed Index** maintains `~/.agentic-memorizer/index.json` with all file information
5. **SessionStart Hook** triggers `read` command when Claude Code starts
6. **Fast Read** loads and formats the precomputed index (~10-50ms) for Claude's context

The daemon handles all the heavy lifting in the background, so Claude Code startup remains fast regardless of how many files you have.

## Quick Start

Get up and running in 3 minutes:

### 1. Install

```bash
go install github.com/leefowlercu/agentic-memorizer@latest
```

### 2. Set API Key

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

### 3. Initialize with Daemon and Hooks

```bash
agentic-memorizer init --setup-hooks --with-daemon
```

This will:
- Create config at `~/.agentic-memorizer/config.yaml`
- Create memory directory at `~/.agentic-memorizer/memory/`
- Configure Claude Code SessionStart hooks automatically
- Start the background daemon for automatic indexing

### 4. Add Files to Memory

```bash
# Add any files you want Claude to be aware of
cp ~/important-notes.md ~/.agentic-memorizer/memory/
cp ~/project-docs/*.pdf ~/.agentic-memorizer/memory/documents/
```

The daemon will automatically detect and index these files within seconds.

### 5. Start Claude Code

```bash
claude
```

Claude now automatically knows about all files in your memory directory!

---

For detailed installation options, configuration, and advanced usage, see the sections below.

## Installation

### Prerequisites

- Go 1.25.1 or later
- Claude API key ([get one here](https://console.anthropic.com/))
- Claude Code (or a Claude Agent that loads settings from `~/.claude/settings.json`) installed

### Build and Install

#### Option 1: Using go install (Recommended)

```bash
go install github.com/leefowlercu/agentic-memorizer@latest
```

Then run the init command to set up configuration:

```bash
# Interactive setup (prompts for hooks and daemon)
agentic-memorizer init

# Or with flags for automated setup
agentic-memorizer init --setup-hooks --with-daemon
```

This creates:
- Config file at `~/.agentic-memorizer/config.yaml`
- Memory directory at `~/.agentic-memorizer/memory/`
- Cache directory at `~/.agentic-memorizer/.cache/` (for semantic analysis cache)
- Index file at `~/.agentic-memorizer/index.json` (created by daemon on first run)

The init command can optionally:
- Configure Claude Code SessionStart hooks automatically (`--setup-hooks`)
- Start the background daemon immediately (`--with-daemon`)
- Both are recommended for the best experience

#### Option 2: Using Makefile

```bash
make install
```

This will:
- Build the `agentic-memorizer` binary
- Install it to `~/.local/bin/agentic-memorizer`
- Create default config at `~/.agentic-memorizer/config.yaml`

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
agentic-memorizer init --memory-root ~/my-memory

# Custom cache directory
agentic-memorizer init --cache-dir ~/my-memory/.cache

# Force overwrite existing config
agentic-memorizer init --force
```

### Configure Claude Code Hook

#### Automatic Setup (Recommended)

The easiest way to set up hooks is to use the `--setup-hooks` flag during initialization:

```bash
agentic-memorizer init --setup-hooks
```

This will:
- Auto-detect the binary location
- Configure all four SessionStart matchers (`startup`, `resume`, `clear`, `compact`)
- Preserve any existing hooks you have configured
- Add the `read --format xml --wrap-json` command for proper Claude Code integration

You can also run the init command without flags and it will prompt you to set up hooks interactively.

#### Manual Setup

Alternatively, add to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer read --format xml --wrap-json"
          }
        ]
      }
      // Repeat for "resume", "clear", and "compact" matchers
    ]
  }
}
```

**Note**: Include all four SessionStart matchers (`startup`, `resume`, `clear`, `compact`) to ensure the memory index loads throughout your session lifecycle. Each matcher should use the same command: `agentic-memorizer read --format xml --wrap-json`.

### Using with Claude Agents

For [Claude Agents](https://docs.claude.com/en/api/agent-sdk/overview):

1. Ensure the `agentic-memorizer` program is installed and configured on the same machine running the agent
2. Ensure the background daemon is running: `agentic-memorizer daemon start`
3. Configure your agent to run: `agentic-memorizer read --format xml --wrap-json` or `agentic-memorizer read --format markdown --wrap-json` on SessionStart Hook

### Using with Other AI Agents

The Agentic Memorizer works with any system that can execute shell commands and capture output. Make sure:

1. The program is installed and configured
2. The background daemon is running: `agentic-memorizer daemon start`
3. Your agent runs this command and adds output to context:

```bash
agentic-memorizer read --format <xml|markdown>
```

## Usage

### Background Daemon (Required)

The background daemon is the core of Agentic Memorizer. It maintains a precomputed index for fast (<50ms) startup times, watching your memory directory and automatically updating the index as files change.

#### Quick Start

```bash
# Start the daemon (run in foreground - use Ctrl+C to stop, or run in background with systemd/launchd)
agentic-memorizer daemon start
```

**Note**: If you used `init --setup-hooks --with-daemon`, the daemon and hooks are already configured. Otherwise, make sure your hooks call `agentic-memorizer read --format xml --wrap-json`.

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
```

#### How It Works

The daemon:
1. **Watches** your memory directory for file changes using fsnotify
2. **Processes** files in parallel using a worker pool (3 workers by default)
3. **Rate limits** API calls to respect Claude API limits (20/min default)
4. **Maintains** a precomputed `index.json` file with all metadata and semantic analysis
5. **Updates** the index automatically when files are added/modified/deleted

When you run `agentic-memorizer read`, it simply loads the precomputed index from disk (~10-50ms) instead of analyzing all files (~120ms-15s).

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

#### Running as a Service

**macOS (launchd):**
See `examples/com.agentic-memorizer.daemon.plist` for a complete launchd configuration.

**Linux (systemd):**
See `examples/agentic-memorizer.service` for a complete systemd unit file.

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

This outputs the index (XML by default) that Claude Code receives from SessionStart hooks. The daemon must be running (or have completed at least one indexing cycle) for the index file to exist.

### CLI Usage

**Commands:**

```bash
# Initialize config and memory directory
agentic-memorizer init [flags]

# Manage background daemon
agentic-memorizer daemon start
agentic-memorizer daemon stop
agentic-memorizer daemon status

# Read precomputed index (for SessionStart hooks)
agentic-memorizer read [flags]

# Get help
agentic-memorizer --help
agentic-memorizer init --help
agentic-memorizer daemon --help
agentic-memorizer read --help
```

**Common Flags:**

```bash
# Read command flags
--format <xml|markdown>      # Output format
--wrap-json                  # Wrap output in SessionStart JSON structure
--verbose                    # Verbose output

# Init command flags
--memory-root <dir>          # Custom memory directory
--cache-dir <dir>            # Custom cache directory
--force                      # Overwrite existing config
--setup-hooks                # Configure Claude Code hooks
--with-daemon                # Start daemon after init
```

**Examples:**

```bash
# Initialize with daemon
agentic-memorizer init --with-daemon

# Read index (XML format)
agentic-memorizer read

# Read index (Markdown format)
agentic-memorizer read --format markdown

# Read with JSON wrapper (for SessionStart hooks)
agentic-memorizer read --format xml --wrap-json

# Verbose output
agentic-memorizer read --verbose

# Start daemon
agentic-memorizer daemon start

# Check daemon status
agentic-memorizer daemon status

# Force rebuild index
agentic-memorizer daemon rebuild
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

The index tells Claude Code which method to use for each file.

## Configuration Options

See `config.yaml.example` for all options:

- **API Settings**: Model, tokens, timeout
- **Analysis**: Enable/disable, file size limits, parallel processing, file exclusions
- **Output**: Format (xml/markdown), verbosity, recent activity days
- **Caching**: Automatic based on file hashes

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

### Output Formats

The memorizer supports two output formats:

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

#### JSON Wrapping for Hooks

Use the `--wrap-json` flag to wrap the output (markdown or XML) in structured JSON conforming to Claude Code's hook specification. This is used when the memorizer is called from Claude Code or Claude Agent hooks:

```bash
# XML wrapped in JSON (recommended for hooks)
agentic-memorizer read --format xml --wrap-json

# Markdown wrapped in JSON
agentic-memorizer read --format markdown --wrap-json
```

Or configure in `config.yaml`:

```yaml
output:
  format: xml
  wrap_json: true
```

**JSON Output Structure:**

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
- **hookSpecificOutput**: Contains the full index (XML or Markdown) in `additionalContext`, which Claude Code adds to the context window


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
├── main.go           # Main entry point
├── cmd/
│   ├── root.go               # Root command (orchestrates subcommands)
│   ├── init/                 # Initialization subcommand
│   ├── daemon/               # Daemon management (start/stop/status/restart/rebuild/logs)
│   └── read/                 # Read precomputed index (fast path for hooks)
├── internal/
│   ├── config/               # Configuration loading and path management
│   ├── daemon/               # Background daemon implementation
│   ├── index/                # Index management and atomic writes
│   ├── watcher/              # File system watching (fsnotify)
│   ├── hooks/                # Claude Code hook management
│   ├── walker/               # File system traversal
│   ├── metadata/             # File metadata extraction
│   ├── semantic/             # Claude API integration
│   ├── cache/                # Analysis caching
│   ├── output/               # Output formatting (XML/Markdown)
│   └── version/              # Version information
├── pkg/types/                # Shared types and data structures
├── examples/                 # Service configuration examples (systemd, launchd)
└── testdata/                 # Test files
```

### Building

```bash
make build      # Build binary
make install    # Build and install
make test       # Run tests
make clean      # Remove build artifacts
make deps       # Update dependencies
```

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

## Performance

With the daemon-based architecture:

- **Read command** (SessionStart hooks): ~10-50ms (loads precomputed index)
- **Daemon initial indexing** (10 files): ~10-15 seconds (happens in background)
- **Daemon incremental updates**: ~1-2 seconds per file (happens automatically)
- **Cache hit rate**: Typically >95% after first daemon run
- **API usage**: Only for new/modified files (daemon handles this in background)

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

### Index not appearing in Claude Code

1. Verify daemon is running: `agentic-memorizer daemon status`
2. Check hooks are configured in `~/.claude/settings.json` with `read` command
3. Verify binary path is correct (`~/.local/bin/agentic-memorizer` or `~/go/bin/agentic-memorizer`)
4. Test manually: `agentic-memorizer read`
5. Check Claude Code terminal output for errors

### Slow performance

First-time indexing of many files:
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

[Add license information here]
