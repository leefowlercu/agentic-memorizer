package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// ValidatorFunc is a function that validates input.
type ValidatorFunc func(string) error

// TextInput wraps the bubbles textinput with additional functionality.
type TextInput struct {
	input     textinput.Model
	label     string
	validator ValidatorFunc
}

// NewTextInput creates a new text input with the given label and default value.
func NewTextInput(label, defaultValue string) TextInput {
	ti := textinput.New()
	ti.SetValue(defaultValue)
	ti.CharLimit = 256
	ti.Width = 40

	return TextInput{
		input: ti,
		label: label,
	}
}

// Init implements tea.Model.
func (t TextInput) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input events.
func (t TextInput) Update(msg tea.Msg) (TextInput, tea.Cmd) {
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return t, cmd
}

// View renders the text input.
func (t TextInput) View() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(styles.Secondary).
		MarginBottom(1)

	return labelStyle.Render(t.label) + "\n" + t.input.View()
}

// Value returns the current input value.
func (t TextInput) Value() string {
	return t.input.Value()
}

// SetValue sets the input value.
func (t *TextInput) SetValue(value string) {
	t.input.SetValue(value)
}

// Focus focuses the input.
func (t *TextInput) Focus() tea.Cmd {
	return t.input.Focus()
}

// Blur removes focus from the input.
func (t *TextInput) Blur() {
	t.input.Blur()
}

// Focused returns whether the input is focused.
func (t TextInput) Focused() bool {
	return t.input.Focused()
}

// SetPlaceholder sets the placeholder text.
func (t *TextInput) SetPlaceholder(placeholder string) {
	t.input.Placeholder = placeholder
}

// SetMasked enables or disables password masking.
func (t *TextInput) SetMasked(masked bool) {
	if masked {
		t.input.EchoMode = textinput.EchoPassword
		t.input.EchoCharacter = '•'
	} else {
		t.input.EchoMode = textinput.EchoNormal
	}
}

// SetValidator sets the validation function.
func (t *TextInput) SetValidator(fn ValidatorFunc) {
	t.validator = fn
}

// Validate runs the validator if set.
func (t TextInput) Validate() error {
	if t.validator != nil {
		return t.validator(t.input.Value())
	}
	return nil
}

// SetWidth sets the input width.
func (t *TextInput) SetWidth(width int) {
	t.input.Width = width
}

// SetCharLimit sets the character limit.
func (t *TextInput) SetCharLimit(limit int) {
	t.input.CharLimit = limit
}

// Reset clears the input value.
func (t *TextInput) Reset() {
	t.input.Reset()
}
