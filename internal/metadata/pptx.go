package metadata

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// PptxHandler extracts metadata from PPTX files
type PptxHandler struct{}

// CanHandle returns true if this handler can process the file
func (h *PptxHandler) CanHandle(ext string) bool {
	return ext == ".pptx"
}

// Extract extracts metadata from a PPTX file
func (h *PptxHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "pptx",
			Category:   "presentations",
			IsReadable: false,
		},
	}

	// Open PPTX as ZIP
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return metadata, err
	}
	defer zipReader.Close()

	// Count slides
	slideCount := 0
	for _, file := range zipReader.File {
		if strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			slideCount++
		}
	}

	if slideCount > 0 {
		metadata.SlideCount = &slideCount
	}

	// Extract core properties
	for _, file := range zipReader.File {
		if file.Name == "docProps/core.xml" {
			if err := h.extractCoreProps(file, metadata); err != nil {
				continue
			}
		}
	}

	return metadata, nil
}

// ExtractText extracts text content from all slides in a PPTX file
func (h *PptxHandler) ExtractText(path string) (string, error) {
	// Open PPTX as ZIP
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zipReader.Close()

	var allText strings.Builder

	// Extract text from each slide
	for _, file := range zipReader.File {
		if strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			rc, err := file.Open()
			if err != nil {
				continue
			}

			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Extract text between <a:t> tags (PowerPoint text runs)
			text := string(data)
			for {
				start := strings.Index(text, "<a:t")
				if start == -1 {
					break
				}

				// Find the end of the opening tag
				tagEnd := strings.Index(text[start:], ">")
				if tagEnd == -1 {
					break
				}

				// Find the closing tag
				end := strings.Index(text[start:], "</a:t>")
				if end == -1 {
					break
				}

				// Extract text content
				content := text[start+tagEnd+1 : start+end]
				if content != "" {
					allText.WriteString(content)
					allText.WriteString(" ")
				}

				text = text[start+end+6:] // Move past this element
			}

			allText.WriteString("\n\n")
		}
	}

	return strings.TrimSpace(allText.String()), nil
}

// extractCoreProps extracts core document properties
func (h *PptxHandler) extractCoreProps(file *zip.File, metadata *types.FileMetadata) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

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
