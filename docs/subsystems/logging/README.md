# Structured Logging

Centralized logging infrastructure with slog integration, file rotation, context propagation, and standardized field names for consistent observability.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-31

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Structured Logging subsystem provides centralized logging infrastructure built on Go's standard log/slog package. Rather than scattering logging configuration throughout the codebase, this subsystem offers a factory pattern for creating loggers with consistent configuration: file rotation via lumberjack, handler type selection (JSON or text), log level parsing, and context propagation. The subsystem also defines standardized field names aligned with OpenTelemetry conventions.

The subsystem addresses several logging challenges: ensuring consistent structured fields across components, managing log file rotation without manual intervention, propagating logger instances through request contexts, and generating time-ordered identifiers for correlation. All log messages use lowercase formatting per project conventions.

Key capabilities include:

- **Logger factory** - Functional options pattern for creating configured slog.Logger instances
- **File rotation** - Automatic log rotation via lumberjack with size, backup, and age limits
- **Handler types** - JSON for machine parsing, text for human readability
- **Standardized fields** - Consistent field names aligned with OpenTelemetry conventions
- **Context propagation** - Store and retrieve loggers from context.Context
- **UUIDv7 identifiers** - Time-ordered unique IDs for process, session, and client correlation
- **Logger enrichment** - Helper functions for adding component and client metadata

## Design Principles

### Factory with Functional Options

The NewLogger function uses functional options (WithLogFile, WithLogLevel, WithHandler, WithAdditionalOutputs) for flexible configuration. This pattern allows callers to specify only the options they need while providing sensible defaults. The factory returns both the slog.Logger and the underlying lumberjack.Logger for hot-reload scenarios.

### slog as Foundation

The subsystem builds on Go's standard log/slog package rather than third-party logging libraries. This ensures compatibility with the broader Go ecosystem, takes advantage of slog's structured logging capabilities, and avoids external dependencies for core logging functionality. Handler creation wraps slog.NewTextHandler and slog.NewJSONHandler.

### Standardized Field Names

Field names use dot notation (process.id, client.type, session.id) aligned with OpenTelemetry semantic conventions. This enables future integration with observability platforms and ensures consistent structured data across all log entries. Field names are defined as constants to prevent typos and enable refactoring.

### Logger Enrichment Pattern

Rather than passing individual fields to each log call, the subsystem provides enrichment functions (WithProcessID, WithComponent, WithClientInfo) that return a new logger with fields pre-attached. This reduces boilerplate and ensures consistent field presence within a component's logs.

### Context-Based Propagation

Loggers can be stored in context.Context and retrieved later, enabling request-scoped logging without explicit logger passing through every function. The FromContext function returns a fallback logger if none is found, preventing nil pointer issues.

### Time-Ordered UUIDs

The subsystem uses UUIDv7 for process, session, and client identifiers. UUIDv7 includes a timestamp component, making IDs naturally sortable by creation time. This aids debugging by enabling chronological ordering of related events across distributed components.

### Automatic Rotation

Log files are automatically rotated based on size (default 10MB), with configurable retention (default 3 backups, 28 days). Old files are compressed to save space. This prevents log files from growing unbounded while preserving history for debugging.

## Key Components

### LoggerConfig Struct

The LoggerConfig struct holds all configuration for logger creation: LogFile path (empty means stdout/stderr), LogLevel as string (debug, info, warn, error), Handler type (JSON or Text), and Outputs for additional writers.

### LoggerOption Type

The LoggerOption type defines functional options for NewLogger. Each option function modifies the LoggerConfig. Available options include WithLogFile, WithLogLevel, WithHandler, and WithAdditionalOutputs.

### NewLogger Function

The NewLogger factory creates a configured slog.Logger. It applies functional options to defaults, parses the log level, creates the appropriate handler, and sets up file rotation if a log file is specified. Returns the logger, the lumberjack.Logger for hot-reload, and any error.

### ParseLogLevel Function

The ParseLogLevel function converts string level names to slog.Level values. Supports debug, info, warn, and error (case-insensitive). Returns an error for invalid levels with a helpful message listing valid options.

### RotationConfig Struct

The RotationConfig struct holds lumberjack rotation settings: MaxSize in megabytes before rotation, MaxBackups count of old files to retain, MaxAge in days for retention, and Compress flag for gzip compression.

### NewRotatingWriter Function

The NewRotatingWriter function creates a lumberjack.Logger with default rotation settings (10MB size, 3 backups, 28 days, compression enabled). Used by the logger factory when a log file is specified.

### Field Constants

The fields.go file defines standardized field name constants: FieldProcessID, FieldSessionID, FieldRequestID, FieldClientID, FieldClientType, FieldClientVersion, FieldComponent, FieldClientName, and FieldTraceID. Using constants prevents typos and enables IDE autocomplete.

### Component Constants

Component name constants (ComponentMCPServer, ComponentSSEClient, ComponentDaemonSSE, ComponentGraphManager) provide consistent component identification in log entries.

### Header Constants

HTTP header constants (HeaderClientID, HeaderClientType, HeaderClientVersion) standardize client identification headers used by the daemon SSE endpoint.

### Identifier Functions

NewProcessID, NewSessionID, and NewClientID generate UUIDv7 identifiers for correlation. Each falls back to UUIDv4 if UUIDv7 generation fails (extremely unlikely). IsValidUUIDv7 provides validation for testing.

### Enrichment Functions

WithProcessID, WithSessionID, WithComponent, WithClientInfo, WithMCPProcess, WithSSEClient, and WithDaemonSSE add standardized fields to loggers. Each returns a new logger with additional fields attached.

### Context Functions

WithLogger stores a logger in context. FromContext retrieves a logger from context with a fallback. WithProcessIDContext and ProcessIDFromContext handle process ID propagation through context.

## Integration Points

### Daemon Subsystem

The daemon creates its logger via NewLogger with file rotation configured from daemon.log_file. The logger is enriched with component information and passed to subsystems (worker pool, watcher, graph manager). Hot-reload updates log level without recreating the logger.

### MCP Server

The MCP server creates a separate logger for its process, using WithMCPProcess for enrichment. MCP logging goes to a dedicated file (mcp.log_file) to separate MCP protocol messages from daemon logs. The SSE client uses WithSSEClient for component identification.

### SSE Hub

The daemon's SSE hub uses WithDaemonSSE to enrich connection logs with client ID, type, and version from HTTP headers. This enables tracing client connections and debugging session issues.

### Configuration System

The config subsystem defines log-related settings: daemon.log_file, daemon.log_level, mcp.log_file, mcp.log_level. Log level is hot-reloadable without daemon restart. File paths require restart to change.

### CLI Commands

The daemon start and mcp start commands use the logging factory to create their respective loggers. They pass configuration values from the config subsystem to the factory options.

## Glossary

**Component**
A logical part of the application identified in logs via the component field. Examples: mcp-server, sse-client, daemon-sse, graph-manager.

**Enrichment**
The process of adding standardized fields to a logger instance. Creates a new logger with fields pre-attached to all subsequent log entries.

**Functional Options**
A Go pattern using variadic function parameters to configure struct creation. Enables optional configuration with sensible defaults.

**Handler**
An slog.Handler implementation that formats and writes log entries. The subsystem supports JSON handlers (machine-readable) and text handlers (human-readable).

**Lumberjack**
A third-party Go library for log file rotation. Handles automatic file rotation based on size, retention of old files, and compression.

**Process ID**
A UUIDv7 identifier assigned when a daemon or MCP server starts. Enables correlation of all logs from a single process invocation.

**Session ID**
A UUIDv7 identifier for a logical session (e.g., MCP client connection). Enables grouping related operations.

**slog**
Go's standard library structured logging package (log/slog) introduced in Go 1.21. Provides typed, structured logging with handler abstraction.

**UUIDv7**
A UUID variant that includes a timestamp component, making IDs naturally sortable by creation time. Used for process, session, and client identifiers.
