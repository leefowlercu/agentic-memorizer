package ingest

import (
	"bytes"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/leefowlercu/agentic-memorizer/internal/filetype"
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
	ModeSkip         Mode = "skip"
)

const (
	ReasonTooLarge       = "too_large"
	ReasonBinary         = "binary"
	ReasonArchive        = "archive"
	ReasonMedia          = "media"
	ReasonImage          = "image"
	ReasonVisionDisabled = "vision_disabled"
	ReasonUnsupported    = "unsupported"
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
	mimeType := detectMIME(ext, peek)
	language := filetype.DetectLanguage(path)

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
		return ModeMetadataOnly, ReasonImage
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

func detectMIME(ext string, peek []byte) string {
	extMime := strings.TrimSpace(mime.TypeByExtension(ext))
	if idx := strings.Index(extMime, ";"); idx != -1 {
		extMime = strings.TrimSpace(extMime[:idx])
	}

	if extMime == "" {
		extMime = extensionToMIME(ext)
	}

	sniffed := http.DetectContentType(peek)
	if idx := strings.Index(sniffed, ";"); idx != -1 {
		sniffed = strings.TrimSpace(sniffed[:idx])
	}

	if extMime != "" {
		if sniffed == "" || sniffed == "application/octet-stream" || sniffed == "text/plain" {
			return extMime
		}
	}

	if sniffed != "" {
		return sniffed
	}

	return extMime
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

	mimeType := extensionToMIME(ext)
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

func extensionToMIME(ext string) string {
	mimeMap := map[string]string{
		// Programming languages
		".go":    "text/x-go",
		".py":    "text/x-python",
		".js":    "text/javascript",
		".ts":    "text/typescript",
		".tsx":   "text/typescript-jsx",
		".jsx":   "text/javascript-jsx",
		".rs":    "text/x-rust",
		".rb":    "text/x-ruby",
		".java":  "text/x-java",
		".kt":    "text/x-kotlin",
		".swift": "text/x-swift",
		".c":     "text/x-c",
		".cpp":   "text/x-c++",
		".h":     "text/x-c-header",
		".hpp":   "text/x-c++-header",
		".cs":    "text/x-csharp",
		".php":   "text/x-php",
		".scala": "text/x-scala",
		".clj":   "text/x-clojure",
		".ex":    "text/x-elixir",
		".exs":   "text/x-elixir",
		".erl":   "text/x-erlang",
		".hs":    "text/x-haskell",
		".lua":   "text/x-lua",
		".pl":    "text/x-perl",
		".r":     "text/x-r",
		".sql":   "text/x-sql",
		".sh":    "text/x-shellscript",
		".bash":  "text/x-shellscript",
		".zsh":   "text/x-shellscript",
		".fish":  "text/x-shellscript",
		".ps1":   "text/x-powershell",
		".vim":   "text/x-vim",
		".zig":   "text/x-zig",

		// Markup and config
		".md":         "text/markdown",
		".markdown":   "text/markdown",
		".rst":        "text/x-rst",
		".adoc":       "text/asciidoc",
		".tex":        "text/x-tex",
		".yaml":       "text/yaml",
		".yml":        "text/yaml",
		".toml":       "text/toml",
		".ini":        "text/ini",
		".cfg":        "text/ini",
		".conf":       "text/plain",
		".env":        "text/plain",
		".properties": "text/x-java-properties",

		// Data formats
		".json":   "application/json",
		".jsonl":  "application/x-ndjson",
		".ndjson": "application/x-ndjson",
		".csv":    "text/csv",
		".tsv":    "text/tab-separated-values",
		".xml":    "application/xml",

		// Documents
		".pdf":  "application/pdf",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".doc":  "application/msword",
		".xls":  "application/vnd.ms-excel",
		".ppt":  "application/vnd.ms-powerpoint",
		".odt":  "application/vnd.oasis.opendocument.text",
		".ods":  "application/vnd.oasis.opendocument.spreadsheet",
		".odp":  "application/vnd.oasis.opendocument.presentation",
		".rtf":  "application/rtf",

		// Images
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".bmp":  "image/bmp",
		".tiff": "image/tiff",
		".tif":  "image/tiff",
		".heic": "image/heic",
		".heif": "image/heif",
		".avif": "image/avif",

		// Audio
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".ogg":  "audio/ogg",

		// Video
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".webm": "video/webm",

		// Archives
		".zip": "application/zip",
		".tar": "application/x-tar",
		".gz":  "application/gzip",
		".tgz": "application/gzip",
		".bz2": "application/x-bzip2",
		".xz":  "application/x-xz",
		".7z":  "application/x-7z-compressed",
		".rar": "application/vnd.rar",
		".jar": "application/java-archive",
		".war": "application/java-archive",
		".ear": "application/java-archive",

		// Binary/executable
		".exe":   "application/x-executable",
		".dll":   "application/x-executable",
		".so":    "application/x-sharedlib",
		".dylib": "application/x-sharedlib",
		".a":     "application/x-archive",
		".o":     "application/x-object",
		".wasm":  "application/wasm",
	}

	return mimeMap[ext]
}
