package graph

import (
	"time"
)

// Node labels for the graph schema.
const (
	LabelFile           = "File"
	LabelChunk          = "Chunk"
	LabelDirectory      = "Directory"
	LabelTag            = "Tag"
	LabelTopic          = "Topic"
	LabelEntity         = "Entity"
	LabelCodeMeta       = "CodeMeta"
	LabelDocumentMeta   = "DocumentMeta"
	LabelNotebookMeta   = "NotebookMeta"
	LabelBuildMeta      = "BuildMeta"
	LabelInfraMeta      = "InfraMeta"
	LabelSchemaMeta     = "SchemaMeta"
	LabelStructuredMeta = "StructuredMeta"
	LabelSQLMeta        = "SQLMeta"
	LabelLogMeta        = "LogMeta"
	LabelChunkEmbedding = "ChunkEmbedding"
)

// Relationship types for the graph schema.
const (
	RelContains        = "CONTAINS"          // Directory -> File/Directory
	RelHasChunk        = "HAS_CHUNK"         // File -> Chunk
	RelHasTag          = "HAS_TAG"           // File -> Tag
	RelCoversTopic     = "COVERS_TOPIC"      // File -> Topic
	RelMentions        = "MENTIONS"          // File/Chunk -> Entity
	RelReferences      = "REFERENCES"        // File/Chunk -> File/URL
	RelSimilarTo       = "SIMILAR_TO"        // Chunk -> Chunk (semantic similarity)
	RelDependsOn       = "DEPENDS_ON"        // File -> File (code dependencies)
	RelHasCodeMeta     = "HAS_CODE_META"     // Chunk -> CodeMeta
	RelHasDocMeta      = "HAS_DOC_META"      // Chunk -> DocumentMeta
	RelHasNotebookMeta = "HAS_NOTEBOOK_META" // Chunk -> NotebookMeta
	RelHasBuildMeta    = "HAS_BUILD_META"    // Chunk -> BuildMeta
	RelHasInfraMeta    = "HAS_INFRA_META"    // Chunk -> InfraMeta
	RelHasSchemaMeta   = "HAS_SCHEMA_META"   // Chunk -> SchemaMeta
	RelHasStructMeta   = "HAS_STRUCT_META"   // Chunk -> StructuredMeta
	RelHasSQLMeta      = "HAS_SQL_META"      // Chunk -> SQLMeta
	RelHasLogMeta      = "HAS_LOG_META"      // Chunk -> LogMeta
	RelHasEmbedding    = "HAS_EMBEDDING"     // Chunk -> ChunkEmbedding
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

	// IngestKind is the coarse file classification (text, image, etc.).
	IngestKind string `json:"ingest_kind,omitempty"`

	// IngestMode is the processing decision (chunk, metadata_only, skip).
	IngestMode string `json:"ingest_mode,omitempty"`

	// IngestReason explains why the ingest mode was selected.
	IngestReason string `json:"ingest_reason,omitempty"`

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
// Metadata (code, document, etc.) and embeddings are stored in separate nodes
// connected via relationships (HAS_CODE_META, HAS_DOC_META, HAS_EMBEDDING, etc.).
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

	// TokenCount is the estimated token count.
	TokenCount int `json:"token_count,omitempty"`

	// Summary is the semantic summary of the chunk.
	Summary string `json:"summary,omitempty"`

	// CreatedAt is when the node was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the node was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// CodeMetaNode stores code-specific metadata for a chunk.
type CodeMetaNode struct {
	Language     string   `json:"language,omitempty"`
	FunctionName string   `json:"function_name,omitempty"`
	ClassName    string   `json:"class_name,omitempty"`
	Signature    string   `json:"signature,omitempty"`
	ReturnType   string   `json:"return_type,omitempty"`
	Parameters   []string `json:"parameters,omitempty"`
	Decorators   []string `json:"decorators,omitempty"`
	Implements   []string `json:"implements,omitempty"`
	Visibility   string   `json:"visibility,omitempty"`
	Docstring    string   `json:"docstring,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	ParentClass  string   `json:"parent_class,omitempty"`
	IsAsync      bool     `json:"is_async,omitempty"`
	IsStatic     bool     `json:"is_static,omitempty"`
	IsExported   bool     `json:"is_exported,omitempty"`
	LineStart    int      `json:"line_start,omitempty"`
	LineEnd      int      `json:"line_end,omitempty"`
}

// DocumentMetaNode stores document-specific metadata for a chunk.
type DocumentMetaNode struct {
	Heading      string   `json:"heading,omitempty"`
	HeadingLevel int      `json:"heading_level,omitempty"`
	SectionPath  []string `json:"section_path,omitempty"`
	PageNumber   int      `json:"page_number,omitempty"`
	ListType     string   `json:"list_type,omitempty"`
	ListDepth    int      `json:"list_depth,omitempty"`
	IsFootnote   bool     `json:"is_footnote,omitempty"`
	IsCitation   bool     `json:"is_citation,omitempty"`
	IsBlockquote bool     `json:"is_blockquote,omitempty"`
}

// NotebookMetaNode stores notebook-specific metadata for a chunk.
type NotebookMetaNode struct {
	CellIndex       int    `json:"cell_index,omitempty"`
	CellType        string `json:"cell_type,omitempty"`
	ExecutionCount  int    `json:"execution_count,omitempty"`
	HasOutput       bool   `json:"has_output,omitempty"`
	OutputTruncated bool   `json:"output_truncated,omitempty"`
}

// BuildMetaNode stores build configuration metadata for a chunk.
type BuildMetaNode struct {
	TargetName   string   `json:"target_name,omitempty"`
	TargetType   string   `json:"target_type,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	StageName    string   `json:"stage_name,omitempty"`
	ImageName    string   `json:"image_name,omitempty"`
	BuildArgs    []string `json:"build_args,omitempty"`
}

// InfraMetaNode stores infrastructure configuration metadata for a chunk.
type InfraMetaNode struct {
	ResourceType string   `json:"resource_type,omitempty"`
	ResourceName string   `json:"resource_name,omitempty"`
	Provider     string   `json:"provider,omitempty"`
	BlockType    string   `json:"block_type,omitempty"`
	ModuleName   string   `json:"module_name,omitempty"`
	References   []string `json:"references,omitempty"`
}

// SchemaMetaNode stores schema definition metadata for a chunk.
type SchemaMetaNode struct {
	MessageName  string   `json:"message_name,omitempty"`
	ServiceName  string   `json:"service_name,omitempty"`
	RPCName      string   `json:"rpc_name,omitempty"`
	TypeName     string   `json:"type_name,omitempty"`
	Fields       []string `json:"fields,omitempty"`
	IsDeprecated bool     `json:"is_deprecated,omitempty"`
}

// StructuredMetaNode stores structured data metadata for a chunk.
type StructuredMetaNode struct {
	RecordIndex int      `json:"record_index,omitempty"`
	RecordCount int      `json:"record_count,omitempty"`
	KeyNames    []string `json:"key_names,omitempty"`
	ArrayPath   string   `json:"array_path,omitempty"`
}

// SQLMetaNode stores SQL-specific metadata for a chunk.
type SQLMetaNode struct {
	StatementType string   `json:"statement_type,omitempty"`
	TableNames    []string `json:"table_names,omitempty"`
	JoinedTables  []string `json:"joined_tables,omitempty"`
	HasSubquery   bool     `json:"has_subquery,omitempty"`
}

// LogMetaNode stores log-specific metadata for a chunk.
type LogMetaNode struct {
	LogLevel   string    `json:"log_level,omitempty"`
	TimeRange  [2]string `json:"time_range,omitempty"`
	Source     string    `json:"source,omitempty"`
	EntryCount int       `json:"entry_count,omitempty"`
}

// ChunkEmbeddingNode stores vector embeddings for a chunk.
// Supports multiple embeddings from different providers/models.
type ChunkEmbeddingNode struct {
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Dimensions int       `json:"dimensions"`
	Embedding  []float32 `json:"embedding"`
	CreatedAt  time.Time `json:"created_at"`
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
	NodesCreated     int
	NodesDeleted     int
	RelationsCreated int
	RelationsDeleted int
	PropertiesSet    int
	ExecutionTimeMs  float64
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
	File       FileNode    `json:"file"`
	Tags       []string    `json:"tags"`
	Topics     []Topic     `json:"topics"`
	Entities   []Entity    `json:"entities"`
	References []Reference `json:"references"`
	ChunkCount int         `json:"chunk_count"`
}
