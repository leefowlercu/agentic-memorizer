package formatters

import (
	"fmt"
	"sort"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// MarkdownFormatter renders output as Markdown
type MarkdownFormatter struct{}

// NewMarkdownFormatter creates a new Markdown formatter
func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

// Format renders a single buildable structure
func (f *MarkdownFormatter) Format(b format.Buildable) (string, error) {
	if err := b.Validate(); err != nil {
		return "", fmt.Errorf("validation failed; %w", err)
	}

	switch v := b.(type) {
	case *format.Section:
		return f.formatSection(v), nil
	case *format.Table:
		return f.formatTable(v), nil
	case *format.List:
		return f.formatList(v, 0), nil
	case *format.Progress:
		return f.formatProgress(v), nil
	case *format.Status:
		return f.formatStatus(v), nil
	case *format.Error:
		return f.formatError(v), nil
	case *format.FilesContent:
		// FilesContent uses specialized markdown formatting
		return f.formatFiles(v)
	case *format.FactsContent:
		// FactsContent uses specialized markdown formatting
		return f.formatFacts(v)
	default:
		return "", fmt.Errorf("unsupported builder type: %s", b.Type())
	}
}

// FormatMultiple renders multiple buildable structures
func (f *MarkdownFormatter) FormatMultiple(builders []format.Buildable) (string, error) {
	var parts []string
	for _, b := range builders {
		formatted, err := f.Format(b)
		if err != nil {
			return "", err
		}
		parts = append(parts, formatted)
	}
	return strings.Join(parts, "\n\n---\n\n"), nil
}

// Name returns the formatter name
func (f *MarkdownFormatter) Name() string {
	return "markdown"
}

// SupportsColors returns false (Markdown doesn't support colors)
func (f *MarkdownFormatter) SupportsColors() bool {
	return false
}

// formatSection renders a section as Markdown
func (f *MarkdownFormatter) formatSection(s *format.Section) string {
	var sb strings.Builder

	// Render title with appropriate heading level (# for level 0, ## for level 1, etc.)
	if s.Title != "" {
		headingLevel := s.Level + 1
		if headingLevel > 6 {
			headingLevel = 6
		}
		sb.WriteString(strings.Repeat("#", headingLevel))
		sb.WriteString(" ")
		sb.WriteString(s.Title)
		sb.WriteString("\n\n")
	}

	// Render items
	for i, item := range s.Items {
		if item.Type == format.SectionItemKeyValue {
			sb.WriteString("**")
			sb.WriteString(item.Key)
			sb.WriteString("**: ")
			sb.WriteString(item.Value)
			sb.WriteString("\n")
		} else if item.Type == format.SectionItemSubsection {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(f.formatSection(item.Subsection))
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// formatTable renders a table as Markdown
func (f *MarkdownFormatter) formatTable(t *format.Table) string {
	if len(t.Rows) == 0 {
		return ""
	}

	var sb strings.Builder

	// Render headers (unless hidden)
	if !t.HideHeaders {
		sb.WriteString("| ")
		sb.WriteString(strings.Join(t.Headers, " | "))
		sb.WriteString(" |\n")

		// Render separator line with alignment
		sb.WriteString("|")
		for i := range t.Headers {
			align := format.AlignLeft
			if i < len(t.Alignments) {
				align = t.Alignments[i]
			}

			switch align {
			case format.AlignLeft:
				sb.WriteString(" :--- |")
			case format.AlignRight:
				sb.WriteString(" ---: |")
			case format.AlignCenter:
				sb.WriteString(" :---: |")
			default:
				sb.WriteString(" --- |")
			}
		}
		sb.WriteString("\n")
	}

	// Render rows
	for _, row := range t.Rows {
		sb.WriteString("| ")
		sb.WriteString(strings.Join(row, " | "))
		sb.WriteString(" |\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// formatList renders a list as Markdown
func (f *MarkdownFormatter) formatList(l *format.List, depth int) string {
	var sb strings.Builder

	indent := strings.Repeat("  ", depth)

	for i, item := range l.Items {
		sb.WriteString(indent)

		// Add bullet or number
		if l.ListType == format.ListTypeUnordered {
			sb.WriteString("- ")
		} else {
			sb.WriteString(fmt.Sprintf("%d. ", i+1))
		}

		sb.WriteString(item.Content)
		sb.WriteString("\n")

		// Render nested list if present
		if item.Nested != nil {
			nested := f.formatList(item.Nested, depth+1)
			sb.WriteString(nested)
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// formatProgress renders a progress indicator as Markdown
func (f *MarkdownFormatter) formatProgress(p *format.Progress) string {
	var sb strings.Builder

	switch p.ProgressType {
	case format.ProgressTypeBar:
		percentage := p.Percentage()
		filled := int(percentage / 10)
		empty := 10 - filled

		sb.WriteString("`[")
		sb.WriteString(strings.Repeat("█", filled))
		sb.WriteString(strings.Repeat("░", empty))
		sb.WriteString("]`")

		if p.Message != "" {
			sb.WriteString(" ")
			sb.WriteString(p.Message)
		}

		sb.WriteString(fmt.Sprintf(" **%.1f%%**", percentage))

	case format.ProgressTypePercentage:
		if p.Message != "" {
			sb.WriteString(p.Message)
			sb.WriteString(": ")
		}
		sb.WriteString(fmt.Sprintf("**%.1f%%**", p.Percentage()))

	case format.ProgressTypeSpinner:
		if p.Message != "" {
			sb.WriteString("*")
			sb.WriteString(p.Message)
			sb.WriteString("...*")
		} else {
			sb.WriteString("*Processing...*")
		}
	}

	return sb.String()
}

// formatStatus renders a status message as Markdown
func (f *MarkdownFormatter) formatStatus(s *format.Status) string {
	var sb strings.Builder

	// Get symbol
	symbol := s.CustomSymbol
	if symbol == "" {
		symbol = format.GetStatusSymbol(s.Severity)
	}

	// Format status line
	sb.WriteString(symbol)
	sb.WriteString(" **")
	sb.WriteString(s.Message)
	sb.WriteString("**")

	// Add details as nested list
	if len(s.Details) > 0 {
		sb.WriteString("\n")
		for _, detail := range s.Details {
			sb.WriteString("  - ")
			sb.WriteString(detail)
			sb.WriteString("\n")
		}
		// Trim final newline
		return strings.TrimSuffix(sb.String(), "\n")
	}

	return sb.String()
}

// formatError renders an error message as Markdown
func (f *MarkdownFormatter) formatError(e *format.Error) string {
	var sb strings.Builder

	// Error header
	sb.WriteString("**Error**: ")
	sb.WriteString(e.Message)
	sb.WriteString("\n\n")

	// Field and value (if present)
	if e.Field != "" {
		sb.WriteString("- **Field**: `")
		sb.WriteString(e.Field)
		sb.WriteString("`\n")

		if e.Value != nil {
			sb.WriteString("- **Value**: `")
			sb.WriteString(fmt.Sprintf("%v", e.Value))
			sb.WriteString("`\n")
		}
	}

	// Details
	if len(e.Details) > 0 {
		if e.Field != "" {
			sb.WriteString("\n")
		}
		sb.WriteString("**Details**:\n")
		for _, detail := range e.Details {
			sb.WriteString("- ")
			sb.WriteString(detail)
			sb.WriteString("\n")
		}
	}

	// Suggestion
	if e.Suggestion != "" {
		if len(e.Details) > 0 || e.Field != "" {
			sb.WriteString("\n")
		}
		sb.WriteString("**Suggestion**: ")
		sb.WriteString(e.Suggestion)
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// formatFiles renders FileIndex as Markdown (backward compatibility with legacy output)
func (f *MarkdownFormatter) formatFiles(fc *format.FilesContent) (string, error) {
	index := fc.Index
	var sb strings.Builder

	sb.WriteString("# Claude Code Agentic Memory Index\n")
	sb.WriteString(fmt.Sprintf("📅 Generated: %s\n", index.Generated.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("📁 Files: %d | 💾 Total Size: %s\n", index.Stats.TotalFiles, format.FormatBytes(index.Stats.TotalSize)))
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

	// Categories
	categories := groupFilesByCategory(index.Files)

	categoryOrder := []string{"documents", "presentations", "images", "transcripts", "data", "code", "videos", "audio", "archives", "other"}
	for _, category := range categoryOrder {
		if files, ok := categories[category]; ok && len(files) > 0 {
			sb.WriteString(formatFileCategoryMarkdown(category, files))
		}
	}

	// Usage guide
	if index.UsageGuide != nil {
		sb.WriteString(formatUsageGuideMarkdown(index.UsageGuide))
	}

	return sb.String(), nil
}

// formatFileCategoryMarkdown formats a category section for FileEntry
func formatFileCategoryMarkdown(category string, files []types.FileEntry) string {
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
		format.FormatBytes(totalSize)))

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	for i, file := range files {
		if i > 0 {
			sb.WriteString("---\n\n")
		}
		sb.WriteString(formatFileEntryMarkdown(&file))
	}

	return sb.String()
}

// formatFileEntryMarkdown formats a single FileEntry as Markdown
func formatFileEntryMarkdown(file *types.FileEntry) string {
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
		sb.WriteString(fmt.Sprintf(" | **Words**: %s", format.FormatNumber(int64(*file.WordCount))))
	}
	if file.WordCount != nil && file.Category == "code" {
		sb.WriteString(fmt.Sprintf(" | **Lines**: %s", format.FormatNumber(int64(*file.WordCount))))
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

	// Semantic understanding
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

// formatUsageGuideMarkdown returns the usage guide section
func formatUsageGuideMarkdown(guide *types.UsageGuide) string {
	var sb strings.Builder

	sb.WriteString("## Usage Guide\n\n")
	sb.WriteString(fmt.Sprintf("**About**: %s\n\n", guide.Description))
	sb.WriteString(fmt.Sprintf("**When to use**: %s\n\n", guide.WhenToUse))
	sb.WriteString("**Reading Files**:\n")
	sb.WriteString(fmt.Sprintf("- ✅ **Direct**: %s\n", guide.DirectReadable))
	sb.WriteString(fmt.Sprintf("- ⚠️ **Extraction needed**: %s\n", guide.ExtractionRequired))

	return sb.String()
}

// formatFacts renders FactsIndex as Markdown
func (f *MarkdownFormatter) formatFacts(fc *format.FactsContent) (string, error) {
	index := fc.Index
	var sb strings.Builder

	sb.WriteString("# Facts\n\n")
	sb.WriteString(fmt.Sprintf("**Generated**: %s\n", index.Generated.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Total Facts**: %d / %d\n\n", index.Stats.TotalFacts, index.Stats.MaxFacts))

	if len(index.Facts) == 0 {
		sb.WriteString("*No facts stored.*\n")
		return sb.String(), nil
	}

	sb.WriteString("---\n\n")

	for i, fact := range index.Facts {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", fact.ID))
		sb.WriteString(fmt.Sprintf("%s\n\n", fact.Content))
		sb.WriteString(fmt.Sprintf("*Created*: %s", fact.CreatedAt.Format("2006-01-02 15:04:05")))
		if !fact.UpdatedAt.IsZero() {
			sb.WriteString(fmt.Sprintf(" | *Updated*: %s", fact.UpdatedAt.Format("2006-01-02 15:04:05")))
		}
		sb.WriteString(fmt.Sprintf(" | *Source*: %s\n", fact.Source))
	}

	return sb.String(), nil
}

func init() {
	// Register Markdown formatter
	format.RegisterFormatter("markdown", NewMarkdownFormatter())
}
