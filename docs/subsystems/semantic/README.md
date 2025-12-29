# Semantic Analysis

Multi-provider AI-powered content understanding with intelligent content routing, shared prompts, and graceful fallbacks.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Semantic Analysis subsystem provides the second phase of Agentic Memorizer's three-phase file processing pipeline. After metadata extraction, this subsystem uses AI providers (Claude, OpenAI, Gemini) to understand file content and produce structured semantic output: summaries, tags, topics, entities, and references. The subsystem abstracts provider differences behind a common interface while enabling provider-specific optimizations.

The subsystem implements a provider pattern with registry-based lookup. Each provider implements the Provider interface, registering itself via init functions. Content routing directs files through appropriate analysis methods based on file type and provider capabilities: text analysis for readable files, vision API for images, document blocks for PDFs, and text extraction for Office documents. When analysis cannot proceed, a fallback mechanism produces metadata-only results with reduced confidence.

Key capabilities include:

- **Multi-provider support** - Claude, OpenAI, and Gemini with provider-specific optimizations
- **Provider interface** - Common contract with Analyze, SupportsVision, and SupportsDocuments methods
- **Content routing** - Intelligent dispatch based on file type and provider capabilities
- **Shared prompts** - Common prompt templates for consistent analysis output
- **Vision support** - Image analysis via provider vision APIs with MIME type detection
- **Document handling** - Native PDF blocks (Claude, Gemini) or text extraction (OpenAI)
- **Graceful fallback** - Metadata-only analysis for unsupported or failed files
- **Rate limiting** - Token bucket integration respecting provider API quotas

## Design Principles

### Provider Abstraction Pattern

All providers implement a common Provider interface with methods for analysis, capability detection, and identification. The interface enables uniform handling by the daemon worker pool while allowing provider-specific implementations. Callers interact with providers through the interface, enabling runtime provider switching via configuration without code changes.

### Registry with Auto-Registration

The provider registry uses a singleton pattern with sync.Once initialization. Providers register factory functions via init calls when their packages are imported. The daemon imports all provider packages with blank imports, triggering registration on startup. This enables automatic discovery without hardcoded provider lists and supports future provider additions without core changes.

### Content-Aware Routing

Each provider routes content through appropriate analysis methods based on file type and capabilities. Images route to vision APIs when supported. PDFs route to native document blocks (Claude, Gemini) or fall back to metadata-only (OpenAI). Office documents (PPTX, DOCX) extract text first. Text files use standard analysis. Binary files use metadata-only fallback. This routing maximizes provider capabilities while maintaining consistent output structure.

### Shared Prompt Templates

The common package provides prompt templates used by all providers. Templates are parameterized for different content types (text, image, document) and include consistent output schema expectations. This ensures all providers produce the same output structure (summary, tags, topics, entities, references) regardless of implementation differences.

### Response Parsing with Fallback

Provider responses undergo JSON extraction that handles both raw JSON and code-block-wrapped responses. If parsing fails after attempting multiple extraction patterns, the error propagates clearly. This accommodates variations in how different models format JSON output while maintaining strict output typing.

### Graceful Degradation

When content cannot be analyzed (unsupported binary, extraction failure, provider error), the fallback mechanism produces a SemanticAnalysis with metadata-derived values and 0.5 confidence. This ensures the processing pipeline continues and files remain indexed even when AI analysis fails. The reduced confidence signals downstream consumers about analysis quality.

### Provider-Specific Optimizations

Each provider implementation leverages its unique capabilities. Claude uses native document content blocks for efficient PDF handling. Gemini uses multimodal blobs for native PDF and image processing. OpenAI uses data URL encoding for vision. These optimizations improve analysis quality and efficiency while the interface provides consistent abstractions.

## Key Components

### Provider Interface (`provider.go`)

The Provider interface defines the contract for semantic analysis implementations. Analyze performs content understanding and returns SemanticAnalysis. Name returns the provider identifier (claude, openai, gemini). Model returns the specific model being used. SupportsVision indicates image analysis capability. SupportsDocuments indicates native PDF handling capability. All providers must implement this interface.

### Provider Registry (`registry.go`)

The Registry struct manages provider registration and lookup. GlobalRegistry returns the singleton instance via sync.Once. Register stores provider factory functions keyed by name. Get retrieves factory functions for instantiation. List returns all registered provider names. Thread safety is ensured via RWMutex. Factory functions enable deferred instantiation with runtime configuration.

### Provider Configuration (`provider_config.go`)

ProviderConfig contains shared configuration for all providers: APIKey for credentials, Model for selection, MaxTokens for response limits, Timeout in seconds, EnableVision flag, and MaxFileSize limit. Each provider extracts the fields it needs. This shared structure enables uniform configuration handling across providers.

### Claude Provider (`providers/claude/`)

The Claude provider implements full semantic analysis with vision and native document support. It uses a custom HTTP client with exponential backoff retry logic (max 3 retries, handles 429/500/502/503/504). Images are base64 encoded for vision API. PDFs use native document content blocks, the most efficient handling. Office documents extract text via the document package. Text is truncated at 100KB (text files) or 50KB (documents).

### OpenAI Provider (`providers/openai/`)

The OpenAI provider uses the external go-openai client. Vision is supported for GPT-4o, GPT-5.x, and GPT-4-Vision models via data URL encoding. PDFs fall back to metadata-only as OpenAI lacks native document handling. Office documents extract text. Vision capability is detected by model name patterns. The provider integrates with OpenAI's chat completion API.

### Gemini Provider (`providers/gemini/`)

The Gemini provider uses the external generative-ai-go client. It supports native multimodal handling for both images and PDFs using genai.Blob with raw binary data. Vision is supported for Gemini 2.x, 3.x, 1.5, and Pro Vision models. Office documents extract text. Response parsing extracts text from genai.Part arrays. Native PDF handling provides efficient analysis without text extraction.

### Common Prompts (`common/prompts.go`)

Three prompt templates handle different content types. TextAnalysisPromptTemplate formats prompts for text files with content. ImageAnalysisPromptTemplate formats prompts for images with dimension metadata. DocumentAnalysisPromptTemplate formats prompts for PDFs and Office documents with document-specific metadata. Builder functions (BuildTextPrompt, BuildImagePrompt, BuildPptxPrompt, BuildDocxPrompt, BuildPdfPrompt) fill templates with file metadata.

### Response Parsing (`common/response.go`)

ParseAnalysisResponse converts LLM text output to typed SemanticAnalysis. It first attempts parsing as raw JSON. If that fails, it extracts JSON from code blocks (```json or ```) and parses again. This handles variation in how models format JSON responses. Parse errors propagate with descriptive messages.

### Media Helpers (`common/media.go`)

GetMediaType maps file extensions to MIME types for vision APIs. Supported formats include PNG, JPEG, GIF, WebP, and Apple HEIC/HEIF. Unknown extensions default to image/jpeg. This enables correct content-type headers for vision API requests.

### Fallback Analysis (`common/fallback.go`)

AnalyzeBinary produces metadata-only SemanticAnalysis for unsupported files. The summary derives from file type and category (e.g., "PDF FILE with 50 pages"). Tags include category and type. Confidence is 0.5 to indicate reduced quality. This ensures all files produce some analysis even when AI processing fails.

### SemanticAnalysis Structure

The output structure contains: Summary (2-3 sentences), Tags (3-5 lowercase hyphenated terms), KeyTopics (3-5 main themes), DocumentType (purpose classification), Confidence (0.0-1.0 score), Entities (named entities with type), and References (topic relationships with type and confidence). This consistent structure enables downstream processing regardless of which provider generated it.

## Integration Points

### Daemon Worker Pool

The daemon worker pool calls the provider's Analyze method for each file. The worker first checks the cache (provider-specific subdirectory). On cache miss, it waits for a rate limiter token, calls Analyze, caches the result, and returns. The provider instance is held by the pool and created at daemon startup using the registry's factory function.

### Rate Limiter

The daemon's rate limiter controls API call frequency. Workers call rateLimiter.Wait before invoking Analyze. Rate limits are provider-specific defaults (Claude: 20/min, OpenAI: 60/min, Gemini: 100/min) and configurable via semantic.rate_limit_per_min. Rate limits are hot-reloadable via config reload without daemon restart.

### Cache Subsystem

Semantic analysis results are cached in provider-specific subdirectories under ~/.memorizer/cache/. Cache keys combine content hash and version numbers. Provider name is part of the cache path, enabling cache isolation between providers. Cache hits skip API calls entirely. Provider changes invalidate relevant cache entries.

### Metadata Subsystem

Semantic analysis receives FileMetadata as input. Metadata provides file identity (path, hash), classification (type, category), and type-specific fields (dimensions, word count, slide count). The metadata informs prompt construction and content routing decisions.

### Document Package

Office document text extraction uses internal/document for PPTX and DOCX files. ExtractPptxText reads all slides. ExtractDocxText reads all paragraphs. Extracted text is truncated to 50KB before analysis. Empty extraction results trigger metadata-only fallback.

### Configuration System

Provider selection and configuration come from the semantic config section: provider name, API key, model, max_tokens, timeout, enable_vision, max_file_size, and rate_limit_per_min. Most settings are hot-reloadable. API keys can come from environment variables (ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY).

### Knowledge Graph

SemanticAnalysis output flows to the knowledge graph. Tags create HAS_TAG edges. Topics create COVERS_TOPIC edges. Entities create MENTIONS edges with type preservation. The summary is stored on the File node for full-text search. Entity types enable typed entity search.

## Glossary

**Confidence**
A score from 0.0 to 1.0 indicating analysis quality. AI-generated analysis has 1.0 confidence. Metadata-only fallback has 0.5 confidence. Downstream consumers use this to weight results.

**Content Routing**
The process of selecting the appropriate analysis method based on file type and provider capabilities. Routes include text analysis, vision API, document blocks, text extraction, and metadata-only fallback.

**Document Blocks**
Native handling of PDFs where the document is sent as a content block rather than extracted text. Supported by Claude and Gemini, providing higher quality analysis.

**Entity**
A named thing identified in content with a type classification: technology, person, concept, organization, or project. Entities flow to the knowledge graph as MENTIONS relationships.

**Fallback Analysis**
Metadata-only analysis produced when AI analysis cannot proceed. Contains derived summary, basic tags, and 0.5 confidence. Ensures pipeline continuity.

**Factory Function**
A function that creates a provider instance given configuration. Stored in registry and invoked at runtime to create the configured provider.

**Multimodal**
Provider capability to process multiple content types (text, images, documents) in a single request. Gemini supports native multimodal for images and PDFs.

**Provider**
An implementation of semantic analysis using a specific AI service (Claude, OpenAI, Gemini). Providers implement the Provider interface.

**Reference**
A topic relationship identified in content. Includes topic name, relationship type (requires, extends, related-to, implements), and confidence score.

**Registry**
The singleton managing provider factory registration and lookup. Uses sync.Once for initialization and RWMutex for thread safety.

**SemanticAnalysis**
The structured output of semantic analysis containing summary, tags, key topics, document type, confidence, entities, and references.

**Text Extraction**
The process of extracting readable text from binary formats (PPTX, DOCX) for analysis. Uses the document package. Extracted text is truncated before analysis.

**Vision API**
Provider capability to analyze images directly. Supported by all three providers with different encoding methods (base64, data URL, binary blob).
