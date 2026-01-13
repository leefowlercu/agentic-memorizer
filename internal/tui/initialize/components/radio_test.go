package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRadioGroup_New(t *testing.T) {
	options := []RadioOption{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
		{Label: "Option 3", Value: "opt3"},
	}

	r := NewRadioGroup(options)

	if r.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", r.cursor)
	}

	if len(r.options) != 3 {
		t.Errorf("expected 3 options, got %d", len(r.options))
	}
}

func TestRadioGroup_Navigation(t *testing.T) {
	options := []RadioOption{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
		{Label: "Option 3", Value: "opt3"},
	}

	r := NewRadioGroup(options)

	// Move down
	r, _ = r.Update(tea.KeyMsg{Type: tea.KeyDown})
	if r.cursor != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", r.cursor)
	}

	// Move down again
	r, _ = r.Update(tea.KeyMsg{Type: tea.KeyDown})
	if r.cursor != 2 {
		t.Errorf("expected cursor at 2 after second down, got %d", r.cursor)
	}

	// Move down at bottom should wrap to top
	r, _ = r.Update(tea.KeyMsg{Type: tea.KeyDown})
	if r.cursor != 0 {
		t.Errorf("expected cursor to wrap to 0, got %d", r.cursor)
	}

	// Move up should wrap to bottom
	r, _ = r.Update(tea.KeyMsg{Type: tea.KeyUp})
	if r.cursor != 2 {
		t.Errorf("expected cursor to wrap to 2, got %d", r.cursor)
	}
}

func TestRadioGroup_Selection(t *testing.T) {
	options := []RadioOption{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
	}

	r := NewRadioGroup(options)

	// Initial selection
	if r.Selected() != "opt1" {
		t.Errorf("expected initial selection 'opt1', got '%s'", r.Selected())
	}

	// Move to second option
	r, _ = r.Update(tea.KeyMsg{Type: tea.KeyDown})
	if r.Selected() != "opt2" {
		t.Errorf("expected selection 'opt2' after down, got '%s'", r.Selected())
	}
}

func TestRadioGroup_View(t *testing.T) {
	options := []RadioOption{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
	}

	r := NewRadioGroup(options)
	view := r.View()

	if !strings.Contains(view, "Option 1") {
		t.Error("view should contain 'Option 1'")
	}

	if !strings.Contains(view, "Option 2") {
		t.Error("view should contain 'Option 2'")
	}
}

func TestRadioGroup_SetOptions(t *testing.T) {
	options := []RadioOption{
		{Label: "Option 1", Value: "opt1"},
	}

	r := NewRadioGroup(options)

	newOptions := []RadioOption{
		{Label: "New 1", Value: "new1"},
		{Label: "New 2", Value: "new2"},
	}

	r.SetOptions(newOptions)

	if len(r.options) != 2 {
		t.Errorf("expected 2 options after SetOptions, got %d", len(r.options))
	}

	if r.Selected() != "new1" {
		t.Errorf("expected selection 'new1' after SetOptions, got '%s'", r.Selected())
	}
}

func TestRadioGroup_SetCursor(t *testing.T) {
	options := []RadioOption{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
		{Label: "Option 3", Value: "opt3"},
	}

	r := NewRadioGroup(options)

	r.SetCursor(2)
	if r.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", r.cursor)
	}

	// Out of bounds should be clamped
	r.SetCursor(10)
	if r.cursor != 2 {
		t.Errorf("expected cursor to remain at 2, got %d", r.cursor)
	}

	r.SetCursor(-1)
	if r.cursor != 0 {
		t.Errorf("expected cursor to be 0 for negative, got %d", r.cursor)
	}
}
