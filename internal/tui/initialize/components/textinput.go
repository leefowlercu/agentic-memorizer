package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// TextInput wraps bubbletea's textinput with additional features
type TextInput struct {
	input       textinput.Model
	label       string
	placeholder string
	masked      bool
}

// NewTextInput creates a new text input with a label
func NewTextInput(label string) *TextInput {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 50

	return &TextInput{
		input: ti,
		label: label,
	}
}

// WithPlaceholder sets the placeholder text
func (t *TextInput) WithPlaceholder(placeholder string) *TextInput {
	t.placeholder = placeholder
	t.input.Placeholder = placeholder
	return t
}

// WithMasked enables password masking
func (t *TextInput) WithMasked() *TextInput {
	t.masked = true
	t.input.EchoMode = textinput.EchoPassword
	t.input.EchoCharacter = '•'
	return t
}

// WithWidth sets the input width
func (t *TextInput) WithWidth(width int) *TextInput {
	t.input.Width = width
	return t
}

// Focus sets the focus state
func (t *TextInput) Focus() tea.Cmd {
	return t.input.Focus()
}

// Blur removes focus
func (t *TextInput) Blur() {
	t.input.Blur()
}

// IsFocused returns true if the input is focused
func (t *TextInput) IsFocused() bool {
	return t.input.Focused()
}

// Value returns the current input value
func (t *TextInput) Value() string {
	return t.input.Value()
}

// SetValue sets the input value
func (t *TextInput) SetValue(value string) {
	t.input.SetValue(value)
}

// Update handles keyboard input
func (t *TextInput) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return cmd
}

// View renders the text input with its label
func (t *TextInput) View() string {
	labelStyle := styles.Label
	if t.input.Focused() {
		labelStyle = styles.Focused
	}

	label := labelStyle.Render(t.label + ":")
	input := t.input.View()

	return lipgloss.JoinVertical(lipgloss.Left, label, input)
}

// ViewInline renders the text input with label on the same line
func (t *TextInput) ViewInline() string {
	labelStyle := styles.Label
	if t.input.Focused() {
		labelStyle = styles.Focused
	}

	return labelStyle.Render(t.label+": ") + t.input.View()
}
