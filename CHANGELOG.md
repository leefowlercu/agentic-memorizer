# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.3] - 2025-10-06

### Fixed
- Config YAML key mismatch preventing API key from being loaded from configuration file (added `yaml` struct tags to match `mapstructure` tags)
- Image semantic analysis failing with media type validation error (image handler now sets specific file extension instead of generic "image" type)

## [0.4.2] - 2025-10-06

### Added
- CHANGELOG.md following Keep a Changelog specification

## [0.4.1] - 2025-10-05

### Added
- Quick Start section in README for 3-minute setup guide
- Example Outputs section showing XML and Markdown format examples with realistic data

### Changed
- Removed XML declaration (`<?xml version="1.0" encoding="UTF-8"?>`) from XML output for cleaner AI consumption
- Updated documentation to reflect XML as default output format

## [0.4.0] - 2025-10-05

### Changed
- **BREAKING**: Default output format changed from Markdown to XML
- Hook commands now use `--format xml --wrap-json` by default
- Updated all documentation and examples to reflect XML as default

### Fixed
- Updated configuration examples to use XML format

## [0.3.0] - 2025-10-05

### Added
- XML output format for structured AI-friendly prompting following Anthropic guidelines
- `--wrap-json` explicit flag for JSON wrapping (replaces implicit behavior)
- Comprehensive unit test suite for output formatter
- Test coverage for hooks manager (table-driven tests)

### Changed
- JSON wrapping now requires explicit `--wrap-json` flag instead of `--format json`
- Improved hook setup code with settings preservation
- Code cleanup: removed redundant comments across codebase

### Fixed
- Bug #1: Hook setup no longer deletes other Claude Code settings (awsCredentialExport, permissions, etc.)
- Bug #2: Hook commands now update correctly using index-based access instead of range variables
- Settings preservation now uses `map[string]any` to maintain all JSON fields

## [0.2.0] - 2025-10-05

### Added
- Comprehensive unit testing suite for metadata package (91.3% coverage)
- Table-based tests for all file type handlers
- Test coverage for metadata extraction, caching, and error handling

## [0.1.0] - 2025-10-04

### Added
- Initial release of Agentic Memorizer
- Semantic file indexing with Claude API integration
- Support for multiple file types (Markdown, DOCX, PPTX, PDF, images, code files, VTT transcripts)
- Hash-based caching system (SHA-256) for efficient re-analysis
- SessionStart hook integration with Claude Code
- `init` subcommand with `--setup-hooks` for automatic configuration
- Metadata extraction for file-specific attributes
- Vision analysis for images using Claude's vision capabilities
- Configurable parallel processing for semantic analysis
- File exclusion system (hidden files, skip lists)
- Markdown output format with emoji-rich formatting
- Cache management with automatic invalidation
- Configuration via YAML file and environment variables
- Command-line interface with Cobra + Viper
- Automatic hook configuration for Claude Code (startup, resume, clear, compact matchers)

[unreleased]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.4.3...HEAD
[0.4.3]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/leefowlercu/agentic-memorizer/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/leefowlercu/agentic-memorizer/releases/tag/v0.1.0
