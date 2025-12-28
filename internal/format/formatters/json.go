package formatters

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
)

// JSONFormatter renders output as JSON
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Format renders a single buildable structure
func (f *JSONFormatter) Format(b format.Buildable) (string, error) {
	if err := b.Validate(); err != nil {
		return "", fmt.Errorf("validation failed; %w", err)
	}

	var data any

	switch v := b.(type) {
	case *format.Section:
		data = f.sectionToJSON(v)
	case *format.Table:
		data = f.tableToJSON(v)
	case *format.List:
		data = f.listToJSON(v)
	case *format.Progress:
		data = f.progressToJSON(v)
	case *format.Status:
		data = f.statusToJSON(v)
	case *format.Error:
		data = f.errorToJSON(v)
	case *format.GraphContent:
		// GraphContent marshals the GraphIndex directly
		return f.formatGraph(v)
	case *format.FactsContent:
		// FactsContent marshals the FactsIndex directly
		return f.formatFacts(v)
	default:
		return "", fmt.Errorf("unsupported builder type: %s", b.Type())
	}

	return f.marshal(data)
}

// FormatMultiple renders multiple buildable structures
func (f *JSONFormatter) FormatMultiple(builders []format.Buildable) (string, error) {
	items := make([]any, 0, len(builders))

	for _, b := range builders {
		if err := b.Validate(); err != nil {
			return "", fmt.Errorf("validation failed; %w", err)
		}

		switch v := b.(type) {
		case *format.Section:
			items = append(items, f.sectionToJSON(v))
		case *format.Table:
			items = append(items, f.tableToJSON(v))
		case *format.List:
			items = append(items, f.listToJSON(v))
		case *format.Progress:
			items = append(items, f.progressToJSON(v))
		case *format.Status:
			items = append(items, f.statusToJSON(v))
		case *format.Error:
			items = append(items, f.errorToJSON(v))
		default:
			return "", fmt.Errorf("unsupported builder type: %s", b.Type())
		}
	}

	return f.marshal(items)
}

// Name returns the formatter name
func (f *JSONFormatter) Name() string {
	return "json"
}

// SupportsColors returns false (JSON doesn't support colors)
func (f *JSONFormatter) SupportsColors() bool {
	return false
}

// JSON structure types

type jsonSection struct {
	Type   string            `json:"type"`
	Title  string            `json:"title"`
	Level  int               `json:"level,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
	Items  []jsonSectionItem `json:"items,omitempty"`
}

type jsonSectionItem struct {
	Type       string       `json:"type"`
	Key        string       `json:"key,omitempty"`
	Value      string       `json:"value,omitempty"`
	Subsection *jsonSection `json:"subsection,omitempty"`
}

type jsonTable struct {
	Type       string     `json:"type"`
	Headers    []string   `json:"headers"`
	Rows       [][]string `json:"rows"`
	Alignments []string   `json:"alignments,omitempty"`
	Compact    bool       `json:"compact,omitempty"`
}

type jsonList struct {
	Type     string         `json:"type"`
	ListType string         `json:"list_type"`
	Items    []jsonListItem `json:"items"`
	Compact  bool           `json:"compact,omitempty"`
}

type jsonListItem struct {
	Content string    `json:"content"`
	Nested  *jsonList `json:"nested,omitempty"`
}

type jsonProgress struct {
	Type         string  `json:"type"`
	ProgressType string  `json:"progress_type"`
	Current      int     `json:"current"`
	Total        int     `json:"total"`
	Percentage   float64 `json:"percentage"`
	Message      string  `json:"message,omitempty"`
}

type jsonStatus struct {
	Type     string   `json:"type"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Symbol   string   `json:"symbol"`
	Details  []string `json:"details,omitempty"`
}

type jsonError struct {
	Type       string   `json:"type"`
	ErrorType  string   `json:"error_type"`
	Message    string   `json:"message"`
	Field      string   `json:"field,omitempty"`
	Value      any      `json:"value,omitempty"`
	Details    []string `json:"details,omitempty"`
	Suggestion string   `json:"suggestion,omitempty"`
}

// Conversion methods

func (f *JSONFormatter) sectionToJSON(s *format.Section) *jsonSection {
	result := &jsonSection{
		Type:   "section",
		Title:  s.Title,
		Level:  s.Level,
		Fields: make(map[string]string),
		Items:  make([]jsonSectionItem, 0),
	}

	for _, item := range s.Items {
		if item.Type == format.SectionItemKeyValue {
			result.Fields[item.Key] = item.Value
			result.Items = append(result.Items, jsonSectionItem{
				Type:  "key_value",
				Key:   item.Key,
				Value: item.Value,
			})
		} else if item.Type == format.SectionItemSubsection {
			result.Items = append(result.Items, jsonSectionItem{
				Type:       "subsection",
				Subsection: f.sectionToJSON(item.Subsection),
			})
		}
	}

	return result
}

func (f *JSONFormatter) tableToJSON(t *format.Table) *jsonTable {
	alignments := make([]string, len(t.Alignments))
	for i, a := range t.Alignments {
		alignments[i] = string(a)
	}

	return &jsonTable{
		Type:       "table",
		Headers:    t.Headers,
		Rows:       t.Rows,
		Alignments: alignments,
		Compact:    t.IsCompact,
	}
}

func (f *JSONFormatter) listToJSON(l *format.List) *jsonList {
	items := make([]jsonListItem, len(l.Items))
	for i, item := range l.Items {
		items[i] = jsonListItem{
			Content: item.Content,
		}
		if item.Nested != nil {
			items[i].Nested = f.listToJSON(item.Nested)
		}
	}

	return &jsonList{
		Type:     "list",
		ListType: string(l.ListType),
		Items:    items,
		Compact:  l.IsCompact,
	}
}

func (f *JSONFormatter) progressToJSON(p *format.Progress) *jsonProgress {
	return &jsonProgress{
		Type:         "progress",
		ProgressType: string(p.ProgressType),
		Current:      p.Current,
		Total:        p.Total,
		Percentage:   p.Percentage(),
		Message:      p.Message,
	}
}

func (f *JSONFormatter) statusToJSON(s *format.Status) *jsonStatus {
	symbol := s.CustomSymbol
	if symbol == "" {
		symbol = format.GetStatusSymbol(s.Severity)
	}

	return &jsonStatus{
		Type:     "status",
		Severity: string(s.Severity),
		Message:  s.Message,
		Symbol:   symbol,
		Details:  s.Details,
	}
}

func (f *JSONFormatter) errorToJSON(e *format.Error) *jsonError {
	return &jsonError{
		Type:       "error",
		ErrorType:  string(e.ErrorType),
		Message:    e.Message,
		Field:      e.Field,
		Value:      e.Value,
		Details:    e.Details,
		Suggestion: e.Suggestion,
	}
}

// formatGraph renders GraphIndex as pretty-printed JSON (backward compatibility)
func (f *JSONFormatter) formatGraph(gc *format.GraphContent) (string, error) {
	// Marshal the GraphIndex directly with pretty-printing and no HTML escaping
	return f.marshal(gc.Index)
}

// formatFacts renders FactsIndex as pretty-printed JSON
func (f *JSONFormatter) formatFacts(fc *format.FactsContent) (string, error) {
	// Marshal the FactsIndex directly with pretty-printing and no HTML escaping
	return f.marshal(fc.Index)
}

// marshal converts data to JSON with proper formatting
func (f *JSONFormatter) marshal(data any) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(data); err != nil {
		return "", fmt.Errorf("JSON encoding failed; %w", err)
	}

	// Trim trailing newline added by encoder
	result := buf.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}

func init() {
	// Register JSON formatter
	format.RegisterFormatter("json", NewJSONFormatter())
}
