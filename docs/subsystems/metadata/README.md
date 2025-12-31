# Metadata Extraction

Fast, deterministic file metadata extraction with handler pattern for 9 file type categories and 26+ file extensions.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-31

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Metadata Extraction subsystem provides the first phase of Agentic Memorizer's three-phase file processing pipeline. Before semantic analysis can understand file content, this subsystem quickly extracts file-specific metadata: dimensions for images, word counts for documents, slide counts for presentations, durations for transcripts, and language detection for code files. The extraction is fast, deterministic, and independent of AI providers.

The subsystem implements a handler/adapter pattern with a registry-based extension lookup. Each handler implements a FileHandler interface for a specific file category, registering its supported extensions with the main Extractor. This design enables O(1) extension-to-handler lookup, easy addition of new file types without core changes, and graceful degradation when handlers encounter errors.

Key capabilities include:

- **9 file categories** - Documents, presentations, images, transcripts, data, code, videos, audio, archives, other
- **26+ file extensions** - Markdown, Office docs, PDFs, images, code files, JSON/YAML, VTT transcripts
- **Handler pattern** - FileHandler interface with CanHandle, Extract, and SupportedExtensions methods
- **Registry pattern** - O(1) extension-to-handler lookup via hash map
- **Graceful degradation** - Returns base metadata when handlers fail, never blocks pipeline
- **Content hash integration** - SHA-256 hash computed for cache key generation
- **Readability classification** - Identifies which files Claude Code can read directly

## Design Principles

### Fast and Deterministic

Metadata extraction operates without external dependencies or AI providers. All extraction uses Go standard library functionality: image.DecodeConfig for image dimensions, bufio.Scanner for line counting, archive/zip for Office documents. This ensures consistent, reproducible results regardless of network state or provider availability. The extraction phase typically completes in milliseconds.

### Handler/Adapter Pattern

Each file type category has a dedicated handler struct implementing the FileHandler interface. Handlers encapsulate type-specific extraction logic while exposing a uniform interface. The MarkdownHandler knows how to parse headings and count words; the ImageHandler knows how to decode image dimensions; the CodeHandler knows how to detect programming languages. New file types are added by implementing the interface without modifying core extraction logic.

### Registry with O(1) Lookup

The Extractor maintains an extension-to-handler map populated at construction. When a file arrives for extraction, the Extractor looks up the handler by extension in constant time. If no handler exists, base metadata is returned. This avoids nested conditionals or switch statements and scales efficiently with handler count.

### Graceful Degradation

Handler extraction failures never block the processing pipeline. If a handler encounters an error (corrupted ZIP archive, malformed image header, unreadable file), the Extractor returns base metadata with category classification but without type-specific fields. This ensures files proceed to semantic analysis even when metadata extraction partially fails.

### Separation from Semantic Analysis

Metadata extraction is intentionally separated from semantic analysis. Metadata (file size, dimensions, word count) is structural and deterministic; semantic analysis (summaries, tags, topics) requires AI interpretation. This separation enables efficient caching (metadata can be computed quickly for hash generation), parallel processing (metadata extraction doesn't consume rate limiter tokens), and reliability (metadata always available even when AI providers are down).

### Readability Classification

The subsystem classifies files by whether Claude Code can read them directly. Text files, code files, and images are readable; Office documents and PDFs require extraction or conversion. This classification flows to the file index where it informs integration hooks and MCP tools about file accessibility.

## Key Components

### Extractor (`extractor.go`)

The Extractor struct is the main entry point for metadata extraction. It holds a map of extensions to handlers and provides the Extract method. Construction via NewExtractor registers all built-in handlers. The categorizeFile function maps extensions to one of 9 categories. The isReadable function determines Claude Code accessibility. Extract looks up the handler, calls it if found, and returns either type-specific metadata or base metadata with category.

### FileHandler Interface

The FileHandler interface defines three methods all handlers must implement. Extract performs the actual metadata extraction given a file path and os.FileInfo. CanHandle returns true if the handler processes a given extension. SupportedExtensions returns the list of extensions this handler manages. The interface enables polymorphic handling while maintaining type-specific logic in each implementation.

### MarkdownHandler (`markdown.go`)

Handles .md and .markdown files. Extracts word count via string splitting and sections via heading detection (lines starting with #). Uses bufio.Scanner for efficient line-by-line reading. Returns FileMetadata with WordCount and Sections populated.

### CodeHandler (`code.go`)

Handles 10 programming language extensions: .go, .py, .js, .ts, .java, .c, .cpp, .rs, .rb, .php. Extracts language name via internal extension-to-language map. Counts lines (stored in WordCount field for compatibility). Language detection is instantaneous via map lookup.

### ImageHandler (`image.go`)

Handles 5 image formats: .png, .jpg, .jpeg, .gif, .webp. Extracts dimensions (width and height) using Go's image.DecodeConfig which reads only the image header without loading the full file. Supports WebP via golang.org/x/image/webp. Returns FileMetadata with Dimensions populated.

### PDFHandler (`pdf.go`)

Handles .pdf files. Currently provides minimal extraction (base metadata only) as PDF content parsing is deferred to semantic analysis where Claude or GPT-4 can process PDF document blocks. Future enhancement may add page count via external library.

### DocxHandler (`docx.go`)

Handles .docx files. Delegates to internal/document package for Office file operations. Extracts word count from word/document.xml text runs and author from docProps/core.xml. Returns FileMetadata with WordCount and Author populated.

### PptxHandler (`pptx.go`)

Handles .pptx files. Delegates to internal/document package for Office file operations. Counts slides by enumerating ppt/slides/slide*.xml files. Extracts author from core properties. Provides ExtractText for semantic analysis integration. Returns FileMetadata with SlideCount and Author populated.

### JSONHandler (`json.go`)

Handles .json, .yaml, .yml, .toml files. Returns base metadata as structure and semantics rely on AI analysis. Category is "data" and files are marked readable for direct Claude Code consumption.

### VTTHandler (`vtt.go`)

Handles .vtt and .srt subtitle files. Extracts duration from timestamp patterns (HH:MM:SS --> HH:MM:SS format). Uses regex matching to find the last timestamp in the file. Returns FileMetadata with Duration populated as a string.

### Document Package (`internal/document/`)

Provides shared utilities for Office file handling. OpenOfficeFile opens DOCX/PPTX as ZIP archives. ExtractTextFromTags parses XML to extract text runs from specific tags. ReadZipFile and FindFileInZip provide ZIP navigation. DocxMetadata and PptxMetadata structs hold extracted fields. This package is used by DocxHandler and PptxHandler.

### FileMetadata Structure (`pkg/types/`)

FileMetadata contains all extracted metadata fields. FileInfo holds base fields (Path, Hash, Size, Modified, Type, Category, IsReadable). Type-specific optional fields include WordCount, PageCount, SlideCount, Dimensions, Duration, Sections, Language, and Author. All type-specific fields are pointers to indicate presence/absence.

## Integration Points

### Daemon Worker Pool

The daemon's worker pool calls the Extractor for every file before semantic analysis. The pool creates the Extractor via NewExtractor at initialization and holds it as a field. When processing a file event, the worker calls Extract to get metadata, computes the content hash, and uses these for cache key generation and semantic provider input.

### Cache Subsystem

Metadata extraction produces the content hash used as the primary cache key component. The cache stores semantic analysis results keyed by content hash and version numbers. When a file changes, new metadata extraction produces a new hash, causing a cache miss and triggering fresh semantic analysis.

### Semantic Analysis

Semantic analysis providers receive FileMetadata as input. The metadata helps providers understand file context: language for code, dimensions for images, slide count for presentations. Some handlers (DocxHandler, PptxHandler) provide ExtractText methods that semantic providers call for content extraction from binary formats.

### File Index and Integration Hooks

The FileMetadata.IsReadable field flows to the file index and informs integration hooks. SessionStart hooks use this to indicate which files Claude Code can read directly versus which require extraction. MCP tools can filter or annotate results based on readability.

### Knowledge Graph

File metadata fields (Category, Author) are stored in the knowledge graph. Category determines the IN_CATEGORY edge. Author could potentially create entity relationships. The graph stores file metadata alongside semantic analysis results.

## Glossary

**Category**
One of 9 classifications for files: documents, presentations, images, transcripts, data, code, videos, audio, archives, or other. Determined by file extension mapping.

**Content Hash**
SHA-256 hash of file contents used for content-addressable caching. Enables cache hits across file renames and automatic invalidation on content changes.

**Dimensions**
Width and height in pixels for image files. Extracted via image.DecodeConfig without loading the full image.

**Extractor**
The main struct orchestrating metadata extraction. Holds handler registry and provides the Extract method.

**FileHandler**
The interface all metadata handlers implement. Defines Extract, CanHandle, and SupportedExtensions methods.

**FileMetadata**
The struct containing all extracted metadata. Includes base FileInfo plus optional type-specific fields like WordCount, Dimensions, and Language.

**Graceful Degradation**
The pattern of returning base metadata when handler extraction fails, ensuring the processing pipeline continues.

**Handler**
A struct implementing FileHandler for a specific file category. Examples: MarkdownHandler, ImageHandler, CodeHandler.

**Office File**
DOCX or PPTX files that are ZIP archives containing XML. The document package provides utilities for opening and parsing these formats.

**Readable**
Classification indicating Claude Code can read a file directly. True for text, code, and images; false for Office docs and PDFs.

**Registry**
The extension-to-handler map in Extractor enabling O(1) handler lookup by file extension.

**Sections**
Heading names extracted from Markdown files. Detected by lines starting with # prefix.

**Type-Specific Fields**
Optional FileMetadata fields that only certain file types populate: WordCount, PageCount, SlideCount, Dimensions, Duration, Sections, Language, Author.
