# MCP Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
   - [What is MCP?](#what-is-mcp)
   - [Purpose of the MCP Subsystem](#purpose-of-the-mcp-subsystem)
   - [Strategic Value](#strategic-value)
   - [Current Implementation Status](#current-implementation-status)
2. [Design Principles](#design-principles)
   - [JSON-RPC 2.0 Protocol Adherence](#json-rpc-20-protocol-adherence)
   - [Handler Registry Pattern](#handler-registry-pattern)
   - [Separation of Concerns](#separation-of-concerns)
   - [Error Handling Strategy](#error-handling-strategy)
3. [Key Components](#key-components)
   - [Protocol Layer](#protocol-layer)
   - [Transport Layer](#transport-layer)
   - [Server Orchestrator](#server-orchestrator)
   - [Search Integration](#search-integration)
   - [Tool Implementations](#tool-implementations)
   - [Prompt Implementations](#prompt-implementations)
4. [Configuration](#configuration)
5. [Integration Points](#integration-points)
   - [Index System Integration](#index-system-integration)
   - [Output Processor Integration](#output-processor-integration)
   - [Daemon Integration](#daemon-integration)
   - [External Client Integration](#external-client-integration)
6. [Glossary](#glossary)
7. [Debugging and Logging](#debugging-and-logging)
   - [MCP Server Logs](#mcp-server-logs)
8. [Additional Resources](#additional-resources)

---

## Overview

### What is MCP?

The Model Context Protocol (MCP) is an open standard for enabling AI tools to integrate with external context sources and capabilities. Introduced by Anthropic, MCP provides a standardized way for AI assistants to access data, invoke tools, and interact with external systems through a JSON-RPC 2.0-based protocol.

### Purpose of the MCP Subsystem

The MCP subsystem transforms agentic-memorizer from a Claude Code-specific integration into a universal memory provider for any MCP-enabled AI tool. It exposes the precomputed file index through a standardized server interface that supports:

- **Static Context Delivery**: Serving the complete index in multiple formats (XML, Markdown, JSON)
- **Dynamic Queries**: Enabling semantic search across indexed files during active sessions
- **Metadata Retrieval**: Providing detailed file metadata and semantic analysis on demand
- **Time-Based Filtering**: Surfacing recently modified files for context-aware workflows

### Strategic Value

The MCP subsystem addresses several limitations of the existing SessionStart hook integration:

- **Platform Independence**: Works with GitHub Copilot CLI, Claude Code, and any future MCP client
- **Bidirectional Communication**: Enables request/response patterns instead of one-way context injection
- **Runtime Queries**: Allows AI tools to search and filter the index during conversations
- **Protocol Standardization**: Aligns with industry-standard integration method for AI tool ecosystems

The subsystem maintains backward compatibility with existing Claude Code hooks while opening the door to a broader range of AI development tools.

### Current Implementation Status

The MCP subsystem has completed Phase 5 of the implementation plan:

- **Phase 1: Protocol Foundation** (Complete) - JSON-RPC 2.0 server, stdio transport, handshake protocol
- **Phase 2: Resource Implementation** (Complete) - Static index serving in three formats
- **Phase 3: Tool Implementation** (Complete) - Three tools for search, metadata, and recent files
- **Phase 4: Integration & Testing** (Complete) - SSE notifications, index subscriptions, daemon integration
- **Phase 5: MCP Prompts** (Complete) - Pre-configured prompt templates with index-aware message generation

---

## Design Principles

### JSON-RPC 2.0 Protocol Adherence

The subsystem strictly implements the JSON-RPC 2.0 specification to ensure compatibility with MCP clients:

- **Version Validation**: All messages must include `"jsonrpc": "2.0"` field
- **Request/Response Correlation**: Request IDs are preserved in responses for proper matching
- **Notification Handling**: Fire-and-forget messages (no ID) never receive responses
- **Protocol Version Negotiation**: Server supports multiple MCP protocol versions (2024-11-05, 2025-06-18) and echoes back the client's requested version during handshake for backward/forward compatibility
- **Standard Error Codes**: Uses JSON-RPC error codes (-32700 to -32603) plus MCP-specific codes
- **Error Structure**: Errors include code, message, and optional data fields

This strict adherence ensures the server works correctly with any standards-compliant MCP client.

### Handler Registry Pattern

The subsystem uses a registry pattern to decouple message routing from handler implementation:

- **Map-Based Dispatch**: Handler functions stored in maps keyed by tool/resource/prompt name
- **Registration at Initialization**: Handlers registered in `NewServer()` constructor
- **Type-Safe Signatures**: Handler functions enforce consistent context + params → result patterns
- **Runtime Extensibility**: New capabilities can be added without modifying the router
- **Future Plugin Support**: Enables dynamic handler registration for user-defined extensions

This pattern promotes clean separation between protocol handling and business logic.

### Separation of Concerns

The subsystem is organized into three distinct layers with clear responsibilities:

**Protocol Layer** (`internal/mcp/protocol/`)
- Pure data types representing MCP messages
- No I/O operations or business logic
- JSON marshaling/unmarshaling only
- Shared between transport and server layers

**Transport Layer** (`internal/mcp/transport/`)
- I/O abstraction with Read/Write/Close interface
- No knowledge of protocol semantics
- Currently implements stdio transport (line-delimited JSON)
- Future-proof for HTTP, WebSocket, or SSE transports

**Server Layer** (`internal/mcp/server.go`)
- Message routing and handler orchestration
- Protocol state management (initialization, capabilities)
- Delegates business logic to handlers
- Coordinates between transport and subsystems

This layering enables independent testing, transport swapping, and clear boundaries between concerns.

### Error Handling Strategy

The subsystem implements a two-tier error handling strategy:

**Protocol-Level Errors** (JSON-RPC errors)
- Malformed JSON or invalid JSON-RPC structure
- Unknown method names
- Server not initialized when capability invoked
- Returned as JSON-RPC error responses with standard codes

**Tool-Level Errors** (Tool response with isError flag)
- Invalid tool arguments
- Missing files or empty index
- Search failures or timeouts
- Returned as successful JSON-RPC responses with `isError: true` in tool content

This separation allows tools to report failures without breaking the protocol flow, enabling clients to distinguish between communication errors and execution errors.

---

## Key Components

### Protocol Layer

The protocol layer defines the message types that flow between MCP clients and the server. It implements the MCP specification through five key modules:

**JSON-RPC 2.0 Primitives** (`protocol/messages.go`)
- `JSONRPCRequest`: Method invocations requiring responses
- `JSONRPCResponse`: Success or error results
- `JSONRPCNotification`: Fire-and-forget messages
- `JSONRPCError`: Standard error structure with codes and messages

**Handshake Protocol** (`protocol/initialize.go`)
- `InitializeRequest`: Client declares protocol version (supports 2024-11-05, 2025-06-18) and identity
- `InitializeResponse`: Server declares capabilities and echoes client's protocol version
- `ServerCapabilities`: Flags indicating supported features (resources, tools, prompts)
- **Notification Compatibility**: Accepts both `initialized` and `notifications/initialized` notification methods for client compatibility
- Enables capability negotiation, version compatibility, and backward/forward compatibility

**Resource Types** (`protocol/resources.go`)
- `Resource`: Static context endpoints with URI, name, description, MIME type
- `ResourcesListResponse`: Array of available resources
- `ResourcesReadRequest`: URI-based content retrieval
- `ResourcesReadResponse`: Resource content with metadata

**Tool Types** (`protocol/tools.go`)
- `Tool`: Executable function with JSON Schema parameter definition
- `InputSchema`: JSON Schema Draft 7 subset for parameter validation
- `ToolsCallRequest`: Tool invocation with name and arguments
- `ToolsCallResponse`: Result content array with optional error flag

**Prompt Types** (Future - Phase 5)
- Templated workflows for common operations
- Prompt discovery and parameter collection
- Type definitions planned for Phase 5 (handler stubs exist returning "not yet implemented" errors)

### Transport Layer

The transport layer abstracts communication mechanisms through a simple interface:

**Transport Interface** (`transport/transport.go`)
- `Read() ([]byte, error)`: Blocking read of single message
- `Write(data []byte) error`: Thread-safe message transmission
- `Close() error`: Graceful connection termination

**Stdio Implementation** (`transport/stdio.go`)
- Reads from `os.Stdin` via buffered reader
- Writes to `os.Stdout` with mutex for thread safety
- Line-delimited JSON parsing (one message per line)
- **Whitespace normalization**: Trims whitespace from messages and skips empty lines for protocol robustness
- Logs to `os.Stderr` to keep stdout protocol-only (server logs to file when configured)
- Returns EOF on client disconnect for graceful shutdown

The interface design enables future transport implementations (HTTP long-polling, Server-Sent Events, WebSockets) without changes to server logic.

### Server Orchestrator

The server orchestrator coordinates all subsystem components and manages protocol state:

**Core Responsibilities**
- Loading precomputed index at startup
- Initializing transport and entering message loop
- Routing incoming requests to appropriate handlers
- Managing initialization state and capability enforcement
- Sending responses and errors via transport
- Graceful shutdown on disconnect or signals

**Handler Registries**
- `resourceHandlers`: Declared for future extensibility but currently unused (resources use direct switch-case routing in `handleResourcesRead()` for simplicity)
- `toolHandlers`: Maps tool names to execution functions
- `promptHandlers`: Reserved for Phase 5 prompt implementations

**Message Processing Flow**
1. Read JSON-RPC message from transport
2. Validate JSON-RPC 2.0 version field
3. Parse method name and route to handler
4. Check initialization state for capability methods
5. Execute handler with context and parameters
6. Format result as JSON-RPC response
7. Write response to transport
8. Loop until EOF or shutdown signal

**Lifecycle Management**
- Signal handling for SIGINT and SIGTERM
- Context cancellation propagation to handlers
- Clean resource release on shutdown
- Error recovery without server crash

### Search Integration

The search component provides token-based semantic search capabilities over the precomputed index:

**Searcher** (`internal/search/semantic.go`)
- Wraps the index for query operations
- Stateless design (no internal caching)
- Read-only access to index entries

**Search Query Model**
- `Query`: Search terms processed through tokenization and stop word filtering
- `Categories`: Optional filter by file category (documents, code, images, presentations, etc.)
- `MaxResults`: Limit on returned results (default 10, maximum 100)

**Token-Based Matching Algorithm**

Queries are tokenized through a multi-step process:
1. Convert to lowercase
2. Split on whitespace into individual words
3. Filter common stop words (`a`, `an`, `and`, `the`, `is`, `for`, `with`, etc.)
4. Remove punctuation from word boundaries (`.`, `,`, `!`, `?`, `;`, `:`, `"`, `'`, `()`, `[]`, `{}`)
5. Discard tokens shorter than 2 characters

Each file is scored by counting how many query tokens substring-match in each searchable field, normalized by the total number of query tokens. Token matching uses case-insensitive substring containment (not fuzzy algorithms like Levenshtein distance), so the query token "form" would match the word "terraform". The minimum relevance threshold is hardcoded to 0.1; entries scoring below this value are filtered out.

**Weighted Scoring System**

Results are ranked using cumulative weighted scoring with partial token matching:
- **Filename match**: 3.0 × (matched_tokens / total_tokens) - highest priority
- **Summary match**: 2.0 × (matched_tokens / total_tokens)
- **Tag match**: 1.5 × (matched_tokens / total_tokens) - aggregated across all tags
- **Category match**: 1.0 × (matched_tokens / total_tokens) - file category field
- **Topic match**: 1.0 × (matched_tokens / total_tokens) - aggregated across all topics
- **File type match**: 0.5 × (matched_tokens / total_tokens) - file extension and type field
- **Document type match**: 0.5 × (matched_tokens / total_tokens) - AI-classified document type

Entries must score above 0.1 (minimum relevance threshold, hardcoded in implementation at `semantic.go:94`) to be included in results. Results are sorted by cumulative score descending and limited to the requested maximum. The match type indicating the first matching field in weighted order (not necessarily the highest-scoring field) is returned for display purposes.

**Example Scoring**:

Query: `"NIH terraform workshop"` → Tokens: `["nih", "terraform", "workshop"]` (3 tokens)

File: `nih-cit-technical-workshop.pptx`
- Filename contains "nih" and "workshop" → 2/3 tokens × 3.0 = 2.0 points
- Summary contains all 3 tokens → 3/3 tokens × 2.0 = 2.0 points
- Category "presentations" contains 0 tokens → 0 points
- Total score: 4.0 (high relevance, ranked first)

**Result Model**
- Complete `IndexEntry` with metadata and semantic analysis
- Relevance score (float) for ranking
- Match type indicating the highest-weighted field that matched (filename, summary, tag, category, topic, file_type, document_type)

### Tool Implementations

The subsystem implements three tools for interacting with the file index:

**search_files**
- Performs token-based semantic search with stop word filtering across all indexed files
- Searches: filename, category, file type, summary, tags, topics, document type
- Uses weighted scoring with partial token matching (see Search Integration section for algorithm details)
- Parameters: query (required), categories (optional filter), max_results (default 10, max 100)
- Returns: Query echo, result count, array of matches with path/score/match_type/summary/tags
- Use case: "Find all documents about Terraform" or "Search for PowerPoint presentations about workshops"

**get_file_metadata**
- Retrieves detailed metadata for a specific file
- Parameters: path (required, supports partial matching)
- Returns: Complete index entry with all metadata and semantic analysis
- Use case: "Show me the metadata for the deployment guide" or "What tags are on config.yaml?"

**list_recent_files**
- Lists files modified within a specified time window
- Parameters: days (default 7), limit (default 20)
- Returns: Files sorted by modification time descending with full metadata
- Use case: "What files changed this week?" or "Show recent documentation updates"

All tools validate inputs, handle missing data gracefully, and return structured JSON results that clients can parse and present to users.

### Prompt Implementations

The subsystem provides three pre-configured prompt templates that generate context-aware messages using index data:

**analyze-file**
- Deep analysis of a specific file with semantic metadata
- Required arguments: `file_path` (string)
- Optional arguments: None
- Implementation: `prompts.go:118-180`
  - Validates file exists in index
  - Validates file has semantic analysis
  - Generates formatted prompt including metadata (path, type, category, size), semantic summary, tags, and key topics
  - Returns protocol.PromptMessage array with role="user" and text content
- Use case: "I want to analyze the API documentation" or "Help me understand this configuration file"
- Error handling: Returns error if file not found or lacks semantic analysis

**search-context**
- Builds effective semantic search queries with guidance
- Required arguments: `topic` (string)
- Optional arguments: `category` (string, e.g., "documents", "code", "images")
- Implementation: `prompts.go:182-216`
  - Generates search guidance prompt with topic and optional category filter
  - Provides suggestions for constructing effective searches
  - Does not require index data (generates guidance, not results)
- Use case: "How do I search for authentication-related files?" or "Find deployment documentation"
- Error handling: Returns error if topic is missing

**explain-summary**
- Detailed explanation of a file's AI-generated semantic analysis
- Required arguments: `file_path` (string)
- Optional arguments: None
- Implementation: `prompts.go:218-276`
  - Validates file exists in index with semantic analysis
  - Generates explanation prompt including summary, tags, topics, document type, and confidence
  - Returns detailed context about how the AI analyzed the file
- Use case: "Explain the semantic analysis for this file" or "What do these tags mean?"
- Error handling: Returns error if file not found or lacks semantic analysis

**Prompt Registry** (`prompts.go:10-87`)
- Centralized registry managing all available prompts
- NewPromptRegistry() initializes 3 default prompts with metadata
- ListPrompts() returns all registered prompts for client discovery
- GetPrompt(name) retrieves prompt definition by name
- GeneratePromptMessages(name, args, server) creates context-aware messages
- Validates required arguments before message generation
- Routes to specific generator functions based on prompt name

**Message Format**
All prompt generators return `protocol.PromptMessage` array with:
- Role: "user" (all current prompts generate user messages)
- Content: protocol.PromptContent with Type="text" and formatted Text field
- Text includes structured markdown with file metadata, semantic data, and user guidance

**Client Integration**
MCP clients discover prompts via `prompts/list` request and invoke them using `prompts/get` with arguments. The server capabilities declare `prompts` support, making these templates available in client UI (e.g., Claude Code's prompt selector).

### Configuration

The MCP subsystem uses dedicated configuration settings separate from the daemon:

**MCP Configuration** (`internal/config/types.go`)
- `log_file`: Path to MCP server log file (default: `~/.agentic-memorizer/mcp.log`)
- `log_level`: Logging verbosity - debug, info, warn, error (default: `info`)
- Log rotation: Automatic rotation at 10MB with 3 backups, 28-day retention, compression enabled

**Config File Example** (`.agentic-memorizer/config.yaml`):
```yaml
mcp:
  log_file: ~/.agentic-memorizer/mcp.log
  log_level: info
```

**Command-Line Override**:
```bash
# Start with debug logging (overrides config file)
agentic-memorizer mcp start --log-level debug

# View logs in real-time
tail -f ~/.agentic-memorizer/mcp.log
```

**Dual Output**:
- **File**: Text-format logs written to `log_file` with rotation for persistent debugging
- **Stderr**: Text-format real-time logs visible to MCP client for immediate feedback

**Note**: Both outputs use `slog.NewTextHandler` (text format), not JSON format, despite the comment at line 164 of `cmd/mcp/subcommands/start.go` claiming JSON for file output. The actual implementation uses text format for both channels.

**Environment Variables**:
Configuration values can also be set via environment variables:
```bash
export MEMORIZER_MCP_LOG_FILE=~/.agentic-memorizer/mcp.log
export MEMORIZER_MCP_LOG_LEVEL=debug
```

---

## Integration Points

### Index System Integration

The MCP server integrates with the index subsystem through read-only access patterns:

**Index Loading**
- Server loads precomputed index at startup via `index.Manager.LoadComputed()`
- Uses standard index path from configuration
- Fails gracefully if index missing with actionable error message
- No write operations or locking required

**Index Schema Compatibility**
- Uses existing `types.Index` and `types.IndexEntry` structures
- No MCP-specific modifications to index format
- Compatible with all existing index generation logic
- Shares schema versioning with rest of system

**Error Handling**
- Missing index returns error directing user to start daemon
- Malformed index fails startup with validation error
- Empty index works correctly (returns zero results)

The integration maintains complete separation between index generation (daemon) and index consumption (MCP server).

### Output Processor Integration

The MCP server reuses existing output processors for resource formatting:

**Format Delegation**
- XML format → `output.NewXMLProcessor().Format(index)`
- Markdown format → `output.NewMarkdownProcessor().Format(index)`
- JSON format → `output.NewJSONProcessor().Format(index)`

**Benefits**
- Zero code duplication between hooks and MCP
- Consistent formatting across all integration methods
- Automatic inheritance of format improvements
- Schema versioning handled by processors

**Resource URIs**
- `memorizer://index` → XML (Claude Code format)
- `memorizer://index/markdown` → Markdown (human-readable)
- `memorizer://index/json` → JSON (structured data)

Clients can request their preferred format through URI routing, with MIME types provided for content negotiation.

### Daemon Integration

The MCP server and daemon maintain independent operation:

**Separation of Concerns**
- Daemon: Watches memory directory, generates/updates index
- MCP Server: Reads precomputed index, serves to clients
- No direct communication between processes

**Shared State**
- Both read index file from same path (`config.GetIndexPath()`)
- Daemon writes atomically (temp file + rename)
- MCP server reads at startup only (no live reload)

**User Workflow**
1. Start daemon: `agentic-memorizer daemon start`
2. Daemon builds initial index
3. Start MCP server: `agentic-memorizer mcp start` (invoked by AI tool)
4. Server loads current index snapshot
5. Daemon continues updating index in background
6. Server restart required to pick up index changes

This decoupling ensures the MCP server remains simple and reliable while the daemon handles complex file watching and semantic analysis.

### External Client Integration

The MCP server exposes a standard interface for AI tool integration:

**GitHub Copilot CLI**
- Configuration: `~/.github-copilot/config.json` MCP servers section
- Invocation: Copilot spawns `agentic-memorizer mcp start` as subprocess
- Transport: Stdio (line-delimited JSON over stdin/stdout)
- Capabilities: Can list resources, read index, call search tools
- Use case: "@memorizer find all Terraform modules" during CLI chat

**Claude Code**
- Configuration: Dual mode - SessionStart hooks (existing) OR MCP integration (future)
- MCP mode provides richer interaction than static hook injection
- Same stdio transport as Copilot
- Enables dynamic search during coding sessions

**Other MCP Clients**
- Cline, Continue, Cursor, Aider (roadmap support)
- Any tool implementing MCP client protocol
- Standard subprocess invocation model
- Configuration varies by tool but protocol remains consistent

**Client Responsibilities**
- Spawn server process with correct environment
- Handle stdio transport (send/receive JSON-RPC messages)
- Perform initialize handshake
- Discover capabilities via list endpoints
- Invoke resources/tools as needed
- Clean up subprocess on exit

The server remains agnostic to client implementation details, ensuring broad compatibility across the AI tool ecosystem.

---

## Debugging and Logging

### MCP Server Logs

The MCP server maintains detailed logs for debugging protocol issues and client interactions:

**Log Configuration** (`.agentic-memorizer/config.yaml`):
```yaml
mcp:
  log_file: ~/.agentic-memorizer/mcp.log  # Log file path
  log_level: info                          # debug, info, warn, error
```

**Log Output**:
- **File**: `~/.agentic-memorizer/mcp.log` (structured logs for debugging and analysis)
- **Stderr**: Real-time logs visible to MCP client (text format, live feedback)
- **Rotation**: Automatic at 10MB file size, keeps 3 backup files, 28-day retention, compression enabled

**Log Levels**:
- `debug`: All messages including raw JSON-RPC traffic and detailed protocol flow
- `info`: Server lifecycle events, client connections, tool invocations, resource requests
- `warn`: Protocol violations, unrecognized methods, invalid protocol versions
- `error`: Parsing failures, handler errors, transport issues

**Viewing Logs**:
```bash
# Tail live logs
tail -f ~/.agentic-memorizer/mcp.log

# View with filtering
grep -i error ~/.agentic-memorizer/mcp.log

# Search for specific client session
grep "client=claude-code" ~/.agentic-memorizer/mcp.log

# Start server with debug logging
agentic-memorizer mcp start --log-level debug
```

**Common Debugging Scenarios**:

*Protocol handshake failures*:
```bash
# Check for initialize request and protocol version compatibility
grep "Received initialize request" ~/.agentic-memorizer/mcp.log
```
Look for protocol version mismatches or missing client info.

*Tool call errors*:
```bash
# Find tool invocations and their results
grep "Calling tool" ~/.agentic-memorizer/mcp.log
```
Check for invalid arguments, missing index data, or search failures.

*Client disconnects*:
```bash
# Search for disconnect events
grep -E "(Client disconnected|EOF)" ~/.agentic-memorizer/mcp.log
```
Unexpected disconnects may indicate protocol errors or client-side issues.

*Performance issues*:
```bash
# Enable debug logging to see message timing
agentic-memorizer mcp start --log-level debug 2>&1 | grep -E "(Received|Sending)"
```
Watch for slow handlers or large response payloads.

**Debug Logging Examples**:

With `log_level: debug`, the server logs every JSON-RPC message:
```
time=2025-11-06T18:20:36.717 level=DEBUG msg="Received message" raw_data="{\"jsonrpc\":\"2.0\",\"id\":0,\"method\":\"initialize\",...}"
time=2025-11-06T18:20:36.717 level=DEBUG msg="Sending response" raw_data="{\"jsonrpc\":\"2.0\",\"id\":0,\"result\":{...}}"
```

This enables complete protocol-level debugging including message contents, timing, and sequencing.

---

## Glossary

**Capability Negotiation**
The process during MCP handshake where client and server exchange information about supported features. Clients learn which resources, tools, and prompts are available; servers learn client identity and protocol version. Enables graceful degradation when features are unsupported.

**Handler Registry**
A design pattern using maps to associate capability names (tool names, resource URIs, prompt names) with their implementation functions. Enables dynamic dispatch without hardcoded routing logic and supports runtime registration of new capabilities.

**JSON-RPC 2.0**
A stateless, lightweight remote procedure call protocol encoded in JSON. Defines standard message structures for requests (method calls), responses (results or errors), and notifications (fire-and-forget). Forms the transport layer for MCP.

**MCP (Model Context Protocol)**
An open standard protocol introduced by Anthropic for integrating AI tools with external context sources and capabilities. Enables standardized access to data (resources), executable functions (tools), and templated workflows (prompts) across different AI assistant platforms.

**Prompts**
Templated workflows in MCP that guide users through multi-step operations. Prompts can collect parameters, provide context, and return structured guidance for AI tools to follow. Planned for Phase 5 implementation in agentic-memorizer.

**Resources**
Static context endpoints in MCP that provide read-only access to data sources. In agentic-memorizer, resources expose the precomputed file index in three formats (XML, Markdown, JSON) via URIs like `memorizer://index`.

**Semantic Search**
A search technique that matches queries against multiple semantic fields (summaries, tags, topics) beyond simple filename matching. Uses weighted scoring to rank results by relevance, with higher weights for more specific matches like filenames.

**Stdio Transport**
A communication mechanism using standard input/output streams for bidirectional message passing. MCP clients spawn server processes and exchange JSON-RPC messages over stdin/stdout, with each message terminated by a newline. Logging goes to stderr to keep protocol channels clean.

**Tools**
Executable functions in MCP that AI assistants can invoke with parameters. Tools define their inputs using JSON Schema for validation and return structured results. In agentic-memorizer, tools enable search, metadata retrieval, and time-based file filtering during AI sessions.

**Weighted Scoring**
An algorithm that assigns different point values to different types of matches when ranking search results. In agentic-memorizer's semantic search, filename matches receive the highest weight (3.0), followed by summary matches (2.0), tag matches (1.5), topic matches (1.0), and document type matches (0.5). Results are sorted by cumulative score.

---

## Additional Resources

For detailed implementation information, see:
- Protocol types: `internal/mcp/protocol/`
- Server implementation: `internal/mcp/server.go`
- Search implementation: `internal/search/semantic.go`
- CLI commands: `cmd/mcp/`

For MCP specification details, visit: https://modelcontextprotocol.io
