// Package components provides reusable TUI components for the initialization wizard.
package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// RadioOption represents a single option in a radio group.
type RadioOption struct {
	Label       string
	Value       string
	Description string
}

// RadioGroup is a component for selecting one option from a list.
type RadioGroup struct {
	options []RadioOption
	cursor  int
}

// NewRadioGroup creates a new radio group with the given options.
func NewRadioGroup(options []RadioOption) RadioGroup {
	return RadioGroup{
		options: options,
		cursor:  0,
	}
}

// Init implements tea.Model.
func (r RadioGroup) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for navigation.
func (r RadioGroup) Update(msg tea.Msg) (RadioGroup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyShiftTab:
			r.cursor--
			if r.cursor < 0 {
				r.cursor = len(r.options) - 1
			}
		case tea.KeyDown, tea.KeyTab:
			r.cursor++
			if r.cursor >= len(r.options) {
				r.cursor = 0
			}
		}

		switch msg.String() {
		case "k":
			r.cursor--
			if r.cursor < 0 {
				r.cursor = len(r.options) - 1
			}
		case "j":
			r.cursor++
			if r.cursor >= len(r.options) {
				r.cursor = 0
			}
		}
	}

	return r, nil
}

// View renders the radio group.
func (r RadioGroup) View() string {
	var b strings.Builder

	descStyle := lipgloss.NewStyle().
		Foreground(styles.Muted).
		MarginLeft(4)

	for i, opt := range r.options {
		cursor := "  "
		style := styles.Unfocused

		if i == r.cursor {
			cursor = styles.CursorIndicator + " "
			style = styles.Cursor
		}

		b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt.Label)))

		// Always show description if present
		if opt.Description != "" {
			b.WriteString(descStyle.Render(opt.Description))
			b.WriteString("\n")
		}

		// Add vertical separation between options
		if i < len(r.options)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// Selected returns the value of the currently selected option.
func (r RadioGroup) Selected() string {
	if len(r.options) == 0 {
		return ""
	}
	return r.options[r.cursor].Value
}

// SelectedLabel returns the label of the currently selected option.
func (r RadioGroup) SelectedLabel() string {
	if len(r.options) == 0 {
		return ""
	}
	return r.options[r.cursor].Label
}

// SetOptions replaces the options and resets the cursor.
func (r *RadioGroup) SetOptions(options []RadioOption) {
	r.options = options
	r.cursor = 0
}

// SetCursor sets the cursor position, clamping to valid range.
func (r *RadioGroup) SetCursor(pos int) {
	if pos < 0 {
		r.cursor = 0
	} else if pos >= len(r.options) {
		r.cursor = len(r.options) - 1
	} else {
		r.cursor = pos
	}
}

// Cursor returns the current cursor position.
func (r RadioGroup) Cursor() int {
	return r.cursor
}

// Options returns the current options.
func (r RadioGroup) Options() []RadioOption {
	return r.options
}
