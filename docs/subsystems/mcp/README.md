# MCP Server

Model Context Protocol implementation with JSON-RPC 2.0 messaging, stdio transport, and graph-powered tools for AI assistant integration.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The MCP subsystem provides a standardized Model Context Protocol server that enables AI assistants to access Memorizer's file index and knowledge graph through on-demand tool invocations. The server implements JSON-RPC 2.0 over stdio transport, supporting protocol versions 2024-11-05, 2025-06-18, and 2025-11-25. It exposes five graph-powered tools for semantic search, file metadata retrieval, recent files listing, related file discovery, and entity search.

The subsystem implements a three-layer architecture: the protocol layer defines JSON-RPC 2.0 message types and MCP capabilities, the transport layer handles stdio communication with line-delimited JSON, and the handlers layer implements tool business logic with daemon API integration. Real-time index updates flow through Server-Sent Events from the daemon, enabling the MCP server to serve current data without polling.

Key capabilities include:

- **JSON-RPC 2.0 protocol** - Standard request/response/notification messaging with error codes
- **Stdio transport** - Line-delimited JSON over stdin/stdout for subprocess communication
- **Five graph tools** - search_files, get_file_metadata, list_recent_files, get_related_files, search_entities
- **Three resources** - File index in XML, JSON, and Markdown formats with subscription support
- **Three prompts** - Built-in prompts for file analysis, search context, and summary explanation
- **Partial fallback** - Two tools (get_file_metadata, list_recent_files) degrade to in-memory index when daemon unavailable
- **Real-time updates** - SSE client receives index changes and notifies subscribed clients

## Design Principles

### Separation of Protocol, Transport, and Handlers

The subsystem cleanly separates concerns across three layers. The protocol layer (`protocol/`) defines JSON-RPC 2.0 message structures and MCP-specific types without knowledge of transport mechanics. The transport layer (`transport/`) handles stdio I/O with buffered reading and synchronized writing. The handlers layer (`handlers/`) implements tool logic using dependency injection for daemon communication. This separation enables testing each layer independently and potential transport substitution.

### Handler Registry with Dependency Injection

Tool handlers implement a common Handler interface with Name, Execute, and ToolDefinition methods. The server maintains a handler map keyed by tool name. On tool calls, the server looks up the handler and invokes Execute with a Dependencies struct containing DaemonURL, HTTPClient, IndexProvider, and Logger. This pattern enables consistent tool behavior, shared HTTP client configuration, and testable handler implementations.

### Dual-Source Fallback Strategy

Two of the five tools (get_file_metadata, list_recent_files) implement dual-source logic: they first attempt the daemon HTTP API for current graph data, then fall back to the in-memory index if the daemon is unavailable. This enables degraded operation without complete failure. The remaining three tools (search_files, get_related_files, search_entities) require the daemon's graph database for their full functionality and return clear error messages when the daemon is unavailable.

### Thread-Safe Index Updates

The server protects its file index with an RWMutex. The GetIndex method acquires a read lock for concurrent tool access. The ReloadIndex method acquires a write lock for atomic replacement when SSE events arrive. This avoids index corruption during concurrent operations and eliminates the need for server restarts on index changes.

### Subscription-Based Notifications

The subscription manager tracks which resource URIs clients have subscribed to via resources/subscribe. When the SSE client receives index update events, it queries active subscriptions and sends notifications/resources/updated only to subscribed URIs. This prevents unnecessary notification traffic for unsubscribed resources.

### Graceful Lifecycle Management

The server implements a controlled lifecycle: initialize validates the client and exchanges capabilities, the message loop processes requests until EOF or context cancellation, and shutdown closes the transport cleanly. Signal handlers catch SIGTERM/SIGINT for graceful termination. Malformed messages are logged but don't crash the server, ensuring robustness against protocol violations.

## Key Components

### Server (`server.go`)

The Server struct orchestrates MCP operations. It holds configuration (daemon URL, process ID), state (transport, logger, initialized flag, subscriptions, prompts), and the protected file index. The Run method implements the message loop: read from transport, parse JSON-RPC, route to handler, send response. The handleMessage function dispatches on method name to specific handlers (initialize, tools/list, tools/call, resources/list, resources/read, etc.). Thread safety is achieved through RWMutex for index access and subscription manager for URI tracking.

### Transport Interface and Stdio (`transport/`)

The Transport interface defines Read, Write, and Close methods for message I/O. The StdioTransport implementation provides line-delimited JSON communication: Read uses a buffered reader to read until newline and returns raw JSON bytes; Write uses a mutex-protected stdout write with automatic newline termination. Empty lines are skipped during read. This enables reliable subprocess communication without complex framing.

### Protocol Types (`protocol/`)

The protocol package defines all JSON-RPC 2.0 and MCP message types. Core types include JSONRPCRequest, JSONRPCResponse, JSONRPCNotification, and JSONRPCError with standard error codes (-32700 through -32603) plus MCP-specific codes (-1 through -3). Specialized types include InitializeRequest/Response for capability negotiation, ToolsListResponse/ToolsCallRequest/ToolsCallResponse for tool operations, and ResourcesListResponse/ResourcesReadRequest/ResourcesReadResponse for resource operations. Prompt types define the prompts/list and prompts/get interfaces.

### Handlers (`handlers/`)

Five tool handlers implement the Handler interface:

**search_files** - Semantic search with query, optional categories filter, and max_results limit. Requires daemon for graph-powered search across filenames, tags, topics, and entities.

**get_file_metadata** - Complete metadata for a file by path. Tries daemon API first, falls back to case-insensitive index lookup with substring matching.

**list_recent_files** - Files modified within a time window (days parameter) with result limit. Tries daemon API first, falls back to index filtering and sorting.

**get_related_files** - Related files through graph relationships (shared tags, topics, entities). Daemon-only with no fallback.

**search_entities** - Files mentioning a specific entity with optional type filter. Daemon-only with no fallback.

Each handler receives Dependencies for daemon communication and index access. The BaseHandler provides common functionality including CallDaemonAPI for HTTP requests.

### SSE Client (`sse_client.go`)

The SSEClient maintains a persistent connection to the daemon's /sse endpoint for real-time notifications. It sends correlation headers (X-Client-ID, X-Client-Type, X-Client-Version) for distributed logging. On receiving events (index_snapshot, index_updated), it calls server.ReloadIndex to update the index atomically, then sends notifications/resources/updated for each subscribed resource URI. Automatic reconnection with 5-second backoff handles transient failures. Buffer sizing starts at 64KB and expands to 10MB for large index payloads.

### Subscription Manager (`subscriptions.go`)

The subscription manager provides thread-safe tracking of resource subscriptions. Subscribe and Unsubscribe modify a map of URI to bool. GetSubscriptions returns all subscribed URIs for notification delivery. IsSubscribed checks individual URIs. The manager enables efficient notification filtering without iterating all possible resources.

### Prompt Registry (`prompts.go`)

The prompt registry defines three built-in prompts for file analysis workflows:

**analyze-file** - Takes file_path argument, generates detailed analysis prompt covering purpose, concepts, relationships, and patterns.

**search-context** - Takes topic (required) and category (optional) arguments, helps construct effective search queries with key terms and file types.

**explain-summary** - Takes file_path argument, explains semantic analysis results including tags, topics, and document type.

GeneratePromptMessages looks up files in the index and formats contextual prompts with semantic metadata.

### Resources

The server exposes three resources representing the file index in different formats:

**memorizer://index** - XML format with schema metadata, suitable for Claude Code integration.

**memorizer://index/markdown** - Human-readable Markdown format for display.

**memorizer://index/json** - Structured JSON format for programmatic access.

Resources are read via the resources/read method, which formats the current index using the internal/format package. Subscriptions track which resources clients want change notifications for.

## Integration Points

### Daemon HTTP API

The MCP server acts as an HTTP client to the daemon API. Tool handlers call daemon endpoints for current data: POST /api/v1/search for search_files, GET /api/v1/files/{path} for get_file_metadata, GET /api/v1/files/recent for list_recent_files, GET /api/v1/files/related for get_related_files, and GET /api/v1/entities/search for search_entities. The 30-second timeout handles slow graph queries gracefully.

### Daemon SSE Endpoint

The SSE client connects to the daemon's /sse endpoint for real-time index updates. Events include index_snapshot (full replacement) and index_updated (incremental). This enables the MCP server to serve current data without polling or restart.

### Integration Adapters

Integration adapters (Claude Code, Gemini CLI, Codex CLI) configure their respective tools to spawn the MCP server as a subprocess. The server communicates via stdio, receiving JSON-RPC requests and returning responses. Configuration includes the binary path and "mcp start" arguments.

### Configuration System

MCP configuration is read from the config subsystem: mcp.daemon_host and mcp.daemon_port construct the daemon URL, mcp.log_file sets the log output path, and mcp.log_level controls verbosity. The --log-level flag overrides the config setting for debugging.

### Format Subsystem

Resource reading uses the format subsystem for output generation. FilesContent wraps the file index, and formatters (XML, JSON, Markdown) render it to the requested format. This ensures consistent output styling across resources and CLI commands.

### CLI Commands

The cmd/mcp/subcommands/start.go command initializes the MCP server. It loads configuration, constructs the daemon URL, attempts initial index fetch, creates the server with handlers and transport, and enters the message loop. Signal handling enables graceful shutdown on SIGTERM/SIGINT.

## Glossary

**Capability**
A feature supported by the MCP server or client, exchanged during initialization. Server capabilities include resources (with subscribe and listChanged), tools, and prompts.

**Fallback**
The strategy of attempting the daemon API first, then using the in-memory index if unavailable. Two tools (get_file_metadata, list_recent_files) support fallback for degraded operation.

**Handler**
An implementation of tool logic that receives dependencies and returns results. Handlers are registered by name and invoked on tools/call requests.

**Initialize**
The first message exchange establishing protocol version and capabilities. The server waits for initialize before processing other requests.

**JSON-RPC 2.0**
A stateless remote procedure call protocol using JSON encoding. MCP uses JSON-RPC for all request/response/notification messaging.

**MCP (Model Context Protocol)**
A standardized protocol for AI assistants to access external tools and resources. Memorizer's MCP server provides file discovery and search capabilities.

**Notification**
A JSON-RPC message that expects no response (no id field). Used for initialized acknowledgment and resources/updated events.

**Prompt**
A template for generating contextual messages that assistants can use. Prompts include arguments for customization and return formatted messages.

**Resource**
A named data source accessible via URI. Memorizer exposes the file index as three resources in different formats.

**SSE (Server-Sent Events)**
A protocol for server-to-client streaming over HTTP. The daemon sends index updates via SSE, which the MCP server receives for real-time synchronization.

**Stdio Transport**
Communication over standard input/output streams. The MCP server reads JSON-RPC from stdin and writes responses to stdout, enabling subprocess integration.

**Subscription**
A client's request to receive notifications when a resource changes. The server tracks subscriptions and sends updates only for subscribed URIs.

**Tool**
A callable function exposed via the MCP protocol. Memorizer provides five tools for file search, metadata, recents, related files, and entity search.
