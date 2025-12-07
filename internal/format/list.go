package format

import "fmt"

// MaxListDepth is the maximum allowed nesting depth for lists
const MaxListDepth = 5

// ListType represents the type of list
type ListType string

const (
	// ListTypeUnordered represents bullet lists
	ListTypeUnordered ListType = "unordered"

	// ListTypeOrdered represents numbered lists
	ListTypeOrdered ListType = "ordered"
)

// ListItem represents an item in a list
type ListItem struct {
	Content string
	Nested  *List // Optional nested list
}

// List represents ordered or unordered lists with nesting
type List struct {
	ListType  ListType
	Items     []ListItem
	IsCompact bool // Compact mode uses less spacing
}

// NewList creates a new list of the specified type
func NewList(listType ListType) *List {
	return &List{
		ListType:  listType,
		Items:     make([]ListItem, 0),
		IsCompact: false,
	}
}

// Compact enables compact mode (less spacing between items)
func (l *List) Compact() *List {
	l.IsCompact = true
	return l
}

// AddItem adds a simple item to the list
func (l *List) AddItem(content string) *List {
	l.Items = append(l.Items, ListItem{
		Content: content,
		Nested:  nil,
	})
	return l
}

// AddItemf adds a formatted item to the list
func (l *List) AddItemf(format string, args ...any) *List {
	content := fmt.Sprintf(format, args...)
	return l.AddItem(content)
}

// AddNested adds an item with a nested list
func (l *List) AddNested(content string, nested *List) *List {
	l.Items = append(l.Items, ListItem{
		Content: content,
		Nested:  nested,
	})
	return l
}

// Type returns the builder type
func (l *List) Type() BuilderType {
	return BuilderTypeList
}

// Validate checks if the list is correctly constructed
func (l *List) Validate() error {
	if l.ListType != ListTypeUnordered && l.ListType != ListTypeOrdered {
		return fmt.Errorf("invalid list type %q; must be %q or %q", l.ListType, ListTypeUnordered, ListTypeOrdered)
	}

	if len(l.Items) == 0 {
		return fmt.Errorf("list must have at least one item")
	}

	// Check for empty items and validate depth
	if err := l.validateDepth(0, make(map[*List]bool)); err != nil {
		return err
	}

	return nil
}

// validateDepth checks for circular references and enforces max depth
func (l *List) validateDepth(currentDepth int, visited map[*List]bool) error {
	// Check for circular reference
	if visited[l] {
		return fmt.Errorf("circular reference detected in list")
	}

	visited[l] = true
	defer delete(visited, l)

	if currentDepth > MaxListDepth {
		return fmt.Errorf("list nesting exceeds maximum depth %d", MaxListDepth)
	}

	for i, item := range l.Items {
		if item.Content == "" {
			return fmt.Errorf("list item %d has empty content", i)
		}

		if item.Nested != nil {
			if err := item.Nested.validateDepth(currentDepth+1, visited); err != nil {
				return err
			}
		}
	}

	return nil
}
