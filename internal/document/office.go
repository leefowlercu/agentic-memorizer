package document

import (
	"archive/zip"
	"io"
	"strings"
)

// OpenOfficeFile opens an Office file (PPTX, DOCX, XLSX) as a ZIP archive.
// Caller is responsible for closing the returned reader.
func OpenOfficeFile(path string) (*zip.ReadCloser, error) {
	return zip.OpenReader(path)
}

// ExtractTextFromTags extracts text content between XML tags with the given prefix.
// For example, ExtractTextFromTags(data, "a:t") extracts text from <a:t>content</a:t>.
func ExtractTextFromTags(data []byte, tagPrefix string) string {
	text := string(data)
	var result strings.Builder
	openTag := "<" + tagPrefix
	closeTag := "</" + tagPrefix + ">"

	for {
		start := strings.Index(text, openTag)
		if start == -1 {
			break
		}

		// Find the end of the opening tag
		tagEnd := strings.Index(text[start:], ">")
		if tagEnd == -1 {
			break
		}

		// Find the closing tag
		end := strings.Index(text[start:], closeTag)
		if end == -1 {
			break
		}

		// Extract text content between opening tag end and closing tag
		content := text[start+tagEnd+1 : start+end]
		if content != "" {
			result.WriteString(content)
			result.WriteString(" ")
		}

		text = text[start+end+len(closeTag):]
	}

	return strings.TrimSpace(result.String())
}

// ReadZipFile reads a file from a ZIP archive and returns its contents.
func ReadZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// FindFileInZip finds a file in a ZIP archive by exact name.
func FindFileInZip(reader *zip.ReadCloser, name string) *zip.File {
	for _, file := range reader.File {
		if file.Name == name {
			return file
		}
	}
	return nil
}

// FindFilesWithPrefix finds all files in a ZIP archive matching a prefix and suffix.
func FindFilesWithPrefix(reader *zip.ReadCloser, prefix, suffix string) []*zip.File {
	var matches []*zip.File
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, prefix) && strings.HasSuffix(file.Name, suffix) {
			matches = append(matches, file)
		}
	}
	return matches
}
