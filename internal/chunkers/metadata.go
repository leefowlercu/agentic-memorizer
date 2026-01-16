package chunkers

import "time"

// ChunkMetadata contains type-specific metadata for a chunk.
// Only one of the typed metadata pointers will be populated based on Type.
type ChunkMetadata struct {
	// Type indicates the content type of this chunk.
	Type ChunkType

	// TokenEstimate is an accurate token count via tiktoken.
	TokenEstimate int

	// Type-specific metadata (only one populated based on Type)
	Code       *CodeMetadata
	Document   *DocumentMetadata
	Notebook   *NotebookMetadata
	Build      *BuildMetadata
	Infra      *InfraMetadata
	Schema     *SchemaMetadata
	Structured *StructuredMetadata
	SQL        *SQLMetadata
	Log        *LogMetadata
}

// CodeMetadata contains metadata for code chunks from Tree-sitter parsing.
type CodeMetadata struct {
	// Language is the programming language (go, python, javascript, etc.)
	Language string

	// FunctionName is the function/method name.
	FunctionName string

	// ClassName is the class/struct/interface name.
	ClassName string

	// Signature is the full signature (e.g., "func(x int) (string, error)").
	Signature string

	// ReturnType is the return type annotation.
	ReturnType string

	// Parameters contains parameter names.
	Parameters []string

	// Visibility is normalized across languages: public, private, protected, internal, package.
	Visibility string

	// IsAsync indicates an async/await function.
	IsAsync bool

	// IsStatic indicates a static method.
	IsStatic bool

	// IsExported indicates an exported/public symbol.
	IsExported bool

	// IsGenerator indicates a generator function (JS/Python).
	IsGenerator bool

	// IsGetter indicates a getter method.
	IsGetter bool

	// IsSetter indicates a setter method.
	IsSetter bool

	// IsConstructor indicates a constructor method.
	IsConstructor bool

	// Decorators contains decorator/annotation names.
	Decorators []string

	// Docstring is the extracted documentation.
	Docstring string

	// Namespace is the package/module/namespace path.
	Namespace string

	// ParentClass is the containing class for methods.
	ParentClass string

	// Implements contains interfaces implemented.
	Implements []string

	// LineStart is the starting line number (1-indexed).
	LineStart int

	// LineEnd is the ending line number (1-indexed).
	LineEnd int
}

// DocumentMetadata contains metadata for document chunks (Markdown, HTML, DOCX, PDF, etc.)
type DocumentMetadata struct {
	// Heading is the section heading text.
	Heading string

	// HeadingLevel is the heading depth (1-6).
	HeadingLevel int

	// SectionPath is the full path (e.g., "Chapter 1 > Section 2 > Subsection A").
	SectionPath string

	// SectionNumber is the section numbering (e.g., "1.2.3").
	SectionNumber string

	// Author is the document author.
	Author string

	// CreatedDate is the document creation date.
	CreatedDate time.Time

	// ModifiedDate is the document modification date.
	ModifiedDate time.Time

	// PageNumber is the page number (1-indexed, for PDFs).
	PageNumber int

	// PageCount is the total pages in source document.
	PageCount int

	// WordCount is the word count in chunk.
	WordCount int

	// HasCodeBlock indicates the chunk contains a code block/listing.
	HasCodeBlock bool

	// CodeLanguage is the language of embedded code block.
	CodeLanguage string

	// ListDepth is the nesting depth if in a list.
	ListDepth int

	// IsTable indicates the chunk is/contains a table.
	IsTable bool

	// IsFootnote indicates the chunk is a footnote/endnote.
	IsFootnote bool

	// ExtractionQuality indicates PDF extraction quality: "high", "medium", "low".
	ExtractionQuality string
}

// NotebookMetadata contains metadata for Jupyter notebook cells.
type NotebookMetadata struct {
	// CellType is the cell type: code, markdown, raw.
	CellType string

	// CellIndex is the position in notebook (0-indexed).
	CellIndex int

	// ExecutionCount is the execution order number.
	ExecutionCount int

	// HasOutput indicates the cell has execution output.
	HasOutput bool

	// OutputTypes contains types of outputs: text, image, error, html, etc.
	OutputTypes []string

	// Kernel is the kernel name (python3, julia, ir, etc.)
	Kernel string
}

// BuildMetadata contains metadata for Dockerfile and Makefile chunks.
type BuildMetadata struct {
	// TargetName is the Make target name.
	TargetName string

	// Dependencies contains target dependencies.
	Dependencies []string

	// StageName is the Docker build stage name (from AS clause).
	StageName string

	// BaseImage is the base image (from FROM).
	BaseImage string
}

// InfraMetadata contains metadata for Terraform/HCL chunks.
type InfraMetadata struct {
	// ResourceType is the resource type (e.g., aws_instance, google_compute_instance).
	ResourceType string

	// ResourceName is the resource identifier.
	ResourceName string

	// BlockType is the block type: resource, data, variable, output, module, provider.
	BlockType string
}

// SchemaMetadata contains metadata for Protocol Buffers and GraphQL chunks.
type SchemaMetadata struct {
	// MessageName is the Protobuf message name.
	MessageName string

	// ServiceName is the service name (also used for GraphQL).
	ServiceName string

	// RPCName is the RPC method name.
	RPCName string

	// TypeName is the GraphQL type name.
	TypeName string

	// TypeKind is the GraphQL type kind: type, input, interface, union, enum, scalar.
	TypeKind string
}

// StructuredMetadata contains metadata for JSON, XML, TOML, CSV chunks.
type StructuredMetadata struct {
	// SchemaPath is the JSON path / TOML key path.
	SchemaPath string

	// ElementName is the XML element name.
	ElementName string

	// ElementPath is the full XML element path (e.g., "/catalog/book/title").
	ElementPath string

	// TablePath is the TOML table path (e.g., "servers.alpha").
	TablePath string

	// RecordIndex is the record number for arrays/sequences.
	RecordIndex int

	// RecordCount is the number of records in chunk.
	RecordCount int

	// KeyNames contains keys/columns present.
	KeyNames []string
}

// SQLMetadata contains metadata for SQL file chunks.
type SQLMetadata struct {
	// StatementType is the statement type: CREATE, ALTER, INSERT, SELECT, etc.
	StatementType string

	// ObjectType is the object type: TABLE, VIEW, INDEX, PROCEDURE, FUNCTION, TRIGGER.
	ObjectType string

	// TableName is the target table name.
	TableName string

	// ProcedureName is the stored procedure/function name.
	ProcedureName string

	// SQLDialect is the detected dialect: postgresql, mysql, sqlite, sqlserver, oracle.
	SQLDialect string
}

// LogMetadata contains metadata for log file chunks.
type LogMetadata struct {
	// TimeStart is the earliest timestamp in chunk.
	TimeStart time.Time

	// TimeEnd is the latest timestamp in chunk.
	TimeEnd time.Time

	// LogLevel is the predominant level: DEBUG, INFO, WARN, ERROR, FATAL.
	LogLevel string

	// LogFormat is the detected format: json, apache, nginx, syslog, custom.
	LogFormat string

	// ErrorCount is the count of error/fatal entries in chunk.
	ErrorCount int

	// SourceApp is the application name if detectable.
	SourceApp string
}
