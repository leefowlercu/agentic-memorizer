# Format Subsystem

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
  - [Separation of Concerns](#separation-of-concerns)
  - [Extensibility Through Registration](#extensibility-through-registration)
  - [Type Safety and Validation](#type-safety-and-validation)
  - [Consistent Rendering](#consistent-rendering)
- [Key Components](#key-components)
  - [Builder Layer](#builder-layer)
  - [Formatter Layer](#formatter-layer)
  - [Writer Layer](#writer-layer)
  - [Utility Functions](#utility-functions)
- [Integration Points](#integration-points)
  - [CLI Commands](#cli-commands)
  - [Integration Adapters](#integration-adapters)
  - [Daemon HTTP API](#daemon-http-api)
- [Glossary](#glossary)

## Overview

The Format subsystem provides centralized CLI output formatting for agentic-memorizer. It implements a three-tier architecture that separates content construction from presentation, enabling consistent output across all commands while supporting multiple output formats (text, JSON, YAML, XML, markdown).

The subsystem eliminates ad-hoc `fmt.Printf` calls throughout the codebase by providing structured builders for common output patterns (status messages, tables, sections, lists, progress indicators, errors). Each builder can be rendered to any registered format, enabling commands to easily support multiple output modes via flags.

**Key Characteristics:**
- **Format-agnostic content**: Builders describe what to display, formatters decide how to display it
- **Multi-format support**: Single builder can render to text, JSON, YAML, XML, or markdown
- **Consistent styling**: Unified symbols, colors, and alignment across all output
- **Thread-safe registry**: Formatters can be registered and retrieved concurrently

## Design Principles

### Separation of Concerns

The subsystem enforces strict separation between three responsibilities:

1. **Content Construction** (Builders): Commands create structured representations of their output without knowing the target format. A status message, table, or section is built once and can be rendered to any format.

2. **Format Rendering** (Formatters): Each formatter understands how to render builders to a specific output format. Formatters handle format-specific details like JSON escaping, XML structure, or ANSI color codes.

3. **I/O Management** (Writers): Writers handle buffered output and error handling, abstracting away file vs stdout differences and ensuring data is flushed correctly.

This separation enables commands to focus on what to display while remaining agnostic to how it will be presented.

### Extensibility Through Registration

The subsystem uses a registry pattern for formatters, allowing new output formats to be added without modifying existing code. Formatters register themselves during package initialization via `init()` functions, making the system self-configuring.

**Registration Flow:**
- Each formatter package imports the base format package
- Formatter init() functions call `format.RegisterFormatter(name, formatter)`
- Commands retrieve formatters by name via `format.GetFormatter(name)`
- Missing formatters return clear errors rather than runtime panics

This pattern mirrors the handler registration approach used in the metadata subsystem, providing a consistent extension mechanism across the codebase.

### Type Safety and Validation

All builders implement the `Buildable` interface, which requires a `Validate() error` method. Validation occurs before rendering, catching construction errors early with specific error messages.

**Validation Enforces:**
- Non-empty required fields (titles, messages, headers)
- Correct array sizes (alignments match column count)
- Maximum nesting depths (prevents stack overflow)
- Valid enum values (status severities, list types)
- Circular reference detection (prevents infinite recursion)

Formatters call `Validate()` before rendering, ensuring invalid builders never reach the rendering phase. This prevents runtime panics and provides actionable error messages to developers.

### Consistent Rendering

The subsystem provides shared utilities and constants that ensure consistency across all formatters:

- **Status Symbols**: Unified symbols (✓, ✗, ⚠, ○, ▸, ■) for success, error, warning, info, running, stopped
- **Color Codes**: Centralized ANSI color helpers (Green, Red, Yellow, Blue, Bold)
- **Number Formatting**: Consistent byte sizes (1.5 MB), thousands separators (1,234,567), durations (2.5h)
- **Text Alignment**: Shared alignment logic with ANSI-aware length calculation

Formatters use these utilities rather than implementing their own, ensuring status messages look identical whether rendered as text, markdown, or JSON (structurally).

## Key Components

### Builder Layer

Builders construct structured representations of output content. Each builder type corresponds to a common CLI output pattern.

**Available Builder Types:**

1. **Status** (`status.go`): Status messages with severity levels (success, error, warning, info, running, stopped), optional detail lines, and custom symbols.

2. **Section** (`section.go`): Hierarchical key-value pairs with headers, dividers, and nestable subsections. Supports up to 5 levels of nesting with automatic indentation.

3. **Table** (`table.go`): Columnar data with headers, per-column alignment (left, center, right), optional header hiding, and compact mode for reduced spacing.

4. **List** (`list.go`): Ordered or unordered lists with up to 5 levels of nesting. Supports compact mode for space efficiency.

5. **Progress** (`progress.go`): Progress indicators including progress bars, percentages, and spinners. Used primarily in long-running operations and E2E tests.

6. **Error** (`error.go`): Structured error messages with field names, values, detail lines, and suggestions. Distinguishes validation errors from runtime errors.

7. **GraphContent** (`graph.go`): Special-purpose builder wrapping `GraphIndex` for integration output. Preserves backward compatibility with legacy XML/markdown formats.

**Builder Characteristics:**
- Immutable after construction (fluent APIs return new builders)
- Self-validating via `Validate() error` method
- Type-safe via `Buildable` interface
- Format-agnostic (no references to JSON, XML, etc.)

### Formatter Layer

Formatters implement the `Formatter` interface, converting builders to specific output formats. Each formatter handles the structural requirements of its target format.

**Registered Formatters:**

1. **Text** (`formatters/text.go`): Plain text with box-drawing characters, optional ANSI colors, aligned columns. Default formatter for CLI output.

2. **JSON** (`formatters/json.go`): Structured JSON with proper escaping, pretty-printing, and no HTML entity escaping. Used for machine-readable output and API responses.

3. **YAML** (`formatters/yaml.go`): YAML format with proper indentation and type preservation. Useful for configuration-like output.

4. **XML** (`formatters/xml.go`): Well-formed XML with proper escaping and namespacing. Maintains backward compatibility with legacy graph index format.

5. **Markdown** (`formatters/markdown.go`): Rich markdown with emojis, tables, and formatting. Used for integration output and documentation generation.

**Formatter Responsibilities:**
- Render builders to format-specific strings
- Handle format-specific escaping (XML entities, JSON quotes)
- Apply format-specific structure (JSON objects, XML tags)
- Support both single and multiple builder rendering
- Provide format name and color support information

**Registry Management:**
The global registry (`defaultRegistry`) manages all formatters with thread-safe registration and lookup. Commands access formatters via `GetFormatter(name)`, which returns descriptive errors for unknown formats.

### Writer Layer

Writers provide buffered I/O with error handling, abstracting file vs stdout differences. Commands use writers to handle output without worrying about flushing or error propagation.

**Writer Types:**

1. **BufferedWriter** (`writer.go`): Generic buffered writer wrapping any `io.Writer`. Handles automatic flushing and close semantics.

2. **StdoutWriter**: Convenience constructor for stdout output. Most commands use this for terminal output.

3. **FileWriter**: File-based writer with automatic parent directory creation and safe file permissions (0644).

**Writer Characteristics:**
- Buffered for performance (reduces syscall overhead)
- Explicit flush control for timing-sensitive output
- Error propagation with context-rich messages
- Automatic cleanup via `Close()` method

### Utility Functions

The subsystem provides shared formatting utilities used by formatters and commands:

**Number Formatting** (`utils.go`):
- `FormatBytes(int64)`: Human-readable byte sizes (1.5 MB, 2.3 GB, 15.7 TB)
- `FormatNumber(int64)`: Thousands separators (1,234,567)
- `FormatDuration(time.Duration)`: Human-readable durations (2.5h, 30m, 15s)

**Text Utilities** (`utils.go`):
- `TruncateString(string, int)`: Truncate with ellipsis
- `AlignText(string, int, Alignment)`: ANSI-aware text alignment
- `StripANSI(string)`: Remove ANSI color codes for length calculation

**Status Symbols** (`utils.go`):
- `GetStatusSymbol(StatusSeverity)`: Consistent symbols for all severity levels
- Predefined constants for all symbols (SymbolSuccess, SymbolError, etc.)

**Color Helpers** (`utils.go`):
- `Green(string)`, `Red(string)`, `Yellow(string)`, `Blue(string)`, `Bold(string)`
- Only used in text formatter when colors are enabled
- Other formatters ignore colors (JSON, XML) or use format-specific styling (markdown emojis)

## Integration Points

### CLI Commands

CLI commands are the primary consumers of the format subsystem. Commands use builders to construct output and formatters to render it.

**Typical Command Pattern:**

Commands define helper functions (often in `subcommands/helpers.go`) that encapsulate the format-get-render pattern:

```
func outputStatus(status *format.Status) error {
    formatter, err := format.GetFormatter("text")
    if err != nil {
        return fmt.Errorf("failed to get formatter; %w", err)
    }
    output, err := formatter.Format(status)
    if err != nil {
        return fmt.Errorf("failed to format status; %w", err)
    }
    fmt.Println(output)
    return nil
}
```

Commands then use these helpers in their `RunE` functions:

```
status := format.NewStatus(format.StatusSuccess, "Operation completed")
status.AddDetail("Files processed: 42")
return outputStatus(status)
```

**Commands Using Format Package:**
- `cmd/daemon/subcommands/`: Status, start, stop, restart (status messages)
- `cmd/cache/subcommands/`: Status, clear (sections, tables, status messages)
- `cmd/config/subcommands/`: Validate, show-schema, reload (sections, errors)
- `cmd/graph/subcommands/`: Status (sections, status messages)
- `cmd/integrations/subcommands/`: List, health, detect (tables, sections)
- `cmd/version/`: Version (section)
- `cmd/read/`: Read (graph content with multiple format support)

### Integration Adapters

Integration adapters use formatters to generate framework-specific output. The `GraphContent` builder provides a unified interface for rendering the graph index.

**Integration Output Paths:**

1. **Claude Code Hook** (`internal/integrations/adapters/claude/hook_output.go`): Supports multiple formatters (XML, Markdown, JSON) for SessionStart hook injection. Defaults to XML format. Wraps formatted output in JSON envelope with systemMessage field.

2. **MCP Server** (`internal/mcp/server.go`): Provides all three formatters (XML, Markdown, JSON) via resource URIs (`memorizer://index`, `memorizer://index/markdown`, `memorizer://index/json`). Tool responses use JSON-RPC 2.0 structure with formatted content.

3. **MCP Adapters** (`internal/integrations/adapters/*/mcp_adapter.go`): Do not use formatters directly. Output is provided through MCP protocol resources and tool responses rather than via FormatOutput method.

**Why GraphContent Builder?**

The GraphContent builder wraps `types.GraphIndex` to integrate with the format system while maintaining backward compatibility with legacy XML/markdown output formats. This allows integration code to use the same formatter infrastructure as CLI commands.

### Daemon HTTP API

The daemon HTTP API uses formatters indirectly through the integration adapters. API responses are rendered to JSON via the JSON formatter, ensuring consistent structure across CLI and HTTP interfaces.

**API Response Pattern:**

The daemon serves formatted graph content via the `/api/v1/graph` endpoint, using the same GraphContent builder and JSON formatter that the `read` command uses. This ensures CLI and API output remain synchronized.

## Glossary

**Builder**: Structured representation of output content, independent of target format. Implements `Buildable` interface with `Type()` and `Validate()` methods.

**Formatter**: Renderer that converts builders to specific output formats (text, JSON, YAML, XML, markdown). Implements `Formatter` interface.

**Writer**: Buffered I/O abstraction for output, handling file vs stdout differences. Implements `Writer` interface with `Write()`, `WriteLine()`, `Flush()`, `Close()` methods.

**Registry**: Thread-safe global registry managing formatter registration and lookup. Accessed via `RegisterFormatter()` and `GetFormatter()` package functions.

**Buildable**: Interface implemented by all builders, requiring `Type() BuilderType` and `Validate() error` methods.

**Formatter Interface**: Core interface defining `Format(Buildable)`, `FormatMultiple([]Buildable)`, `Name()`, and `SupportsColors()` methods.

**Validation**: Pre-render checking of builder construction, enforcing required fields, correct sizes, and structural constraints. Returns descriptive errors for invalid builders.

**Status Severity**: Enum representing status message levels: success, error, warning, info, running, stopped. Maps to consistent symbols and colors.

**Alignment**: Enum for text/column alignment: left, center, right. Used in tables and text utilities.

**ANSI-aware**: Functions that correctly handle ANSI escape codes when calculating text length, preventing misalignment in colored output.

**Fluent API**: Builder methods that return `*Builder`, enabling method chaining: `NewStatus(...).AddDetail(...).WithSymbol(...)`.

**Registry Pattern**: Design pattern using global registry for component registration, enabling extension without modifying core code. Used for both formatters and metadata handlers.

**GraphContent**: Special builder wrapping `types.GraphIndex` for integration output, preserving backward compatibility with legacy XML/markdown formats.

**init() Registration**: Go pattern where packages register components during initialization, making systems self-configuring without explicit wiring.

---

**Last Updated:** 2025-12-09
