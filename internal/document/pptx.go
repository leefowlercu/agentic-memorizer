package document

import (
	"strings"
)

// PptxMetadata contains extracted metadata from a PPTX file.
type PptxMetadata struct {
	SlideCount int
	Author     string
}

// ExtractPptxText extracts all text content from a PPTX file.
// Text is extracted from <a:t> tags within slide XML files.
func ExtractPptxText(path string) (string, error) {
	reader, err := OpenOfficeFile(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var allText strings.Builder

	// Find all slide files
	slides := FindFilesWithPrefix(reader, "ppt/slides/slide", ".xml")

	for _, slide := range slides {
		data, err := ReadZipFile(slide)
		if err != nil {
			continue
		}

		// Extract text from <a:t> tags (PowerPoint text runs)
		text := ExtractTextFromTags(data, "a:t")
		if text != "" {
			allText.WriteString(text)
			allText.WriteString("\n\n")
		}
	}

	return strings.TrimSpace(allText.String()), nil
}

// ExtractPptxMetadata extracts metadata from a PPTX file.
func ExtractPptxMetadata(path string) (*PptxMetadata, error) {
	reader, err := OpenOfficeFile(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	metadata := &PptxMetadata{}

	// Count slides
	slides := FindFilesWithPrefix(reader, "ppt/slides/slide", ".xml")
	metadata.SlideCount = len(slides)

	// Extract author from core properties
	coreProps := FindFileInZip(reader, "docProps/core.xml")
	if coreProps != nil {
		data, err := ReadZipFile(coreProps)
		if err == nil {
			metadata.Author = extractCreatorFromCoreProps(data)
		}
	}

	return metadata, nil
}
