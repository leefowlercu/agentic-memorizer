package output

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

type Formatter struct {
	verbose        bool
	showRecentDays int
}

type HookOutput struct {
	Continue           bool                `json:"continue"`
	StopReason         *string             `json:"stopReason,omitempty"`
	SuppressOutput     bool                `json:"suppressOutput"`
	SystemMessage      string              `json:"systemMessage,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

type HookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

func NewFormatter(verbose bool, showRecentDays int) *Formatter {
	return &Formatter{
		verbose:        verbose,
		showRecentDays: showRecentDays,
	}
}

func (f *Formatter) generateSystemMessage(index *types.Index) string {
	categories := f.groupByCategory(index.Entries)
	categoryCount := len(categories)

	categoryParts := []string{}
	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if entries, ok := categories[category]; ok && len(entries) > 0 {
			categoryParts = append(categoryParts, fmt.Sprintf("%d %s", len(entries), category))
		}
	}

	msg := fmt.Sprintf("Memory index updated: %d files", index.Stats.TotalFiles)

	if categoryCount > 0 {
		msg += fmt.Sprintf(" (%s)", strings.Join(categoryParts, ", "))
	}

	msg += fmt.Sprintf(", %s total", formatSize(index.Stats.TotalSize))

	if index.Stats.CachedFiles > 0 || index.Stats.AnalyzedFiles > 0 {
		msg += fmt.Sprintf(" — %d cached, %d analyzed", index.Stats.CachedFiles, index.Stats.AnalyzedFiles)
	}

	return msg
}

func (f *Formatter) FormatMarkdown(index *types.Index) string {
	var sb strings.Builder

	sb.WriteString("# Claude Code Agentic Memory Index\n")
	sb.WriteString(fmt.Sprintf("📅 Generated: %s\n", index.Generated.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("📁 Files: %d | 💾 Total Size: %s\n", index.Stats.TotalFiles, formatSize(index.Stats.TotalSize)))
	sb.WriteString(fmt.Sprintf("📂 Root: %s\n\n", index.Root))

	if f.showRecentDays > 0 {
		recentEntries := f.getRecentEntries(index.Entries, f.showRecentDays)
		if len(recentEntries) > 0 {
			sb.WriteString(fmt.Sprintf("## 🕐 Recent Activity (Last %d Days)\n", f.showRecentDays))
			for _, entry := range recentEntries {
				relPath, _ := filepath.Rel(index.Root, entry.Metadata.Path)
				sb.WriteString(fmt.Sprintf("- %s: `%s`\n",
					entry.Metadata.Modified.Format("2006-01-02"),
					relPath))
			}
			sb.WriteString("\n---\n\n")
		}
	}

	categories := f.groupByCategory(index.Entries)

	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if entries, ok := categories[category]; ok && len(entries) > 0 {
			sb.WriteString(f.formatCategory(category, entries, index.Root))
		}
	}

	sb.WriteString(f.formatUsageGuide())

	return sb.String()
}

func (f *Formatter) FormatXML(index *types.Index) string {
	var sb strings.Builder

	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<memory_index>\n")

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

	if f.showRecentDays > 0 {
		recentEntries := f.getRecentEntries(index.Entries, f.showRecentDays)
		if len(recentEntries) > 0 {
			sb.WriteString(fmt.Sprintf("  <recent_activity days=\"%d\">\n", f.showRecentDays))
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

	categories := f.groupByCategory(index.Entries)
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
				sb.WriteString(f.formatXMLEntry(&entry))
			}

			sb.WriteString("    </category>\n")
		}
	}

	sb.WriteString("  </categories>\n\n")

	sb.WriteString("  <usage_guide>\n")
	sb.WriteString("    <direct_read_extensions>md, txt, json, yaml, vtt, go, py, js, ts, png, jpg</direct_read_extensions>\n")
	sb.WriteString("    <direct_read_tool>Read tool</direct_read_tool>\n")
	sb.WriteString("    <extraction_required_extensions>docx, pptx, pdf</extraction_required_extensions>\n")
	sb.WriteString("    <extraction_required_tool>Bash + conversion tools</extraction_required_tool>\n")
	sb.WriteString("  </usage_guide>\n")

	sb.WriteString("</memory_index>\n")

	return sb.String()
}

func (f *Formatter) formatXMLEntry(entry *types.IndexEntry) string {
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

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func (f *Formatter) WrapJSON(content string, index *types.Index) (string, error) {
	systemMsg := f.generateSystemMessage(index)

	output := HookOutput{
		Continue:       true,
		SuppressOutput: true,
		SystemMessage:  systemMsg,
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     "SessionStart",
			AdditionalContext: content,
		},
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON; %w", err)
	}

	return string(jsonBytes), nil
}

func (f *Formatter) formatCategory(category string, entries []types.IndexEntry, root string) string {
	var sb strings.Builder

	emoji := getCategoryEmoji(category)
	totalSize := int64(0)
	for _, entry := range entries {
		totalSize += entry.Metadata.Size
	}

	sb.WriteString(fmt.Sprintf("## %s %s (%d files, %s)\n\n",
		emoji,
		strings.Title(category),
		len(entries),
		formatSize(totalSize)))

	sort.Slice(entries, func(i, j int) bool {
		return filepath.Base(entries[i].Metadata.Path) < filepath.Base(entries[j].Metadata.Path)
	})

	for i, entry := range entries {
		if i > 0 {
			sb.WriteString("---\n\n")
		}
		sb.WriteString(f.formatEntry(&entry, root))
	}

	return sb.String()
}

func (f *Formatter) formatEntry(entry *types.IndexEntry, root string) string {
	var sb strings.Builder

	filename := filepath.Base(entry.Metadata.Path)

	sb.WriteString(fmt.Sprintf("### %s\n", filename))
	sb.WriteString(fmt.Sprintf("**Path**: `%s`  \n", entry.Metadata.Path))

	sb.WriteString(fmt.Sprintf("**Modified**: %s | **Size**: %s",
		entry.Metadata.Modified.Format("2006-01-02"),
		formatSize(entry.Metadata.Size)))

	if entry.Metadata.PageCount != nil {
		sb.WriteString(fmt.Sprintf(" | **Pages**: %d", *entry.Metadata.PageCount))
	}
	if entry.Metadata.SlideCount != nil {
		sb.WriteString(fmt.Sprintf(" | **Slides**: %d", *entry.Metadata.SlideCount))
	}
	if entry.Metadata.WordCount != nil && entry.Metadata.Category == "documents" {
		sb.WriteString(fmt.Sprintf(" | **Words**: %s", formatNumber(*entry.Metadata.WordCount)))
	}
	if entry.Metadata.WordCount != nil && entry.Metadata.Category == "code" {
		sb.WriteString(fmt.Sprintf(" | **Lines**: %s", formatNumber(*entry.Metadata.WordCount)))
	}
	if entry.Metadata.Dimensions != nil {
		sb.WriteString(fmt.Sprintf(" | **Dimensions**: %dx%d",
			entry.Metadata.Dimensions.Width,
			entry.Metadata.Dimensions.Height))
	}
	if entry.Metadata.Duration != nil {
		sb.WriteString(fmt.Sprintf(" | **Duration**: %s", *entry.Metadata.Duration))
	}
	sb.WriteString("  \n")

	typeDesc := strings.Title(entry.Metadata.Type)
	if entry.Metadata.Language != nil {
		typeDesc += fmt.Sprintf(" • %s", *entry.Metadata.Language)
	}
	sb.WriteString(fmt.Sprintf("**Type**: %s", typeDesc))

	if entry.Semantic != nil && entry.Semantic.DocumentType != "" {
		sb.WriteString(fmt.Sprintf(" • %s", strings.Title(entry.Semantic.DocumentType)))
	}
	sb.WriteString("\n\n")

	if entry.Semantic != nil {
		sb.WriteString(fmt.Sprintf("**Summary**: %s\n\n", entry.Semantic.Summary))

		if len(entry.Semantic.KeyTopics) > 0 {
			sb.WriteString(fmt.Sprintf("**Topics**: %s  \n", strings.Join(entry.Semantic.KeyTopics, ", ")))
		}

		if len(entry.Semantic.Tags) > 0 {
			tags := make([]string, len(entry.Semantic.Tags))
			for i, tag := range entry.Semantic.Tags {
				tags[i] = fmt.Sprintf("`%s`", tag)
			}
			sb.WriteString(fmt.Sprintf("**Tags**: %s  \n", strings.Join(tags, " ")))
		}

		// Sections for documents
		if len(entry.Metadata.Sections) > 0 {
			sb.WriteString(fmt.Sprintf("**Sections**: %s\n", strings.Join(entry.Metadata.Sections, " • ")))
		}

		sb.WriteString("\n")
	}

	if entry.Error != nil {
		sb.WriteString(fmt.Sprintf("⚠️ **Error**: %s\n\n", *entry.Error))
	}

	if entry.Metadata.IsReadable {
		sb.WriteString("✅ Directly readable with Read tool\n\n")
	} else {
		sb.WriteString("⚠️ Requires extraction (use Bash + conversion tools)\n\n")
	}

	return sb.String()
}

func (f *Formatter) formatUsageGuide() string {
	return `## Usage Guide

**Reading Files**:
- ✅ **Direct**: Markdown, text, VTT, JSON, YAML, images → Use Read tool
- ⚠️ **Extraction needed**: DOCX, PPTX, PDF → Use Bash + conversion tools

**When to access**: Ask me to read any file when relevant to your query. I'll use the appropriate method based on file type.

**Re-indexing**: Index auto-updates on session start. Manual re-index: run memorizer
`
}

func (f *Formatter) getRecentEntries(entries []types.IndexEntry, days int) []types.IndexEntry {
	cutoff := time.Now().AddDate(0, 0, -days)
	recent := []types.IndexEntry{}

	for _, entry := range entries {
		if entry.Metadata.Modified.After(cutoff) {
			recent = append(recent, entry)
		}
	}

	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Metadata.Modified.After(recent[j].Metadata.Modified)
	})

	if len(recent) > 10 {
		recent = recent[:10]
	}

	return recent
}

func (f *Formatter) groupByCategory(entries []types.IndexEntry) map[string][]types.IndexEntry {
	categories := make(map[string][]types.IndexEntry)

	for _, entry := range entries {
		category := entry.Metadata.Category
		categories[category] = append(categories[category], entry)
	}

	return categories
}

func getCategoryEmoji(category string) string {
	emojis := map[string]string{
		"documents":     "📄",
		"presentations": "🎤",
		"images":        "🖼️",
		"transcripts":   "🎬",
		"data":          "📊",
		"code":          "💻",
		"videos":        "🎥",
		"audio":         "🎵",
		"archives":      "📦",
		"other":         "📎",
	}

	if emoji, ok := emojis[category]; ok {
		return emoji
	}
	return "📎"
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result string
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}
