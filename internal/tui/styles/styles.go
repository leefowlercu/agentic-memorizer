// Package styles provides shared lipgloss styles for TUI components.
package styles

import "github.com/charmbracelet/lipgloss"

// Color palette using ANSI colors for broad terminal compatibility.
var (
	Primary   = lipgloss.Color("4")   // Blue
	Secondary = lipgloss.Color("245") // Light gray (visible on dark backgrounds)
	Success   = lipgloss.Color("2")   // Green
	Warning   = lipgloss.Color("3")   // Yellow
	Error     = lipgloss.Color("1")   // Red
	Highlight = lipgloss.Color("12")  // Bright blue
	Muted     = lipgloss.Color("245") // Light gray (visible on dark backgrounds)
)

// Text styles.
var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	Subtitle = lipgloss.NewStyle().
			Foreground(Secondary).
			Italic(true)

	Label = lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	ErrorText = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	SuccessText = lipgloss.NewStyle().
			Foreground(Success)

	MutedText = lipgloss.NewStyle().
			Foreground(Muted)

	HelpText = lipgloss.NewStyle().
			Foreground(Secondary).
			Italic(true)
)

// Component styles.
var (
	Focused = lipgloss.NewStyle().
		Foreground(Highlight)

	Unfocused = lipgloss.NewStyle().
			Foreground(Secondary)

	Selected = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	Cursor = lipgloss.NewStyle().
		Foreground(Highlight).
		Bold(true)
)

// Layout styles.
var (
	Container = lipgloss.NewStyle().
			PaddingTop(1).
			PaddingLeft(2).
			PaddingRight(2)

	StepContainer = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Secondary).
			Padding(1, 2).
			MarginBottom(1)
)

// Indicators.
const (
	RadioSelected   = "(●)"
	RadioUnselected = "( )"

	CheckboxSelected   = "[✓]"
	CheckboxUnselected = "[ ]"

	ProgressFilled = "●"
	ProgressEmpty  = "○"

	CursorIndicator = "▸"
)
