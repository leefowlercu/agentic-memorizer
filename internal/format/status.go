package format

import "fmt"

// StatusSeverity represents the severity level of a status message
type StatusSeverity string

const (
	// StatusSuccess represents successful completion
	StatusSuccess StatusSeverity = "success"

	// StatusInfo represents informational messages
	StatusInfo StatusSeverity = "info"

	// StatusWarning represents warnings
	StatusWarning StatusSeverity = "warning"

	// StatusError represents errors
	StatusError StatusSeverity = "error"

	// StatusRunning represents running/active state
	StatusRunning StatusSeverity = "running"

	// StatusStopped represents stopped/inactive state
	StatusStopped StatusSeverity = "stopped"
)

// Status represents status messages with severity and symbols
type Status struct {
	Severity     StatusSeverity
	Message      string
	Details      []string
	CustomSymbol string // Optional custom symbol override
}

// NewStatus creates a new status message
func NewStatus(severity StatusSeverity, message string) *Status {
	return &Status{
		Severity:     severity,
		Message:      message,
		Details:      make([]string, 0),
		CustomSymbol: "",
	}
}

// AddDetail adds a detail line to the status
func (s *Status) AddDetail(detail string) *Status {
	s.Details = append(s.Details, detail)
	return s
}

// WithSymbol sets a custom symbol override
func (s *Status) WithSymbol(symbol string) *Status {
	s.CustomSymbol = symbol
	return s
}

// Type returns the builder type
func (s *Status) Type() BuilderType {
	return BuilderTypeStatus
}

// Validate checks if the status is correctly constructed
func (s *Status) Validate() error {
	validSeverities := []StatusSeverity{
		StatusSuccess,
		StatusInfo,
		StatusWarning,
		StatusError,
		StatusRunning,
		StatusStopped,
	}

	isValid := false
	for _, valid := range validSeverities {
		if s.Severity == valid {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid status severity %q", s.Severity)
	}

	if s.Message == "" {
		return fmt.Errorf("status message cannot be empty")
	}

	return nil
}
