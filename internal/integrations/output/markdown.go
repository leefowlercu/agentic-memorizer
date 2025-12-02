package output

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// MarkdownProcessor formats the memory index as Markdown
type MarkdownProcessor struct {
	options Options
}

// NewMarkdownProcessor creates a new Markdown output processor
func NewMarkdownProcessor(opts ...Options) *MarkdownProcessor {
	options := DefaultOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	return &MarkdownProcessor{
		options: options,
	}
}

// GetFormat returns the format name
func (p *MarkdownProcessor) GetFormat() string {
	return "markdown"
}

// formatUsageGuide returns the usage guide section
func (p *MarkdownProcessor) formatUsageGuide() string {
	return `## Usage Guide

**Reading Files**:
- ✅ **Direct**: Markdown, text, VTT, JSON, YAML, images → Use Read tool
- ⚠️ **Extraction needed**: DOCX, PPTX, PDF → Use Bash + conversion tools

**When to access**: Ask me to read any file when relevant to your query. I'll use the appropriate method based on file type.

**Re-indexing**: Index auto-updates on session start. Manual re-index: run memorizer
`
}

// getCategoryEmoji returns the emoji for a given category
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

// formatNumber formats a number with thousands separators
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

// FormatGraph renders the graph index as Markdown (new flattened format)
func (p *MarkdownProcessor) FormatGraph(index *types.GraphIndex) (string, error) {
	var sb strings.Builder

	sb.WriteString("# Claude Code Agentic Memory Index\n")
	sb.WriteString(fmt.Sprintf("📅 Generated: %s\n", index.Generated.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("📁 Files: %d | 💾 Total Size: %s\n", index.Stats.TotalFiles, formatSize(index.Stats.TotalSize)))
	sb.WriteString(fmt.Sprintf("📂 Root: %s\n", index.MemoryRoot))

	// Graph statistics (if available)
	if index.Stats.TotalTags > 0 || index.Stats.TotalTopics > 0 || index.Stats.TotalEntities > 0 {
		sb.WriteString(fmt.Sprintf("🏷️ Tags: %d | 📚 Topics: %d | 🔖 Entities: %d | 🔗 Edges: %d\n",
			index.Stats.TotalTags, index.Stats.TotalTopics, index.Stats.TotalEntities, index.Stats.TotalEdges))
	}
	sb.WriteString("\n")

	// Knowledge summary (if available)
	if index.Knowledge != nil && (len(index.Knowledge.TopTags) > 0 || len(index.Knowledge.TopTopics) > 0 || len(index.Knowledge.TopEntities) > 0) {
		sb.WriteString("## 🧠 Knowledge Landscape\n\n")

		if len(index.Knowledge.TopTags) > 0 {
			sb.WriteString("**Top Tags**: ")
			tagStrs := make([]string, len(index.Knowledge.TopTags))
			for i, tag := range index.Knowledge.TopTags {
				tagStrs[i] = fmt.Sprintf("`%s` (%d)", tag.Name, tag.Count)
			}
			sb.WriteString(strings.Join(tagStrs, ", "))
			sb.WriteString("  \n")
		}

		if len(index.Knowledge.TopTopics) > 0 {
			sb.WriteString("**Top Topics**: ")
			topicStrs := make([]string, len(index.Knowledge.TopTopics))
			for i, topic := range index.Knowledge.TopTopics {
				topicStrs[i] = fmt.Sprintf("%s (%d)", topic.Name, topic.Count)
			}
			sb.WriteString(strings.Join(topicStrs, ", "))
			sb.WriteString("  \n")
		}

		if len(index.Knowledge.TopEntities) > 0 {
			sb.WriteString("**Top Entities**: ")
			entityStrs := make([]string, len(index.Knowledge.TopEntities))
			for i, entity := range index.Knowledge.TopEntities {
				entityStrs[i] = fmt.Sprintf("%s [%s] (%d)", entity.Name, entity.Type, entity.Count)
			}
			sb.WriteString(strings.Join(entityStrs, ", "))
			sb.WriteString("  \n")
		}

		sb.WriteString("\n---\n\n")
	}

	// Insights section (verbose mode only)
	if index.Insights != nil {
		sb.WriteString("## 🔍 Graph Insights\n\n")

		// Top connected files
		if len(index.Insights.Recommendations) > 0 {
			sb.WriteString("### Most Connected Files\n")
			for _, rec := range index.Insights.Recommendations {
				sb.WriteString(fmt.Sprintf("- **%s** (%.0f connections) - %s\n",
					rec.TargetName, rec.Score, rec.Reason))
			}
			sb.WriteString("\n")
		}

		// Topic clusters
		if len(index.Insights.TopicClusters) > 0 {
			sb.WriteString("### Topic Clusters\n")
			for _, cluster := range index.Insights.TopicClusters {
				sb.WriteString(fmt.Sprintf("- **%s** (%d files)", cluster.Name, cluster.FileCount))
				if len(cluster.CommonTags) > 0 {
					sb.WriteString(fmt.Sprintf(" - Tags: `%s`", strings.Join(cluster.CommonTags, "`, `")))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}

		// Coverage gaps
		if len(index.Insights.CoverageGaps) > 0 {
			sb.WriteString("### Coverage Gaps\n")
			for _, gap := range index.Insights.CoverageGaps {
				severityEmoji := "⚠️"
				if gap.Severity == "high" {
					severityEmoji = "🔴"
				} else if gap.Severity == "low" {
					severityEmoji = "🟡"
				}
				sb.WriteString(fmt.Sprintf("- %s **[%s]** %s: %s\n",
					severityEmoji, gap.Type, gap.Name, gap.Description))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	// Recent activity section (if enabled)
	if p.options.ShowRecentDays > 0 {
		recentFiles := getRecentFileEntries(index.Files, p.options.ShowRecentDays)
		if len(recentFiles) > 0 {
			sb.WriteString(fmt.Sprintf("## 🕐 Recent Activity (Last %d Days)\n", p.options.ShowRecentDays))
			for _, file := range recentFiles {
				relPath, _ := filepath.Rel(index.MemoryRoot, file.Path)
				sb.WriteString(fmt.Sprintf("- %s: `%s`\n",
					file.Modified.Format("2006-01-02"),
					relPath))
			}
			sb.WriteString("\n---\n\n")
		}
	}

	// Categories
	categories := groupFilesByCategory(index.Files)

	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if files, ok := categories[category]; ok && len(files) > 0 {
			sb.WriteString(p.formatFileCategory(category, files, index.MemoryRoot))
		}
	}

	// Usage guide
	sb.WriteString(p.formatUsageGuide())

	return sb.String(), nil
}

// formatFileCategory formats a category section for FileEntry
func (p *MarkdownProcessor) formatFileCategory(category string, files []types.FileEntry, root string) string {
	var sb strings.Builder

	emoji := getCategoryEmoji(category)
	totalSize := int64(0)
	for _, file := range files {
		totalSize += file.Size
	}

	sb.WriteString(fmt.Sprintf("## %s %s (%d files, %s)\n\n",
		emoji,
		strings.Title(category),
		len(files),
		formatSize(totalSize)))

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	for i, file := range files {
		if i > 0 {
			sb.WriteString("---\n\n")
		}
		sb.WriteString(p.formatFileEntryMarkdown(&file, root))
	}

	return sb.String()
}

// formatFileEntryMarkdown formats a single FileEntry as Markdown
func (p *MarkdownProcessor) formatFileEntryMarkdown(file *types.FileEntry, root string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("### %s\n", file.Name))
	sb.WriteString(fmt.Sprintf("**Path**: `%s`  \n", file.Path))

	sb.WriteString(fmt.Sprintf("**Modified**: %s | **Size**: %s",
		file.Modified.Format("2006-01-02"),
		file.SizeHuman))

	if file.PageCount != nil {
		sb.WriteString(fmt.Sprintf(" | **Pages**: %d", *file.PageCount))
	}
	if file.SlideCount != nil {
		sb.WriteString(fmt.Sprintf(" | **Slides**: %d", *file.SlideCount))
	}
	if file.WordCount != nil && file.Category == "documents" {
		sb.WriteString(fmt.Sprintf(" | **Words**: %s", formatNumber(*file.WordCount)))
	}
	if file.WordCount != nil && file.Category == "code" {
		sb.WriteString(fmt.Sprintf(" | **Lines**: %s", formatNumber(*file.WordCount)))
	}
	if file.Dimensions != nil {
		sb.WriteString(fmt.Sprintf(" | **Dimensions**: %dx%d",
			file.Dimensions.Width,
			file.Dimensions.Height))
	}
	if file.Duration != nil {
		sb.WriteString(fmt.Sprintf(" | **Duration**: %s", *file.Duration))
	}
	sb.WriteString("  \n")

	typeDesc := strings.Title(file.Type)
	if file.Language != nil {
		typeDesc += fmt.Sprintf(" • %s", *file.Language)
	}
	sb.WriteString(fmt.Sprintf("**Type**: %s", typeDesc))

	if file.DocumentType != "" {
		sb.WriteString(fmt.Sprintf(" • %s", strings.Title(file.DocumentType)))
	}
	sb.WriteString("\n\n")

	// Semantic understanding (flattened in new format)
	if file.Summary != "" {
		sb.WriteString(fmt.Sprintf("**Summary**: %s\n\n", file.Summary))
	}

	if len(file.Topics) > 0 {
		sb.WriteString(fmt.Sprintf("**Topics**: %s  \n", strings.Join(file.Topics, ", ")))
	}

	if len(file.Tags) > 0 {
		tags := make([]string, len(file.Tags))
		for i, tag := range file.Tags {
			tags[i] = fmt.Sprintf("`%s`", tag)
		}
		sb.WriteString(fmt.Sprintf("**Tags**: %s  \n", strings.Join(tags, " ")))
	}

	if len(file.Entities) > 0 {
		entities := make([]string, len(file.Entities))
		for i, entity := range file.Entities {
			entities[i] = fmt.Sprintf("%s (%s)", entity.Name, entity.Type)
		}
		sb.WriteString(fmt.Sprintf("**Entities**: %s  \n", strings.Join(entities, ", ")))
	}

	// Related files (verbose mode only)
	if len(file.RelatedFiles) > 0 {
		sb.WriteString("**Related Files**:  \n")
		for _, related := range file.RelatedFiles {
			sb.WriteString(fmt.Sprintf("- `%s` via %s (%s)  \n",
				related.Name, related.Via, strings.Join(related.Shared, ", ")))
		}
	}

	sb.WriteString("\n")

	if file.Error != nil {
		sb.WriteString(fmt.Sprintf("⚠️ **Error**: %s\n\n", *file.Error))
	}

	if file.IsReadable {
		sb.WriteString("✅ Directly readable with Read tool\n\n")
	} else {
		sb.WriteString("⚠️ Requires extraction (use Bash + conversion tools)\n\n")
	}

	return sb.String()
}
