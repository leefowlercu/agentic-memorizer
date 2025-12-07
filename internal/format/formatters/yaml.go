package formatters

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"gopkg.in/yaml.v3"
)

// YAMLFormatter renders output as YAML
type YAMLFormatter struct{}

// NewYAMLFormatter creates a new YAML formatter
func NewYAMLFormatter() *YAMLFormatter {
	return &YAMLFormatter{}
}

// Format renders a single buildable structure
func (f *YAMLFormatter) Format(b format.Buildable) (string, error) {
	if err := b.Validate(); err != nil {
		return "", fmt.Errorf("validation failed; %w", err)
	}

	var data any

	switch v := b.(type) {
	case *format.Section:
		data = f.sectionToYAML(v)
	case *format.Table:
		data = f.tableToYAML(v)
	case *format.List:
		data = f.listToYAML(v)
	case *format.Progress:
		data = f.progressToYAML(v)
	case *format.Status:
		data = f.statusToYAML(v)
	case *format.Error:
		data = f.errorToYAML(v)
	case *format.GraphContent:
		// GraphContent marshals the GraphIndex directly
		return f.formatGraph(v)
	default:
		return "", fmt.Errorf("unsupported builder type: %s", b.Type())
	}

	return f.marshal(data)
}

// FormatMultiple renders multiple buildable structures
func (f *YAMLFormatter) FormatMultiple(builders []format.Buildable) (string, error) {
	items := make([]any, 0, len(builders))

	for _, b := range builders {
		if err := b.Validate(); err != nil {
			return "", fmt.Errorf("validation failed; %w", err)
		}

		switch v := b.(type) {
		case *format.Section:
			items = append(items, f.sectionToYAML(v))
		case *format.Table:
			items = append(items, f.tableToYAML(v))
		case *format.List:
			items = append(items, f.listToYAML(v))
		case *format.Progress:
			items = append(items, f.progressToYAML(v))
		case *format.Status:
			items = append(items, f.statusToYAML(v))
		case *format.Error:
			items = append(items, f.errorToYAML(v))
		default:
			return "", fmt.Errorf("unsupported builder type: %s", b.Type())
		}
	}

	return f.marshal(items)
}

// Name returns the formatter name
func (f *YAMLFormatter) Name() string {
	return "yaml"
}

// SupportsColors returns false (YAML doesn't support colors)
func (f *YAMLFormatter) SupportsColors() bool {
	return false
}

// YAML structure types (reuse JSON types with yaml tags)

type yamlSection struct {
	Type   string                 `yaml:"type"`
	Title  string                 `yaml:"title"`
	Level  int                    `yaml:"level,omitempty"`
	Fields map[string]string      `yaml:"fields,omitempty"`
	Items  []yamlSectionItem      `yaml:"items,omitempty"`
}

type yamlSectionItem struct {
	Type       string       `yaml:"type"`
	Key        string       `yaml:"key,omitempty"`
	Value      string       `yaml:"value,omitempty"`
	Subsection *yamlSection `yaml:"subsection,omitempty"`
}

type yamlTable struct {
	Type       string     `yaml:"type"`
	Headers    []string   `yaml:"headers"`
	Rows       [][]string `yaml:"rows"`
	Alignments []string   `yaml:"alignments,omitempty"`
	Compact    bool       `yaml:"compact,omitempty"`
}

type yamlList struct {
	Type     string         `yaml:"type"`
	ListType string         `yaml:"list_type"`
	Items    []yamlListItem `yaml:"items"`
	Compact  bool           `yaml:"compact,omitempty"`
}

type yamlListItem struct {
	Content string    `yaml:"content"`
	Nested  *yamlList `yaml:"nested,omitempty"`
}

type yamlProgress struct {
	Type         string  `yaml:"type"`
	ProgressType string  `yaml:"progress_type"`
	Current      int     `yaml:"current"`
	Total        int     `yaml:"total"`
	Percentage   float64 `yaml:"percentage"`
	Message      string  `yaml:"message,omitempty"`
}

type yamlStatus struct {
	Type     string   `yaml:"type"`
	Severity string   `yaml:"severity"`
	Message  string   `yaml:"message"`
	Symbol   string   `yaml:"symbol"`
	Details  []string `yaml:"details,omitempty"`
}

type yamlError struct {
	Type       string   `yaml:"type"`
	ErrorType  string   `yaml:"error_type"`
	Message    string   `yaml:"message"`
	Field      string   `yaml:"field,omitempty"`
	Value      any      `yaml:"value,omitempty"`
	Details    []string `yaml:"details,omitempty"`
	Suggestion string   `yaml:"suggestion,omitempty"`
}

// Conversion methods

func (f *YAMLFormatter) sectionToYAML(s *format.Section) *yamlSection {
	result := &yamlSection{
		Type:   "section",
		Title:  s.Title,
		Level:  s.Level,
		Fields: make(map[string]string),
		Items:  make([]yamlSectionItem, 0),
	}

	for _, item := range s.Items {
		if item.Type == format.SectionItemKeyValue {
			result.Fields[item.Key] = item.Value
			result.Items = append(result.Items, yamlSectionItem{
				Type:  "key_value",
				Key:   item.Key,
				Value: item.Value,
			})
		} else if item.Type == format.SectionItemSubsection {
			result.Items = append(result.Items, yamlSectionItem{
				Type:       "subsection",
				Subsection: f.sectionToYAML(item.Subsection),
			})
		}
	}

	return result
}

func (f *YAMLFormatter) tableToYAML(t *format.Table) *yamlTable {
	alignments := make([]string, len(t.Alignments))
	for i, a := range t.Alignments {
		alignments[i] = string(a)
	}

	return &yamlTable{
		Type:       "table",
		Headers:    t.Headers,
		Rows:       t.Rows,
		Alignments: alignments,
		Compact:    t.IsCompact,
	}
}

func (f *YAMLFormatter) listToYAML(l *format.List) *yamlList {
	items := make([]yamlListItem, len(l.Items))
	for i, item := range l.Items {
		items[i] = yamlListItem{
			Content: item.Content,
		}
		if item.Nested != nil {
			items[i].Nested = f.listToYAML(item.Nested)
		}
	}

	return &yamlList{
		Type:     "list",
		ListType: string(l.ListType),
		Items:    items,
		Compact:  l.IsCompact,
	}
}

func (f *YAMLFormatter) progressToYAML(p *format.Progress) *yamlProgress {
	return &yamlProgress{
		Type:         "progress",
		ProgressType: string(p.ProgressType),
		Current:      p.Current,
		Total:        p.Total,
		Percentage:   p.Percentage(),
		Message:      p.Message,
	}
}

func (f *YAMLFormatter) statusToYAML(s *format.Status) *yamlStatus {
	symbol := s.CustomSymbol
	if symbol == "" {
		symbol = format.GetStatusSymbol(s.Severity)
	}

	return &yamlStatus{
		Type:     "status",
		Severity: string(s.Severity),
		Message:  s.Message,
		Symbol:   symbol,
		Details:  s.Details,
	}
}

func (f *YAMLFormatter) errorToYAML(e *format.Error) *yamlError {
	return &yamlError{
		Type:       "error",
		ErrorType:  string(e.ErrorType),
		Message:    e.Message,
		Field:      e.Field,
		Value:      e.Value,
		Details:    e.Details,
		Suggestion: e.Suggestion,
	}
}

// formatGraph renders GraphIndex as YAML (backward compatibility)
func (f *YAMLFormatter) formatGraph(gc *format.GraphContent) (string, error) {
	// Marshal the GraphIndex directly as YAML
	return f.marshal(gc.Index)
}

// marshal converts data to YAML
func (f *YAMLFormatter) marshal(data any) (string, error) {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("YAML encoding failed; %w", err)
	}

	return string(bytes), nil
}

func init() {
	// Register YAML formatter
	format.RegisterFormatter("yaml", NewYAMLFormatter())
}
