# Output Formatting

Structured CLI output with multiple format support through a builder pattern and pluggable formatters.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-31

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Format subsystem provides a unified approach to CLI output formatting across all commands. Rather than using direct printf-style output, commands construct structured content using builder types, then render that content through format-specific formatters. This separation enables consistent output styling, machine-readable alternatives to human-readable text, and a single point of control for output conventions.

The subsystem implements a three-tier architecture: builders define structured content (sections, tables, lists, status messages), formatters render that content to specific formats (text, JSON, YAML, XML), and writers handle buffered I/O for efficient output. A thread-safe registry manages formatter discovery, enabling automatic registration via init functions.

Key capabilities include:

- **Builder pattern** - Fluent API for constructing structured content with validation
- **Multiple formats** - Text, JSON, YAML, and XML output from the same builders
- **Status messaging** - Severity-based status with symbols and color support
- **Hierarchical sections** - Nested key-value pairs with subsections up to 5 levels deep
- **Table formatting** - Columnar data with alignment control and compact mode
- **Registry pattern** - Thread-safe formatter lookup with auto-registration

## Design Principles

### Separation of Structure and Presentation

Builders define what content to display; formatters decide how to display it. Commands never directly format strings for output. Instead, they construct builders describing the content structure, then pass those builders to formatters. This enables switching output formats via a flag without changing command logic, ensures consistent styling across all commands, and allows format-specific optimizations in each formatter.

### Validation Before Rendering

All builders implement a Validate method that formatters call before rendering. This catches structural issues (empty titles, excessive nesting, mismatched column counts) before attempting to produce output. Validation errors propagate clearly with context about what failed, enabling actionable error messages.

### Fluent Builder API

Builder methods return the builder pointer for chaining, enabling readable construction without temporary variables. This pattern is used consistently across all builder types, creating a familiar API for command authors. Builders accumulate state through method calls, with validation deferred until formatting.

### Registry with Auto-Registration

Formatters register themselves via init functions, enabling automatic discovery when the formatters package is imported. The registry uses a thread-safe singleton pattern (matching integrations and semantic providers), allowing concurrent formatter lookups without explicit initialization. New formats can be added by implementing the Formatter interface and registering in an init function.

### Color and Terminal Awareness

The text formatter supports optional ANSI color codes for terminal output, with colors disabled by default for machine processing. Status severity maps to consistent colors (green for success, red for error, yellow for warning, blue for info/running). Utility functions strip ANSI codes for width calculations and non-terminal contexts.

## Key Components

### Formatter Interface and Registry

The Formatter interface defines methods for rendering builders: Format for single builders, FormatMultiple for arrays, Name for identification, and SupportsColors for capability detection. The Registry manages formatters with thread-safe lookup via GetFormatter and registration via RegisterFormatter. Five formatters are provided: text (with ANSI colors), json (structured data), yaml (human-readable structured), xml (with custom schemas for files and facts), and markdown (documentation-style output).

### Buildable Interface

All builder types implement Buildable, which requires Type (returning the builder's type constant) and Validate (checking structural integrity). Formatters use type switching on the Type method to dispatch to format-specific rendering, enabling extensibility without modifying formatter code.

### Section Builder

Section represents hierarchical key-value data with optional subsections. It supports titled sections with key-value pairs, plain text lines, and nested subsections up to 5 levels deep. Validation ensures non-empty titles, checks nesting depth, and detects circular references. Sections are commonly used for configuration display, status output, and resource details.

### Table Builder

Table represents columnar data with headers, rows, and alignment control. Columns can be aligned left, right, or center, with alignment applying consistently across all rows. Tables support compact mode for reduced spacing and header hiding for data-only output. Validation ensures all rows match header count and alignments match column count.

### List Builder

List represents ordered or unordered lists with optional nesting. Items can contain plain text or nested sublists, with nesting limited to 5 levels. Compact mode reduces spacing between items. Lists support both bullet markers (unordered) and numeric prefixes (ordered).

### Status Builder

Status represents severity-based messages with optional details. Six severity levels are defined: Success, Info, Warning, Error, Running, and Stopped. Each level has a default symbol and color. Status messages can include detail lines and custom symbol overrides. This builder is the primary way commands communicate success or failure.

### Progress Builder

Progress represents operation progress with three display modes: bar (visual bar with percentage), spinner (animation state), and percentage (numeric only). Progress tracks current and total values, with automatic percentage calculation. Bar mode supports configurable width.

### Error Builder

Error represents structured error messages with context. It includes error type (validation, runtime, input), message, optional field and value for validation errors, detail lines, and resolution suggestions. This enables consistent error presentation with actionable guidance.

### Files and Facts Content

FilesContent and FactsContent wrap domain types (FileIndex and FactsIndex) for formatting. These adapters enable the format system to render file index and facts output through the same formatter pipeline, with format-specific custom rendering in text and XML formatters.

### BufferedWriter

BufferedWriter provides efficient I/O with Write (without newline), WriteLine (with newline), Flush, and Close methods. It wraps bufio.Writer for buffering and supports stdout, file output (with automatic directory creation), and arbitrary io.Writers. Writers handle error propagation and resource cleanup.

### Utility Functions

The utils module provides formatting helpers used across formatters: FormatBytes for human-readable sizes (KB, MB, GB), FormatNumber for thousands separators, FormatDuration for human-readable durations, TruncateString for length limits, AlignText for column alignment with ANSI awareness, StripANSI for removing color codes, and color functions (Green, Red, Yellow, Blue, Bold) for ANSI formatting.

## Integration Points

### CLI Commands

All CLI commands use the format system for output. Commands construct appropriate builders (Status for results, Section for details, Table for lists, Error for failures), retrieve a formatter via GetFormatter, call Format on the builder, and print the result. Commands accept a --format flag for format selection, defaulting to text.

### Integration Hooks

Integration hooks (SessionStart, UserPromptSubmit) use FilesContent and FactsContent builders with the XML formatter to generate structured context payloads. The XML formatter provides custom rendering for files and facts with proper schema structure.

### Read Commands

The read files and read facts commands use FilesContent and FactsContent builders with user-selected formatters. This enables machine-readable output (JSON, YAML) for scripting alongside human-readable output (text) for interactive use.

### Daemon Status

Daemon status and health commands use Section and Status builders to present hierarchical status information. The section builder's subsection support enables organized presentation of daemon state, configuration, and metrics.

### Error Handling

CLI command error handling uses the Error builder for consistent error presentation. Validation errors include field and value context. Runtime errors include suggestions where applicable. The format system ensures all errors follow the same presentation pattern.

## Glossary

**Builder**
A type implementing fluent methods for constructing structured content. Builders accumulate state through method calls, validate structure on demand, and produce content that formatters render.

**Buildable**
The interface all builders implement, requiring Type and Validate methods. Formatters use this interface to handle any builder type uniformly.

**Formatter**
An implementation that renders builders to a specific output format. Formatters implement Format for single builders and FormatMultiple for arrays.

**Registry**
The thread-safe singleton managing formatter registration and lookup. Formatters register via init functions; consumers retrieve formatters by name.

**Section**
A builder for hierarchical key-value content with optional subsections, text lines, and dividers. Maximum nesting depth is 5 levels.

**Severity**
One of six status levels (Success, Info, Warning, Error, Running, Stopped) that determine status symbol and color in text output.

**Status**
A builder for severity-based messages with optional details and custom symbols. The primary mechanism for communicating command results.

**Table**
A builder for columnar data with headers, rows, and per-column alignment (left, right, center). Supports compact mode and header hiding.

**Writer**
An interface for buffered output with Write, WriteLine, Flush, and Close methods. BufferedWriter implements this for stdout and file output.
