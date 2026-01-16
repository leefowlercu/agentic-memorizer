package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultMaxStructuredSize is the default maximum size for structured data files (50MB).
const DefaultMaxStructuredSize = 50 * 1024 * 1024

// DefaultSampleSize is the default number of records to include in sample.
const DefaultSampleSize = 10

// StructuredDataHandler handles structured data files (JSON, CSV, YAML, XML).
type StructuredDataHandler struct {
	maxSize    int64
	sampleSize int
}

// StructuredDataHandlerOption configures the StructuredDataHandler.
type StructuredDataHandlerOption func(*StructuredDataHandler)

// WithMaxStructuredSize sets the maximum file size for processing.
func WithMaxStructuredSize(size int64) StructuredDataHandlerOption {
	return func(h *StructuredDataHandler) {
		h.maxSize = size
	}
}

// WithSampleSize sets the number of records to include in sample.
func WithSampleSize(size int) StructuredDataHandlerOption {
	return func(h *StructuredDataHandler) {
		h.sampleSize = size
	}
}

// NewStructuredDataHandler creates a new StructuredDataHandler with the given options.
func NewStructuredDataHandler(opts ...StructuredDataHandlerOption) *StructuredDataHandler {
	h := &StructuredDataHandler{
		maxSize:    DefaultMaxStructuredSize,
		sampleSize: DefaultSampleSize,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Name returns the handler's unique identifier.
func (h *StructuredDataHandler) Name() string {
	return "structured_data"
}

// CanHandle returns true if this handler can process the given MIME type and extension.
func (h *StructuredDataHandler) CanHandle(mimeType string, ext string) bool {
	structuredMIMEs := map[string]bool{
		"application/json":          true,
		"application/x-ndjson":      true,
		"text/csv":                  true,
		"text/tab-separated-values": true,
		"text/yaml":                 true,
		"application/yaml":          true,
		"application/xml":           true,
		"text/xml":                  true,
	}

	if structuredMIMEs[mimeType] {
		return true
	}

	structuredExts := map[string]bool{
		".json":   true,
		".jsonl":  true,
		".ndjson": true,
		".csv":    true,
		".tsv":    true,
		".yaml":   true,
		".yml":    true,
		".xml":    true,
	}

	return structuredExts[strings.ToLower(ext)]
}

// Extract extracts content from the structured data file.
func (h *StructuredDataHandler) Extract(ctx context.Context, path string, size int64) (*ExtractedContent, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Check file size
	if size > h.maxSize {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        fmt.Sprintf("file too large: %d bytes (max %d)", size, h.maxSize),
			Metadata:     h.extractBasicMetadata(path, size),
		}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))

	var textContent string
	var schemaInfo map[string]any
	var extractErr error

	switch ext {
	case ".json":
		textContent, schemaInfo, extractErr = h.extractJSON(path)
	case ".jsonl", ".ndjson":
		textContent, schemaInfo, extractErr = h.extractNDJSON(path)
	case ".csv":
		textContent, schemaInfo, extractErr = h.extractCSV(path, ',')
	case ".tsv":
		textContent, schemaInfo, extractErr = h.extractCSV(path, '\t')
	case ".yaml", ".yml":
		textContent, schemaInfo, extractErr = h.extractYAML(path)
	case ".xml":
		textContent, schemaInfo, extractErr = h.extractXML(path)
	default:
		extractErr = fmt.Errorf("unsupported structured data format: %s", ext)
	}

	metadata := h.extractBasicMetadata(path, size)
	if schemaInfo != nil {
		metadata.Extra = schemaInfo
	}

	if extractErr != nil {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        extractErr.Error(),
			Metadata:     metadata,
		}, nil
	}

	return &ExtractedContent{
		Handler:     h.Name(),
		TextContent: textContent,
		Metadata:    metadata,
	}, nil
}

// MaxSize returns the maximum file size this handler will process.
func (h *StructuredDataHandler) MaxSize() int64 {
	return h.maxSize
}

// RequiresVision returns false as structured data is text-based.
func (h *StructuredDataHandler) RequiresVision() bool {
	return false
}

// SupportedExtensions returns the file extensions this handler supports.
func (h *StructuredDataHandler) SupportedExtensions() []string {
	return []string{".json", ".jsonl", ".ndjson", ".csv", ".tsv", ".yaml", ".yml", ".xml"}
}

// extractBasicMetadata extracts basic file metadata.
func (h *StructuredDataHandler) extractBasicMetadata(path string, size int64) *FileMetadata {
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

// extractJSON extracts schema and sample from a JSON file.
func (h *StructuredDataHandler) extractJSON(path string) (string, map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read json; %w", err)
	}

	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", nil, fmt.Errorf("invalid json; %w", err)
	}

	schemaInfo := make(map[string]any)
	schemaInfo["format"] = "json"
	schemaInfo["schema"] = inferJSONSchema(parsed)

	// Generate human-readable summary
	var summary strings.Builder
	summary.WriteString("JSON Document\n\n")
	summary.WriteString("Schema:\n")
	summary.WriteString(formatSchema(schemaInfo["schema"]))
	summary.WriteString("\n\nSample Data:\n")

	// Pretty print sample
	sample := getSample(parsed, h.sampleSize)
	sampleJSON, _ := json.MarshalIndent(sample, "", "  ")
	summary.Write(sampleJSON)

	return summary.String(), schemaInfo, nil
}

// extractNDJSON extracts schema and sample from a newline-delimited JSON file.
func (h *StructuredDataHandler) extractNDJSON(path string) (string, map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open ndjson; %w", err)
	}
	defer file.Close()

	schemaInfo := make(map[string]any)
	schemaInfo["format"] = "ndjson"

	var records []any
	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() && len(records) < h.sampleSize {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		lineCount++
		var record any
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			records = append(records, record)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("error reading ndjson; %w", err)
	}

	schemaInfo["record_count"] = lineCount
	if len(records) > 0 {
		schemaInfo["schema"] = inferJSONSchema(records[0])
	}

	// Generate summary
	var summary strings.Builder
	summary.WriteString("NDJSON Document\n\n")
	summary.WriteString(fmt.Sprintf("Records: %d\n\n", lineCount))
	summary.WriteString("Schema (from first record):\n")
	if schema, ok := schemaInfo["schema"]; ok {
		summary.WriteString(formatSchema(schema))
	}
	summary.WriteString("\n\nSample Records:\n")

	for i, record := range records {
		recordJSON, _ := json.MarshalIndent(record, "", "  ")
		summary.WriteString(fmt.Sprintf("--- Record %d ---\n", i+1))
		summary.Write(recordJSON)
		summary.WriteString("\n")
	}

	return summary.String(), schemaInfo, nil
}

// extractCSV extracts schema and sample from a CSV/TSV file.
func (h *StructuredDataHandler) extractCSV(path string, delimiter rune) (string, map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open csv; %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	schemaInfo := make(map[string]any)
	if delimiter == '\t' {
		schemaInfo["format"] = "tsv"
	} else {
		schemaInfo["format"] = "csv"
	}

	// Read header
	header, err := reader.Read()
	if err != nil {
		return "", nil, fmt.Errorf("failed to read csv header; %w", err)
	}

	schemaInfo["columns"] = header
	schemaInfo["column_count"] = len(header)

	// Read sample rows
	var rows [][]string
	rowCount := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip malformed rows
		}
		rowCount++
		if len(rows) < h.sampleSize {
			rows = append(rows, row)
		}
	}

	schemaInfo["row_count"] = rowCount

	// Generate summary
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("%s Document\n\n", strings.ToUpper(schemaInfo["format"].(string))))
	summary.WriteString(fmt.Sprintf("Columns: %d\n", len(header)))
	summary.WriteString(fmt.Sprintf("Rows: %d\n\n", rowCount))
	summary.WriteString("Column Names:\n")
	for i, col := range header {
		summary.WriteString(fmt.Sprintf("  %d. %s\n", i+1, col))
	}
	summary.WriteString("\nSample Data:\n")

	// Format as table-like structure
	for i, row := range rows {
		summary.WriteString(fmt.Sprintf("--- Row %d ---\n", i+1))
		for j, val := range row {
			colName := ""
			if j < len(header) {
				colName = header[j]
			}
			summary.WriteString(fmt.Sprintf("  %s: %s\n", colName, val))
		}
	}

	return summary.String(), schemaInfo, nil
}

// extractYAML extracts content from a YAML file.
func (h *StructuredDataHandler) extractYAML(path string) (string, map[string]any, error) {
	// Read file content directly since we don't want to add a YAML dependency
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read yaml; %w", err)
	}

	schemaInfo := make(map[string]any)
	schemaInfo["format"] = "yaml"

	// YAML is text-based, so we can include it directly
	content := string(data)
	schemaInfo["has_content"] = len(content) > 0

	var summary strings.Builder
	summary.WriteString("YAML Document\n\n")
	summary.WriteString("Content:\n")
	summary.WriteString(content)

	return summary.String(), schemaInfo, nil
}

// extractXML extracts content from an XML file.
func (h *StructuredDataHandler) extractXML(path string) (string, map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read xml; %w", err)
	}

	schemaInfo := make(map[string]any)
	schemaInfo["format"] = "xml"

	// Include XML content directly
	content := string(data)

	// Try to extract root element
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("<?xml")) {
		schemaInfo["has_declaration"] = true
	}

	var summary strings.Builder
	summary.WriteString("XML Document\n\n")
	summary.WriteString("Content:\n")
	summary.WriteString(content)

	return summary.String(), schemaInfo, nil
}

// inferJSONSchema infers a simple schema from JSON data.
func inferJSONSchema(data any) map[string]any {
	schema := make(map[string]any)

	switch v := data.(type) {
	case map[string]any:
		schema["type"] = "object"
		fields := make(map[string]any)
		for key, val := range v {
			fields[key] = inferJSONSchema(val)
		}
		schema["fields"] = fields
	case []any:
		schema["type"] = "array"
		if len(v) > 0 {
			schema["items"] = inferJSONSchema(v[0])
		}
		schema["length"] = len(v)
	case string:
		schema["type"] = "string"
	case float64:
		schema["type"] = "number"
	case bool:
		schema["type"] = "boolean"
	case nil:
		schema["type"] = "null"
	default:
		schema["type"] = "unknown"
	}

	return schema
}

// formatSchema formats a schema for human-readable output.
func formatSchema(schema any) string {
	var buf strings.Builder
	formatSchemaHelper(&buf, schema, 0)
	return buf.String()
}

func formatSchemaHelper(buf *strings.Builder, schema any, indent int) {
	prefix := strings.Repeat("  ", indent)

	switch s := schema.(type) {
	case map[string]any:
		schemaType, _ := s["type"].(string)
		fmt.Fprintf(buf, "%s%s", prefix, schemaType)

		if fields, ok := s["fields"].(map[string]any); ok {
			buf.WriteString(" {\n")
			for key, val := range fields {
				fmt.Fprintf(buf, "%s  %s: ", prefix, key)
				formatSchemaHelper(buf, val, indent+1)
				buf.WriteString("\n")
			}
			fmt.Fprintf(buf, "%s}", prefix)
		} else if items, ok := s["items"]; ok {
			buf.WriteString(" of ")
			formatSchemaHelper(buf, items, indent)
		}
	default:
		fmt.Fprintf(buf, "%v", schema)
	}
}

// getSample returns a sample of the data.
func getSample(data any, maxSize int) any {
	switch v := data.(type) {
	case []any:
		if len(v) <= maxSize {
			return v
		}
		return v[:maxSize]
	case map[string]any:
		// For objects, return the whole thing
		return v
	default:
		return data
	}
}
