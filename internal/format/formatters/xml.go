package formatters

import (
	"encoding/xml"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
)

// XMLFormatter renders output as XML
type XMLFormatter struct{}

// NewXMLFormatter creates a new XML formatter
func NewXMLFormatter() *XMLFormatter {
	return &XMLFormatter{}
}

// Format renders a single buildable structure
func (f *XMLFormatter) Format(b format.Buildable) (string, error) {
	if err := b.Validate(); err != nil {
		return "", fmt.Errorf("validation failed; %w", err)
	}

	var data any

	switch v := b.(type) {
	case *format.Section:
		data = f.sectionToXML(v)
	case *format.Table:
		data = f.tableToXML(v)
	case *format.List:
		data = f.listToXML(v)
	case *format.Progress:
		data = f.progressToXML(v)
	case *format.Status:
		data = f.statusToXML(v)
	case *format.Error:
		data = f.errorToXML(v)
	default:
		return "", fmt.Errorf("unsupported builder type: %s", b.Type())
	}

	return f.marshal(data)
}

// FormatMultiple renders multiple buildable structures
func (f *XMLFormatter) FormatMultiple(builders []format.Buildable) (string, error) {
	items := make([]any, 0, len(builders))

	for _, b := range builders {
		if err := b.Validate(); err != nil {
			return "", fmt.Errorf("validation failed; %w", err)
		}

		switch v := b.(type) {
		case *format.Section:
			items = append(items, f.sectionToXML(v))
		case *format.Table:
			items = append(items, f.tableToXML(v))
		case *format.List:
			items = append(items, f.listToXML(v))
		case *format.Progress:
			items = append(items, f.progressToXML(v))
		case *format.Status:
			items = append(items, f.statusToXML(v))
		case *format.Error:
			items = append(items, f.errorToXML(v))
		default:
			return "", fmt.Errorf("unsupported builder type: %s", b.Type())
		}
	}

	// Wrap in root element
	wrapper := xmlMultiple{Items: items}
	return f.marshal(wrapper)
}

// Name returns the formatter name
func (f *XMLFormatter) Name() string {
	return "xml"
}

// SupportsColors returns false (XML doesn't support colors)
func (f *XMLFormatter) SupportsColors() bool {
	return false
}

// XML structure types

type xmlSection struct {
	XMLName xml.Name         `xml:"section"`
	Title   string           `xml:"title,attr"`
	Level   int              `xml:"level,attr,omitempty"`
	Items   []xmlSectionItem `xml:"item"`
}

type xmlSectionItem struct {
	Type       string      `xml:"type,attr"`
	Key        string      `xml:"key,attr,omitempty"`
	Value      string      `xml:"value,attr,omitempty"`
	Subsection *xmlSection `xml:"section,omitempty"`
}

type xmlTable struct {
	XMLName    xml.Name   `xml:"table"`
	Headers    []string   `xml:"headers>header"`
	Rows       []xmlRow   `xml:"rows>row"`
	Alignments []string   `xml:"alignments>alignment,omitempty"`
	Compact    bool       `xml:"compact,attr,omitempty"`
}

type xmlRow struct {
	Cells []string `xml:"cell"`
}

type xmlList struct {
	XMLName  xml.Name      `xml:"list"`
	ListType string        `xml:"type,attr"`
	Compact  bool          `xml:"compact,attr,omitempty"`
	Items    []xmlListItem `xml:"item"`
}

type xmlListItem struct {
	Content string   `xml:"content"`
	Nested  *xmlList `xml:"list,omitempty"`
}

type xmlProgress struct {
	XMLName      xml.Name `xml:"progress"`
	ProgressType string   `xml:"type,attr"`
	Current      int      `xml:"current"`
	Total        int      `xml:"total"`
	Percentage   float64  `xml:"percentage"`
	Message      string   `xml:"message,omitempty"`
}

type xmlStatus struct {
	XMLName  xml.Name `xml:"status"`
	Severity string   `xml:"severity,attr"`
	Symbol   string   `xml:"symbol,attr"`
	Message  string   `xml:"message"`
	Details  []string `xml:"details>detail,omitempty"`
}

type xmlError struct {
	XMLName    xml.Name `xml:"error"`
	ErrorType  string   `xml:"type,attr"`
	Message    string   `xml:"message"`
	Field      string   `xml:"field,omitempty"`
	Value      string   `xml:"value,omitempty"`
	Details    []string `xml:"details>detail,omitempty"`
	Suggestion string   `xml:"suggestion,omitempty"`
}

type xmlMultiple struct {
	XMLName xml.Name `xml:"output"`
	Items   []any    `xml:",any"`
}

// Conversion methods

func (f *XMLFormatter) sectionToXML(s *format.Section) *xmlSection {
	result := &xmlSection{
		Title: s.Title,
		Level: s.Level,
		Items: make([]xmlSectionItem, 0),
	}

	for _, item := range s.Items {
		if item.Type == format.SectionItemKeyValue {
			result.Items = append(result.Items, xmlSectionItem{
				Type:  "key_value",
				Key:   item.Key,
				Value: item.Value,
			})
		} else if item.Type == format.SectionItemSubsection {
			result.Items = append(result.Items, xmlSectionItem{
				Type:       "subsection",
				Subsection: f.sectionToXML(item.Subsection),
			})
		}
	}

	return result
}

func (f *XMLFormatter) tableToXML(t *format.Table) *xmlTable {
	alignments := make([]string, len(t.Alignments))
	for i, a := range t.Alignments {
		alignments[i] = string(a)
	}

	rows := make([]xmlRow, len(t.Rows))
	for i, row := range t.Rows {
		rows[i] = xmlRow{Cells: row}
	}

	return &xmlTable{
		Headers:    t.Headers,
		Rows:       rows,
		Alignments: alignments,
		Compact:    t.IsCompact,
	}
}

func (f *XMLFormatter) listToXML(l *format.List) *xmlList {
	items := make([]xmlListItem, len(l.Items))
	for i, item := range l.Items {
		items[i] = xmlListItem{
			Content: item.Content,
		}
		if item.Nested != nil {
			items[i].Nested = f.listToXML(item.Nested)
		}
	}

	return &xmlList{
		ListType: string(l.ListType),
		Items:    items,
		Compact:  l.IsCompact,
	}
}

func (f *XMLFormatter) progressToXML(p *format.Progress) *xmlProgress {
	return &xmlProgress{
		ProgressType: string(p.ProgressType),
		Current:      p.Current,
		Total:        p.Total,
		Percentage:   p.Percentage(),
		Message:      p.Message,
	}
}

func (f *XMLFormatter) statusToXML(s *format.Status) *xmlStatus {
	symbol := s.CustomSymbol
	if symbol == "" {
		symbol = format.GetStatusSymbol(s.Severity)
	}

	return &xmlStatus{
		Severity: string(s.Severity),
		Symbol:   symbol,
		Message:  s.Message,
		Details:  s.Details,
	}
}

func (f *XMLFormatter) errorToXML(e *format.Error) *xmlError {
	valueStr := ""
	if e.Value != nil {
		valueStr = fmt.Sprintf("%v", e.Value)
	}

	return &xmlError{
		ErrorType:  string(e.ErrorType),
		Message:    e.Message,
		Field:      e.Field,
		Value:      valueStr,
		Details:    e.Details,
		Suggestion: e.Suggestion,
	}
}

// marshal converts data to XML with proper formatting
func (f *XMLFormatter) marshal(data any) (string, error) {
	bytes, err := xml.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("XML encoding failed; %w", err)
	}

	return xml.Header + string(bytes), nil
}

func init() {
	// Register XML formatter
	format.RegisterFormatter("xml", NewXMLFormatter())
}
