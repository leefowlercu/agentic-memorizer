# Claude Code Integration Guide

**Integration Type**: Automatic Setup
**Status**: Fully Supported
**Version**: 1.0.0

## Overview

The Claude Code integration provides automatic setup and configuration for Claude Code's SessionStart hooks. This enables Claude to automatically load your memory index at the start of every session (startup, resume, clear, compact).

## Quick Start

### Prerequisites

- Claude Code installed
- Agentic Memorizer installed and initialized
- Background daemon running (recommended)

### Automatic Setup

The easiest way to setup Claude Code integration:

```bash
# During initialization
agentic-memorizer init --setup-integrations --with-daemon

# Or setup after installation
agentic-memorizer integrations setup claude-code
```

This automatically:
1. Detects your Claude settings file (`~/.claude/settings.json`)
2. Adds SessionStart hooks for all 4 matchers
3. Configures the correct command with integration flag
4. Preserves any existing hooks you have configured

### Verify Setup

```bash
# Check if Claude Code integration is configured
agentic-memorizer integrations detect

# Validate the configuration
agentic-memorizer integrations validate
```

## How It Works

### SessionStart Hooks

The Claude Code adapter configures SessionStart hooks that run automatically when Claude Code starts a new session. There are 4 hook matchers:

1. **startup** - When Claude Code first launches
2. **resume** - When resuming a previous session
3. **clear** - When clearing context
4. **compact** - When compacting context

All 4 matchers are configured to ensure your memory index loads in all scenarios.

### Command Execution

The hook executes this command:

```bash
agentic-memorizer read --format xml --integration claude-code
```

Breaking this down:
- `read` - Loads the precomputed index
- `--format xml` - Uses XML format (structured, semantic hierarchy)
- `--integration claude-code` - Wraps output in SessionStart JSON envelope

### SessionStart JSON Format

The adapter wraps the formatted index in a JSON structure that Claude Code expects:

```json
{
  "continue": true,
  "suppressOutput": true,
  "systemMessage": "Memory index updated: 15 files (5 documents, 3 images), 2.3 MB total",
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "<memory_index>...</memory_index>"
  }
}
```

**Fields**:
- `continue`: Always `true` - allows session to proceed normally
- `suppressOutput`: Always `true` - prevents verbose index from appearing in transcript
- `systemMessage`: Concise summary shown to user in UI
- `hookSpecificOutput.additionalContext`: Full formatted index added to Claude's context

## Configuration

### Settings File Location

The adapter modifies:
```
~/.claude/settings.json
```

### Manual Configuration

If you prefer to manually configure the hooks, add this to your `~/.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer read --format xml --integration claude-code",
            "timeout": 5.0
          }
        ]
      },
      {
        "matcher": "resume",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer read --format xml --integration claude-code",
            "timeout": 5.0
          }
        ]
      },
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer read --format xml --integration claude-code",
            "timeout": 5.0
          }
        ]
      },
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/agentic-memorizer read --format xml --integration claude-code",
            "timeout": 5.0
          }
        ]
      }
    ]
  }
}
```

**Important**: Replace `/path/to/agentic-memorizer` with the actual path to your binary.

### Binary Path Detection

The adapter auto-detects the binary path by:
1. Checking the current executable path
2. Looking in common installation locations:
   - `~/.local/bin/agentic-memorizer`
   - `~/go/bin/agentic-memorizer`
   - `/usr/local/bin/agentic-memorizer`
3. Searching PATH

You can override with `--binary-path` flag:

```bash
agentic-memorizer integrations setup claude-code --binary-path /custom/path/agentic-memorizer
```

## Usage

Once configured, the integration works automatically. You don't need to do anything special.

### Every Claude Code Session

When you start Claude Code (or resume/clear/compact), the hook automatically:
1. Executes `agentic-memorizer read --format xml --integration claude-code`
2. Loads the precomputed index (10-50ms)
3. Wraps it in SessionStart JSON
4. Claude Code adds the index to context
5. Session starts with full memory awareness

### What Claude Sees

Claude receives the full memory index in its context, which includes for each file:
- File path and name
- File type and category
- Size and modification time
- Metadata (page count, dimensions, word count, etc.)
- AI-generated summary of content
- Semantic tags and key topics
- Instructions on how to access the file

### Performance

- **Hook execution time**: <100ms total
- **Index load time**: 10-50ms (from precomputed index)
- **No impact on startup**: All analysis done in background by daemon
- **No repeated API calls**: Analysis cached, only new/modified files analyzed

## Output Formats

The Claude Code integration supports all three output formats:

### XML (Default - Recommended)

```bash
command: "agentic-memorizer read --format xml --integration claude-code"
```

Structured XML with semantic hierarchy. Best for Claude to parse and understand relationships.

### Markdown

```bash
command: "agentic-memorizer read --format markdown --integration claude-code"
```

Human-readable markdown format. Useful if you want to review what Claude sees.

### JSON

```bash
command: "agentic-memorizer read --format json --integration claude-code"
```

Pretty-printed JSON of the index structure. Good for programmatic access.

All formats are wrapped in the same SessionStart JSON envelope.

## Troubleshooting

### Integration Not Detected

```bash
$ agentic-memorizer integrations detect
No agent frameworks detected on this system
```

**Solution**: Check if `~/.claude/` directory exists. If not, launch Claude Code at least once to create it.

### Integration Setup Fails

```bash
Error: failed to setup claude-code: failed to write settings: permission denied
```

**Solution**: Check file permissions on `~/.claude/settings.json`. You need write permission.

### Hook Not Executing

If the memory index isn't loading in Claude Code sessions:

1. **Check daemon is running**:
   ```bash
   agentic-memorizer daemon status
   ```
   If not running, start it:
   ```bash
   agentic-memorizer daemon start
   ```

2. **Verify index exists**:
   ```bash
   ls -la ~/.agentic-memorizer/index.json
   ```
   If missing, the daemon hasn't completed its first indexing cycle yet.

3. **Test command manually**:
   ```bash
   agentic-memorizer read --format xml --integration claude-code
   ```
   Should output SessionStart JSON. If this fails, the hook will fail too.

4. **Check Claude Code settings**:
   ```bash
   cat ~/.claude/settings.json | grep -A 20 SessionStart
   ```
   Verify hooks are present for all matchers.

5. **Validate integration**:
   ```bash
   agentic-memorizer integrations validate
   ```

### Hooks Disappear After Update

If you update Claude Code and hooks disappear:

```bash
agentic-memorizer integrations setup claude-code
```

This will reconfigure the hooks.

### Performance Issues

If hook execution is slow:

1. **Check index file size**:
   ```bash
   ls -lh ~/.agentic-memorizer/index.json
   ```
   Large indexes (>10MB) may slow loading. Consider reducing files in memory directory.

2. **Check daemon status**:
   ```bash
   agentic-memorizer daemon status
   ```
   If daemon is rebuilding, wait for it to complete.

3. **Reduce timeout if needed**: Edit settings.json and increase timeout value (default: 5.0 seconds).

## Advanced Usage

### Multiple Memory Directories

If you use different memory directories for different projects:

1. Create separate config files
2. Use environment variable to switch:
   ```bash
   export AGENTIC_MEMORIZER_CONFIG=/path/to/project/config.yaml
   ```
3. Hook will use the config specified by environment

### Custom System Message

The `systemMessage` field is generated automatically based on index statistics. To customize, you would need to modify the adapter code in `internal/integrations/adapters/claude/output.go`.

### Selective Matcher Configuration

If you only want hooks on certain matchers, manually edit `~/.claude/settings.json` and remove unwanted matchers.

## Maintenance

### Updating the Integration

To update after changing binary location:

```bash
agentic-memorizer integrations setup claude-code --binary-path /new/path/agentic-memorizer
```

This will update all 4 matcher commands.

### Removing the Integration

```bash
agentic-memorizer integrations remove claude-code
```

This removes all SessionStart hooks for agentic-memorizer from your settings.

### Checking Integration Health

```bash
# Quick check
agentic-memorizer integrations validate

# Detailed status
agentic-memorizer integrations list
```

## Best Practices

1. **Always run the daemon**: Without it, hooks will fail or be slow
   ```bash
   agentic-memorizer daemon start
   ```

2. **Organize your memory directory**: Use subdirectories to categorize files
   ```
   ~/.agentic-memorizer/memory/
   ├── documents/
   ├── presentations/
   ├── images/
   └── transcripts/
   ```

3. **Monitor daemon logs**: Check for indexing errors
   ```bash
   agentic-memorizer daemon logs
   ```

4. **Keep index reasonable**: Don't put thousands of large files in memory
   - Daemon processes all files on startup
   - Large indexes take longer to load in hooks
   - Be selective about what you add

5. **Test before relying on it**: Manually run the read command to verify output
   ```bash
   agentic-memorizer read --integration claude-code
   ```

## References

- [Claude Code Hooks Documentation](https://docs.claude.com/en/docs/claude-code/hooks)
- [SessionStart Hook Specification](https://docs.claude.com/en/docs/claude-code/hooks#sessionstart-decision-control)
- [Architecture Documentation](../architecture.md)
- [Integration Interface](../../internal/integrations/adapters/claude/)
