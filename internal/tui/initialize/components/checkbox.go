package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// CheckboxItem represents a single checkbox option
type CheckboxItem struct {
	Label       string
	Description string
	Value       any
	Checked     bool
	Enabled     bool // If false, item is shown but not selectable
}

// CheckboxList manages a list of checkboxes with multi-selection
type CheckboxList struct {
	items   []CheckboxItem
	cursor  int
	focused bool
}

// NewCheckboxList creates a new checkbox list.
// Creates a copy of the items slice to avoid modifying the caller's data.
// Callers must explicitly set Enabled for each item.
func NewCheckboxList(items []CheckboxItem) *CheckboxList {
	// Create a copy to avoid modifying caller's slice
	itemsCopy := make([]CheckboxItem, len(items))
	copy(itemsCopy, items)

	return &CheckboxList{
		items:   itemsCopy,
		cursor:  0,
		focused: false,
	}
}

// Focus sets the focus state
func (c *CheckboxList) Focus() {
	c.focused = true
}

// Blur removes focus
func (c *CheckboxList) Blur() {
	c.focused = false
}

// IsFocused returns true if the checkbox list is focused
func (c *CheckboxList) IsFocused() bool {
	return c.focused
}

// Items returns all items
func (c *CheckboxList) Items() []CheckboxItem {
	return c.items
}

// CheckedItems returns only the checked items
func (c *CheckboxList) CheckedItems() []CheckboxItem {
	var checked []CheckboxItem
	for _, item := range c.items {
		if item.Checked {
			checked = append(checked, item)
		}
	}
	return checked
}

// CheckedValues returns the values of checked items
func (c *CheckboxList) CheckedValues() []any {
	var values []any
	for _, item := range c.items {
		if item.Checked {
			values = append(values, item.Value)
		}
	}
	return values
}

// SetChecked sets the checked state by index
func (c *CheckboxList) SetChecked(index int, checked bool) {
	if index >= 0 && index < len(c.items) && c.items[index].Enabled {
		c.items[index].Checked = checked
	}
}

// Toggle toggles the current item
func (c *CheckboxList) Toggle() {
	if c.cursor >= 0 && c.cursor < len(c.items) && c.items[c.cursor].Enabled {
		c.items[c.cursor].Checked = !c.items[c.cursor].Checked
	}
}

// SelectAll checks all enabled items
func (c *CheckboxList) SelectAll() {
	for i := range c.items {
		if c.items[i].Enabled {
			c.items[i].Checked = true
		}
	}
}

// SelectNone unchecks all items
func (c *CheckboxList) SelectNone() {
	for i := range c.items {
		c.items[i].Checked = false
	}
}

// Update handles keyboard input
func (c *CheckboxList) Update(msg tea.Msg) tea.Cmd {
	if !c.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			c.cursor--
			if c.cursor < 0 {
				c.cursor = len(c.items) - 1
			}
		case "down", "j":
			c.cursor++
			if c.cursor >= len(c.items) {
				c.cursor = 0
			}
		case " ", "x":
			c.Toggle()
		case "a":
			c.SelectAll()
		case "n":
			c.SelectNone()
		}
	}

	return nil
}

// View renders the checkbox list
func (c *CheckboxList) View() string {
	var b strings.Builder

	for i, item := range c.items {
		cursor := "  "
		indicator := styles.CheckboxUnselected
		style := styles.Unfocused

		if item.Checked {
			indicator = styles.CheckboxSelected
			style = styles.Selected
		}

		if !item.Enabled {
			style = styles.MutedText
		}

		if i == c.cursor && c.focused {
			cursor = styles.Cursor.Render(styles.CursorIndicator + " ")
			if item.Enabled {
				style = styles.Focused
				if item.Checked {
					style = styles.Selected
				}
			}
		}

		line := cursor + style.Render(indicator+" "+item.Label)
		b.WriteString(line)

		if item.Description != "" {
			desc := styles.MutedText.Render("    " + item.Description)
			b.WriteString("\n" + desc)
		}

		if i < len(c.items)-1 {
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

// ViewHelp renders help text for the checkbox list
func (c *CheckboxList) ViewHelp() string {
	return styles.HelpText.Render("space: toggle  a: select all  n: select none")
}
