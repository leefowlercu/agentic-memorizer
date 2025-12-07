package formatters

import (
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
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

func init() {
	// Register Markdown formatter
	format.RegisterFormatter("markdown", NewMarkdownFormatter())
}
