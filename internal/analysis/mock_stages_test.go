package analysis

import (
	"context"
	"os"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
)

// mockFileReaderStage is a mock implementation of FileReaderStage for testing.
type mockFileReaderStage struct {
	result *FileReadResult
	err    error
}

func (m *mockFileReaderStage) Read(ctx context.Context, item WorkItem, mode DegradationMode) (*FileReadResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	// Return a default result based on a real file
	info, err := os.Stat(item.FilePath)
	if err != nil {
		return nil, err
	}
	return &FileReadResult{
		Info:         info,
		Kind:         ingest.KindText,
		MIMEType:     "text/plain",
		IngestMode:   ingest.ModeChunk,
		Content:      []byte("mock content"),
		ContentHash:  "mockhash123",
		MetadataHash: "metahash456",
	}, nil
}

// mockChunkerStage is a mock implementation of ChunkerStageInterface for testing.
type mockChunkerStage struct {
	result *chunkers.ChunkResult
	err    error
}

func (m *mockChunkerStage) Chunk(ctx context.Context, content []byte, mimeType, language string) (*chunkers.ChunkResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &chunkers.ChunkResult{
		Chunks: []chunkers.Chunk{
			{
				Index:       0,
				Content:     string(content),
				StartOffset: 0,
				EndOffset:   len(content),
				Metadata: chunkers.ChunkMetadata{
					Type:          chunkers.ChunkTypeUnknown,
					TokenEstimate: 10,
				},
			},
		},
		TotalChunks:  1,
		ChunkerUsed:  "mock-chunker",
		OriginalSize: len(content),
	}, nil
}

// mockSemanticStage is a mock implementation of SemanticStageInterface for testing.
type mockSemanticStage struct {
	result    *SemanticResult
	summaries []string
	err       error
}

func (m *mockSemanticStage) Analyze(ctx context.Context, path, contentHash string, chunks []chunkers.Chunk) (*SemanticResult, []string, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	if m.result != nil {
		return m.result, m.summaries, nil
	}
	summaries := make([]string, len(chunks))
	for i := range chunks {
		summaries[i] = "Mock summary for chunk"
	}
	return &SemanticResult{
		Summary:    "Mock file summary",
		Tags:       []string{"mock", "test"},
		Topics:     []string{"testing"},
		Complexity: 3,
	}, summaries, nil
}

// mockEmbeddingsStage is a mock implementation of EmbeddingsStageInterface for testing.
type mockEmbeddingsStage struct {
	embedding []float32
	err       error
}

func (m *mockEmbeddingsStage) Generate(ctx context.Context, path string, analyzedChunks []AnalyzedChunk) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Set embeddings on chunks
	for i := range analyzedChunks {
		if m.embedding != nil {
			analyzedChunks[i].Embedding = m.embedding
		} else {
			analyzedChunks[i].Embedding = []float32{0.1, 0.2, 0.3}
		}
	}
	if m.embedding != nil {
		return m.embedding, nil
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

// mockPersistenceStage is a mock implementation of PersistenceStageInterface for testing.
type mockPersistenceStage struct {
	persisted []*AnalysisResult
	err       error
}

func (m *mockPersistenceStage) Persist(ctx context.Context, result *AnalysisResult) error {
	if m.err != nil {
		return m.err
	}
	m.persisted = append(m.persisted, result)
	return nil
}

// mockFileInfo implements os.FileInfo for testing.
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }
