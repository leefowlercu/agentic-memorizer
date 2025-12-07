package formatters

import (
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTextFormatter(t *testing.T) {
	formatter := NewTextFormatter(false)
	assert.Equal(t, "text", formatter.Name())
	assert.False(t, formatter.SupportsColors())

	formatterWithColors := NewTextFormatter(true)
	assert.True(t, formatterWithColors.SupportsColors())
}

func TestTextFormatter_FormatSection(t *testing.T) {
	formatter := NewTextFormatter(false)

	section := format.NewSection("Test Section")
	section.AddKeyValue("Name", "John Doe")
	section.AddKeyValue("Age", "30")
	section.AddKeyValue("City", "New York")

	output, err := formatter.Format(section)
	require.NoError(t, err)

	expected := `Test Section
Name: John Doe
Age:  30
City: New York`

	assert.Equal(t, expected, output)
}

func TestTextFormatter_FormatSectionWithDivider(t *testing.T) {
	formatter := NewTextFormatter(false)

	section := format.NewSection("Status").AddDivider()
	section.AddKeyValue("Running", "Yes")
	section.AddKeyValue("PID", "12345")

	output, err := formatter.Format(section)
	require.NoError(t, err)

	assert.Contains(t, output, "Status")
	assert.Contains(t, output, "======")
	assert.Contains(t, output, "Running: Yes")
	assert.Contains(t, output, "PID:     12345")
}

func TestTextFormatter_FormatSectionNested(t *testing.T) {
	formatter := NewTextFormatter(false)

	subsection := format.NewSection("Subsection")
	subsection.AddKeyValue("Detail", "Value")

	section := format.NewSection("Main")
	section.AddKeyValue("Field1", "Value1")
	section.AddSubsection(subsection)

	output, err := formatter.Format(section)
	require.NoError(t, err)

	assert.Contains(t, output, "Main")
	assert.Contains(t, output, "Field1: Value1")
	assert.Contains(t, output, "  Subsection")
	assert.Contains(t, output, "  Detail: Value")
}

func TestTextFormatter_FormatTable(t *testing.T) {
	formatter := NewTextFormatter(false)

	table := format.NewTable("Name", "Age", "City")
	table.AddRow("Alice", "30", "NYC")
	table.AddRow("Bob", "25", "SF")
	table.AddRow("Charlie", "35", "LA")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	lines := strings.Split(output, "\n")
	assert.Len(t, lines, 5) // headers + separator + 3 rows

	// Check header
	assert.Contains(t, lines[0], "Name")
	assert.Contains(t, lines[0], "Age")
	assert.Contains(t, lines[0], "City")

	// Check separator
	assert.Contains(t, lines[1], "---")

	// Check data rows
	assert.Contains(t, lines[2], "Alice")
	assert.Contains(t, lines[3], "Bob")
	assert.Contains(t, lines[4], "Charlie")
}

func TestTextFormatter_FormatTableWithAlignment(t *testing.T) {
	formatter := NewTextFormatter(false)

	table := format.NewTable("Name", "Count", "Size")
	table.SetAlignments(format.AlignLeft, format.AlignRight, format.AlignCenter)
	table.AddRow("Item1", "100", "Large")
	table.AddRow("Item2", "50", "Small")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	// Verify output has proper spacing
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Count")
	assert.Contains(t, output, "Size")
}

func TestTextFormatter_FormatTableCompact(t *testing.T) {
	formatter := NewTextFormatter(false)

	table := format.NewTable("A", "B")
	table.Compact()
	table.AddRow("1", "2")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	// Compact mode should have single space spacing
	lines := strings.Split(output, "\n")
	assert.Contains(t, lines[0], "A B")
}

func TestTextFormatter_FormatTableHideHeader(t *testing.T) {
	formatter := NewTextFormatter(false)

	table := format.NewTable("A", "B")
	table.HideHeader()
	table.AddRow("1", "2")
	table.AddRow("3", "4")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	lines := strings.Split(output, "\n")
	assert.Len(t, lines, 2) // Only data rows, no headers

	assert.NotContains(t, output, "---") // No separator
}

func TestTextFormatter_FormatList(t *testing.T) {
	formatter := NewTextFormatter(false)

	list := format.NewList(format.ListTypeUnordered)
	list.AddItem("First item")
	list.AddItem("Second item")
	list.AddItem("Third item")

	output, err := formatter.Format(list)
	require.NoError(t, err)

	// Non-compact lists have blank lines between items
	expected := `- First item

- Second item

- Third item`

	assert.Equal(t, expected, output)
}

func TestTextFormatter_FormatListOrdered(t *testing.T) {
	formatter := NewTextFormatter(false)

	list := format.NewList(format.ListTypeOrdered)
	list.AddItem("First")
	list.AddItem("Second")
	list.AddItem("Third")

	output, err := formatter.Format(list)
	require.NoError(t, err)

	assert.Contains(t, output, "1. First")
	assert.Contains(t, output, "2. Second")
	assert.Contains(t, output, "3. Third")
}

func TestTextFormatter_FormatListNested(t *testing.T) {
	formatter := NewTextFormatter(false)

	nested := format.NewList(format.ListTypeUnordered)
	nested.AddItem("Nested A")
	nested.AddItem("Nested B")

	list := format.NewList(format.ListTypeOrdered)
	list.AddItem("Item 1")
	list.AddNested("Item 2 with nested", nested)
	list.AddItem("Item 3")

	output, err := formatter.Format(list)
	require.NoError(t, err)

	assert.Contains(t, output, "1. Item 1")
	assert.Contains(t, output, "2. Item 2 with nested")
	assert.Contains(t, output, "  - Nested A")
	assert.Contains(t, output, "  - Nested B")
	assert.Contains(t, output, "3. Item 3")
}

func TestTextFormatter_FormatListCompact(t *testing.T) {
	formatter := NewTextFormatter(false)

	list := format.NewList(format.ListTypeUnordered)
	list.Compact()
	list.AddItem("Item 1")
	list.AddItem("Item 2")
	list.AddItem("Item 3")

	output, err := formatter.Format(list)
	require.NoError(t, err)

	lines := strings.Split(output, "\n")
	assert.Len(t, lines, 3) // Compact mode, no extra blank lines
}

func TestTextFormatter_FormatProgress(t *testing.T) {
	formatter := NewTextFormatter(false)

	tests := []struct {
		name     string
		progress *format.Progress
		contains []string
	}{
		{
			"bar",
			format.NewProgress(format.ProgressTypeBar, 50, 100).SetMessage("Processing"),
			[]string{"[", "=", ">", "]", "Processing", "50.0%"},
		},
		{
			"percentage",
			format.NewProgress(format.ProgressTypePercentage, 75, 100).SetMessage("Loading"),
			[]string{"Loading:", "75.0%"},
		},
		{
			"spinner",
			format.NewProgress(format.ProgressTypeSpinner, 0, 0).SetMessage("Working"),
			[]string{"Working"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatter.Format(tt.progress)
			require.NoError(t, err)

			for _, substr := range tt.contains {
				assert.Contains(t, output, substr)
			}
		})
	}
}

func TestTextFormatter_FormatStatus(t *testing.T) {
	formatter := NewTextFormatter(false)

	status := format.NewStatus(format.StatusSuccess, "Operation completed")
	status.AddDetail("Step 1 finished")
	status.AddDetail("Step 2 finished")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	assert.Contains(t, output, format.SymbolSuccess)
	assert.Contains(t, output, "Operation completed")
	assert.Contains(t, output, "Step 1 finished")
	assert.Contains(t, output, "Step 2 finished")
}

func TestTextFormatter_FormatStatusWithCustomSymbol(t *testing.T) {
	formatter := NewTextFormatter(false)

	status := format.NewStatus(format.StatusInfo, "Custom").WithSymbol("🎉")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	assert.Contains(t, output, "🎉")
	assert.Contains(t, output, "Custom")
}

func TestTextFormatter_FormatStatusWithColors(t *testing.T) {
	formatter := NewTextFormatter(true)

	tests := []struct {
		severity format.StatusSeverity
		message  string
	}{
		{format.StatusSuccess, "Success message"},
		{format.StatusError, "Error message"},
		{format.StatusWarning, "Warning message"},
		{format.StatusRunning, "Running message"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			status := format.NewStatus(tt.severity, tt.message)
			output, err := formatter.Format(status)
			require.NoError(t, err)

			// Should contain ANSI color codes
			assert.Contains(t, output, "\033[")
			assert.Contains(t, output, tt.message)
		})
	}
}

func TestTextFormatter_FormatError(t *testing.T) {
	formatter := NewTextFormatter(false)

	err := format.NewError(format.ErrorTypeValidation, "Invalid input")
	err.SetField("username")
	err.SetValue("ab")
	err.AddDetail("Must be at least 3 characters")
	err.WithSuggestion("Use a longer username")

	output, errFmt := formatter.Format(err)
	require.NoError(t, errFmt)

	assert.Contains(t, output, "Error: Invalid input")
	assert.Contains(t, output, "Field:  username")
	assert.Contains(t, output, "Value:  ab")
	assert.Contains(t, output, "• Must be at least 3 characters")
	assert.Contains(t, output, "Suggestion: Use a longer username")
}

func TestTextFormatter_FormatMultiple(t *testing.T) {
	formatter := NewTextFormatter(false)

	section := format.NewSection("Section")
	section.AddKeyValue("Key", "Value")

	list := format.NewList(format.ListTypeUnordered)
	list.AddItem("Item 1")
	list.AddItem("Item 2")

	builders := []format.Buildable{section, list}
	output, err := formatter.FormatMultiple(builders)
	require.NoError(t, err)

	assert.Contains(t, output, "Section")
	assert.Contains(t, output, "Key: Value")
	assert.Contains(t, output, "- Item 1")
	assert.Contains(t, output, "- Item 2")

	// Should have double newline between sections
	assert.Contains(t, output, "\n\n")
}

func TestTextFormatter_ValidationError(t *testing.T) {
	formatter := NewTextFormatter(false)

	// Create invalid section (empty title)
	section := &format.Section{Title: "", Items: []format.SectionItem{}}

	_, err := formatter.Format(section)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestTextFormatter_UnsupportedType(t *testing.T) {
	formatter := NewTextFormatter(false)

	// Use a mock buildable with unknown type
	mockBuildable := &mockBuildable{builderType: "unknown"}

	_, err := formatter.Format(mockBuildable)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported builder type")
}

// mockBuildable for testing unsupported types
type mockBuildable struct {
	builderType format.BuilderType
}

func (m *mockBuildable) Type() format.BuilderType {
	return m.builderType
}

func (m *mockBuildable) Validate() error {
	return nil
}
