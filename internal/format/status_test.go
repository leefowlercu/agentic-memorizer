package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStatus(t *testing.T) {
	status := NewStatus(StatusSuccess, "Operation completed")

	assert.Equal(t, StatusSuccess, status.Severity)
	assert.Equal(t, "Operation completed", status.Message)
	assert.Empty(t, status.Details)
	assert.Equal(t, "", status.CustomSymbol)
	assert.Equal(t, BuilderTypeStatus, status.Type())
}

func TestStatus_AddDetail(t *testing.T) {
	status := NewStatus(StatusInfo, "Processing")
	status.AddDetail("Step 1 complete")
	status.AddDetail("Step 2 complete")

	require.Len(t, status.Details, 2)
	assert.Equal(t, "Step 1 complete", status.Details[0])
	assert.Equal(t, "Step 2 complete", status.Details[1])
}

func TestStatus_WithSymbol(t *testing.T) {
	status := NewStatus(StatusSuccess, "Done").WithSymbol("🎉")

	assert.Equal(t, "🎉", status.CustomSymbol)
}

func TestStatus_FluentAPI(t *testing.T) {
	status := NewStatus(StatusWarning, "Check required").
		AddDetail("Detail 1").
		AddDetail("Detail 2").
		WithSymbol("⚠")

	assert.Equal(t, StatusWarning, status.Severity)
	assert.Len(t, status.Details, 2)
	assert.Equal(t, "⚠", status.CustomSymbol)
}

func TestStatus_ValidateAllSeverities(t *testing.T) {
	severities := []StatusSeverity{
		StatusSuccess,
		StatusInfo,
		StatusWarning,
		StatusError,
		StatusRunning,
		StatusStopped,
	}

	for _, severity := range severities {
		t.Run(string(severity), func(t *testing.T) {
			status := NewStatus(severity, "Test message")
			err := status.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestStatus_ValidateInvalidSeverity(t *testing.T) {
	status := &Status{
		Severity: "invalid",
		Message:  "Test",
	}

	err := status.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status severity")
}

func TestStatus_ValidateEmptyMessage(t *testing.T) {
	status := NewStatus(StatusSuccess, "")

	err := status.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status message cannot be empty")
}

func TestStatus_ValidateValid(t *testing.T) {
	status := NewStatus(StatusSuccess, "Operation complete")
	status.AddDetail("Detail 1")
	status.AddDetail("Detail 2")

	err := status.Validate()
	assert.NoError(t, err)
}
