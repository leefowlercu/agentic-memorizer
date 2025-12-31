# Vector Embeddings

Multi-provider text embedding generation with content-addressable caching for semantic similarity search in the knowledge graph.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-31

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Vector Embeddings subsystem converts file content into high-dimensional vector representations that enable semantic similarity search. When files are processed, their summaries are embedded into vectors stored in FalkorDB, enabling queries like "find files similar to this one" without relying solely on tag or topic matches. The subsystem supports three embedding providers: OpenAI, Voyage AI, and Google Gemini.

The subsystem follows the same provider pattern used by semantic analysis: a Provider interface with a registry enabling multi-provider support. Providers self-register via Go init() functions. A dedicated cache stores embeddings by content hash with provider segregation, avoiding redundant API calls for unchanged content. The worker pool generates embeddings during file processing and stores them in provider-prefixed properties in the knowledge graph.

Key capabilities include:

- **Provider registry** - Self-registering providers via init() functions and factory pattern
- **Three providers** - OpenAI (1536d), Voyage AI (1024d), and Gemini (768d) with model selection
- **Binary caching** - Content-addressable storage with provider-segregated directories
- **Batch processing** - EmbedBatch method for efficient multi-text embedding in single API call
- **Rate limiting** - Provider-specific rate limits (OpenAI: 500, Voyage: 300, Gemini: 100 RPM)
- **Vector storage** - Provider-prefixed HNSW indexes in FalkorDB for similarity search

## Design Principles

### Provider Registry Pattern

The subsystem uses a registry pattern with self-registering providers. Each provider registers via Go init() functions through blank imports. The registry stores factory functions that create provider instances on demand. This pattern enables adding new providers without modifying core code - just import the provider package and it registers itself.

### Batch-First API Design

The Embed method delegates to EmbedBatch with a single-element array. This ensures consistent behavior and makes batch processing the primary codepath. OpenAI's embedding API naturally supports batching, making this design efficient for both single and bulk operations.

### Binary Cache Format

Embeddings are cached as binary files rather than JSON. The format stores dimension count as a uint32 followed by little-endian float32 values. This binary format is more compact than JSON (4 bytes per float vs ~10+ characters) and faster to read/write. The cache uses a two-level directory structure (first 4 hash characters) with provider-specific subdirectories (embeddings/openai, embeddings/voyage, embeddings/gemini) to segregate embeddings by provider.

### Content-Addressable Storage

Like the semantic analysis cache, embeddings are keyed by content hash. If a file's content hasn't changed, its cached embedding remains valid regardless of renames or moves. This eliminates redundant API calls during rebuilds when file content is unchanged.

### Separate Rate Limiting

The worker pool maintains a separate rate limiter for embedding API calls distinct from semantic analysis rate limiting. Each provider has specific default rate limits: OpenAI (500 RPM), Voyage AI (300 RPM), Gemini (100 RPM). This separation prevents embedding generation from consuming semantic analysis quota and vice versa.

### Optional Enablement

Embeddings require a provider-specific API key and are disabled by default. Configuration specifies provider (openai, voyage, gemini), model, and API key. When disabled, files are processed without embeddings and vector similarity search falls back to relationship-based queries.

## Key Components

### Provider Interface

The Provider interface defines the contract for embedding generation with six methods: Embed generates a single vector, EmbedBatch generates vectors for multiple texts efficiently, Dimensions returns vector size, Model returns the model identifier, Name returns the provider name, and DefaultRateLimit returns the provider's recommended rate limit. All providers must implement this interface.

### Registry

The Registry provides thread-safe registration and retrieval of embedding providers. Providers register via Register() with a factory function. The Get() method returns a new provider instance by name. List() returns all registered provider names. The GlobalRegistry() function returns the singleton registry instance.

### EmbeddingResult Struct

The EmbeddingResult struct pairs original text with its embedding vector and model name. Used for returning embedding results with context about what was embedded.

### OpenAI Provider

Located in providers/openai/. Implements the Provider interface using the go-openai client library. Supports models: text-embedding-3-small (1536d), text-embedding-3-large (3072d), text-embedding-ada-002 (1536d). Default rate limit: 500 RPM. Self-registers via init().

### Voyage Provider

Located in providers/voyage/. Implements the Provider interface for Voyage AI's embedding API. Supports models: voyage-3 (1024d), voyage-3-lite (512d), voyage-code-3 (1024d). Default rate limit: 300 RPM. Self-registers via init().

### Gemini Provider

Located in providers/gemini/. Implements the Provider interface for Google's Gemini embedding API. Supports models: text-embedding-004 (768d), embedding-001 (768d). Default rate limit: 100 RPM. Self-registers via init().

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

The config subsystem defines embeddings settings: enabled, provider (openai, voyage, gemini), model, dimensions, and api_key. Environment variables are provider-specific: OPENAI_API_KEY, VOYAGE_API_KEY, GOOGLE_API_KEY. Validation ensures model and dimension compatibility for each provider.

### Knowledge Graph Storage

File nodes in FalkorDB include provider-prefixed embedding properties (embedding_openai, embedding_voyage, embedding_gemini). The graph subsystem provides UpsertFileWithEmbedding with a provider parameter for storing embeddings during updates. The schema creates separate HNSW vector indexes for each provider with appropriate dimensions.

### Vector Similarity Search

The graph Manager exposes VectorSearch with a provider parameter for finding similar files. The Queries struct implements the actual search using FalkorDB's vector query API (db.idx.vector.queryNodes) on the provider-specific property. Results include similarity scores enabling ranked results.

### Semantic Analysis Workflow

Embeddings are generated from the summary field of semantic analysis output, not raw file content. This means files must complete semantic analysis before embedding generation. The summary provides a dense, meaningful text representation of the file's content.

### Rate Limiting

The worker pool creates a separate rate limiter for embeddings distinct from semantic analysis rate limiting. The rate limit uses the provider's DefaultRateLimit() (OpenAI: 500, Voyage: 300, Gemini: 100 RPM). This prevents embedding generation from consuming all available API quota and vice versa.

## Glossary

**Content Hash**
SHA-256 hash of file contents used as cache key. Enables cache hits across file renames and automatic invalidation on content changes.

**Dimensions**
The number of elements in an embedding vector. OpenAI uses 1536-3072d, Voyage uses 512-1024d, Gemini uses 768d. Higher dimensions can capture more nuance but require more storage.

**Embedding**
A dense vector representation of text in high-dimensional space. Similar texts produce vectors that are geometrically close, enabling similarity search.

**HNSW**
Hierarchical Navigable Small World graph algorithm for approximate nearest neighbor search. FalkorDB uses HNSW indexes for efficient vector similarity queries.

**Provider**
An implementation of the embedding generation interface. Supported providers: OpenAI, Voyage AI, Google Gemini. Each provider self-registers via init() functions.

**Rate Limiter**
Token bucket rate limiter controlling API call frequency. The embeddings subsystem uses provider-specific rate limits separate from semantic analysis.

**Registry**
Thread-safe container for provider factory functions. Enables self-registration via init() and dynamic provider creation by name.

**Vector Index**
A specialized database index optimized for nearest neighbor search in high-dimensional vector space. Created on provider-specific properties (embedding_openai, embedding_voyage, embedding_gemini) in FalkorDB.

**Vector Search**
Finding items with similar embedding vectors using distance metrics (cosine similarity). Returns ranked results by similarity score using the appropriate provider-specific index.
