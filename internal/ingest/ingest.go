package ingest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/leefowlercu/agentic-memorizer/internal/fsutil"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// Kind is the coarse content classification.
type Kind string

const (
	KindText       Kind = "text"
	KindStructured Kind = "structured"
	KindDocument   Kind = "document"
	KindImage      Kind = "image"
	KindArchive    Kind = "archive"
	KindMedia      Kind = "media"
	KindBinary     Kind = "binary"
	KindUnknown    Kind = "unknown"
)

// Mode is the processing decision.
type Mode string

const (
	ModeChunk        Mode = "chunk"
	ModeMetadataOnly Mode = "metadata_only"
	ModeSemanticOnly Mode = "semantic_only"
	ModeSkip         Mode = "skip"
)

const (
	ReasonTooLarge         = "too_large"
	ReasonBinary           = "binary"
	ReasonArchive          = "archive"
	ReasonMedia            = "media"
	ReasonImage            = "image"
	ReasonVisionDisabled   = "vision_disabled"
	ReasonSemanticDisabled = "semantic_disabled"
	ReasonUnsupported      = "unsupported"
)

// Decision explains how and why a file is handled.
type Decision struct {
	Kind   Kind
	MIME   string
	Lang   string
	Mode   Mode
	Reason string
}

// MaxChunkBytes defines the maximum size for chunking text content.
const MaxChunkBytes = 100 * 1024 * 1024

// Probe inspects the file path, info, and a small byte sample.
func Probe(path string, info os.FileInfo, peek []byte) (Kind, string, string) {
	_ = info

	ext := strings.ToLower(filepath.Ext(path))
	mimeType := fsutil.DetectMIME(path, peek)
	language := fsutil.DetectLanguage(path)

	kind := kindFromMIME(mimeType)
	if kind == KindUnknown {
		kind = kindFromExtension(ext)
	}

	if kind == KindText || kind == KindStructured || kind == KindDocument {
		if !isLikelyText(peek) {
			kind = KindBinary
		}
	}

	if kind == KindUnknown {
		if isLikelyText(peek) {
			kind = KindText
		} else if len(peek) > 0 {
			kind = KindBinary
		}
	}

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return kind, mimeType, language
}

// Decide chooses the processing mode based on kind, config, and size.
func Decide(kind Kind, cfg *registry.PathConfig, size int64) (Mode, string) {
	if size > MaxChunkBytes {
		return ModeMetadataOnly, ReasonTooLarge
	}

	switch kind {
	case KindText, KindStructured, KindDocument:
		return ModeChunk, ""
	case KindImage:
		if cfg != nil && cfg.UseVision != nil && !*cfg.UseVision {
			return ModeMetadataOnly, ReasonVisionDisabled
		}
		return ModeSemanticOnly, ReasonImage
	case KindArchive:
		return ModeMetadataOnly, ReasonArchive
	case KindMedia:
		return ModeMetadataOnly, ReasonMedia
	case KindBinary:
		return ModeMetadataOnly, ReasonBinary
	default:
		return ModeSkip, ReasonUnsupported
	}
}

func kindFromMIME(mimeType string) Kind {
	mimeType = strings.ToLower(mimeType)

	switch {
	case isStructuredMIME(mimeType):
		return KindStructured
	case isDocumentMIME(mimeType):
		return KindDocument
	case isImageMIME(mimeType):
		return KindImage
	case isArchiveMIME(mimeType):
		return KindArchive
	case isMediaMIME(mimeType):
		return KindMedia
	case isBinaryMIME(mimeType):
		return KindBinary
	case isTextMIME(mimeType):
		return KindText
	default:
		return KindUnknown
	}
}

func kindFromExtension(ext string) Kind {
	if ext == "" {
		return KindUnknown
	}

	mimeType := fsutil.MIMEFromExtension(ext)
	if mimeType == "" {
		return KindUnknown
	}
	return kindFromMIME(mimeType)
}

func isLikelyText(peek []byte) bool {
	if len(peek) == 0 {
		return true
	}
	if bytes.IndexByte(peek, 0) != -1 {
		return false
	}
	if !utf8.Valid(peek) {
		return false
	}
	return true
}

func isTextMIME(mimeType string) bool {
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	textMIMEs := map[string]bool{
		"application/javascript": true,
		"application/x-ndjson":   true,
		"application/x-yaml":     true,
		"application/yaml":       true,
		"text/x-rst":             true,
		"text/x-tex":             true,
	}

	return textMIMEs[mimeType]
}

func isStructuredMIME(mimeType string) bool {
	structuredMIMEs := map[string]bool{
		"application/json":          true,
		"application/x-ndjson":      true,
		"text/csv":                  true,
		"text/tab-separated-values": true,
		"text/yaml":                 true,
		"application/yaml":          true,
		"application/xml":           true,
		"text/xml":                  true,
	}

	return structuredMIMEs[mimeType]
}

func isDocumentMIME(mimeType string) bool {
	documentMIMEs := map[string]bool{
		"application/pdf":    true,
		"text/html":          true,
		"text/markdown":      true,
		"text/asciidoc":      true,
		"application/rtf":    true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
		"application/vnd.ms-excel":                        true,
		"application/vnd.ms-powerpoint":                   true,
		"application/vnd.oasis.opendocument.text":         true,
		"application/vnd.oasis.opendocument.spreadsheet":  true,
		"application/vnd.oasis.opendocument.presentation": true,
	}

	return documentMIMEs[mimeType]
}

func isImageMIME(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

func isArchiveMIME(mimeType string) bool {
	archiveMIMEs := map[string]bool{
		"application/zip":             true,
		"application/x-tar":           true,
		"application/gzip":            true,
		"application/x-bzip2":         true,
		"application/x-xz":            true,
		"application/x-7z-compressed": true,
		"application/vnd.rar":         true,
		"application/java-archive":    true,
	}

	return archiveMIMEs[mimeType]
}

func isMediaMIME(mimeType string) bool {
	return strings.HasPrefix(mimeType, "audio/") || strings.HasPrefix(mimeType, "video/")
}

func isBinaryMIME(mimeType string) bool {
	if mimeType == "application/octet-stream" {
		return true
	}

	binaryMIMEs := map[string]bool{
		"application/x-executable": true,
		"application/x-sharedlib":  true,
		"application/x-archive":    true,
		"application/x-object":     true,
		"application/wasm":         true,
	}

	return binaryMIMEs[mimeType]
}
