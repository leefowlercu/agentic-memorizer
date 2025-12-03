package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// RadioOption represents a single radio button option
type RadioOption struct {
	Label       string
	Description string
}

// RadioGroup manages a group of radio buttons with single selection
type RadioGroup struct {
	options  []RadioOption
	selected int
	focused  bool
}

// NewRadioGroup creates a new radio group with the given options
func NewRadioGroup(options []RadioOption) *RadioGroup {
	return &RadioGroup{
		options:  options,
		selected: 0,
		focused:  false,
	}
}

// NewRadioGroupSimple creates a radio group from simple string labels
func NewRadioGroupSimple(labels []string) *RadioGroup {
	options := make([]RadioOption, len(labels))
	for i, label := range labels {
		options[i] = RadioOption{Label: label}
	}
	return NewRadioGroup(options)
}

// Focus sets the focus state
func (r *RadioGroup) Focus() {
	r.focused = true
}

// Blur removes focus
func (r *RadioGroup) Blur() {
	r.focused = false
}

// IsFocused returns true if the radio group is focused
func (r *RadioGroup) IsFocused() bool {
	return r.focused
}

// Selected returns the index of the selected option
func (r *RadioGroup) Selected() int {
	return r.selected
}

// SetSelected sets the selected option by index
func (r *RadioGroup) SetSelected(index int) {
	if index >= 0 && index < len(r.options) {
		r.selected = index
	}
}

// SelectedOption returns the selected option
func (r *RadioGroup) SelectedOption() RadioOption {
	if r.selected >= 0 && r.selected < len(r.options) {
		return r.options[r.selected]
	}
	return RadioOption{}
}

// Update handles keyboard input
func (r *RadioGroup) Update(msg tea.Msg) tea.Cmd {
	if !r.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			r.selected--
			if r.selected < 0 {
				r.selected = len(r.options) - 1
			}
		case "down", "j":
			r.selected++
			if r.selected >= len(r.options) {
				r.selected = 0
			}
		}
	}

	return nil
}

// View renders the radio group
func (r *RadioGroup) View() string {
	var b strings.Builder

	for i, opt := range r.options {
		cursor := "  "
		indicator := styles.RadioUnselected
		style := styles.Unfocused

		if i == r.selected {
			indicator = styles.RadioSelected
			style = styles.Selected
			if r.focused {
				cursor = styles.Cursor.Render(styles.CursorIndicator + " ")
			}
		}

		line := cursor + style.Render(indicator+" "+opt.Label)
		b.WriteString(line)

		if opt.Description != "" {
			desc := styles.MutedText.Render("    " + opt.Description)
			b.WriteString("\n" + desc)
		}

		if i < len(r.options)-1 {
			b.WriteString("\n\n")
		}
	}

	return b.String()
}
