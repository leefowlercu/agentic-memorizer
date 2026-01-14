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
		ID:               "chunk-1",
		FilePath:         "/test/file.go",
		Index:            0,
		Content:          "func main() {}",
		ContentHash:      "xyz789",
		StartOffset:      0,
		EndOffset:        14,
		ChunkType:        "code",
		FunctionName:     "main",
		Summary:          "Main function",
		EmbeddingVersion: 1,
		TokenCount:       5,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if chunk.ID != "chunk-1" {
		t.Errorf("ID = %q, want %q", chunk.ID, "chunk-1")
	}
	if chunk.FunctionName != "main" {
		t.Errorf("FunctionName = %q, want %q", chunk.FunctionName, "main")
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
