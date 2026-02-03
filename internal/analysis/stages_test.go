package analysis

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/fsutil"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
)

type stubChunker struct {
	called bool
}

func (s *stubChunker) Name() string { return "stub" }
func (s *stubChunker) CanHandle(mimeType string, language string) bool {
	return true
}
func (s *stubChunker) Chunk(ctx context.Context, content []byte, opts chunkers.ChunkOptions) (*chunkers.ChunkResult, error) {
	s.called = true
	return &chunkers.ChunkResult{
		Chunks: []chunkers.Chunk{{
			Index:       0,
			Content:     string(content),
			StartOffset: 0,
			EndOffset:   len(content),
			Metadata: chunkers.ChunkMetadata{
				Type:          chunkers.ChunkTypeUnknown,
				TokenEstimate: chunkers.EstimateTokens(string(content)),
			},
		}},
		TotalChunks:  1,
		ChunkerUsed:  s.Name(),
		OriginalSize: len(content),
	}, nil
}
func (s *stubChunker) Priority() int { return 100 }

func TestFileReaderReadChunkMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	reader := NewFileReader(nil)
	result, err := reader.Read(context.Background(), WorkItem{FilePath: path}, DegradationFull)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if result.IngestMode != ingest.ModeChunk {
		t.Fatalf("IngestMode = %q, want %q", result.IngestMode, ingest.ModeChunk)
	}
	if string(result.Content) != string(content) {
		t.Fatalf("Content = %q, want %q", result.Content, content)
	}

	expectedHash := fsutil.HashBytes(content)
	if result.ContentHash != expectedHash {
		t.Fatalf("ContentHash = %q, want %q", result.ContentHash, expectedHash)
	}
	if result.MetadataHash == "" {
		t.Fatal("MetadataHash should be populated")
	}
	if result.DegradedMetadata {
		t.Fatal("DegradedMetadata should be false for full mode")
	}
}

func TestFileReaderReadDegradedMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	reader := NewFileReader(nil)
	result, err := reader.Read(context.Background(), WorkItem{FilePath: path}, DegradationMetadata)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if result.IngestMode != ingest.ModeMetadataOnly {
		t.Fatalf("IngestMode = %q, want %q", result.IngestMode, ingest.ModeMetadataOnly)
	}
	if !result.DegradedMetadata {
		t.Fatal("DegradedMetadata should be true when degraded")
	}
	if len(result.Content) != 0 {
		t.Fatalf("Content should be empty in metadata-only mode, got %q", result.Content)
	}
	expectedHash, err := fsutil.HashFile(path)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}
	if result.ContentHash != expectedHash {
		t.Fatalf("ContentHash = %q, want %q", result.ContentHash, expectedHash)
	}
}

func TestFileReaderReadSemanticDisabledImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.png")
	content := []byte("not-a-real-png")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	reader := NewFileReader(nil, WithSemanticEnabled(false))
	result, err := reader.Read(context.Background(), WorkItem{FilePath: path}, DegradationFull)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if result.IngestMode != ingest.ModeMetadataOnly {
		t.Fatalf("IngestMode = %q, want %q", result.IngestMode, ingest.ModeMetadataOnly)
	}
	if result.IngestReason != ingest.ReasonSemanticDisabled {
		t.Fatalf("IngestReason = %q, want %q", result.IngestReason, ingest.ReasonSemanticDisabled)
	}
	if len(result.Content) != 0 {
		t.Fatalf("Content should be empty in metadata-only mode, got %q", result.Content)
	}
}

func TestChunkerStageUsesRegistry(t *testing.T) {
	registry := chunkers.NewRegistry()
	chunker := &stubChunker{}
	registry.Register(chunker)

	stage := NewChunkerStage(registry)
	result, err := stage.Chunk(context.Background(), []byte("sample"), "text/plain", "")
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if !chunker.called {
		t.Fatal("expected chunker to be invoked")
	}
	if result.ChunkerUsed != "stub" {
		t.Fatalf("ChunkerUsed = %q, want %q", result.ChunkerUsed, "stub")
	}
	if result.TotalChunks != 1 {
		t.Fatalf("TotalChunks = %d, want 1", result.TotalChunks)
	}
}
