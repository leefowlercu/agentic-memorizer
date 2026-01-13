package export

import (
	"context"
	"fmt"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/export/envelopes"
	"github.com/leefowlercu/agentic-memorizer/internal/export/formatters"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// ExportStats contains statistics about an export operation.
type ExportStats struct {
	FileCount         int           `json:"file_count"`
	DirectoryCount    int           `json:"directory_count"`
	ChunkCount        int           `json:"chunk_count"`
	TagCount          int           `json:"tag_count"`
	TopicCount        int           `json:"topic_count"`
	EntityCount       int           `json:"entity_count"`
	RelationshipCount int           `json:"relationship_count"`
	ExportedAt        time.Time     `json:"exported_at"`
	Duration          time.Duration `json:"duration"`
	Format            string        `json:"format"`
	OutputSize        int           `json:"output_size"`
}

// ExportOptions configures an export operation.
type ExportOptions struct {
	// Format specifies the output format (xml, json, toon).
	Format string

	// Envelope specifies the envelope wrapper (none, claude-code, gemini-cli).
	Envelope string

	// IncludeContent includes file/chunk content in export.
	IncludeContent bool

	// IncludeEmbeddings includes vector embeddings in export.
	IncludeEmbeddings bool

	// MaxFiles limits the number of files exported (0 = unlimited).
	MaxFiles int

	// FilterTags limits export to files with specific tags.
	FilterTags []string

	// FilterTopics limits export to files covering specific topics.
	FilterTopics []string

	// FilterPaths limits export to files matching path patterns.
	FilterPaths []string
}

// DefaultExportOptions returns sensible defaults.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		Format:            "xml",
		Envelope:          "none",
		IncludeContent:    false,
		IncludeEmbeddings: false,
		MaxFiles:          0,
	}
}

// Exporter exports the knowledge graph in various formats.
type Exporter struct {
	graph      graph.Graph
	formatters map[string]formatters.Formatter
	envelopes  map[string]envelopes.Envelope
}

// NewExporter creates a new exporter.
func NewExporter(g graph.Graph) *Exporter {
	e := &Exporter{
		graph:      g,
		formatters: make(map[string]formatters.Formatter),
		envelopes:  make(map[string]envelopes.Envelope),
	}

	// Register default formatters
	e.RegisterFormatter("xml", formatters.NewXMLFormatter())
	e.RegisterFormatter("json", formatters.NewJSONFormatter())
	e.RegisterFormatter("toon", formatters.NewTOONFormatter())

	// Register default envelopes
	e.RegisterEnvelope("none", envelopes.NewNoneEnvelope())
	e.RegisterEnvelope("claude-code", envelopes.NewClaudeCodeEnvelope())
	e.RegisterEnvelope("gemini-cli", envelopes.NewGeminiCLIEnvelope())

	return e
}

// RegisterFormatter registers a formatter for a format name.
func (e *Exporter) RegisterFormatter(name string, f formatters.Formatter) {
	e.formatters[name] = f
}

// RegisterEnvelope registers an envelope for an envelope name.
func (e *Exporter) RegisterEnvelope(name string, env envelopes.Envelope) {
	e.envelopes[name] = env
}

// Export exports the knowledge graph with the given options.
func (e *Exporter) Export(ctx context.Context, opts ExportOptions) ([]byte, *ExportStats, error) {
	startTime := time.Now()

	// Get formatter
	formatter, ok := e.formatters[opts.Format]
	if !ok {
		return nil, nil, fmt.Errorf("unknown format: %s", opts.Format)
	}

	// Get envelope
	envelope, ok := e.envelopes[opts.Envelope]
	if !ok {
		return nil, nil, fmt.Errorf("unknown envelope: %s", opts.Envelope)
	}

	// Export snapshot from graph
	snapshot, err := e.graph.ExportSnapshot(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to export snapshot; %w", err)
	}

	// Apply filters
	snapshot = e.applyFilters(snapshot, opts)

	// Format the snapshot
	formatted, err := formatter.Format(snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format snapshot; %w", err)
	}

	// Build stats
	stats := &ExportStats{
		FileCount:         len(snapshot.Files),
		DirectoryCount:    len(snapshot.Directories),
		ChunkCount:        snapshot.TotalChunks,
		TagCount:          len(snapshot.Tags),
		TopicCount:        len(snapshot.Topics),
		EntityCount:       len(snapshot.Entities),
		RelationshipCount: snapshot.TotalRelationships,
		ExportedAt:        time.Now(),
		Duration:          time.Since(startTime),
		Format:            opts.Format,
		OutputSize:        len(formatted),
	}

	// Convert to envelope stats type
	envStats := &envelopes.ExportStats{
		FileCount:         stats.FileCount,
		DirectoryCount:    stats.DirectoryCount,
		ChunkCount:        stats.ChunkCount,
		TagCount:          stats.TagCount,
		TopicCount:        stats.TopicCount,
		EntityCount:       stats.EntityCount,
		RelationshipCount: stats.RelationshipCount,
		Format:            stats.Format,
		OutputSize:        stats.OutputSize,
	}

	// Wrap in envelope
	output, err := envelope.Wrap(formatted, envStats)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to wrap in envelope; %w", err)
	}

	stats.OutputSize = len(output)

	return output, stats, nil
}

// applyFilters filters the snapshot based on export options.
func (e *Exporter) applyFilters(snapshot *graph.GraphSnapshot, opts ExportOptions) *graph.GraphSnapshot {
	if opts.MaxFiles == 0 && len(opts.FilterTags) == 0 && len(opts.FilterTopics) == 0 && len(opts.FilterPaths) == 0 {
		return snapshot
	}

	// Create filtered copy
	filtered := &graph.GraphSnapshot{
		Directories:        snapshot.Directories,
		Tags:               snapshot.Tags,
		Topics:             snapshot.Topics,
		Entities:           snapshot.Entities,
		TotalChunks:        snapshot.TotalChunks,
		TotalRelationships: snapshot.TotalRelationships,
		ExportedAt:         snapshot.ExportedAt,
		Version:            snapshot.Version,
	}

	// Filter files
	var filteredFiles []graph.FileNode
	for _, file := range snapshot.Files {
		if e.fileMatchesFilters(file, opts) {
			filteredFiles = append(filteredFiles, file)
			if opts.MaxFiles > 0 && len(filteredFiles) >= opts.MaxFiles {
				break
			}
		}
	}
	filtered.Files = filteredFiles

	return filtered
}

// fileMatchesFilters checks if a file matches the filter criteria.
func (e *Exporter) fileMatchesFilters(file graph.FileNode, opts ExportOptions) bool {
	// Path filter
	if len(opts.FilterPaths) > 0 {
		matched := false
		for _, pattern := range opts.FilterPaths {
			if matchPath(file.Path, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// matchPath checks if a path matches a pattern (simple prefix/suffix matching).
func matchPath(path, pattern string) bool {
	if pattern == "" {
		return true
	}

	// Simple prefix matching for now
	if len(pattern) > 0 && pattern[0] == '*' {
		// Suffix match
		suffix := pattern[1:]
		return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
	}

	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		// Prefix match
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}

	// Exact match
	return path == pattern
}

// ListFormats returns available format names.
func (e *Exporter) ListFormats() []string {
	formats := make([]string, 0, len(e.formatters))
	for name := range e.formatters {
		formats = append(formats, name)
	}
	return formats
}

// ListEnvelopes returns available envelope names.
func (e *Exporter) ListEnvelopes() []string {
	envelopes := make([]string, 0, len(e.envelopes))
	for name := range e.envelopes {
		envelopes = append(envelopes, name)
	}
	return envelopes
}
