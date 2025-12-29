# Vector Embeddings

Provider-based text embedding generation with content-addressable caching for semantic similarity search in the knowledge graph.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Vector Embeddings subsystem converts file content into high-dimensional vector representations that enable semantic similarity search. When files are processed, their summaries are embedded into vectors stored in FalkorDB, enabling queries like "find files similar to this one" without relying solely on tag or topic matches. The subsystem currently supports OpenAI's embedding API with plans for additional providers.

The subsystem follows the same provider pattern used by semantic analysis: a Provider interface enabling future multi-provider support, with OpenAI as the initial implementation. A dedicated cache stores embeddings by content hash, avoiding redundant API calls for unchanged content. The worker pool generates embeddings during file processing and stores them alongside semantic analysis results in the knowledge graph.

Key capabilities include:

- **Provider interface** - Abstract contract for embedding generation with batch support
- **OpenAI integration** - Production-ready implementation using text-embedding-3-small (1536 dimensions)
- **Binary caching** - Content-addressable storage with efficient float32 serialization
- **Batch processing** - EmbedBatch method for efficient multi-text embedding in single API call
- **Rate limiting** - Separate rate limiter (500 RPM) for embedding API calls
- **Vector storage** - HNSW index in FalkorDB for similarity search

## Design Principles

### Provider Interface Pattern

The subsystem defines a Provider interface with four methods: Embed for single texts, EmbedBatch for efficient multi-text embedding, Dimensions for vector size, and Model for identification. This abstraction enables future provider additions (Voyage AI, Cohere, local models) without changing consuming code. The interface matches patterns used in the semantic analysis subsystem.

### Batch-First API Design

The Embed method delegates to EmbedBatch with a single-element array. This ensures consistent behavior and makes batch processing the primary codepath. OpenAI's embedding API naturally supports batching, making this design efficient for both single and bulk operations.

### Binary Cache Format

Embeddings are cached as binary files rather than JSON. The format stores dimension count as a uint32 followed by little-endian float32 values. This binary format is more compact than JSON (4 bytes per float vs ~10+ characters) and faster to read/write. The cache uses a two-level directory structure (first 4 hash characters) to avoid filesystem limitations with many files.

### Content-Addressable Storage

Like the semantic analysis cache, embeddings are keyed by content hash. If a file's content hasn't changed, its cached embedding remains valid regardless of renames or moves. This eliminates redundant API calls during rebuilds when file content is unchanged.

### Separate Rate Limiting

The worker pool maintains a separate rate limiter for embedding API calls (500 RPM) distinct from semantic analysis rate limiting. OpenAI's embedding API has different rate limits than their chat/completion API, and this separation prevents one from starving the other.

### Optional Enablement

Embeddings require an OpenAI API key and are disabled by default. The configuration derives embeddings.enabled from API key presence. When disabled, files are processed without embeddings and vector similarity search falls back to relationship-based queries.

## Key Components

### Provider Interface

The Provider interface defines the contract for embedding generation with four methods: Embed generates a single vector, EmbedBatch generates vectors for multiple texts efficiently, Dimensions returns vector size (e.g., 1536), and Model returns the model identifier. All providers must implement this interface.

### EmbeddingResult Struct

The EmbeddingResult struct pairs original text with its embedding vector and model name. Used for returning embedding results with context about what was embedded.

### OpenAIProvider

The OpenAI provider implements the Provider interface using the go-openai client library. Default configuration uses text-embedding-3-small with 1536 dimensions. The provider logs embedding generation with timing, token usage, and dimension information.

### OpenAIConfig Struct

The OpenAIConfig struct holds configuration for the OpenAI provider: APIKey for authentication, Model for model selection (text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002), and Dimensions for vector size. DefaultOpenAIConfig returns sensible defaults.

### Cache Struct

The Cache struct provides thread-safe, content-hash-based storage for embeddings. Uses RWMutex for concurrent access. The cache directory uses a two-level structure (hash[:2]/hash[2:4]) to distribute files and avoid filesystem bottlenecks.

### Cache Get Method

The Get method retrieves an embedding by content hash, returning the vector and a boolean indicating success. Returns nil and false on cache miss. Handles file read and decode errors gracefully, logging warnings without failing.

### Cache Set Method

The Set method stores an embedding by content hash. Creates parent directories as needed. Uses binary encoding for efficient storage. Thread-safe via mutex locking.

### Cache Clear and Delete Methods

The Clear method removes all cached embeddings by deleting the embeddings subdirectory. The Delete method removes a single embedding by hash. Both handle missing files gracefully.

### Binary Encoding Functions

The encodeEmbedding function converts float32 slices to bytes with a dimension count prefix. The decodeEmbedding function reverses this, validating length before parsing. Little-endian byte order matches most modern systems.

## Integration Points

### Daemon Worker Pool

The worker pool receives an optional embedding provider and cache at construction. During file processing, if embeddings are enabled and the file has a summary, the pool generates an embedding for the summary text. Cache checks happen first to avoid redundant API calls. Generated embeddings are returned in JobResult for graph storage.

### Configuration System

The config subsystem defines embeddings settings: api_key (or OPENAI_API_KEY env var), provider (only "openai" supported), model, and dimensions. The embeddings.enabled flag is derived from API key presence. Validation ensures model and dimension compatibility.

### Knowledge Graph Storage

File nodes in FalkorDB include an optional embedding property storing the float32 vector. The graph subsystem provides UpsertFileWithEmbedding for storing embeddings during updates. The schema creates an HNSW vector index on File.embedding for similarity search.

### Vector Similarity Search

The graph Manager exposes VectorSearch for finding similar files. The Queries struct implements the actual search using FalkorDB's vector query API (db.idx.vector.queryNodes). Results include similarity scores enabling ranked results.

### Semantic Analysis Workflow

Embeddings are generated from the summary field of semantic analysis output, not raw file content. This means files must complete semantic analysis before embedding generation. The summary provides a dense, meaningful text representation of the file's content.

### Rate Limiting

The worker pool creates a separate rate limiter for embeddings (500 RPM with burst of 10) distinct from semantic analysis rate limiting. This prevents embedding generation from consuming all available API quota and vice versa.

## Glossary

**Content Hash**
SHA-256 hash of file contents used as cache key. Enables cache hits across file renames and automatic invalidation on content changes.

**Dimensions**
The number of elements in an embedding vector. OpenAI's text-embedding-3-small produces 1536-dimensional vectors. Higher dimensions can capture more nuance but require more storage.

**Embedding**
A dense vector representation of text in high-dimensional space. Similar texts produce vectors that are geometrically close, enabling similarity search.

**HNSW**
Hierarchical Navigable Small World graph algorithm for approximate nearest neighbor search. FalkorDB uses HNSW indexes for efficient vector similarity queries.

**Provider**
An implementation of the embedding generation interface. Currently only OpenAI is supported; the interface enables future provider additions.

**Rate Limiter**
Token bucket rate limiter controlling API call frequency. The embeddings subsystem uses a separate limiter (500 RPM) from semantic analysis.

**Vector Index**
A specialized database index optimized for nearest neighbor search in high-dimensional vector space. Created on the File.embedding property in FalkorDB.

**Vector Search**
Finding items with similar embedding vectors using distance metrics (cosine similarity, Euclidean distance). Returns ranked results by similarity score.
