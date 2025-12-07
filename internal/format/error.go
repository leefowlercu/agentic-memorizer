package format

import "fmt"

// ErrorType represents the type of error
type ErrorType string

const (
	// ErrorTypeValidation represents validation errors
	ErrorTypeValidation ErrorType = "validation"

	// ErrorTypeRuntime represents runtime errors
	ErrorTypeRuntime ErrorType = "runtime"

	// ErrorTypeInput represents user input errors
	ErrorTypeInput ErrorType = "input"
)

// Error represents structured error messages with suggestions
type Error struct {
	ErrorType  ErrorType
	Message    string
	Field      string   // Optional field name (for validation errors)
	Value      any      // Optional value that caused the error
	Details    []string // Additional details
	Suggestion string   // Optional suggestion for resolution
}

// NewError creates a new error message
func NewError(errorType ErrorType, message string) *Error {
	return &Error{
		ErrorType:  errorType,
		Message:    message,
		Field:      "",
		Value:      nil,
		Details:    make([]string, 0),
		Suggestion: "",
	}
}

// SetField sets the field name (for validation errors)
func (e *Error) SetField(field string) *Error {
	e.Field = field
	return e
}

// SetValue sets the value that caused the error
func (e *Error) SetValue(value any) *Error {
	e.Value = value
	return e
}

// AddDetail adds a detail line to the error
func (e *Error) AddDetail(detail string) *Error {
	e.Details = append(e.Details, detail)
	return e
}

// WithSuggestion sets a suggestion for resolving the error
func (e *Error) WithSuggestion(suggestion string) *Error {
	e.Suggestion = suggestion
	return e
}

// Type returns the builder type
func (e *Error) Type() BuilderType {
	return BuilderTypeError
}

// Validate checks if the error is correctly constructed
func (e *Error) Validate() error {
	validTypes := []ErrorType{
		ErrorTypeValidation,
		ErrorTypeRuntime,
		ErrorTypeInput,
	}

	isValid := false
	for _, valid := range validTypes {
		if e.ErrorType == valid {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid error type %q", e.ErrorType)
	}

	if e.Message == "" {
		return fmt.Errorf("error message cannot be empty")
	}

	return nil
}
