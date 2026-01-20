package chunkers

import (
	"encoding/json"
	"testing"
	"time"
)

func TestChunkMetadataTypeDiscrimination(t *testing.T) {
	t.Run("code metadata type", func(t *testing.T) {
		meta := ChunkMetadata{
			Type:          ChunkTypeCode,
			TokenEstimate: 100,
			Code: &CodeMetadata{
				Language:     "go",
				FunctionName: "main",
			},
		}

		if meta.Type != ChunkTypeCode {
			t.Errorf("Type = %v, want %v", meta.Type, ChunkTypeCode)
		}
		if meta.Code == nil {
			t.Error("Code should not be nil")
		}
		if meta.Document != nil {
			t.Error("Document should be nil for code type")
		}
	})

	t.Run("document metadata type", func(t *testing.T) {
		meta := ChunkMetadata{
			Type:          ChunkTypeMarkdown,
			TokenEstimate: 50,
			Document: &DocumentMetadata{
				Heading:      "Introduction",
				HeadingLevel: 1,
			},
		}

		if meta.Type != ChunkTypeMarkdown {
			t.Errorf("Type = %v, want %v", meta.Type, ChunkTypeMarkdown)
		}
		if meta.Document == nil {
			t.Error("Document should not be nil")
		}
		if meta.Code != nil {
			t.Error("Code should be nil for document type")
		}
	})

	t.Run("empty metadata", func(t *testing.T) {
		meta := ChunkMetadata{}

		if meta.Type != "" {
			t.Errorf("Type = %v, want empty", meta.Type)
		}
		if meta.TokenEstimate != 0 {
			t.Errorf("TokenEstimate = %d, want 0", meta.TokenEstimate)
		}
	})
}

func TestCodeMetadataFields(t *testing.T) {
	meta := CodeMetadata{
		Language:      "python",
		FunctionName:  "process_data",
		ClassName:     "DataProcessor",
		Signature:     "def process_data(self, data: list) -> dict",
		ReturnType:    "dict",
		Parameters:    []string{"self", "data"},
		Visibility:    "public",
		IsAsync:       true,
		IsStatic:      false,
		IsExported:    true,
		IsGenerator:   false,
		IsGetter:      false,
		IsSetter:      false,
		IsConstructor: false,
		Decorators:    []string{"staticmethod", "cache"},
		Docstring:     "Processes the input data.",
		Namespace:     "mypackage",
		ParentClass:   "BaseProcessor",
		Implements:    []string{"Processor", "Serializable"},
		LineStart:     10,
		LineEnd:       25,
	}

	t.Run("basic fields", func(t *testing.T) {
		if meta.Language != "python" {
			t.Errorf("Language = %q, want %q", meta.Language, "python")
		}
		if meta.FunctionName != "process_data" {
			t.Errorf("FunctionName = %q, want %q", meta.FunctionName, "process_data")
		}
		if meta.ClassName != "DataProcessor" {
			t.Errorf("ClassName = %q, want %q", meta.ClassName, "DataProcessor")
		}
	})

	t.Run("array fields", func(t *testing.T) {
		if len(meta.Parameters) != 2 {
			t.Errorf("len(Parameters) = %d, want 2", len(meta.Parameters))
		}
		if len(meta.Decorators) != 2 {
			t.Errorf("len(Decorators) = %d, want 2", len(meta.Decorators))
		}
		if len(meta.Implements) != 2 {
			t.Errorf("len(Implements) = %d, want 2", len(meta.Implements))
		}
	})

	t.Run("boolean fields", func(t *testing.T) {
		if !meta.IsAsync {
			t.Error("IsAsync should be true")
		}
		if meta.IsStatic {
			t.Error("IsStatic should be false")
		}
		if !meta.IsExported {
			t.Error("IsExported should be true")
		}
	})

	t.Run("line numbers", func(t *testing.T) {
		if meta.LineStart != 10 {
			t.Errorf("LineStart = %d, want 10", meta.LineStart)
		}
		if meta.LineEnd != 25 {
			t.Errorf("LineEnd = %d, want 25", meta.LineEnd)
		}
		if meta.LineEnd < meta.LineStart {
			t.Error("LineEnd should be >= LineStart")
		}
	})
}

func TestCodeMetadataJSONSerialization(t *testing.T) {
	meta := CodeMetadata{
		Language:     "go",
		FunctionName: "TestFunc",
		Parameters:   []string{"ctx", "opts"},
		Decorators:   []string{},
		Implements:   nil,
		IsAsync:      true,
	}

	t.Run("serialize and deserialize", func(t *testing.T) {
		data, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded CodeMetadata
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Language != meta.Language {
			t.Errorf("Language = %q, want %q", decoded.Language, meta.Language)
		}
		if decoded.FunctionName != meta.FunctionName {
			t.Errorf("FunctionName = %q, want %q", decoded.FunctionName, meta.FunctionName)
		}
		if !decoded.IsAsync {
			t.Error("IsAsync should be true after deserialization")
		}
	})

	t.Run("nil array vs empty array", func(t *testing.T) {
		data, _ := json.Marshal(meta)

		// Empty slice may serialize to [] or be omitted with omitempty
		// Just verify it can round-trip without error
		var decoded CodeMetadata
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
	})
}

func TestDocumentMetadataFields(t *testing.T) {
	now := time.Now()
	meta := DocumentMetadata{
		Heading:           "Configuration",
		HeadingLevel:      2,
		SectionPath:       "Setup > Configuration",
		SectionNumber:     "2.1",
		Author:            "John Doe",
		CreatedDate:       now.Add(-24 * time.Hour),
		ModifiedDate:      now,
		PageNumber:        5,
		PageCount:         20,
		WordCount:         150,
		HasCodeBlock:      true,
		CodeLanguage:      "yaml",
		ListDepth:         2,
		IsTable:           false,
		IsFootnote:        false,
		ExtractionQuality: "high",
	}

	t.Run("heading fields", func(t *testing.T) {
		if meta.Heading != "Configuration" {
			t.Errorf("Heading = %q, want %q", meta.Heading, "Configuration")
		}
		if meta.HeadingLevel != 2 {
			t.Errorf("HeadingLevel = %d, want 2", meta.HeadingLevel)
		}
		if meta.SectionPath != "Setup > Configuration" {
			t.Errorf("SectionPath = %q, want %q", meta.SectionPath, "Setup > Configuration")
		}
	})

	t.Run("page fields", func(t *testing.T) {
		if meta.PageNumber != 5 {
			t.Errorf("PageNumber = %d, want 5", meta.PageNumber)
		}
		if meta.PageCount != 20 {
			t.Errorf("PageCount = %d, want 20", meta.PageCount)
		}
	})

	t.Run("content fields", func(t *testing.T) {
		if !meta.HasCodeBlock {
			t.Error("HasCodeBlock should be true")
		}
		if meta.CodeLanguage != "yaml" {
			t.Errorf("CodeLanguage = %q, want %q", meta.CodeLanguage, "yaml")
		}
	})

	t.Run("extraction quality", func(t *testing.T) {
		validQualities := []string{"high", "medium", "low"}
		found := false
		for _, q := range validQualities {
			if meta.ExtractionQuality == q {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ExtractionQuality = %q, want one of %v", meta.ExtractionQuality, validQualities)
		}
	})
}

func TestNotebookMetadataFields(t *testing.T) {
	meta := NotebookMetadata{
		CellType:       "code",
		CellIndex:      5,
		ExecutionCount: 10,
		HasOutput:      true,
		OutputTypes:    []string{"text", "image"},
		Kernel:         "python3",
	}

	t.Run("cell fields", func(t *testing.T) {
		if meta.CellType != "code" {
			t.Errorf("CellType = %q, want %q", meta.CellType, "code")
		}
		if meta.CellIndex != 5 {
			t.Errorf("CellIndex = %d, want 5", meta.CellIndex)
		}
	})

	t.Run("execution fields", func(t *testing.T) {
		if meta.ExecutionCount != 10 {
			t.Errorf("ExecutionCount = %d, want 10", meta.ExecutionCount)
		}
		if !meta.HasOutput {
			t.Error("HasOutput should be true")
		}
	})

	t.Run("output types array", func(t *testing.T) {
		if len(meta.OutputTypes) != 2 {
			t.Errorf("len(OutputTypes) = %d, want 2", len(meta.OutputTypes))
		}
		if meta.OutputTypes[0] != "text" {
			t.Errorf("OutputTypes[0] = %q, want %q", meta.OutputTypes[0], "text")
		}
	})
}

func TestBuildMetadataFields(t *testing.T) {
	meta := BuildMetadata{
		TargetName:   "build",
		Dependencies: []string{"test", "lint"},
		StageName:    "production",
		BaseImage:    "golang:1.22",
	}

	t.Run("target fields", func(t *testing.T) {
		if meta.TargetName != "build" {
			t.Errorf("TargetName = %q, want %q", meta.TargetName, "build")
		}
		if len(meta.Dependencies) != 2 {
			t.Errorf("len(Dependencies) = %d, want 2", len(meta.Dependencies))
		}
	})

	t.Run("docker fields", func(t *testing.T) {
		if meta.StageName != "production" {
			t.Errorf("StageName = %q, want %q", meta.StageName, "production")
		}
		if meta.BaseImage != "golang:1.22" {
			t.Errorf("BaseImage = %q, want %q", meta.BaseImage, "golang:1.22")
		}
	})
}

func TestInfraMetadataFields(t *testing.T) {
	meta := InfraMetadata{
		ResourceType: "aws_instance",
		ResourceName: "web_server",
		BlockType:    "resource",
	}

	t.Run("resource fields", func(t *testing.T) {
		if meta.ResourceType != "aws_instance" {
			t.Errorf("ResourceType = %q, want %q", meta.ResourceType, "aws_instance")
		}
		if meta.ResourceName != "web_server" {
			t.Errorf("ResourceName = %q, want %q", meta.ResourceName, "web_server")
		}
	})

	t.Run("block type", func(t *testing.T) {
		validTypes := []string{"resource", "data", "variable", "output", "module", "provider"}
		found := false
		for _, bt := range validTypes {
			if meta.BlockType == bt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("BlockType = %q, want one of %v", meta.BlockType, validTypes)
		}
	})
}

func TestSchemaMetadataFields(t *testing.T) {
	t.Run("protobuf schema", func(t *testing.T) {
		meta := SchemaMetadata{
			MessageName: "UserRequest",
			ServiceName: "UserService",
			RPCName:     "GetUser",
		}

		if meta.MessageName != "UserRequest" {
			t.Errorf("MessageName = %q, want %q", meta.MessageName, "UserRequest")
		}
		if meta.ServiceName != "UserService" {
			t.Errorf("ServiceName = %q, want %q", meta.ServiceName, "UserService")
		}
	})

	t.Run("graphql schema", func(t *testing.T) {
		meta := SchemaMetadata{
			TypeName: "User",
			TypeKind: "type",
		}

		if meta.TypeName != "User" {
			t.Errorf("TypeName = %q, want %q", meta.TypeName, "User")
		}
		validKinds := []string{"type", "input", "interface", "union", "enum", "scalar"}
		found := false
		for _, k := range validKinds {
			if meta.TypeKind == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("TypeKind = %q, want one of %v", meta.TypeKind, validKinds)
		}
	})
}

func TestStructuredMetadataFields(t *testing.T) {
	meta := StructuredMetadata{
		SchemaPath:  "$.users[0].name",
		ElementName: "user",
		ElementPath: "/catalog/book/title",
		TablePath:   "servers.alpha",
		RecordIndex: 0,
		RecordCount: 100,
		KeyNames:    []string{"id", "name", "email"},
	}

	t.Run("path fields", func(t *testing.T) {
		if meta.SchemaPath != "$.users[0].name" {
			t.Errorf("SchemaPath = %q, want %q", meta.SchemaPath, "$.users[0].name")
		}
		if meta.ElementPath != "/catalog/book/title" {
			t.Errorf("ElementPath = %q, want %q", meta.ElementPath, "/catalog/book/title")
		}
	})

	t.Run("record fields", func(t *testing.T) {
		if meta.RecordCount != 100 {
			t.Errorf("RecordCount = %d, want 100", meta.RecordCount)
		}
		if len(meta.KeyNames) != 3 {
			t.Errorf("len(KeyNames) = %d, want 3", len(meta.KeyNames))
		}
	})
}

func TestSQLMetadataFields(t *testing.T) {
	meta := SQLMetadata{
		StatementType: "CREATE",
		ObjectType:    "TABLE",
		TableName:     "users",
		ProcedureName: "",
		SQLDialect:    "postgresql",
	}

	t.Run("statement fields", func(t *testing.T) {
		if meta.StatementType != "CREATE" {
			t.Errorf("StatementType = %q, want %q", meta.StatementType, "CREATE")
		}
		if meta.ObjectType != "TABLE" {
			t.Errorf("ObjectType = %q, want %q", meta.ObjectType, "TABLE")
		}
	})

	t.Run("dialect", func(t *testing.T) {
		validDialects := []string{"postgresql", "mysql", "sqlite", "sqlserver", "oracle"}
		found := false
		for _, d := range validDialects {
			if meta.SQLDialect == d {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SQLDialect = %q, want one of %v", meta.SQLDialect, validDialects)
		}
	})
}

func TestLogMetadataFields(t *testing.T) {
	now := time.Now()
	meta := LogMetadata{
		TimeStart:  now.Add(-1 * time.Hour),
		TimeEnd:    now,
		LogLevel:   "ERROR",
		LogFormat:  "json",
		ErrorCount: 5,
		SourceApp:  "my-service",
	}

	t.Run("time fields", func(t *testing.T) {
		if meta.TimeEnd.Before(meta.TimeStart) {
			t.Error("TimeEnd should be after TimeStart")
		}
	})

	t.Run("level field", func(t *testing.T) {
		validLevels := []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
		found := false
		for _, l := range validLevels {
			if meta.LogLevel == l {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("LogLevel = %q, want one of %v", meta.LogLevel, validLevels)
		}
	})

	t.Run("format field", func(t *testing.T) {
		validFormats := []string{"json", "apache", "nginx", "syslog", "custom"}
		found := false
		for _, f := range validFormats {
			if meta.LogFormat == f {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("LogFormat = %q, want one of %v", meta.LogFormat, validFormats)
		}
	})

	t.Run("error count", func(t *testing.T) {
		if meta.ErrorCount < 0 {
			t.Error("ErrorCount should be >= 0")
		}
	})
}

func TestChunkMetadataJSONRoundTrip(t *testing.T) {
	meta := ChunkMetadata{
		Type:          ChunkTypeCode,
		TokenEstimate: 42,
		Code: &CodeMetadata{
			Language:     "go",
			FunctionName: "TestFunc",
			Parameters:   []string{"t", "testing.T"},
			IsAsync:      false,
		},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ChunkMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != meta.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, meta.Type)
	}
	if decoded.TokenEstimate != meta.TokenEstimate {
		t.Errorf("TokenEstimate = %d, want %d", decoded.TokenEstimate, meta.TokenEstimate)
	}
	if decoded.Code == nil {
		t.Fatal("Code should not be nil after round-trip")
	}
	if decoded.Code.FunctionName != meta.Code.FunctionName {
		t.Errorf("Code.FunctionName = %q, want %q", decoded.Code.FunctionName, meta.Code.FunctionName)
	}
}

func TestChunkMetadataWithNilPointers(t *testing.T) {
	t.Run("all nil pointers", func(t *testing.T) {
		meta := ChunkMetadata{
			Type:          ChunkTypeUnknown,
			TokenEstimate: 10,
		}

		// Should be safe to access all nil pointers
		if meta.Code != nil {
			t.Error("Code should be nil")
		}
		if meta.Document != nil {
			t.Error("Document should be nil")
		}
		if meta.Notebook != nil {
			t.Error("Notebook should be nil")
		}
		if meta.Build != nil {
			t.Error("Build should be nil")
		}
		if meta.Infra != nil {
			t.Error("Infra should be nil")
		}
		if meta.Schema != nil {
			t.Error("Schema should be nil")
		}
		if meta.Structured != nil {
			t.Error("Structured should be nil")
		}
		if meta.SQL != nil {
			t.Error("SQL should be nil")
		}
		if meta.Log != nil {
			t.Error("Log should be nil")
		}
	})

	t.Run("serialize with nil pointers", func(t *testing.T) {
		meta := ChunkMetadata{
			Type:          ChunkTypeProse,
			TokenEstimate: 50,
		}

		data, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ChunkMetadata
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		// All pointers should still be nil
		if decoded.Code != nil {
			t.Error("Code should be nil after deserialization")
		}
	})
}

func TestChunkTypeConstants(t *testing.T) {
	types := []ChunkType{
		ChunkTypeCode,
		ChunkTypeMarkdown,
		ChunkTypeProse,
		ChunkTypeStructured,
		ChunkTypeUnknown,
	}

	for _, ct := range types {
		if ct == "" {
			t.Error("ChunkType constant should not be empty")
		}
	}

	// Verify distinctness
	seen := make(map[ChunkType]bool)
	for _, ct := range types {
		if seen[ct] {
			t.Errorf("Duplicate ChunkType: %v", ct)
		}
		seen[ct] = true
	}
}
