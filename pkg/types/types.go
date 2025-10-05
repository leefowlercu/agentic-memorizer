package types

import "time"

// FileInfo represents basic file metadata
type FileInfo struct {
	Path       string    `json:"path"`
	RelPath    string    `json:"rel_path"`
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	Modified   time.Time `json:"modified"`
	Type       string    `json:"type"`
	Category   string    `json:"category"`
	IsReadable bool      `json:"is_readable"`
}

// FileMetadata represents extracted file-specific metadata
type FileMetadata struct {
	FileInfo
	// Type-specific fields
	WordCount  *int      `json:"word_count,omitempty"`
	PageCount  *int      `json:"page_count,omitempty"`
	SlideCount *int      `json:"slide_count,omitempty"`
	Dimensions *ImageDim `json:"dimensions,omitempty"`
	Duration   *string   `json:"duration,omitempty"`
	Sections   []string  `json:"sections,omitempty"`
	Language   *string   `json:"language,omitempty"`
	Author     *string   `json:"author,omitempty"`
}

// ImageDim represents image dimensions
type ImageDim struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// SemanticAnalysis represents AI-generated understanding
type SemanticAnalysis struct {
	Summary      string   `json:"summary"`
	Tags         []string `json:"tags"`
	KeyTopics    []string `json:"key_topics"`
	DocumentType string   `json:"document_type"`
	Confidence   float64  `json:"confidence"`
}

// IndexEntry combines metadata and semantic analysis
type IndexEntry struct {
	Metadata FileMetadata      `json:"metadata"`
	Semantic *SemanticAnalysis `json:"semantic,omitempty"`
	Error    *string           `json:"error,omitempty"`
}

// Index represents the complete memory index
type Index struct {
	Generated time.Time    `json:"generated"`
	Root      string       `json:"root"`
	Entries   []IndexEntry `json:"entries"`
	Stats     IndexStats   `json:"stats"`
}

// IndexStats provides summary statistics
type IndexStats struct {
	TotalFiles    int   `json:"total_files"`
	TotalSize     int64 `json:"total_size"`
	AnalyzedFiles int   `json:"analyzed_files"`
	CachedFiles   int   `json:"cached_files"`
	ErrorFiles    int   `json:"error_files"`
}

// CachedAnalysis represents a cached semantic analysis result
type CachedAnalysis struct {
	FilePath   string            `json:"file_path"`
	FileHash   string            `json:"file_hash"`
	AnalyzedAt time.Time         `json:"analyzed_at"`
	Metadata   FileMetadata      `json:"metadata"`
	Semantic   *SemanticAnalysis `json:"semantic,omitempty"`
	Error      *string           `json:"error,omitempty"`
}
