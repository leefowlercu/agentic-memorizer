package format

import "fmt"

// MaxSectionDepth is the maximum allowed nesting depth for sections
const MaxSectionDepth = 5

// SectionItemType represents the type of item in a section
type SectionItemType string

const (
	// SectionItemKeyValue represents a key-value pair
	SectionItemKeyValue SectionItemType = "key_value"

	// SectionItemSubsection represents a nested subsection
	SectionItemSubsection SectionItemType = "subsection"

	// SectionItemText represents a plain text line
	SectionItemText SectionItemType = "text"
)

// SectionItem represents an item in a section
type SectionItem struct {
	Type       SectionItemType
	Key        string
	Value      string
	Text       string   // For SectionItemText type
	Subsection *Section
}

// Section represents hierarchical key-value pairs with headers
type Section struct {
	Title       string
	Level       int           // 0 = main section, 1 = subsection, etc.
	WithDivider bool          // Whether to show a divider line under title
	Items       []SectionItem
}

// NewSection creates a new section with the given title
func NewSection(title string) *Section {
	return &Section{
		Title:       title,
		Level:       0,
		WithDivider: false,
		Items:       make([]SectionItem, 0),
	}
}

// SetLevel sets the section level (0 = top level, 1 = subsection, etc.)
func (s *Section) SetLevel(level int) *Section {
	s.Level = level
	return s
}

// WithDivider adds a divider line under the section title
func (s *Section) AddDivider() *Section {
	s.WithDivider = true
	return s
}

// AddKeyValue adds a key-value pair to the section
func (s *Section) AddKeyValue(key, value string) *Section {
	s.Items = append(s.Items, SectionItem{
		Type:  SectionItemKeyValue,
		Key:   key,
		Value: value,
	})
	return s
}

// AddKeyValuef adds a formatted key-value pair to the section
func (s *Section) AddKeyValuef(key, format string, args ...any) *Section {
	value := fmt.Sprintf(format, args...)
	return s.AddKeyValue(key, value)
}

// AddSubsection adds a nested subsection
func (s *Section) AddSubsection(sub *Section) *Section {
	s.Items = append(s.Items, SectionItem{
		Type:       SectionItemSubsection,
		Subsection: sub,
	})
	return s
}

// AddTextLine adds a plain text line to the section
func (s *Section) AddTextLine(text string) *Section {
	s.Items = append(s.Items, SectionItem{
		Type: SectionItemText,
		Text: text,
	})
	return s
}

// Type returns the builder type
func (s *Section) Type() BuilderType {
	return BuilderTypeSection
}

// Validate checks if the section is correctly constructed
func (s *Section) Validate() error {
	if s.Title == "" {
		return fmt.Errorf("section title cannot be empty")
	}

	if s.Level < 0 {
		return fmt.Errorf("section level cannot be negative; got %d", s.Level)
	}

	if s.Level > MaxSectionDepth {
		return fmt.Errorf("section level exceeds maximum depth %d; got %d", MaxSectionDepth, s.Level)
	}

	// Check for circular references and max depth
	if err := s.validateDepth(s.Level, make(map[*Section]bool)); err != nil {
		return err
	}

	return nil
}

// validateDepth checks for circular references and enforces max depth
func (s *Section) validateDepth(currentDepth int, visited map[*Section]bool) error {
	// Check for circular reference
	if visited[s] {
		return fmt.Errorf("circular reference detected in section %q", s.Title)
	}

	visited[s] = true
	defer delete(visited, s)

	for _, item := range s.Items {
		if item.Type == SectionItemSubsection {
			if item.Subsection == nil {
				return fmt.Errorf("subsection is nil in section %q", s.Title)
			}

			nextDepth := currentDepth + 1
			if nextDepth > MaxSectionDepth {
				return fmt.Errorf("section nesting exceeds maximum depth %d", MaxSectionDepth)
			}

			if err := item.Subsection.validateDepth(nextDepth, visited); err != nil {
				return err
			}
		}
	}

	return nil
}
