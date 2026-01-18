package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

func TestProbeKinds(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  []byte
		wantKind Kind
		wantMIME string
	}{
		{
			name:     "text",
			filename: "sample.go",
			content:  []byte("package main\n"),
			wantKind: KindText,
			wantMIME: "text/x-go",
		},
		{
			name:     "structured",
			filename: "sample.json",
			content:  []byte("{\"ok\":true}"),
			wantKind: KindStructured,
			wantMIME: "application/json",
		},
		{
			name:     "document",
			filename: "sample.pdf",
			content:  []byte("%PDF-1.4"),
			wantKind: KindDocument,
			wantMIME: "application/pdf",
		},
		{
			name:     "image",
			filename: "sample.png",
			content:  []byte{0x89, 0x50, 0x4e, 0x47},
			wantKind: KindImage,
			wantMIME: "image/png",
		},
		{
			name:     "archive",
			filename: "sample.zip",
			content:  []byte{0x50, 0x4b, 0x03, 0x04},
			wantKind: KindArchive,
			wantMIME: "application/zip",
		},
		{
			name:     "media",
			filename: "sample.mp3",
			content:  []byte("ID3"),
			wantKind: KindMedia,
			wantMIME: "audio/mpeg",
		},
		{
			name:     "binary",
			filename: "sample.bin",
			content:  []byte{0x00, 0xff, 0x00},
			wantKind: KindBinary,
			wantMIME: "application/octet-stream",
		},
		{
			name:     "binary with text extension",
			filename: "sample.go",
			content:  []byte{0x00, 0x01, 0x02},
			wantKind: KindBinary,
			wantMIME: "text/x-go",
		},
		{
			name:     "extensionless text",
			filename: "README",
			content:  []byte("plain text"),
			wantKind: KindText,
			wantMIME: "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(path, tt.content, 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("failed to stat test file: %v", err)
			}

			kind, mimeType, _ := Probe(path, info, tt.content)
			if kind != tt.wantKind {
				t.Fatalf("kind = %q, want %q", kind, tt.wantKind)
			}
			if mimeType != tt.wantMIME {
				t.Fatalf("mime = %q, want %q", mimeType, tt.wantMIME)
			}
		})
	}
}

func TestDecide(t *testing.T) {
	visionOff := false
	visionOn := true

	tests := []struct {
		name       string
		kind       Kind
		cfg        *registry.PathConfig
		size       int64
		wantMode   Mode
		wantReason string
	}{
		{
			name:     "text chunk",
			kind:     KindText,
			size:     1024,
			wantMode: ModeChunk,
		},
		{
			name:     "max chunk size inclusive",
			kind:     KindText,
			size:     MaxChunkBytes,
			wantMode: ModeChunk,
		},
		{
			name:       "too large",
			kind:       KindText,
			size:       MaxChunkBytes + 1,
			wantMode:   ModeMetadataOnly,
			wantReason: ReasonTooLarge,
		},
		{
			name:       "unknown too large",
			kind:       KindUnknown,
			size:       MaxChunkBytes + 1,
			wantMode:   ModeMetadataOnly,
			wantReason: ReasonTooLarge,
		},
		{
			name:       "image vision disabled",
			kind:       KindImage,
			cfg:        &registry.PathConfig{UseVision: &visionOff},
			size:       1024,
			wantMode:   ModeMetadataOnly,
			wantReason: ReasonVisionDisabled,
		},
		{
			name:       "image vision enabled",
			kind:       KindImage,
			cfg:        &registry.PathConfig{UseVision: &visionOn},
			size:       1024,
			wantMode:   ModeMetadataOnly,
			wantReason: ReasonImage,
		},
		{
			name:       "image vision default",
			kind:       KindImage,
			cfg:        &registry.PathConfig{UseVision: nil},
			size:       1024,
			wantMode:   ModeMetadataOnly,
			wantReason: ReasonImage,
		},
		{
			name:       "archive metadata",
			kind:       KindArchive,
			size:       1024,
			wantMode:   ModeMetadataOnly,
			wantReason: ReasonArchive,
		},
		{
			name:       "binary metadata",
			kind:       KindBinary,
			size:       1024,
			wantMode:   ModeMetadataOnly,
			wantReason: ReasonBinary,
		},
		{
			name:       "unknown skip",
			kind:       KindUnknown,
			size:       1024,
			wantMode:   ModeSkip,
			wantReason: ReasonUnsupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, reason := Decide(tt.kind, tt.cfg, tt.size)
			if mode != tt.wantMode {
				t.Fatalf("mode = %q, want %q", mode, tt.wantMode)
			}
			if reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
