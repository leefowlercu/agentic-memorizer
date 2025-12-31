package cache

import "path/filepath"

const (
	// HashPrefix is the standard prefix for SHA-256 content hashes
	HashPrefix = "sha256:"
)

// ShardPath generates a two-level sharded directory path for a given hash.
// This prevents filesystem performance degradation when directories contain
// many files by distributing entries across 65,536 possible subdirectories.
//
// For hashes with "sha256:" prefix, the prefix is stripped before sharding
// to use the actual hash value for directory distribution.
//
// Examples:
//   - "sha256:41d6..." → "{base}/41/d6/{filename}"
//   - "abc123..."      → "{base}/ab/c1/{filename}"
//   - "ab" (short)     → "{base}/{filename}" (no sharding)
func ShardPath(basePath, hash, filename string) string {
	shardKey := extractHashValue(hash)

	if len(shardKey) < 4 {
		return filepath.Join(basePath, filename)
	}

	return filepath.Join(basePath, shardKey[:2], shardKey[2:4], filename)
}

// extractHashValue returns the hash value, stripping "sha256:" prefix if present.
func extractHashValue(hash string) string {
	if len(hash) > len(HashPrefix) && hash[:len(HashPrefix)] == HashPrefix {
		return hash[len(HashPrefix):]
	}
	return hash
}
