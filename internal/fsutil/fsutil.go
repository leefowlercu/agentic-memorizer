package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HashFile computes the SHA-256 hash of a file's contents.
func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// HashBytes computes the SHA-256 hash of the provided bytes.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// DetectMIME determines the MIME type of content.
func DetectMIME(path string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	extMime := extensionToMIME(ext)
	if extMime == "" {
		extMime = strings.TrimSpace(mime.TypeByExtension(ext))
		if idx := strings.Index(extMime, ";"); idx != -1 {
			extMime = strings.TrimSpace(extMime[:idx])
		}
	}

	var sniffed string
	if len(content) > 0 {
		sniffed = http.DetectContentType(content)
		if idx := strings.Index(sniffed, ";"); idx != -1 {
			sniffed = strings.TrimSpace(sniffed[:idx])
		}
	}

	if extMime != "" {
		if sniffed == "" || sniffed == "application/octet-stream" || sniffed == "text/plain" {
			return extMime
		}
	}

	if sniffed != "" {
		return sniffed
	}

	if extMime != "" {
		return extMime
	}

	return "application/octet-stream"
}

// MIMEFromExtension returns a best-effort MIME type for a file extension.
// The extension may be provided with or without a leading dot.
func MIMEFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return extensionToMIME(ext)
}

// DetectLanguage determines the programming language from file extension.
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh", ".bash":
		return "bash"
	case ".sql":
		return "sql"
	default:
		return ""
	}
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
