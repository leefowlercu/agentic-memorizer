package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewError(t *testing.T) {
	err := NewError(ErrorTypeValidation, "Invalid value")

	assert.Equal(t, ErrorTypeValidation, err.ErrorType)
	assert.Equal(t, "Invalid value", err.Message)
	assert.Equal(t, "", err.Field)
	assert.Nil(t, err.Value)
	assert.Empty(t, err.Details)
	assert.Equal(t, "", err.Suggestion)
	assert.Equal(t, BuilderTypeError, err.Type())
}

func TestError_SetField(t *testing.T) {
	err := NewError(ErrorTypeValidation, "Invalid").SetField("username")

	assert.Equal(t, "username", err.Field)
}

func TestError_SetValue(t *testing.T) {
	err := NewError(ErrorTypeInput, "Invalid value").SetValue("invalid-input")

	assert.Equal(t, "invalid-input", err.Value)
}

func TestError_AddDetail(t *testing.T) {
	err := NewError(ErrorTypeRuntime, "Operation failed")
	err.AddDetail("Database connection lost")
	err.AddDetail("Retry failed after 3 attempts")

	require.Len(t, err.Details, 2)
	assert.Equal(t, "Database connection lost", err.Details[0])
	assert.Equal(t, "Retry failed after 3 attempts", err.Details[1])
}

func TestError_WithSuggestion(t *testing.T) {
	err := NewError(ErrorTypeValidation, "Invalid format").
		WithSuggestion("Use format: YYYY-MM-DD")

	assert.Equal(t, "Use format: YYYY-MM-DD", err.Suggestion)
}

func TestError_FluentAPI(t *testing.T) {
	err := NewError(ErrorTypeValidation, "Invalid email").
		SetField("email").
		SetValue("not-an-email").
		AddDetail("Must contain @ symbol").
		WithSuggestion("Use format: user@example.com")

	assert.Equal(t, ErrorTypeValidation, err.ErrorType)
	assert.Equal(t, "email", err.Field)
	assert.Equal(t, "not-an-email", err.Value)
	assert.Len(t, err.Details, 1)
	assert.Equal(t, "Use format: user@example.com", err.Suggestion)
}

func TestError_ValidateAllTypes(t *testing.T) {
	types := []ErrorType{
		ErrorTypeValidation,
		ErrorTypeRuntime,
		ErrorTypeInput,
	}

	for _, errorType := range types {
		t.Run(string(errorType), func(t *testing.T) {
			err := NewError(errorType, "Test error")
			valErr := err.Validate()
			assert.NoError(t, valErr)
		})
	}
}

func TestError_ValidateInvalidType(t *testing.T) {
	err := &Error{
		ErrorType: "invalid",
		Message:   "Test",
	}

	valErr := err.Validate()
	assert.Error(t, valErr)
	assert.Contains(t, valErr.Error(), "invalid error type")
}

func TestError_ValidateEmptyMessage(t *testing.T) {
	err := NewError(ErrorTypeValidation, "")

	valErr := err.Validate()
	assert.Error(t, valErr)
	assert.Contains(t, valErr.Error(), "error message cannot be empty")
}

func TestError_ValidateValid(t *testing.T) {
	err := NewError(ErrorTypeValidation, "Validation failed")
	err.SetField("age")
	err.SetValue(-1)
	err.AddDetail("Age must be positive")
	err.WithSuggestion("Provide a positive integer")

	valErr := err.Validate()
	assert.NoError(t, valErr)
}
