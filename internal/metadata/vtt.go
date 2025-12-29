package metadata

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// VTTHandler extracts metadata from VTT transcript files
type VTTHandler struct{}

// SupportedExtensions returns the list of extensions this handler supports
func (h *VTTHandler) SupportedExtensions() []string {
	return []string{".vtt", ".srt"}
}

// CanHandle returns true if this handler can process the file
func (h *VTTHandler) CanHandle(ext string) bool {
	return ext == ".vtt" || ext == ".srt"
}

// Extract extracts metadata from a VTT file
func (h *VTTHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "transcript",
			Category:   "transcripts",
			IsReadable: true,
		},
	}

	// Read file
	file, err := os.Open(path)
	if err != nil {
		return metadata, err
	}
	defer file.Close()

	// Parse timestamps to get duration
	scanner := bufio.NewScanner(file)
	var lastTimestamp string
	timestampRegex := regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2})`)

	for scanner.Scan() {
		line := scanner.Text()

		// Look for timestamps
		if timestampRegex.MatchString(line) {
			// Extract the end timestamp (typically "00:00:00 --> 00:05:30")
			parts := strings.Split(line, "-->")
			if len(parts) == 2 {
				lastTimestamp = strings.TrimSpace(parts[1])
			}
		}
	}

	if lastTimestamp != "" {
		metadata.Duration = &lastTimestamp
	}

	return metadata, scanner.Err()
}
