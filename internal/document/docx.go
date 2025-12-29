package document

import (
	"encoding/xml"
	"strings"
)

// DocxMetadata contains extracted metadata from a DOCX file.
type DocxMetadata struct {
	WordCount int
	Author    string
}

// ExtractDocxText extracts all text content from a DOCX file.
// Text is extracted from <w:t> tags within word/document.xml.
func ExtractDocxText(path string) (string, error) {
	reader, err := OpenOfficeFile(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	// Find word/document.xml
	docFile := FindFileInZip(reader, "word/document.xml")
	if docFile == nil {
		return "", nil
	}

	data, err := ReadZipFile(docFile)
	if err != nil {
		return "", err
	}

	// Extract text from <w:t> tags (Word text runs)
	return ExtractTextFromTags(data, "w:t"), nil
}

// ExtractDocxMetadata extracts metadata from a DOCX file.
func ExtractDocxMetadata(path string) (*DocxMetadata, error) {
	reader, err := OpenOfficeFile(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	metadata := &DocxMetadata{}

	// Extract word count from document content
	docFile := FindFileInZip(reader, "word/document.xml")
	if docFile != nil {
		data, err := ReadZipFile(docFile)
		if err == nil {
			text := ExtractTextFromTags(data, "w:t")
			words := strings.Fields(text)
			metadata.WordCount = len(words)
		}
	}

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

// extractCreatorFromCoreProps extracts the creator field from core.xml data.
func extractCreatorFromCoreProps(data []byte) string {
	type CoreProperties struct {
		Creator string `xml:"creator"`
	}

	var props CoreProperties
	if err := xml.Unmarshal(data, &props); err == nil {
		return props.Creator
	}
	return ""
}
