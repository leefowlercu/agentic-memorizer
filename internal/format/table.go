package format

import "fmt"

// Alignment represents text alignment in table columns
type Alignment string

const (
	// AlignLeft aligns text to the left
	AlignLeft Alignment = "left"

	// AlignRight aligns text to the right
	AlignRight Alignment = "right"

	// AlignCenter centers text
	AlignCenter Alignment = "center"
)

// Table represents columnar data with headers and alignment
type Table struct {
	Headers     []string
	Rows        [][]string
	Alignments  []Alignment
	HideHeaders bool
	IsCompact   bool // Compact mode uses less spacing
}

// NewTable creates a new table with the given headers
func NewTable(headers ...string) *Table {
	alignments := make([]Alignment, len(headers))
	for i := range alignments {
		alignments[i] = AlignLeft // Default to left alignment
	}

	return &Table{
		Headers:     headers,
		Rows:        make([][]string, 0),
		Alignments:  alignments,
		HideHeaders: false,
		IsCompact:   false,
	}
}

// SetAlignments sets the column alignments
func (t *Table) SetAlignments(alignments ...Alignment) *Table {
	if len(alignments) > 0 {
		t.Alignments = alignments
	}
	return t
}

// HideHeader hides the table header row
func (t *Table) HideHeader() *Table {
	t.HideHeaders = true
	return t
}

// Compact enables compact mode (less spacing)
func (t *Table) Compact() *Table {
	t.IsCompact = true
	return t
}

// AddRow adds a data row to the table
func (t *Table) AddRow(cells ...string) *Table {
	t.Rows = append(t.Rows, cells)
	return t
}

// AddRowf adds a formatted row to the table
func (t *Table) AddRowf(formats []string, values [][]any) *Table {
	if len(formats) != len(values) {
		// Skip malformed rows
		return t
	}

	cells := make([]string, len(formats))
	for i, format := range formats {
		cells[i] = fmt.Sprintf(format, values[i]...)
	}

	return t.AddRow(cells...)
}

// Type returns the builder type
func (t *Table) Type() BuilderType {
	return BuilderTypeTable
}

// Validate checks if the table is correctly constructed
func (t *Table) Validate() error {
	if len(t.Headers) == 0 {
		return fmt.Errorf("table must have at least one header")
	}

	// Check alignments match header count
	if len(t.Alignments) != len(t.Headers) {
		return fmt.Errorf("alignment count (%d) must match header count (%d)", len(t.Alignments), len(t.Headers))
	}

	// Check all rows have the correct number of columns
	for i, row := range t.Rows {
		if len(row) != len(t.Headers) {
			return fmt.Errorf("row %d has %d columns, expected %d", i, len(row), len(t.Headers))
		}
	}

	return nil
}
