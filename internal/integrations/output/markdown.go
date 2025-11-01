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

// Format renders the index as Markdown
func (p *MarkdownProcessor) Format(index *types.Index) (string, error) {
	var sb strings.Builder

	sb.WriteString("# Claude Code Agentic Memory Index\n")
	sb.WriteString(fmt.Sprintf("📅 Generated: %s\n", index.Generated.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("📁 Files: %d | 💾 Total Size: %s\n", index.Stats.TotalFiles, formatSize(index.Stats.TotalSize)))
	sb.WriteString(fmt.Sprintf("📂 Root: %s\n\n", index.Root))

	// Recent activity section (if enabled)
	if p.options.ShowRecentDays > 0 {
		recentEntries := getRecentEntries(index.Entries, p.options.ShowRecentDays)
		if len(recentEntries) > 0 {
			sb.WriteString(fmt.Sprintf("## 🕐 Recent Activity (Last %d Days)\n", p.options.ShowRecentDays))
			for _, entry := range recentEntries {
				relPath, _ := filepath.Rel(index.Root, entry.Metadata.Path)
				sb.WriteString(fmt.Sprintf("- %s: `%s`\n",
					entry.Metadata.Modified.Format("2006-01-02"),
					relPath))
			}
			sb.WriteString("\n---\n\n")
		}
	}

	// Categories
	categories := groupByCategory(index.Entries)

	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if entries, ok := categories[category]; ok && len(entries) > 0 {
			sb.WriteString(p.formatCategory(category, entries, index.Root))
		}
	}

	// Usage guide
	sb.WriteString(p.formatUsageGuide())

	return sb.String(), nil
}

// formatCategory formats a category section
func (p *MarkdownProcessor) formatCategory(category string, entries []types.IndexEntry, root string) string {
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
		sb.WriteString(p.formatEntry(&entry, root))
	}

	return sb.String()
}

// formatEntry formats a single index entry
func (p *MarkdownProcessor) formatEntry(entry *types.IndexEntry, root string) string {
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
