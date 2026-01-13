package formatters

import (
	"encoding/json"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// JSONFormatter formats snapshots as JSON.
type JSONFormatter struct {
	pretty bool
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{pretty: true}
}

// NewCompactJSONFormatter creates a JSON formatter without indentation.
func NewCompactJSONFormatter() *JSONFormatter {
	return &JSONFormatter{pretty: false}
}

// Name returns the formatter name.
func (f *JSONFormatter) Name() string {
	return "json"
}

// ContentType returns the MIME content type.
func (f *JSONFormatter) ContentType() string {
	return "application/json"
}

// FileExtension returns the typical file extension.
func (f *JSONFormatter) FileExtension() string {
	return ".json"
}

// jsonSnapshot is the JSON representation of a graph snapshot.
type jsonSnapshot struct {
	Version            int             `json:"version"`
	ExportedAt         string          `json:"exported_at"`
	Files              []jsonFile      `json:"files"`
	Directories        []jsonDirectory `json:"directories"`
	Tags               []jsonTag       `json:"tags"`
	Topics             []jsonTopic     `json:"topics"`
	Entities           []jsonEntity    `json:"entities"`
	TotalChunks        int             `json:"total_chunks"`
	TotalRelationships int             `json:"total_relationships"`
}

type jsonFile struct {
	Path            string `json:"path"`
	Name            string `json:"name"`
	Extension       string `json:"extension,omitempty"`
	MIMEType        string `json:"mime_type"`
	Language        string `json:"language,omitempty"`
	Size            int64  `json:"size"`
	ModTime         string `json:"mod_time"`
	ContentHash     string `json:"content_hash"`
	Summary         string `json:"summary,omitempty"`
	Complexity      int    `json:"complexity,omitempty"`
	AnalysisVersion int    `json:"analysis_version,omitempty"`
}

type jsonDirectory struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	IsRemembered bool   `json:"is_remembered"`
	FileCount    int    `json:"file_count"`
}

type jsonTag struct {
	Name       string `json:"name"`
	UsageCount int    `json:"usage_count"`
}

type jsonTopic struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	UsageCount  int    `json:"usage_count"`
}

type jsonEntity struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	UsageCount int    `json:"usage_count"`
}

// Format converts the graph snapshot to JSON.
func (f *JSONFormatter) Format(snapshot *graph.GraphSnapshot) ([]byte, error) {
	js := jsonSnapshot{
		Version:            snapshot.Version,
		ExportedAt:         snapshot.ExportedAt.Format("2006-01-02T15:04:05Z"),
		TotalChunks:        snapshot.TotalChunks,
		TotalRelationships: snapshot.TotalRelationships,
	}

	// Convert files
	js.Files = make([]jsonFile, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		js.Files = append(js.Files, jsonFile{
			Path:            file.Path,
			Name:            file.Name,
			Extension:       file.Extension,
			MIMEType:        file.MIMEType,
			Language:        file.Language,
			Size:            file.Size,
			ModTime:         file.ModTime.Format("2006-01-02T15:04:05Z"),
			ContentHash:     file.ContentHash,
			Summary:         file.Summary,
			Complexity:      file.Complexity,
			AnalysisVersion: file.AnalysisVersion,
		})
	}

	// Convert directories
	js.Directories = make([]jsonDirectory, 0, len(snapshot.Directories))
	for _, dir := range snapshot.Directories {
		js.Directories = append(js.Directories, jsonDirectory{
			Path:         dir.Path,
			Name:         dir.Name,
			IsRemembered: dir.IsRemembered,
			FileCount:    dir.FileCount,
		})
	}

	// Convert tags
	js.Tags = make([]jsonTag, 0, len(snapshot.Tags))
	for _, tag := range snapshot.Tags {
		js.Tags = append(js.Tags, jsonTag{
			Name:       tag.Name,
			UsageCount: tag.UsageCount,
		})
	}

	// Convert topics
	js.Topics = make([]jsonTopic, 0, len(snapshot.Topics))
	for _, topic := range snapshot.Topics {
		js.Topics = append(js.Topics, jsonTopic{
			Name:        topic.Name,
			Description: topic.Description,
			UsageCount:  topic.UsageCount,
		})
	}

	// Convert entities
	js.Entities = make([]jsonEntity, 0, len(snapshot.Entities))
	for _, entity := range snapshot.Entities {
		js.Entities = append(js.Entities, jsonEntity{
			Name:       entity.Name,
			Type:       entity.Type,
			UsageCount: entity.UsageCount,
		})
	}

	// Marshal to JSON
	var data []byte
	var err error

	if f.pretty {
		data, err = json.MarshalIndent(js, "", "  ")
	} else {
		data, err = json.Marshal(js)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode JSON; %w", err)
	}

	return data, nil
}
