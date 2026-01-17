package filetype

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
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
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".js":
		return "text/javascript"
	case ".ts":
		return "text/typescript"
	case ".md", ".markdown":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".txt":
		return "text/plain"
	case ".csv":
		return "text/csv"
	default:
		// Basic content sniffing
		if len(content) > 0 {
			if content[0] == '{' || content[0] == '[' {
				return "application/json"
			}
		}
		return "application/octet-stream"
	}
}

// DetectLanguage determines the programming language from file extension.
func DetectLanguage(path string) string {
	ext := filepath.Ext(path)
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
