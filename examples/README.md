# Configuration Examples

This directory contains example configuration files for Agentic Memorizer.

## Files

### config-basic.yaml

Basic configuration suitable for first-time users:
- Daemon disabled (manual indexing)
- Default settings
- Environment variable for API key
- Suitable for trying out the system

**Use when:**
- First time using Agentic Memorizer
- Want to understand configuration options
- Testing on-demand indexing

### config-with-integrations.yaml

Production-ready configuration with daemon enabled:
- Background daemon for automatic indexing
- Optimized performance settings
- Integration configuration placeholder
- Suitable for daily use

**Use when:**
- Regular daily use with Claude Code or other frameworks
- Want fast startup times (<50ms)
- Have reliable API key setup

## Usage

Copy an example to your config directory:

```bash
# Copy basic config
cp examples/config-basic.yaml ~/.agentic-memorizer/config.yaml

# Or copy production config
cp examples/config-with-integrations.yaml ~/.agentic-memorizer/config.yaml
```

Then customize the settings for your needs.

## Configuration Options

### Memory Root

```yaml
memory_root: ~/.agentic-memorizer/memory
```

Where to store files for indexing. Can be:
- Absolute path: `/Users/you/Documents/memory`
- Home-relative: `~/Documents/memory`
- Config-relative: `~/.agentic-memorizer/memory` (default)

### Claude API

```yaml
claude:
  api_key_env: ANTHROPIC_API_KEY
  model: claude-sonnet-4-5-20250929
  max_tokens: 1500
```

**Recommended**: Use environment variable for API key:
```bash
export ANTHROPIC_API_KEY="your-key-here"
```

### Output Format

```yaml
output:
  format: xml  # xml, markdown, or json
  verbose: false
```

- **xml**: Structured, semantic hierarchy (recommended for Claude)
- **markdown**: Human-readable
- **json**: Programmatic access

### Analysis

```yaml
analysis:
  enable: true
  max_file_size: 10485760  # 10 MB
  parallel: 3
```

- `enable`: Turn on/off semantic analysis (requires API key)
- `max_file_size`: Skip files larger than this
- `parallel`: Number of concurrent API calls (higher = faster, more API usage)

### Daemon

```yaml
daemon:
  enabled: true
  workers: 3
  rate_limit_per_min: 20
```

- `enabled`: Run background daemon for automatic indexing
- `workers`: Parallel file processing (3 recommended)
- `rate_limit_per_min`: API call throttling (20 default, adjust if hitting limits)

**With daemon enabled:**
- Auto-indexes files in background
- <50ms startup for read command
- No manual rebuilds needed

**With daemon disabled:**
- Manual indexing only
- Slower startup (analyzes on read)
- Suitable for occasional use

## Integration Setup

Integrations are configured via CLI commands, not config file:

```bash
# Setup Claude Code integration
agentic-memorizer integrations setup claude-code

# Setup other integrations
agentic-memorizer integrations setup continue
agentic-memorizer integrations setup cline
```

The `integrations` section in config is reserved for future use.

## Common Customizations

### High API Usage Scenario

If you have many files and want fast indexing:

```yaml
daemon:
  workers: 5  # More parallel workers
  rate_limit_per_min: 40  # Higher rate limit
```

### Low API Usage Scenario

If you want to minimize API usage:

```yaml
daemon:
  workers: 1  # Single worker
  rate_limit_per_min: 10  # Lower rate limit

analysis:
  parallel: 1  # One API call at a time
```

### Large File Support

To analyze larger files:

```yaml
analysis:
  max_file_size: 52428800  # 50 MB
```

### Custom Cache Location

```yaml
analysis:
  cache_dir: /custom/path/cache
```

### Debug Logging

```yaml
daemon:
  log_level: debug

output:
  verbose: true
```

## Validation

Validate your config after making changes:

```bash
# Validate config file syntax and settings
agentic-memorizer config validate

# Check if daemon is running
agentic-memorizer daemon status

# Check read command works
agentic-memorizer read --format xml

# Validate integrations
agentic-memorizer integrations validate
```

## Troubleshooting

### Config not found

```
Error: failed to initialize configuration
```

**Solution**: Run `agentic-memorizer init` to create default config

### Invalid YAML

```
Error: failed to parse config: yaml: ...
```

**Solution**: Check YAML syntax, especially indentation

### API key not found

```
Error: ANTHROPIC_API_KEY not set
```

**Solution**:
```bash
export ANTHROPIC_API_KEY="your-key-here"
```

Or set in config (not recommended):
```yaml
claude:
  api_key: "your-key-here"
```

## References

- [Main README](../README.md)
- [Subsystem Documentation](../docs/subsystems/)
- [CHANGELOG](../CHANGELOG.md)
