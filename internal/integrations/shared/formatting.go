package shared

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// FormatSize formats bytes into human-readable size
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GroupByCategory groups file entries by their category
func GroupByCategory(files []types.FileEntry) map[string][]types.FileEntry {
	categories := make(map[string][]types.FileEntry)
	for _, file := range files {
		categories[file.Category] = append(categories[file.Category], file)
	}
	return categories
}

// Join concatenates strings with a separator
func Join(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}

// OutputFormatToString converts an OutputFormat to a formatter string.
// Returns the format string (e.g., "xml", "json") or an error for unsupported formats.
func OutputFormatToString(format integrations.OutputFormat) (string, error) {
	switch format {
	case integrations.FormatXML:
		return "xml", nil
	case integrations.FormatJSON:
		return "json", nil
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}
}
