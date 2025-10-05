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
6. **Markdown Index** outputs structured index that Claude Code receives in context

## Installation

### Prerequisites

- Go 1.21 or later
- Claude API key ([get one here](https://console.anthropic.com/))
- Claude Code installed

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
- Add the `--format json` flag for proper Claude Code integration

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
            "command": "/Users/YOUR_USERNAME/.local/bin/agentic-memorizer --format json"
          }
        ]
      },
      {
        "matcher": "resume",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOUR_USERNAME/.local/bin/agentic-memorizer --format json"
          }
        ]
      },
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOUR_USERNAME/.local/bin/agentic-memorizer --format json"
          }
        ]
      },
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOUR_USERNAME/.local/bin/agentic-memorizer --format json"
          }
        ]
      }
    ]
  }
}
```

Replace `YOUR_USERNAME` with your actual username.

**Note**: The configuration includes all four SessionStart matchers to ensure the memory index stays current throughout your session lifecycle.

### Using with Claude Agents

For Claude Agents or other systems that can execute commands:

1. Configure your agent to run: `agentic-memorizer --format json`
2. Parse the JSON response to extract the index
3. Add the `additionalContext` to your agent's context

The memorizer works with any system that can execute shell commands and capture output.

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

This outputs the markdown index that Claude Code receives.

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
--config <path>              # Custom config file path
--format <markdown|json>     # Output format
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

# JSON output for hooks
agentic-memorizer --format json

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
- **Output**: Format (markdown/json), verbosity, recent activity days
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

#### Markdown (Default)

Standard markdown output that can be displayed directly or piped to files:

```bash
agentic-memorizer
```

#### JSON (Claude Code Hooks)

Structured JSON output conforming to Claude Code's hook specification. Use this format when the memorizer is called from Claude Code hooks:

```bash
agentic-memorizer --format json
```

Or configure in `config.yaml`:

```yaml
output:
  format: json
```

**JSON Output Structure:**

```json
{
  "continue": true,
  "suppressOutput": true,
  "systemMessage": "Memory index updated: 15 files (5 documents, 3 images, 2 presentations, 5 code files), 2.3 MB total — 12 cached, 3 analyzed",
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "# Claude Code Agentic Memory Index\n..."
  }
}
```

- **continue**: Always `true` - allows session to proceed
- **suppressOutput**: Always `true` - keeps verbose index out of transcript
- **systemMessage**: Concise summary visible to user in UI
- **hookSpecificOutput**: Contains the full markdown index in `additionalContext`, which Claude Code adds to the context window

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
