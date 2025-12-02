# FalkorDB Knowledge Graph Subsystem

This document provides comprehensive technical documentation for the FalkorDB Knowledge Graph subsystem of Agentic Memorizer.

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
  - [Graph Manager](#graph-manager)
  - [Queries](#queries)
  - [Schema](#schema)
  - [Exporter](#exporter)
  - [HTTP API](#http-api)
  - [CLI Commands](#cli-commands)
- [Graph Schema](#graph-schema)
  - [Node Types](#node-types)
  - [Relationship Types](#relationship-types)
  - [Constraints and Indexes](#constraints-and-indexes)
- [Integration Points](#integration-points)
- [Configuration](#configuration)
- [Operational Guide](#operational-guide)
  - [Starting FalkorDB](#starting-falkordb)
  - [Health Monitoring](#health-monitoring)
  - [Troubleshooting](#troubleshooting)
- [API Reference](#api-reference)
- [Glossary](#glossary)

## Overview

The FalkorDB Knowledge Graph subsystem provides persistent graph-based storage for indexed files and their semantic relationships. It enables relationship-aware queries that go beyond simple text matching, allowing discovery of related files through shared tags, topics, and entities.

**Purpose:**
- Store file metadata and semantic analysis in a graph structure
- Enable relationship traversal for "related files" queries
- Support graph-powered semantic search via Cypher queries
- Provide persistent storage that survives daemon restarts
- Enable rich queries across file relationships

**Why a Knowledge Graph:**
1. **Relationship Discovery** - Find files sharing tags, topics, or entities
2. **Semantic Queries** - Cypher enables complex multi-hop traversals
3. **Persistence** - Graph data survives daemon restarts
4. **Scalability** - FalkorDB handles large file collections efficiently
5. **Future Extensibility** - Graph model supports adding new node/edge types

## Design Principles

### Graceful Degradation

The graph subsystem implements graceful degradation when FalkorDB is unavailable:

```go
// Connection attempt with fallback
func (m *Manager) IsConnected() bool {
    if m.client == nil {
        return false
    }
    // Check connection health
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    return m.client.Ping(ctx).Err() == nil
}
```

When FalkorDB is unavailable:
- Daemon continues operating normally
- Graph queries return empty results (not errors)
- File processing continues without graph storage
- Warnings logged but not escalated to errors

### Separation of Concerns

The graph subsystem separates responsibilities clearly:

| Component | Responsibility |
|-----------|----------------|
| Manager | Connection lifecycle, health checks |
| Queries | Cypher query execution, CRUD operations |
| Schema | Node/edge definitions, constraints |
| Exporter | Convert graph data to Index format |
| HTTP API | REST endpoints for external access |
| CLI | Container management commands |

### Idempotent Operations

All graph operations are idempotent:
- `MERGE` used instead of `CREATE` for nodes
- Relationships created with existence checks
- Rebuild operations clear and repopulate atomically

## Key Components

### Graph Manager

**Location:** `internal/graph/manager.go`

The Manager handles FalkorDB connection lifecycle and health monitoring.

```go
type Manager struct {
    client   *redis.Client
    graph    *falkordb.Graph
    queries  *Queries
    logger   *slog.Logger
    mu       sync.RWMutex
}

// Key methods
func NewManager(logger *slog.Logger) *Manager
func (m *Manager) Connect(ctx context.Context, addr string) error
func (m *Manager) Close() error
func (m *Manager) IsConnected() bool
func (m *Manager) Queries() *Queries
```

**Connection Flow:**
1. Parse Redis address (host:port)
2. Create Redis client with connection pool
3. Create FalkorDB graph handle
4. Initialize Queries helper
5. Ensure schema constraints exist

### Queries

**Location:** `internal/graph/queries.go`

Queries encapsulates all Cypher query operations for the graph.

**Core Operations:**

| Method | Purpose |
|--------|---------|
| `UpsertFile` | Create/update File node with metadata |
| `DeleteFile` | Remove File node and relationships |
| `GetFile` | Retrieve File by path |
| `GetFileConnections` | Get tags, topics, entities for a file |
| `Search` | Multi-signal semantic search |
| `GetRecentFiles` | Files modified within time window |
| `GetRelatedFiles` | Files sharing tags/topics |
| `SearchByEntity` | Files mentioning entity |
| `ClearGraph` | Remove all nodes and relationships |
| `GetStats` | Node/relationship counts |

**Search Query Structure:**
```cypher
MATCH (f:File)
WHERE f.name CONTAINS $query
   OR f.summary CONTAINS $query
   OR EXISTS {
       MATCH (f)-[:HAS_TAG]->(t:Tag)
       WHERE t.name CONTAINS $query
   }
   OR EXISTS {
       MATCH (f)-[:COVERS_TOPIC]->(topic:Topic)
       WHERE topic.name CONTAINS $query
   }
RETURN f
ORDER BY f.modified_time DESC
LIMIT $limit
```

### Schema

**Location:** `internal/graph/schema.go`

Schema defines node types, relationship types, and constraints.

```go
// Node type constants
const (
    NodeFile     = "File"
    NodeTag      = "Tag"
    NodeTopic    = "Topic"
    NodeEntity   = "Entity"
    NodeCategory = "Category"
)

// Relationship type constants
const (
    RelHasTag      = "HAS_TAG"
    RelCoversTopic = "COVERS_TOPIC"
    RelMentions    = "MENTIONS"
    RelInCategory  = "IN_CATEGORY"
)
```

**Constraint Initialization:**
```go
func (m *Manager) EnsureSchema(ctx context.Context) error {
    // Create unique constraints
    constraints := []string{
        "CREATE CONSTRAINT IF NOT EXISTS ON (f:File) ASSERT f.path IS UNIQUE",
        "CREATE CONSTRAINT IF NOT EXISTS ON (t:Tag) ASSERT t.name IS UNIQUE",
        "CREATE CONSTRAINT IF NOT EXISTS ON (topic:Topic) ASSERT topic.name IS UNIQUE",
        "CREATE CONSTRAINT IF NOT EXISTS ON (e:Entity) ASSERT e.name IS UNIQUE",
        "CREATE CONSTRAINT IF NOT EXISTS ON (c:Category) ASSERT c.name IS UNIQUE",
    }
    // Execute each constraint
}
```

### Exporter

**Location:** `internal/graph/exporter.go`

Exporter converts graph data back to the Index format for compatibility.

```go
type Exporter struct {
    manager *Manager
    logger  *slog.Logger
}

func (e *Exporter) ToGraphIndex(ctx context.Context, memoryRoot string) (*types.GraphIndex, error)
```

**Export Process:**
1. Query all File nodes from graph
2. For each file, retrieve connections (tags, topics, entities)
3. Build FileEntry from file properties and connections
4. Assemble GraphIndex with metadata

### HTTP API

**Location:** `internal/daemon/api/`

The HTTP API exposes graph queries over HTTP for MCP and other clients.

**Files:**
- `server.go` - HTTP server and routing
- `types.go` - Request/response types
- `sse.go` - Server-Sent Events hub

**Endpoints:**

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Health status with graph metrics |
| `/api/v1/index` | GET | Export full index |
| `/api/v1/search` | POST | Semantic search |
| `/api/v1/files/{path}` | GET | File metadata with connections |
| `/api/v1/files/recent` | GET | Recently modified files |
| `/api/v1/files/related` | GET | Related files by path |
| `/api/v1/entities/search` | GET | Files mentioning entity |
| `/api/v1/rebuild` | POST | Trigger index rebuild |
| `/sse` | GET | Server-Sent Events stream |

### CLI Commands

**Location:** `cmd/graph/`

CLI commands for managing the FalkorDB Docker container.

**Structure:**
```
cmd/graph/
├── graph.go              # Parent command
└── subcommands/
    ├── start.go          # Start FalkorDB container
    ├── stop.go           # Stop container
    ├── status.go         # Show connection status
    └── rebuild.go        # Trigger rebuild via API
```

**Commands:**

| Command | Description |
|---------|-------------|
| `graph start` | Pull and start FalkorDB Docker container |
| `graph stop` | Stop and remove container |
| `graph status` | Show connection status and node counts |

To rebuild the graph, use `daemon rebuild [--force]`.

## Graph Schema

### Node Types

#### File Node

Represents an indexed file with its metadata and semantic analysis.

| Property | Type | Description |
|----------|------|-------------|
| `path` | string | Absolute file path (unique) |
| `name` | string | Filename |
| `hash` | string | SHA-256 content hash |
| `size` | int64 | File size in bytes |
| `category` | string | File category |
| `file_type` | string | File type |
| `modified_time` | int64 | Unix timestamp |
| `summary` | string | Semantic summary |
| `document_type` | string | Document classification |
| `confidence` | float64 | Analysis confidence |
| `readable` | bool | Human-readable flag |

#### Tag Node

Represents a semantic tag.

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Tag name (unique, lowercase) |

#### Topic Node

Represents a key topic from content analysis.

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Topic name (unique, lowercase) |

#### Entity Node

Represents a named entity (person, organization, concept).

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Entity name (unique) |
| `type` | string | Entity type (person, org, etc.) |

#### Category Node

Represents a file category (documents, images, code, data, other).

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Category name (unique) |

### Relationship Types

#### HAS_TAG

Connects File to Tag nodes.

```
(File)-[:HAS_TAG]->(Tag)
```

#### COVERS_TOPIC

Connects File to Topic nodes.

```
(File)-[:COVERS_TOPIC]->(Topic)
```

#### MENTIONS

Connects File to Entity nodes.

```
(File)-[:MENTIONS]->(Entity)
```

#### IN_CATEGORY

Connects File to Category nodes.

```
(File)-[:IN_CATEGORY]->(Category)
```

### Constraints and Indexes

**Unique Constraints:**
- `File.path` - Ensures no duplicate file entries
- `Tag.name` - Ensures tag deduplication
- `Topic.name` - Ensures topic deduplication
- `Entity.name` - Ensures entity deduplication
- `Category.name` - Ensures category deduplication

**Indexes:**
- Automatically created by unique constraints
- Support efficient lookups by unique properties

## Integration Points

### Daemon Integration

The daemon initializes and manages the graph connection:

```go
// In daemon.go
graphManager := graph.NewManager(logger)
if err := graphManager.Connect(ctx, graphAddr); err != nil {
    logger.Warn("FalkorDB not available", "error", err)
    // Continue without graph - graceful degradation
}
```

### Worker Pool Integration

Workers store processed entries in the graph:

```go
// After processing file
if graphManager != nil && graphManager.IsConnected() {
    if err := graphManager.Queries().UpsertFile(ctx, entry); err != nil {
        logger.Warn("failed to store in graph", "error", err)
    }
}
```

### MCP Server Integration

MCP server connects to daemon HTTP API for queries:

```go
// MCP tool handler
response, err := http.Post(daemonURL+"/api/v1/search", "application/json", body)
```

### SessionStart Hook Integration

The `read` command exports from graph when available:

```go
if graphManager != nil && graphManager.IsConnected() {
    index, err = exporter.ToIndex(ctx, memoryRoot)
} else {
    // Fall back to index file
    index, err = indexManager.Load()
}
```

## Configuration

### Graph Configuration

FalkorDB configuration in `config.yaml`:

```yaml
graph:
  # FalkorDB connection address
  address: "localhost:6379"

  # Connection timeout
  connect_timeout: 10s

  # Graph name
  graph_name: "memorizer"

  # Docker settings for graph start command
  docker:
    image: "falkordb/falkordb:latest"
    container_name: "memorizer-falkordb"
    redis_port: 6379
    ui_port: 3000
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `FALKORDB_PORT` | Redis protocol port | 6379 |
| `FALKORDB_UI_PORT` | Browser UI port | 3000 |
| `MEMORIZER_APP_DIR` | Data directory | ~/.agentic-memorizer |

### Docker Compose

Alternative to `graph start` command:

```yaml
# docker-compose.yml
services:
  falkordb:
    image: falkordb/falkordb:latest
    container_name: memorizer-falkordb
    ports:
      - "${FALKORDB_PORT:-6379}:6379"
      - "${FALKORDB_UI_PORT:-3000}:3000"
    volumes:
      - ${MEMORIZER_APP_DIR:-~/.agentic-memorizer}/falkordb:/data
    restart: unless-stopped
```

## Operational Guide

### Starting FalkorDB

**Option 1: CLI Command**
```bash
# Start container
agentic-memorizer graph start

# Verify status
agentic-memorizer graph status
```

**Option 2: Docker Compose**
```bash
# Start in background
docker-compose up -d

# View logs
docker-compose logs -f
```

**Option 3: Direct Docker**
```bash
docker run -d \
  --name memorizer-falkordb \
  -p 6379:6379 \
  -p 3000:3000 \
  -v ~/.agentic-memorizer/falkordb:/data \
  falkordb/falkordb:latest
```

### Health Monitoring

**Graph Status Command:**
```bash
agentic-memorizer graph status
```

Output:
```
FalkorDB Status:
  Container: memorizer-falkordb
  Status: running
  Connection: connected

Graph Statistics:
  Files: 57
  Tags: 148
  Topics: 211
  Entities: 0
  Categories: 5
  Relationships: 573
```

**Health Endpoint:**
```bash
curl http://localhost:8765/health | jq
```

Response:
```json
{
  "status": "healthy",
  "metrics": {
    "graph_connected": true,
    "graph_files": 57,
    "graph_tags": 148,
    "graph_topics": 211
  }
}
```

**Browser UI:**
Access FalkorDB browser at `http://localhost:3000` for visual graph exploration.

### Troubleshooting

#### Container Not Starting

```bash
# Check Docker status
docker ps -a | grep memorizer-falkordb

# View container logs
docker logs memorizer-falkordb

# Remove and restart
agentic-memorizer graph stop
agentic-memorizer graph start
```

#### Connection Failures

```bash
# Verify port binding
lsof -i :6379

# Test Redis protocol
redis-cli -h localhost -p 6379 ping

# Check daemon logs
tail -f ~/.agentic-memorizer/logs/daemon.log
```

#### Graph Data Issues

```bash
# Rebuild index (updates existing entries)
agentic-memorizer daemon rebuild

# Force rebuild (clears graph first, then rebuilds)
agentic-memorizer daemon rebuild --force

# Or via API directly
curl -X POST "http://localhost:8765/api/v1/rebuild?force=true"
```

#### Disk Space

FalkorDB stores data in `~/.agentic-memorizer/falkordb/`. Monitor disk usage:

```bash
du -sh ~/.agentic-memorizer/falkordb/
```

## API Reference

### POST /api/v1/search

Search files using semantic matching.

**Request:**
```json
{
  "query": "authentication",
  "limit": 10,
  "category": "code"
}
```

**Response:**
```json
{
  "results": [
    {
      "path": "/path/to/auth.go",
      "name": "auth.go",
      "category": "code",
      "summary": "Authentication middleware...",
      "tags": ["authentication", "middleware"],
      "score": 0.95
    }
  ],
  "count": 1
}
```

### GET /api/v1/files/{path}

Get file metadata with graph connections.

**Response:**
```json
{
  "entry": {
    "path": "/path/to/file.md",
    "name": "file.md",
    "category": "documents",
    "summary": "...",
    "tags": ["tag1", "tag2"],
    "topics": ["topic1"]
  },
  "connections": {
    "tags": ["tag1", "tag2"],
    "topics": ["topic1"],
    "entities": [],
    "related_files": [...]
  }
}
```

### GET /api/v1/files/recent

Get recently modified files.

**Query Parameters:**
- `days` (int, default: 7) - Days to look back
- `limit` (int, default: 20) - Max results

### GET /api/v1/files/related

Get files related by shared tags/topics.

**Query Parameters:**
- `path` (string, required) - File path
- `limit` (int, default: 10) - Max results

### GET /api/v1/entities/search

Find files mentioning an entity.

**Query Parameters:**
- `entity` (string, required) - Entity name
- `limit` (int, default: 10) - Max results

### POST /api/v1/rebuild

Trigger index rebuild.

**Query Parameters:**
- `force` (bool, default: false) - Clear graph before rebuild

**Response:**
```json
{
  "status": "started",
  "message": "Rebuild started in background"
}
```

## Glossary

| Term | Definition |
|------|------------|
| **FalkorDB** | Redis-compatible graph database using Cypher query language |
| **Cypher** | Graph query language originally developed for Neo4j |
| **Node** | Vertex in the graph representing an entity (File, Tag, Topic) |
| **Relationship** | Edge connecting two nodes (HAS_TAG, COVERS_TOPIC) |
| **MERGE** | Cypher operation that creates or matches existing nodes |
| **Constraint** | Database rule ensuring uniqueness of properties |
| **Graceful Degradation** | System continues operating when dependencies unavailable |
| **SSE** | Server-Sent Events for real-time streaming updates |

---

**Last Updated:** 2025-11-30

**Related Documentation:**
- [Daemon Subsystem](../daemon/README.md)
- [MCP Server](../mcp/README.md)
- [Semantic Search](../semantic-search/README.md)
