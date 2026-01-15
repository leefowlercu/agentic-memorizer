// Package registry provides SQLite-based storage for remembered paths and file state.
package registry

import (
	"encoding/json"
	"time"
)

// RememberedPath represents a directory that has been registered for tracking.
type RememberedPath struct {
	// ID is the unique identifier for this path.
	ID int64

	// Path is the absolute canonical path to the directory.
	Path string

	// Config contains skip/include rules and other settings for this path.
	Config *PathConfig

	// LastWalkAt is the timestamp of the last directory walk.
	LastWalkAt *time.Time

	// CreatedAt is when this path was first remembered.
	CreatedAt time.Time

	// UpdatedAt is when this path was last modified.
	UpdatedAt time.Time
}

// PathConfig contains configuration for a remembered path.
type PathConfig struct {
	// SkipExtensions lists file extensions to skip (e.g., ".exe", ".dll").
	SkipExtensions []string `json:"skip_extensions,omitempty"`

	// SkipDirectories lists directory names to skip (e.g., "node_modules", ".git").
	SkipDirectories []string `json:"skip_directories,omitempty"`

	// SkipFiles lists specific file names to skip (e.g., ".DS_Store").
	SkipFiles []string `json:"skip_files,omitempty"`

	// SkipHidden indicates whether to skip hidden files and directories.
	SkipHidden bool `json:"skip_hidden"`

	// IncludeExtensions lists extensions to include even if in SkipExtensions.
	IncludeExtensions []string `json:"include_extensions,omitempty"`

	// IncludeDirectories lists directories to include even if in SkipDirectories.
	IncludeDirectories []string `json:"include_directories,omitempty"`

	// IncludeFiles lists files to include even if in SkipFiles.
	IncludeFiles []string `json:"include_files,omitempty"`

	// UseVision indicates whether to use vision API for images/PDFs.
	// nil means use global default.
	UseVision *bool `json:"use_vision,omitempty"`
}

// MarshalJSON implements json.Marshaler for PathConfig.
func (c *PathConfig) MarshalJSON() ([]byte, error) {
	type Alias PathConfig
	return json.Marshal((*Alias)(c))
}

// UnmarshalJSON implements json.Unmarshaler for PathConfig.
func (c *PathConfig) UnmarshalJSON(data []byte) error {
	type Alias PathConfig
	return json.Unmarshal(data, (*Alias)(c))
}

// Clone returns a deep copy of the PathConfig.
func (c *PathConfig) Clone() *PathConfig {
	if c == nil {
		return nil
	}

	clone := &PathConfig{
		SkipHidden: c.SkipHidden,
	}

	// Deep copy slices
	if c.SkipExtensions != nil {
		clone.SkipExtensions = make([]string, len(c.SkipExtensions))
		copy(clone.SkipExtensions, c.SkipExtensions)
	}
	if c.SkipDirectories != nil {
		clone.SkipDirectories = make([]string, len(c.SkipDirectories))
		copy(clone.SkipDirectories, c.SkipDirectories)
	}
	if c.SkipFiles != nil {
		clone.SkipFiles = make([]string, len(c.SkipFiles))
		copy(clone.SkipFiles, c.SkipFiles)
	}
	if c.IncludeExtensions != nil {
		clone.IncludeExtensions = make([]string, len(c.IncludeExtensions))
		copy(clone.IncludeExtensions, c.IncludeExtensions)
	}
	if c.IncludeDirectories != nil {
		clone.IncludeDirectories = make([]string, len(c.IncludeDirectories))
		copy(clone.IncludeDirectories, c.IncludeDirectories)
	}
	if c.IncludeFiles != nil {
		clone.IncludeFiles = make([]string, len(c.IncludeFiles))
		copy(clone.IncludeFiles, c.IncludeFiles)
	}

	// Deep copy pointer
	if c.UseVision != nil {
		v := *c.UseVision
		clone.UseVision = &v
	}

	return clone
}

// FileState tracks the state of a file for incremental processing.
type FileState struct {
	// ID is the unique identifier for this file state.
	ID int64

	// Path is the absolute path to the file.
	Path string

	// ContentHash is the SHA256 hash of the file content.
	ContentHash string

	// MetadataHash is a hash of file metadata (for detecting metadata-only changes).
	MetadataHash string

	// Size is the file size in bytes.
	Size int64

	// ModTime is the file modification time.
	ModTime time.Time

	// LastAnalyzedAt is when this file was last analyzed (legacy, kept for compatibility).
	LastAnalyzedAt *time.Time

	// AnalysisVersion is the schema version when the file was last analyzed.
	AnalysisVersion string

	// Granular analysis state tracking

	// MetadataAnalyzedAt is when metadata extraction was completed.
	MetadataAnalyzedAt *time.Time

	// SemanticAnalyzedAt is when semantic analysis was completed.
	SemanticAnalyzedAt *time.Time

	// SemanticError is the last error from semantic analysis (nil if successful).
	SemanticError *string

	// SemanticRetryCount is the number of failed semantic analysis attempts.
	SemanticRetryCount int

	// EmbeddingsAnalyzedAt is when embeddings generation was completed.
	EmbeddingsAnalyzedAt *time.Time

	// EmbeddingsError is the last error from embeddings generation (nil if successful).
	EmbeddingsError *string

	// EmbeddingsRetryCount is the number of failed embeddings generation attempts.
	EmbeddingsRetryCount int

	// CreatedAt is when this file state was first created.
	CreatedAt time.Time

	// UpdatedAt is when this file state was last modified.
	UpdatedAt time.Time
}

// IsStale returns true if the file appears to have changed based on size or mod time.
func (f *FileState) IsStale(size int64, modTime time.Time) bool {
	return f.Size != size || !f.ModTime.Equal(modTime)
}

// NeedsAnalysis returns true if the file has never been analyzed or was analyzed
// with a different schema version.
func (f *FileState) NeedsAnalysis(currentVersion string) bool {
	return f.LastAnalyzedAt == nil || f.AnalysisVersion != currentVersion
}

// PathStatus represents the health status of a remembered path.
type PathStatus struct {
	// Path is the remembered path being checked.
	Path string

	// Status indicates the accessibility of the path.
	// Values: "ok", "missing", "denied", "error"
	Status string

	// Error is the underlying error, if any.
	Error error
}

// Path status constants.
const (
	PathStatusOK      = "ok"
	PathStatusMissing = "missing"
	PathStatusDenied  = "denied"
	PathStatusError   = "error"
)
