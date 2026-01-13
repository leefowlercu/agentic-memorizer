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

	// IncludeHidden indicates whether to include hidden files even if SkipHidden is true.
	IncludeHidden bool `json:"include_hidden,omitempty"`

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

	// LastAnalyzedAt is when this file was last analyzed.
	LastAnalyzedAt *time.Time

	// AnalysisVersion is the schema version when the file was last analyzed.
	AnalysisVersion string

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
