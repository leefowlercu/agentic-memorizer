package output

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// XMLProcessor formats the memory index as XML
type XMLProcessor struct {
	options Options
}

// NewXMLProcessor creates a new XML output processor
func NewXMLProcessor(opts ...Options) *XMLProcessor {
	options := DefaultOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	return &XMLProcessor{
		options: options,
	}
}

// GetFormat returns the format name
func (p *XMLProcessor) GetFormat() string {
	return "xml"
}

// Format renders the index as XML
func (p *XMLProcessor) Format(index *types.Index) (string, error) {
	var sb strings.Builder

	sb.WriteString("<memory_index>\n")

	// Metadata section
	sb.WriteString("  <metadata>\n")
	sb.WriteString(fmt.Sprintf("    <generated>%s</generated>\n", index.Generated.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("    <file_count>%d</file_count>\n", index.Stats.TotalFiles))
	sb.WriteString(fmt.Sprintf("    <total_size_bytes>%d</total_size_bytes>\n", index.Stats.TotalSize))
	sb.WriteString(fmt.Sprintf("    <total_size_human>%s</total_size_human>\n", xmlEscape(formatSize(index.Stats.TotalSize))))
	sb.WriteString(fmt.Sprintf("    <root_path>%s</root_path>\n", xmlEscape(index.Root)))
	sb.WriteString("    <cache_stats>\n")
	sb.WriteString(fmt.Sprintf("      <cached_files>%d</cached_files>\n", index.Stats.CachedFiles))
	sb.WriteString(fmt.Sprintf("      <analyzed_files>%d</analyzed_files>\n", index.Stats.AnalyzedFiles))
	sb.WriteString("    </cache_stats>\n")
	sb.WriteString("  </metadata>\n\n")

	// Recent activity section (if enabled)
	if p.options.ShowRecentDays > 0 {
		recentEntries := getRecentEntries(index.Entries, p.options.ShowRecentDays)
		if len(recentEntries) > 0 {
			sb.WriteString(fmt.Sprintf("  <recent_activity days=\"%d\">\n", p.options.ShowRecentDays))
			for _, entry := range recentEntries {
				relPath, _ := filepath.Rel(index.Root, entry.Metadata.Path)
				sb.WriteString("    <file>\n")
				sb.WriteString(fmt.Sprintf("      <path>%s</path>\n", xmlEscape(relPath)))
				sb.WriteString(fmt.Sprintf("      <modified>%s</modified>\n", entry.Metadata.Modified.Format("2006-01-02")))
				sb.WriteString("    </file>\n")
			}
			sb.WriteString("  </recent_activity>\n\n")
		}
	}

	// Categories section
	categories := groupByCategory(index.Entries)
	sb.WriteString("  <categories>\n")

	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if entries, ok := categories[category]; ok && len(entries) > 0 {
			totalSize := int64(0)
			for _, entry := range entries {
				totalSize += entry.Metadata.Size
			}

			sb.WriteString(fmt.Sprintf("    <category name=\"%s\" count=\"%d\" total_size=\"%s\">\n",
				xmlEscape(category), len(entries), xmlEscape(formatSize(totalSize))))

			sort.Slice(entries, func(i, j int) bool {
				return filepath.Base(entries[i].Metadata.Path) < filepath.Base(entries[j].Metadata.Path)
			})

			for _, entry := range entries {
				sb.WriteString(p.formatEntry(&entry))
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

// formatEntry formats a single index entry as XML
func (p *XMLProcessor) formatEntry(entry *types.IndexEntry) string {
	var sb strings.Builder

	filename := filepath.Base(entry.Metadata.Path)

	sb.WriteString("      <file>\n")
	sb.WriteString(fmt.Sprintf("        <name>%s</name>\n", xmlEscape(filename)))
	sb.WriteString(fmt.Sprintf("        <path>%s</path>\n", xmlEscape(entry.Metadata.Path)))
	sb.WriteString(fmt.Sprintf("        <modified>%s</modified>\n", entry.Metadata.Modified.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("        <size_bytes>%d</size_bytes>\n", entry.Metadata.Size))
	sb.WriteString(fmt.Sprintf("        <size_human>%s</size_human>\n", xmlEscape(formatSize(entry.Metadata.Size))))
	sb.WriteString(fmt.Sprintf("        <file_type>%s</file_type>\n", xmlEscape(entry.Metadata.Type)))
	sb.WriteString(fmt.Sprintf("        <category>%s</category>\n", xmlEscape(entry.Metadata.Category)))
	sb.WriteString(fmt.Sprintf("        <readable>%t</readable>\n", entry.Metadata.IsReadable))

	hasMetadata := entry.Metadata.PageCount != nil || entry.Metadata.SlideCount != nil ||
		entry.Metadata.WordCount != nil || entry.Metadata.Dimensions != nil ||
		entry.Metadata.Duration != nil || entry.Metadata.Language != nil

	if hasMetadata {
		sb.WriteString("        <metadata>\n")
		if entry.Metadata.PageCount != nil {
			sb.WriteString(fmt.Sprintf("          <page_count>%d</page_count>\n", *entry.Metadata.PageCount))
		}
		if entry.Metadata.SlideCount != nil {
			sb.WriteString(fmt.Sprintf("          <slide_count>%d</slide_count>\n", *entry.Metadata.SlideCount))
		}
		if entry.Metadata.WordCount != nil {
			sb.WriteString(fmt.Sprintf("          <word_count>%d</word_count>\n", *entry.Metadata.WordCount))
		}
		if entry.Metadata.Dimensions != nil {
			sb.WriteString(fmt.Sprintf("          <dimensions width=\"%d\" height=\"%d\"/>\n",
				entry.Metadata.Dimensions.Width, entry.Metadata.Dimensions.Height))
		}
		if entry.Metadata.Duration != nil {
			sb.WriteString(fmt.Sprintf("          <duration>%s</duration>\n", xmlEscape(*entry.Metadata.Duration)))
		}
		if entry.Metadata.Language != nil {
			sb.WriteString(fmt.Sprintf("          <language>%s</language>\n", xmlEscape(*entry.Metadata.Language)))
		}
		if len(entry.Metadata.Sections) > 0 {
			sb.WriteString("          <sections>\n")
			for _, section := range entry.Metadata.Sections {
				sb.WriteString(fmt.Sprintf("            <section>%s</section>\n", xmlEscape(section)))
			}
			sb.WriteString("          </sections>\n")
		}
		sb.WriteString("        </metadata>\n")
	}

	if entry.Semantic != nil {
		sb.WriteString("        <semantic>\n")
		sb.WriteString(fmt.Sprintf("          <summary>%s</summary>\n", xmlEscape(entry.Semantic.Summary)))
		if entry.Semantic.DocumentType != "" {
			sb.WriteString(fmt.Sprintf("          <document_type>%s</document_type>\n", xmlEscape(entry.Semantic.DocumentType)))
		}
		if len(entry.Semantic.KeyTopics) > 0 {
			sb.WriteString("          <topics>\n")
			for _, topic := range entry.Semantic.KeyTopics {
				sb.WriteString(fmt.Sprintf("            <topic>%s</topic>\n", xmlEscape(topic)))
			}
			sb.WriteString("          </topics>\n")
		}
		if len(entry.Semantic.Tags) > 0 {
			sb.WriteString("          <tags>\n")
			for _, tag := range entry.Semantic.Tags {
				sb.WriteString(fmt.Sprintf("            <tag>%s</tag>\n", xmlEscape(tag)))
			}
			sb.WriteString("          </tags>\n")
		}
		sb.WriteString("        </semantic>\n")
	}

	if entry.Error != nil {
		sb.WriteString(fmt.Sprintf("        <error>%s</error>\n", xmlEscape(*entry.Error)))
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
