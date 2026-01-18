package graph

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 6379 {
		t.Errorf("Port = %d, want %d", cfg.Port, 6379)
	}
	if cfg.GraphName != "memorizer" {
		t.Errorf("GraphName = %q, want %q", cfg.GraphName, "memorizer")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, 3)
	}
	if cfg.RetryDelay != time.Second {
		t.Errorf("RetryDelay = %v, want %v", cfg.RetryDelay, time.Second)
	}
	if cfg.WriteQueueSize != 1000 {
		t.Errorf("WriteQueueSize = %d, want %d", cfg.WriteQueueSize, 1000)
	}
}

func TestNewFalkorDBGraph(t *testing.T) {
	g := NewFalkorDBGraph()

	if g == nil {
		t.Fatal("NewFalkorDBGraph returned nil")
	}
	if g.config.Host != "localhost" {
		t.Errorf("config.Host = %q, want %q", g.config.Host, "localhost")
	}
	if g.logger == nil {
		t.Error("logger should not be nil")
	}
	if g.writeQueue == nil {
		t.Error("writeQueue should not be nil")
	}
	if cap(g.writeQueue) != DefaultConfig().WriteQueueSize {
		t.Errorf("writeQueue capacity = %d, want %d", cap(g.writeQueue), DefaultConfig().WriteQueueSize)
	}
	if g.stopChan == nil {
		t.Error("stopChan should not be nil")
	}
}

func TestNewFalkorDBGraphWithOptions(t *testing.T) {
	customConfig := Config{
		Host:       "custom-host",
		Port:       6380,
		GraphName:  "custom-graph",
		MaxRetries: 5,
		RetryDelay: 2 * time.Second,
		WriteQueueSize: 42,
	}

	g := NewFalkorDBGraph(WithConfig(customConfig))

	if g.config.Host != "custom-host" {
		t.Errorf("config.Host = %q, want %q", g.config.Host, "custom-host")
	}
	if g.config.Port != 6380 {
		t.Errorf("config.Port = %d, want %d", g.config.Port, 6380)
	}
	if g.config.GraphName != "custom-graph" {
		t.Errorf("config.GraphName = %q, want %q", g.config.GraphName, "custom-graph")
	}
	if g.config.MaxRetries != 5 {
		t.Errorf("config.MaxRetries = %d, want %d", g.config.MaxRetries, 5)
	}
	if cap(g.writeQueue) != 42 {
		t.Errorf("writeQueue capacity = %d, want %d", cap(g.writeQueue), 42)
	}
}

func TestFalkorDBGraphName(t *testing.T) {
	g := NewFalkorDBGraph()
	if g.Name() != "graph" {
		t.Errorf("Name() = %q, want %q", g.Name(), "graph")
	}
}

func TestFalkorDBGraphIsConnected(t *testing.T) {
	g := NewFalkorDBGraph()

	if g.IsConnected() {
		t.Error("IsConnected() should return false when not connected")
	}
}

func TestEscapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello'world", "hello\\'world"},
		{"test\\path", "test\\\\path"},
		{"it's a \"test\"", "it\\'s a \"test\""},
		{"path\\with'quotes", "path\\\\with\\'quotes"},
		{"", ""},
	}

	for _, tt := range tests {
		result := escapeString(tt.input)
		if result != tt.expected {
			t.Errorf("escapeString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "hello"},
		{"UPPERCASE", "uppercase"},
		{"MixedCase", "mixedcase"},
		{"already lowercase", "already lowercase"},
		{"With123Numbers", "with123numbers"},
		{"", ""},
	}

	for _, tt := range tests {
		result := normalizeString(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNodeLabels(t *testing.T) {
	if LabelFile != "File" {
		t.Errorf("LabelFile = %q, want %q", LabelFile, "File")
	}
	if LabelChunk != "Chunk" {
		t.Errorf("LabelChunk = %q, want %q", LabelChunk, "Chunk")
	}
	if LabelDirectory != "Directory" {
		t.Errorf("LabelDirectory = %q, want %q", LabelDirectory, "Directory")
	}
	if LabelTag != "Tag" {
		t.Errorf("LabelTag = %q, want %q", LabelTag, "Tag")
	}
	if LabelTopic != "Topic" {
		t.Errorf("LabelTopic = %q, want %q", LabelTopic, "Topic")
	}
	if LabelEntity != "Entity" {
		t.Errorf("LabelEntity = %q, want %q", LabelEntity, "Entity")
	}
}

func TestRelationshipTypes(t *testing.T) {
	if RelContains != "CONTAINS" {
		t.Errorf("RelContains = %q, want %q", RelContains, "CONTAINS")
	}
	if RelHasChunk != "HAS_CHUNK" {
		t.Errorf("RelHasChunk = %q, want %q", RelHasChunk, "HAS_CHUNK")
	}
	if RelHasTag != "HAS_TAG" {
		t.Errorf("RelHasTag = %q, want %q", RelHasTag, "HAS_TAG")
	}
	if RelCoversTopic != "COVERS_TOPIC" {
		t.Errorf("RelCoversTopic = %q, want %q", RelCoversTopic, "COVERS_TOPIC")
	}
	if RelMentions != "MENTIONS" {
		t.Errorf("RelMentions = %q, want %q", RelMentions, "MENTIONS")
	}
	if RelReferences != "REFERENCES" {
		t.Errorf("RelReferences = %q, want %q", RelReferences, "REFERENCES")
	}
	if RelSimilarTo != "SIMILAR_TO" {
		t.Errorf("RelSimilarTo = %q, want %q", RelSimilarTo, "SIMILAR_TO")
	}
	if RelDependsOn != "DEPENDS_ON" {
		t.Errorf("RelDependsOn = %q, want %q", RelDependsOn, "DEPENDS_ON")
	}
}

func TestFileNodeFields(t *testing.T) {
	now := time.Now()
	file := FileNode{
		Path:            "/test/file.go",
		Name:            "file.go",
		Extension:       ".go",
		MIMEType:        "text/x-go",
		Language:        "go",
		Size:            1024,
		ModTime:         now,
		ContentHash:     "abc123",
		MetadataHash:    "def456",
		Summary:         "Test file",
		Complexity:      5,
		AnalyzedAt:      now,
		AnalysisVersion: 1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if file.Path != "/test/file.go" {
		t.Errorf("Path = %q, want %q", file.Path, "/test/file.go")
	}
	if file.Size != 1024 {
		t.Errorf("Size = %d, want %d", file.Size, 1024)
	}
	if file.Language != "go" {
		t.Errorf("Language = %q, want %q", file.Language, "go")
	}
}

func TestChunkNodeFields(t *testing.T) {
	now := time.Now()
	chunk := ChunkNode{
		ID:          "chunk-1",
		FilePath:    "/test/file.go",
		Index:       0,
		Content:     "func main() {}",
		ContentHash: "xyz789",
		StartOffset: 0,
		EndOffset:   14,
		ChunkType:   "code",
		Summary:     "Main function",
		TokenCount:  5,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if chunk.ID != "chunk-1" {
		t.Errorf("ID = %q, want %q", chunk.ID, "chunk-1")
	}
	if chunk.ChunkType != "code" {
		t.Errorf("ChunkType = %q, want %q", chunk.ChunkType, "code")
	}
}

func TestDirectoryNodeFields(t *testing.T) {
	now := time.Now()
	dir := DirectoryNode{
		Path:         "/test/dir",
		Name:         "dir",
		IsRemembered: true,
		FileCount:    10,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if dir.Path != "/test/dir" {
		t.Errorf("Path = %q, want %q", dir.Path, "/test/dir")
	}
	if !dir.IsRemembered {
		t.Error("IsRemembered should be true")
	}
}

func TestTagNodeFields(t *testing.T) {
	now := time.Now()
	tag := TagNode{
		Name:           "golang",
		NormalizedName: "golang",
		UsageCount:     5,
		CreatedAt:      now,
	}

	if tag.Name != "golang" {
		t.Errorf("Name = %q, want %q", tag.Name, "golang")
	}
	if tag.UsageCount != 5 {
		t.Errorf("UsageCount = %d, want %d", tag.UsageCount, 5)
	}
}

func TestTopicNodeFields(t *testing.T) {
	now := time.Now()
	topic := TopicNode{
		Name:           "Programming",
		NormalizedName: "programming",
		Description:    "About programming",
		UsageCount:     3,
		CreatedAt:      now,
	}

	if topic.Name != "Programming" {
		t.Errorf("Name = %q, want %q", topic.Name, "Programming")
	}
	if topic.Description != "About programming" {
		t.Errorf("Description = %q, want %q", topic.Description, "About programming")
	}
}

func TestEntityNodeFields(t *testing.T) {
	now := time.Now()
	entity := EntityNode{
		Name:           "Go",
		Type:           "language",
		NormalizedName: "go",
		UsageCount:     10,
		CreatedAt:      now,
	}

	if entity.Name != "Go" {
		t.Errorf("Name = %q, want %q", entity.Name, "Go")
	}
	if entity.Type != "language" {
		t.Errorf("Type = %q, want %q", entity.Type, "language")
	}
}

func TestTopicStruct(t *testing.T) {
	topic := Topic{
		Name:       "Testing",
		Confidence: 0.95,
	}

	if topic.Name != "Testing" {
		t.Errorf("Name = %q, want %q", topic.Name, "Testing")
	}
	if topic.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want %f", topic.Confidence, 0.95)
	}
}

func TestEntityStruct(t *testing.T) {
	entity := Entity{
		Name: "Python",
		Type: "language",
	}

	if entity.Name != "Python" {
		t.Errorf("Name = %q, want %q", entity.Name, "Python")
	}
	if entity.Type != "language" {
		t.Errorf("Type = %q, want %q", entity.Type, "language")
	}
}

func TestReferenceStruct(t *testing.T) {
	ref := Reference{
		Type:   "file",
		Target: "/path/to/file.go",
	}

	if ref.Type != "file" {
		t.Errorf("Type = %q, want %q", ref.Type, "file")
	}
	if ref.Target != "/path/to/file.go" {
		t.Errorf("Target = %q, want %q", ref.Target, "/path/to/file.go")
	}
}

func TestQueryResult(t *testing.T) {
	qr := QueryResult{
		Columns: []string{"name", "age"},
		Rows: [][]any{
			{"Alice", 30},
			{"Bob", 25},
		},
		Stats: QueryStats{
			NodesCreated:     2,
			RelationsCreated: 1,
			ExecutionTimeMs:  5.5,
		},
	}

	if len(qr.Columns) != 2 {
		t.Errorf("len(Columns) = %d, want %d", len(qr.Columns), 2)
	}
	if len(qr.Rows) != 2 {
		t.Errorf("len(Rows) = %d, want %d", len(qr.Rows), 2)
	}
	if qr.Stats.NodesCreated != 2 {
		t.Errorf("NodesCreated = %d, want %d", qr.Stats.NodesCreated, 2)
	}
}

func TestGraphSnapshot(t *testing.T) {
	now := time.Now()
	snapshot := GraphSnapshot{
		Files:              []FileNode{{Path: "/test/file.go"}},
		Directories:        []DirectoryNode{{Path: "/test"}},
		Tags:               []TagNode{{Name: "go"}},
		Topics:             []TopicNode{{Name: "Programming"}},
		Entities:           []EntityNode{{Name: "Go"}},
		TotalChunks:        10,
		TotalRelationships: 20,
		ExportedAt:         now,
		Version:            1,
	}

	if len(snapshot.Files) != 1 {
		t.Errorf("len(Files) = %d, want %d", len(snapshot.Files), 1)
	}
	if snapshot.TotalChunks != 10 {
		t.Errorf("TotalChunks = %d, want %d", snapshot.TotalChunks, 10)
	}
	if snapshot.Version != 1 {
		t.Errorf("Version = %d, want %d", snapshot.Version, 1)
	}
}

func TestFileWithRelations(t *testing.T) {
	fwr := FileWithRelations{
		File:       FileNode{Path: "/test/file.go"},
		Tags:       []string{"go", "test"},
		Topics:     []Topic{{Name: "Programming", Confidence: 0.9}},
		Entities:   []Entity{{Name: "Go", Type: "language"}},
		References: []Reference{{Type: "file", Target: "/other/file.go"}},
		ChunkCount: 5,
	}

	if fwr.File.Path != "/test/file.go" {
		t.Errorf("File.Path = %q, want %q", fwr.File.Path, "/test/file.go")
	}
	if len(fwr.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want %d", len(fwr.Tags), 2)
	}
	if fwr.ChunkCount != 5 {
		t.Errorf("ChunkCount = %d, want %d", fwr.ChunkCount, 5)
	}
}

func TestOperationsWithoutConnection(t *testing.T) {
	g := NewFalkorDBGraph()

	t.Run("UpsertFile", func(t *testing.T) {
		err := g.UpsertFile(nil, &FileNode{Path: "/test"})
		if err == nil {
			t.Error("Expected error when not connected")
		}
	})

	t.Run("DeleteFile", func(t *testing.T) {
		err := g.DeleteFile(nil, "/test")
		if err == nil {
			t.Error("Expected error when not connected")
		}
	})

	t.Run("GetFile", func(t *testing.T) {
		_, err := g.GetFile(nil, "/test")
		if err == nil {
			t.Error("Expected error when not connected")
		}
	})

	t.Run("Query", func(t *testing.T) {
		_, err := g.Query(nil, "MATCH (n) RETURN n")
		if err == nil {
			t.Error("Expected error when not connected")
		}
	})

	t.Run("ExportSnapshot", func(t *testing.T) {
		_, err := g.ExportSnapshot(nil)
		if err == nil {
			t.Error("Expected error when not connected")
		}
	})

	t.Run("DeleteFilesUnderPath", func(t *testing.T) {
		err := g.DeleteFilesUnderPath(nil, "/test")
		if err == nil {
			t.Error("Expected error when not connected")
		}
	})

	t.Run("DeleteDirectoriesUnderPath", func(t *testing.T) {
		err := g.DeleteDirectoriesUnderPath(nil, "/test")
		if err == nil {
			t.Error("Expected error when not connected")
		}
	})
}

func TestMetadataNodeLabels(t *testing.T) {
	labels := []struct {
		name     string
		label    string
		expected string
	}{
		{"CodeMeta", LabelCodeMeta, "CodeMeta"},
		{"DocumentMeta", LabelDocumentMeta, "DocumentMeta"},
		{"NotebookMeta", LabelNotebookMeta, "NotebookMeta"},
		{"BuildMeta", LabelBuildMeta, "BuildMeta"},
		{"InfraMeta", LabelInfraMeta, "InfraMeta"},
		{"SchemaMeta", LabelSchemaMeta, "SchemaMeta"},
		{"StructuredMeta", LabelStructuredMeta, "StructuredMeta"},
		{"SQLMeta", LabelSQLMeta, "SQLMeta"},
		{"LogMeta", LabelLogMeta, "LogMeta"},
		{"ChunkEmbedding", LabelChunkEmbedding, "ChunkEmbedding"},
	}

	for _, tt := range labels {
		t.Run(tt.name, func(t *testing.T) {
			if tt.label != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.label, tt.expected)
			}
		})
	}
}

func TestMetadataRelationshipTypes(t *testing.T) {
	relationships := []struct {
		name     string
		rel      string
		expected string
	}{
		{"HasCodeMeta", RelHasCodeMeta, "HAS_CODE_META"},
		{"HasDocMeta", RelHasDocMeta, "HAS_DOC_META"},
		{"HasNotebookMeta", RelHasNotebookMeta, "HAS_NOTEBOOK_META"},
		{"HasBuildMeta", RelHasBuildMeta, "HAS_BUILD_META"},
		{"HasInfraMeta", RelHasInfraMeta, "HAS_INFRA_META"},
		{"HasSchemaMeta", RelHasSchemaMeta, "HAS_SCHEMA_META"},
		{"HasStructMeta", RelHasStructMeta, "HAS_STRUCT_META"},
		{"HasSQLMeta", RelHasSQLMeta, "HAS_SQL_META"},
		{"HasLogMeta", RelHasLogMeta, "HAS_LOG_META"},
		{"HasEmbedding", RelHasEmbedding, "HAS_EMBEDDING"},
	}

	for _, tt := range relationships {
		t.Run(tt.name, func(t *testing.T) {
			if tt.rel != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.rel, tt.expected)
			}
		})
	}
}

func TestCodeMetaNodeFields(t *testing.T) {
	meta := CodeMetaNode{
		Language:     "go",
		FunctionName: "main",
		ClassName:    "Handler",
		Signature:    "func main()",
		ReturnType:   "error",
		Parameters:   []string{"ctx", "opts"},
		Decorators:   []string{"test"},
		Implements:   []string{"http.Handler"},
		Visibility:   "public",
		Docstring:    "Main entry point",
		Namespace:    "main",
		ParentClass:  "",
		IsAsync:      false,
		IsStatic:     true,
		IsExported:   true,
		LineStart:    10,
		LineEnd:      25,
	}

	if meta.Language != "go" {
		t.Errorf("Language = %q, want %q", meta.Language, "go")
	}
	if meta.FunctionName != "main" {
		t.Errorf("FunctionName = %q, want %q", meta.FunctionName, "main")
	}
	if len(meta.Parameters) != 2 {
		t.Errorf("len(Parameters) = %d, want 2", len(meta.Parameters))
	}
	if !meta.IsExported {
		t.Error("IsExported should be true")
	}
	if meta.LineStart != 10 {
		t.Errorf("LineStart = %d, want 10", meta.LineStart)
	}
}

func TestDocumentMetaNodeFields(t *testing.T) {
	meta := DocumentMetaNode{
		Heading:      "Introduction",
		HeadingLevel: 1,
		SectionPath:  []string{"Chapter 1", "Introduction"},
		PageNumber:   5,
		ListType:     "bullet",
		ListDepth:    2,
		IsFootnote:   false,
		IsCitation:   true,
		IsBlockquote: false,
	}

	if meta.Heading != "Introduction" {
		t.Errorf("Heading = %q, want %q", meta.Heading, "Introduction")
	}
	if meta.HeadingLevel != 1 {
		t.Errorf("HeadingLevel = %d, want 1", meta.HeadingLevel)
	}
	if len(meta.SectionPath) != 2 {
		t.Errorf("len(SectionPath) = %d, want 2", len(meta.SectionPath))
	}
	if !meta.IsCitation {
		t.Error("IsCitation should be true")
	}
}

func TestNotebookMetaNodeFields(t *testing.T) {
	meta := NotebookMetaNode{
		CellIndex:       5,
		CellType:        "code",
		ExecutionCount:  10,
		HasOutput:       true,
		OutputTruncated: false,
	}

	if meta.CellIndex != 5 {
		t.Errorf("CellIndex = %d, want 5", meta.CellIndex)
	}
	if meta.CellType != "code" {
		t.Errorf("CellType = %q, want %q", meta.CellType, "code")
	}
	if !meta.HasOutput {
		t.Error("HasOutput should be true")
	}
}

func TestBuildMetaNodeFields(t *testing.T) {
	meta := BuildMetaNode{
		TargetName:   "build",
		TargetType:   "phony",
		Dependencies: []string{"test", "lint"},
		StageName:    "production",
		ImageName:    "golang:1.22",
		BuildArgs:    []string{"--platform=linux/amd64"},
	}

	if meta.TargetName != "build" {
		t.Errorf("TargetName = %q, want %q", meta.TargetName, "build")
	}
	if len(meta.Dependencies) != 2 {
		t.Errorf("len(Dependencies) = %d, want 2", len(meta.Dependencies))
	}
	if meta.StageName != "production" {
		t.Errorf("StageName = %q, want %q", meta.StageName, "production")
	}
}

func TestInfraMetaNodeFields(t *testing.T) {
	meta := InfraMetaNode{
		ResourceType: "aws_instance",
		ResourceName: "web_server",
		Provider:     "aws",
		BlockType:    "resource",
		ModuleName:   "compute",
		References:   []string{"aws_vpc.main", "aws_subnet.public"},
	}

	if meta.ResourceType != "aws_instance" {
		t.Errorf("ResourceType = %q, want %q", meta.ResourceType, "aws_instance")
	}
	if meta.BlockType != "resource" {
		t.Errorf("BlockType = %q, want %q", meta.BlockType, "resource")
	}
	if len(meta.References) != 2 {
		t.Errorf("len(References) = %d, want 2", len(meta.References))
	}
}

func TestSchemaMetaNodeFields(t *testing.T) {
	meta := SchemaMetaNode{
		MessageName:  "UserRequest",
		ServiceName:  "UserService",
		RPCName:      "GetUser",
		TypeName:     "User",
		Fields:       []string{"id", "name", "email"},
		IsDeprecated: false,
	}

	if meta.MessageName != "UserRequest" {
		t.Errorf("MessageName = %q, want %q", meta.MessageName, "UserRequest")
	}
	if meta.ServiceName != "UserService" {
		t.Errorf("ServiceName = %q, want %q", meta.ServiceName, "UserService")
	}
	if len(meta.Fields) != 3 {
		t.Errorf("len(Fields) = %d, want 3", len(meta.Fields))
	}
}

func TestStructuredMetaNodeFields(t *testing.T) {
	meta := StructuredMetaNode{
		RecordIndex: 0,
		RecordCount: 100,
		KeyNames:    []string{"id", "name", "value"},
		ArrayPath:   "$.users",
	}

	if meta.RecordCount != 100 {
		t.Errorf("RecordCount = %d, want 100", meta.RecordCount)
	}
	if len(meta.KeyNames) != 3 {
		t.Errorf("len(KeyNames) = %d, want 3", len(meta.KeyNames))
	}
	if meta.ArrayPath != "$.users" {
		t.Errorf("ArrayPath = %q, want %q", meta.ArrayPath, "$.users")
	}
}

func TestSQLMetaNodeFields(t *testing.T) {
	meta := SQLMetaNode{
		StatementType: "CREATE",
		TableNames:    []string{"users"},
		JoinedTables:  []string{"roles", "permissions"},
		HasSubquery:   true,
	}

	if meta.StatementType != "CREATE" {
		t.Errorf("StatementType = %q, want %q", meta.StatementType, "CREATE")
	}
	if len(meta.TableNames) != 1 {
		t.Errorf("len(TableNames) = %d, want 1", len(meta.TableNames))
	}
	if !meta.HasSubquery {
		t.Error("HasSubquery should be true")
	}
}

func TestLogMetaNodeFields(t *testing.T) {
	meta := LogMetaNode{
		LogLevel:   "ERROR",
		TimeRange:  [2]string{"2024-01-01T00:00:00Z", "2024-01-01T01:00:00Z"},
		Source:     "my-service",
		EntryCount: 50,
	}

	if meta.LogLevel != "ERROR" {
		t.Errorf("LogLevel = %q, want %q", meta.LogLevel, "ERROR")
	}
	if meta.TimeRange[0] != "2024-01-01T00:00:00Z" {
		t.Errorf("TimeRange[0] = %q, want %q", meta.TimeRange[0], "2024-01-01T00:00:00Z")
	}
	if meta.EntryCount != 50 {
		t.Errorf("EntryCount = %d, want 50", meta.EntryCount)
	}
}

func TestChunkEmbeddingNodeFields(t *testing.T) {
	now := time.Now()
	embedding := ChunkEmbeddingNode{
		Provider:   "openai",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		Embedding:  make([]float32, 1536),
		CreatedAt:  now,
	}

	if embedding.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", embedding.Provider, "openai")
	}
	if embedding.Model != "text-embedding-3-small" {
		t.Errorf("Model = %q, want %q", embedding.Model, "text-embedding-3-small")
	}
	if embedding.Dimensions != 1536 {
		t.Errorf("Dimensions = %d, want 1536", embedding.Dimensions)
	}
	if len(embedding.Embedding) != 1536 {
		t.Errorf("len(Embedding) = %d, want 1536", len(embedding.Embedding))
	}
}

func TestChunkEmbeddingMultipleProviders(t *testing.T) {
	// Test that we can have embeddings from different providers
	openaiEmbed := ChunkEmbeddingNode{
		Provider:   "openai",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		Embedding:  make([]float32, 1536),
	}

	cohereEmbed := ChunkEmbeddingNode{
		Provider:   "cohere",
		Model:      "embed-english-v3.0",
		Dimensions: 1024,
		Embedding:  make([]float32, 1024),
	}

	if openaiEmbed.Provider == cohereEmbed.Provider {
		t.Error("Providers should be different")
	}
	if openaiEmbed.Model == cohereEmbed.Model {
		t.Error("Models should be different")
	}
	if openaiEmbed.Dimensions == cohereEmbed.Dimensions {
		t.Error("Dimensions should be different for this test")
	}
}
