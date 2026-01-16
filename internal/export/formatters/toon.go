package formatters

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// TOONFormatter formats snapshots in Token-Optimized Notation.
// TOON is a compact format designed to minimize token usage for LLMs
// while remaining human-readable.
//
// Format:
//
//	@kg v=1 t=2024-01-01T00:00:00Z
//	#f path|name|ext|mime|lang|size|sum
//	#d path|name|rem|cnt
//	#t name|cnt
//	#p name|desc|cnt
//	#e name|type|cnt
//	@stats f=10 d=5 c=100 t=5 p=3 e=8 r=50
type TOONFormatter struct{}

// NewTOONFormatter creates a new TOON formatter.
func NewTOONFormatter() *TOONFormatter {
	return &TOONFormatter{}
}

// Name returns the formatter name.
func (f *TOONFormatter) Name() string {
	return "toon"
}

// ContentType returns the MIME content type.
func (f *TOONFormatter) ContentType() string {
	return "text/plain"
}

// FileExtension returns the typical file extension.
func (f *TOONFormatter) FileExtension() string {
	return ".toon"
}

// Format converts the graph snapshot to TOON format.
func (f *TOONFormatter) Format(snapshot *graph.GraphSnapshot) ([]byte, error) {
	var buf bytes.Buffer

	// Header
	fmt.Fprintf(&buf, "@kg v=%d t=%s\n",
		snapshot.Version,
		snapshot.ExportedAt.Format("2006-01-02T15:04:05Z"))

	// Files section
	if len(snapshot.Files) > 0 {
		buf.WriteString("#f\n")
		for _, file := range snapshot.Files {
			// path|name|ext|mime|lang|size|summary (truncated)
			summary := truncate(file.Summary, 100)
			fmt.Fprintf(&buf, "%s|%s|%s|%s|%s|%d|%s\n",
				file.Path,
				file.Name,
				file.Extension,
				shortenMIME(file.MIMEType),
				file.Language,
				file.Size,
				escapeTOON(summary))
		}
	}

	// Directories section
	if len(snapshot.Directories) > 0 {
		buf.WriteString("#d\n")
		for _, dir := range snapshot.Directories {
			rem := "0"
			if dir.IsRemembered {
				rem = "1"
			}
			fmt.Fprintf(&buf, "%s|%s|%s|%d\n",
				dir.Path,
				dir.Name,
				rem,
				dir.FileCount)
		}
	}

	// Tags section
	if len(snapshot.Tags) > 0 {
		buf.WriteString("#t\n")
		for _, tag := range snapshot.Tags {
			fmt.Fprintf(&buf, "%s|%d\n", tag.Name, tag.UsageCount)
		}
	}

	// Topics section
	if len(snapshot.Topics) > 0 {
		buf.WriteString("#p\n")
		for _, topic := range snapshot.Topics {
			desc := truncate(topic.Description, 50)
			fmt.Fprintf(&buf, "%s|%s|%d\n",
				topic.Name,
				escapeTOON(desc),
				topic.UsageCount)
		}
	}

	// Entities section
	if len(snapshot.Entities) > 0 {
		buf.WriteString("#e\n")
		for _, entity := range snapshot.Entities {
			fmt.Fprintf(&buf, "%s|%s|%d\n",
				entity.Name,
				entity.Type,
				entity.UsageCount)
		}
	}

	// Stats footer
	fmt.Fprintf(&buf, "@stats f=%d d=%d c=%d t=%d p=%d e=%d r=%d\n",
		len(snapshot.Files),
		len(snapshot.Directories),
		snapshot.TotalChunks,
		len(snapshot.Tags),
		len(snapshot.Topics),
		len(snapshot.Entities),
		snapshot.TotalRelationships)

	return buf.Bytes(), nil
}

// truncate shortens a string to max length, adding ellipsis if needed.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// escapeTOON escapes special characters for TOON format.
func escapeTOON(s string) string {
	// Replace pipe and newline which are delimiters
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// shortenMIME abbreviates common MIME types.
func shortenMIME(mime string) string {
	abbreviations := map[string]string{
		"text/plain":               "txt",
		"text/x-go":                "go",
		"text/x-python":            "py",
		"text/javascript":          "js",
		"text/typescript":          "ts",
		"text/markdown":            "md",
		"text/x-markdown":          "md",
		"text/html":                "html",
		"text/css":                 "css",
		"text/yaml":                "yaml",
		"text/x-yaml":              "yaml",
		"application/json":         "json",
		"application/xml":          "xml",
		"application/javascript":   "js",
		"application/typescript":   "ts",
		"application/octet-stream": "bin",
	}

	if abbr, ok := abbreviations[mime]; ok {
		return abbr
	}

	// Return last part of MIME type
	if idx := strings.LastIndex(mime, "/"); idx >= 0 {
		return mime[idx+1:]
	}

	return mime
}
