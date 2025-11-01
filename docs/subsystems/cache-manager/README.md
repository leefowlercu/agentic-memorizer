# Cache Manager Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Content-Addressable Storage](#content-addressable-storage)
   - [File-Based Persistence](#file-based-persistence)
   - [Cache-First Pattern](#cache-first-pattern)
   - [Separation of Concerns](#separation-of-concerns)
3. [Key Components](#key-components)
   - [Manager Component](#manager-component)
   - [Cache Operations](#cache-operations)
   - [Content Hashing](#content-hashing)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Worker Pool](#worker-pool)
   - [Semantic Analyzer](#semantic-analyzer)
   - [Type System](#type-system)
5. [Glossary](#glossary)

## Overview

The Cache Manager subsystem provides intelligent caching of semantic analysis results to avoid redundant API calls to Claude. It stores AI-generated file analyses keyed by content hash, enabling the system to reuse expensive semantic analysis results when files haven't changed. This dramatically reduces API costs (often achieving cache hit rates exceeding 95%), accelerates index builds, and improves overall system performance.

The subsystem uses a content-addressable storage strategy where each cached analysis is keyed by a SHA-256 hash of the file's content. This approach provides automatic cache invalidation when file content changes and enables cache hits even when files are renamed or moved. Cache entries are persisted as individual JSON files in a designated cache directory, providing durability across daemon restarts and human-readable storage for debugging purposes.

By wrapping the Semantic Analyzer with cache-first logic, the Cache Manager transforms expensive Claude API calls into fast local file reads for unchanged content. This optimization is critical for making the agentic-memorizer system practical for real-world use, where files are frequently re-indexed without content changes during development workflows, git operations, and daemon restarts.

## Design Principles

### Content-Addressable Storage

The Cache Manager uses content hashing as its fundamental addressing mechanism, where the storage location and retrieval key for cached data is derived from the content itself rather than arbitrary identifiers. This design provides several critical properties that enable robust, efficient caching.

**Content Hash as Cache Key:**
The subsystem computes a SHA-256 cryptographic hash of each file's complete content and uses this hash (prefixed with "sha256:") as the cache key. Because cryptographic hashes have negligible collision probability and are deterministic for identical content, this approach ensures that:
- Files with identical content share the same cache entry regardless of filename or location
- Content changes automatically invalidate the cache through hash mismatch
- Cache entries can be safely reused across file operations (rename, move, copy)

**Automatic Invalidation:**
Unlike time-based or manual invalidation strategies, content addressing provides automatic cache invalidation. When a file's content changes, its hash changes, resulting in a cache miss that triggers fresh analysis. Old cache entries become orphaned (no longer referenced by any current file hash) but don't cause correctness issues. This eliminates the need for complex invalidation logic, cache versioning, or explicit expiration management.

**Location Independence:**
Content addressing makes caching resilient to file system operations. A file can be renamed, moved to a different directory, or copied to multiple locations, and all instances will share the same cached analysis as long as content remains unchanged. This property is particularly valuable during git operations (branch switches, rebases) and refactoring activities that reorganize file structures.

### File-Based Persistence

The Cache Manager implements persistence using individual JSON files stored in a designated cache directory, rather than in-memory storage or database systems. This file-based approach provides several architectural benefits while maintaining simplicity and debuggability.

**Durability Across Restarts:**
Cache entries persist to disk immediately upon creation, surviving daemon restarts without requiring explicit serialization or backup mechanisms. When the daemon restarts, previously cached analyses remain available, avoiding redundant API calls during index rebuilds. This durability is particularly valuable during development, where daemons are frequently restarted.

**Human-Readable Format:**
Each cache entry is stored as pretty-printed JSON (with indentation), making cache contents directly inspectable by developers and users. This readability aids debugging, manual cache inspection, and understanding of cached analysis structure. The JSON format also provides forward compatibility as the schema can evolve while maintaining backwards compatibility through optional fields.

**No Memory Pressure:**
File-based storage avoids memory consumption that would grow with cache size. Large caches (thousands of files) don't impact daemon memory usage since entries are only loaded on cache hits. This design enables caching to scale with repository size without imposing memory constraints.

**Simple Implementation:**
File system operations (read, write, delete) map directly to cache operations without serialization complexity, transaction management, or connection pooling. The implementation uses standard Go file I/O with minimal code and no external dependencies beyond the standard library.

### Cache-First Pattern

The Cache Manager implements a cache-first processing pattern where expensive operations are attempted from cache before falling back to full computation. This pattern is consistently applied in both the worker pool (parallel processing) and daemon event handler (incremental updates).

**Processing Flow:**
1. Extract file metadata (fast, always performed)
2. Compute content hash (relatively fast, enables cache lookup)
3. Check cache for matching hash (fastest path)
4. On cache hit with valid hash: Use cached semantic analysis, skip API call entirely
5. On cache miss: Perform semantic analysis via Claude API, store result in cache
6. All subsequent files with identical content reuse the cached result

**Performance Impact:**
The cache-first pattern transforms the performance characteristics of file processing. Without caching, every file requires a Claude API call (network I/O, 1-5 seconds typical latency, API quota consumption, cost per call). With caching and typical hit rates exceeding 95%, most files complete processing in milliseconds through local file reads. This makes index rebuilds practical even for large repositories.

**Statistics Tracking:**
The worker pool maintains cache hit and API call counters, enabling visibility into cache effectiveness. These metrics are logged during processing and exposed through health endpoints, allowing users to understand cache performance and identify scenarios where hit rates are lower than expected.

### Separation of Concerns

The Cache Manager maintains clear boundaries with other subsystems, focusing exclusively on storage and retrieval of cached analyses without entanglement in semantic analysis logic, daemon lifecycle management, or worker coordination.

**Single Responsibility:**
The manager handles only cache operations: storing analyses to disk, retrieving analyses by hash, checking staleness, and clearing cache. It doesn't perform semantic analysis, coordinate workers, or manage daemon state. This focused responsibility makes the component simple to understand, test, and maintain.

**Stateless Operations:**
Each cache operation (Get, Set) is independent and stateless, enabling concurrent access from multiple workers without coordination overhead. The manager holds only configuration (cache directory path) without maintaining runtime state like LRU lists, hit counters, or locks. This stateless design simplifies the implementation and avoids concurrency bugs.

**Clean Integration Boundaries:**
The Semantic Analyzer has no knowledge of caching - it simply performs analysis when called. The Cache Manager wraps the analyzer without modifying its behavior. The daemon and worker pool implement the integration logic, deciding when to check cache versus invoke analysis. This separation allows each component to evolve independently.

## Key Components

### Manager Component

The Manager struct (`internal/cache/manager.go`) provides the central interface for cache operations, maintaining configuration state and implementing storage/retrieval logic.

**Core State:**
- `cacheDir string` - Base directory path for cache storage (typically `~/.agentic-memorizer/.cache`)

The manager maintains minimal state, holding only the cache directory path. All other information (cached analyses, metadata, timestamps) is stored in individual JSON files within the cache directory structure.

**Initialization:**
The `NewManager()` constructor accepts a cache directory path, validates it to prevent security issues (rejects parent directory references like `..`), expands home directory notation (`~`), and creates the `summaries` subdirectory structure. This initialization ensures the cache directory is ready for storage operations before any caching occurs.

**Thread Safety:**
The current implementation provides implicit thread safety through file system atomicity. Individual file writes are atomic at the OS level, and concurrent reads don't conflict. However, the implementation lacks explicit locking, so future enhancements requiring coordinated operations (like cache size tracking or LRU eviction) would need to add synchronization primitives.

### Cache Operations

The Cache Manager provides five core operations that implement the complete cache lifecycle:

**Get Operation:**
The `Get(fileHash string)` method retrieves cached analysis for a given content hash. It constructs the cache file path from the hash (using the first 16 characters as the filename), attempts to read the JSON file, and unmarshals the content into a `CachedAnalysis` structure. On success, it returns the cached analysis. On failure (file not found, JSON parse error), it returns nil, indicating a cache miss. This simple API enables callers to use the pattern `if cached := manager.Get(hash); cached != nil { use cached }`.

**Set Operation:**
The `Set(cached *CachedAnalysis)` method stores a new cache entry. It marshals the `CachedAnalysis` structure to pretty-printed JSON and writes it to a file named after the first 16 characters of the file hash. The operation is synchronous, ensuring the cache entry is durable before returning. Errors during write operations are logged but don't prevent the caller from continuing (the analysis result is still valid, just not cached).

**Staleness Check:**
The `IsStale(cached *CachedAnalysis, currentHash string)` method determines whether a cached entry is still valid by comparing the cached file hash against the current file's content hash. If the hashes match, the cache is valid. If they differ, the content has changed, and the cache entry is stale. This simple comparison implements content-based cache invalidation without time-based expiration or version tracking.

**Clear Operation:**
The `Clear()` method removes all cached files by deleting the entire `summaries` subdirectory and recreating it empty. This operation provides manual cache maintenance capability, useful for testing, troubleshooting, or recovering from cache corruption. The operation is not typically needed during normal operation since content addressing prevents stale cache usage.

**Hash Computation:**
The `HashFile(filePath string)` function computes the SHA-256 hash of a file's content. It streams the entire file through the hasher without size limits, producing a deterministic hash value. The hash is returned in the format `"sha256:" + hex-encoded-hash`, matching the cache key format used throughout the system. This function is typically called by worker threads before cache lookup.

### Content Hashing

The content hashing mechanism provides the foundation for the cache's content-addressable storage strategy, implementing deterministic hash computation that enables reliable cache keying.

**Algorithm:**
The subsystem uses SHA-256, a cryptographic hash function that produces 256-bit (32-byte) hash values. SHA-256 provides collision resistance (probability of two different files producing the same hash is negligible), determinism (same content always produces same hash), and one-way properties (can't reverse hash to recover content).

**Hash Format:**
Computed hashes use the format `"sha256:<hex-encoded-hash>"` where the hex encoding produces a 64-character string. The `sha256:` prefix provides algorithm identification, enabling future support for alternative hash algorithms without breaking existing caches. This format is consistent across metadata structures, cache keys, and file names.

**Filename Convention:**
Cache files are named using the first 16 characters of the hex-encoded hash followed by `.json` extension. For example, a file with hash `sha256:abc123def456...` would have cache file `abc123def456.json`. This truncation provides manageable filename lengths while maintaining negligible collision probability (16 hex characters = 64 bits, 2^64 possible values).

**Streaming Computation:**
The hash computation streams file content through the hasher rather than loading the entire file into memory. This streaming approach enables hashing of arbitrarily large files without memory constraints. The hasher processes chunks as they're read from disk, accumulating the hash state incrementally.

**Integration with Cache Keys:**
Once computed, the file hash serves multiple purposes:
- Cache lookup key: Used in `Get()` to locate cached analysis
- Cache storage key: Used in `Set()` to determine where to store new entry
- Staleness check: Compared in `IsStale()` to validate cache currency
- Index entry field: Stored in `FileMetadata.Hash` for reference and debugging

## Integration Points

### Daemon Subsystem

The Daemon subsystem (`internal/daemon/daemon.go`) creates and manages the Cache Manager as an optional component that's only initialized when semantic analysis is enabled.

**Initialization:**
During daemon startup, if `cfg.Analysis.Enable` is true, the daemon creates a cache manager using `cache.NewManager(cfg.Analysis.CacheDir)`. The cache directory path comes from configuration (default: `~/.agentic-memorizer/.cache`). If semantic analysis is disabled, the cache manager is not created, and all caching logic is bypassed.

**Worker Pool Distribution:**
The daemon passes the cache manager instance to the worker pool during initialization. All worker threads share this single cache manager instance, enabling coordinated cache access across parallel processing. The cache manager's stateless design ensures this sharing is safe without explicit synchronization.

**Incremental Updates:**
When the File Watcher detects a file change, the daemon's event handler uses the cache manager directly. The handler extracts metadata, computes the file hash, checks the cache, and either uses cached results or performs fresh analysis. This integration ensures that incremental updates benefit from caching just like full index builds.

**Configuration Integration:**
The daemon respects the `analysis.cache_dir` configuration parameter, enabling users to customize cache location. The configuration system validates the path, expands home directory notation, and prevents security issues from path traversal. The daemon passes the validated path to the cache manager constructor.

### Worker Pool

The Worker Pool subsystem (`internal/daemon/worker_pool.go`) implements the cache-first processing pattern that wraps semantic analysis with cache lookup logic.

**Processing Pipeline:**
Each worker processes files through a multi-stage pipeline where caching is integrated after metadata extraction and hash computation but before semantic analysis:

1. **Metadata Extraction**: Worker calls metadata extractor to gather file metadata (fast operation)
2. **Hash Computation**: Worker calls `cache.HashFile()` to compute content hash (relatively fast)
3. **Cache Lookup**: Worker calls `cacheManager.Get(fileHash)` to check for cached analysis
4. **Cache Hit Path**: If cached analysis exists and isn't stale, worker uses it immediately and increments cache hit counter
5. **Cache Miss Path**: If no cached analysis or stale entry, worker waits for rate limiter token, calls semantic analyzer, stores result via `cacheManager.Set()`, and increments API call counter
6. **Index Update**: Worker adds entry to index regardless of cache hit/miss

**Statistics Tracking:**
The worker pool maintains counters for cache hits and API calls, enabling calculation of cache hit rate. These statistics are logged during processing and exposed through health metrics. High cache hit rates (>95%) indicate effective caching, while low rates may indicate frequent file changes or cache issues.

**Rate Limiting Integration:**
Rate limiting occurs only on the cache miss path. When cache provides results, no rate limiter token is consumed, allowing workers to process cached files at maximum speed without API quota constraints. This integration ensures that caching provides not just performance benefits but also quota preservation.

**Concurrency:**
Multiple workers access the cache manager concurrently during parallel processing. The cache manager's stateless design and file system atomicity ensure this concurrent access is safe. Each worker independently checks cache, performs analysis on misses, and stores results without coordination overhead.

### Semantic Analyzer

The Cache Manager wraps the Semantic Analyzer subsystem to reduce Claude API calls, implementing a transparent caching layer that the analyzer doesn't need to know about.

**Wrapper Pattern:**
The integration implements a classic wrapper pattern where cache logic surrounds semantic analysis calls. The Semantic Analyzer exposes a simple `Analyze(metadata)` interface, and the worker pool wraps this with cache-first logic. The analyzer never sees cache hits - it's only invoked on cache misses.

**Complete Result Caching:**
The cache stores the entire `SemanticAnalysis` structure returned by the analyzer, including summary (2-3 sentence description), tags (semantic keywords), key topics (main themes), document type (genre classification), and confidence score. All fields are preserved through caching, ensuring cache hits are indistinguishable from fresh analysis.

**Metadata Preservation:**
The cache stores not just semantic analysis results but also the complete `FileMetadata` structure alongside them. This enables reconstruction of full index entries from cache without re-extracting metadata. The cached metadata also provides context for debugging and cache inspection.

**Error Caching:**
The cache structure includes an optional `Error` field that stores error messages when analysis fails. This enables caching of negative results (analysis attempts that failed), preventing repeated API calls for files that consistently fail analysis. Future enhancements could implement retry logic that respects this cached error state.

### Type System

The Cache Manager uses structures defined in the Type System (`pkg/types/types.go`) to represent cached data with full fidelity.

**CachedAnalysis Structure:**
The primary cache data structure contains:
- `FilePath string` - Original file path (for reference and debugging, not used as cache key)
- `FileHash string` - SHA-256 content hash in "sha256:hex" format (the actual cache key)
- `AnalyzedAt time.Time` - Timestamp of when semantic analysis was performed
- `Metadata FileMetadata` - Complete file metadata including path, size, type, category, word count, dimensions, etc.
- `Semantic *SemanticAnalysis` - AI-generated semantic understanding (nil if analysis failed or was disabled)
- `Error *string` - Optional error message if analysis failed

**Relationship to Index Entries:**
The `CachedAnalysis` structure closely mirrors `IndexEntry`, containing the same `Metadata` and `Semantic` fields. This alignment enables direct conversion from cached analysis to index entry without transformation logic. The cache essentially stores serialized index entries keyed by content hash.

**JSON Serialization:**
All cached types use JSON struct tags for serialization. The `json:"field_name"` tags provide consistent field naming in JSON files. Optional fields use `omitempty` to exclude them when nil or empty. Pretty-printing (indented JSON) makes cache files human-readable for debugging and inspection.

**Hash Field Integration:**
The `FileHash` field in `CachedAnalysis` serves as both the cache key and a stored field. This redundancy enables staleness checking (compare stored hash against current hash) and provides self-describing cache entries where each file contains its own cache key.

## Glossary

**Content Hash**: SHA-256 cryptographic hash of file contents prefixed with "sha256:", used as the primary cache key for storing and retrieving cached analyses.

**Cache Hit**: Successful retrieval of cached analysis for a file whose content hasn't changed, avoiding expensive Claude API call through fast local file read.

**Cache Miss**: Failed cache lookup requiring new semantic analysis via Claude API, typically occurring on first analysis or after file content changes.

**Cache Hit Rate**: Percentage of files served from cache versus total files processed, typically exceeding 95% in stable codebases indicating effective caching.

**Cache Staleness**: Condition where a cached entry's file hash doesn't match the current file's content hash, indicating content has changed and cache entry is invalid.

**Content-Addressable Storage**: Storage system where data location is determined by content hash rather than arbitrary key, enabling automatic invalidation and location independence.

**File-Based Cache**: Persistence strategy using individual JSON files for each cached entry rather than database or in-memory storage, providing durability and debuggability.

**Orphaned Cache Entry**: Cache file no longer referenced by any current file (old hash after content change), harmless but consuming disk space until manual cleanup.

**Cache-First Pattern**: Processing strategy where expensive operations are attempted from cache before falling back to full computation, implemented consistently in worker pool and daemon.

**Pretty-Printed JSON**: JSON format with indentation and newlines for human readability, used for all cache files to aid debugging despite slightly larger file size.

**Automatic Invalidation**: Cache invalidation strategy where content changes automatically trigger cache misses through hash mismatch without explicit invalidation logic or expiration.

**Stateless Operations**: Cache operations that don't maintain runtime state between calls, enabling concurrent access from multiple workers without coordination overhead.

**Hash Prefix**: First 16 characters of content hash used as cache filename, providing manageable filename lengths while maintaining negligible collision probability.

**Cache Directory**: Designated file system location for storing cache files (default: `~/.agentic-memorizer/.cache`), containing `summaries` subdirectory with JSON files.

**Durability**: Property where cached entries persist across daemon restarts through file-based storage, avoiding redundant API calls during index rebuilds.

**Location Independence**: Property where files retain cached analysis even when renamed or moved, enabled by content-based rather than path-based cache keys.
