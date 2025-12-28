package formatters

import (
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMarkdownFormatter(t *testing.T) {
	formatter := NewMarkdownFormatter()
	assert.Equal(t, "markdown", formatter.Name())
	assert.False(t, formatter.SupportsColors())
}

func TestMarkdownFormatter_FormatSection(t *testing.T) {
	formatter := NewMarkdownFormatter()

	section := format.NewSection("Test Section")
	section.AddKeyValue("Name", "John Doe")
	section.AddKeyValue("Age", "30")

	output, err := formatter.Format(section)
	require.NoError(t, err)

	expected := `# Test Section

**Name**: John Doe
**Age**: 30`

	assert.Equal(t, expected, output)
}

func TestMarkdownFormatter_FormatSectionWithLevel(t *testing.T) {
	formatter := NewMarkdownFormatter()

	section := format.NewSection("Level 0")
	section.Level = 0

	subsection := format.NewSection("Level 1")
	subsection.Level = 1
	subsection.AddKeyValue("Key", "Value")

	section.AddSubsection(subsection)

	output, err := formatter.Format(section)
	require.NoError(t, err)

	assert.Contains(t, output, "# Level 0")
	assert.Contains(t, output, "## Level 1")
	assert.Contains(t, output, "**Key**: Value")
}

func TestMarkdownFormatter_FormatTable(t *testing.T) {
	formatter := NewMarkdownFormatter()

	table := format.NewTable("Name", "Age", "City")
	table.AddRow("Alice", "30", "NYC")
	table.AddRow("Bob", "25", "SF")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	lines := strings.Split(output, "\n")
	assert.Len(t, lines, 4) // headers + separator + 2 rows

	// Check header
	assert.Contains(t, lines[0], "| Name | Age | City |")

	// Check separator with left alignment by default
	assert.Contains(t, lines[1], "| :--- | :--- | :--- |")

	// Check data rows
	assert.Contains(t, lines[2], "| Alice | 30 | NYC |")
	assert.Contains(t, lines[3], "| Bob | 25 | SF |")
}

func TestMarkdownFormatter_FormatTableWithAlignment(t *testing.T) {
	formatter := NewMarkdownFormatter()

	table := format.NewTable("Name", "Count", "Size")
	table.SetAlignments(format.AlignLeft, format.AlignRight, format.AlignCenter)
	table.AddRow("Item1", "100", "Large")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	lines := strings.Split(output, "\n")
	separator := lines[1]

	assert.Contains(t, separator, ":---")  // Left
	assert.Contains(t, separator, "---:")  // Right
	assert.Contains(t, separator, ":---:") // Center
}

func TestMarkdownFormatter_FormatTableHideHeader(t *testing.T) {
	formatter := NewMarkdownFormatter()

	table := format.NewTable("A", "B")
	table.HideHeader()
	table.AddRow("1", "2")
	table.AddRow("3", "4")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	lines := strings.Split(output, "\n")
	assert.Len(t, lines, 2) // Only data rows

	// Should not contain header row or separator
	assert.NotContains(t, output, "| A | B |")
	assert.NotContains(t, output, ":---")
}

func TestMarkdownFormatter_FormatList(t *testing.T) {
	formatter := NewMarkdownFormatter()

	list := format.NewList(format.ListTypeUnordered)
	list.AddItem("First item")
	list.AddItem("Second item")
	list.AddItem("Third item")

	output, err := formatter.Format(list)
	require.NoError(t, err)

	expected := `- First item
- Second item
- Third item`

	assert.Equal(t, expected, output)
}

func TestMarkdownFormatter_FormatListOrdered(t *testing.T) {
	formatter := NewMarkdownFormatter()

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

func TestMarkdownFormatter_FormatListNested(t *testing.T) {
	formatter := NewMarkdownFormatter()

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

func TestMarkdownFormatter_FormatProgress(t *testing.T) {
	formatter := NewMarkdownFormatter()

	tests := []struct {
		name     string
		progress *format.Progress
		contains []string
	}{
		{
			"bar",
			format.NewProgress(format.ProgressTypeBar, 50, 100).SetMessage("Processing"),
			[]string{"`[", "█", "░", "]`", "Processing", "**50.0%**"},
		},
		{
			"percentage",
			format.NewProgress(format.ProgressTypePercentage, 75, 100).SetMessage("Loading"),
			[]string{"Loading:", "**75.0%**"},
		},
		{
			"spinner",
			format.NewProgress(format.ProgressTypeSpinner, 0, 0).SetMessage("Working"),
			[]string{"*Working...*"},
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

func TestMarkdownFormatter_FormatStatus(t *testing.T) {
	formatter := NewMarkdownFormatter()

	status := format.NewStatus(format.StatusSuccess, "Operation completed")
	status.AddDetail("Step 1 finished")
	status.AddDetail("Step 2 finished")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	assert.Contains(t, output, format.SymbolSuccess)
	assert.Contains(t, output, "**Operation completed**")
	assert.Contains(t, output, "- Step 1 finished")
	assert.Contains(t, output, "- Step 2 finished")
}

func TestMarkdownFormatter_FormatStatusCustomSymbol(t *testing.T) {
	formatter := NewMarkdownFormatter()

	status := format.NewStatus(format.StatusInfo, "Custom").WithSymbol("🎉")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	assert.Contains(t, output, "🎉")
	assert.Contains(t, output, "**Custom**")
}

func TestMarkdownFormatter_FormatError(t *testing.T) {
	formatter := NewMarkdownFormatter()

	err := format.NewError(format.ErrorTypeValidation, "Invalid input")
	err.SetField("username")
	err.SetValue("ab")
	err.AddDetail("Must be at least 3 characters")
	err.WithSuggestion("Use a longer username")

	output, errFmt := formatter.Format(err)
	require.NoError(t, errFmt)

	assert.Contains(t, output, "**Error**: Invalid input")
	assert.Contains(t, output, "**Field**: `username`")
	assert.Contains(t, output, "**Value**: `ab`")
	assert.Contains(t, output, "- Must be at least 3 characters")
	assert.Contains(t, output, "**Suggestion**: Use a longer username")
}

func TestMarkdownFormatter_FormatMultiple(t *testing.T) {
	formatter := NewMarkdownFormatter()

	section := format.NewSection("Section")
	section.AddKeyValue("Key", "Value")

	list := format.NewList(format.ListTypeUnordered)
	list.AddItem("Item 1")
	list.AddItem("Item 2")

	builders := []format.Buildable{section, list}
	output, err := formatter.FormatMultiple(builders)
	require.NoError(t, err)

	assert.Contains(t, output, "# Section")
	assert.Contains(t, output, "**Key**: Value")
	assert.Contains(t, output, "- Item 1")
	assert.Contains(t, output, "- Item 2")

	// Should have separator
	assert.Contains(t, output, "---")
}

func TestMarkdownFormatter_ValidationError(t *testing.T) {
	formatter := NewMarkdownFormatter()

	// Create invalid section (empty title)
	section := &format.Section{Title: "", Items: []format.SectionItem{}}

	_, err := formatter.Format(section)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestMarkdownFormatter_FormatFacts(t *testing.T) {
	formatter := NewMarkdownFormatter()

	now := time.Now()
	index := &types.FactsIndex{
		Generated: now,
		Facts: []types.Fact{
			{
				ID:        "fact-1",
				Content:   "This is a test fact",
				CreatedAt: now,
				Source:    "cli",
			},
			{
				ID:        "fact-2",
				Content:   "Another test fact",
				CreatedAt: now.Add(-time.Hour),
				UpdatedAt: now,
				Source:    "cli",
			},
		},
		Stats: types.FactStats{
			TotalFacts: 2,
			MaxFacts:   50,
		},
	}

	fc := format.NewFactsContent(index)
	output, err := formatter.Format(fc)
	require.NoError(t, err)

	// Verify Markdown structure
	assert.Contains(t, output, "# Facts")
	assert.Contains(t, output, "**Generated**:")
	assert.Contains(t, output, "**Total Facts**: 2 / 50")
	assert.Contains(t, output, "### fact-1")
	assert.Contains(t, output, "This is a test fact")
	assert.Contains(t, output, "*Source*: cli")
	assert.Contains(t, output, "### fact-2")
	assert.Contains(t, output, "*Updated*:")
}

func TestMarkdownFormatter_FormatFactsEmpty(t *testing.T) {
	formatter := NewMarkdownFormatter()

	index := &types.FactsIndex{
		Generated: time.Now(),
		Facts:     []types.Fact{},
		Stats: types.FactStats{
			TotalFacts: 0,
			MaxFacts:   50,
		},
	}

	fc := format.NewFactsContent(index)
	output, err := formatter.Format(fc)
	require.NoError(t, err)

	assert.Contains(t, output, "# Facts")
	assert.Contains(t, output, "**Total Facts**: 0 / 50")
	assert.Contains(t, output, "*No facts stored.*")
	// Should not have the separator when empty
	assert.NotContains(t, output, "---")
}

func TestMarkdownFormatter_FormatFactsValidationError(t *testing.T) {
	formatter := NewMarkdownFormatter()

	fc := format.NewFactsContent(nil)
	_, err := formatter.Format(fc)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}
