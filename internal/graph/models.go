package graph

import (
	"time"
)

// Node labels for the graph schema.
const (
	LabelFile      = "File"
	LabelChunk     = "Chunk"
	LabelDirectory = "Directory"
	LabelTag       = "Tag"
	LabelTopic     = "Topic"
	LabelEntity    = "Entity"
)

// Relationship types for the graph schema.
const (
	RelContains     = "CONTAINS"      // Directory -> File/Directory
	RelHasChunk     = "HAS_CHUNK"     // File -> Chunk
	RelHasTag       = "HAS_TAG"       // File -> Tag
	RelCoversTopic  = "COVERS_TOPIC"  // File -> Topic
	RelMentions     = "MENTIONS"      // File/Chunk -> Entity
	RelReferences   = "REFERENCES"    // File/Chunk -> File/URL
	RelSimilarTo    = "SIMILAR_TO"    // Chunk -> Chunk (semantic similarity)
	RelDependsOn    = "DEPENDS_ON"    // File -> File (code dependencies)
)

// FileNode represents a file in the knowledge graph.
type FileNode struct {
	// Path is the absolute path to the file.
	Path string `json:"path"`

	// Name is the file name without path.
	Name string `json:"name"`

	// Extension is the file extension.
	Extension string `json:"extension"`

	// MIMEType is the detected MIME type.
	MIMEType string `json:"mime_type"`

	// Language is the programming language (for code files).
	Language string `json:"language,omitempty"`

	// Size is the file size in bytes.
	Size int64 `json:"size"`

	// ModTime is the last modification time.
	ModTime time.Time `json:"mod_time"`

	// ContentHash is the hash of the file content.
	ContentHash string `json:"content_hash"`

	// MetadataHash is the hash of the file metadata.
	MetadataHash string `json:"metadata_hash"`

	// Summary is the semantic summary of the file.
	Summary string `json:"summary,omitempty"`

	// Complexity is the complexity score (1-10).
	Complexity int `json:"complexity,omitempty"`

	// AnalyzedAt is when the file was last analyzed.
	AnalyzedAt time.Time `json:"analyzed_at,omitempty"`

	// AnalysisVersion is the version of the analysis.
	AnalysisVersion int `json:"analysis_version,omitempty"`

	// CreatedAt is when the node was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the node was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// ChunkNode represents a chunk of a file in the knowledge graph.
type ChunkNode struct {
	// ID is the unique chunk identifier.
	ID string `json:"id"`

	// FilePath is the path to the containing file.
	FilePath string `json:"file_path"`

	// Index is the chunk index within the file.
	Index int `json:"index"`

	// Content is the chunk text content.
	Content string `json:"content"`

	// ContentHash is the hash of the chunk content.
	ContentHash string `json:"content_hash"`

	// StartOffset is the byte offset where the chunk starts.
	StartOffset int `json:"start_offset"`

	// EndOffset is the byte offset where the chunk ends.
	EndOffset int `json:"end_offset"`

	// ChunkType is the type of content (code, markdown, prose, etc.).
	ChunkType string `json:"chunk_type"`

	// FunctionName is the function/method name (for code chunks).
	FunctionName string `json:"function_name,omitempty"`

	// ClassName is the class/struct name (for code chunks).
	ClassName string `json:"class_name,omitempty"`

	// Heading is the section heading (for markdown chunks).
	Heading string `json:"heading,omitempty"`

	// HeadingLevel is the heading depth (for markdown chunks).
	HeadingLevel int `json:"heading_level,omitempty"`

	// Summary is the semantic summary of the chunk.
	Summary string `json:"summary,omitempty"`

	// Embedding is the vector embedding.
	Embedding []float32 `json:"embedding,omitempty"`

	// EmbeddingVersion is the version of the embedding model.
	EmbeddingVersion int `json:"embedding_version,omitempty"`

	// TokenCount is the estimated token count.
	TokenCount int `json:"token_count,omitempty"`

	// CreatedAt is when the node was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the node was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// DirectoryNode represents a directory in the knowledge graph.
type DirectoryNode struct {
	// Path is the absolute path to the directory.
	Path string `json:"path"`

	// Name is the directory name.
	Name string `json:"name"`

	// IsRemembered indicates if this is a remembered root directory.
	IsRemembered bool `json:"is_remembered"`

	// FileCount is the number of files in this directory (not recursive).
	FileCount int `json:"file_count"`

	// CreatedAt is when the node was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the node was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// TagNode represents a tag in the knowledge graph.
type TagNode struct {
	// Name is the tag name.
	Name string `json:"name"`

	// NormalizedName is the lowercase normalized name for matching.
	NormalizedName string `json:"normalized_name"`

	// UsageCount is how many files use this tag.
	UsageCount int `json:"usage_count"`

	// CreatedAt is when the node was created.
	CreatedAt time.Time `json:"created_at"`
}

// TopicNode represents a topic in the knowledge graph.
type TopicNode struct {
	// Name is the topic name.
	Name string `json:"name"`

	// NormalizedName is the lowercase normalized name for matching.
	NormalizedName string `json:"normalized_name"`

	// Description is an optional description of the topic.
	Description string `json:"description,omitempty"`

	// UsageCount is how many files cover this topic.
	UsageCount int `json:"usage_count"`

	// CreatedAt is when the node was created.
	CreatedAt time.Time `json:"created_at"`
}

// EntityNode represents a named entity in the knowledge graph.
type EntityNode struct {
	// Name is the entity name.
	Name string `json:"name"`

	// Type is the entity type (person, organization, location, concept, etc.).
	Type string `json:"type"`

	// NormalizedName is the lowercase normalized name for matching.
	NormalizedName string `json:"normalized_name"`

	// UsageCount is how many times this entity is mentioned.
	UsageCount int `json:"usage_count"`

	// CreatedAt is when the node was created.
	CreatedAt time.Time `json:"created_at"`
}

// Topic represents a topic with confidence score.
type Topic struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

// Entity represents a named entity.
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Reference represents a reference to another resource.
type Reference struct {
	Type   string `json:"type"`   // url, file, package, symbol
	Target string `json:"target"` // the actual reference value
}

// QueryResult contains the results of a Cypher query.
type QueryResult struct {
	// Columns are the column names returned.
	Columns []string

	// Rows are the result rows.
	Rows [][]any

	// Stats contains query execution statistics.
	Stats QueryStats
}

// QueryStats contains statistics about query execution.
type QueryStats struct {
	NodesCreated      int
	NodesDeleted      int
	RelationsCreated  int
	RelationsDeleted  int
	PropertiesSet     int
	ExecutionTimeMs   float64
}

// GraphSnapshot contains a point-in-time snapshot of the graph.
type GraphSnapshot struct {
	// Files are all file nodes.
	Files []FileNode `json:"files"`

	// Directories are all directory nodes.
	Directories []DirectoryNode `json:"directories"`

	// Tags are all tag nodes.
	Tags []TagNode `json:"tags"`

	// Topics are all topic nodes.
	Topics []TopicNode `json:"topics"`

	// Entities are all entity nodes.
	Entities []EntityNode `json:"entities"`

	// TotalChunks is the total number of chunks.
	TotalChunks int `json:"total_chunks"`

	// TotalRelationships is the total number of relationships.
	TotalRelationships int `json:"total_relationships"`

	// ExportedAt is when the snapshot was created.
	ExportedAt time.Time `json:"exported_at"`

	// Version is the snapshot format version.
	Version int `json:"version"`
}

// FileWithRelations contains a file node with its related data.
type FileWithRelations struct {
	File       FileNode   `json:"file"`
	Tags       []string   `json:"tags"`
	Topics     []Topic    `json:"topics"`
	Entities   []Entity   `json:"entities"`
	References []Reference `json:"references"`
	ChunkCount int        `json:"chunk_count"`
}
