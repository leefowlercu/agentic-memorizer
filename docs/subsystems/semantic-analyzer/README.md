# Semantic Analyzer Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Content-Based Routing](#content-based-routing)
   - [Retry Logic with Exponential Backoff](#retry-logic-with-exponential-backoff)
   - [Graceful Degradation](#graceful-degradation)
   - [Configuration-Driven Behavior](#configuration-driven-behavior)
3. [Key Components](#key-components)
   - [Analyzer Component](#analyzer-component)
   - [Client Component](#client-component)
   - [Entity Extraction](#entity-extraction)
   - [Reference Extraction](#reference-extraction)
   - [Analysis Strategies](#analysis-strategies)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Metadata Extractor](#metadata-extractor)
   - [Cache Manager](#cache-manager)
   - [Type System](#type-system)
5. [Glossary](#glossary)

## Overview

The Semantic Analyzer subsystem provides AI-powered understanding of files using the Claude API. It transforms raw file content into structured semantic information including summaries, tags, key topics, document type classifications, entity extraction, reference mapping, and confidence scores. This subsystem operates as the second stage of file processing (after metadata extraction), enriching file entries with intelligent analysis that enables AI agents to understand file content and purpose before reading them.

The subsystem is architected around three core components: an Analyzer orchestration layer that routes files to appropriate analysis strategies, a Client integration layer that manages Claude API communication with retry logic and error handling, and specialized analysis methods for different content types (text, images, Office documents, PDFs). The analyzer intelligently selects the optimal Claude API capability for each file type, using vision analysis for images, document blocks for PDFs, and text extraction for Office formats.

By providing structured semantic understanding of files, the Semantic Analyzer enables AI agents to quickly grasp file content, identify relevant files for tasks, and understand relationships between files without reading entire contents. The subsystem integrates tightly with the cache manager to avoid redundant API calls, respects rate limits to maintain quota compliance, and provides graceful degradation when analysis fails or is disabled.

## Design Principles

### Content-Based Routing

The Semantic Analyzer implements intelligent content-based routing that selects the optimal analysis strategy based on file type and metadata characteristics. Rather than applying a one-size-fits-all approach, the analyzer examines the file's category, type, size, and readability to determine the most effective analysis method.

The routing decision tree operates as follows:
1. Files exceeding the maximum size limit are rejected before analysis begins
2. Files in the "images" category with vision enabled are routed to vision analysis with base64 encoding
3. Files with type "docx" or "pptx" trigger document analysis with ZIP-based text extraction
4. All other readable files (including PDFs) undergo text analysis with content truncation at 100KB
5. Non-readable or binary files fall back to metadata-only analysis

**Note on PDF Handling**: While PDF analysis code with document content blocks exists in `analyzeDocument()` (lines 143-190 of analyzer.go), it is currently unreachable from the main `Analyze()` routing method. PDFs route through `analyzeText()` which attempts to read them as text files. The document content block implementation represents planned future functionality that requires routing logic updates to activate.

This routing strategy maximizes the effectiveness of Claude's multimodal capabilities by matching file characteristics to appropriate API features. Vision analysis leverages Claude's ability to understand visual content in diagrams and screenshots. Document blocks enable native PDF comprehension. Text extraction surfaces content from Office formats. The fallback strategy ensures that all files receive at least baseline analysis from metadata.

### Retry Logic with Exponential Backoff

The Client component implements robust retry logic with exponential backoff to handle transient API failures gracefully. This pattern increases system resilience against temporary network issues, rate limiting responses, and server errors without overwhelming the API with repeated requests.

The retry mechanism operates with the following characteristics:
- Maximum of 4 total attempts per API request (1 initial attempt + 3 retries)
- Exponential delay sequence: 1 second, 2 seconds, 4 seconds between retry attempts
- Retries triggered by network errors and specific HTTP status codes (429 rate limiting, 500/502/503/504 server errors)
- Immediate failure without retry for client errors (400 bad request, 401 unauthorized, 403 forbidden)
- Preservation of the last error message for debugging and user visibility

This approach balances reliability with efficiency. The exponential backoff prevents retry storms that could exacerbate API load during incidents. The selective retry logic avoids wasting attempts on permanent failures like authentication errors. The bounded retry count ensures that processing doesn't hang indefinitely on persistent failures.

### Graceful Degradation

The Semantic Analyzer prioritizes system availability and progress over perfect analysis results. When semantic analysis fails due to API errors, file format issues, or disabled configuration, the subsystem implements graceful degradation strategies that ensure files can still be indexed with baseline information.

Degradation strategies include:
- Binary/unknown files receive synthesized analysis from metadata (e.g., "PPTX file with 15 slides")
- Failed text extraction from Office documents falls back to binary analysis
- API errors during analysis allow the file to be indexed with metadata only
- Disabled semantic analysis configuration results in metadata-only index entries
- Lower confidence scores (0.5) are assigned to fallback analyses

This design ensures that a single corrupted file, API outage, or configuration choice doesn't prevent the entire index from being built. Users receive structured information about all files, with rich semantic understanding where analysis succeeds and basic metadata where it doesn't. The confidence scores provide transparency about analysis quality, allowing downstream consumers to adjust their trust accordingly.

### Configuration-Driven Behavior

All semantic analyzer behavior is controlled through configuration parameters, enabling users to tune the subsystem's operation without code changes. This design separates policy decisions (what to analyze, how aggressively) from mechanism implementation (how to analyze).

Key configuration parameters include:
- `analysis.enabled` - Global toggle for semantic analysis (derived from Claude API key presence)
- `analysis.max_file_size` - Maximum file size for analysis in bytes (default: 10MB)
- `claude.enable_vision` - Toggle for image vision analysis (hardcoded: true)
- `claude.model` - Claude model identifier (default: claude-sonnet-4-5-20250929)
- `claude.max_tokens` - Response length limit (default: 1500 tokens)
- `claude.timeout_seconds` - API request timeout (default: 30 seconds)
- `claude.api_key` - Anthropic API authentication key (required when analysis enabled)
- `daemon.rate_limit_per_min` - API call rate limit (default: 20 calls/minute)

This configuration-driven approach enables several important use cases:
- Running without semantic analysis for metadata-only indexing
- Adjusting rate limits to match API quota tier
- Disabling vision analysis to reduce API costs
- Tuning file size limits based on content characteristics
- Experimenting with different Claude models for analysis quality vs. cost trade-offs

## Key Components

### Analyzer Component

The Analyzer component (`internal/semantic/analyzer.go`) serves as the orchestration layer that routes files to appropriate analysis strategies and manages the overall analysis workflow.

**Core State:**
- `client *Client` - Claude API client instance for making analysis requests
- `enableVision bool` - Configuration flag controlling image vision analysis
- `maxFileSize int64` - Size limit enforcement threshold in bytes

**Primary Responsibilities:**
- **Content Routing**: Examines file metadata (category, type, size, readability) to select optimal analysis strategy
- **Text Analysis**: Reads and truncates text content, sends to Claude API for semantic understanding
- **Vision Analysis**: Encodes images in base64, determines MIME types, invokes Claude vision capabilities
- **Document Processing**: Extracts text from DOCX/PPTX via ZIP parsing and XML text run extraction
- **PDF Handling**: Encodes PDFs for native Claude document understanding
- **Binary Fallback**: Synthesizes analysis from metadata when content analysis isn't possible
- **Prompt Construction**: Builds structured prompts with file context and output format requirements requesting summary, tags, topics, document type, entities, and references
- **Entity Extraction**: Identifies named entities (technologies, people, organizations, concepts, projects) mentioned in file content
- **Reference Mapping**: Extracts topic dependencies and relationships (requires, extends, related-to, implements) with confidence scoring
- **Response Parsing**: Extracts JSON from Claude responses, including fallback extraction from markdown code blocks

**Analysis Methods:**

**`Analyze()`** - Main entry point that enforces size limits and routes to appropriate strategy based on file characteristics

**`analyzeText()`** - Handles text-based files (Markdown, code, JSON, YAML, plain text) by reading content, truncating at 100KB, and sending text message to Claude

**`analyzeImage()`** - Processes image files when vision is enabled by base64 encoding the image, determining MIME type, and sending vision message with image content block

**`analyzeDocument()`** - Routes Office documents and PDFs to specialized handlers based on file type (PPTX, DOCX, PDF)

**`analyzePptx()`** - Extracts text from PowerPoint presentations by opening as ZIP archive, parsing slide XML files, and extracting text runs from `<a:t>` tags

**`analyzeDocx()`** - Extracts text from Word documents by opening as ZIP archive, parsing document XML, and extracting text runs from `<w:t>` tags

**`analyzeBinary()`** - Creates fallback analysis from metadata only, synthesizing summary from file type and characteristics with confidence of 0.5, and initializing empty entity and reference arrays

**`buildPrompt()`** - Constructs analysis prompts that include file metadata context and request JSON-formatted responses with specific fields (summary, tags, key_topics, document_type, entities, references)

**`extractJSON()`** - Parses Claude responses, first attempting direct JSON unmarshaling, then falling back to extracting JSON from markdown code blocks

**`getMediaType()`** - Maps file extensions to MIME types for image analysis (image/png, image/jpeg, image/gif, image/webp)

### Client Component

The Client component (`internal/semantic/client.go`) manages all HTTP communication with the Claude API, implementing authentication, request formatting, retry logic, and error handling.

**Core Configuration:**
- `apiKey string` - Anthropic API key for authentication via X-API-Key header
- `model string` - Claude model identifier (default: claude-sonnet-4-5-20250929)
- `maxTokens int` - Maximum response length in tokens (default: 1500)
- `timeout time.Duration` - HTTP request timeout (default: 30 seconds)
- `httpClient *http.Client` - Configured HTTP client with timeout and connection pooling

**API Integration:**
- **Endpoint**: https://api.anthropic.com/v1/messages
- **API Version**: 2023-06-01 (sent via anthropic-version header)
- **Authentication**: API key sent via X-API-Key header
- **Content Type**: application/json for all requests
- **Request Format**: Messages API with system and user messages, content blocks

**Communication Methods:**

**`SendMessage()`** - Sends text-based analysis request with simple text content block, used for analyzing text files, code, and extracted document content

**`SendMessageWithImage()`** - Sends vision analysis request with image content block containing base64-encoded image data and MIME type, used for analyzing images when vision is enabled

**`SendMessageWithDocument()`** - Sends document analysis request with document content block containing base64-encoded PDF data, used for native PDF understanding

**`doWithRetry()`** - Executes HTTP requests with exponential backoff retry logic, handling transient failures and rate limiting responses

**Request Structure:**
All API requests use the Claude Messages API format with:
- `model` field specifying the Claude model
- `max_tokens` field limiting response length
- `messages` array with single user message containing content blocks (text, image, or document)

**Note**: The current implementation does not use the `system` field. All prompts are sent as user messages without system-level instructions.

**Error Handling:**
The client distinguishes between retryable errors (network failures, 429 rate limiting, 500/502/503/504 server errors) and permanent errors (400/401/403 client errors), implementing retry logic only for transient failures.

### Entity Extraction

The Semantic Analyzer extracts named entities from file content to identify key technologies, people, organizations, concepts, and projects mentioned or depicted in files. Entity extraction is performed across all analysis strategies (text, vision, document, presentation) to build a comprehensive understanding of what and who files reference.

**Entity Structure:**
Each extracted entity consists of two fields:
- `name` (string) - The entity name (e.g., "Terraform", "AWS", "Docker", "Claude", "Python")
- `type` (string) - Entity classification: technology, person, concept, organization, project

**Extraction Guidance by Analysis Type:**
- **Image Analysis**: Identifies logos, product names, people, and technologies visible in the image
- **Document/Presentation Analysis**: Focuses on specific technologies, tools, people, organizations, and key concepts mentioned in text
- **Text Analysis**: Extracts technologies, frameworks, people, and concepts referenced in code or documentation

**Implementation Details:**
All five analysis methods (`analyzeText()`, `analyzeImage()`, `analyzeDocument()`, `analyzePptx()`, `analyzeDocx()`) request entities as the 5th field in their Claude API prompts. The binary fallback strategy (`analyzeBinary()`) initializes an empty entity array to maintain structural consistency.

**Use Cases:**
Entity extraction enables several key capabilities:
- Finding all files that mention a specific technology (e.g., "Show me files about Docker")
- Understanding technology dependencies across the knowledge base
- Identifying subject matter experts mentioned in documentation
- Building technology relationship graphs through the FalkorDB integration

### Reference Extraction

The Semantic Analyzer extracts topic references and dependencies from file content to map how files relate to, build upon, or depend on other concepts. Reference extraction captures the relational structure of knowledge, identifying what files require, extend, relate to, or implement.

**Reference Structure:**
Each extracted reference consists of three fields:
- `topic` (string) - The referenced topic or concept (e.g., "containerization", "authentication", "REST APIs")
- `type` (string) - Relationship classification: requires, extends, related-to, implements
- `confidence` (float64) - Strength of the reference from 0.0 to 1.0

**Reference Types:**
- **requires**: The file depends on or assumes knowledge of the referenced topic
- **extends**: The file builds upon or expands the referenced topic
- **related-to**: The file has a general connection to the referenced topic
- **implements**: The file provides an implementation of the referenced concept

**Extraction Guidance by Analysis Type:**
- **Image Analysis**: Identifies concepts the image relates to or explains
- **Document/Presentation Analysis**: Identifies what the content depends on, builds upon, or relates to
- **Text Analysis**: Extracts dependencies and relationships mentioned in code or documentation

**Implementation Details:**
All five analysis methods request references as the 6th field in their Claude API prompts. The binary fallback strategy initializes an empty reference array. Claude provides confidence scores for each reference to indicate the strength and certainty of the relationship.

**Use Cases:**
Reference extraction enables advanced knowledge graph capabilities:
- Understanding prerequisite knowledge for learning paths
- Identifying tightly coupled concepts that should be learned together
- Finding files that build upon or extend specific topics
- Mapping conceptual dependencies across documentation

### Analysis Strategies

The Semantic Analyzer employs distinct strategies for different file types, each optimized for the characteristics and content structure of its target format.

#### Text Analysis Strategy

**Target Files**: Markdown, code files (Go, Python, JavaScript, etc.), JSON, YAML, TOML, plain text

**Approach**:
1. Read entire file content into memory
2. Truncate at 100KB if content exceeds limit (appends "[Content truncated...]" message)
3. Build prompt including filename, type, category, size, and full/truncated content
4. Send text message to Claude API
5. Parse JSON response into SemanticAnalysis structure

**Optimization**: Direct file reading with size-based truncation balances thoroughness with API token costs

#### Vision Analysis Strategy

**Target Files**: PNG, JPG, JPEG, GIF, WebP images (when vision enabled)

**Approach**:
1. Read entire image file into memory
2. Base64 encode the image data for transmission
3. Determine MIME type from file extension (image/png, image/jpeg, etc.)
4. Build prompt including filename, type, and dimensions from metadata
5. Send vision message with image content block to Claude API
6. Parse JSON response extracting visual understanding

**Capability**: Leverages Claude's multimodal capabilities to understand visual content, diagrams, screenshots, charts, and graphical information

#### PowerPoint Analysis Strategy

**Target Files**: PPTX presentations

**Approach**:
1. Open PPTX file as ZIP archive using standard library
2. Enumerate and parse slide XML files from `ppt/slides/` directory
3. Extract text runs from `<a:t>` XML tags across all slides
4. Concatenate extracted text, truncate at 50KB if needed
5. Build prompt including filename, slide count from metadata, and extracted text
6. Send text message to Claude API with presentation context
7. Parse JSON response understanding presentation structure and content

**Design Note**: Text extraction happens in analyzer (not metadata extractor) to maintain separation of concerns and avoid circular dependencies

#### Word Document Analysis Strategy

**Target Files**: DOCX documents

**Approach**:
1. Open DOCX file as ZIP archive (Office Open XML format)
2. Parse `word/document.xml` file containing document structure
3. Extract text runs from `<w:t>` XML tags throughout document
4. Concatenate extracted text, truncate at 50KB if needed
5. Build prompt including filename, word count from metadata, and extracted text
6. Send text message to Claude API with document context
7. Parse JSON response understanding document content and structure

**Optimization**: Simple XML text extraction (not full DOM parsing) reduces dependencies and improves performance

#### PDF Analysis Strategy (Currently Inactive)

**Target Files**: PDF documents

**Current Status**: While PDF document content block analysis code exists in the implementation (`analyzeDocument()` method at analyzer.go:143-190), this functionality is currently **not reachable** from the main routing logic. PDFs currently route through the text analysis strategy which attempts to read them as text files.

**Planned Approach** (when activated):
1. Read entire PDF file into memory
2. Base64 encode the PDF data for transmission
3. Build prompt including filename, size, and optional page count from metadata
4. Send document message with document content block (MIME type: application/pdf)
5. Parse JSON response leveraging Claude's native PDF understanding

**Future Capability**: Once routing is updated, this will use Claude's built-in PDF comprehension to understand document structure, text, and layout without external PDF parsing libraries

#### Binary/Fallback Strategy

**Target Files**: Non-readable files, files where extraction fails, files when analysis is disabled

**Approach**:
1. Synthesize summary from metadata (e.g., "PPTX file with 15 slides")
2. Generate basic tags from category and type (e.g., ["presentations", "pptx"])
3. Create minimal key topics from category
4. Set document type to category value
5. Assign confidence score of 0.5 to indicate metadata-only analysis
6. Initialize empty entity array to maintain structural consistency
7. Initialize empty reference array to maintain structural consistency

**Purpose**: Ensures all files receive structured analysis entries, enabling graceful degradation when semantic understanding isn't available

## Integration Points

### Daemon Subsystem

The Daemon subsystem (`internal/daemon/daemon.go`) creates and manages the Semantic Analyzer as an optional component controlled by the `analysis.enabled` configuration parameter.

**Initialization**:
During daemon startup, if semantic analysis is enabled, the daemon creates a Client instance with API key, model, max tokens, and timeout from configuration. It then creates an Analyzer instance with the client, vision enablement flag, and max file size limit. If semantic analysis is disabled, the analyzer remains nil throughout the daemon's lifecycle.

**Worker Pool Integration**:
The daemon passes the analyzer (or nil) to the worker pool during initialization. Each worker thread uses the analyzer to process files from its work queue. The analyzer invocation occurs as the third stage of the processing pipeline, after metadata extraction and file hashing.

**Incremental Update Integration**:
When the File Watcher detects a file change, the daemon's event handler invokes the analyzer directly (if enabled) after extracting metadata and computing the file hash. This ensures that incremental updates receive the same semantic analysis as full index builds.

**Processing Flow**:
1. Daemon initializes analyzer with configuration parameters
2. File requires processing (initial walk or watcher event)
3. Metadata extractor gathers file metadata
4. Cache manager checks for existing analysis by content hash
5. On cache miss, analyzer performs semantic understanding
6. Result is cached and added to index
7. Index is persisted atomically to disk

**Optional Component Pattern**:
The analyzer is only created when `analysis.enabled` is true. When disabled, the daemon processes files with metadata only, allowing the system to operate without Claude API access or for metadata-only indexing use cases.

### Metadata Extractor

The Semantic Analyzer depends on the Metadata Extractor subsystem to provide baseline file information that guides analysis strategy selection and enriches prompts with file context.

**Dependency Flow**:
The Metadata Extractor always runs first, producing a `FileMetadata` structure that is passed to the analyzer's `Analyze()` method. This structure contains path, type, category, size, modification time, readability flag, and type-specific metadata (dimensions, word count, slide count, etc.).

**Routing Dependencies**:
The analyzer examines specific metadata fields to determine analysis strategy:
- `Category` field routes images to vision analysis when set to "images"
- `Type` field detects Office documents ("docx", "pptx") requiring text extraction
- `IsReadable` flag triggers binary fallback for non-readable files
- `Size` field enforces maximum file size limit before analysis begins

**Context Enrichment**:
Metadata fields are included in analysis prompts to provide Claude with context:
- Image dimensions help Claude understand scale and aspect ratio
- Slide counts indicate presentation length and structure
- Word counts suggest document depth and complexity
- File categories guide analysis focus and output format

**Separation of Concerns**:
The Metadata Extractor focuses on fast, deterministic structural metadata extraction. The Semantic Analyzer performs slower AI-powered content understanding. This separation enables efficient caching (metadata extraction always occurs to compute hashes, semantic analysis is cached), parallel processing (metadata extraction is CPU-bound, semantic analysis is I/O-bound), and graceful degradation (files can be indexed with metadata when semantic analysis fails).

### Cache Manager

The Cache Manager subsystem (`internal/cache/manager.go`) wraps the Semantic Analyzer to avoid redundant API calls for files with unchanged content, significantly reducing API costs and improving indexing performance.

**Cache Key Strategy**:
After metadata extraction, the daemon or worker computes a SHA-256 hash of the file's content using `cache.HashFile()`. This content hash serves as the cache key, enabling cache hits even when files are renamed or moved as long as their content remains unchanged.

**Cache Lookup Flow**:
1. Worker extracts metadata and computes content hash
2. Worker calls `cacheManager.Get(fileHash)` to check for existing analysis
3. On cache hit, cached analysis is returned immediately (no API call)
4. On cache miss, worker calls `semanticAnalyzer.Analyze(metadata)`
5. Result is stored via `cacheManager.Set(&CachedAnalysis{...})`
6. Subsequent files with identical content skip analysis

**Cache Structure**:
Cached entries are stored as JSON files in `~/.memorizer/.cache/summaries/` with filenames derived from the first 16 characters of the content hash and version information. Each entry contains:
- Schema version, metadata version, semantic version (for staleness detection)
- File path (for reference, not cache key)
- Content hash (actual cache key)
- Analysis timestamp
- Complete file metadata
- Semantic analysis results (including entities and references)
- Error message (if analysis failed)

**Performance Impact**:
The cache achieves high hit rates (often >95%) in typical usage patterns where files are frequently re-indexed without content changes. This dramatically reduces Claude API calls, lowers costs, and accelerates index rebuilds. Files that change content receive fresh analysis while unchanged files reuse cached results instantly.

**Cache Invalidation**:
Content-based hashing provides automatic cache invalidation. When a file's content changes, its hash changes, resulting in a cache miss and triggering new analysis. The cache contains no explicit invalidation or expiration logic since the content hash inherently tracks file state.

**Cache Versioning**:
The cache implements a three-tier versioning system to detect when cached entries become stale due to changes in extraction logic, analysis prompts, or data structures. Each cache entry includes three version fields that are checked during cache lookups:

**Version Tiers**:
- **SchemaVersion** (current: 1) - Tracks changes to CachedAnalysis structure itself (adding/removing/renaming fields, changing field types, altering cache storage format)
- **MetadataVersion** (current: 1) - Tracks changes to metadata extraction logic (new FileMetadata fields, extraction algorithm changes, handler updates, categorization changes)
- **SemanticVersion** (current: 1) - Tracks changes to semantic analysis logic (prompt template updates, new SemanticAnalysis fields, analysis routing changes, entity/reference extraction updates)

**Cache Key Format**:
Cache filenames include version information: `{hash[:16]}-v{schema}-{metadata}-{semantic}.json` (e.g., `sha256:abc12345-v1-1-1.json`). Legacy entries from before versioning use the format `{hash[:16]}.json` and are treated as version 0.0.0.

**Staleness Detection**:
During cache lookups, the cache manager evaluates staleness using the following rules:
- Schema version mismatch (any direction) = always stale (incompatible structure)
- Metadata version behind current = stale (missing newer metadata fields)
- Semantic version behind current = stale (outdated analysis prompts or logic)
- Future versions (newer than current) = not stale (forward compatible)

When a stale entry is detected, the file is re-analyzed with current logic and the cache entry is updated with new version numbers. This versioning system ensures that application upgrades automatically trigger re-analysis of files that would benefit from newer extraction or analysis capabilities.

**Version Bump Scenarios**:
Developers increment version constants in `internal/cache/version.go` when making changes:
- Bump SchemaVersion when modifying CachedAnalysis struct or cache storage format
- Bump MetadataVersion when changing metadata extraction handlers or adding FileMetadata fields
- Bump SemanticVersion when updating prompts, adding entity/reference fields, or changing analysis routing

The `cache status` and `cache clear --old-versions` commands help manage versioned cache entries.

### Type System

The Semantic Analyzer populates the `SemanticAnalysis` structure defined in the Type System (`pkg/types/types.go`), which represents the structured understanding produced by Claude API analysis.

**SemanticAnalysis Structure**:
- `Summary` (string) - 2-3 sentence description of file content and purpose
- `Tags` ([]string) - 3-5 semantic tags in lowercase hyphenated format (e.g., "project-documentation", "api-design")
- `KeyTopics` ([]string) - 3-5 main themes or subjects covered in the file
- `DocumentType` (string) - Genre or category classification (e.g., "technical-documentation", "configuration", "test-code")
- `Confidence` (float64) - Analysis confidence score from 0.0 to 1.0 indicating reliability
- `Entities` ([]Entity) - Named entities extracted from content, each with `name` (string) and `type` (technology, person, concept, organization, project)
- `References` ([]Reference) - Topic dependencies and relationships, each with `topic` (string), `type` (requires, extends, related-to, implements), and `confidence` (float64)

**Integration with Index Entries**:
The `SemanticAnalysis` structure is embedded in `IndexEntry` as an optional pointer field:
```
IndexEntry {
    Metadata FileMetadata
    Semantic *SemanticAnalysis (optional, nil when analysis disabled or failed)
    Error *string (optional, contains error message if analysis failed)
}
```

**Presence Logic**:
The `Semantic` field is nil in three cases:
1. Semantic analysis is disabled via `analysis.enabled = false` configuration
2. Analysis failed with an error (error message stored in `Error` field)
3. File was processed before semantic analysis was implemented

**Confidence Scoring**:
Confidence scores indicate analysis reliability:
- **0.5**: Explicitly set for fallback analysis synthesized from metadata only (binary files, extraction failures) at `analyzeBinary()` line 455
- **0.0**: Default value for Claude API responses (Go's zero value for float64)

**Important Note**: The current implementation does NOT request confidence scores from Claude in its prompts. None of the analysis prompts (lines 99-104, 166-171, 289-294, 410-415, 470-475) ask Claude to provide a confidence field. As a result, successful Claude analyses return with Confidence=0.0 (the zero value) unless Claude spontaneously includes a confidence field in the JSON response.

Downstream consumers should be aware that confidence scores are only meaningful for binary fallback analyses (0.5). The absence of a confidence request in prompts means this field doesn't reliably distinguish between successful Claude analyses and other scenarios.

## Glossary

**Semantic Analysis**: AI-powered extraction of meaning, purpose, and themes from file content, producing structured understanding beyond basic metadata (summary, tags, topics, document type).

**Vision Analysis**: Using Claude's multimodal capabilities to understand visual content in images, diagrams, screenshots, and charts by analyzing pixel data with base64-encoded image content blocks.

**Content-Based Routing**: Strategy selection based on file type and metadata characteristics, matching file properties to optimal Claude API capabilities (text, vision, document blocks).

**Exponential Backoff**: Retry strategy that doubles wait time between retry attempts (1s, 2s, 4s) for up to 4 total attempts (1 initial + 3 retries), preventing API overload while handling transient failures gracefully.

**Rate Limiting**: Token bucket algorithm controlling Claude API call frequency to maintain quota compliance (default 20 calls/minute). **Note**: Rate limiting is implemented in the Daemon subsystem's worker pool (`internal/daemon/worker_pool.go`), not in the Semantic Analyzer subsystem. The worker pool calls `rateLimiter.Wait(ctx)` before invoking the analyzer on cache misses. The analyzer itself has no rate limiting logic.

**Content Truncation**: Limiting analyzed content to fixed sizes (100KB for text, 50KB for extracted document text) to manage API token usage and control costs.

**Document Content Block**: Claude API message content type for native PDF understanding, allowing Claude to read PDF structure, text, and layout directly without external parsing.

**Image Content Block**: Claude API message content type for vision analysis, sending base64-encoded image data with MIME type for visual understanding and diagram interpretation.

**Office Open XML Extraction**: Treating DOCX/PPTX as ZIP archives and parsing internal XML files to extract text content for semantic analysis without specialized Office libraries.

**Graceful Degradation**: Fallback to metadata-only analysis when semantic analysis fails, ensuring all files receive structured index entries with at least baseline information.

**JSON Extraction**: Parsing JSON responses from Claude, including fallback to extract JSON from markdown code blocks when Claude wraps responses in markdown formatting.

**Analysis Strategy**: Content-type-specific approach to preparing and analyzing files (text reading, image encoding, document extraction, PDF handling, binary fallback).

**Content Hash**: SHA-256 hash of file content used as cache key, enabling cache hits when files move or rename as long as content remains unchanged.

**Retry Logic**: Error handling pattern that attempts failed API requests multiple times with increasing delays, distinguishing between transient failures (retry) and permanent errors (fail immediately).

**Confidence Score**: Numeric value from 0.0 to 1.0 indicating reliability of semantic analysis. Currently, 0.5 is explicitly set for metadata-only fallback (binary files), while Claude API responses default to 0.0 since confidence is not requested in prompts. This field does not currently provide reliable distinction between successful and unsuccessful Claude analyses.

**Entity Extraction**: Process of identifying and extracting named entities (technologies, people, organizations, concepts, projects) from file content. All analysis strategies request entities as the 5th field in prompts. Entities consist of a name and type classification, enabling technology dependency tracking and subject matter expert identification.

**Entity Types**: Five classifications for extracted entities: technology (programming languages, frameworks, tools), person (individuals mentioned or depicted), concept (abstract ideas or methodologies), organization (companies or groups), project (specific software projects or initiatives).

**Reference Extraction**: Process of identifying topic dependencies and relationships within file content. All analysis strategies request references as the 6th field in prompts. References capture how files relate to, require, extend, or implement other concepts, building a knowledge dependency graph.

**Reference Types**: Four classifications for extracted references: requires (file depends on the topic), extends (file builds upon the topic), related-to (general connection to the topic), implements (file provides implementation of the topic). Each reference includes a confidence score from 0.0 to 1.0.

**Cache Versioning**: Three-tier versioning system (SchemaVersion, MetadataVersion, SemanticVersion) that tracks cache entry compatibility with current code. Version mismatches or outdated versions trigger automatic re-analysis during cache lookups. Cache keys include version information in format `{hash[:16]}-v{schema}-{metadata}-{semantic}.json`.

**Schema Version**: Cache version tier tracking CachedAnalysis structure changes (field additions, removals, renames, type changes). Schema version mismatches in either direction indicate incompatible cache entries that must be re-analyzed.

**Metadata Version**: Cache version tier tracking metadata extraction logic changes (new FileMetadata fields, handler updates, categorization changes). Metadata versions behind current trigger re-analysis to capture newer metadata fields.

**Semantic Version**: Cache version tier tracking semantic analysis logic changes (prompt updates, new SemanticAnalysis fields, routing changes, entity/reference extraction updates). Semantic versions behind current trigger re-analysis with updated prompts.

**Token Budget**: Maximum number of tokens Claude can use in response (default 1500), controlling response length and API costs while ensuring complete analysis output.
