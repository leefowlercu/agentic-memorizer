package cache

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestCacheVersion(t *testing.T) {
	version := CacheVersion()

	// Should return format "vX.Y.Z"
	if version == "" {
		t.Error("CacheVersion() returned empty string")
	}

	if version[0] != 'v' {
		t.Errorf("CacheVersion() = %q, should start with 'v'", version)
	}

	// Should match current version constants
	expected := "v1.1.1"
	if version != expected {
		t.Errorf("CacheVersion() = %q, want %q", version, expected)
	}
}

func TestParseCacheVersion(t *testing.T) {
	tests := []struct {
		name           string
		cached         *types.CachedAnalysis
		wantSchema     int
		wantMetadata   int
		wantSemantic   int
	}{
		{
			name:           "nil entry",
			cached:         nil,
			wantSchema:     0,
			wantMetadata:   0,
			wantSemantic:   0,
		},
		{
			name: "legacy entry (all zeros)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   0,
				MetadataVersion: 0,
				SemanticVersion: 0,
			},
			wantSchema:   0,
			wantMetadata: 0,
			wantSemantic: 0,
		},
		{
			name: "current version entry",
			cached: &types.CachedAnalysis{
				SchemaVersion:   1,
				MetadataVersion: 1,
				SemanticVersion: 1,
			},
			wantSchema:   1,
			wantMetadata: 1,
			wantSemantic: 1,
		},
		{
			name: "mixed version entry",
			cached: &types.CachedAnalysis{
				SchemaVersion:   1,
				MetadataVersion: 2,
				SemanticVersion: 3,
			},
			wantSchema:   1,
			wantMetadata: 2,
			wantSemantic: 3,
		},
		{
			name: "partial version (only schema set)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   1,
				MetadataVersion: 0,
				SemanticVersion: 0,
			},
			wantSchema:   1,
			wantMetadata: 0,
			wantSemantic: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, metadata, semantic := ParseCacheVersion(tt.cached)

			if schema != tt.wantSchema {
				t.Errorf("ParseCacheVersion() schema = %d, want %d", schema, tt.wantSchema)
			}
			if metadata != tt.wantMetadata {
				t.Errorf("ParseCacheVersion() metadata = %d, want %d", metadata, tt.wantMetadata)
			}
			if semantic != tt.wantSemantic {
				t.Errorf("ParseCacheVersion() semantic = %d, want %d", semantic, tt.wantSemantic)
			}
		})
	}
}

func TestIsStaleVersion(t *testing.T) {
	tests := []struct {
		name   string
		cached *types.CachedAnalysis
		want   bool
	}{
		{
			name:   "nil entry",
			cached: nil,
			want:   true,
		},
		{
			name: "legacy entry (v0.0.0)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   0,
				MetadataVersion: 0,
				SemanticVersion: 0,
			},
			want: true,
		},
		{
			name: "current version entry",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion,
				SemanticVersion: CacheSemanticVersion,
			},
			want: false,
		},
		{
			name: "schema version mismatch (lower)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion - 1,
				MetadataVersion: CacheMetadataVersion,
				SemanticVersion: CacheSemanticVersion,
			},
			want: true,
		},
		{
			name: "schema version mismatch (higher)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion + 1,
				MetadataVersion: CacheMetadataVersion,
				SemanticVersion: CacheSemanticVersion,
			},
			want: true, // Schema mismatch is always stale
		},
		{
			name: "metadata version behind",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion - 1,
				SemanticVersion: CacheSemanticVersion,
			},
			want: true,
		},
		{
			name: "metadata version ahead (forward compatible)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion + 1,
				SemanticVersion: CacheSemanticVersion,
			},
			want: false,
		},
		{
			name: "semantic version behind",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion,
				SemanticVersion: CacheSemanticVersion - 1,
			},
			want: true,
		},
		{
			name: "semantic version ahead (forward compatible)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion,
				SemanticVersion: CacheSemanticVersion + 1,
			},
			want: false,
		},
		{
			name: "both metadata and semantic behind",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion - 1,
				SemanticVersion: CacheSemanticVersion - 1,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsStaleVersion(tt.cached)
			if got != tt.want {
				t.Errorf("IsStaleVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCurrentVersion(t *testing.T) {
	tests := []struct {
		name   string
		cached *types.CachedAnalysis
		want   bool
	}{
		{
			name:   "nil entry",
			cached: nil,
			want:   false,
		},
		{
			name: "legacy entry",
			cached: &types.CachedAnalysis{
				SchemaVersion:   0,
				MetadataVersion: 0,
				SemanticVersion: 0,
			},
			want: false,
		},
		{
			name: "current version",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion,
				SemanticVersion: CacheSemanticVersion,
			},
			want: true,
		},
		{
			name: "old version",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion - 1,
				SemanticVersion: CacheSemanticVersion,
			},
			want: false,
		},
		{
			name: "future version",
			cached: &types.CachedAnalysis{
				SchemaVersion:   CacheSchemaVersion,
				MetadataVersion: CacheMetadataVersion + 1,
				SemanticVersion: CacheSemanticVersion,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCurrentVersion(tt.cached)
			if got != tt.want {
				t.Errorf("IsCurrentVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsLegacyVersion(t *testing.T) {
	tests := []struct {
		name   string
		cached *types.CachedAnalysis
		want   bool
	}{
		{
			name:   "nil entry",
			cached: nil,
			want:   false,
		},
		{
			name: "legacy entry (all zeros)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   0,
				MetadataVersion: 0,
				SemanticVersion: 0,
			},
			want: true,
		},
		{
			name: "current version",
			cached: &types.CachedAnalysis{
				SchemaVersion:   1,
				MetadataVersion: 1,
				SemanticVersion: 1,
			},
			want: false,
		},
		{
			name: "partial zeros (not legacy)",
			cached: &types.CachedAnalysis{
				SchemaVersion:   1,
				MetadataVersion: 0,
				SemanticVersion: 0,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLegacyVersion(tt.cached)
			if got != tt.want {
				t.Errorf("IsLegacyVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		name   string
		cached *types.CachedAnalysis
		want   string
	}{
		{
			name:   "nil entry",
			cached: nil,
			want:   "v0.0.0",
		},
		{
			name: "legacy entry",
			cached: &types.CachedAnalysis{
				SchemaVersion:   0,
				MetadataVersion: 0,
				SemanticVersion: 0,
			},
			want: "v0.0.0",
		},
		{
			name: "current version",
			cached: &types.CachedAnalysis{
				SchemaVersion:   1,
				MetadataVersion: 1,
				SemanticVersion: 1,
			},
			want: "v1.1.1",
		},
		{
			name: "mixed version",
			cached: &types.CachedAnalysis{
				SchemaVersion:   2,
				MetadataVersion: 3,
				SemanticVersion: 4,
			},
			want: "v2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VersionString(tt.cached)
			if got != tt.want {
				t.Errorf("VersionString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionConstants(t *testing.T) {
	// Verify version constants are positive
	if CacheSchemaVersion < 1 {
		t.Errorf("CacheSchemaVersion = %d, should be >= 1", CacheSchemaVersion)
	}
	if CacheMetadataVersion < 1 {
		t.Errorf("CacheMetadataVersion = %d, should be >= 1", CacheMetadataVersion)
	}
	if CacheSemanticVersion < 1 {
		t.Errorf("CacheSemanticVersion = %d, should be >= 1", CacheSemanticVersion)
	}
}
