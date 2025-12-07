package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewList(t *testing.T) {
	unordered := NewList(ListTypeUnordered)
	assert.Equal(t, ListTypeUnordered, unordered.ListType)
	assert.Empty(t, unordered.Items)
	assert.False(t, unordered.IsCompact)
	assert.Equal(t, BuilderTypeList, unordered.Type())

	ordered := NewList(ListTypeOrdered)
	assert.Equal(t, ListTypeOrdered, ordered.ListType)
}

func TestList_AddItem(t *testing.T) {
	list := NewList(ListTypeUnordered)
	list.AddItem("Item 1")
	list.AddItem("Item 2")

	require.Len(t, list.Items, 2)
	assert.Equal(t, "Item 1", list.Items[0].Content)
	assert.Equal(t, "Item 2", list.Items[1].Content)
	assert.Nil(t, list.Items[0].Nested)
}

func TestList_AddItemf(t *testing.T) {
	list := NewList(ListTypeUnordered)
	list.AddItemf("Found %d files", 42)
	list.AddItemf("Size: %.1f MB", 15.5)

	require.Len(t, list.Items, 2)
	assert.Equal(t, "Found 42 files", list.Items[0].Content)
	assert.Equal(t, "Size: 15.5 MB", list.Items[1].Content)
}

func TestList_AddNested(t *testing.T) {
	nested := NewList(ListTypeUnordered)
	nested.AddItem("Nested Item 1")
	nested.AddItem("Nested Item 2")

	list := NewList(ListTypeOrdered)
	list.AddNested("Main Item with nested list", nested)

	require.Len(t, list.Items, 1)
	assert.Equal(t, "Main Item with nested list", list.Items[0].Content)
	assert.NotNil(t, list.Items[0].Nested)
	assert.Len(t, list.Items[0].Nested.Items, 2)
}

func TestList_Compact(t *testing.T) {
	list := NewList(ListTypeUnordered).Compact()

	assert.True(t, list.IsCompact)
}

func TestList_FluentAPI(t *testing.T) {
	list := NewList(ListTypeOrdered).
		Compact().
		AddItem("Item 1").
		AddItemf("Item %d", 2).
		AddItem("Item 3")

	assert.True(t, list.IsCompact)
	assert.Len(t, list.Items, 3)
	assert.Equal(t, "Item 2", list.Items[1].Content)
}

func TestList_ValidateInvalidType(t *testing.T) {
	list := &List{
		ListType: "invalid",
		Items:    []ListItem{{Content: "Item"}},
	}

	err := list.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid list type")
}

func TestList_ValidateEmptyList(t *testing.T) {
	list := NewList(ListTypeUnordered)

	err := list.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must have at least one item")
}

func TestList_ValidateEmptyItem(t *testing.T) {
	list := NewList(ListTypeUnordered)
	list.AddItem("Valid Item")
	list.Items = append(list.Items, ListItem{Content: ""})

	err := list.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has empty content")
}

func TestList_ValidateMaxDepth(t *testing.T) {
	// Create deeply nested list at max depth
	list := NewList(ListTypeUnordered)
	current := list

	for i := 0; i < MaxListDepth; i++ {
		nested := NewList(ListTypeUnordered)
		current.AddNested("Nested item", nested)
		current = nested
	}

	// At max depth, add an item to the deepest list
	current.AddItem("Final item at max depth")

	// This should be valid (exactly at max depth)
	err := list.Validate()
	assert.NoError(t, err)

	// Now exceed max depth by adding another level
	tooDeep := NewList(ListTypeUnordered)
	tooDeep.AddItem("Too deep")
	current.AddNested("Beyond max", tooDeep)

	err = list.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum depth")
}

func TestList_ValidateCircularReference(t *testing.T) {
	list1 := NewList(ListTypeUnordered)
	list2 := NewList(ListTypeUnordered)

	list1.AddNested("Item with list2", list2)
	list2.AddNested("Item with list1", list1)

	err := list1.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular reference")
}

func TestList_ValidateValid(t *testing.T) {
	list := NewList(ListTypeOrdered)
	list.AddItem("Item 1")
	list.AddItem("Item 2")

	nested := NewList(ListTypeUnordered)
	nested.AddItem("Nested A")
	nested.AddItem("Nested B")
	list.AddNested("Item 3 with nested", nested)

	err := list.Validate()
	assert.NoError(t, err)
}
