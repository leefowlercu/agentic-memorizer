package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProgress(t *testing.T) {
	progress := NewProgress(ProgressTypeBar, 50, 100)

	assert.Equal(t, ProgressTypeBar, progress.ProgressType)
	assert.Equal(t, 50, progress.Current)
	assert.Equal(t, 100, progress.Total)
	assert.Equal(t, 40, progress.BarWidth)
	assert.Equal(t, "", progress.Message)
	assert.Equal(t, BuilderTypeProgress, progress.Type())
}

func TestProgress_SetMessage(t *testing.T) {
	progress := NewProgress(ProgressTypePercentage, 0, 100).SetMessage("Processing files...")

	assert.Equal(t, "Processing files...", progress.Message)
}

func TestProgress_SetCurrent(t *testing.T) {
	progress := NewProgress(ProgressTypeBar, 0, 100)
	progress.SetCurrent(75)

	assert.Equal(t, 75, progress.Current)
}

func TestProgress_ShowPercentage(t *testing.T) {
	progress := NewProgress(ProgressTypeBar, 50, 100).ShowPercentage()

	assert.Equal(t, ProgressTypePercentage, progress.ProgressType)
}

func TestProgress_ShowBar(t *testing.T) {
	progress := NewProgress(ProgressTypePercentage, 50, 100).ShowBar(60)

	assert.Equal(t, ProgressTypeBar, progress.ProgressType)
	assert.Equal(t, 60, progress.BarWidth)
}

func TestProgress_FluentAPI(t *testing.T) {
	progress := NewProgress(ProgressTypeBar, 0, 100).
		SetMessage("Loading...").
		SetCurrent(25).
		ShowBar(50)

	assert.Equal(t, "Loading...", progress.Message)
	assert.Equal(t, 25, progress.Current)
	assert.Equal(t, 50, progress.BarWidth)
}

func TestProgress_Percentage(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		total    int
		expected float64
	}{
		{"half", 50, 100, 50.0},
		{"quarter", 25, 100, 25.0},
		{"complete", 100, 100, 100.0},
		{"zero", 0, 100, 0.0},
		{"zero total", 0, 0, 100.0}, // Edge case: 0/0 = 100%
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := NewProgress(ProgressTypeBar, tt.current, tt.total)
			assert.Equal(t, tt.expected, progress.Percentage())
		})
	}
}

func TestProgress_ValidateInvalidType(t *testing.T) {
	progress := &Progress{
		ProgressType: "invalid",
		Current:      50,
		Total:        100,
	}

	err := progress.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid progress type")
}

func TestProgress_ValidateNegativeTotal(t *testing.T) {
	progress := NewProgress(ProgressTypeBar, 0, -1)

	err := progress.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "total must be non-negative")
}

func TestProgress_ValidateNegativeCurrent(t *testing.T) {
	progress := NewProgress(ProgressTypeBar, -1, 100)

	err := progress.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "current must be non-negative")
}

func TestProgress_ValidateCurrentExceedsTotal(t *testing.T) {
	progress := NewProgress(ProgressTypeBar, 150, 100)

	err := progress.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot exceed total")
}

func TestProgress_ValidateInvalidBarWidth(t *testing.T) {
	progress := NewProgress(ProgressTypeBar, 50, 100)
	progress.BarWidth = 0

	err := progress.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bar width must be positive")
}

func TestProgress_ValidateValid(t *testing.T) {
	tests := []struct {
		name     string
		progress *Progress
	}{
		{
			"valid bar",
			NewProgress(ProgressTypeBar, 50, 100),
		},
		{
			"valid percentage",
			NewProgress(ProgressTypePercentage, 75, 100),
		},
		{
			"valid spinner",
			NewProgress(ProgressTypeSpinner, 0, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.progress.Validate()
			assert.NoError(t, err)
		})
	}
}
