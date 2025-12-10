package formatters

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// XMLFormatter renders output as XML
type XMLFormatter struct{}

// NewXMLFormatter creates a new XML formatter
func NewXMLFormatter() *XMLFormatter {
	return &XMLFormatter{}
}

// Format renders a single buildable structure
func (f *XMLFormatter) Format(b format.Buildable) (string, error) {
	if err := b.Validate(); err != nil {
		return "", fmt.Errorf("validation failed; %w", err)
	}

	var data any

	switch v := b.(type) {
	case *format.Section:
		data = f.sectionToXML(v)
	case *format.Table:
		data = f.tableToXML(v)
	case *format.List:
		data = f.listToXML(v)
	case *format.Progress:
		data = f.progressToXML(v)
	case *format.Status:
		data = f.statusToXML(v)
	case *format.Error:
		data = f.errorToXML(v)
	case *format.GraphContent:
		// GraphContent uses direct string generation for backward compatibility
		return f.formatGraph(v)
	default:
		return "", fmt.Errorf("unsupported builder type: %s", b.Type())
	}

	return f.marshal(data)
}

// FormatMultiple renders multiple buildable structures
func (f *XMLFormatter) FormatMultiple(builders []format.Buildable) (string, error) {
	items := make([]any, 0, len(builders))

	for _, b := range builders {
		if err := b.Validate(); err != nil {
			return "", fmt.Errorf("validation failed; %w", err)
		}

		switch v := b.(type) {
		case *format.Section:
			items = append(items, f.sectionToXML(v))
		case *format.Table:
			items = append(items, f.tableToXML(v))
		case *format.List:
			items = append(items, f.listToXML(v))
		case *format.Progress:
			items = append(items, f.progressToXML(v))
		case *format.Status:
			items = append(items, f.statusToXML(v))
		case *format.Error:
			items = append(items, f.errorToXML(v))
		default:
			return "", fmt.Errorf("unsupported builder type: %s", b.Type())
		}
	}

	// Wrap in root element
	wrapper := xmlMultiple{Items: items}
	return f.marshal(wrapper)
}

// Name returns the formatter name
func (f *XMLFormatter) Name() string {
	return "xml"
}

// SupportsColors returns false (XML doesn't support colors)
func (f *XMLFormatter) SupportsColors() bool {
	return false
}

// XML structure types

type xmlSection struct {
	XMLName xml.Name         `xml:"section"`
	Title   string           `xml:"title,attr"`
	Level   int              `xml:"level,attr,omitempty"`
	Items   []xmlSectionItem `xml:"item"`
}

type xmlSectionItem struct {
	Type       string      `xml:"type,attr"`
	Key        string      `xml:"key,attr,omitempty"`
	Value      string      `xml:"value,attr,omitempty"`
	Subsection *xmlSection `xml:"section,omitempty"`
}

type xmlTable struct {
	XMLName    xml.Name `xml:"table"`
	Headers    []string `xml:"headers>header"`
	Rows       []xmlRow `xml:"rows>row"`
	Alignments []string `xml:"alignments>alignment,omitempty"`
	Compact    bool     `xml:"compact,attr,omitempty"`
}

type xmlRow struct {
	Cells []string `xml:"cell"`
}

type xmlList struct {
	XMLName  xml.Name      `xml:"list"`
	ListType string        `xml:"type,attr"`
	Compact  bool          `xml:"compact,attr,omitempty"`
	Items    []xmlListItem `xml:"item"`
}

type xmlListItem struct {
	Content string   `xml:"content"`
	Nested  *xmlList `xml:"list,omitempty"`
}

type xmlProgress struct {
	XMLName      xml.Name `xml:"progress"`
	ProgressType string   `xml:"type,attr"`
	Current      int      `xml:"current"`
	Total        int      `xml:"total"`
	Percentage   float64  `xml:"percentage"`
	Message      string   `xml:"message,omitempty"`
}

type xmlStatus struct {
	XMLName  xml.Name `xml:"status"`
	Severity string   `xml:"severity,attr"`
	Symbol   string   `xml:"symbol,attr"`
	Message  string   `xml:"message"`
	Details  []string `xml:"details>detail,omitempty"`
}

type xmlError struct {
	XMLName    xml.Name `xml:"error"`
	ErrorType  string   `xml:"type,attr"`
	Message    string   `xml:"message"`
	Field      string   `xml:"field,omitempty"`
	Value      string   `xml:"value,omitempty"`
	Details    []string `xml:"details>detail,omitempty"`
	Suggestion string   `xml:"suggestion,omitempty"`
}

type xmlMultiple struct {
	XMLName xml.Name `xml:"output"`
	Items   []any    `xml:",any"`
}

// Conversion methods

func (f *XMLFormatter) sectionToXML(s *format.Section) *xmlSection {
	result := &xmlSection{
		Title: s.Title,
		Level: s.Level,
		Items: make([]xmlSectionItem, 0),
	}

	for _, item := range s.Items {
		if item.Type == format.SectionItemKeyValue {
			result.Items = append(result.Items, xmlSectionItem{
				Type:  "key_value",
				Key:   item.Key,
				Value: item.Value,
			})
		} else if item.Type == format.SectionItemSubsection {
			result.Items = append(result.Items, xmlSectionItem{
				Type:       "subsection",
				Subsection: f.sectionToXML(item.Subsection),
			})
		}
	}

	return result
}

func (f *XMLFormatter) tableToXML(t *format.Table) *xmlTable {
	alignments := make([]string, len(t.Alignments))
	for i, a := range t.Alignments {
		alignments[i] = string(a)
	}

	rows := make([]xmlRow, len(t.Rows))
	for i, row := range t.Rows {
		rows[i] = xmlRow{Cells: row}
	}

	return &xmlTable{
		Headers:    t.Headers,
		Rows:       rows,
		Alignments: alignments,
		Compact:    t.IsCompact,
	}
}

func (f *XMLFormatter) listToXML(l *format.List) *xmlList {
	items := make([]xmlListItem, len(l.Items))
	for i, item := range l.Items {
		items[i] = xmlListItem{
			Content: item.Content,
		}
		if item.Nested != nil {
			items[i].Nested = f.listToXML(item.Nested)
		}
	}

	return &xmlList{
		ListType: string(l.ListType),
		Items:    items,
		Compact:  l.IsCompact,
	}
}

func (f *XMLFormatter) progressToXML(p *format.Progress) *xmlProgress {
	return &xmlProgress{
		ProgressType: string(p.ProgressType),
		Current:      p.Current,
		Total:        p.Total,
		Percentage:   p.Percentage(),
		Message:      p.Message,
	}
}

func (f *XMLFormatter) statusToXML(s *format.Status) *xmlStatus {
	symbol := s.CustomSymbol
	if symbol == "" {
		symbol = format.GetStatusSymbol(s.Severity)
	}

	return &xmlStatus{
		Severity: string(s.Severity),
		Symbol:   symbol,
		Message:  s.Message,
		Details:  s.Details,
	}
}

func (f *XMLFormatter) errorToXML(e *format.Error) *xmlError {
	valueStr := ""
	if e.Value != nil {
		valueStr = fmt.Sprintf("%v", e.Value)
	}

	return &xmlError{
		ErrorType:  string(e.ErrorType),
		Message:    e.Message,
		Field:      e.Field,
		Value:      valueStr,
		Details:    e.Details,
		Suggestion: e.Suggestion,
	}
}

// formatGraph renders GraphIndex as XML (backward compatibility with legacy output)
func (f *XMLFormatter) formatGraph(gc *format.GraphContent) (string, error) {
	index := gc.Index
	var sb strings.Builder

	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<memory_index>\n")

	// Metadata section
	sb.WriteString("  <metadata>\n")
	sb.WriteString(fmt.Sprintf("    <generated>%s</generated>\n", index.Generated.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("    <memory_root>%s</memory_root>\n", xmlEscape(index.MemoryRoot)))
	sb.WriteString(fmt.Sprintf("    <file_count>%d</file_count>\n", index.Stats.TotalFiles))
	sb.WriteString(fmt.Sprintf("    <total_size_bytes>%d</total_size_bytes>\n", index.Stats.TotalSize))
	sb.WriteString(fmt.Sprintf("    <total_size_human>%s</total_size_human>\n", xmlEscape(format.FormatBytes(index.Stats.TotalSize))))

	// Graph statistics
	if index.Stats.TotalTags > 0 || index.Stats.TotalTopics > 0 || index.Stats.TotalEntities > 0 {
		sb.WriteString("    <graph_stats>\n")
		sb.WriteString(fmt.Sprintf("      <total_tags>%d</total_tags>\n", index.Stats.TotalTags))
		sb.WriteString(fmt.Sprintf("      <total_topics>%d</total_topics>\n", index.Stats.TotalTopics))
		sb.WriteString(fmt.Sprintf("      <total_entities>%d</total_entities>\n", index.Stats.TotalEntities))
		sb.WriteString(fmt.Sprintf("      <total_edges>%d</total_edges>\n", index.Stats.TotalEdges))
		sb.WriteString("    </graph_stats>\n")
	}

	// Coverage metrics
	if index.Stats.FilesWithSummary > 0 {
		sb.WriteString("    <coverage>\n")
		sb.WriteString(fmt.Sprintf("      <files_with_summary>%d</files_with_summary>\n", index.Stats.FilesWithSummary))
		sb.WriteString(fmt.Sprintf("      <files_with_tags>%d</files_with_tags>\n", index.Stats.FilesWithTags))
		sb.WriteString(fmt.Sprintf("      <files_with_topics>%d</files_with_topics>\n", index.Stats.FilesWithTopics))
		sb.WriteString(fmt.Sprintf("      <files_with_entities>%d</files_with_entities>\n", index.Stats.FilesWithEntities))
		sb.WriteString(fmt.Sprintf("      <avg_tags_per_file>%.1f</avg_tags_per_file>\n", index.Stats.AvgTagsPerFile))
		sb.WriteString("    </coverage>\n")
	}

	sb.WriteString("  </metadata>\n\n")

	// Knowledge summary (if available)
	if index.Knowledge != nil {
		sb.WriteString("  <knowledge>\n")
		if len(index.Knowledge.TopTags) > 0 {
			sb.WriteString("    <top_tags>\n")
			for _, tag := range index.Knowledge.TopTags {
				sb.WriteString(fmt.Sprintf("      <tag name=\"%s\" count=\"%d\"/>\n",
					xmlEscape(tag.Name), tag.Count))
			}
			sb.WriteString("    </top_tags>\n")
		}
		if len(index.Knowledge.TopTopics) > 0 {
			sb.WriteString("    <top_topics>\n")
			for _, topic := range index.Knowledge.TopTopics {
				sb.WriteString(fmt.Sprintf("      <topic name=\"%s\" count=\"%d\"/>\n",
					xmlEscape(topic.Name), topic.Count))
			}
			sb.WriteString("    </top_topics>\n")
		}
		if len(index.Knowledge.TopEntities) > 0 {
			sb.WriteString("    <top_entities>\n")
			for _, entity := range index.Knowledge.TopEntities {
				sb.WriteString(fmt.Sprintf("      <entity name=\"%s\" type=\"%s\" count=\"%d\"/>\n",
					xmlEscape(entity.Name), xmlEscape(entity.Type), entity.Count))
			}
			sb.WriteString("    </top_entities>\n")
		}
		sb.WriteString("  </knowledge>\n\n")
	}

	// Insights section (verbose mode only)
	if index.Insights != nil {
		sb.WriteString("  <insights>\n")

		// Recommendations
		if len(index.Insights.Recommendations) > 0 {
			sb.WriteString("    <top_connected_files>\n")
			for _, rec := range index.Insights.Recommendations {
				sb.WriteString(fmt.Sprintf("      <file path=\"%s\" connections=\"%.0f\">\n",
					xmlEscape(rec.TargetPath), rec.Score))
				sb.WriteString(fmt.Sprintf("        <name>%s</name>\n", xmlEscape(rec.TargetName)))
				sb.WriteString(fmt.Sprintf("        <reason>%s</reason>\n", xmlEscape(rec.Reason)))
				sb.WriteString("      </file>\n")
			}
			sb.WriteString("    </top_connected_files>\n")
		}

		// Topic clusters
		if len(index.Insights.TopicClusters) > 0 {
			sb.WriteString("    <topic_clusters>\n")
			for _, cluster := range index.Insights.TopicClusters {
				sb.WriteString(fmt.Sprintf("      <cluster name=\"%s\" file_count=\"%d\">\n",
					xmlEscape(cluster.Name), cluster.FileCount))
				if len(cluster.CommonTags) > 0 {
					sb.WriteString(fmt.Sprintf("        <common_tags>%s</common_tags>\n",
						xmlEscape(strings.Join(cluster.CommonTags, ", "))))
				}
				if len(cluster.FilePaths) > 0 {
					sb.WriteString("        <files>\n")
					for _, path := range cluster.FilePaths {
						sb.WriteString(fmt.Sprintf("          <file>%s</file>\n", xmlEscape(path)))
					}
					sb.WriteString("        </files>\n")
				}
				sb.WriteString("      </cluster>\n")
			}
			sb.WriteString("    </topic_clusters>\n")
		}

		// Coverage gaps
		if len(index.Insights.CoverageGaps) > 0 {
			sb.WriteString("    <coverage_gaps>\n")
			for _, gap := range index.Insights.CoverageGaps {
				sb.WriteString(fmt.Sprintf("      <gap type=\"%s\" severity=\"%s\">\n",
					xmlEscape(gap.Type), xmlEscape(gap.Severity)))
				sb.WriteString(fmt.Sprintf("        <name>%s</name>\n", xmlEscape(gap.Name)))
				sb.WriteString(fmt.Sprintf("        <description>%s</description>\n", xmlEscape(gap.Description)))
				if gap.Suggestion != "" {
					sb.WriteString(fmt.Sprintf("        <suggestion>%s</suggestion>\n", xmlEscape(gap.Suggestion)))
				}
				sb.WriteString("      </gap>\n")
			}
			sb.WriteString("    </coverage_gaps>\n")
		}

		sb.WriteString("  </insights>\n\n")
	}

	// Categories section
	categories := groupFilesByCategory(index.Files)
	sb.WriteString("  <categories>\n")

	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if files, ok := categories[category]; ok && len(files) > 0 {
			totalSize := int64(0)
			for _, file := range files {
				totalSize += file.Size
			}

			sb.WriteString(fmt.Sprintf("    <category name=\"%s\" count=\"%d\" total_size=\"%s\">\n",
				xmlEscape(category), len(files), xmlEscape(format.FormatBytes(totalSize))))

			sort.Slice(files, func(i, j int) bool {
				return files[i].Name < files[j].Name
			})

			for _, file := range files {
				sb.WriteString(formatFileEntryXML(&file))
			}

			sb.WriteString("    </category>\n")
		}
	}

	sb.WriteString("  </categories>\n\n")

	// Usage guide
	sb.WriteString("  <usage_guide>\n")
	sb.WriteString("    <direct_read_extensions>md, txt, json, yaml, vtt, go, py, js, ts, png, jpg</direct_read_extensions>\n")
	sb.WriteString("    <direct_read_tool>Read tool</direct_read_tool>\n")
	sb.WriteString("    <extraction_required_extensions>docx, pptx, pdf</extraction_required_extensions>\n")
	sb.WriteString("    <extraction_required_tool>Bash + conversion tools</extraction_required_tool>\n")
	sb.WriteString("  </usage_guide>\n")

	sb.WriteString("</memory_index>\n")

	return sb.String(), nil
}

// formatFileEntryXML formats a single FileEntry as XML
func formatFileEntryXML(file *types.FileEntry) string {
	var sb strings.Builder

	sb.WriteString("      <file>\n")
	sb.WriteString(fmt.Sprintf("        <name>%s</name>\n", xmlEscape(file.Name)))
	sb.WriteString(fmt.Sprintf("        <path>%s</path>\n", xmlEscape(file.Path)))
	sb.WriteString(fmt.Sprintf("        <modified>%s</modified>\n", file.Modified.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("        <size>%d</size>\n", file.Size))
	sb.WriteString(fmt.Sprintf("        <size_human>%s</size_human>\n", xmlEscape(file.SizeHuman)))
	sb.WriteString(fmt.Sprintf("        <type>%s</type>\n", xmlEscape(file.Type)))
	sb.WriteString(fmt.Sprintf("        <category>%s</category>\n", xmlEscape(file.Category)))
	sb.WriteString(fmt.Sprintf("        <readable>%t</readable>\n", file.IsReadable))

	// Type-specific metadata
	hasMetadata := file.PageCount != nil || file.SlideCount != nil ||
		file.WordCount != nil || file.Dimensions != nil ||
		file.Duration != nil || file.Language != nil

	if hasMetadata {
		sb.WriteString("        <metadata>\n")
		if file.PageCount != nil {
			sb.WriteString(fmt.Sprintf("          <page_count>%d</page_count>\n", *file.PageCount))
		}
		if file.SlideCount != nil {
			sb.WriteString(fmt.Sprintf("          <slide_count>%d</slide_count>\n", *file.SlideCount))
		}
		if file.WordCount != nil {
			sb.WriteString(fmt.Sprintf("          <word_count>%d</word_count>\n", *file.WordCount))
		}
		if file.Dimensions != nil {
			sb.WriteString(fmt.Sprintf("          <dimensions width=\"%d\" height=\"%d\"/>\n",
				file.Dimensions.Width, file.Dimensions.Height))
		}
		if file.Duration != nil {
			sb.WriteString(fmt.Sprintf("          <duration>%s</duration>\n", xmlEscape(*file.Duration)))
		}
		if file.Language != nil {
			sb.WriteString(fmt.Sprintf("          <language>%s</language>\n", xmlEscape(*file.Language)))
		}
		sb.WriteString("        </metadata>\n")
	}

	// Semantic understanding
	if file.Summary != "" {
		sb.WriteString(fmt.Sprintf("        <summary>%s</summary>\n", xmlEscape(file.Summary)))
	}
	if file.DocumentType != "" {
		sb.WriteString(fmt.Sprintf("        <document_type>%s</document_type>\n", xmlEscape(file.DocumentType)))
	}

	// Graph relationships
	if len(file.Topics) > 0 {
		sb.WriteString("        <topics>\n")
		for _, topic := range file.Topics {
			sb.WriteString(fmt.Sprintf("          <topic>%s</topic>\n", xmlEscape(topic)))
		}
		sb.WriteString("        </topics>\n")
	}
	if len(file.Tags) > 0 {
		sb.WriteString("        <tags>\n")
		for _, tag := range file.Tags {
			sb.WriteString(fmt.Sprintf("          <tag>%s</tag>\n", xmlEscape(tag)))
		}
		sb.WriteString("        </tags>\n")
	}
	if len(file.Entities) > 0 {
		sb.WriteString("        <entities>\n")
		for _, entity := range file.Entities {
			sb.WriteString(fmt.Sprintf("          <entity name=\"%s\" type=\"%s\"/>\n",
				xmlEscape(entity.Name), xmlEscape(entity.Type)))
		}
		sb.WriteString("        </entities>\n")
	}

	// Related files (verbose mode only)
	if len(file.RelatedFiles) > 0 {
		sb.WriteString("        <related_files>\n")
		for _, related := range file.RelatedFiles {
			sb.WriteString(fmt.Sprintf("          <related path=\"%s\" via=\"%s\" shared=\"%s\"/>\n",
				xmlEscape(related.Path), xmlEscape(related.Via), xmlEscape(strings.Join(related.Shared, ", "))))
		}
		sb.WriteString("        </related_files>\n")
	}

	if file.Error != nil {
		sb.WriteString(fmt.Sprintf("        <error>%s</error>\n", xmlEscape(*file.Error)))
	}

	sb.WriteString("      </file>\n")

	return sb.String()
}

// xmlEscape escapes special XML characters
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// groupFilesByCategory groups file entries by their category
func groupFilesByCategory(files []types.FileEntry) map[string][]types.FileEntry {
	categories := make(map[string][]types.FileEntry)

	for _, file := range files {
		categories[file.Category] = append(categories[file.Category], file)
	}

	return categories
}

// marshal converts data to XML with proper formatting
func (f *XMLFormatter) marshal(data any) (string, error) {
	bytes, err := xml.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("XML encoding failed; %w", err)
	}

	return xml.Header + string(bytes), nil
}

func init() {
	// Register XML formatter
	format.RegisterFormatter("xml", NewXMLFormatter())
}
