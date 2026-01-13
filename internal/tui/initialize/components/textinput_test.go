package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTextInput_New(t *testing.T) {
	ti := NewTextInput("Enter value", "default")

	if ti.Value() != "default" {
		t.Errorf("expected default value 'default', got '%s'", ti.Value())
	}
}

func TestTextInput_SetValue(t *testing.T) {
	ti := NewTextInput("Enter value", "")
	ti.SetValue("test value")

	if ti.Value() != "test value" {
		t.Errorf("expected 'test value', got '%s'", ti.Value())
	}
}

func TestTextInput_Focus(t *testing.T) {
	ti := NewTextInput("Enter value", "")

	if ti.Focused() {
		t.Error("expected input to not be focused initially")
	}

	ti.Focus()
	if !ti.Focused() {
		t.Error("expected input to be focused after Focus()")
	}

	ti.Blur()
	if ti.Focused() {
		t.Error("expected input to not be focused after Blur()")
	}
}

func TestTextInput_Masked(t *testing.T) {
	ti := NewTextInput("Enter API key", "")
	ti.SetMasked(true)
	ti.SetValue("secret123")

	view := ti.View()
	// Masked input should not show the actual value
	if strings.Contains(view, "secret123") {
		t.Error("masked input should not display the actual value")
	}
}

func TestTextInput_Placeholder(t *testing.T) {
	ti := NewTextInput("Enter value", "")
	ti.SetPlaceholder("Type here...")

	// When empty, view should contain placeholder
	view := ti.View()
	if !strings.Contains(view, "Type here...") {
		t.Error("view should contain placeholder when empty")
	}
}

func TestTextInput_Update(t *testing.T) {
	ti := NewTextInput("Enter value", "")
	ti.Focus()

	// Simulate typing
	ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	if ti.Value() != "hi" {
		t.Errorf("expected 'hi', got '%s'", ti.Value())
	}
}

func TestTextInput_Validation(t *testing.T) {
	ti := NewTextInput("Enter port", "")

	// Set a validator that requires non-empty value
	ti.SetValidator(func(s string) error {
		if s == "" {
			return errEmpty
		}
		return nil
	})

	err := ti.Validate()
	if err == nil {
		t.Error("expected validation error for empty value")
	}

	ti.SetValue("7600")
	err = ti.Validate()
	if err != nil {
		t.Errorf("expected no validation error, got %v", err)
	}
}

var errEmpty = &validationError{"value cannot be empty"}

type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}
