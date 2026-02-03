package analysis

import (
	"context"
	"fmt"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/fsutil"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// FileReader performs file stat, head read, ingest decision, and hashing.
type FileReader struct {
	registry        registry.Registry
	semanticEnabled bool
}

// FileReaderOption configures a FileReader.
type FileReaderOption func(*FileReader)

// WithSemanticEnabled sets whether semantic analysis is enabled.
func WithSemanticEnabled(enabled bool) FileReaderOption {
	return func(r *FileReader) {
		r.semanticEnabled = enabled
	}
}

// NewFileReader creates a file reader stage.
func NewFileReader(reg registry.Registry, opts ...FileReaderOption) *FileReader {
	reader := &FileReader{
		registry:        reg,
		semanticEnabled: true,
	}
	for _, opt := range opts {
		opt(reader)
	}
	return reader
}

// Read collects file metadata, ingest decisions, and content hash.
func (r *FileReader) Read(ctx context.Context, item WorkItem, mode DegradationMode) (*FileReadResult, error) {
	info, err := os.Stat(item.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file; %w", err)
	}
	peek, err := readHead(item.FilePath, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to read file head; %w", err)
	}

	kind, mimeType, language := ingest.Probe(item.FilePath, info, peek)
	var pathConfig *registry.PathConfig
	if r.registry != nil {
		cfg, err := r.registry.GetEffectiveConfig(ctx, item.FilePath)
		if err == nil {
			pathConfig = cfg
		}
	}

	ingestMode, ingestReason := ingest.Decide(kind, pathConfig, info.Size())
	degradedMetadata := false
	if !r.semanticEnabled && ingestMode == ingest.ModeSemanticOnly {
		ingestMode = ingest.ModeMetadataOnly
		ingestReason = ingest.ReasonSemanticDisabled
	}
	if mode == DegradationMetadata && (ingestMode == ingest.ModeChunk || ingestMode == ingest.ModeSemanticOnly) {
		ingestMode = ingest.ModeMetadataOnly
		degradedMetadata = true
	}

	var content []byte
	var contentHash string
	if ingestMode == ingest.ModeChunk || ingestMode == ingest.ModeSemanticOnly {
		content, err = os.ReadFile(item.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file; %w", err)
		}
		contentHash = fsutil.HashBytes(content)
	} else {
		contentHash, err = fsutil.HashFile(item.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to hash file; %w", err)
		}
	}

	return &FileReadResult{
		Info:             info,
		Peek:             peek,
		Kind:             kind,
		MIMEType:         mimeType,
		Language:         language,
		IngestMode:       ingestMode,
		IngestReason:     ingestReason,
		DegradedMetadata: degradedMetadata,
		Content:          content,
		ContentHash:      contentHash,
		MetadataHash:     computeMetadataHash(item.FilePath, info.Size(), info.ModTime()),
	}, nil
}
