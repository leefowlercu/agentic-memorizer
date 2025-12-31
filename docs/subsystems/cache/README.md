# Cache

Content-addressable caching for semantic analysis results with three-tier versioning, provider isolation, and directory sharding.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-31

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Cache subsystem provides persistent caching for semantic analysis results, enabling efficient re-use of expensive AI-powered analysis across daemon restarts and file operations. By using content-addressable storage keyed on file content hashes rather than file paths, the cache automatically handles file renames and moves without requiring re-analysis.

The subsystem implements a three-tier versioning system that tracks changes to the cache structure, metadata extraction logic, and semantic analysis logic independently. This allows cache entries to be intelligently invalidated when processing logic changes, while preserving valid entries when unrelated components are updated.

Provider isolation ensures that cache entries from different semantic analysis providers (Claude, OpenAI, Gemini) are stored separately, preventing cross-contamination and enabling clean provider switching.

Directory sharding distributes cache files across a two-level directory hierarchy based on content hash prefixes, preventing filesystem performance degradation when the cache grows large. This sharding mechanism is shared with the embeddings cache subsystem for consistency.

## Design Principles

### Content-Addressable Storage

Cache keys are derived from SHA-256 hashes of file content rather than file paths. This approach:

- Enables cache hits when files are renamed or moved
- Automatically invalidates entries when file content changes
- Provides deterministic cache behavior independent of file location

### Three-Tier Versioning

The cache uses independent version tracking for three logical tiers:

- **Schema Version** - Tracks changes to the CachedAnalysis structure itself
- **Metadata Version** - Tracks changes to metadata extraction logic and handlers
- **Semantic Version** - Tracks changes to semantic analysis prompts and routing

Version mismatches trigger selective re-analysis: schema mismatches always invalidate (incompatible structure), while metadata or semantic version differences indicate stale analysis that needs refreshing.

### Provider Isolation

Each semantic analysis provider stores cache entries in its own subdirectory:

```
~/.memorizer/.cache/summaries/
├── claude/   # Claude provider entries
├── openai/   # OpenAI provider entries
└── gemini/   # Gemini provider entries
```

This isolation enables:

- Clean provider switching without cache conflicts
- Independent cache management per provider
- Clear visibility into per-provider cache usage

### Directory Sharding

Cache files are distributed across a two-level directory hierarchy based on content hash prefixes:

```
~/.memorizer/.cache/summaries/claude/
├── 41/
│   └── d6/
│       └── sha256:41d63309f-v1-1-2.json
├── ab/
│   └── cd/
│       └── sha256:abcdef123-v1-1-2.json
```

This sharding approach:

- Prevents filesystem performance degradation with many cached entries
- Distributes entries across up to 65,536 possible subdirectories (256 × 256)
- Uses the first four characters of the hash value (after stripping the "sha256:" prefix) for directory names
- Is shared with the embeddings cache for consistent storage patterns

### Forward Compatibility

Cache entries with version numbers higher than the current application version are considered valid (not stale). This forward compatibility allows rollback scenarios where a newer application version created cache entries that should remain usable by older versions.

## Key Components

### Manager

The Manager type (`internal/cache/manager.go`) provides the core cache operations:

- **Get** - Retrieves cached analysis by content hash and provider, with legacy format fallback
- **Set** - Stores analysis results with current version stamps and provider routing
- **IsStale** - Determines if a cache entry needs re-analysis based on content hash and version
- **Clear** - Removes all cache entries across all providers
- **ClearOldVersions** - Selectively removes entries with outdated version stamps
- **GetStats** - Returns cache statistics including entry counts and version distribution

### Version Functions

Version management functions (`internal/cache/version.go`) handle version comparisons and staleness detection:

- **CacheVersion** - Returns the combined version string (e.g., "v1.1.2")
- **IsStaleVersion** - Checks if a cache entry requires re-analysis
- **IsCurrentVersion** - Checks if a cache entry matches current versions
- **IsLegacyVersion** - Identifies pre-versioning cache entries (v0.0.0)
- **ParseCacheVersion** - Extracts version components from a cached entry

### HashFile Function

The HashFile utility computes SHA-256 content hashes for files, producing cache keys in the format `sha256:{hex-encoded-hash}`.

### Shard Functions

The shard utilities (`internal/cache/shard.go`) provide directory sharding capabilities:

- **ShardPath** - Generates a two-level sharded directory path for a given hash and filename
- **extractHashValue** - Extracts the hash value from a prefixed hash string (strips "sha256:" prefix)
- **HashPrefix** - Constant defining the standard prefix for SHA-256 content hashes

These functions are used by both the semantic cache Manager and the embeddings cache to ensure consistent directory sharding across the application.

### CacheStats

The CacheStats type provides visibility into cache state:

- Total entry count and size
- Legacy entry count (pre-versioning entries)
- Version distribution across cached entries

## Integration Points

### Worker Pool

The daemon worker pool consults the cache before performing semantic analysis:

1. Computes file content hash via HashFile
2. Calls Manager.Get with hash and current provider
3. On cache hit, checks staleness via Manager.IsStale
4. On cache miss or stale entry, performs semantic analysis
5. Stores result via Manager.Set

### Semantic Analyzer

The semantic analyzer provides the analysis results stored in the cache. The CachedAnalysis structure captures both the analysis output and version metadata to enable staleness detection.

### Configuration

Cache behavior is configured via the config subsystem:

- `semantic.cache_dir` - Base directory for cache storage (default: `~/.memorizer/cache`)
- `semantic.provider` - Determines which provider subdirectory to use

### CLI Commands

The cache CLI commands (`cmd/cache/`) provide user-facing cache management:

- `cache status` - Displays cache statistics via GetStats
- `cache clear --stale` - Removes outdated entries via ClearOldVersions
- `cache clear --all` - Removes all entries via Clear

### Daemon Rebuild

During daemon rebuild operations with `--clear-stale`, the ClearOldVersions function removes entries that would be detected as stale, reducing unnecessary API calls during the rebuild.

### Embeddings Cache

The embeddings subsystem uses the shared ShardPath function from this subsystem to maintain consistent directory sharding. Both caches use the same two-level sharding strategy based on content hash prefixes, ensuring uniform storage patterns across the application.

## Glossary

| Term | Definition |
|------|------------|
| Content Hash | SHA-256 hash of file contents, used as the primary cache key component |
| Cache Key | Composite identifier combining content hash prefix, version tuple, and file extension |
| Directory Sharding | Storage strategy that distributes cache files across a two-level directory hierarchy based on hash prefixes |
| Legacy Entry | Cache entry from before versioning was implemented, identified by version v0.0.0 |
| Provider Isolation | Storage strategy where each semantic provider uses a separate subdirectory |
| Schema Version | Version number tracking changes to the CachedAnalysis structure |
| Metadata Version | Version number tracking changes to metadata extraction logic |
| Semantic Version | Version number tracking changes to semantic analysis logic |
| Shard Key | First four characters of a content hash (after prefix), used to determine subdirectory placement |
| Staleness | Condition where a cache entry is outdated due to content or version changes |
| Version Tuple | Combined version identifier in format v{schema}.{metadata}.{semantic} |
