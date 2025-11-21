# Semantic Search Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
   - [Purpose](#purpose)
   - [Role in the System](#role-in-the-system)
   - [Key Features](#key-features)
2. [Design Principles](#design-principles)
   - [Weighted Scoring Strategy](#weighted-scoring-strategy)
   - [Stateless Operation](#stateless-operation)
   - [Case-Insensitive Matching](#case-insensitive-matching)
   - [Pure Function Design](#pure-function-design)
3. [Key Components](#key-components)
   - [Searcher](#searcher)
   - [SearchQuery](#searchquery)
   - [Query Processing](#query-processing)
   - [SearchResult](#searchresult)
   - [Scoring Algorithm](#scoring-algorithm)
4. [Integration Points](#integration-points)
   - [Index System](#index-system)
   - [MCP Server](#mcp-server)
   - [Data Dependencies](#data-dependencies)
5. [Glossary](#glossary)
6. [Additional Resources](#additional-resources)

---

## Overview

### Purpose

The Semantic Search subsystem provides weighted, relevance-based search capabilities across the precomputed file index using token-based matching. It enables AI assistants and users to query the knowledge base dynamically during active sessions, finding relevant files based on semantic understanding rather than just filename patterns.

The subsystem searches across seven searchable fields:
- Filenames (basename only)
- File categories (documents, code, images, presentations, etc.)
- File types (extensions and type field)
- AI-generated content summaries
- Semantic tags
- Key topics
- Document type classifications

Results are ranked by relevance using a token-based proportional weighted scoring algorithm with stop word filtering, ensuring the most pertinent files surface first.

### Role in the System

The Semantic Search subsystem transforms the precomputed index from a passive context dump into an actively queryable knowledge base. While the index subsystem maintains the file metadata and the semantic analyzer generates the understanding, this subsystem enables runtime discovery and retrieval.

Key responsibilities:
- **Query Processing**: Accept search queries with optional category filters and result limits
- **Relevance Scoring**: Apply weighted scoring across multiple semantic fields
- **Result Ranking**: Sort matches by cumulative relevance score
- **Category Filtering**: Narrow searches to specific file types (documents, code, images)
- **Graceful Degradation**: Handle entries without semantic analysis by falling back to filename matching

The subsystem serves as the primary search engine for the MCP server's `search_files` tool, enabling AI assistants to dynamically discover relevant context during conversations.

### Key Features

**Multi-Dimensional Search**
- Searches across seven weighted fields simultaneously using token-based matching
- Cumulative scoring rewards multi-dimensional matches with proportional scoring
- Single query can match filenames, categories, file types, content summaries, tags, topics, and document types
- Stop word filtering removes non-meaningful terms (the, and, a, etc.)

**Relevance-Based Ranking**
- Proportional weighted algorithm prioritizes filename matches (weight 3.0)
- Content summaries provide strong signals (weight 2.0)
- Metadata tags supplement relevance (weight 1.5)
- Categories and topics provide thematic context (weight 1.0 each)
- File types and document classifications provide categorical context (weight 0.5 each)
- Scoring formula: (matched_tokens / total_tokens) × field_weight

**Flexible Filtering**
- Optional category filtering (documents, code, images, etc.)
- Configurable result limits (default 10 for MCP tool)
- Case-insensitive matching for user convenience

**Thread-Safe Operation**
- Stateless design enables concurrent searches
- No internal caching or shared state
- Read-only access to index

---

## Design Principles

### Weighted Scoring Strategy

The subsystem uses a token-based proportional weighting system that reflects how users typically search for files. Each searchable field contributes a score proportional to the fraction of query tokens matched:

**Scoring Formula**: `(matched_tokens / total_tokens) × field_weight`

This enables partial matching where files containing some but not all query terms still rank by relevance.

**Filename Priority** (weight: 3.0)
- Highest weight reflects that users often search by remembered file names
- Matches against basename only (ignores directory path)
- Strong signal for direct file lookup
- Example: Query `"terraform guide"` (2 tokens) matches filename `terraform-aws-guide.md` (2/2 tokens) → **3.0 points**

**Summary Relevance** (weight: 2.0)
- AI-generated summaries capture high-level content understanding
- Provides content-level relevance without requiring full-text indexing
- Middle-tier weight balances precision and recall
- Example: Query `"terraform deployment"` (2 tokens) matches summary mentioning both → 2/2 × 2.0 = **2.0 points**

**Tag Matching** (weight: 1.5)
- Semantic tags provide keyword-level metadata
- Accumulates across ALL tags (not counted once) for comprehensive coverage
- Supplementary signal to filename and summary
- Example: Query `"terraform aws"` (2 tokens), tags `["terraform", "infrastructure", "aws"]` → 2/2 × 1.5 = **1.5 points**

**Category Matching** (weight: 1.0)
- File category classification (`documents`, `code`, `images`, `presentations`, etc.)
- Enables filtering by file type
- Example: Query `"presentation slides"` (2 tokens), category `presentations` → 1/2 × 1.0 = **0.5 points**

**Topic Relevance** (weight: 1.0)
- Key topics represent thematic areas
- Accumulates across ALL topics (not counted once) for comprehensive coverage
- Broader than tags but more specific than document type
- Example: Query `"deployment automation"` (2 tokens), topics include both terms across multiple topics → 2/2 × 1.0 = **1.0 point**

**File Type** (weight: 0.5)
- File extension and type field (`md`, `pdf`, `pptx`, etc.)
- Enables searching by file format
- Checks both Type field and extracted extension
- Example: Query `"powerpoint pptx"` (2 tokens), type `pptx` → 1/2 × 0.5 = **0.25 points**

**Document Type** (weight: 0.5)
- AI-classified categorical type (e.g., `terraform-configuration`, `technical-guide`)
- Lowest weight reflects broad categorization
- Provides context but least precise match signal
- Example: Query `"technical guide"` (2 tokens), document type `technical-guide` → 2/2 × 0.5 = **0.5 points**

**Cumulative Design Rationale**: Files matching in multiple dimensions receive higher scores, reflecting that multi-dimensional matches indicate stronger relevance. Proportional scoring ensures that:
- Complete matches (all tokens found) receive full field weight
- Partial matches (some tokens found) receive proportional credit
- Files with zero matches in a field contribute 0 to that field's score
- Total base weight across all fields: 9.5 points (3.0 + 2.0 + 1.5 + 1.0 + 1.0 + 0.5 + 0.5)
- Maximum achievable score: 9.5 points (if all query tokens match in all fields)
- Minimum included score: 0.1 points (threshold filters out weak matches)

**Example**: Single-token query `"terraform"` matching filename, category, summary, all tags, all topics, file type, and document type achieves the maximum 9.5 points (1/1 × 9.5).

### Stateless Operation

The Searcher maintains no internal state between queries:

**No Caching**: Each search operation is independent, avoiding cache invalidation complexity and stale data issues.

**No Locks Required**: Read-only access to the index eliminates synchronization overhead.

**Thread-Safe by Design**: Multiple goroutines can create separate Searcher instances or share a single instance safely.

**Deterministic Results**: Same query + same index always produces identical results (except for equal-score tie-breaking, which is non-deterministic).

This stateless design simplifies testing, eliminates side effects, and enables straightforward concurrency.

### Case-Insensitive Matching

All text comparisons use lowercase transformation for both query and target fields:

**User Experience**: Users don't need to remember exact capitalization of filenames, tags, or content.

**Cross-Convention Compatibility**: Works across camelCase, snake_case, kebab-case, and other naming conventions.

**Semantic Focus**: Case-insensitivity aligns with semantic search goals—content meaning matters, not formatting.

**Consistent Behavior**: Applied uniformly to queries, filenames, summaries, tags, topics, document types, and category filters.

### Pure Function Design

Search operations are pure transformations with no side effects:

**Input**: SearchQuery (query string, categories, max results) + Index (read-only)

**Output**: SearchResult array (entries, scores, match types)

**No Side Effects**: No logging, no metrics, no I/O, no state mutation

**Deterministic**: Same inputs always produce same outputs (modulo non-stable sorting of equal scores)

This functional purity enables:
- Simple unit testing without mocks
- Easy reasoning about behavior
- Composability with other pure functions
- Predictable performance characteristics

---

## Key Components

### Searcher

The Searcher is the primary interface for search operations:

**Structure**: Wraps a pointer to the precomputed index

**Initialization**: Created via `NewSearcher(index *types.Index)` constructor

**Methods**:
- `Search(query SearchQuery) []SearchResult` - Execute search and return ranked results

**Characteristics**:
- No internal state beyond the index reference
- Thread-safe for concurrent use
- Read-only access to index
- Can be instantiated once and reused or created per-query

**Lifecycle**: Typically created when needed (e.g., in MCP tool handler) and discarded after use, though reuse is safe.

### SearchQuery

The SearchQuery encapsulates all search parameters:

**Fields**:
- `Query` (string) - Search term, required, undergoes tokenization and stop word filtering
- `Categories` ([]string) - Optional category filter, empty means all categories
- `MaxResults` (int) - Result limit, 0 means unlimited

**Query Processing**:
- Query string is tokenized (split on whitespace, lowercase, punctuation removed)
- Stop words filtered (26 common words like "the", "and", "a")
- Tokens shorter than 2 characters discarded
- If no tokens remain after processing, returns zero results immediately
- See Query Processing section for detailed algorithm

**Validation**:
- Empty query strings return zero results immediately
- Queries with only stop words return zero results (no meaningful tokens)
- Category matching is case-insensitive
- MaxResults applied after sorting (top-N selection)

**Usage Pattern**:
```go
results := searcher.Search(SearchQuery{
    Query:      "terraform",
    Categories: []string{"documents"},
    MaxResults: 10,
})
```

**Defaults**: MCP tool handler applies default MaxResults=10 if not specified by client.

### Query Processing

Before search execution, queries undergo multi-step processing to extract meaningful tokens:

**Step 1: Case Normalization**
- Convert entire query string to lowercase
- Ensures case-insensitive matching throughout

**Step 2: Tokenization**
- Split on whitespace into individual words
- Each word becomes a candidate token

**Step 3: Stop Word Filtering**
- Remove common non-meaningful words that don't contribute to relevance
- Stop word list (26 words): `a`, `an`, `and`, `are`, `as`, `at`, `be`, `by`, `for`, `from`, `has`, `he`, `in`, `is`, `it`, `its`, `of`, `on`, `that`, `the`, `to`, `was`, `will`, `with`
- Example: `"Find the Terraform guide"` → `["Find", "Terraform", "guide"]`

**Step 4: Punctuation Removal**
- Trim punctuation from token boundaries
- Removed characters: `. , ! ? ; : " ' ( ) [ ] { }`
- Example: `"Terraform,"` → `"Terraform"`

**Step 5: Short Token Filtering**
- Discard tokens shorter than 2 characters
- Prevents matching on articles and single-character noise
- Example: `["a", "go", "guide"]` → `["go", "guide"]`

**Step 6: Empty Result Handling**
- If no tokens remain after filtering, return empty results immediately
- Prevents scoring overhead for meaningless queries
- Example: `"the and a"` → 0 tokens → 0 results

**Processing Examples**:

| Original Query | Tokens After Processing | Note |
|----------------|-------------------------|------|
| `"terraform"` | `["terraform"]` | Single token, no filtering |
| `"Terraform Guide"` | `["terraform", "guide"]` | Case normalized |
| `"Find the Terraform deployment guide"` | `["find", "terraform", "deployment", "guide"]` | Stop words removed |
| `"AWS, Azure, and GCP"` | `["aws", "azure", "gcp"]` | Punctuation + stop words |
| `"the a an"` | `[]` | All stop words → 0 results |
| `"a b c"` | `[]` | All short tokens → 0 results |

**Implementation Detail**: Tokenization occurs once per query in `Search()` function before scoring begins. The token list is then passed to `scoreEntry()` for each index entry.

### SearchResult

SearchResult represents a single match with relevance metadata:

**Fields**:
- `Entry` (types.IndexEntry) - Complete index entry including metadata and semantic analysis
- `Score` (float64) - Cumulative relevance score from token-based proportional weighted algorithm
- `MatchType` (string) - Primary match type: "filename", "summary", "tag", "category", "topic", "file_type", or "document_type"

**Match Type Logic**: Records the highest-weighted field that matched, used for display and tie-breaking. Priority order:
1. "filename" (weight 3.0) - highest priority
2. "summary" (weight 2.0)
3. "tag" (weight 1.5)
4. "category" (weight 1.0)
5. "topic" (weight 1.0)
6. "file_type" (weight 0.5)
7. "document_type" (weight 0.5) - lowest priority

**Score Characteristics**:
- Minimum threshold: 0.1 (entries scoring below are filtered out)
- Maximum theoretical: 9.5 points (single-token query matching all fields perfectly)
- Typical range: 0.5 to 7.0 points for multi-token natural language queries
- Proportional scoring means partial matches receive fractional credit based on `(matched_tokens / total_tokens)` ratio

**Examples**:
- Single token `"terraform"` matching filename only: 3.0 points
- Two tokens `"terraform guide"` with 2/2 filename match + 1/2 summary match: 3.0 + 1.0 = 4.0 points
- Three tokens `"NIH terraform workshop"` matching 3/3 in filename, 3/3 in summary, 2/3 in tags: 3.0 + 2.0 + 1.0 = 6.0 points

**Usage**: Results are sorted by Score descending before being returned to the caller. Minimum score threshold prevents returning weakly related files.

### Scoring Algorithm

The scoring algorithm uses token-based matching with proportional scoring for relevance ranking:

**Query Tokenization** (preprocessing)
1. Convert query to lowercase
2. Split on whitespace into individual words
3. Filter common stop words (`a`, `an`, `and`, `the`, `is`, `for`, `with`, etc. - 26 total)
4. Remove punctuation from word boundaries (`.`, `,`, `!`, `?`, `;`, `:`, `"`, `'`, `()`, `[]`, `{}`)
5. Discard tokens shorter than 2 characters
6. If no tokens remain, return empty results

Example: `"Find the Terraform guide"` → `["find", "terraform", "guide"]` (3 tokens)

**Token Matching Helper**
- `countMatches(text, tokens)`: Counts how many query tokens appear as substrings in the text
- Case-insensitive matching via `strings.ToLower()`
- Each token can match only once per field

**Phase 1: Filename Matching** (weight: 3.0)
- Extract basename from full path
- Count query tokens present in filename
- Score: `(matched_tokens / total_tokens) × 3.0`
- Set primary match type to "filename" if matched

Example: Filename `terraform-guide.md`, Query `["terraform", "guide"]` → 2/2 tokens × 3.0 = **3.0 points**

**Phase 2: Category Matching** (weight: 1.0)
- Count query tokens present in category field (`documents`, `code`, `images`, `presentations`, etc.)
- Score: `(matched_tokens / total_tokens) × 1.0`
- Set match type to "category" if no higher-weighted match exists

Example: Category `presentations`, Query `["presentation", "slides"]` → 1/2 tokens × 1.0 = **0.5 points**

**Phase 3: File Type Matching** (weight: 0.5)
- Check both Type field and file extension
- Count query tokens present in either
- Score: `(matched_tokens / total_tokens) × 0.5`
- Set match type to "file_type" if no higher-weighted match exists

Example: Type `pptx`, Query `["powerpoint", "pptx"]` → 1/2 tokens × 0.5 = **0.25 points**

**Phase 4: Semantic Analysis Check**
- If `entry.Semantic` is nil, return current score and match type
- Files without semantic analysis can only score on filename, category, and file type (max 4.5 points with perfect matches)

**Phase 5: Summary Matching** (weight: 2.0)
- Count query tokens present in semantic summary
- Score: `(matched_tokens / total_tokens) × 2.0`
- Set match type to "summary" if no higher-weighted match exists

Example: Summary contains "Terraform infrastructure guide", Query `["terraform", "guide"]` → 2/2 tokens × 2.0 = **2.0 points**

**Phase 6: Tag Matching** (weight: 1.5)
- Aggregate token matches across ALL semantic tags
- Count total tokens found in any tag
- Score: `(matched_tokens / total_tokens) × 1.5`
- Set match type to "tag" if no higher-weighted match exists

Example: Tags `["terraform", "infrastructure", "aws"]`, Query `["terraform", "aws"]` → 2/2 tokens × 1.5 = **1.5 points**

**Phase 7: Topic Matching** (weight: 1.0)
- Aggregate token matches across ALL key topics
- Count total tokens found in any topic
- Score: `(matched_tokens / total_tokens) × 1.0`
- Set match type to "topic" if no higher-weighted match exists

Example: Topics contain "Terraform deployment" and "Infrastructure automation", Query `["terraform", "deployment"]` → 2/2 tokens × 1.0 = **1.0 point**

**Phase 8: Document Type Matching** (weight: 0.5)
- Count query tokens present in document type field
- Score: `(matched_tokens / total_tokens) × 0.5`
- Set match type to "document_type" if no higher-weighted match exists

Example: Document type `technical-guide`, Query `["technical", "guide"]` → 2/2 tokens × 0.5 = **0.5 points**

**Cumulative Scoring**: Scores from all matching phases are summed. Entries with total score ≤ 0.1 are filtered out as not relevant enough.

**Match Type Priority**: The primary match type is the highest-weighted field that matched, used for display and sorting tie-breaking:
1. filename (3.0)
2. summary (2.0)
3. tag (1.5)
4. category (1.0)
5. topic (1.0)
6. file_type (0.5)
7. document_type (0.5)

**Result**: Return cumulative score and primary match type

**Example Full Calculation**:
- File: `terraform-aws-guide.pdf` with summary "Guide to Terraform on AWS", tags `["terraform", "aws", "infrastructure"]`, category `documents`
- Query: `"terraform aws guide"` → Tokens: `["terraform", "aws", "guide"]` (3 tokens)
- Filename: 2/3 tokens (`terraform`, `aws`) × 3.0 = 2.0
- Category: 0/3 tokens × 1.0 = 0.0
- Summary: 3/3 tokens × 2.0 = 2.0
- Tags: 2/3 tokens × 1.5 = 1.0
- **Total Score: 5.0**, Match Type: `filename` (highest weighted match)

---

## Integration Points

### Index System

The Semantic Search subsystem integrates with the Index Manager as a read-only consumer:

**Integration Pattern**:
- Searcher receives `*types.Index` at construction
- Accesses `index.Entries` slice directly
- No write operations or locking required
- Compatible with existing index schema

**Data Flow**:
1. Index Manager loads precomputed index from disk
2. Index reference passed to Searcher constructor
3. Searcher iterates entries during search
4. Results reference original index entries (no copying)

**Schema Compatibility**: Uses standard `types.Index` and `types.IndexEntry` structures with no search-specific extensions.

**Graceful Degradation**: If `entry.Semantic` field is nil (analysis failed or skipped), searcher gracefully falls back to filename-only matching.

### MCP Server

The MCP server is the primary consumer of the Semantic Search subsystem:

**Tool Implementation**: The `search_files` tool handler in `internal/mcp/server.go` uses the searcher.

**Integration Flow**:
1. MCP client sends `tools/call` request with tool name "search_files"
2. Server unmarshals parameters (query, categories, max_results)
3. Server validates query is non-empty
4. Server creates Searcher with its index reference
5. Server executes search with query parameters
6. Server formats results as JSON with path, name, category, score, match_type, size, modified time, summary, tags
7. Server returns ToolsCallResponse with formatted results

**Error Handling**: Invalid arguments (empty query, malformed JSON) return tool-level errors with `isError: true` flag, allowing the MCP protocol to continue functioning.

**Default Parameters**: MCP handler applies MaxResults=10 if client doesn't specify a limit.

**Result Formatting**: Handler adds human-readable fields like `size_human` and formats timestamps as RFC3339 for client display.

### Data Dependencies

The subsystem depends on type definitions from the `pkg/types` package:

**types.Index**:
- `Entries` ([]IndexEntry) - Array of all indexed files

**types.IndexEntry**:
- `Metadata` (FileMetadata) - File information including path and category
- `Semantic` (*SemanticAnalysis) - Optional semantic understanding, nil if analysis failed

**types.FileMetadata**:
- `Path` (string) - Full file path, basename used for filename matching
- `Category` (string) - File category (documents, code, images, etc.)
- `Size` (int64) - File size in bytes
- `Modified` (time.Time) - Last modification timestamp

**types.SemanticAnalysis**:
- `Summary` (string) - AI-generated content summary
- `Tags` ([]string) - Array of semantic keywords
- `KeyTopics` ([]string) - Array of key subject areas
- `DocumentType` (string) - AI-classified type (e.g., "terraform-configuration")

**No External Dependencies**: The subsystem requires only standard library packages (`path/filepath`, `sort`, `strings`) beyond the types package.

---

## Glossary

**Basename**
The filename component of a path without directory information. For example, the basename of `/docs/terraform-guide.md` is `terraform-guide.md`. The scoring algorithm matches queries against basenames to avoid directory path noise.

**Case-Insensitive Matching**
Text comparison that ignores uppercase/lowercase distinctions. Implemented by converting both query and target strings to lowercase before comparison. Ensures "Terraform", "terraform", and "TERRAFORM" all match each other.

**Category Filtering**
The ability to restrict searches to specific file types. Categories include documents, code, images, audio, video, presentations, archives, data, and other. Implemented as an OR filter—results must match at least one requested category.

**Cumulative Scoring**
A scoring approach where points from multiple matching fields are added together. A file matching in filename (3.0), summary (2.0), and tags (1.5) receives a cumulative score of 6.5 points. Reflects that multi-dimensional matches indicate stronger relevance.

**Graceful Degradation**
The ability to continue functioning with reduced capability when data is incomplete. If semantic analysis is missing (Semantic field is nil), the searcher still functions by scoring based on filename alone.

**Match Type**
An indicator of which field produced the highest-weighted match for a result. Possible values: "filename", "category", "file_type", "summary", "tag", "topic", "document_type". Helps explain why a result appeared in search results.

**Pure Function**
A function that always produces the same output for the same input and has no side effects (no state mutation, no I/O, no logging). The search operation is a pure function—deterministic transformation of query + index into results.

**Proportional Scoring**
A scoring approach where the score contribution from a field is proportional to the fraction of query tokens matched. Formula: `(matched_tokens / total_tokens) × field_weight`. If 3 out of 4 query tokens match in filename (weight 3.0), the contribution is (3/4) × 3.0 = 2.25 points. Rewards partial matches while giving higher scores to more complete matches.

**Query Tokenization**
The process of converting a natural language query into individual searchable tokens. Steps: (1) lowercase conversion, (2) whitespace splitting, (3) punctuation removal, (4) stop word filtering, (5) short token elimination (≤1 character). Query "Find the Terraform docs" becomes tokens ["find", "terraform", "docs"].

**Relevance Score**
A numeric value (minimum 0.1) indicating how well a file matches a search query. Higher scores indicate stronger relevance. Calculated using proportional scoring across all matching fields. Maximum theoretical score is 9.5 (sum of all field weights), but scores can exceed this when multiple tags or topics match. Only results scoring above 0.1 threshold are returned.

**Semantic Fields**
Non-filename attributes that capture the meaning and content of files: summaries (AI-generated descriptions), tags (semantic keywords), topics (key subject areas), and document types (AI classifications). Enable content-based search beyond filename patterns.

**Stateless Operation**
An operational pattern where components maintain no internal state between invocations. Each search is independent of previous searches. Simplifies concurrency, testing, and reasoning about behavior.

**Stop Words**
Common words with little semantic value that are filtered from queries during tokenization. The list includes 26 words: "a", "an", "and", "are", "as", "at", "be", "by", "for", "from", "has", "he", "in", "is", "it", "its", "of", "on", "that", "the", "to", "was", "will", "with". Removing stop words improves search precision by focusing on meaningful terms.

**Substring Matching**
A text comparison where a search term can appear anywhere within the target string. Used as the matching mechanism for individual tokens within the token-based search algorithm. "terra" matches "Terraform", "subterranean", and "Mediterranean". Implemented using `strings.Contains()`. Each token from the query is checked for substring matches independently.

**Token-Based Matching**
A search approach where queries are split into individual tokens (words) that are matched independently. Query "terraform azure" becomes tokens ["terraform", "azure"], each checked separately. Enables natural language queries, handles multi-word searches gracefully, and supports proportional scoring based on token match ratio. Replaces exact phrase matching with more flexible token-level matching.

**Weighted Scoring Algorithm**
A relevance ranking approach that assigns different point values to different types of matches. Filename matches receive higher weight (3.0) than summary matches (2.0), which receive higher weight than tag matches (1.5), and so on. Reflects the relative importance of different match signals.

---

## Additional Resources

For detailed implementation information, see:
- Implementation: `internal/search/semantic.go`
- Test suite: `internal/search/semantic_test.go`
- MCP integration: `internal/mcp/server.go` (handleSearchFiles)
- Type definitions: `pkg/types/types.go`
- MCP subsystem docs: `docs/subsystems/mcp/README.md`
