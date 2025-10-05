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

// Formatter generates output in various formats
type Formatter struct {
	verbose        bool
	showRecentDays int
}

// HookOutput represents the JSON output structure for Claude Code hooks
type HookOutput struct {
	Continue           bool                `json:"continue"`
	StopReason         *string             `json:"stopReason,omitempty"`
	SuppressOutput     bool                `json:"suppressOutput"`
	SystemMessage      string              `json:"systemMessage,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput contains SessionStart hook-specific fields
type HookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// NewFormatter creates a new output formatter
func NewFormatter(verbose bool, showRecentDays int) *Formatter {
	return &Formatter{
		verbose:        verbose,
		showRecentDays: showRecentDays,
	}
}

// generateSystemMessage creates a concise summary of the memory index
func (f *Formatter) generateSystemMessage(index *types.Index) string {
	// Group by category to count categories with files
	categories := f.groupByCategory(index.Entries)
	categoryCount := len(categories)

	// Build category breakdown
	categoryParts := []string{}
	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if entries, ok := categories[category]; ok && len(entries) > 0 {
			categoryParts = append(categoryParts, fmt.Sprintf("%d %s", len(entries), category))
		}
	}

	// Format message
	msg := fmt.Sprintf("Memory index updated: %d files", index.Stats.TotalFiles)

	if categoryCount > 0 {
		msg += fmt.Sprintf(" (%s)", strings.Join(categoryParts, ", "))
	}

	msg += fmt.Sprintf(", %s total", formatSize(index.Stats.TotalSize))

	// Add cache stats if semantic analysis was performed
	if index.Stats.CachedFiles > 0 || index.Stats.AnalyzedFiles > 0 {
		msg += fmt.Sprintf(" — %d cached, %d analyzed", index.Stats.CachedFiles, index.Stats.AnalyzedFiles)
	}

	return msg
}

// FormatMarkdown formats the index as markdown
func (f *Formatter) FormatMarkdown(index *types.Index) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Claude Code Agentic Memory Index\n")
	sb.WriteString(fmt.Sprintf("📅 Generated: %s\n", index.Generated.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("📁 Files: %d | 💾 Total Size: %s\n", index.Stats.TotalFiles, formatSize(index.Stats.TotalSize)))
	sb.WriteString(fmt.Sprintf("📂 Root: %s\n\n", index.Root))

	// Recent activity
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

	// Group by category
	categories := f.groupByCategory(index.Entries)

	// Sort categories
	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if entries, ok := categories[category]; ok && len(entries) > 0 {
			sb.WriteString(f.formatCategory(category, entries, index.Root))
		}
	}

	// Usage guide
	sb.WriteString(f.formatUsageGuide())

	return sb.String()
}

// FormatJSON formats the index as JSON for Claude Code hook output
func (f *Formatter) FormatJSON(index *types.Index) (string, error) {
	// Generate system message
	systemMsg := f.generateSystemMessage(index)

	// Generate markdown for additional context
	markdown := f.FormatMarkdown(index)

	// Build hook output
	output := HookOutput{
		Continue:       true,
		SuppressOutput: true,
		SystemMessage:  systemMsg,
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     "SessionStart",
			AdditionalContext: markdown,
		},
	}

	// Marshal to JSON with indentation for readability
	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// formatCategory formats a category section
func (f *Formatter) formatCategory(category string, entries []types.IndexEntry, root string) string {
	var sb strings.Builder

	// Category header with emoji
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

	// Sort entries by name
	sort.Slice(entries, func(i, j int) bool {
		return filepath.Base(entries[i].Metadata.Path) < filepath.Base(entries[j].Metadata.Path)
	})

	// Format each entry
	for i, entry := range entries {
		if i > 0 {
			sb.WriteString("---\n\n")
		}
		sb.WriteString(f.formatEntry(&entry, root))
	}

	return sb.String()
}

// formatEntry formats a single file entry
func (f *Formatter) formatEntry(entry *types.IndexEntry, root string) string {
	var sb strings.Builder

	filename := filepath.Base(entry.Metadata.Path)

	// Filename header
	sb.WriteString(fmt.Sprintf("### %s\n", filename))
	sb.WriteString(fmt.Sprintf("**Path**: `%s`  \n", entry.Metadata.Path))

	// Metadata line
	sb.WriteString(fmt.Sprintf("**Modified**: %s | **Size**: %s",
		entry.Metadata.Modified.Format("2006-01-02"),
		formatSize(entry.Metadata.Size)))

	// Type-specific metadata
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

	// Type
	typeDesc := strings.Title(entry.Metadata.Type)
	if entry.Metadata.Language != nil {
		typeDesc += fmt.Sprintf(" • %s", *entry.Metadata.Language)
	}
	sb.WriteString(fmt.Sprintf("**Type**: %s", typeDesc))

	// Document type from semantic analysis
	if entry.Semantic != nil && entry.Semantic.DocumentType != "" {
		sb.WriteString(fmt.Sprintf(" • %s", strings.Title(entry.Semantic.DocumentType)))
	}
	sb.WriteString("\n\n")

	// Semantic analysis
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

	// Error if present
	if entry.Error != nil {
		sb.WriteString(fmt.Sprintf("⚠️ **Error**: %s\n\n", *entry.Error))
	}

	// Readability indicator
	if entry.Metadata.IsReadable {
		sb.WriteString("✅ Directly readable with Read tool\n\n")
	} else {
		sb.WriteString("⚠️ Requires extraction (use Bash + conversion tools)\n\n")
	}

	return sb.String()
}

// formatUsageGuide formats the usage guide
func (f *Formatter) formatUsageGuide() string {
	return `## Usage Guide

**Reading Files**:
- ✅ **Direct**: Markdown, text, VTT, JSON, YAML, images → Use Read tool
- ⚠️ **Extraction needed**: DOCX, PPTX, PDF → Use Bash + conversion tools

**When to access**: Ask me to read any file when relevant to your query. I'll use the appropriate method based on file type.

**Re-indexing**: Index auto-updates on session start. Manual re-index: run memorizer
`
}

// getRecentEntries returns entries modified within the last N days
func (f *Formatter) getRecentEntries(entries []types.IndexEntry, days int) []types.IndexEntry {
	cutoff := time.Now().AddDate(0, 0, -days)
	recent := []types.IndexEntry{}

	for _, entry := range entries {
		if entry.Metadata.Modified.After(cutoff) {
			recent = append(recent, entry)
		}
	}

	// Sort by modified date (newest first)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Metadata.Modified.After(recent[j].Metadata.Modified)
	})

	// Limit to 10
	if len(recent) > 10 {
		recent = recent[:10]
	}

	return recent
}

// groupByCategory groups entries by category
func (f *Formatter) groupByCategory(entries []types.IndexEntry) map[string][]types.IndexEntry {
	categories := make(map[string][]types.IndexEntry)

	for _, entry := range entries {
		category := entry.Metadata.Category
		categories[category] = append(categories[category], entry)
	}

	return categories
}

// getCategoryEmoji returns an emoji for a category
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

// formatSize formats a size in bytes to human-readable format
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

// formatNumber formats a number with commas
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	// Add commas
	var result string
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}
