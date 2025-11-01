# Generic Integration Guide

**Integration Type**: Manual Setup
**Status**: Generic Support
**Frameworks**: Continue.dev, Cline, Aider, Cursor AI, and others

## Overview

Generic integrations provide manual setup instructions for agent frameworks that don't have automatic setup support. While these integrations can't automatically configure your framework, they provide the exact commands you need to add to your framework's configuration.

## Supported Frameworks

The following frameworks have generic adapter support:

- **Continue.dev** - VS Code extension for AI coding assistance
- **Cline** - AI coding assistant
- **Aider** - AI pair programming in your terminal
- **Cursor AI** - AI-powered code editor
- **Custom** - Any other framework that can execute shell commands

## Quick Start

### 1. Get Setup Instructions

```bash
agentic-memorizer integrations setup <framework-name>
```

Example for Continue.dev:
```bash
agentic-memorizer integrations setup continue
```

This will output:
```
Warning: continue does not appear to be installed (auto-detection may not work for all frameworks)
Attempting setup anyway...

Setting up continue integration...
Binary path: /path/to/agentic-memorizer

Error: automatic setup not supported for continue.

Please add this command to your framework's configuration:
  /path/to/agentic-memorizer read --format markdown

Consult your framework's documentation for how to add custom commands or tools
```

### 2. Add Command to Your Framework

Copy the command provided and add it to your framework's configuration file. See framework-specific instructions below.

### 3. Start the Daemon

Ensure the background daemon is running:

```bash
agentic-memorizer daemon start
```

### 4. Test the Integration

Manually run the command to verify it works:

```bash
/path/to/agentic-memorizer read --format markdown
```

You should see your memory index in markdown format.

## Framework-Specific Instructions

### Continue.dev

**Configuration File**: `~/.continue/config.json` or `~/.continue/config.ts`

Continue.dev uses a tools array for custom commands. You'll need to manually edit the config file.

**For JSON config** (`~/.continue/config.json`):

```json
{
  "models": [...],
  "tools": [
    {
      "name": "memory",
      "description": "Access agentic memory index with semantic understanding of all files",
      "command": "/path/to/agentic-memorizer read --format markdown"
    }
  ]
}
```

**For TypeScript config** (`~/.continue/config.ts`):

```typescript
export function modifyConfig(config: Config): Config {
  config.tools = config.tools || [];
  config.tools.push({
    name: "memory",
    description: "Access agentic memory index with semantic understanding of all files",
    command: "/path/to/agentic-memorizer read --format markdown"
  });
  return config;
}
```

**Usage in Continue**:
- Type `@memory` in your prompt to invoke the tool
- Continue will execute the command and include the output in context

### Cline

**Configuration**: VS Code extension settings or `.cline/config.ts`

Cline configuration depends on your setup. Common approaches:

**Option 1: Custom tools in settings.json**

Add to VS Code settings (`.vscode/settings.json`):

```json
{
  "cline.customTools": [
    {
      "name": "memory",
      "command": "/path/to/agentic-memorizer read --format markdown",
      "description": "Load memory index"
    }
  ]
}
```

**Option 2: Startup command**

If Cline supports startup commands:

```json
{
  "cline.startupCommands": [
    "/path/to/agentic-memorizer read --format markdown"
  ]
}
```

**Note**: Cline configuration may vary by version. Consult Cline documentation for the correct configuration method.

### Aider

**Configuration File**: `.aider.conf.yml` or command-line arguments

Aider may not have a direct plugin system, but you can include the memory index in your messages:

**Option 1: Pre-load before starting Aider**

```bash
# Save index to file
agentic-memorizer read --format markdown > /tmp/memory-index.md

# Start Aider with the file
aider --read /tmp/memory-index.md
```

**Option 2: Include in initial message**

```bash
# Start Aider
aider

# First message includes memory
/add $(agentic-memorizer read --format markdown)
```

**Option 3: Shell alias**

```bash
# Add to ~/.bashrc or ~/.zshrc
alias aider-memory='agentic-memorizer read --format markdown > /tmp/memory.md && aider --read /tmp/memory.md'

# Use it
aider-memory
```

### Cursor AI

**Configuration**: Settings or custom commands

Cursor AI configuration depends on the version and feature set:

**Option 1: Custom commands**

If Cursor supports custom commands, add:

```json
{
  "cursor.customCommands": [
    {
      "name": "Load Memory",
      "command": "/path/to/agentic-memorizer read --format markdown"
    }
  ]
}
```

**Option 2: Include in context manually**

Run the command and copy output to your Cursor session:

```bash
agentic-memorizer read --format markdown | pbcopy  # macOS
agentic-memorizer read --format markdown | xclip -selection clipboard  # Linux
```

Then paste into Cursor.

### Custom Frameworks

For any framework that can execute shell commands or read files:

**Command to execute**:
```bash
/path/to/agentic-memorizer read --format markdown
```

**Alternative: Save to file**:
```bash
agentic-memorizer read --format markdown > ~/.agentic-memorizer/memory-index.md
```

Then configure your framework to read `~/.agentic-memorizer/memory-index.md`.

## Output Format Options

Generic integrations support all output formats:

### Markdown (Recommended for most frameworks)

```bash
agentic-memorizer read --format markdown
```

Human-readable, easy to parse, good for most AI models.

### XML

```bash
agentic-memorizer read --format xml
```

Structured format with semantic hierarchy. Good for frameworks that parse XML well.

### JSON

```bash
agentic-memorizer read --format json
```

Programmatic access to the index structure. Good for custom scripts or frameworks with JSON parsers.

## Customization

### Binary Path

When getting setup instructions, you can specify a custom binary path:

```bash
agentic-memorizer integrations setup continue --binary-path /custom/path/agentic-memorizer
```

This is useful if:
- You have multiple versions installed
- Binary is in a non-standard location
- You're using a wrapper script

### Update Frequency

Different strategies for keeping the index fresh:

**Strategy 1: Manual refresh**
- Run command when you want to update
- Good for infrequent changes

**Strategy 2: Automated refresh**
- Set up a cron job or systemd timer
- Good for frequently changing memory

**Strategy 3: On-demand**
- Configure framework to run command before each session
- Always fresh, but adds startup overhead

Example cron job (refresh every 5 minutes):
```bash
*/5 * * * * /path/to/agentic-memorizer read --format markdown > ~/.agentic-memorizer/memory-index.md
```

## Troubleshooting

### Command Not Found

```bash
sh: agentic-memorizer: command not found
```

**Solution**: Use absolute path to binary

```bash
# Find the binary
which agentic-memorizer

# Or use full path
/Users/yourname/.local/bin/agentic-memorizer read --format markdown
```

### No Index File

```bash
Error: failed to get index path: ...
```

**Solution**: Ensure initialization and daemon are running

```bash
# Initialize
agentic-memorizer init

# Start daemon
agentic-memorizer daemon start

# Wait for first index
sleep 10

# Try again
agentic-memorizer read --format markdown
```

### Permission Denied

```bash
Error: permission denied: ~/.agentic-memorizer/index.json
```

**Solution**: Check file permissions

```bash
chmod 644 ~/.agentic-memorizer/index.json
```

### Output Too Large

If the memory index is too large for your framework's context:

**Option 1: Reduce files**
- Remove less important files from memory directory
- Use skip patterns in config

**Option 2: Filter output**
- Use custom scripts to extract relevant sections
- Focus on specific categories

**Option 3: Summarize**
- Create a script that summarizes the index
- Include only file list and summaries, not full metadata

## Best Practices

1. **Use absolute paths**: Framework environments may not have correct PATH
   ```bash
   /Users/yourname/.local/bin/agentic-memorizer read --format markdown
   ```

2. **Test the command first**: Run manually before configuring framework
   ```bash
   /path/to/agentic-memorizer read --format markdown
   ```

3. **Keep daemon running**: Ensure fast performance
   ```bash
   agentic-memorizer daemon start
   ```

4. **Choose appropriate format**:
   - Markdown for readability
   - XML for structure
   - JSON for programmatic access

5. **Monitor performance**: Large indexes may slow your framework
   ```bash
   # Check index size
   ls -lh ~/.agentic-memorizer/index.json

   # Check command execution time
   time agentic-memorizer read --format markdown > /dev/null
   ```

6. **Version control your config**: Keep framework configuration in git
   ```bash
   git add ~/.continue/config.json
   git commit -m "Add agentic-memorizer integration"
   ```

## Limitations

Generic integrations have limitations compared to automatic integrations:

- ❌ No auto-detection of framework installation
- ❌ No automatic configuration updates
- ❌ No validation of framework configuration
- ❌ No framework-specific output wrapping
- ❌ Manual setup required for each machine

Benefits:
- ✅ Works with any framework that supports custom commands
- ✅ Simple, transparent integration
- ✅ No framework version dependencies
- ✅ Easy to debug and customize

## Getting Help

If you're having trouble integrating with a specific framework:

1. Check the framework's documentation for custom commands/tools
2. Try different output formats (markdown, xml, json)
3. Test the command manually first
4. Check agentic-memorizer logs: `agentic-memorizer daemon logs`
5. Open an issue: https://github.com/leefowlercu/agentic-memorizer/issues

## Future Automatic Support

If you'd like automatic setup support for your framework:

1. Research the framework's configuration format
2. Implement a dedicated adapter (see [custom.md](custom.md))
3. Submit a pull request
4. See [CONTRIBUTING.md](../CONTRIBUTING.md) for details

## References

- [Architecture Documentation](../architecture.md)
- [Custom Integration Guide](custom.md)
- [Generic Adapter Implementation](../../internal/integrations/adapters/generic/)
