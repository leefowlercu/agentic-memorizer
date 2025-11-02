# Metadata Extractor Subsystem Documentation

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
   - [Handler Pattern](#handler-pattern)
   - [Extensibility](#extensibility)
   - [Graceful Degradation](#graceful-degradation)
   - [Separation of Concerns](#separation-of-concerns)
3. [Key Components](#key-components)
   - [Extractor Orchestrator](#extractor-orchestrator)
   - [Handler Interface](#handler-interface)
   - [File Type Handlers](#file-type-handlers)
4. [Integration Points](#integration-points)
   - [Daemon Subsystem](#daemon-subsystem)
   - [Worker Pool](#worker-pool)
   - [Semantic Analyzer](#semantic-analyzer)
   - [Type System](#type-system)
5. [Glossary](#glossary)

## Overview

The Metadata Extractor subsystem is a file processing component that extracts structural and format-specific metadata from various file types without performing AI analysis. It operates as the first stage of file processing, gathering information like word counts, dimensions, page counts, sections, and other file-type-specific attributes before files are optionally passed to the Semantic Analyzer for AI-powered analysis.

The subsystem implements a handler pattern with specialized extractors for ten distinct file type categories: documents (Markdown, DOCX, PDF), presentations (PPTX), images (PNG, JPG, GIF, WebP), code files, structured data (JSON, YAML, TOML), transcripts (VTT, SRT), videos, audio, archives, and others. Each handler implements a common interface while providing type-specific extraction logic optimized for its target format.

The Metadata Extractor serves as the foundation for file understanding in Agentic Memorizer, providing consistent baseline metadata for all files while enabling rich type-specific metadata extraction where possible. This separation of metadata extraction from semantic analysis allows the system to efficiently process files, cache results effectively, and provide graceful degradation when AI analysis is unavailable or unnecessary.

## Design Principles

### Handler Pattern

The Metadata Extractor implements a classic Strategy/Handler pattern that provides clean separation between file type detection and metadata extraction logic. The pattern consists of three layers:

1. **Handler Interface**: Defines a contract that all handlers must implement with two methods: `Extract()` for performing the extraction, and `CanHandle()` for determining file type compatibility
2. **Extractor Orchestrator**: Maintains a registry of handlers, routes files to appropriate handlers based on extension matching, and provides fallback behavior for unknown file types
3. **Type-Specific Handlers**: Specialized implementations that understand the structure and format of specific file types

This pattern enables the system to add support for new file types without modifying existing code. New handlers simply implement the interface and register themselves with the orchestrator. The routing logic remains stable while the set of supported file types can grow organically.

### Extensibility

The Metadata Extractor is designed for easy extension. Adding support for a new file type requires only:

1. Creating a new handler struct that implements the `FileHandler` interface
2. Implementing the `Extract()` method with type-specific extraction logic
3. Implementing the `CanHandle()` method to declare supported file extensions
4. Registering the handler with the extractor via `RegisterHandler()`

No changes are required to the orchestrator, existing handlers, or any other subsystems. This extensibility allows the system to evolve its file type support independently from its core architecture. The registry-based approach also enables runtime handler discovery and potential plugin architectures in the future.

### Graceful Degradation

The Metadata Extractor implements a graceful degradation strategy that prioritizes system availability over complete metadata extraction. When a handler fails to extract type-specific metadata (due to file corruption, format variations, or implementation limitations), the extractor:

1. Logs a warning with details about the failure
2. Returns base metadata (path, size, modified time, type, category, isReadable flag)
3. Continues processing without propagating the error

This approach ensures that a single corrupted file or unsupported format variant doesn't prevent the entire index from being built. Users receive baseline metadata for all files, with rich metadata where extraction succeeds. The logged warnings provide visibility into extraction issues without halting the indexing process.

### Separation of Concerns

The Metadata Extractor maintains clear boundaries between three distinct processing stages:

1. **Metadata Extraction**: Gathering structural and format-specific information from files (this subsystem)
2. **Semantic Analysis**: AI-powered understanding of file content and meaning (Semantic Analyzer subsystem)
3. **File Categorization**: Classification of files into high-level categories (built into extractor)

This separation allows each stage to evolve independently. Metadata extraction focuses on fast, deterministic operations that parse file structure. Semantic analysis handles the slower, AI-powered understanding of content. Categorization provides organizational grouping without dictating processing behavior. The boundaries enable efficient caching (metadata can be cached separately from semantic analysis), parallel processing (multiple files can be extracted simultaneously), and graceful degradation (files can be indexed with metadata only when AI analysis is disabled or unavailable).

## Key Components

### Extractor Orchestrator

The Extractor (`internal/metadata/extractor.go`) serves as the central orchestrator and handler registry for the subsystem. It manages the lifecycle of all handlers and routes files to appropriate handlers based on file extension matching.

**Primary Responsibilities:**
- Maintains a registry of all registered handlers indexed by handler type
- Routes files to appropriate handlers based on `CanHandle()` matching
- Provides fallback to base metadata when no handler matches
- Categorizes files into ten high-level categories for organizational purposes
- Determines file readability (whether Claude Code can process the file directly)

**File Categories:**
The orchestrator categorizes files into semantic groupings that guide downstream processing:
- **documents**: Text-based files like Markdown, DOCX, PDF, RTF
- **presentations**: Slide-based formats like PPTX, PPT, Keynote
- **images**: Visual formats like PNG, JPG, GIF, SVG, WebP
- **transcripts**: Time-coded text like VTT and SRT subtitle files
- **data**: Structured data formats like JSON, YAML, TOML, XML
- **code**: Source code files across ten programming languages
- **videos**: Video container formats like MP4, MOV, AVI, MKV
- **audio**: Audio formats like MP3, WAV, OGG, FLAC
- **archives**: Compressed archives like ZIP, TAR, GZ, 7Z
- **other**: Unknown or unsupported file types

**Readability Detection:**
The orchestrator marks files as "readable" when Claude Code can process them directly without intermediate extraction. Readable file types include text formats (Markdown, code, JSON, YAML, XML), images (which Claude's vision capabilities can process), and transcripts. Binary formats like DOCX, PPTX, and archives are marked as not readable since they require extraction before processing.

### Handler Interface

The `FileHandler` interface defines the contract that all metadata extractors must implement. This interface provides the foundation for the handler pattern and enables polymorphic routing of files to appropriate extractors.

**Interface Methods:**
- `Extract(path string, info os.FileInfo) (*types.FileMetadata, error)`: Extracts metadata from a file at the specified path, returning a populated FileMetadata structure or an error if extraction fails
- `CanHandle(ext string) bool`: Returns true if the handler can process files with the given extension (e.g., ".md", ".docx")

**Design Considerations:**
The interface is intentionally minimal to reduce implementation burden on handler authors. The `Extract()` method receives both the file path and `os.FileInfo` to avoid redundant stat calls. Handlers can choose to read the full file, parse only headers, or use specialized libraries based on their file format's characteristics.

### File Type Handlers

The subsystem includes eight specialized handlers, each optimized for specific file format characteristics:

#### MarkdownHandler

Extracts metadata from Markdown and text-based documentation files by parsing the file line-by-line using efficient streaming techniques.

**Extraction Strategy:**
- Uses `bufio.Scanner` for memory-efficient line-by-line processing
- Counts words by splitting lines into whitespace-separated fields
- Identifies headings by detecting lines starting with `#` characters
- Builds section hierarchy based on heading levels

**Metadata Provided:**
- Word count (total words across all lines)
- Section headings (document outline for navigation)

**Supported Extensions:** `.md`, `.markdown`

**Readability:** Marked as readable (Claude Code can process Markdown directly)

#### ImageHandler

Extracts dimensional metadata from image files using Go's standard image decoding capabilities, optimized to avoid loading full image data into memory.

**Extraction Strategy:**
- Uses `image.DecodeConfig()` for efficient dimension extraction without full decode
- Supports standard formats via `image` package (PNG, JPG, GIF)
- Adds WebP support via `golang.org/x/image/webp` package
- Opens file, decodes only the header, and extracts width/height

**Metadata Provided:**
- Image dimensions (width and height in pixels)

**Supported Extensions:** `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`

**Readability:** Marked as readable (Claude's vision capabilities can process images)

#### DocxHandler

Extracts metadata from Microsoft Word documents by treating them as ZIP archives containing XML files, following the Office Open XML (OOXML) specification.

**Extraction Strategy:**
- Opens DOCX file as ZIP archive using standard `archive/zip` library
- Parses `docProps/core.xml` to extract author information
- Parses `word/document.xml` to extract text content
- Extracts text from `<w:t>` XML tags (word text runs)
- Counts words from extracted text using whitespace splitting

**Metadata Provided:**
- Word count (total words in document)
- Author (document creator from core properties)

**Supported Extensions:** `.docx`

**Readability:** Marked as not readable (requires extraction before Claude can process)

**Design Note:** Uses simple XML parsing rather than full DOM parsing to minimize dependencies and improve performance, adequate for metadata extraction purposes.

#### PptxHandler

Extracts metadata from Microsoft PowerPoint presentations using similar ZIP-based extraction as DocxHandler, with additional slide enumeration capabilities.

**Extraction Strategy:**
- Opens PPTX file as ZIP archive
- Counts slide files in `ppt/slides/` directory to determine slide count
- Parses `docProps/core.xml` for author information
- Provides `ExtractText()` method that parses `<a:t>` tags (PowerPoint text runs) from all slide XML files

**Metadata Provided:**
- Slide count (number of presentation slides)
- Author (presentation creator from core properties)

**Additional Capabilities:**
The handler includes an `ExtractText()` method used by the Semantic Analyzer to gather full presentation content for AI analysis. This method is separate from the metadata extraction to maintain clear separation of concerns.

**Supported Extensions:** `.pptx`

**Readability:** Marked as not readable (requires extraction)

#### PDFHandler

Provides a placeholder handler for PDF files that currently returns only base metadata, deferring content extraction to the Semantic Analyzer.

**Extraction Strategy:**
- Returns base metadata only (path, size, modified time, type, category)
- Relies on Semantic Analyzer for content understanding
- Documented as intentional design pending integration with PDF libraries

**Metadata Provided:**
- Base metadata only (no PDF-specific metadata yet)

**Supported Extensions:** `.pdf`

**Readability:** Marked as not readable

**Future Enhancement:** Integration with PDF parsing libraries (pdfcpu or ledongthuc/pdf) could enable page count extraction, table of contents parsing, and metadata reading from PDF properties.

#### CodeHandler

Extracts metadata from source code files across ten programming languages, focusing on line counts and language detection.

**Extraction Strategy:**
- Counts lines in the file using `bufio.Scanner`
- Detects programming language from file extension
- Repurposes the `WordCount` field to store line count (lines are the analogous metric for code)

**Metadata Provided:**
- Line count (stored in `WordCount` field)
- Programming language (detected from extension)

**Supported Languages:**
Go, Python, JavaScript, TypeScript, Java, C, C++, Rust, Ruby, PHP

**Supported Extensions:** `.go`, `.py`, `.js`, `.ts`, `.java`, `.c`, `.cpp`, `.rs`, `.rb`, `.php`

**Readability:** Marked as readable (Claude Code can process source code directly)

**Design Note:** Using `WordCount` for line count avoids adding complexity to the type system while providing an appropriate metric for code files.

#### JSONHandler

Provides minimal handling for structured data files, returning base metadata and relying on Claude Code's ability to read these formats directly.

**Extraction Strategy:**
- Returns base metadata only
- No parsing or schema detection
- Assumes files are text-based and directly readable

**Metadata Provided:**
- Base metadata only

**Supported Extensions:** `.json`, `.yaml`, `.yml`, `.toml`

**Readability:** Marked as readable (text-based structured formats)

**Future Enhancement:** Schema detection, size estimation, or structure analysis could be added without impacting other handlers.

#### VTTHandler

Extracts timing metadata from video transcript files by parsing timestamp formats used in subtitle and caption files.

**Extraction Strategy:**
- Parses VTT/SRT format using regex pattern matching
- Searches for timestamps in format `HH:MM:SS.mmm` or `HH:MM:SS`
- Handles VTT arrow format: `00:00:00 --> 00:05:30`
- Finds the last timestamp in the file to determine total duration

**Metadata Provided:**
- Duration (last timestamp from transcript, indicating video length)

**Supported Extensions:** `.vtt`, `.srt`

**Readability:** Marked as readable (text-based transcript files)

**Use Case:** Helps users understand video length from transcript files when the original video may not be accessible or indexed.

## Integration Points

### Daemon Subsystem

The Daemon subsystem (`internal/daemon/daemon.go`) creates and manages the Metadata Extractor as part of its initialization process. The extractor is instantiated once at daemon startup and shared across all file processing operations.

**Initialization:**
During daemon initialization, a new extractor is created with all handlers automatically registered. The extractor is then passed to the worker pool for use during parallel file processing, and stored as a daemon field for use during incremental file updates triggered by the File Watcher.

**Full Index Build:**
When performing a full index rebuild, the daemon passes the extractor to the worker pool. Each worker thread receives the shared extractor instance and uses it to extract metadata from files in its work queue. The metadata extraction occurs as the first step in the processing pipeline, before file hashing and semantic analysis.

**Incremental Updates:**
When the File Watcher detects a file change, the daemon's event handler directly invokes the extractor to gather updated metadata. This metadata is then hashed and checked against the cache before deciding whether semantic analysis is needed. The incremental path uses the same extractor instance as the full build, ensuring consistency in metadata extraction behavior.

**Processing Flow:**
1. Daemon initializes extractor with all handlers registered
2. File event occurs (either from initial walk or file watcher)
3. Daemon or worker calls `extractor.Extract(path, info)`
4. Extractor routes to appropriate handler based on file extension
5. Handler returns metadata or extractor provides base metadata on failure
6. Metadata flows to next processing stage (hashing, caching, semantic analysis)

### Worker Pool

The Worker Pool subsystem (`internal/daemon/worker_pool.go`) orchestrates parallel file processing and establishes the order of operations for each file. The Metadata Extractor is the first stage in this pipeline.

**Processing Pipeline:**
Each worker executes a three-stage pipeline for every file:

1. **Metadata Extraction**: Worker calls `metadataExtractor.Extract()` to gather file metadata. This is the first and fastest stage, providing baseline information about the file.
2. **File Hashing**: Worker computes a SHA-256 hash of file contents using `cache.HashFile()` for cache key generation. The hash is added to the metadata structure.
3. **Semantic Analysis**: If enabled and not cached, worker calls `semanticAnalyzer.Analyze()` with the metadata. The semantic analyzer uses metadata fields (category, type, size, dimensions, word count) to guide its analysis strategy.

**Parallel Processing:**
Multiple workers share the same extractor instance, which is stateless and thread-safe. The handler pattern ensures that each worker's calls to `Extract()` are independent and don't interfere with concurrent extractions. This parallelism enables efficient processing of large directory trees with thousands of files.

**Error Handling:**
If metadata extraction fails, the worker continues processing with base metadata. If hashing or semantic analysis fails, the worker logs the error and moves to the next file. This graceful degradation ensures that processing continues even when individual files are problematic.

### Semantic Analyzer

The Semantic Analyzer subsystem (`internal/semantic/analyzer.go`) receives metadata from the extractor and uses it to guide AI analysis strategy. The metadata informs decisions about which analysis approach to use and what context to provide to the AI model.

**Routing Logic:**
The analyzer examines metadata fields to determine the appropriate analysis path:
- Files larger than the configured maximum size are rejected before analysis
- Files in the "images" category with vision enabled are routed to image analysis
- Files with type "pptx" or "docx" trigger document analysis with text extraction
- All other readable files undergo standard text analysis

**Context Enrichment:**
The analyzer includes relevant metadata fields in prompts sent to Claude. For example:
- Image dimensions help Claude understand scale and aspect ratio
- Slide counts provide context about presentation structure
- Word counts indicate document length and depth
- File categories guide analysis focus and output format

**Text Extraction:**
For PPTX and DOCX files, the analyzer must re-extract text content since the Metadata Extractor focuses only on metadata. The analyzer uses handler-specific extraction methods (like `PptxHandler.ExtractText()`) or directly parses the Office XML to gather content for AI analysis. This separation avoids circular dependencies while allowing handlers to provide extraction capabilities to other subsystems.

**Caching Integration:**
The metadata's hash field (computed after metadata extraction) serves as the cache key for semantic analysis results. When a file's hash matches a cached entry, the analyzer skips AI analysis entirely. The metadata extraction always occurs (to detect file changes and compute hashes), but the expensive semantic analysis is bypassed when the cache hits.

### Type System

The Type System (`pkg/types/types.go`) defines the data structures used throughout the subsystem and provides the foundation for metadata representation. The Metadata Extractor populates these structures, and all other subsystems consume them.

**Core Structures:**

**FileInfo**: Base metadata for all files, containing path, relative path, hash, size, modification time, type (file extension), category (semantic grouping), and readability flag. Every file in the system has this baseline information.

**FileMetadata**: Extended metadata structure that inherits from FileInfo and adds optional type-specific fields. These fields use pointers to indicate when data is available versus absent:
- `WordCount`: Used by Markdown, DOCX handlers (and repurposed for line count in code files)
- `PageCount`: Reserved for future PDF handler enhancement
- `SlideCount`: Used by PPTX handler
- `Dimensions`: Used by image handler to store width/height
- `Duration`: Used by VTT handler for video length
- `Sections`: Used by Markdown handler for document outline
- `Language`: Used by code handler for programming language
- `Author`: Used by DOCX and PPTX handlers

**Design Rationale:**
The pointer-based optional fields allow the type system to distinguish between "field not applicable" (nil), "extraction failed" (nil with logged warning), and "extraction succeeded" (populated pointer). This three-state model supports graceful degradation and enables consumers to adapt their behavior based on metadata availability.

**Integration Flow:**
1. Extractor creates FileMetadata with base FileInfo fields populated
2. Handler adds type-specific fields based on successful extraction
3. Worker pool adds hash field after extraction completes
4. Semantic analyzer reads metadata to guide analysis strategy
5. Index manager stores complete metadata in the index file
6. Read command retrieves metadata from index for display to users

## Glossary

**Base Metadata**: The minimal set of metadata extracted for all files, including path, size, modified time, type, category, and readability flag. Base metadata is always provided, even when type-specific extraction fails, enabling graceful degradation.

**Handler**: A component implementing the `FileHandler` interface that knows how to extract metadata from a specific file type or category. Handlers encapsulate format-specific parsing logic and shield the orchestrator from file format complexity.

**Handler Pattern**: A software design pattern where different strategies for handling objects are encapsulated in separate handler classes that implement a common interface. The orchestrator routes objects to appropriate handlers without knowing implementation details.

**File Categorization**: The process of mapping file extensions to high-level semantic categories (documents, images, code, transcripts, etc.) for organizational purposes and to guide downstream processing decisions.

**Graceful Degradation**: The pattern of continuing processing with reduced functionality when extraction fails. Rather than halting the entire indexing process, the system falls back to base metadata and logs warnings for troubleshooting.

**IsReadable Flag**: A boolean indicator marking files that Claude Code can process directly without intermediate extraction or conversion. Readable files include text formats, code, images (via vision), and structured data.

**Type-Specific Metadata**: Additional metadata fields beyond base metadata that are populated based on file type. Examples include word count for documents, dimensions for images, slide count for presentations, and duration for transcripts.

**Office Open XML (OOXML)**: The ZIP-based file format specification used by Microsoft Office applications. DOCX and PPTX files are OOXML archives containing XML files that describe document structure and content, allowing extraction via standard ZIP libraries.

**Extraction Strategy**: The specific technical approach a handler uses to gather metadata from its file type. Strategies vary from line-by-line parsing (Markdown) to ZIP extraction (DOCX/PPTX) to library-based decoding (images).

**Handler Registry**: The map of registered handlers maintained by the Extractor orchestrator, used to route files to appropriate handlers based on file extension matching via `CanHandle()` method calls.

**Streaming Processing**: A technique for processing files incrementally without loading entire contents into memory. The Markdown and Code handlers use streaming via `bufio.Scanner` to handle arbitrarily large files efficiently.

**Readability Detection**: The process of determining whether Claude Code can directly process a file's contents. This detection informs integration decisions, cache strategies, and whether intermediate extraction is needed before AI analysis.
