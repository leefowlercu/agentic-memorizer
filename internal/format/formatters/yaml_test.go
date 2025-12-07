package formatters

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewYAMLFormatter(t *testing.T) {
	formatter := NewYAMLFormatter()
	assert.Equal(t, "yaml", formatter.Name())
	assert.False(t, formatter.SupportsColors())
}

func TestYAMLFormatter_FormatSection(t *testing.T) {
	formatter := NewYAMLFormatter()

	section := format.NewSection("Test Section")
	section.AddKeyValue("Name", "John")
	section.AddKeyValue("Age", "30")

	output, err := formatter.Format(section)
	require.NoError(t, err)

	// Unmarshal and verify structure
	var result yamlSection
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "section", result.Type)
	assert.Equal(t, "Test Section", result.Title)
	assert.Equal(t, "John", result.Fields["Name"])
	assert.Equal(t, "30", result.Fields["Age"])
	assert.Len(t, result.Items, 2)
}

func TestYAMLFormatter_FormatSectionNested(t *testing.T) {
	formatter := NewYAMLFormatter()

	subsection := format.NewSection("Sub")
	subsection.AddKeyValue("Detail", "Value")

	section := format.NewSection("Main")
	section.AddKeyValue("Field", "Data")
	section.AddSubsection(subsection)

	output, err := formatter.Format(section)
	require.NoError(t, err)

	var result yamlSection
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "Main", result.Title)
	assert.Len(t, result.Items, 2)

	// Check subsection
	assert.Equal(t, "subsection", result.Items[1].Type)
	assert.NotNil(t, result.Items[1].Subsection)
	assert.Equal(t, "Sub", result.Items[1].Subsection.Title)
}

func TestYAMLFormatter_FormatTable(t *testing.T) {
	formatter := NewYAMLFormatter()

	table := format.NewTable("Name", "Age", "City")
	table.SetAlignments(format.AlignLeft, format.AlignRight, format.AlignCenter)
	table.AddRow("Alice", "30", "NYC")
	table.AddRow("Bob", "25", "SF")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	var result yamlTable
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "table", result.Type)
	assert.Equal(t, []string{"Name", "Age", "City"}, result.Headers)
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, []string{"Alice", "30", "NYC"}, result.Rows[0])
	assert.Equal(t, []string{"left", "right", "center"}, result.Alignments)
}

func TestYAMLFormatter_FormatList(t *testing.T) {
	formatter := NewYAMLFormatter()

	list := format.NewList(format.ListTypeOrdered)
	list.AddItem("First")
	list.AddItem("Second")
	list.AddItem("Third")

	output, err := formatter.Format(list)
	require.NoError(t, err)

	var result yamlList
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "list", result.Type)
	assert.Equal(t, "ordered", result.ListType)
	assert.Len(t, result.Items, 3)
	assert.Equal(t, "First", result.Items[0].Content)
	assert.Equal(t, "Second", result.Items[1].Content)
	assert.Equal(t, "Third", result.Items[2].Content)
}

func TestYAMLFormatter_FormatListNested(t *testing.T) {
	formatter := NewYAMLFormatter()

	nested := format.NewList(format.ListTypeUnordered)
	nested.AddItem("Nested A")
	nested.AddItem("Nested B")

	list := format.NewList(format.ListTypeOrdered)
	list.AddItem("Item 1")
	list.AddNested("Item 2", nested)

	output, err := formatter.Format(list)
	require.NoError(t, err)

	var result yamlList
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Len(t, result.Items, 2)
	assert.NotNil(t, result.Items[1].Nested)
	assert.Equal(t, "unordered", result.Items[1].Nested.ListType)
	assert.Len(t, result.Items[1].Nested.Items, 2)
}

func TestYAMLFormatter_FormatProgress(t *testing.T) {
	formatter := NewYAMLFormatter()

	progress := format.NewProgress(format.ProgressTypeBar, 50, 100)
	progress.SetMessage("Processing")

	output, err := formatter.Format(progress)
	require.NoError(t, err)

	var result yamlProgress
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "progress", result.Type)
	assert.Equal(t, "bar", result.ProgressType)
	assert.Equal(t, 50, result.Current)
	assert.Equal(t, 100, result.Total)
	assert.Equal(t, 50.0, result.Percentage)
	assert.Equal(t, "Processing", result.Message)
}

func TestYAMLFormatter_FormatStatus(t *testing.T) {
	formatter := NewYAMLFormatter()

	status := format.NewStatus(format.StatusSuccess, "Operation complete")
	status.AddDetail("Step 1 done")
	status.AddDetail("Step 2 done")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	var result yamlStatus
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "status", result.Type)
	assert.Equal(t, "success", result.Severity)
	assert.Equal(t, "Operation complete", result.Message)
	assert.Equal(t, format.SymbolSuccess, result.Symbol)
	assert.Len(t, result.Details, 2)
}

func TestYAMLFormatter_FormatStatusCustomSymbol(t *testing.T) {
	formatter := NewYAMLFormatter()

	status := format.NewStatus(format.StatusInfo, "Custom").WithSymbol("🎉")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	var result yamlStatus
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "🎉", result.Symbol)
}

func TestYAMLFormatter_FormatError(t *testing.T) {
	formatter := NewYAMLFormatter()

	err := format.NewError(format.ErrorTypeValidation, "Invalid input")
	err.SetField("username")
	err.SetValue("ab")
	err.AddDetail("Must be at least 3 characters")
	err.WithSuggestion("Use a longer username")

	output, errFmt := formatter.Format(err)
	require.NoError(t, errFmt)

	var result yamlError
	errParse := yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, errParse)

	assert.Equal(t, "error", result.Type)
	assert.Equal(t, "validation", result.ErrorType)
	assert.Equal(t, "Invalid input", result.Message)
	assert.Equal(t, "username", result.Field)
	assert.Equal(t, "ab", result.Value)
	assert.Len(t, result.Details, 1)
	assert.Equal(t, "Use a longer username", result.Suggestion)
}

func TestYAMLFormatter_FormatMultiple(t *testing.T) {
	formatter := NewYAMLFormatter()

	section := format.NewSection("Section")
	section.AddKeyValue("Key", "Value")

	list := format.NewList(format.ListTypeUnordered)
	list.AddItem("Item 1")

	builders := []format.Buildable{section, list}
	output, err := formatter.FormatMultiple(builders)
	require.NoError(t, err)

	// Should be a YAML array
	var result []map[string]any
	err = yaml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "section", result[0]["type"])
	assert.Equal(t, "list", result[1]["type"])
}

func TestYAMLFormatter_ValidationError(t *testing.T) {
	formatter := NewYAMLFormatter()

	// Invalid section (empty title)
	section := &format.Section{Title: "", Items: []format.SectionItem{}}

	_, err := formatter.Format(section)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestYAMLFormatter_ValidYAML(t *testing.T) {
	formatter := NewYAMLFormatter()

	// Test that all builder types produce valid YAML
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

		// Verify it's valid YAML
		var result map[string]any
		err = yaml.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "builder type %s produced invalid YAML", b.Type())

		// All should have a "type" field
		assert.NotEmpty(t, result["type"])
	}
}
