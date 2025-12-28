# Index Management Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Graph-Native Storage](#graph-native-storage)
   - [Real-Time Persistence](#real-time-persistence)
   - [Exporter Pattern](#exporter-pattern)
   - [Thread Safety](#thread-safety)
   - [Separation of Concerns](#separation-of-concerns)
3. [Key Components](#key-components)
   - [Graph Manager](#graph-manager)
   - [Graph Exporter](#graph-exporter)
   - [FileIndex Structure](#graphindex-structure)
   - [Type Definitions](#type-definitions)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Read Command](#read-command)
   - [HTTP API](#http-api)
   - [Type System](#type-system)
5. [Operational Patterns](#operational-patterns)
   - [Full Index Rebuild](#full-index-rebuild)
   - [Incremental Updates](#incremental-updates)
   - [Concurrent Access](#concurrent-access)
   - [Startup and Recovery](#startup-and-recovery)
6. [Glossary](#glossary)

## Overview

The Index Management subsystem manages the precomputed memory index through a graph-based architecture. Unlike traditional file-based indexes, this system stores all file metadata and semantic relationships in a FalkorDB graph database, providing rich relationship queries, real-time updates, and graph analytics capabilities.

### Purpose

The Index Management subsystem provides several critical capabilities:

- **Graph-Native Storage**: All file metadata, semantic analysis, and relationships are stored in FalkorDB (Redis-compatible graph database)
- **Real-Time Updates**: Changes are persisted immediately to the graph database without intermediate file operations
- **Rich Queries**: Supports semantic search across tags, topics, entities, and relationships via Cypher queries
- **On-Demand Export**: Converts graph data to FileIndex format when needed by consumers (read command, HTTP API)
- **Graph Analytics**: Provides recommendations, cluster detection, gap analysis, and temporal tracking

### Role in the System

The Index Management subsystem acts as the persistence and query layer for the memory index. The Graph Manager handles all CRUD operations on the index, while the Graph Exporter converts graph data to the FileIndex format when needed. The daemon streams file processing results directly to the graph, and consumers query the graph to retrieve index data in various formats.

## Design Principles

### Graph-Native Storage

All index data is stored in FalkorDB, a Redis-compatible graph database. Files become File nodes, and semantic elements (tags, topics, entities) become their own nodes with relationships connecting them. This enables powerful graph queries like "find files related to this file through shared topics" or "find all files mentioning this entity."

The graph schema includes:
- **Node Types**: File, Tag, Topic, Entity, Category, Directory
- **Relationship Types**: HAS_TAG, COVERS_TOPIC, MENTIONS, IN_CATEGORY, IN_DIRECTORY, REFERENCES

This graph-native approach replaces the old file-based index with a more powerful and flexible storage system.

### Real-Time Persistence

Every file change is immediately persisted to FalkorDB via the Graph Manager's UpdateSingle or RemoveFile methods. There is no intermediate "save" step or batch write operation. The graph database is always the authoritative source of truth for the index state.

When the daemon processes a file, it calls `graphManager.UpdateSingle(ctx, entry, info)` which:
1. Creates or updates the File node with metadata
2. Removes old relationships (for updates)
3. Creates new relationships to tags, topics, entities, and categories
4. Returns UpdateResult indicating whether this was an addition or update

This real-time approach ensures the index is always current and eliminates the need for atomic file write operations.

### Exporter Pattern

The FileIndex structure (the public output format) is not stored directly. Instead, the Graph Exporter (`internal/graph/export.go`) converts graph data to FileIndex format on-demand when consumers need it.

This pattern provides several benefits:
- **Always Fresh**: Index output is always current since it's generated from the live graph
- **Flexible Output**: Different export modes (normal vs verbose) can be generated from the same graph data
- **Efficient Storage**: The graph stores only the core relationships; derived data (like related files) is computed on-demand

The read command and HTTP API both use the Graph Exporter's `ToFileIndex()` method to produce FileIndex output.

### Thread Safety

The Graph Manager uses `sync.RWMutex` to protect concurrent access to its connection state. FalkorDB itself provides its own concurrency control through connection pooling. This combination enables safe concurrent operations without the need for an in-memory index lock.

Read operations acquire read locks, allowing multiple concurrent readers. Write operations (UpdateSingle, RemoveFile, ClearGraph) acquire write locks for exclusive access. This design maximizes concurrency while ensuring data consistency.

### Separation of Concerns

The Index Management subsystem is cleanly separated into distinct responsibilities:

- **Graph Manager** (`internal/graph/manager.go`) - CRUD operations, connection management, query orchestration
- **Graph Exporter** (`internal/graph/export.go`) - Converting graph data to FileIndex format
- **Nodes** (`internal/graph/nodes.go`) - Low-level node CRUD operations
- **Edges** (`internal/graph/edges.go`) - Low-level relationship management
- **Queries** (`internal/graph/queries.go`) - Complex read-only queries (search, related files, recent files)
- **Analytics** (`internal/graph/disambiguation.go`, `recommendations.go`, `clusters.go`, `gaps.go`, `temporal.go`) - Advanced graph analytics

This separation enables each component to be tested, understood, and modified independently.

## Key Components

### Graph Manager

The Manager type (`internal/graph/manager.go`) provides the primary interface for index operations. It orchestrates all graph operations including CRUD, queries, and analytics.

**Constructor:**
- `NewManager(config ManagerConfig, logger *slog.Logger) *Manager` - Creates a new graph manager with configuration for FalkorDB connection and schema

**Lifecycle Operations:**
- `Initialize(ctx) error` - Connects to FalkorDB and initializes schema
- `Close() error` - Closes the graph connection
- `IsConnected() bool` - Returns connection status
- `Health(ctx) (*HealthStatus, error)` - Returns health status with graph metrics

**Core Index Operations:**
- `UpdateSingle(ctx, entry IndexEntry, info UpdateInfo) (UpdateResult, error)` - Creates or updates a single file entry with its relationships
- `UpdateSingleWithEmbedding(ctx, entry IndexEntry, info UpdateInfo, embedding []float32) (UpdateResult, error)` - Updates with vector embedding for similarity search
- `RemoveFile(ctx, path string) error` - Removes a file and all its relationships from the graph
- `GetAll(ctx) ([]IndexEntry, error)` - Retrieves all file entries (similar to old GetCurrent)
- `GetFile(ctx, path string) (*IndexEntry, error)` - Retrieves a single file entry
- `GetStats(ctx) (*Stats, error)` - Returns graph statistics (file counts, node counts, edge counts)
- `ClearGraph(ctx) error` - Removes all data from the graph (used during `daemon rebuild --force`)

**Search Operations:**
- `Search(ctx, query string, limit int, categoryFilter string) ([]SearchResult, error)` - Multi-dimensional search across filename, tags, topics, entities
- `VectorSearch(ctx, embedding []float32, limit int) ([]SearchResult, error)` - Vector similarity search (when embeddings enabled)
- `GetRecentFiles(ctx, days int, limit int) ([]SearchResult, error)` - Returns recently modified files
- `GetRelatedFiles(ctx, filePath string, limit int) ([]RelatedFile, error)` - Finds files related through shared tags/topics/entities
- `GetFileConnections(ctx, filePath string) (*FileConnections, error)` - Returns all connections for a file

**Analytics Operations:**
- `NormalizeEntities(ctx) (int64, error)` - Entity deduplication and normalization
- `RecommendRelated(ctx, filePath string, limit int) ([]Recommendation, error)` - File recommendations based on graph relationships
- `DetectTopicClusters(ctx, minSize int) ([]Cluster, error)` - Finds clusters of files sharing common topics
- `AnalyzeGaps(ctx) (*GapReport, error)` - Identifies coverage gaps in the knowledge base
- `GetRecentModifications(ctx, since time.Time, limit int) ([]ModificationEvent, error)` - Temporal tracking of file changes

**UpdateInfo and UpdateResult Types:**

```go
// UpdateInfo provides context about the entry being updated
type UpdateInfo struct {
    WasAnalyzed bool // true if semantic analysis was performed (API call)
    WasCached   bool // true if cached analysis was used
    HadError    bool // true if there was an error processing this file
}

// UpdateResult contains information about what the update operation did
type UpdateResult struct {
    Added   bool // true if new entry was added (vs updated)
    Updated bool // true if existing entry was modified
}
```

The daemon uses these types to track processing outcomes and update health metrics appropriately.

### Graph Exporter

The Exporter type (`internal/graph/export.go`) handles converting graph data to the FileIndex output format.

**Constructor:**
- `NewExporter(manager *Manager, logger *slog.Logger) *Exporter` - Creates a new exporter

**Export Methods:**
- `ToFileIndex(ctx, memoryRoot string, verbose ...bool) (*FileIndex, error)` - Exports graph to FileIndex format
  - Normal mode: Returns files with metadata, semantic analysis, and knowledge summary
  - Verbose mode: Includes related files per entry and graph insights (recommendations, clusters, gaps)
- `ToSummary(ctx, memoryRoot string, recentDays, topN int) (*ExportSummary, error)` - Exports condensed summary for context windows
- `GetFileEntry(ctx, path string, relatedLimit int) (*FileEntry, error)` - Exports a single file with related files

The exporter queries the graph via the Graph Manager's GetAll method, retrieves statistics, computes coverage metrics, and converts internal IndexEntry structures to flattened FileEntry structures for output.

### FileIndex Structure

The FileIndex type (`pkg/types/types.go`) represents the public output format consumed by integrations and the read command. This is a flattened, graph-native structure optimized for consumption.

```go
type FileIndex struct {
    Generated  time.Time        // When index was exported
    MemoryRoot string           // Memory directory root
    Files      []FileEntry      // Flattened file list
    Stats      IndexStats       // Aggregate statistics
    Knowledge  *KnowledgeSummary // Top tags, topics, entities (optional)
    Insights   *IndexInsights    // Graph analytics (verbose mode only)
}

type FileEntry struct {
    // Identity
    Path string
    Name string
    Hash string

    // Classification
    Type     string // file extension
    Category string // documents, code, images, etc.

    // Physical attributes
    Size       int64
    SizeHuman  string
    Modified   time.Time
    IsReadable bool

    // Type-specific metadata (optional)
    WordCount  *int
    PageCount  *int
    SlideCount *int
    Dimensions *ImageDim
    Duration   *string
    Language   *string
    Author     *string

    // Semantic understanding
    Summary      string
    DocumentType string
    Confidence   float64

    // Graph relationships
    Tags     []string
    Topics   []string
    Entities []EntityRef

    // Related files (verbose mode only)
    RelatedFiles []RelatedFile

    // Error (if analysis failed)
    Error *string
}
```

Key differences from the old nested IndexEntry format:
- **Flattened structure**: No nested Metadata/Semantic objects
- **Graph relationships**: Tags, topics, entities are top-level arrays
- **Related files**: Computed from graph relationships, included in verbose mode
- **Human-readable sizes**: SizeHuman field for display
- **Knowledge summary**: Top tags/topics/entities from the entire graph
- **Insights**: Graph analytics results (recommendations, clusters, gaps) in verbose mode

### Type Definitions

The Index Management subsystem relies on several type definitions:

**Internal Processing Types** (used within the processing pipeline):
- `FileInfo` - Basic file metadata
- `FileMetadata` - Extracted file-specific metadata (word counts, dimensions, etc.)
- `SemanticAnalysis` - AI-generated understanding (summary, tags, topics, entities)
- `IndexEntry` - Combines metadata and semantic analysis (internal format)

**Graph-Native Types** (public output format):
- `FileIndex` - The graph-native index structure
- `FileEntry` - Flattened file representation optimized for output
- `KnowledgeSummary` - Overview of knowledge landscape (top tags, topics, entities)
- `IndexInsights` - Graph analytics results (recommendations, clusters, gaps)
- `RelatedFile` - A file related through graph connections

**Statistics Types**:
- `IndexStats` - Aggregate statistics (file counts, graph metrics, coverage metrics)

These types are defined in `pkg/types/types.go` and are shared across all subsystems.

## Integration Points

### Daemon Subsystem

The daemon (`internal/daemon/daemon.go`) is the primary producer of index data. It creates and initializes the Graph Manager during startup, processes files through the worker pool, and streams results to the graph.

**Initialization:**
```go
graphManager := graph.NewManager(graphConfig, logger)
if err := graphManager.Initialize(ctx); err != nil {
    return fmt.Errorf("failed to initialize graph; %w", err)
}
```

**Full Rebuild:**
During a full rebuild, the daemon:
1. Walks the entire memory directory to collect file paths
2. Submits paths to the worker pool for parallel processing
3. For each result, calls `graphManager.UpdateSingle(ctx, entry, info)`
4. Tracks statistics via health metrics
5. Broadcasts updates via SSE

**Incremental Updates:**
For file system events (create, modify), the daemon:
1. Processes the single file through the worker pool
2. Calls `graphManager.UpdateSingle(ctx, entry, info)` with UpdateInfo tracking whether analysis was performed or cached
3. Broadcasts update via SSE

For file deletions:
1. Calls `graphManager.RemoveFile(ctx, path)`
2. Broadcasts deletion via SSE

The daemon never reads the index for display purposes—it only writes to the graph.

### Read Command

The read command (`cmd/read/read.go`) is a consumer of the graph data. It connects to FalkorDB, exports the graph to FileIndex format, and outputs it in the requested format (XML, Markdown, or JSON).

**Workflow:**
1. Initialize configuration
2. Connect to FalkorDB via `graph.NewManager()` and `Initialize()`
3. Create Graph Exporter via `graph.NewExporter()`
4. Export graph via `exporter.ToFileIndex(ctx, memoryRoot, verbose)`
5. Format output via output processors (XML, Markdown, JSON)
6. Optionally wrap in integration-specific envelope

The read command does not depend on the daemon running—it queries FalkorDB directly. If FalkorDB is unavailable, it displays a warning and outputs an empty index.

### HTTP API

The HTTP API (`internal/daemon/api/server.go`) provides programmatic access to the index via REST endpoints:

- `GET /health` - Daemon health with graph metrics
- `GET /api/v1/index` - Full index export as FileIndex (supports `?verbose=true`)
- `POST /api/v1/search` - Semantic search across the graph
- `GET /api/v1/files/{path}` - Single file metadata with related files
- `GET /api/v1/files/recent` - Recently modified files
- `GET /api/v1/files/related` - Related files for a given path
- `POST /api/v1/entities/search` - Search for files mentioning an entity
- `POST /api/v1/rebuild` - Trigger full rebuild
- `GET /sse` - Server-Sent Events for real-time updates

All endpoints query the Graph Manager and use the Graph Exporter when FileIndex output is needed.

### Type System

The Index Management subsystem depends on the `pkg/types` package for core data structures. This dependency is one-way: the subsystem imports and uses types but does not expose graph-specific types to other packages.

The type system defines:
- **Internal types** (IndexEntry, FileMetadata, SemanticAnalysis) used within the processing pipeline
- **Graph-native types** (FileIndex, FileEntry) used for output
- **Statistics types** (IndexStats) used for metrics

The separation allows the type system to evolve independently. New metadata fields or semantic properties can be added to `pkg/types` without modifying the Graph Manager, as long as the core structure remains compatible.

## Operational Patterns

### Full Index Rebuild

A full index rebuild is triggered by:
- `daemon start` (if graph is empty)
- `daemon rebuild` command
- Periodic rebuild (controlled by `daemon.full_rebuild_interval_minutes` config setting)

**Workflow:**
1. Optionally clear the graph (`ClearGraph` if `--force` flag is used)
2. Walker traverses the entire memory directory collecting file paths
3. Paths are submitted to the worker pool for parallel processing
4. For each result:
   - Call `graphManager.UpdateSingle(ctx, entry, info)` to update the graph
   - Update health metrics (file count, analyzed count, cached count, error count)
   - Broadcast SSE notification
5. Log final statistics (duration, files processed, cache hits, API calls)

The rebuild streams updates directly to FalkorDB—there is no final "save" step. The graph is updated incrementally as files are processed.

### Incremental Updates

Incremental updates optimize the common case where only one or a few files change. When the file watcher detects a file modification or creation:

1. Daemon processes the single file through the worker pool
2. Worker returns IndexEntry with UpdateInfo (WasAnalyzed, WasCached, HadError flags)
3. Daemon calls `graphManager.UpdateSingle(ctx, entry, info)`
4. UpdateSingle:
   - Checks if file already exists in graph
   - Creates or updates File node with metadata
   - Removes old relationships (for updates)
   - Creates new relationships to tags, topics, entities, categories
   - Returns UpdateResult (Added or Updated flag)
5. Daemon uses UpdateResult to update health metrics
6. Daemon broadcasts SSE notification

For file deletions:
1. Daemon calls `graphManager.RemoveFile(ctx, path)`
2. RemoveFile deletes the File node and all its relationships
3. Daemon updates health metrics
4. Daemon broadcasts SSE notification

Incremental updates ensure the graph reflects the file system state in near real-time.

### Concurrent Access

The Graph Manager uses `sync.RWMutex` to protect concurrent access to its connection state. FalkorDB provides its own concurrency control through connection pooling.

**Read Operations** (GetAll, GetFile, GetStats, Search, etc.):
- Acquire read lock (`mu.RLock()`)
- Allow multiple concurrent readers without blocking each other
- Check connection status
- Execute query via FalkorDB client
- Release read lock (`mu.RUnlock()`)

**Write Operations** (UpdateSingle, RemoveFile, ClearGraph):
- Acquire write lock (`mu.Lock()`)
- Ensure exclusive access
- Check connection status
- Execute write operations via FalkorDB client
- Release write lock (`mu.Unlock()`)

This locking strategy balances performance and safety. Reads are fast and non-blocking in the common case. Writes are protected but do not block readers longer than necessary.

### Startup and Recovery

During daemon startup:

1. Initialize configuration
2. Create and initialize Graph Manager via `NewManager()` and `Initialize(ctx)`
3. `Initialize(ctx)`:
   - Connects to FalkorDB
   - Initializes schema (creates constraints and indexes)
   - Initializes sub-components (nodes, edges, queries, analytics)
4. If connection fails, return error (daemon cannot start without graph)
5. If connection succeeds, check if graph is empty
6. If graph is empty, trigger full rebuild
7. If graph has data, resume with existing state

The graph database is the authoritative source of truth. There are no index files to recover from disk. If FalkorDB is unavailable, the daemon cannot start (though the read command will show a warning and empty index).

This design ensures the daemon always has a valid graph connection and eliminates the concept of "crash recovery" from file-based indexes.

## Glossary

**Cypher**: The query language for graph databases, used by FalkorDB to query nodes and relationships

**Entity**: A named entity extracted from file content (person, organization, technology, concept, project) stored as Entity nodes in the graph

**Exporter Pattern**: Design pattern where the public output format (FileIndex) is generated on-demand from the graph database rather than being stored directly

**FalkorDB**: Redis-compatible graph database that stores the index as nodes and relationships

**FileEntry**: The flattened file representation in the FileIndex output format, optimized for consumption by integrations

**Graph Manager**: The component responsible for all CRUD operations, queries, and analytics on the graph database (`internal/graph/manager.go`)

**Graph-Native Storage**: Architecture where all index data is stored in a graph database with nodes and relationships, enabling rich semantic queries

**FileIndex**: The public output format for the memory index, containing files, statistics, knowledge summary, and optional insights

**IndexEntry**: Internal processing type that combines FileMetadata and SemanticAnalysis, used within the processing pipeline before storage in the graph

**Knowledge Graph**: The network of files, tags, topics, entities, and their relationships stored in FalkorDB

**Real-Time Persistence**: Design pattern where changes are persisted immediately to the graph database without intermediate file operations or batch writes

**Related Files**: Files connected through shared tags, topics, entities, or other graph relationships, computed on-demand from the graph

**Semantic Search**: Multi-dimensional search across filename, tags, topics, entities, and summary using graph queries

**Tag**: A semantic tag extracted from file content during analysis, stored as Tag nodes in the graph with HAS_TAG relationships to files

**Topic**: A key topic identified from file content, stored as Topic nodes in the graph with COVERS_TOPIC relationships to files

**UpdateInfo**: Context structure tracking whether a file update involved semantic analysis (API call), cached analysis, or an error

**UpdateResult**: Result structure indicating whether UpdateSingle added a new file or updated an existing one

**Verbose Mode**: Export mode that includes related files per entry and graph insights (recommendations, clusters, gaps) in the FileIndex output
