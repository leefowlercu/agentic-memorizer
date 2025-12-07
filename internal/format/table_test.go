package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTable(t *testing.T) {
	table := NewTable("Col1", "Col2", "Col3")

	assert.Equal(t, []string{"Col1", "Col2", "Col3"}, table.Headers)
	assert.Len(t, table.Alignments, 3)
	assert.Equal(t, AlignLeft, table.Alignments[0])
	assert.Equal(t, AlignLeft, table.Alignments[1])
	assert.Equal(t, AlignLeft, table.Alignments[2])
	assert.False(t, table.HideHeaders)
	assert.False(t, table.IsCompact)
	assert.Empty(t, table.Rows)
	assert.Equal(t, BuilderTypeTable, table.Type())
}

func TestTable_SetAlignments(t *testing.T) {
	table := NewTable("Name", "Count", "Size")
	table.SetAlignments(AlignLeft, AlignRight, AlignCenter)

	assert.Equal(t, AlignLeft, table.Alignments[0])
	assert.Equal(t, AlignRight, table.Alignments[1])
	assert.Equal(t, AlignCenter, table.Alignments[2])
}

func TestTable_HideHeader(t *testing.T) {
	table := NewTable("Col1", "Col2").HideHeader()

	assert.True(t, table.HideHeaders)
}

func TestTable_Compact(t *testing.T) {
	table := NewTable("Col1", "Col2").Compact()

	assert.True(t, table.IsCompact)
}

func TestTable_AddRow(t *testing.T) {
	table := NewTable("Name", "Age", "City")
	table.AddRow("Alice", "30", "NYC")
	table.AddRow("Bob", "25", "SF")

	require.Len(t, table.Rows, 2)
	assert.Equal(t, []string{"Alice", "30", "NYC"}, table.Rows[0])
	assert.Equal(t, []string{"Bob", "25", "SF"}, table.Rows[1])
}

func TestTable_AddRowf(t *testing.T) {
	table := NewTable("Name", "Count", "Size")
	table.AddRowf(
		[]string{"%s", "%d", "%.1f MB"},
		[][]any{{"File1"}, {42}, {15.5}},
	)

	require.Len(t, table.Rows, 1)
	assert.Equal(t, "File1", table.Rows[0][0])
	assert.Equal(t, "42", table.Rows[0][1])
	assert.Equal(t, "15.5 MB", table.Rows[0][2])
}

func TestTable_FluentAPI(t *testing.T) {
	table := NewTable("Name", "Count").
		SetAlignments(AlignLeft, AlignRight).
		HideHeader().
		Compact().
		AddRow("Item1", "10").
		AddRow("Item2", "20")

	assert.True(t, table.HideHeaders)
	assert.True(t, table.IsCompact)
	assert.Len(t, table.Rows, 2)
	assert.Equal(t, AlignRight, table.Alignments[1])
}

func TestTable_ValidateNoHeaders(t *testing.T) {
	table := &Table{Headers: []string{}, Rows: [][]string{}}

	err := table.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must have at least one header")
}

func TestTable_ValidateMismatchedAlignments(t *testing.T) {
	table := NewTable("Col1", "Col2", "Col3")
	table.Alignments = []Alignment{AlignLeft} // Only 1 alignment for 3 columns

	err := table.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alignment count")
}

func TestTable_ValidateMismatchedColumns(t *testing.T) {
	table := NewTable("Col1", "Col2", "Col3")
	table.AddRow("A", "B", "C") // Valid
	table.AddRow("D", "E")      // Invalid - only 2 columns

	err := table.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "row 1 has 2 columns, expected 3")
}

func TestTable_ValidateValidTable(t *testing.T) {
	table := NewTable("Name", "Age", "City")
	table.AddRow("Alice", "30", "NYC")
	table.AddRow("Bob", "25", "SF")

	err := table.Validate()
	assert.NoError(t, err)
}

func TestTable_ValidateEmptyTable(t *testing.T) {
	table := NewTable("Col1", "Col2")

	err := table.Validate()
	assert.NoError(t, err) // Empty table with headers is valid
}
