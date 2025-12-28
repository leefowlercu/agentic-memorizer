// Package cache provides semantic analysis caching with content-addressable storage.
//
// # Cache Versioning System
//
// The cache uses a three-tier versioning system to detect when cached entries
// become stale due to changes in extraction logic, analysis prompts, or data structures.
//
// Version Tiers:
//
//   - CacheSchemaVersion: Tracks changes to CachedAnalysis structure itself.
//     Bump when adding/removing/renaming fields in CachedAnalysis.
//
//   - CacheMetadataVersion: Tracks changes to metadata extraction logic.
//     Bump when changing FileMetadata fields, extraction algorithms, or handlers.
//
//   - CacheSemanticVersion: Tracks changes to semantic analysis logic.
//     Bump when changing prompts, SemanticAnalysis fields, or analysis routing.
//
// Cache Key Format:
//
//	Old format: {hash[:16]}.json
//	New format: {hash[:16]}-v{schema}-{metadata}-{semantic}.json
//
// Example: sha256:abc12345-v1-1-1.json
package cache

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// CacheSchemaVersion tracks changes to CachedAnalysis structure.
// Increment when:
//   - Adding fields to CachedAnalysis struct
//   - Removing fields from CachedAnalysis struct
//   - Renaming fields in CachedAnalysis struct
//   - Changing field types in CachedAnalysis struct
//   - Changing cache storage format (JSON structure)
//   - Changing cache key generation algorithm
const CacheSchemaVersion = 1

// CacheMetadataVersion tracks changes to metadata extraction logic.
// Increment when:
//   - Adding fields to FileMetadata
//   - Changing metadata extraction algorithms
//   - Fixing bugs in metadata handlers
//   - Adding new metadata handlers
//   - Changing categorization logic
//   - Updating readability detection
const CacheMetadataVersion = 1

// CacheSemanticVersion tracks changes to semantic analysis logic.
// Increment when:
//   - Changing prompt templates
//   - Adding fields to SemanticAnalysis
//   - Changing analysis routing logic (which analyzer for which file type)
//   - Updating response parsing logic
//   - Changing confidence score calculations
//   - Updating entity/reference extraction
//   - Fixing bugs in semantic analysis
//
// Version 2: Multi-provider semantic analysis refactor (claude.* + analysis.* → semantic.*)
const CacheSemanticVersion = 2

// CacheVersion returns the combined version string in format "v{schema}.{metadata}.{semantic}"
func CacheVersion() string {
	return fmt.Sprintf("v%d.%d.%d", CacheSchemaVersion, CacheMetadataVersion, CacheSemanticVersion)
}

// ParseCacheVersion extracts version components from a cached analysis entry.
// Returns (0, 0, 0) for legacy entries that don't have version fields set.
func ParseCacheVersion(cached *types.CachedAnalysis) (schema, metadata, semantic int) {
	if cached == nil {
		return 0, 0, 0
	}

	// Handle legacy entries (no version fields)
	if cached.SchemaVersion == 0 && cached.MetadataVersion == 0 && cached.SemanticVersion == 0 {
		return 0, 0, 0
	}

	return cached.SchemaVersion, cached.MetadataVersion, cached.SemanticVersion
}

// IsStaleVersion checks if a cache entry is from an older version that should be re-analyzed.
// Returns true if the entry should be considered stale and re-analyzed.
//
// Staleness rules:
//   - Schema version mismatch = always stale (incompatible structure)
//   - Metadata version behind current = stale (missing newer metadata fields)
//   - Semantic version behind current = stale (outdated analysis)
//   - Future versions (newer than current) = not stale (forward compatible)
func IsStaleVersion(cached *types.CachedAnalysis) bool {
	if cached == nil {
		return true
	}

	schema, metadata, semantic := ParseCacheVersion(cached)

	// Schema version mismatch = incompatible structure, always stale
	if schema != CacheSchemaVersion {
		return true
	}

	// Metadata version behind current = stale (needs newer metadata)
	if metadata < CacheMetadataVersion {
		return true
	}

	// Semantic version behind current = stale (needs newer analysis)
	if semantic < CacheSemanticVersion {
		return true
	}

	return false
}

// IsCurrentVersion checks if a cache entry is from the current version.
func IsCurrentVersion(cached *types.CachedAnalysis) bool {
	if cached == nil {
		return false
	}

	schema, metadata, semantic := ParseCacheVersion(cached)
	return schema == CacheSchemaVersion &&
		metadata == CacheMetadataVersion &&
		semantic == CacheSemanticVersion
}

// IsLegacyVersion checks if a cache entry is a legacy entry (version 0.0.0).
// Legacy entries are from before versioning was implemented.
func IsLegacyVersion(cached *types.CachedAnalysis) bool {
	if cached == nil {
		return false
	}

	schema, metadata, semantic := ParseCacheVersion(cached)
	return schema == 0 && metadata == 0 && semantic == 0
}

// VersionString returns the version string for a cached entry in format "v{schema}.{metadata}.{semantic}".
func VersionString(cached *types.CachedAnalysis) string {
	if cached == nil {
		return "v0.0.0"
	}

	schema, metadata, semantic := ParseCacheVersion(cached)
	return fmt.Sprintf("v%d.%d.%d", schema, metadata, semantic)
}
