package formatters

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewXMLFormatter(t *testing.T) {
	formatter := NewXMLFormatter()
	assert.Equal(t, "xml", formatter.Name())
	assert.False(t, formatter.SupportsColors())
}

func TestXMLFormatter_FormatSection(t *testing.T) {
	formatter := NewXMLFormatter()

	section := format.NewSection("Test Section")
	section.AddKeyValue("Name", "John")
	section.AddKeyValue("Age", "30")

	output, err := formatter.Format(section)
	require.NoError(t, err)

	// Unmarshal and verify structure
	var result xmlSection
	err = xml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "Test Section", result.Title)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "key_value", result.Items[0].Type)
	assert.Equal(t, "Name", result.Items[0].Key)
	assert.Equal(t, "John", result.Items[0].Value)
}

func TestXMLFormatter_FormatSectionNested(t *testing.T) {
	formatter := NewXMLFormatter()

	subsection := format.NewSection("Sub")
	subsection.AddKeyValue("Detail", "Value")

	section := format.NewSection("Main")
	section.AddKeyValue("Field", "Data")
	section.AddSubsection(subsection)

	output, err := formatter.Format(section)
	require.NoError(t, err)

	var result xmlSection
	err = xml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "Main", result.Title)
	assert.Len(t, result.Items, 2)

	// Check subsection
	assert.Equal(t, "subsection", result.Items[1].Type)
	assert.NotNil(t, result.Items[1].Subsection)
	assert.Equal(t, "Sub", result.Items[1].Subsection.Title)
}

func TestXMLFormatter_FormatTable(t *testing.T) {
	formatter := NewXMLFormatter()

	table := format.NewTable("Name", "Age", "City")
	table.SetAlignments(format.AlignLeft, format.AlignRight, format.AlignCenter)
	table.AddRow("Alice", "30", "NYC")
	table.AddRow("Bob", "25", "SF")

	output, err := formatter.Format(table)
	require.NoError(t, err)

	var result xmlTable
	err = xml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, []string{"Name", "Age", "City"}, result.Headers)
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, []string{"Alice", "30", "NYC"}, result.Rows[0].Cells)
	assert.Equal(t, []string{"left", "right", "center"}, result.Alignments)
}

func TestXMLFormatter_FormatList(t *testing.T) {
	formatter := NewXMLFormatter()

	list := format.NewList(format.ListTypeOrdered)
	list.AddItem("First")
	list.AddItem("Second")
	list.AddItem("Third")

	output, err := formatter.Format(list)
	require.NoError(t, err)

	var result xmlList
	err = xml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "ordered", result.ListType)
	assert.Len(t, result.Items, 3)
	assert.Equal(t, "First", result.Items[0].Content)
	assert.Equal(t, "Second", result.Items[1].Content)
	assert.Equal(t, "Third", result.Items[2].Content)
}

func TestXMLFormatter_FormatListNested(t *testing.T) {
	formatter := NewXMLFormatter()

	nested := format.NewList(format.ListTypeUnordered)
	nested.AddItem("Nested A")
	nested.AddItem("Nested B")

	list := format.NewList(format.ListTypeOrdered)
	list.AddItem("Item 1")
	list.AddNested("Item 2", nested)

	output, err := formatter.Format(list)
	require.NoError(t, err)

	var result xmlList
	err = xml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Len(t, result.Items, 2)
	assert.NotNil(t, result.Items[1].Nested)
	assert.Equal(t, "unordered", result.Items[1].Nested.ListType)
	assert.Len(t, result.Items[1].Nested.Items, 2)
}

func TestXMLFormatter_FormatProgress(t *testing.T) {
	formatter := NewXMLFormatter()

	progress := format.NewProgress(format.ProgressTypeBar, 50, 100)
	progress.SetMessage("Processing")

	output, err := formatter.Format(progress)
	require.NoError(t, err)

	var result xmlProgress
	err = xml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "bar", result.ProgressType)
	assert.Equal(t, 50, result.Current)
	assert.Equal(t, 100, result.Total)
	assert.Equal(t, 50.0, result.Percentage)
	assert.Equal(t, "Processing", result.Message)
}

func TestXMLFormatter_FormatStatus(t *testing.T) {
	formatter := NewXMLFormatter()

	status := format.NewStatus(format.StatusSuccess, "Operation complete")
	status.AddDetail("Step 1 done")
	status.AddDetail("Step 2 done")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	var result xmlStatus
	err = xml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "success", result.Severity)
	assert.Equal(t, "Operation complete", result.Message)
	assert.Equal(t, format.SymbolSuccess, result.Symbol)
	assert.Len(t, result.Details, 2)
}

func TestXMLFormatter_FormatStatusCustomSymbol(t *testing.T) {
	formatter := NewXMLFormatter()

	status := format.NewStatus(format.StatusInfo, "Custom").WithSymbol("🎉")

	output, err := formatter.Format(status)
	require.NoError(t, err)

	var result xmlStatus
	err = xml.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "🎉", result.Symbol)
}

func TestXMLFormatter_FormatError(t *testing.T) {
	formatter := NewXMLFormatter()

	err := format.NewError(format.ErrorTypeValidation, "Invalid input")
	err.SetField("username")
	err.SetValue("ab")
	err.AddDetail("Must be at least 3 characters")
	err.WithSuggestion("Use a longer username")

	output, errFmt := formatter.Format(err)
	require.NoError(t, errFmt)

	var result xmlError
	errParse := xml.Unmarshal([]byte(output), &result)
	require.NoError(t, errParse)

	assert.Equal(t, "validation", result.ErrorType)
	assert.Equal(t, "Invalid input", result.Message)
	assert.Equal(t, "username", result.Field)
	assert.Equal(t, "ab", result.Value)
	assert.Len(t, result.Details, 1)
	assert.Equal(t, "Use a longer username", result.Suggestion)
}

func TestXMLFormatter_FormatMultiple(t *testing.T) {
	formatter := NewXMLFormatter()

	section := format.NewSection("Section")
	section.AddKeyValue("Key", "Value")

	list := format.NewList(format.ListTypeUnordered)
	list.AddItem("Item 1")

	builders := []format.Buildable{section, list}
	output, err := formatter.FormatMultiple(builders)
	require.NoError(t, err)

	// Should be wrapped in <output> root element
	assert.Contains(t, output, "<output>")
	assert.Contains(t, output, "</output>")
	assert.Contains(t, output, "<section")
	assert.Contains(t, output, "<list")
}

func TestXMLFormatter_ValidationError(t *testing.T) {
	formatter := NewXMLFormatter()

	// Invalid section (empty title)
	section := &format.Section{Title: "", Items: []format.SectionItem{}}

	_, err := formatter.Format(section)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestXMLFormatter_ValidXML(t *testing.T) {
	formatter := NewXMLFormatter()

	// Test that all builder types produce valid XML
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

		// Verify it starts with XML header
		assert.True(t, strings.HasPrefix(output, xml.Header), "builder type %s missing XML header", b.Type())

		// Verify it's valid XML by unmarshaling
		var result any
		err = xml.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "builder type %s produced invalid XML", b.Type())
	}
}
