package formatters

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJSONFormatter(t *testing.T) {
	formatter := NewJSONFormatter()
	assert.Equal(t, "json", formatter.Name())
	assert.False(t, formatter.SupportsColors())
}

func TestJSONFormatter_FormatSection(t *testing.T) {
	formatter := NewJSONFormatter()

	section := format.NewSection("Test Section")
	section.AddKeyValue("Name", "John")
	section.AddKeyValue("Age", "30")

	output, err := formatter.Format(section)
	require.NoError(t, err)

	// Unmarshal and verify structure
	var result jsonSection
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "section", result.Type)
	assert.Equal(t, "Test Section", result.Title)
	assert.Equal(t, "John", result.Fields["Name"])
	assert.Equal(t, "30", result.Fields["Age"])
	assert.Len(t, result.Items, 2)
}

func TestJSONFormatter_FormatSectionNested(t *testing.T) {
	formatter := NewJSONFormatter()

	subsection := format.NewSection("Sub")
	subsection.AddKeyValue("Detail", "Value")

	section := format.NewSection("Main")
	section.AddKeyValue("Field", "Data")
	section.AddSubsection(subsection)

	output, err := formatter.Format(section)
	require.NoError(t, err)

	var result jsonSection
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "Main", result.Title)
	assert.Len(t, result.Items, 2)

	// Check subsection
	assert.Equal(t, "subsection", result.Items[1].Type)
	assert.NotNil(t, result.Items[1].Subsection)
	assert.Equal(t, "Sub", result.Items[1].Subsection.Title)
}

func TestJSONFormatter_FormatTable(t *testing.T) {
	formatter := NewJSONFormatter()

	table := format.NewTable("Name", "Age", "City")
	table.SetAlignments(format.AlignLeft, format.AlignRight, format.AlignCenter)
	table.AddRow("Alice", "30", "NYC")
	table.AddRow("Bob", "25", "SF")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	var result jsonTable
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "table", result.Type)
	assert.Equal(t, []string{"Name", "Age", "City"}, result.Headers)
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, []string{"Alice", "30", "NYC"}, result.Rows[0])
	assert.Equal(t, []string{"left", "right", "center"}, result.Alignments)
}

func TestJSONFormatter_FormatList(t *testing.T) {
	formatter := NewJSONFormatter()

	list := format.NewList(format.ListTypeOrdered)
	list.AddItem("First")
	list.AddItem("Second")
	list.AddItem("Third")

	output, err := formatter.Format(list)
	require.NoError(t, err)

	var result jsonList
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "list", result.Type)
	assert.Equal(t, "ordered", result.ListType)
	assert.Len(t, result.Items, 3)
	assert.Equal(t, "First", result.Items[0].Content)
	assert.Equal(t, "Second", result.Items[1].Content)
	assert.Equal(t, "Third", result.Items[2].Content)
}

func TestJSONFormatter_FormatListNested(t *testing.T) {
	formatter := NewJSONFormatter()

	nested := format.NewList(format.ListTypeUnordered)
	nested.AddItem("Nested A")
	nested.AddItem("Nested B")

	list := format.NewList(format.ListTypeOrdered)
	list.AddItem("Item 1")
	list.AddNested("Item 2", nested)

	output, err := formatter.Format(list)
	require.NoError(t, err)

	var result jsonList
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Len(t, result.Items, 2)
	assert.NotNil(t, result.Items[1].Nested)
	assert.Equal(t, "unordered", result.Items[1].Nested.ListType)
	assert.Len(t, result.Items[1].Nested.Items, 2)
}

func TestJSONFormatter_FormatProgress(t *testing.T) {
	formatter := NewJSONFormatter()

	progress := format.NewProgress(format.ProgressTypeBar, 50, 100)
	progress.SetMessage("Processing")

	output, err := formatter.Format(progress)
	require.NoError(t, err)

	var result jsonProgress
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "progress", result.Type)
	assert.Equal(t, "bar", result.ProgressType)
	assert.Equal(t, 50, result.Current)
	assert.Equal(t, 100, result.Total)
	assert.Equal(t, 50.0, result.Percentage)
	assert.Equal(t, "Processing", result.Message)
}

func TestJSONFormatter_FormatStatus(t *testing.T) {
	formatter := NewJSONFormatter()

	status := format.NewStatus(format.StatusSuccess, "Operation complete")
	status.AddDetail("Step 1 done")
	status.AddDetail("Step 2 done")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	var result jsonStatus
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "status", result.Type)
	assert.Equal(t, "success", result.Severity)
	assert.Equal(t, "Operation complete", result.Message)
	assert.Equal(t, format.SymbolSuccess, result.Symbol)
	assert.Len(t, result.Details, 2)
}

func TestJSONFormatter_FormatStatusCustomSymbol(t *testing.T) {
	formatter := NewJSONFormatter()

	status := format.NewStatus(format.StatusInfo, "Custom").WithSymbol("🎉")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	var result jsonStatus
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "🎉", result.Symbol)
}

func TestJSONFormatter_FormatError(t *testing.T) {
	formatter := NewJSONFormatter()

	err := format.NewError(format.ErrorTypeValidation, "Invalid input")
	err.SetField("username")
	err.SetValue("ab")
	err.AddDetail("Must be at least 3 characters")
	err.WithSuggestion("Use a longer username")

	output, errFmt := formatter.Format(err)
	require.NoError(t, errFmt)

	var result jsonError
	errParse := json.Unmarshal([]byte(output), &result)
	require.NoError(t, errParse)

	assert.Equal(t, "error", result.Type)
	assert.Equal(t, "validation", result.ErrorType)
	assert.Equal(t, "Invalid input", result.Message)
	assert.Equal(t, "username", result.Field)
	assert.Equal(t, "ab", result.Value)
	assert.Len(t, result.Details, 1)
	assert.Equal(t, "Use a longer username", result.Suggestion)
}

func TestJSONFormatter_FormatMultiple(t *testing.T) {
	formatter := NewJSONFormatter()

	section := format.NewSection("Section")
	section.AddKeyValue("Key", "Value")

	list := format.NewList(format.ListTypeUnordered)
	list.AddItem("Item 1")

	builders := []format.Buildable{section, list}
	output, err := formatter.FormatMultiple(builders)
	require.NoError(t, err)

	// Should be a JSON array
	var result []map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "section", result[0]["type"])
	assert.Equal(t, "list", result[1]["type"])
}

func TestJSONFormatter_ValidationError(t *testing.T) {
	formatter := NewJSONFormatter()

	// Invalid section (empty title)
	section := &format.Section{Title: "", Items: []format.SectionItem{}}

	_, err := formatter.Format(section)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestJSONFormatter_ValidJSON(t *testing.T) {
	formatter := NewJSONFormatter()

	// Test that all builder types produce valid JSON
	builders := []format.Buildable{
		format.NewSection("Test").AddKeyValue("Key", "Value"),
		format.NewTable("A", "B").AddRow("1", "2"),
		format.NewList(format.ListTypeUnordered).AddItem("Item"),
		format.NewProgress(format.ProgressTypeBar, 50, 100),
		format.NewStatus(format.StatusInfo, "Message"),
		format.NewError(format.ErrorTypeValidation, "Error message"),
	}

	for _, b := range builders {
		output, err := formatter.Format(b)
		require.NoError(t, err)

		// Verify it's valid JSON
		var result map[string]any
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "builder type %s produced invalid JSON", b.Type())

		// All should have a "type" field
		assert.NotEmpty(t, result["type"])
	}
}

func TestJSONFormatter_FormatFacts(t *testing.T) {
	formatter := NewJSONFormatter()

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

	// Verify it's valid JSON
	var result types.FactsIndex
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Verify content
	assert.Len(t, result.Facts, 2)
	assert.Equal(t, "fact-1", result.Facts[0].ID)
	assert.Equal(t, "This is a test fact", result.Facts[0].Content)
	assert.Equal(t, "cli", result.Facts[0].Source)
	assert.Equal(t, 2, result.Stats.TotalFacts)
	assert.Equal(t, 50, result.Stats.MaxFacts)
}

func TestJSONFormatter_FormatFactsEmpty(t *testing.T) {
	formatter := NewJSONFormatter()

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

	var result types.FactsIndex
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Len(t, result.Facts, 0)
	assert.Equal(t, 0, result.Stats.TotalFacts)
}

func TestJSONFormatter_FormatFactsValidationError(t *testing.T) {
	formatter := NewJSONFormatter()

	fc := format.NewFactsContent(nil)
	_, err := formatter.Format(fc)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}
