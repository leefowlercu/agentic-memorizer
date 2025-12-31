# Graph Storage

FalkorDB-powered knowledge graph for files, semantic relationships, and intelligent discovery.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Graph subsystem provides persistent storage and relationship-based queries for the knowledge graph that underlies Agentic Memorizer. Built on FalkorDB (a Redis-compatible graph database), it stores files as nodes with semantic relationships to tags, topics, entities, categories, and directories. This graph structure enables powerful discovery features including semantic search, related file suggestions, entity disambiguation, and knowledge gap analysis.

The subsystem manages the complete lifecycle of graph data: connection pooling, schema initialization with indexes, node and edge CRUD operations, query execution, and data export. It implements a layered architecture where the Manager facade coordinates specialized handlers for different concerns (nodes, edges, queries, facts, analytics).

Key capabilities include:

- **Persistent storage** - Files, semantic metadata, and user facts stored in FalkorDB with HNSW vector indexes for embedding similarity
- **Relationship navigation** - Tags, topics, entities, and references linked via typed edges enabling multi-hop traversals
- **Semantic search** - Full-text search on summaries plus vector search on embeddings with scoring
- **Entity disambiguation** - Automatic normalization and deduplication of entity names with alias resolution
- **Knowledge analytics** - Clustering, recommendations, temporal analysis, and gap detection
- **User facts** - CRUD operations for persistent user-defined context facts

## Design Principles

### Handler Pattern with Facade

The graph subsystem separates concerns into specialized handler structs (Nodes, Edges, Queries, Facts, Disambiguation, Recommendations, Clusters, GapAnalysis, Temporal) that each receive a shared Client instance. The Manager acts as a facade, orchestrating these handlers through a unified public API while keeping internal complexity encapsulated. This enables focused testing, clear responsibilities, and composable operations.

### MERGE-Based Upserts

All node creation uses Cypher's `MERGE` statement for idempotent operations. This eliminates the need for explicit existence checks and simplifies concurrent access patterns. When a file is updated, its edges are removed and recreated (full rebuild strategy) rather than computing deltas, trading minor inefficiency for guaranteed consistency.

### Lazy Connection with Verification

The FalkorDB client uses lazy initialization but verifies connectivity with a test query (`RETURN 1`) during the Connect phase. This catches misconfiguration early while deferring actual connection establishment until needed. Health checks execute the same verification query.

### Thread Safety via RWMutex

The client implements a dual mutex strategy: a main RWMutex for state protection (allowing parallel read queries while serializing writes) and a dedicated mutex for close operations to prevent double-close race conditions. All query methods acquire appropriate locks before execution.

### Graceful Degradation

When individual edge operations fail during file updates, the Manager logs warnings but continues processing. This partial-success model ensures that a single malformed tag or entity does not prevent the rest of the file's metadata from being stored. Export operations similarly continue collecting available data when optional analytics fail.

### Schema-First Initialization

The Manager initializes components in dependency order: Schema first (creates constraints and indexes), then core handlers (Nodes, Edges, Facts), then read-only handlers (Queries), and finally analytics handlers. This ensures required indexes exist before any data operations.

## Key Components

### Client (`client.go`)

The Client wraps the FalkorDB Go library, providing connection lifecycle management, health monitoring, and type-safe query execution. Key responsibilities include establishing connections with optional password authentication, executing Cypher queries with parameter passing, providing QueryResult wrappers with typed accessors (GetString, GetInt64, GetFloat64), and maintaining connection state with thread-safe checks.

### Manager (`manager.go`)

The Manager orchestrates all graph operations through a unified API. It composes specialized handlers and exposes high-level methods like UpdateSingle for file upserts, Search for multi-dimensional queries, GetRelatedFiles for connection traversal, and GetStats for graph metrics. The UpdateSingle method handles the complete file update flow: upsert the file node, remove existing edges, and create edges to category, directory, tags, topics, entities, and references.

### Schema (`schema.go`)

Schema defines the graph structure and handles initialization. It creates seven node labels (File, Tag, Topic, Category, Entity, Directory, Fact) with appropriate properties, eight relationship types (HAS_TAG, COVERS_TOPIC, IN_CATEGORY, MENTIONS, REFERENCES, SIMILAR_TO, IN_DIRECTORY, PARENT_OF), range indexes for fast property lookups, a full-text index on File.summary for text search, and an HNSW vector index on File.embedding for similarity search.

### Nodes (`nodes.go`)

The Nodes handler provides node CRUD operations including UpsertFile for creating or updating file nodes with all metadata properties, GetFile with smart path resolution (absolute vs relative), GetOrCreate methods for tags, topics, entities, and directories using MERGE patterns, DeleteFile with cascade to all connected edges, and enumeration methods for listing all tags, topics, or entities.

### Edges (`edges.go`)

The Edges handler manages relationship creation and traversal. It provides LinkFileTo methods for connecting files to tags, topics, categories, entities, references, and directories. Entity normalization applies during linking, converting names to lowercase and resolving common aliases (e.g., "tf" to "terraform", "k8s" to "kubernetes"). Retrieval methods return connected nodes with optional scoring.

### Queries (`queries.go`)

The Queries handler provides read-only graph traversals. Search methods span multiple dimensions: VectorSearch for embedding similarity, FullTextSearch on summaries, SearchByFilename for name matching, and SearchByTag/Topic/Entity for relationship-based discovery. GetRecentFiles filters by modification time, GetRelatedFiles calculates connection strength across shared tags and topics, and GetGraphOverview aggregates statistics.

### Facts (`facts.go`)

The Facts handler implements CRUD for user-defined context facts. Facts are stored as nodes with UUID identifiers, content (10-500 characters), timestamps, and source attribution. The handler enforces a 50-fact limit and provides duplicate detection. Facts integrate with integration hooks for context injection.

### Disambiguation (`disambiguation.go`)

Entity disambiguation handles name variations through normalization (lowercase, whitespace cleanup, alias resolution) and deduplication. Over 100 built-in aliases map abbreviations to canonical forms. FindDuplicateEntities detects collisions, MergeEntities redirects relationships, and NormalizeAllEntities performs batch cleanup.

### Analytics Handlers

Four specialized handlers provide knowledge analytics:

**Recommendations** calculates weighted scores based on shared tags, topics, entities, category, and directory membership to suggest related files. Weights favor entities (2.0) over topics (1.5) over tags (1.0).

**Clusters** groups files by shared connections, detecting topic clusters, entity clusters, and overlapping clusters where files bridge multiple knowledge areas.

**Temporal** analyzes modification patterns including recent changes, co-modified file pairs within time windows, activity hotspots by directory, and stale file detection.

**GapAnalysis** identifies documentation gaps: under-documented topics, orphaned tags and entities, isolated files without semantic connections, and coverage statistics.

### Export (`export.go`)

The Export handler transforms graph data into output formats. ToFileIndex converts IndexEntry graph data to FileEntry structures with optional verbose mode that adds related files and insights (recommendations, clusters, gaps). ToSummary provides a condensed view with top tags, topics, entities, and statistics for context-limited scenarios.

## Integration Points

### Daemon Subsystem

The daemon creates a Manager instance during startup and uses it for all file processing. When files are discovered or modified, the daemon calls UpdateSingle to persist metadata and semantic analysis results. The Manager's GetAll method populates the file index for integration hooks, and Search powers the HTTP API's semantic search endpoint.

### Cache Subsystem

The cache uses content hashes (SHA-256) as part of its key structure. When the graph stores a file's hash property, the daemon can detect content changes by comparing stored hashes against newly computed ones. This enables the graph to serve as the source of truth for file state while the cache handles analysis result storage.

### Semantic Analysis

Semantic analysis produces tags, topics, entities, and embeddings that flow into the graph as relationships. The Manager's UpdateSingle method creates edges from files to these semantic elements. Vector embeddings enable similarity search through the HNSW index, complementing keyword and relationship-based search.

### HTTP API

The daemon's HTTP API delegates to the Manager for data operations. The `/api/v1/search` endpoint calls Manager.Search, `/api/v1/files/{path}` calls GetFile and GetFileConnections, `/api/v1/files/related` calls GetRelatedFiles, and `/api/v1/entities/search` calls SearchByEntity. Health endpoints include graph statistics.

### MCP Server

The MCP server's tools map directly to Manager methods: search_files uses Search, get_file_metadata uses GetFile with GetFileConnections, list_recent_files uses GetRecentFiles, get_related_files uses GetRelatedFiles, and search_entities uses SearchByEntity. The Manager provides the data layer for all MCP operations.

### Integration Hooks

Integration hooks (SessionStart, UserPromptSubmit) use the Export handler's ToFileIndex method to generate the context payload. Facts retrieved via the Facts handler are formatted and injected into the UserPromptSubmit hook for persistent user context.

### CLI Commands

Graph CLI commands interact directly with the subsystem. The `graph status` command checks FalkorDB connectivity and displays graph statistics. The `daemon rebuild` command triggers graph clearing and repopulation. The `read files` command uses Export.ToFileIndex for output generation.

## Glossary

**Cypher**
A declarative graph query language used by FalkorDB for pattern matching, data manipulation, and traversal. Queries use MATCH for patterns, MERGE for upserts, CREATE for inserts, and RETURN for results.

**Edge**
A directed relationship between two nodes in the graph, carrying a type (e.g., HAS_TAG, MENTIONS) and optional properties (e.g., score, confidence). Edges enable relationship-based navigation and discovery.

**Entity**
A named concept extracted from semantic analysis, classified by type (technology, person, concept, organization). Entities are normalized to canonical lowercase forms with alias resolution for consistency.

**FalkorDB**
A Redis-compatible graph database that stores nodes and relationships with property support. It provides Cypher query execution, full-text search, and HNSW vector indexes for similarity search.

**Fact**
A user-defined piece of context stored in the graph and injected into AI conversations via integration hooks. Facts enable persistent customization of AI assistant behavior.

**HNSW (Hierarchical Navigable Small World)**
A graph-based algorithm for approximate nearest neighbor search on high-dimensional vectors. The graph subsystem uses HNSW indexing on File.embedding properties for semantic similarity search.

**Knowledge Graph**
The interconnected structure of files, tags, topics, entities, categories, and directories stored in FalkorDB. Relationships between nodes encode semantic meaning and enable discovery features.

**Manager**
The facade component that orchestrates all graph operations, composing specialized handlers and exposing a unified API for the rest of the application.

**MERGE**
A Cypher clause that performs an upsert: creates a node/edge if it doesn't exist, or matches it if it does. This enables idempotent operations without explicit existence checks.

**Node**
A vertex in the graph representing an entity (File, Tag, Topic, Entity, Category, Directory, Fact) with associated properties. Nodes are connected by edges to form the knowledge graph.

**Normalization**
The process of converting entity names to a canonical form through lowercasing, whitespace cleanup, and alias resolution. This ensures consistent entity matching despite name variations.

**QueryResult**
A wrapper around FalkorDB query results that provides type-safe accessors (GetString, GetInt64, GetFloat64) with fallback defaults, simplifying result processing.

**Schema**
The defined structure of the graph including node labels, relationship types, and indexes. The Schema handler initializes this structure during Manager startup.
