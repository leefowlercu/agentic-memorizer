package handlers

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArchiveHandler handles archive files (ZIP, TAR, etc.) by extracting metadata only.
type ArchiveHandler struct{}

// NewArchiveHandler creates a new ArchiveHandler.
func NewArchiveHandler() *ArchiveHandler {
	return &ArchiveHandler{}
}

// Name returns the handler's unique identifier.
func (h *ArchiveHandler) Name() string {
	return "archive"
}

// CanHandle returns true if this handler can process the given MIME type and extension.
func (h *ArchiveHandler) CanHandle(mimeType string, ext string) bool {
	if IsArchiveMIME(mimeType) {
		return true
	}

	archiveExts := map[string]bool{
		".zip": true,
		".tar": true,
		".gz":  true,
		".tgz": true,
		".bz2": true,
		".xz":  true,
		".7z":  true,
		".rar": true,
		".jar": true,
		".war": true,
		".ear": true,
	}

	return archiveExts[strings.ToLower(ext)]
}

// Extract extracts metadata from the archive file.
// Archives always skip semantic analysis but include file listings.
func (h *ArchiveHandler) Extract(ctx context.Context, path string, size int64) (*ExtractedContent, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(path))
	metadata := h.extractBasicMetadata(path, size)

	var fileList []string
	var archiveErr error

	switch ext {
	case ".zip", ".jar", ".war", ".ear":
		fileList, archiveErr = listZipContents(path)
	case ".tar":
		fileList, archiveErr = listTarContents(path)
	case ".gz", ".tgz":
		fileList, archiveErr = listGzipContents(path)
	default:
		// For unsupported archive types, just report metadata
		archiveErr = fmt.Errorf("archive listing not supported for %s", ext)
	}

	if archiveErr != nil {
		metadata.Extra = map[string]any{
			"error": archiveErr.Error(),
		}
	} else {
		metadata.Extra = map[string]any{
			"file_count": len(fileList),
			"files":      fileList,
		}
	}

	// Generate text summary for context
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Archive: %s\n", filepath.Base(path)))
	summary.WriteString(fmt.Sprintf("Size: %d bytes\n", size))
	summary.WriteString(fmt.Sprintf("Format: %s\n\n", ext))

	if len(fileList) > 0 {
		summary.WriteString(fmt.Sprintf("Contents (%d files):\n", len(fileList)))
		maxShow := 50
		for i, f := range fileList {
			if i >= maxShow {
				summary.WriteString(fmt.Sprintf("  ... and %d more files\n", len(fileList)-maxShow))
				break
			}
			summary.WriteString(fmt.Sprintf("  %s\n", f))
		}
	}

	return &ExtractedContent{
		Handler:      h.Name(),
		TextContent:  summary.String(),
		Metadata:     metadata,
		SkipAnalysis: true, // Archives always skip semantic analysis
	}, nil
}

// MaxSize returns 0 as archives don't have a meaningful max size for metadata extraction.
func (h *ArchiveHandler) MaxSize() int64 {
	return 0 // No limit for metadata extraction
}

// RequiresVision returns false as archives don't need vision API.
func (h *ArchiveHandler) RequiresVision() bool {
	return false
}

// SupportedExtensions returns the file extensions this handler supports.
func (h *ArchiveHandler) SupportedExtensions() []string {
	return []string{".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z", ".rar", ".jar", ".war", ".ear"}
}

// extractBasicMetadata extracts basic file metadata.
func (h *ArchiveHandler) extractBasicMetadata(path string, size int64) *FileMetadata {
	ext := filepath.Ext(path)
	info, _ := os.Stat(path)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	return &FileMetadata{
		Path:      path,
		Size:      size,
		ModTime:   modTime,
		MIMEType:  detectMIMEType(path, ext),
		Extension: ext,
	}
}

// listZipContents lists files in a ZIP archive.
func listZipContents(path string) ([]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip; %w", err)
	}
	defer r.Close()

	var files []string
	for _, f := range r.File {
		files = append(files, f.Name)
	}

	return files, nil
}

// listTarContents lists files in a TAR archive.
func listTarContents(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open tar; %w", err)
	}
	defer file.Close()

	tr := tar.NewReader(file)
	var files []string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return files, fmt.Errorf("error reading tar; %w", err)
		}
		files = append(files, header.Name)
	}

	return files, nil
}

// listGzipContents lists the contents of a gzip file (tar.gz or single file).
func listGzipContents(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip; %w", err)
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader; %w", err)
	}
	defer gr.Close()

	// Check if it's a tar.gz
	if strings.HasSuffix(strings.ToLower(path), ".tar.gz") || strings.HasSuffix(strings.ToLower(path), ".tgz") {
		tr := tar.NewReader(gr)
		var files []string

		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return files, fmt.Errorf("error reading tar.gz; %w", err)
			}
			files = append(files, header.Name)
		}

		return files, nil
	}

	// Single file gzip
	name := gr.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".gz")
	}

	return []string{name}, nil
}
