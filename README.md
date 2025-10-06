# Agentic Memorizer

A local file 'memorizer' for Claude Code and Claude Agents that provides automatic awareness and understanding of files in your memory directory through AI-powered semantic analysis.

## Overview

Agentic Memorizer integrates with Claude Code or Claude Agents via SessionStart hooks to automatically index and semantically analyze files in `~/.agentic-memorizer/memory/`. Instead of manually adding files to context, Claude Code or Claude Agents automatically receive a structured index in their context window showing what files exist, what they contain, and how to access them.

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

### Key Features

- **Automatic Indexing**: Runs on every Claude Code session start
- **Semantic Understanding**: Uses Claude API to understand file content and purpose
- **Smart Caching**: Only analyzes new or modified files
- **Multi-Format Support**: Handles documents, presentations, images, code, and more
- **Vision Analysis**: Understands image content using Claude's vision capabilities
- **Fast Performance**: <200ms startup with cached analyses

## How It Works

1. **SessionStart Hook** triggers the indexer when Claude Code starts
2. **File System Walk** discovers all files in `~/.agentic-memorizer/memory/`
3. **Metadata Extraction** pulls file-specific metadata (pages, dimensions, word count, etc.)
4. **Semantic Analysis** uses Claude API to understand content and generate summaries
5. **Smart Caching** stores analyses keyed by file hash (only re-analyze if file changes)
6. **Structured Index** outputs XML or Markdown index that Claude Code receives in context

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

### 3. Initialize with Hook Setup

```bash
agentic-memorizer init --setup-hooks
```

This will:
- Create config at `~/.agentic-memorizer/config.yaml`
- Create memory directory at `~/.agentic-memorizer/memory/`
- Configure Claude Code SessionStart hooks automatically

### 4. Add Files to Memory

```bash
# Add any files you want Claude to be aware of
cp ~/important-notes.md ~/.agentic-memorizer/memory/
cp ~/project-docs/*.pdf ~/.agentic-memorizer/memory/documents/
```

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
agentic-memorizer init
```

This creates:
- Config file at `~/.agentic-memorizer/config.yaml`
- Memory directory at `~/.agentic-memorizer/memory/`
- Cache directory at `~/.agentic-memorizer/memory/.cache/`

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
- Add the `--format xml --wrap-json` flags for proper Claude Code integration

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
            "command": "/path/to/agentic-memorizer --format xml --wrap-json"
          }
        ]
      },
      {
        "matcher": "resume",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer --format xml --wrap-json"
          }
        ]
      },
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer --format xml --wrap-json"
          }
        ]
      },
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer --format xml --wrap-json"
          }
        ]
      }
    ]
  }
}
```

**Note**: The configuration includes all four SessionStart matchers to ensure the memory index stays current throughout your session lifecycle.

### Using with Claude Agents

For [Claude Agents](https://docs.claude.com/en/api/agent-sdk/overview):

1. Ensure the `agentic-memorizer` program is installed and configured on the same machine running the agent
2. Configure your agent to run: `agentic-memorizer --format xml --wrap-json` or `agentic-memorizer --format markdown --wrap-json` on SessionStart Hook

### Using with Other AI Agents

The Agentic Memorizer works with any system that can execute shell commands and capture output. Make sure the program is installed and configured properly, then set up your agent to run:

```bash
agentic-memorizer --format <xml|markdown>
```

And add the output to the agent's context window.

## Usage

### Normal Operation

Once installed and configured, the indexer runs automatically. No manual action needed!

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

Test the indexer manually:

```bash
agentic-memorizer
```

This outputs the index (XML by default) that Claude Code receives.

### CLI Usage

**Commands:**

```bash
# Initialize config and memory directory
agentic-memorizer init [flags]

# Run indexing (default command)
agentic-memorizer [flags]

# Get help
agentic-memorizer --help
agentic-memorizer init --help
```

**Common Flags:**

```bash
# Indexing flags
--format <xml|markdown>      # Output format
--wrap-json                  # Wrap output in SessionStart JSON structure
--verbose                    # Verbose output
--force-analyze              # Force re-analysis (clear cache)
--no-semantic                # Skip semantic analysis
--analyze-file <path>        # Analyze a specific file

# Init flags
--memory-root <dir>          # Custom memory directory
--config-path <path>         # Custom config file location
--cache-dir <dir>            # Custom cache directory
--force                      # Overwrite existing config
```

**Examples:**

```bash
# Standard indexing
agentic-memorizer

# XML output format
agentic-memorizer --format xml

# JSON wrapped output for hooks
agentic-memorizer --format markdown --wrap-json

# Verbose mode
agentic-memorizer --verbose

# Force re-analysis
agentic-memorizer --force-analyze

# Analyze specific file
agentic-memorizer --analyze-file ~/document.pdf
```

### Force Re-analysis

```bash
agentic-memorizer --force-analyze
```

Clears cache and re-analyzes all files.

### Skip Semantic Analysis (Fast Mode)

```bash
agentic-memorizer --no-semantic
```

Only extracts metadata, skips Claude API calls.

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
agentic-memorizer
# or explicitly:
agentic-memorizer --format xml
```

#### Markdown

Human-readable markdown, formatted for direct viewing:

```bash
agentic-memorizer --format markdown
```

#### JSON Wrapping for Hooks

Use the `--wrap-json` flag to wrap the output (markdown or XML) in structured JSON conforming to Claude Code's hook specification. This is used when the memorizer is called from Claude Code or Claude Agent hooks:

```bash
# XML wrapped in JSON (recommended for hooks)
agentic-memorizer --format xml --wrap-json

# Markdown wrapped in JSON
agentic-memorizer --format markdown --wrap-json
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

```xml
<memory_index>
  <metadata>
    <generated>2025-10-05T14:30:22-04:00</generated>
    <file_count>3</file_count>
    <total_size_bytes>2145728</total_size_bytes>
    <total_size_human>2.0 MB</total_size_human>
    <root_path>/Users/username/.agentic-memorizer/memory</root_path>
    <cache_stats>
      <cached_files>2</cached_files>
      <analyzed_files>1</analyzed_files>
    </cache_stats>
  </metadata>

  <recent_activity days="7">
    <file>
      <path>documents/api-design-guide.md</path>
      <modified>2025-10-04</modified>
    </file>
    <file>
      <path>code/database-migration.sql</path>
      <modified>2025-10-03</modified>
    </file>
  </recent_activity>

  <categories>
    <category name="documents" count="1" total_size="45.2 KB">
      <file>
        <name>api-design-guide.md</name>
        <path>/Users/username/.agentic-memorizer/memory/documents/api-design-guide.md</path>
        <modified>2025-10-04</modified>
        <size_bytes>46285</size_bytes>
        <size_human>45.2 KB</size_human>
        <file_type>markdown</file_type>
        <category>documents</category>
        <readable>true</readable>
        <metadata>
          <word_count>4520</word_count>
          <section_count>8</section_count>
        </metadata>
        <semantic>
          <summary>Comprehensive API design guidelines covering RESTful principles, versioning strategies, authentication patterns, and best practices for building scalable microservices. Includes examples of endpoint design, error handling, and rate limiting approaches.</summary>
          <document_type>technical-guide</document_type>
          <topics>
            <topic>RESTful API design principles and conventions</topic>
            <topic>API versioning and backward compatibility</topic>
            <topic>Authentication and authorization patterns</topic>
            <topic>Rate limiting and performance optimization</topic>
          </topics>
          <tags>
            <tag>api-design</tag>
            <tag>rest</tag>
            <tag>microservices</tag>
            <tag>best-practices</tag>
          </tags>
        </semantic>
      </file>
    </category>
    <category name="code" count="1" total_size="12.8 KB">
      <file>
        <name>database-migration.sql</name>
        <path>/Users/username/.agentic-memorizer/memory/code/database-migration.sql</path>
        <modified>2025-10-03</modified>
        <size_bytes>13107</size_bytes>
        <size_human>12.8 KB</size_human>
        <file_type>sql</file_type>
        <category>code</category>
        <readable>true</readable>
        <metadata>
          <line_count>342</line_count>
        </metadata>
        <semantic>
          <summary>Database migration script for transitioning from PostgreSQL 13 to 14, including schema updates for user authentication tables, adding new indexes for performance, and data type migrations for timestamp fields.</summary>
          <document_type>database-migration</document_type>
          <topics>
            <topic>PostgreSQL version upgrade procedures</topic>
            <topic>Schema modifications for authentication system</topic>
            <topic>Index optimization for query performance</topic>
          </topics>
          <tags>
            <tag>database</tag>
            <tag>migration</tag>
            <tag>postgresql</tag>
            <tag>sql</tag>
          </tags>
        </semantic>
      </file>
    </category>
    <category name="images" count="1" total_size="1.4 MB">
      <file>
        <name>system-architecture.png</name>
        <path>/Users/username/.agentic-memorizer/memory/images/system-architecture.png</path>
        <modified>2025-09-28</modified>
        <size_bytes>1468006</size_bytes>
        <size_human>1.4 MB</size_human>
        <file_type>png</file_type>
        <category>images</category>
        <readable>true</readable>
        <metadata>
          <dimensions>2560x1440</dimensions>
        </metadata>
        <semantic>
          <summary>High-level system architecture diagram showing the microservices ecosystem with API gateway, service mesh, message queues, and database clusters. Illustrates data flow between frontend applications, backend services, and external integrations.</summary>
          <document_type>architecture-diagram</document_type>
          <topics>
            <topic>Microservices architecture and service boundaries</topic>
            <topic>API gateway and service mesh patterns</topic>
            <topic>Message queue integration for async processing</topic>
          </topics>
          <tags>
            <tag>architecture</tag>
            <tag>microservices</tag>
            <tag>system-design</tag>
          </tags>
        </semantic>
      </file>
    </category>
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

```markdown
# Claude Code Agentic Memory Index
📅 Generated: 2025-10-05 14:30:24
📁 Files: 3 | 💾 Total Size: 2.0 MB
📂 Root: /Users/username/.agentic-memorizer/memory

## 🕐 Recent Activity (Last 7 Days)
- 2025-10-04: `documents/api-design-guide.md`
- 2025-10-03: `code/database-migration.sql`

---

## 📄 Documents (1 files, 45.2 KB)

### api-design-guide.md
**Path**: `/Users/username/.agentic-memorizer/memory/documents/api-design-guide.md`
**Modified**: 2025-10-04 | **Size**: 45.2 KB | **Words**: 4,520 | **Sections**: 8
**Type**: Markdown • Technical-Guide

**Summary**: Comprehensive API design guidelines covering RESTful principles, versioning strategies, authentication patterns, and best practices for building scalable microservices. Includes examples of endpoint design, error handling, and rate limiting approaches.

**Topics**: RESTful API design principles and conventions, API versioning and backward compatibility, Authentication and authorization patterns, Rate limiting and performance optimization
**Tags**: `api-design` `rest` `microservices` `best-practices`

✓ Use Read tool directly

## 💻 Code (1 files, 12.8 KB)

### database-migration.sql
**Path**: `/Users/username/.agentic-memorizer/memory/code/database-migration.sql`
**Modified**: 2025-10-03 | **Size**: 12.8 KB | **Lines**: 342
**Type**: Sql • Database-Migration

**Summary**: Database migration script for transitioning from PostgreSQL 13 to 14, including schema updates for user authentication tables, adding new indexes for performance, and data type migrations for timestamp fields.

**Topics**: PostgreSQL version upgrade procedures, Schema modifications for authentication system, Index optimization for query performance
**Tags**: `database` `migration` `postgresql` `sql`

✓ Use Read tool directly

## 🖼️ Images (1 files, 1.4 MB)

### system-architecture.png
**Path**: `/Users/username/.agentic-memorizer/memory/images/system-architecture.png`
**Modified**: 2025-09-28 | **Size**: 1.4 MB | **Dimensions**: 2560x1440
**Type**: Png • Architecture-Diagram

**Summary**: High-level system architecture diagram showing the microservices ecosystem with API gateway, service mesh, message queues, and database clusters. Illustrates data flow between frontend applications, backend services, and external integrations.

**Topics**: Microservices architecture and service boundaries, API gateway and service mesh patterns, Message queue integration for async processing
**Tags**: `architecture` `microservices` `system-design`

✓ Use Read tool directly

## Usage Guide

**Reading Files**:
- ✅ **Direct**: Markdown, text, VTT, JSON, YAML, images → Use Read tool
- ⚠️ **Extraction needed**: DOCX, PPTX, PDF → Use Bash + conversion tools

**When to access**: Ask me to read any file when relevant to your query. I'll use the appropriate method based on file type.

**Re-indexing**: Index auto-updates on session start. Manual re-index: run memorizer
```

## Development

### Project Structure

```
agentic-memorizer/
├── main.go           # Main entry point
├── cmd/
│   ├── root.go               # Root command
│   └── init/                 # Init subcommand
├── internal/
│   ├── config/               # Configuration loading
│   ├── hooks/                # Claude Code hook management
│   ├── walker/               # File system traversal
│   ├── metadata/             # File metadata extraction
│   ├── semantic/             # Claude API integration
│   ├── cache/                # Analysis caching
│   └── output/               # Output formatting
├── pkg/types/                # Shared types
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

- **Cold start** (no cache, 10 files): ~10-15 seconds
- **Warm start** (with cache): <200ms
- **Cache hit rate**: Typically >95% after first run
- **API usage**: Only for new/modified files

## Limitations & Known Issues

### Current Limitations

- **API Costs**: Semantic analysis uses Claude API calls (costs apply)
  - Mitigated by caching (only analyzes new/modified files)
  - Can disable with `--no-semantic` for metadata-only mode

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

1. Check hook is configured in `~/.claude/settings.json`
2. Verify binary path is correct (`~/.local/bin/agentic-memorizer`)
3. Test manually: `agentic-memorizer`
4. Check Claude Code terminal output for errors

### Slow performance

- Reduce `analysis.parallel` in config
- Decrease `analysis.max_file_size` to skip large files
- Use `--no-semantic` for quick metadata-only indexing

### Cache issues

Clear cache to force re-analysis:

```bash
# Default location
rm -rf ~/.agentic-memorizer/memory/.cache

# Or use --force-analyze flag
agentic-memorizer --force-analyze
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
