package formatters

import (
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
)

// TextFormatter renders output as plain text with box-drawing characters
type TextFormatter struct {
	useColors bool
}

// NewTextFormatter creates a new text formatter
func NewTextFormatter(useColors bool) *TextFormatter {
	return &TextFormatter{
		useColors: useColors,
	}
}

// Format renders a single buildable structure
func (f *TextFormatter) Format(b format.Buildable) (string, error) {
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
func (f *TextFormatter) FormatMultiple(builders []format.Buildable) (string, error) {
	var parts []string
	for _, b := range builders {
		formatted, err := f.Format(b)
		if err != nil {
			return "", err
		}
		parts = append(parts, formatted)
	}
	return strings.Join(parts, "\n\n"), nil
}

// Name returns the formatter name
func (f *TextFormatter) Name() string {
	return "text"
}

// SupportsColors returns true if the formatter supports color output
func (f *TextFormatter) SupportsColors() bool {
	return f.useColors
}

// formatSection renders a section
func (f *TextFormatter) formatSection(s *format.Section) string {
	var sb strings.Builder

	// Render title
	if s.Title != "" {
		if f.useColors {
			sb.WriteString(format.Bold(s.Title))
		} else {
			sb.WriteString(s.Title)
		}
		sb.WriteString("\n")

		// Add divider if requested
		if s.WithDivider {
			dividerChar := '='
			if s.Level > 0 {
				dividerChar = '-'
			}
			sb.WriteString(strings.Repeat(string(dividerChar), len(s.Title)))
			sb.WriteString("\n")
		}
	}

	// Render items
	if len(s.Items) > 0 {
		// Calculate key width for alignment
		maxKeyWidth := 0
		for _, item := range s.Items {
			if item.Type == format.SectionItemKeyValue && len(item.Key) > maxKeyWidth {
				maxKeyWidth = len(item.Key)
			}
		}

		for i, item := range s.Items {
			if item.Type == format.SectionItemKeyValue {
				// Format: "Key:    Value"
				padding := maxKeyWidth - len(item.Key)
				sb.WriteString(item.Key)
				sb.WriteString(":")
				sb.WriteString(strings.Repeat(" ", padding+1))
				sb.WriteString(item.Value)
				sb.WriteString("\n")
			} else if item.Type == format.SectionItemSubsection {
				// Add spacing before subsection
				if i > 0 {
					sb.WriteString("\n")
				}
				// Render subsection with indentation
				subsectionText := f.formatSection(item.Subsection)
				lines := strings.Split(subsectionText, "\n")
				indent := strings.Repeat("  ", s.Level+1)
				for _, line := range lines {
					if line != "" {
						sb.WriteString(indent)
						sb.WriteString(line)
						sb.WriteString("\n")
					}
				}
			}
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// formatTable renders a table
func (f *TextFormatter) formatTable(t *format.Table) string {
	if len(t.Rows) == 0 {
		return ""
	}

	var sb strings.Builder

	// Calculate column widths
	colWidths := make([]int, len(t.Headers))
	for i, header := range t.Headers {
		colWidths[i] = len(format.StripANSI(header))
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			cellLen := len(format.StripANSI(cell))
			if cellLen > colWidths[i] {
				colWidths[i] = cellLen
			}
		}
	}

	spacing := 2
	if t.IsCompact {
		spacing = 1
	}

	// Render headers (unless hidden)
	if !t.HideHeaders {
		for i, header := range t.Headers {
			if i > 0 {
				sb.WriteString(strings.Repeat(" ", spacing))
			}
			align := format.AlignLeft
			if i < len(t.Alignments) {
				align = t.Alignments[i]
			}
			sb.WriteString(format.AlignText(header, colWidths[i], align))
		}
		sb.WriteString("\n")

		// Render separator line
		for i := range t.Headers {
			if i > 0 {
				sb.WriteString(strings.Repeat(" ", spacing))
			}
			sb.WriteString(strings.Repeat("-", colWidths[i]))
		}
		sb.WriteString("\n")
	}

	// Render rows
	for _, row := range t.Rows {
		for i, cell := range row {
			if i > 0 {
				sb.WriteString(strings.Repeat(" ", spacing))
			}
			align := format.AlignLeft
			if i < len(t.Alignments) {
				align = t.Alignments[i]
			}
			sb.WriteString(format.AlignText(cell, colWidths[i], align))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// formatList renders a list
func (f *TextFormatter) formatList(l *format.List, depth int) string {
	var sb strings.Builder

	indent := strings.Repeat("  ", depth)
	spacing := "\n"
	if l.IsCompact {
		spacing = ""
	}

	for i, item := range l.Items {
		if i > 0 && !l.IsCompact {
			sb.WriteString(spacing)
		}

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
			if i < len(l.Items)-1 {
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// formatProgress renders a progress indicator
func (f *TextFormatter) formatProgress(p *format.Progress) string {
	var sb strings.Builder

	switch p.ProgressType {
	case format.ProgressTypeBar:
		percentage := p.Percentage()
		filled := int(float64(p.BarWidth) * percentage / 100.0)
		empty := p.BarWidth - filled

		sb.WriteString("[")
		sb.WriteString(strings.Repeat("=", filled))
		if filled < p.BarWidth {
			sb.WriteString(">")
			empty--
		}
		sb.WriteString(strings.Repeat(" ", empty))
		sb.WriteString("]")

		if p.Message != "" {
			sb.WriteString(" ")
			sb.WriteString(p.Message)
		}

		sb.WriteString(fmt.Sprintf(" %.1f%%", percentage))

	case format.ProgressTypePercentage:
		if p.Message != "" {
			sb.WriteString(p.Message)
			sb.WriteString(": ")
		}
		sb.WriteString(fmt.Sprintf("%.1f%%", p.Percentage()))

	case format.ProgressTypeSpinner:
		// Simple spinner representation (static in text)
		if p.Message != "" {
			sb.WriteString(p.Message)
		} else {
			sb.WriteString("Processing...")
		}
	}

	return sb.String()
}

// formatStatus renders a status message
func (f *TextFormatter) formatStatus(s *format.Status) string {
	var sb strings.Builder

	// Get symbol
	symbol := s.CustomSymbol
	if symbol == "" {
		symbol = format.GetStatusSymbol(s.Severity)
	}

	// Apply color if enabled
	statusLine := symbol + " " + s.Message
	if f.useColors {
		switch s.Severity {
		case format.StatusSuccess:
			statusLine = format.Green(statusLine)
		case format.StatusError:
			statusLine = format.Red(statusLine)
		case format.StatusWarning:
			statusLine = format.Yellow(statusLine)
		case format.StatusRunning:
			statusLine = format.Blue(statusLine)
		}
	}

	sb.WriteString(statusLine)

	// Add details with indentation
	for _, detail := range s.Details {
		sb.WriteString("\n  ")
		sb.WriteString(detail)
	}

	return sb.String()
}

// formatError renders an error message
func (f *TextFormatter) formatError(e *format.Error) string {
	var sb strings.Builder

	// Error header
	errorPrefix := "Error: "
	if f.useColors {
		errorPrefix = format.Red(format.Bold("Error: "))
	}

	sb.WriteString(errorPrefix)
	sb.WriteString(e.Message)
	sb.WriteString("\n")

	// Field and value (if present)
	if e.Field != "" {
		sb.WriteString("\n")
		sb.WriteString("Field:  ")
		sb.WriteString(e.Field)
		sb.WriteString("\n")

		if e.Value != nil {
			sb.WriteString("Value:  ")
			sb.WriteString(fmt.Sprintf("%v", e.Value))
			sb.WriteString("\n")
		}
	}

	// Details
	if len(e.Details) > 0 {
		sb.WriteString("\n")
		for _, detail := range e.Details {
			sb.WriteString("  • ")
			sb.WriteString(detail)
			sb.WriteString("\n")
		}
	}

	// Suggestion
	if e.Suggestion != "" {
		sb.WriteString("\n")
		suggestionText := "Suggestion: " + e.Suggestion
		if f.useColors {
			suggestionText = format.Blue(suggestionText)
		}
		sb.WriteString(suggestionText)
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func init() {
	// Register text formatter as default (without colors)
	format.RegisterFormatter("text", NewTextFormatter(false))
}
