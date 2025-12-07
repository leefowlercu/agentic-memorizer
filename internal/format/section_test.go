package format

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSection(t *testing.T) {
	section := NewSection("Test Section")

	assert.Equal(t, "Test Section", section.Title)
	assert.Equal(t, 0, section.Level)
	assert.False(t, section.WithDivider)
	assert.Empty(t, section.Items)
	assert.Equal(t, BuilderTypeSection, section.Type())
}

func TestSection_SetLevel(t *testing.T) {
	section := NewSection("Test").SetLevel(2)

	assert.Equal(t, 2, section.Level)
}

func TestSection_AddDivider(t *testing.T) {
	section := NewSection("Test").AddDivider()

	assert.True(t, section.WithDivider)
}

func TestSection_AddKeyValue(t *testing.T) {
	section := NewSection("Test")
	section.AddKeyValue("Key1", "Value1")
	section.AddKeyValue("Key2", "Value2")

	require.Len(t, section.Items, 2)
	assert.Equal(t, SectionItemKeyValue, section.Items[0].Type)
	assert.Equal(t, "Key1", section.Items[0].Key)
	assert.Equal(t, "Value1", section.Items[0].Value)
	assert.Equal(t, "Key2", section.Items[1].Key)
	assert.Equal(t, "Value2", section.Items[1].Value)
}

func TestSection_AddKeyValuef(t *testing.T) {
	section := NewSection("Test")
	section.AddKeyValuef("Count", "%d files", 42)
	section.AddKeyValuef("Size", "%.1f MB", 15.5)

	require.Len(t, section.Items, 2)
	assert.Equal(t, "42 files", section.Items[0].Value)
	assert.Equal(t, "15.5 MB", section.Items[1].Value)
}

func TestSection_AddSubsection(t *testing.T) {
	subsection := NewSection("Subsection")
	subsection.AddKeyValue("SubKey", "SubValue")

	section := NewSection("Main")
	section.AddSubsection(subsection)

	require.Len(t, section.Items, 1)
	assert.Equal(t, SectionItemSubsection, section.Items[0].Type)
	assert.Equal(t, subsection, section.Items[0].Subsection)
}

func TestSection_FluentAPI(t *testing.T) {
	section := NewSection("Fluent Test").
		SetLevel(1).
		AddDivider().
		AddKeyValue("Key1", "Value1").
		AddKeyValue("Key2", "Value2")

	assert.Equal(t, "Fluent Test", section.Title)
	assert.Equal(t, 1, section.Level)
	assert.True(t, section.WithDivider)
	assert.Len(t, section.Items, 2)
}

func TestSection_ValidateEmptyTitle(t *testing.T) {
	section := &Section{Title: "", Level: 0, Items: []SectionItem{}}

	err := section.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "section title cannot be empty")
}

func TestSection_ValidateNegativeLevel(t *testing.T) {
	section := NewSection("Test").SetLevel(-1)

	err := section.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "section level cannot be negative")
}

func TestSection_ValidateMaxDepth(t *testing.T) {
	section := NewSection("Test").SetLevel(MaxSectionDepth + 1)

	err := section.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum depth")
}

func TestSection_ValidateNestedDepth(t *testing.T) {
	// Create deeply nested sections
	section := NewSection("Level 0").SetLevel(0)
	current := section

	for i := 1; i <= MaxSectionDepth; i++ {
		sub := NewSection(fmt.Sprintf("Level %d", i)).SetLevel(i)
		current.AddSubsection(sub)
		current = sub
	}

	// At max depth, validation should pass
	err := section.Validate()
	assert.NoError(t, err)

	// Add one more level to exceed max depth
	tooDeep := NewSection("Too Deep").SetLevel(MaxSectionDepth + 1)
	current.AddSubsection(tooDeep)

	err = section.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum depth")
}

func TestSection_ValidateCircularReference(t *testing.T) {
	section1 := NewSection("Section 1")
	section2 := NewSection("Section 2")

	// Create circular reference: section1 -> section2 -> section1
	section1.AddSubsection(section2)
	section2.AddSubsection(section1)

	err := section1.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular reference detected")
}

func TestSection_ValidateNilSubsection(t *testing.T) {
	section := NewSection("Test")
	section.Items = append(section.Items, SectionItem{
		Type:       SectionItemSubsection,
		Subsection: nil,
	})

	err := section.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subsection is nil")
}

func TestSection_ValidateValidSection(t *testing.T) {
	section := NewSection("Main Section")
	section.AddKeyValue("Key1", "Value1")
	section.AddKeyValue("Key2", "Value2")

	subsection := NewSection("Subsection")
	subsection.AddKeyValue("SubKey", "SubValue")
	section.AddSubsection(subsection)

	err := section.Validate()
	assert.NoError(t, err)
}
