# Semantic Search Subsystem Documentation

**Last Updated:** 2025-12-09

## Table of Contents

1. [Overview](#overview)
   - [Purpose](#purpose)
   - [Role in the System](#role-in-the-system)
   - [Key Features](#key-features)
2. [Design Principles](#design-principles)
   - [Graph-First Architecture](#graph-first-architecture)
   - [Graceful Degradation](#graceful-degradation)
   - [Multi-Signal Search](#multi-signal-search)
   - [Relationship-Aware Queries](#relationship-aware-queries)
3. [Key Components](#key-components)
   - [Graph Manager Search](#graph-manager-search)
   - [Cypher Query Layer](#cypher-query-layer)
   - [Fallback In-Memory Search](#fallback-in-memory-search)
   - [HTTP API Integration](#http-api-integration)
4. [Integration Points](#integration-points)
   - [Daemon HTTP API](#daemon-http-api)
   - [MCP Server](#mcp-server)
   - [FalkorDB Graph Database](#falkordb-graph-database)
5. [Glossary](#glossary)
6. [Additional Resources](#additional-resources)

---

## Overview

### Purpose

The Semantic Search subsystem provides graph-powered, relationship-aware search capabilities across the indexed file knowledge base. It enables AI assistants and users to discover relevant files through multiple search dimensions: filenames, tags, topics, entities, categories, and semantic summaries.

The subsystem operates in two modes:

1. **Primary Mode (Graph-Based)**: Cypher queries against FalkorDB that leverage the knowledge graph's relationship structure for context-aware search
2. **Fallback Mode (In-Memory)**: Token-based search using weighted scoring when FalkorDB is unavailable

This dual-mode architecture ensures search remains available even when the graph database is offline, while providing superior relationship-aware results when the graph is operational.

### Role in the System

The Semantic Search subsystem serves as the primary query interface for the knowledge graph, transforming it from passive storage into an actively queryable knowledge base. While the graph subsystem maintains file relationships and the semantic analyzer generates understanding, this subsystem enables runtime discovery and context-aware retrieval.

Key responsibilities:

- **Graph Query Orchestration**: Execute multi-dimensional Cypher queries against FalkorDB
- **Relationship Traversal**: Leverage graph edges (HAS_TAG, COVERS_TOPIC, MENTIONS) for connected searches
- **Result Deduplication**: Merge results from multiple query types while avoiding duplicates
- **Graceful Fallback**: Switch to in-memory token-based search when graph is unavailable
- **Category Filtering**: Narrow searches to specific file types (documents, code, images)
- **API Exposure**: Provide search endpoints via daemon HTTP API and MCP server tools

The subsystem powers all search interactions: MCP tools (`search_files`, `get_related_files`, `search_entities`), daemon HTTP endpoints, and SessionStart hook index generation.

### Key Features

**Graph-Powered Multi-Signal Search**
- Combines filename, tag, topic, and entity searches using Cypher queries
- Traverses graph relationships (HAS_TAG, COVERS_TOPIC, MENTIONS edges)
- Deduplicates results across multiple query dimensions
- Applies optional category filtering after deduplication
- Returns unified result set ordered by relevance

**Relationship-Aware Queries**
- Find files sharing tags/topics with a given file
- Discover files mentioning specific entities (with normalized name matching)
- Calculate connection strength based on shared relationships
- Identify related files through multi-hop graph traversal

**Specialized Search Types**
- **Filename Search**: Substring matching on file basenames
- **Tag Search**: Files connected via HAS_TAG relationships
- **Topic Search**: Files connected via COVERS_TOPIC relationships
- **Entity Search**: Files connected via MENTIONS relationships (supports normalized entity names)
- **Category Search**: Files in specific categories (documents, code, images, etc.)
- **Recent Files**: Temporal queries for files modified within time windows
- **Vector Search**: Semantic similarity using FalkorDB vector index (when embeddings enabled)
- **Full-Text Search**: FalkorDB full-text index on file summaries

**Graceful Degradation**
- Automatic fallback to in-memory token-based search when graph unavailable
- Weighted scoring in fallback mode (filename 3.0, summary 2.0, tags 1.5, topics 1.0, category 1.0, file_type 0.5, document_type 0.5)
- Query tokenization with stop word filtering (26 common words)
- Proportional scoring: `(matched_tokens / total_tokens) × field_weight`
- Results indicate source ("daemon" for graph, "index" for fallback)

---

## Design Principles

### Graph-First Architecture

The subsystem prioritizes graph-based search as the primary implementation, leveraging FalkorDB's relationship-aware query capabilities.

**Why Graph-First:**
- **Context Awareness**: Cypher queries traverse relationships to find semantically related files, not just keyword matches
- **Multi-Hop Discovery**: Can find files connected through intermediate nodes (e.g., files sharing entities)
- **Relationship Strength**: Calculates connection strength based on number of shared tags/topics/entities
- **Structured Queries**: Cypher provides precise control over query patterns and result ordering
- **Persistent Storage**: Graph survives daemon restarts, unlike in-memory indexes

**Graph Query Execution Pattern:**
1. Manager.Search() coordinates multiple Cypher queries in parallel
2. Each query returns SearchResult structs with path, name, category, summary, score, match_type
3. Results are deduplicated by file path (same file may match multiple dimensions)
4. Optional category filtering applied after deduplication
5. Results limited to requested count

**Fallback Triggers:**
- FalkorDB container not running
- Network connectivity issues to graph database
- Graph manager not initialized
- Query execution errors

### Graceful Degradation

When graph-based search fails, the subsystem automatically falls back to in-memory token-based search to maintain availability.

**Fallback Search Characteristics:**
- **Token-Based Matching**: Splits query into tokens, filters stop words, matches substrings
- **Weighted Scoring**: Different fields contribute different score weights (filename highest at 3.0)
- **Proportional Scoring**: Partial matches receive fractional credit based on token match ratio
- **Stateless Operation**: No caching, deterministic results for same query
- **Read-Only**: Accesses FileIndex.Files slice without locking
- **Case-Insensitive**: All comparisons lowercased for user convenience

**When Fallback Occurs:**
- MCP server cannot reach daemon HTTP API (`internal/mcp/server.go:490`)
- Graph manager reports disconnected state
- Cypher query execution failures

**Source Attribution:**
- Graph results: `"source": "daemon"` in MCP responses
- Fallback results: `"source": "index"` in MCP responses

### Multi-Signal Search

Graph-based search combines results from multiple query types to provide comprehensive coverage.

**Search Signals Executed in Parallel** (`internal/graph/manager.go:559-614`):
1. **Filename Search**: Substring matching on `File.name` property
2. **Tag Search**: Traverse `HAS_TAG` edges to find files connected to matching tags
3. **Topic Search**: Traverse `COVERS_TOPIC` edges to find files covering matching topics
4. **Entity Search**: Traverse `MENTIONS` edges to find files mentioning matching entities

**Deduplication Strategy:**
- Maintains `map[string]bool` of seen file paths
- First occurrence of each path is kept
- Subsequent duplicates from other query types are discarded
- Preserves original score and match_type from first encounter

**Category Filtering:**
- Applied after deduplication to avoid filtering same file multiple times
- Case-insensitive matching on `Category` field
- Empty filter means no filtering (all categories included)

**Result Limiting:**
- Applied after deduplication and filtering
- Returns top N results based on query order (not sorted by score in graph mode)

### Relationship-Aware Queries

The graph enables discovery based on shared relationships, not just content matching.

**Related Files Query** (`internal/graph/queries.go:334-373`):
- Finds files sharing tags or topics with a given source file
- Calculates connection strength: `tagCount + topicCount`
- Returns connection type: `"shared_tags"` or `"shared_topics"`
- Orders results by strength descending
- Excludes the source file itself from results

**Entity-Based Connections:**
- Matches both exact entity names and normalized forms
- Normalized names handle case variations and whitespace
- Example: "FalkorDB" entity has normalized name "falkordb"
- Enables fuzzy matching on entity references

**Temporal Queries:**
- Recent files query uses `File.modified` property with time window
- Supports configurable lookback periods (default 7 days for MCP tool)
- Orders by modification time descending (newest first)

---

## Key Components

### Graph Manager Search

The Graph Manager (`internal/graph/manager.go`) provides the high-level search orchestration layer.

**Manager.Search() Method** (lines 559-614):
```go
func (m *Manager) Search(ctx, query string, limit int, categoryFilter string) ([]SearchResult, error)
```

**Execution Flow:**
1. Acquire read lock and verify connection state
2. Execute four Cypher queries in sequence:
   - `SearchByFilename(ctx, query, limit)`
   - `SearchByTag(ctx, query, limit)`
   - `SearchByTopic(ctx, query, limit)`
   - `SearchByEntity(ctx, query, limit)`
3. Aggregate all results into single slice
4. Deduplicate by file path using map
5. Apply optional category filter (case-insensitive)
6. Limit to requested result count
7. Return deduplicated, filtered results

**SearchResult Structure:**
- `Path` (string) - Full file path
- `Name` (string) - File basename
- `Type` (string) - File extension/type
- `Category` (string) - File category (documents, code, images, etc.)
- `Summary` (string) - AI-generated content summary
- `DocumentType` (string) - AI-classified type
- `Score` (float64) - Relevance score (currently unused in graph mode)
- `MatchType` (string) - "filename", "tag", "topic", or "entity"
- `Tags` ([]string) - Connected tag names
- `Topics` ([]string) - Connected topic names

**Thread Safety:**
- Uses read lock (`m.mu.RLock()`) for concurrent access
- Read-only operations on graph client
- No shared state mutation

### Cypher Query Layer

The Queries component (`internal/graph/queries.go`) encapsulates all Cypher query execution.

**Filename Search** (lines 116-150):
```cypher
MATCH (f:File)
WHERE toLower(f.name) CONTAINS toLower($pattern)
RETURN f.path, f.name, f.type, f.category, f.summary, f.document_type
ORDER BY f.modified DESC
LIMIT $limit
```

**Tag Search** (lines 152-184):
```cypher
MATCH (f:File)-[:HAS_TAG]->(t:Tag)
WHERE toLower(t.name) CONTAINS toLower($tagName)
RETURN DISTINCT f.path, f.name, f.type, f.category, f.summary, f.document_type, t.name
ORDER BY f.modified DESC
LIMIT $limit
```

**Topic Search** (lines 188-220):
```cypher
MATCH (f:File)-[:COVERS_TOPIC]->(t:Topic)
WHERE toLower(t.name) CONTAINS toLower($topicName)
RETURN DISTINCT f.path, f.name, f.type, f.category, f.summary, f.document_type, t.name
ORDER BY f.modified DESC
LIMIT $limit
```

**Entity Search** (lines 224-258):
```cypher
MATCH (f:File)-[:MENTIONS]->(e:Entity)
WHERE toLower(e.name) CONTAINS toLower($entityName)
   OR e.normalized CONTAINS toLower($entityName)
RETURN DISTINCT f.path, f.name, f.type, f.category, f.summary, f.document_type, e.name
ORDER BY f.modified DESC
LIMIT $limit
```

**Related Files** (lines 334-373):
```cypher
MATCH (f:File {path: $filePath})
OPTIONAL MATCH (f)-[:HAS_TAG]->(t:Tag)<-[:HAS_TAG]-(related:File)
WHERE f <> related
WITH related, count(t) as tagCount
OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(topic:Topic)<-[:COVERS_TOPIC]-(related2:File)
WHERE f <> related2
WITH COALESCE(related, related2) as rel, tagCount, count(topic) as topicCount
WHERE rel IS NOT NULL
RETURN rel.path, rel.name, rel.summary,
       tagCount + topicCount as strength,
       CASE WHEN tagCount > 0 THEN 'shared_tags' ELSE 'shared_topics' END as connection_type
ORDER BY strength DESC
LIMIT $limit
```

**Query Execution Pattern:**
1. Parameterized Cypher query constructed with placeholders ($param)
2. Client executes query via `c.client.Query(ctx, graphName, query).Params(params).Exec()`
3. Result parser extracts node properties and edge labels
4. Structs (SearchResult, RelatedFile, etc.) populated from result records
5. Errors propagated to caller for handling

### Fallback In-Memory Search

The Searcher component (`internal/search/semantic.go`) provides token-based search as a fallback.

**Searcher Structure:**
```go
type Searcher struct {
    index *types.FileIndex
}
```

**SearchQuery Parameters:**
```go
type SearchQuery struct {
    Query      string   // Search term (required)
    Categories []string // Category filter (optional, empty = all)
    MaxResults int      // Result limit (0 = unlimited)
}
```

**SearchResult Structure:**
```go
type SearchResult struct {
    File      types.FileEntry // Complete file entry with metadata and semantic fields
    Score     float64         // Cumulative relevance score
    MatchType string          // First matching field: "filename", "category", "file_type", "summary", "tag", "topic", "document_type"
}
```

**Query Processing Pipeline:**
1. Lowercase conversion: `strings.ToLower(query)`
2. Tokenization: `strings.Fields()` splits on whitespace
3. Punctuation removal: `strings.Trim(word, ".,!?;:\"'()[]{}")`
4. Stop word filtering: 26 common words removed (`a`, `an`, `and`, `are`, `as`, `at`, `be`, `by`, `for`, `from`, `has`, `he`, `in`, `is`, `it`, `its`, `of`, `on`, `that`, `the`, `to`, `was`, `will`, `with`)
5. Short token elimination: `len(word) > 1` (discards single characters)
6. Empty check: Returns zero results if no tokens remain

**Scoring Algorithm:**
- Formula: `(matched_tokens / total_tokens) × field_weight`
- Searches seven fields with different weights:
  - Filename (basename): 3.0
  - Category: 1.0
  - File Type (Type + extension): 0.5
  - Summary: 2.0
  - Tags (aggregated): 1.5
  - Topics (aggregated): 1.0
  - DocumentType: 0.5
- Minimum score threshold: 0.1 (filters weak matches)
- Results sorted by score descending

**Match Type Assignment:**
- Set to first field that matched during evaluation
- Evaluation order (not weight order):
  1. filename (3.0)
  2. category (1.0)
  3. file_type (0.5)
  4. summary (2.0)
  5. tag (1.5)
  6. topic (1.0)
  7. document_type (0.5)

**Data Structure:**
- Uses `types.FileIndex` with flattened `types.FileEntry`
- Semantic fields (Summary, Tags, Topics, DocumentType) are top-level properties, not nested
- Empty string checks for Summary and DocumentType
- Empty slice iteration for Tags and Topics (contributes 0 score if empty)

### HTTP API Integration

The daemon HTTP API (`internal/daemon/api/server.go`) exposes search endpoints.

**Search Endpoint** (lines 279-319):
- `POST /api/v1/search`
- Request: `{"query": string, "limit": int, "category": string}`
- Calls: `graphManager.Search(ctx, query, limit, category)`
- Response: `{"results": []SearchResult, "count": int}`

**Entity Search Endpoint** (lines 455-494):
- `GET /api/v1/entities/search?entity=<name>&limit=<n>`
- Calls: `graphManager.Queries().SearchByEntity(ctx, entity, limit)`
- Response: `{"entity": string, "files": []SearchResult, "count": int}`

**Recent Files Endpoint** (lines 375-414):
- `GET /api/v1/files/recent?days=<n>&limit=<n>`
- Calls: `graphManager.GetRecentFiles(ctx, days, limit)`
- Response: `{"files": []FileEntry, "count": int}`

**Related Files Endpoint** (lines 416-453):
- `GET /api/v1/files/related?path=<path>&limit=<n>`
- Calls: `graphManager.GetRelatedFiles(ctx, path, limit)`
- Response: `{"file": string, "related": []RelatedFile, "count": int}`

---

## Integration Points

### Daemon HTTP API

The daemon provides RESTful endpoints for all search operations.

**Integration Pattern:**
- HTTP server listens on `daemon.http_port` (default 8765)
- Graph manager injected as dependency at server construction
- All search endpoints delegate to graph manager methods
- Errors propagated as HTTP 500 with JSON error messages

**Client Usage:**
```go
resp, err := http.Post("http://localhost:8765/api/v1/search",
    "application/json",
    bytes.NewBuffer(jsonBody))
```

**Error Handling:**
- Graph connection errors: HTTP 500 "graph manager not connected"
- Query execution errors: HTTP 500 with FalkorDB error message
- Invalid requests: HTTP 400 with validation error

### MCP Server

The MCP server uses dual-mode search with daemon API priority.

**Tool: search_files** (`internal/mcp/server.go:420-527`):

**Primary Flow (Graph-Based):**
1. Check daemon API availability via `s.hasDaemonAPI()`
2. Construct POST request to `/api/v1/search`
3. Include query, limit, and category filter
4. Parse response and format for MCP
5. Return results with `"source": "daemon"`

**Fallback Flow (In-Memory):**
1. Triggered on daemon API error (line 490)
2. Log fallback: `"daemon search failed, falling back to index"`
3. Create Searcher: `search.NewSearcher(s.index)`
4. Execute search with same parameters
5. Format results with additional fields (size_human, modified)
6. Return results with `"source": "index"`

**Tool: get_related_files** (`internal/mcp/server.go:718-765`):
- Requires daemon API (no fallback)
- Calls `GET /api/v1/files/related?path=<path>&limit=<limit>`
- Returns error if daemon unavailable

**Tool: search_entities** (`internal/mcp/server.go:783-827`):
- Requires daemon API (no fallback)
- Calls `GET /api/v1/entities/search?entity=<entity>&limit=<limit>`
- Returns error if daemon unavailable

**Tool: list_recent_files** (`internal/mcp/server.go:607-658`):
- Requires daemon API (no fallback)
- Calls `GET /api/v1/files/recent?days=<days>&limit=<limit>`
- Returns error if daemon unavailable

**Default Parameters:**
- search_files: max_results=10
- list_recent_files: days=7, limit=20
- get_related_files: limit=10
- search_entities: limit=10

### FalkorDB Graph Database

The subsystem depends on FalkorDB for graph-powered search.

**Connection Management:**
- Graph manager maintains connection pool to FalkorDB
- Health checks verify connectivity before queries
- Graceful degradation when FalkorDB unavailable

**Graph Schema Dependencies:**
- Node types: File, Tag, Topic, Entity, Category
- Edge types: HAS_TAG, COVERS_TOPIC, MENTIONS, IN_CATEGORY
- Node properties: path, name, type, category, summary, document_type, modified
- Entity properties: name, normalized (for fuzzy matching)

**Index Requirements:**
- Vector index on File.embedding (for vector search, optional)
- Full-text index on File.summary (for full-text search, optional)
- Property indexes on File.path (for lookups)

**Query Execution:**
- Uses FalkorDB Go client: `github.com/FalkorDB/falkordb-go`
- Parameterized queries prevent injection
- Result parser handles FalkorDB-specific types

---

## Glossary

**Category Filtering**
The ability to restrict searches to specific file types. Categories include documents, code, images, audio, video, presentations, archives, data, and other. Applied after deduplication in graph mode, before scoring in fallback mode.

**Connection Strength**
Numeric value indicating how strongly two files are related based on shared graph relationships. Calculated as `tagCount + topicCount` where each count represents the number of shared tags or topics between files.

**Cypher Query**
Graph query language used by FalkorDB to traverse nodes and edges. Supports pattern matching, filtering, aggregation, and ordering. All graph-based searches execute Cypher queries.

**Deduplication**
The process of removing duplicate file paths from aggregated search results. Graph-based search executes multiple queries (filename, tag, topic, entity) and deduplicates by path to avoid showing the same file multiple times.

**Fallback Search**
In-memory token-based search that activates when graph-based search fails. Uses weighted scoring across seven fields without relationship traversal. Provides availability when FalkorDB is offline.

**Graph-Powered Search**
Primary search implementation using FalkorDB Cypher queries. Leverages graph structure to find files through relationship traversal (HAS_TAG, COVERS_TOPIC, MENTIONS edges). Context-aware and relationship-aware.

**Match Type**
Indicator of which search dimension produced a result. Graph mode values: "filename", "tag", "topic", "entity". Fallback mode values: "filename", "category", "file_type", "summary", "tag", "topic", "document_type".

**Multi-Signal Search**
Search approach that combines results from multiple query types (filename, tag, topic, entity) to provide comprehensive coverage. Executes queries in parallel and aggregates results with deduplication.

**Normalized Entity Name**
Lowercase version of entity name with whitespace normalized for fuzzy matching. Example: entity "FalkorDB" has normalized name "falkordb". Enables case-insensitive entity searches.

**Proportional Scoring**
Fallback mode scoring where field contribution is proportional to fraction of query tokens matched. Formula: `(matched_tokens / total_tokens) × field_weight`. Rewards partial matches with fractional credit.

**Relationship Traversal**
Graph query pattern that follows edges between nodes to discover connected files. Example: finding files sharing tags requires traversing File → HAS_TAG → Tag ← HAS_TAG ← File.

**SearchResult**
Data structure representing a search match. Contains file metadata (path, name, category, summary), relevance score, match type, and connected entities (tags, topics). Format differs between graph mode and fallback mode.

**Stop Words**
Common words with little semantic value filtered from queries during tokenization. List includes 26 words: "a", "an", "and", "are", "as", "at", "be", "by", "for", "from", "has", "he", "in", "is", "it", "its", "of", "on", "that", "the", "to", "was", "will", "with".

**Token-Based Matching**
Fallback search approach where queries are split into individual tokens (words) matched independently via substring matching. Each token can match anywhere within target field text using `strings.Contains()`.

**Weighted Scoring**
Fallback mode scoring where different fields contribute different point values. Filename (3.0) receives highest weight, summary (2.0) is second, tags (1.5), category/topic (1.0), file_type/document_type (0.5). Total base weight: 9.5 points.

---

## Additional Resources

For detailed implementation information, see:

**Graph-Based Search:**
- Graph manager: `internal/graph/manager.go` (lines 559-614 for Search method)
- Cypher queries: `internal/graph/queries.go` (search methods throughout)
- HTTP API: `internal/daemon/api/server.go` (search endpoints)

**Fallback In-Memory Search:**
- Token search: `internal/search/semantic.go` (full implementation)
- Test suite: `internal/search/semantic_test.go`

**Integration:**
- MCP tools: `internal/mcp/server.go` (handleSearchFiles, handleGetRelatedFiles, handleSearchEntities)
- Type definitions: `pkg/types/types.go` (FileIndex, FileEntry, SearchResult)

**Related Subsystems:**
- Graph subsystem: `docs/subsystems/falkordb-graph/README.md`
- MCP subsystem: `docs/subsystems/mcp/README.md`
- Daemon API: `docs/subsystems/daemon/README.md`
