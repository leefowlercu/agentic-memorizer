# Office Document Processing

Shared utilities for Microsoft Office file extraction including ZIP archive handling, XML text extraction, and format-specific metadata parsing.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-31

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Office Document Processing subsystem provides shared utilities for extracting text content and metadata from Microsoft Office files (DOCX and PPTX). Office documents are ZIP archives containing XML files with structured content, and this subsystem provides the low-level operations needed to navigate and parse these archives. The subsystem serves both the metadata extraction pipeline (for word counts, slide counts, author information) and the semantic analysis pipeline (for text extraction before AI analysis).

The subsystem is deliberately focused on the common operations needed across multiple consumers: opening Office files as ZIP archives, locating specific files within the archive, reading file contents, and extracting text from XML tags. Format-specific logic (DOCX vs PPTX) is organized into dedicated files while sharing the common ZIP and XML utilities.

Key capabilities include:

- **ZIP archive handling** - Opens Office files and navigates their internal structure
- **XML text extraction** - Extracts text content from XML tags using string parsing
- **DOCX processing** - Word count calculation and author extraction from Word documents
- **PPTX processing** - Slide counting, author extraction, and text aggregation from presentations
- **Core properties parsing** - Extracts document metadata from docProps/core.xml

## Design Principles

### Office Open XML Foundation

Microsoft Office documents (DOCX, PPTX, XLSX) use the Office Open XML format where files are ZIP archives containing XML documents. The subsystem embraces this by treating all Office files as ZIP archives first, then navigating to the relevant XML files. This enables a unified approach across different Office formats.

### Shared Utilities Pattern

Common operations (ZIP reading, XML text extraction) are implemented once in the office.go file and reused by format-specific code. This eliminates duplication between DOCX and PPTX handling while keeping format-specific logic separate. New Office formats can be added by implementing format-specific functions using the shared utilities.

### String-Based XML Parsing

Rather than using full XML DOM parsing for text extraction, the subsystem uses efficient string-based tag matching. The ExtractTextFromTags function finds opening and closing tags by string position, extracting content between them. This approach is faster for the specific use case of extracting text runs and avoids the overhead of building a complete DOM tree.

### Structured XML for Metadata

While text extraction uses string parsing, metadata extraction uses Go's encoding/xml package for proper unmarshaling. The core.xml properties file uses a predictable schema where XML unmarshaling provides cleaner, type-safe access to fields like creator (author).

### Graceful Handling of Missing Content

Office documents may have varying internal structures depending on how they were created. The subsystem handles missing files gracefully: if word/document.xml doesn't exist in a DOCX, text extraction returns an empty string rather than an error. This resilience ensures the processing pipeline continues even with unusual documents.

### Caller-Managed Resources

The OpenOfficeFile function returns a zip.ReadCloser that callers must close. This follows Go conventions for resource management and allows callers to perform multiple operations on the same archive without repeatedly opening and closing the file.

## Key Components

### OpenOfficeFile Function

The OpenOfficeFile function opens an Office file as a ZIP archive using Go's archive/zip package. Returns a zip.ReadCloser that provides access to the archive's file list. Callers are responsible for closing the reader when done.

### ExtractTextFromTags Function

The ExtractTextFromTags function extracts text content from XML data using a tag prefix. For example, ExtractTextFromTags(data, "w:t") extracts content from all `<w:t>...</w:t>` elements. Uses iterative string searching rather than XML parsing for efficiency. Returns concatenated text with spaces between elements.

### ReadZipFile Function

The ReadZipFile function reads the contents of a file within a ZIP archive, returning the complete byte slice. Opens the file within the archive, reads all content, and closes the reader.

### FindFileInZip Function

The FindFileInZip function locates a file in a ZIP archive by exact path name. Returns the zip.File pointer if found, nil otherwise. Used to find specific XML files like word/document.xml or docProps/core.xml.

### FindFilesWithPrefix Function

The FindFilesWithPrefix function finds all files matching a prefix and suffix pattern within a ZIP archive. Returns a slice of matching zip.File pointers. Used for finding numbered files like ppt/slides/slide1.xml, slide2.xml, etc.

### DocxMetadata Struct

The DocxMetadata struct holds extracted DOCX metadata: WordCount (number of words in the document) and Author (creator from core properties).

### PptxMetadata Struct

The PptxMetadata struct holds extracted PPTX metadata: SlideCount (number of slides) and Author (creator from core properties).

### ExtractDocxText Function

The ExtractDocxText function extracts all text from a DOCX file by opening the archive, reading word/document.xml, and extracting text from `<w:t>` tags (Word text runs). Returns the concatenated text content.

### ExtractDocxMetadata Function

The ExtractDocxMetadata function extracts metadata from a DOCX file: word count from document content and author from docProps/core.xml. Returns a populated DocxMetadata struct.

### ExtractPptxText Function

The ExtractPptxText function extracts all text from a PPTX file by iterating through all slide files (ppt/slides/slide*.xml) and extracting text from `<a:t>` tags (PowerPoint text runs). Returns concatenated text with double newlines between slides.

### ExtractPptxMetadata Function

The ExtractPptxMetadata function extracts metadata from a PPTX file: slide count from the number of slide files and author from docProps/core.xml. Returns a populated PptxMetadata struct.

### extractCreatorFromCoreProps Function

The extractCreatorFromCoreProps function parses the creator field from core.xml data using XML unmarshaling. Used by both DOCX and PPTX metadata extraction to get the document author.

## Integration Points

### Metadata Extraction Subsystem

The metadata subsystem's DocxHandler and PptxHandler call ExtractDocxMetadata and ExtractPptxMetadata respectively. These functions provide word counts, slide counts, and author information that become part of the FileMetadata structure used throughout the processing pipeline.

### Semantic Analysis Providers

All three semantic providers (Claude, OpenAI, Gemini) use ExtractDocxText and ExtractPptxText to convert Office documents to plain text before analysis. Since these providers analyze text content, the extraction step converts binary Office formats into text that can be included in prompts.

### Content Routing

The semantic analysis subsystem routes DOCX and PPTX files through text extraction before analysis. This is necessary because AI providers cannot directly process Office binary formats. The extracted text is then truncated (typically to 50KB) and included in analysis prompts.

### Cache Key Generation

Metadata extraction happens before caching decisions. The word count and slide count from this subsystem become part of the file metadata used in cache versioning, though the primary cache key is the content hash.

## Glossary

**Core Properties**
The docProps/core.xml file within an Office document containing metadata like creator, title, subject, and dates. Follows the Dublin Core metadata standard.

**DOCX**
Microsoft Word document format using Office Open XML. A ZIP archive containing word/document.xml for content and supporting files for styles, media, and relationships.

**Office Open XML**
Microsoft's XML-based file format for Office documents. Files are ZIP archives containing XML documents, relationships, and embedded content.

**PPTX**
Microsoft PowerPoint presentation format using Office Open XML. A ZIP archive containing ppt/slides/slide*.xml files for each slide and supporting content.

**Text Run**
A contiguous span of text with consistent formatting. In Word documents, text runs are `<w:t>` elements. In PowerPoint, they are `<a:t>` elements.

**ZIP Archive**
A compressed file container format. Office Open XML documents are ZIP archives with a specific internal structure defined by the OOXML standard.
