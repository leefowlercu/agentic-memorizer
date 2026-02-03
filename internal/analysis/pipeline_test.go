package analysis

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
)

func TestNewPipeline(t *testing.T) {
	t.Run("CreatesPipelineWithDefaults", func(t *testing.T) {
		cfg := PipelineConfig{}
		p := NewPipeline(cfg)

		if p == nil {
			t.Fatal("expected non-nil pipeline")
		}
		if p.fileReader == nil {
			t.Error("expected fileReader to be set")
		}
		if p.chunker == nil {
			t.Error("expected chunker to be set")
		}
		if p.semantic == nil {
			t.Error("expected semantic to be set")
		}
		if p.embeddings == nil {
			t.Error("expected embeddings to be set")
		}
		if p.persistence == nil {
			t.Error("expected persistence to be set")
		}
		if p.logger == nil {
			t.Error("expected logger to be set")
		}
	})

	t.Run("OptionsOverrideDefaults", func(t *testing.T) {
		mockReader := &mockFileReaderStage{}
		mockChunker := &mockChunkerStage{}
		mockSemantic := &mockSemanticStage{}
		mockEmbed := &mockEmbeddingsStage{}
		mockPersist := &mockPersistenceStage{}

		cfg := PipelineConfig{}
		p := NewPipeline(cfg,
			WithFileReader(mockReader),
			WithChunker(mockChunker),
			WithSemantic(mockSemantic),
			WithEmbeddings(mockEmbed),
			WithPersistence(mockPersist),
		)

		if p.fileReader != mockReader {
			t.Error("expected fileReader to be overridden")
		}
		if p.chunker != mockChunker {
			t.Error("expected chunker to be overridden")
		}
		if p.semantic != mockSemantic {
			t.Error("expected semantic to be overridden")
		}
		if p.embeddings != mockEmbed {
			t.Error("expected embeddings to be overridden")
		}
		if p.persistence != mockPersist {
			t.Error("expected persistence to be overridden")
		}
	})
}

func TestPipelineExecute(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("ExecutesAllStages", func(t *testing.T) {
		mockReader := &mockFileReaderStage{
			result: &FileReadResult{
				Info: mockFileInfo{
					name:    "test.txt",
					size:    12,
					modTime: time.Now(),
				},
				Kind:         ingest.KindText,
				MIMEType:     "text/plain",
				IngestMode:   ingest.ModeChunk,
				Content:      []byte("test content"),
				ContentHash:  "hash123",
				MetadataHash: "meta456",
			},
		}
		mockChunker := &mockChunkerStage{}
		mockSemantic := &mockSemanticStage{}
		mockEmbed := &mockEmbeddingsStage{embedding: []float32{0.1, 0.2}}
		mockPersist := &mockPersistenceStage{}

		p := NewPipeline(PipelineConfig{},
			WithFileReader(mockReader),
			WithChunker(mockChunker),
			WithSemantic(mockSemantic),
			WithEmbeddings(mockEmbed),
			WithPersistence(mockPersist),
		)

		pctx := NewPipelineContext(WorkItem{FilePath: testFile}, DegradationFull, nil)
		err := p.Execute(context.Background(), pctx)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file reader was called
		if pctx.FileResult == nil {
			t.Error("expected FileResult to be populated")
		}

		// Verify chunker was called
		if pctx.ChunkResult == nil {
			t.Error("expected ChunkResult to be populated")
		}

		// Verify semantic analysis was called
		if pctx.SemanticResult == nil {
			t.Error("expected SemanticResult to be populated")
		}

		// Verify embeddings were generated
		if pctx.Embeddings == nil {
			t.Error("expected Embeddings to be populated")
		}

		// Verify analysis result was built
		if pctx.AnalysisResult == nil {
			t.Fatal("expected AnalysisResult to be populated")
		}
		if pctx.AnalysisResult.ContentHash != "hash123" {
			t.Errorf("ContentHash = %q, want %q", pctx.AnalysisResult.ContentHash, "hash123")
		}
	})

	t.Run("StopsEarlyForMetadataOnly", func(t *testing.T) {
		mockReader := &mockFileReaderStage{
			result: &FileReadResult{
				Info: mockFileInfo{
					name:    "test.txt",
					size:    12,
					modTime: time.Now(),
				},
				Kind:         ingest.KindBinary,
				MIMEType:     "application/octet-stream",
				IngestMode:   ingest.ModeMetadataOnly,
				ContentHash:  "hash123",
				MetadataHash: "meta456",
			},
		}
		mockChunker := &mockChunkerStage{}
		mockSemantic := &mockSemanticStage{}

		p := NewPipeline(PipelineConfig{},
			WithFileReader(mockReader),
			WithChunker(mockChunker),
			WithSemantic(mockSemantic),
		)

		pctx := NewPipelineContext(WorkItem{FilePath: testFile}, DegradationFull, nil)
		err := p.Execute(context.Background(), pctx)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should have file result
		if pctx.FileResult == nil {
			t.Error("expected FileResult to be populated")
		}

		// Should NOT have chunker or semantic results
		if pctx.ChunkResult != nil {
			t.Error("expected ChunkResult to be nil for metadata-only")
		}
		if pctx.SemanticResult != nil {
			t.Error("expected SemanticResult to be nil for metadata-only")
		}
	})

	t.Run("SkipsEmbeddingsInDegradationMode", func(t *testing.T) {
		mockReader := &mockFileReaderStage{
			result: &FileReadResult{
				Info: mockFileInfo{
					name:    "test.txt",
					size:    12,
					modTime: time.Now(),
				},
				Kind:         ingest.KindText,
				MIMEType:     "text/plain",
				IngestMode:   ingest.ModeChunk,
				Content:      []byte("test content"),
				ContentHash:  "hash123",
				MetadataHash: "meta456",
			},
		}
		mockChunker := &mockChunkerStage{}
		mockSemantic := &mockSemanticStage{}
		mockEmbed := &mockEmbeddingsStage{embedding: []float32{0.1, 0.2}}

		p := NewPipeline(PipelineConfig{},
			WithFileReader(mockReader),
			WithChunker(mockChunker),
			WithSemantic(mockSemantic),
			WithEmbeddings(mockEmbed),
		)

		// Use DegradationNoEmbed mode
		pctx := NewPipelineContext(WorkItem{FilePath: testFile}, DegradationNoEmbed, nil)
		err := p.Execute(context.Background(), pctx)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should have chunker and semantic results
		if pctx.ChunkResult == nil {
			t.Error("expected ChunkResult to be populated")
		}
		if pctx.SemanticResult == nil {
			t.Error("expected SemanticResult to be populated")
		}

		// Should NOT have embeddings
		if pctx.Embeddings != nil {
			t.Error("expected Embeddings to be nil in DegradationNoEmbed mode")
		}

		// But chunks should still be populated (for persistence)
		if pctx.AnalyzedChunks == nil {
			t.Error("expected AnalyzedChunks to be populated")
		}
	})

	t.Run("HandlesFileReaderError", func(t *testing.T) {
		mockReader := &mockFileReaderStage{
			err: errors.New("file not found"),
		}

		p := NewPipeline(PipelineConfig{},
			WithFileReader(mockReader),
		)

		pctx := NewPipelineContext(WorkItem{FilePath: "/nonexistent"}, DegradationFull, nil)
		err := p.Execute(context.Background(), pctx)
		if err == nil {
			t.Error("expected error from file reader")
		}
	})

	t.Run("HandlesChunkerError", func(t *testing.T) {
		mockReader := &mockFileReaderStage{
			result: &FileReadResult{
				Info: mockFileInfo{
					name:    "test.txt",
					size:    12,
					modTime: time.Now(),
				},
				Kind:         ingest.KindText,
				MIMEType:     "text/plain",
				IngestMode:   ingest.ModeChunk,
				Content:      []byte("test content"),
				ContentHash:  "hash123",
				MetadataHash: "meta456",
			},
		}
		mockChunker := &mockChunkerStage{
			err: errors.New("chunking failed"),
		}

		p := NewPipeline(PipelineConfig{},
			WithFileReader(mockReader),
			WithChunker(mockChunker),
		)

		pctx := NewPipelineContext(WorkItem{FilePath: testFile}, DegradationFull, nil)
		err := p.Execute(context.Background(), pctx)
		if err == nil {
			t.Error("expected error from chunker")
		}
	})

	t.Run("ContinuesOnSemanticError", func(t *testing.T) {
		mockReader := &mockFileReaderStage{
			result: &FileReadResult{
				Info: mockFileInfo{
					name:    "test.txt",
					size:    12,
					modTime: time.Now(),
				},
				Kind:         ingest.KindText,
				MIMEType:     "text/plain",
				IngestMode:   ingest.ModeChunk,
				Content:      []byte("test content"),
				ContentHash:  "hash123",
				MetadataHash: "meta456",
			},
		}
		mockChunker := &mockChunkerStage{}
		mockSemantic := &mockSemanticStage{
			err: errors.New("semantic analysis failed"),
		}
		mockEmbed := &mockEmbeddingsStage{embedding: []float32{0.1, 0.2}}

		p := NewPipeline(PipelineConfig{},
			WithFileReader(mockReader),
			WithChunker(mockChunker),
			WithSemantic(mockSemantic),
			WithEmbeddings(mockEmbed),
		)

		pctx := NewPipelineContext(WorkItem{FilePath: testFile}, DegradationFull, nil)
		err := p.Execute(context.Background(), pctx)
		if err != nil {
			t.Fatalf("Execute should not fail on semantic error: %v", err)
		}

		// Semantic result should be nil
		if pctx.SemanticResult != nil {
			t.Error("expected SemanticResult to be nil on error")
		}

		// But embeddings should still be generated
		if pctx.Embeddings == nil {
			t.Error("expected Embeddings to be populated despite semantic error")
		}
	})

	t.Run("ContinuesOnEmbeddingsError", func(t *testing.T) {
		mockReader := &mockFileReaderStage{
			result: &FileReadResult{
				Info: mockFileInfo{
					name:    "test.txt",
					size:    12,
					modTime: time.Now(),
				},
				Kind:         ingest.KindText,
				MIMEType:     "text/plain",
				IngestMode:   ingest.ModeChunk,
				Content:      []byte("test content"),
				ContentHash:  "hash123",
				MetadataHash: "meta456",
			},
		}
		mockChunker := &mockChunkerStage{}
		mockSemantic := &mockSemanticStage{}
		mockEmbed := &mockEmbeddingsStage{
			err: errors.New("embeddings generation failed"),
		}

		p := NewPipeline(PipelineConfig{},
			WithFileReader(mockReader),
			WithChunker(mockChunker),
			WithSemantic(mockSemantic),
			WithEmbeddings(mockEmbed),
		)

		pctx := NewPipelineContext(WorkItem{FilePath: testFile}, DegradationFull, nil)
		err := p.Execute(context.Background(), pctx)
		if err != nil {
			t.Fatalf("Execute should not fail on embeddings error: %v", err)
		}

		// Embeddings should be nil
		if pctx.Embeddings != nil {
			t.Error("expected Embeddings to be nil on error")
		}

		// But analysis result should still be built
		if pctx.AnalysisResult == nil {
			t.Error("expected AnalysisResult to be populated despite embeddings error")
		}
	})
}

func TestPipelinePersist(t *testing.T) {
	t.Run("PersistsResult", func(t *testing.T) {
		mockPersist := &mockPersistenceStage{}

		p := NewPipeline(PipelineConfig{},
			WithPersistence(mockPersist),
		)

		pctx := &PipelineContext{
			AnalysisResult: &AnalysisResult{
				FilePath:    "/test/file.txt",
				ContentHash: "hash123",
			},
		}

		err := p.Persist(context.Background(), pctx)
		if err != nil {
			t.Fatalf("Persist failed: %v", err)
		}

		if len(mockPersist.persisted) != 1 {
			t.Errorf("expected 1 persisted result, got %d", len(mockPersist.persisted))
		}
		if mockPersist.persisted[0].FilePath != "/test/file.txt" {
			t.Errorf("FilePath = %q, want %q", mockPersist.persisted[0].FilePath, "/test/file.txt")
		}
	})

	t.Run("HandlesPersistenceError", func(t *testing.T) {
		mockPersist := &mockPersistenceStage{
			err: errors.New("persistence failed"),
		}

		p := NewPipeline(PipelineConfig{},
			WithPersistence(mockPersist),
		)

		pctx := &PipelineContext{
			AnalysisResult: &AnalysisResult{
				FilePath: "/test/file.txt",
			},
		}

		err := p.Persist(context.Background(), pctx)
		if err == nil {
			t.Error("expected error from persistence")
		}
	})

	t.Run("SkipsWhenNoPersistence", func(t *testing.T) {
		p := NewPipeline(PipelineConfig{},
			WithPersistence(nil),
		)

		pctx := &PipelineContext{
			AnalysisResult: &AnalysisResult{
				FilePath: "/test/file.txt",
			},
		}

		err := p.Persist(context.Background(), pctx)
		if err != nil {
			t.Errorf("Persist should not fail when persistence is nil: %v", err)
		}
	})

	t.Run("SkipsWhenNoResult", func(t *testing.T) {
		mockPersist := &mockPersistenceStage{}

		p := NewPipeline(PipelineConfig{},
			WithPersistence(mockPersist),
		)

		pctx := &PipelineContext{
			AnalysisResult: nil,
		}

		err := p.Persist(context.Background(), pctx)
		if err != nil {
			t.Errorf("Persist should not fail when result is nil: %v", err)
		}
		if len(mockPersist.persisted) != 0 {
			t.Error("expected no persisted results")
		}
	})
}

func TestPipelineContext(t *testing.T) {
	t.Run("NewPipelineContext", func(t *testing.T) {
		item := WorkItem{FilePath: "/test/file.txt"}
		pctx := NewPipelineContext(item, DegradationFull, nil)

		if pctx.WorkItem.FilePath != item.FilePath {
			t.Errorf("WorkItem.FilePath = %q, want %q", pctx.WorkItem.FilePath, item.FilePath)
		}
		if pctx.DegradationMode != DegradationFull {
			t.Errorf("DegradationMode = %v, want %v", pctx.DegradationMode, DegradationFull)
		}
		if pctx.StartTime.IsZero() {
			t.Error("expected StartTime to be set")
		}
		if pctx.Logger == nil {
			t.Error("expected Logger to be set")
		}
	})

	t.Run("BuildAnalysisResult", func(t *testing.T) {
		pctx := &PipelineContext{
			WorkItem: WorkItem{FilePath: "/test/file.txt"},
			FileResult: &FileReadResult{
				Info: mockFileInfo{
					name:    "file.txt",
					size:    100,
					modTime: time.Now(),
				},
				MIMEType:     "text/plain",
				Language:     "text",
				Kind:         ingest.KindText,
				IngestMode:   ingest.ModeChunk,
				ContentHash:  "hash123",
				MetadataHash: "meta456",
			},
			ChunkResult: &chunkers.ChunkResult{
				ChunkerUsed: "text-chunker",
				TotalChunks: 3,
			},
			SemanticResult: &SemanticResult{
				Summary:    "Test summary",
				Tags:       []string{"test"},
				Complexity: 5,
			},
			Embeddings: []float32{0.1, 0.2},
			AnalyzedChunks: []AnalyzedChunk{
				{Index: 0, Content: "chunk1"},
			},
			StartTime: time.Now().Add(-100 * time.Millisecond),
		}

		result := pctx.BuildAnalysisResult()

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.FilePath != "/test/file.txt" {
			t.Errorf("FilePath = %q, want %q", result.FilePath, "/test/file.txt")
		}
		if result.ContentHash != "hash123" {
			t.Errorf("ContentHash = %q, want %q", result.ContentHash, "hash123")
		}
		if result.ChunkerUsed != "text-chunker" {
			t.Errorf("ChunkerUsed = %q, want %q", result.ChunkerUsed, "text-chunker")
		}
		if result.ChunksProcessed != 3 {
			t.Errorf("ChunksProcessed = %d, want 3", result.ChunksProcessed)
		}
		if result.Summary != "Test summary" {
			t.Errorf("Summary = %q, want %q", result.Summary, "Test summary")
		}
		if result.Complexity != 5 {
			t.Errorf("Complexity = %d, want 5", result.Complexity)
		}
		if len(result.Embeddings) != 2 {
			t.Errorf("Embeddings length = %d, want 2", len(result.Embeddings))
		}
		if len(result.Chunks) != 1 {
			t.Errorf("Chunks length = %d, want 1", len(result.Chunks))
		}
		if result.ProcessingTime <= 0 {
			t.Error("expected ProcessingTime to be positive")
		}
	})

	t.Run("ShouldChunk", func(t *testing.T) {
		tests := []struct {
			name       string
			ingestMode ingest.Mode
			want       bool
		}{
			{"Chunk mode", ingest.ModeChunk, true},
			{"Semantic only", ingest.ModeSemanticOnly, false},
			{"Metadata only", ingest.ModeMetadataOnly, false},
			{"Skip", ingest.ModeSkip, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pctx := &PipelineContext{
					FileResult: &FileReadResult{
						IngestMode: tt.ingestMode,
					},
				}
				if got := pctx.ShouldChunk(); got != tt.want {
					t.Errorf("ShouldChunk() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("ShouldGenerateEmbeddings", func(t *testing.T) {
		tests := []struct {
			name            string
			ingestMode      ingest.Mode
			degradationMode DegradationMode
			want            bool
		}{
			{"Full mode, chunk", ingest.ModeChunk, DegradationFull, true},
			{"NoEmbed mode, chunk", ingest.ModeChunk, DegradationNoEmbed, false},
			{"Metadata mode, chunk", ingest.ModeChunk, DegradationMetadata, false},
			{"Full mode, semantic only", ingest.ModeSemanticOnly, DegradationFull, false},
			{"Full mode, metadata only", ingest.ModeMetadataOnly, DegradationFull, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pctx := &PipelineContext{
					DegradationMode: tt.degradationMode,
					FileResult: &FileReadResult{
						IngestMode: tt.ingestMode,
					},
				}
				if got := pctx.ShouldGenerateEmbeddings(); got != tt.want {
					t.Errorf("ShouldGenerateEmbeddings() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}
