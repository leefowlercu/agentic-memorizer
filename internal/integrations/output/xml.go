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

// xmlEscape escapes special XML characters
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// FormatGraph renders the graph index as XML (new flattened format)
func (p *XMLProcessor) FormatGraph(index *types.GraphIndex) (string, error) {
	var sb strings.Builder

	sb.WriteString("<memory_index>\n")

	// Metadata section
	sb.WriteString("  <metadata>\n")
	sb.WriteString(fmt.Sprintf("    <generated>%s</generated>\n", index.Generated.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("    <memory_root>%s</memory_root>\n", xmlEscape(index.MemoryRoot)))
	sb.WriteString(fmt.Sprintf("    <file_count>%d</file_count>\n", index.Stats.TotalFiles))
	sb.WriteString(fmt.Sprintf("    <total_size_bytes>%d</total_size_bytes>\n", index.Stats.TotalSize))
	sb.WriteString(fmt.Sprintf("    <total_size_human>%s</total_size_human>\n", xmlEscape(formatSize(index.Stats.TotalSize))))

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

	// Recent activity section (if enabled)
	if p.options.ShowRecentDays > 0 {
		recentFiles := getRecentFileEntries(index.Files, p.options.ShowRecentDays)
		if len(recentFiles) > 0 {
			sb.WriteString(fmt.Sprintf("  <recent_activity days=\"%d\">\n", p.options.ShowRecentDays))
			for _, file := range recentFiles {
				relPath, _ := filepath.Rel(index.MemoryRoot, file.Path)
				sb.WriteString("    <file>\n")
				sb.WriteString(fmt.Sprintf("      <path>%s</path>\n", xmlEscape(relPath)))
				sb.WriteString(fmt.Sprintf("      <modified>%s</modified>\n", file.Modified.Format("2006-01-02")))
				sb.WriteString("    </file>\n")
			}
			sb.WriteString("  </recent_activity>\n\n")
		}
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
				xmlEscape(category), len(files), xmlEscape(formatSize(totalSize))))

			sort.Slice(files, func(i, j int) bool {
				return files[i].Name < files[j].Name
			})

			for _, file := range files {
				sb.WriteString(p.formatFileEntry(&file))
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

// formatFileEntry formats a single FileEntry as XML (flattened format)
func (p *XMLProcessor) formatFileEntry(file *types.FileEntry) string {
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

	// Semantic understanding (flattened in new format)
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
