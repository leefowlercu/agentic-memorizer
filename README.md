# Agentic Memorizer

A semantic file indexer for Claude Code that provides automatic awareness and understanding of files in your memory directory through AI-powered analysis.

## Overview

Agentic Memorizer integrates with Claude Code via SessionStart hooks to automatically index and semantically analyze files in `~/.claude/memory/`. Instead of manually adding files to context, Claude Code automatically receives a structured index showing what files exist, what they contain, and how to access them.

### Key Features

- **Automatic Indexing**: Runs on every Claude Code session start
- **Semantic Understanding**: Uses Claude API to understand file content and purpose
- **Smart Caching**: Only analyzes new or modified files
- **Multi-Format Support**: Handles documents, presentations, images, code, and more
- **Vision Analysis**: Understands image content using Claude's vision capabilities
- **Fast Performance**: <200ms startup with cached analyses

## How It Works

1. **SessionStart Hook** triggers the indexer when Claude Code starts
2. **File System Walk** discovers all files in `~/.claude/memory/`
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
memorizer init
```

This creates:
- Config file at `~/.local/bin/memorizer-config.yaml`
- Memory directory at `~/.claude/memory/`

#### Option 2: Using Makefile

```bash
make install
```

This will:
- Build the `memorizer` binary
- Install it to `~/.local/bin/memorizer`
- Create default config at `~/.local/bin/memorizer-config.yaml`

### Configuration

Set your API key via environment variable (recommended):

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

Or edit `~/.local/bin/memorizer-config.yaml`:

```yaml
claude:
  api_key: "your-api-key-here"
```

**Custom Setup:**

```bash
# Custom memory directory
memorizer init --memory-root ~/my-memory

# Custom config location
memorizer init --config-path ~/.memorizer-config.yaml

# Force overwrite existing config
memorizer init --force
```

### Configure Claude Code Hook

Add to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOUR_USERNAME/.local/bin/memorizer"
          }
        ]
      },
      {
        "matcher": "resume",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOUR_USERNAME/.local/bin/memorizer"
          }
        ]
      },
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOUR_USERNAME/.local/bin/memorizer"
          }
        ]
      },
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOUR_USERNAME/.local/bin/memorizer"
          }
        ]
      }
    ]
  }
}
```

Replace `YOUR_USERNAME` with your actual username.

**Note**: The configuration includes all four SessionStart matchers to ensure the memory index stays current:
- `"startup"`: Runs when starting a new session with `claude`
- `"resume"`: Runs when resuming with `claude --resume`, `claude --continue`, or `/resume`
- `"clear"`: Runs after the `/clear` command clears the context
- `"compact"`: Runs when Claude Code compacts the context (automatically or via `/compact`)

## Usage

### Normal Operation

Once installed and configured, the indexer runs automatically throughout your Claude Code session lifecycle. No manual action needed!

The indexer will run:
- When starting a new session with `claude`
- When resuming a session with `claude --resume`, `claude --continue`, or `/resume`
- After clearing context with `/clear`
- When context is compacted (automatically or via `/compact`)

This ensures your memory index stays current as the context changes throughout your session.

### Adding Files to Memory

Simply add files to `~/.claude/memory/` (or the directory you've configured as the `memory_root` in `memorizer-config.yaml`):

```bash
# Organize however you like
~/.claude/memory/
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
memorizer
```

This outputs the markdown index that Claude Code receives.

### CLI Usage

**Commands:**

```bash
# Initialize config and memory directory
memorizer init [flags]

# Run indexing (default command)
memorizer [flags]

# Get help
memorizer --help
memorizer init --help
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
memorizer

# JSON output for hooks
memorizer --format json

# Verbose mode
memorizer --verbose

# Force re-analysis
memorizer --force-analyze

# Analyze specific file
memorizer --analyze-file ~/document.pdf
```

### Force Re-analysis

```bash
memorizer --force-analyze
```

Clears cache and re-analyzes all files.

### Skip Semantic Analysis (Fast Mode)

```bash
memorizer --no-semantic
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

See `memorizer-config.example.yaml` for all options:

- **API Settings**: Model, tokens, timeout
- **Analysis**: Enable/disable, file size limits, parallel processing, file exclusions
- **Output**: Format (markdown/json), verbosity, recent activity days
- **Caching**: Automatic based on file hashes

### File Exclusions

The indexer automatically excludes:
- Hidden files and directories (starting with `.`)
- The `.cache/` directory (where analyses are cached)
- The `memorizer` binary itself (if located in the memory directory)
- The `memorizer-config.yaml` file (if located in the memory directory)

You can exclude additional files by name in `memorizer-config.yaml`:

```yaml
analysis:
  skip_files:
    - memorizer  # Default
    - my-private-notes.md
    - temp-file.txt
```

Files in the skip list are completely ignored during indexing and won't appear in the generated index.

### Output Formats

The memorizer supports two output formats:

#### Markdown (Default)

Standard markdown output that can be displayed directly or piped to files:

```bash
memorizer
```

#### JSON (Claude Code Hooks)

Structured JSON output conforming to Claude Code's hook specification. Use this format when the memorizer is called from Claude Code hooks:

```bash
memorizer --format json
```

Or configure in `memorizer-config.yaml`:

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
├── cmd/memorizer/    # Main entry point
├── internal/
│   ├── config/               # Configuration loading
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

## Troubleshooting

### "API key is required" error

Set your API key in config or environment:

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

### Index not appearing in Claude Code

1. Check hook is configured in `~/.claude/settings.json`
2. Verify binary path is correct (`~/.local/bin/memorizer`)
3. Test manually: `memorizer`
4. Check Claude Code terminal output for errors

### Slow performance

- Reduce `analysis.parallel` in config
- Decrease `analysis.max_file_size` to skip large files
- Use `--no-semantic` for quick metadata-only indexing

### Cache issues

Clear cache to force re-analysis:

```bash
rm -rf <memory_root>/.cache
```
