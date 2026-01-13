package formatters

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// XMLFormatter formats snapshots as XML.
type XMLFormatter struct{}

// NewXMLFormatter creates a new XML formatter.
func NewXMLFormatter() *XMLFormatter {
	return &XMLFormatter{}
}

// Name returns the formatter name.
func (f *XMLFormatter) Name() string {
	return "xml"
}

// ContentType returns the MIME content type.
func (f *XMLFormatter) ContentType() string {
	return "application/xml"
}

// FileExtension returns the typical file extension.
func (f *XMLFormatter) FileExtension() string {
	return ".xml"
}

// xmlSnapshot is the XML representation of a graph snapshot.
type xmlSnapshot struct {
	XMLName     xml.Name       `xml:"knowledge-graph"`
	Version     int            `xml:"version,attr"`
	ExportedAt  string         `xml:"exported-at,attr"`
	Files       []xmlFile      `xml:"files>file"`
	Directories []xmlDirectory `xml:"directories>directory"`
	Tags        []xmlTag       `xml:"tags>tag"`
	Topics      []xmlTopic     `xml:"topics>topic"`
	Entities    []xmlEntity    `xml:"entities>entity"`
	Stats       xmlStats       `xml:"stats"`
}

type xmlFile struct {
	Path            string `xml:"path,attr"`
	Name            string `xml:"name"`
	Extension       string `xml:"extension,omitempty"`
	MIMEType        string `xml:"mime-type"`
	Language        string `xml:"language,omitempty"`
	Size            int64  `xml:"size"`
	ModTime         string `xml:"mod-time"`
	ContentHash     string `xml:"content-hash"`
	Summary         string `xml:"summary,omitempty"`
	Complexity      int    `xml:"complexity,omitempty"`
	AnalysisVersion int    `xml:"analysis-version,omitempty"`
}

type xmlDirectory struct {
	Path         string `xml:"path,attr"`
	Name         string `xml:"name"`
	IsRemembered bool   `xml:"is-remembered"`
	FileCount    int    `xml:"file-count"`
}

type xmlTag struct {
	Name       string `xml:"name,attr"`
	UsageCount int    `xml:"usage-count"`
}

type xmlTopic struct {
	Name        string `xml:"name,attr"`
	Description string `xml:"description,omitempty"`
	UsageCount  int    `xml:"usage-count"`
}

type xmlEntity struct {
	Name       string `xml:"name,attr"`
	Type       string `xml:"type,attr"`
	UsageCount int    `xml:"usage-count"`
}

type xmlStats struct {
	TotalFiles         int `xml:"total-files"`
	TotalDirectories   int `xml:"total-directories"`
	TotalChunks        int `xml:"total-chunks"`
	TotalTags          int `xml:"total-tags"`
	TotalTopics        int `xml:"total-topics"`
	TotalEntities      int `xml:"total-entities"`
	TotalRelationships int `xml:"total-relationships"`
}

// Format converts the graph snapshot to XML.
func (f *XMLFormatter) Format(snapshot *graph.GraphSnapshot) ([]byte, error) {
	xs := xmlSnapshot{
		Version:    snapshot.Version,
		ExportedAt: snapshot.ExportedAt.Format("2006-01-02T15:04:05Z"),
		Stats: xmlStats{
			TotalFiles:         len(snapshot.Files),
			TotalDirectories:   len(snapshot.Directories),
			TotalChunks:        snapshot.TotalChunks,
			TotalTags:          len(snapshot.Tags),
			TotalTopics:        len(snapshot.Topics),
			TotalEntities:      len(snapshot.Entities),
			TotalRelationships: snapshot.TotalRelationships,
		},
	}

	// Convert files
	for _, file := range snapshot.Files {
		xs.Files = append(xs.Files, xmlFile{
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
	for _, dir := range snapshot.Directories {
		xs.Directories = append(xs.Directories, xmlDirectory{
			Path:         dir.Path,
			Name:         dir.Name,
			IsRemembered: dir.IsRemembered,
			FileCount:    dir.FileCount,
		})
	}

	// Convert tags
	for _, tag := range snapshot.Tags {
		xs.Tags = append(xs.Tags, xmlTag{
			Name:       tag.Name,
			UsageCount: tag.UsageCount,
		})
	}

	// Convert topics
	for _, topic := range snapshot.Topics {
		xs.Topics = append(xs.Topics, xmlTopic{
			Name:        topic.Name,
			Description: topic.Description,
			UsageCount:  topic.UsageCount,
		})
	}

	// Convert entities
	for _, entity := range snapshot.Entities {
		xs.Entities = append(xs.Entities, xmlEntity{
			Name:       entity.Name,
			Type:       entity.Type,
			UsageCount: entity.UsageCount,
		})
	}

	// Marshal to XML
	var buf bytes.Buffer
	buf.WriteString(xml.Header)

	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	if err := encoder.Encode(xs); err != nil {
		return nil, fmt.Errorf("failed to encode XML; %w", err)
	}

	return buf.Bytes(), nil
}
