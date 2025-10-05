package metadata

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// DocxHandler extracts metadata from DOCX files
type DocxHandler struct{}

// CanHandle returns true if this handler can process the file
func (h *DocxHandler) CanHandle(ext string) bool {
	return ext == ".docx"
}

// Extract extracts metadata from a DOCX file
func (h *DocxHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "docx",
			Category:   "documents",
			IsReadable: false,
		},
	}

	// Open DOCX as ZIP
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return metadata, err
	}
	defer zipReader.Close()

	// Extract core properties (metadata)
	for _, file := range zipReader.File {
		if file.Name == "docProps/core.xml" {
			if err := h.extractCoreProps(file, metadata); err != nil {
				// Continue even if core props fail
				continue
			}
		}
	}

	// Extract text content for word count
	for _, file := range zipReader.File {
		if file.Name == "word/document.xml" {
			if err := h.extractWordCount(file, metadata); err != nil {
				// Continue even if word count fails
				continue
			}
			break
		}
	}

	return metadata, nil
}

// extractCoreProps extracts core document properties
func (h *DocxHandler) extractCoreProps(file *zip.File, metadata *types.FileMetadata) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	// Simple XML parsing for creator
	type CoreProperties struct {
		Creator string `xml:"creator"`
	}

	var props CoreProperties
	if err := xml.Unmarshal(data, &props); err == nil {
		if props.Creator != "" {
			metadata.Author = &props.Creator
		}
	}

	return nil
}

// extractWordCount extracts word count from document
func (h *DocxHandler) extractWordCount(file *zip.File, metadata *types.FileMetadata) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	// Extract text between <w:t> tags
	text := string(data)
	wordCount := 0

	// Simple approach: count text in <w:t> elements
	for {
		start := strings.Index(text, "<w:t")
		if start == -1 {
			break
		}

		// Find the end of the opening tag
		tagEnd := strings.Index(text[start:], ">")
		if tagEnd == -1 {
			break
		}

		// Find the closing tag
		end := strings.Index(text[start:], "</w:t>")
		if end == -1 {
			break
		}

		// Extract text content
		content := text[start+tagEnd+1 : start+end]
		words := strings.Fields(content)
		wordCount += len(words)

		text = text[start+end+6:] // Move past this element
	}

	if wordCount > 0 {
		metadata.WordCount = &wordCount
	}

	return nil
}
