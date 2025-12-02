package types

import "time"

// ============================================================================
// Internal Processing Types
// These types are used internally by the processing pipeline (metadata extraction,
// semantic analysis, worker pool, graph storage). They are NOT part of the public
// output format. External consumers should use GraphIndex and FileEntry.
// ============================================================================

// FileInfo represents basic file metadata (internal use)
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

// FileMetadata represents extracted file-specific metadata (internal use)
// Used by metadata extractors to collect file-specific information.
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

// Entity represents a named entity extracted from content (internal use)
// Used by semantic analyzer; converted to EntityRef for output.
type Entity struct {
	Name string `json:"name"` // The entity name (e.g., "Terraform", "AWS")
	Type string `json:"type"` // Entity type: technology, person, concept, organization, project
}

// Reference represents a topical reference or dependency
type Reference struct {
	Topic      string  `json:"topic"`      // The referenced topic
	Type       string  `json:"type"`       // Reference type: requires, extends, related-to, implements
	Confidence float64 `json:"confidence"` // Confidence score 0.0-1.0
}

// SemanticAnalysis represents AI-generated understanding (internal use)
// Used by semantic analyzer and cache; fields are flattened into FileEntry for output.
type SemanticAnalysis struct {
	Summary      string      `json:"summary"`
	Tags         []string    `json:"tags"`
	KeyTopics    []string    `json:"key_topics"`
	DocumentType string      `json:"document_type"`
	Confidence   float64     `json:"confidence"`
	Entities     []Entity    `json:"entities,omitempty"`   // Named entities mentioned in content
	References   []Reference `json:"references,omitempty"` // Topic references and dependencies
}

// IndexEntry combines metadata and semantic analysis (internal use)
// Used by worker pool and graph manager for processing pipeline.
type IndexEntry struct {
	Metadata FileMetadata      `json:"metadata"`
	Semantic *SemanticAnalysis `json:"semantic,omitempty"`
	Error    *string           `json:"error,omitempty"`
}

// IndexStats provides summary statistics
type IndexStats struct {
	// File statistics
	TotalFiles    int   `json:"total_files"`
	TotalSize     int64 `json:"total_size"`
	AnalyzedFiles int   `json:"analyzed_files"`
	CachedFiles   int   `json:"cached_files"`
	ErrorFiles    int   `json:"error_files"`

	// Graph statistics
	TotalTags     int            `json:"total_tags,omitempty"`
	TotalTopics   int            `json:"total_topics,omitempty"`
	TotalEntities int            `json:"total_entities,omitempty"`
	TotalEdges    int            `json:"total_edges,omitempty"`
	ByCategory    map[string]int `json:"by_category,omitempty"`

	// Coverage metrics
	FilesWithSummary  int     `json:"files_with_summary,omitempty"`
	FilesWithTags     int     `json:"files_with_tags,omitempty"`
	FilesWithTopics   int     `json:"files_with_topics,omitempty"`
	FilesWithEntities int     `json:"files_with_entities,omitempty"`
	AvgTagsPerFile    float64 `json:"avg_tags_per_file,omitempty"`
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

// ============================================================================
// Graph-Native Types (Phase 3)
// These types represent the new graph-native index structure with flattened
// file entries and knowledge graph relationships.
// ============================================================================

// GraphIndex represents the graph-native memory index
// This is the new format that replaces the nested Index structure
type GraphIndex struct {
	// Metadata
	Generated  time.Time  `json:"generated"`
	MemoryRoot string     `json:"memory_root"`
	Stats      IndexStats `json:"stats"`

	// Files with their semantic understanding and relationships
	Files []FileEntry `json:"files"`

	// Knowledge graph summary (tag/topic/entity landscape)
	Knowledge *KnowledgeSummary `json:"knowledge,omitempty"`

	// Insights from graph analytics (verbose mode only)
	Insights *IndexInsights `json:"insights,omitempty"`
}

// FileEntry represents a file in the knowledge graph
// This is a flattened structure replacing the nested IndexEntry
type FileEntry struct {
	// Identity
	Path string `json:"path"`
	Name string `json:"name"`
	Hash string `json:"hash"`

	// Classification
	Type     string `json:"type"`     // file extension
	Category string `json:"category"` // documents, code, images, etc.

	// Physical attributes
	Size       int64     `json:"size"`
	SizeHuman  string    `json:"size_human"`
	Modified   time.Time `json:"modified"`
	IsReadable bool      `json:"is_readable"`

	// Type-specific metadata (optional)
	WordCount  *int      `json:"word_count,omitempty"`
	PageCount  *int      `json:"page_count,omitempty"`
	SlideCount *int      `json:"slide_count,omitempty"`
	Dimensions *ImageDim `json:"dimensions,omitempty"`
	Duration   *string   `json:"duration,omitempty"`
	Language   *string   `json:"language,omitempty"`
	Author     *string   `json:"author,omitempty"`

	// Semantic understanding
	Summary      string  `json:"summary,omitempty"`
	DocumentType string  `json:"document_type,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`

	// Graph relationships (the knowledge graph edges)
	Tags     []string     `json:"tags,omitempty"`
	Topics   []string     `json:"topics,omitempty"`
	Entities []EntityRef  `json:"entities,omitempty"`

	// Related files (the graph value-add) - populated in verbose mode
	RelatedFiles []RelatedFile `json:"related_files,omitempty"`

	// Error (if analysis failed)
	Error *string `json:"error,omitempty"`
}

// EntityRef is a reference to an entity from a file
type EntityRef struct {
	Name string `json:"name"`
	Type string `json:"type"` // technology, person, organization, concept, project
}

// RelatedFile represents a file related through the knowledge graph
type RelatedFile struct {
	Path   string   `json:"path"`
	Name   string   `json:"name"`
	Via    string   `json:"via"`    // "tags", "topics", "entities", "similarity"
	Shared []string `json:"shared"` // The actual shared items
	Score  float64  `json:"score,omitempty"`
}

// KnowledgeSummary provides an overview of the knowledge landscape
type KnowledgeSummary struct {
	// Top tags by file count
	TopTags []TagCount `json:"top_tags,omitempty"`

	// Top topics by file count
	TopTopics []TopicCount `json:"top_topics,omitempty"`

	// Top entities by mention count
	TopEntities []EntityCount `json:"top_entities,omitempty"`
}

// TagCount represents a tag with its usage count
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TopicCount represents a topic with its usage count
type TopicCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// EntityCount represents an entity with its mention count
type EntityCount struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// IndexInsights contains graph analytics results (verbose mode only)
type IndexInsights struct {
	// File recommendations based on graph relationships
	Recommendations []Recommendation `json:"recommendations,omitempty"`

	// Topic clusters detected in the graph
	TopicClusters []Cluster `json:"topic_clusters,omitempty"`

	// Coverage gaps identified in the knowledge base
	CoverageGaps []Gap `json:"coverage_gaps,omitempty"`
}

// Recommendation suggests related files based on graph analysis
type Recommendation struct {
	SourcePath  string   `json:"source_path"`
	TargetPath  string   `json:"target_path"`
	TargetName  string   `json:"target_name"`
	Reason      string   `json:"reason"`
	SharedItems []string `json:"shared_items,omitempty"`
	Score       float64  `json:"score"`
}

// Cluster represents a group of related files sharing common themes
type Cluster struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	FileCount   int      `json:"file_count"`
	FilePaths   []string `json:"file_paths,omitempty"`
	CommonTags  []string `json:"common_tags,omitempty"`
}

// Gap represents a knowledge gap or coverage issue
type Gap struct {
	Type        string `json:"type"`        // topic, entity, tag, category
	Name        string `json:"name"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // low, medium, high
	Suggestion  string `json:"suggestion,omitempty"`
}
