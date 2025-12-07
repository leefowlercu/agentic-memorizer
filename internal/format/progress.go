package format

import "fmt"

// ProgressType represents the type of progress indicator
type ProgressType string

const (
	// ProgressTypeBar represents a progress bar
	ProgressTypeBar ProgressType = "bar"

	// ProgressTypeSpinner represents a spinner
	ProgressTypeSpinner ProgressType = "spinner"

	// ProgressTypePercentage represents a percentage
	ProgressTypePercentage ProgressType = "percentage"
)

// Progress represents progress indicators
type Progress struct {
	ProgressType ProgressType
	Current      int
	Total        int
	Message      string
	BarWidth     int // Width of progress bar (default: 40)
}

// NewProgress creates a new progress indicator
func NewProgress(progressType ProgressType, current, total int) *Progress {
	return &Progress{
		ProgressType: progressType,
		Current:      current,
		Total:        total,
		Message:      "",
		BarWidth:     40, // Default width
	}
}

// SetMessage sets the progress message
func (p *Progress) SetMessage(msg string) *Progress {
	p.Message = msg
	return p
}

// SetCurrent updates the current progress value
func (p *Progress) SetCurrent(current int) *Progress {
	p.Current = current
	return p
}

// ShowPercentage enables percentage display
func (p *Progress) ShowPercentage() *Progress {
	p.ProgressType = ProgressTypePercentage
	return p
}

// ShowBar enables progress bar display with the specified width
func (p *Progress) ShowBar(width int) *Progress {
	p.ProgressType = ProgressTypeBar
	p.BarWidth = width
	return p
}

// Type returns the builder type
func (p *Progress) Type() BuilderType {
	return BuilderTypeProgress
}

// Validate checks if the progress indicator is correctly constructed
func (p *Progress) Validate() error {
	if p.ProgressType != ProgressTypeBar && p.ProgressType != ProgressTypeSpinner && p.ProgressType != ProgressTypePercentage {
		return fmt.Errorf("invalid progress type %q", p.ProgressType)
	}

	if p.Total < 0 {
		return fmt.Errorf("total must be non-negative; got %d", p.Total)
	}

	if p.Current < 0 {
		return fmt.Errorf("current must be non-negative; got %d", p.Current)
	}

	if p.Current > p.Total {
		return fmt.Errorf("current (%d) cannot exceed total (%d)", p.Current, p.Total)
	}

	if p.ProgressType == ProgressTypeBar && p.BarWidth <= 0 {
		return fmt.Errorf("bar width must be positive; got %d", p.BarWidth)
	}

	return nil
}

// Percentage returns the completion percentage (0-100)
func (p *Progress) Percentage() float64 {
	if p.Total == 0 {
		return 100.0
	}
	return float64(p.Current) / float64(p.Total) * 100.0
}
