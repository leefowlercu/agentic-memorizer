package steps

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

// Step represents a single wizard step
type Step interface {
	// Init initializes the step with the current config state
	// Called when the step becomes active
	Init(cfg *config.Config) tea.Cmd

	// Update handles messages for this step
	// Returns a command and a boolean indicating if the step is complete
	Update(msg tea.Msg) (tea.Cmd, StepResult)

	// View renders the step content (not including progress bar)
	View() string

	// Title returns the step title for the progress indicator
	Title() string

	// Validate checks if current values are valid
	// Returns nil if valid, error message otherwise
	Validate() error

	// Apply writes the step's values to the config
	// Called after successful validation when moving to next step
	Apply(cfg *config.Config) error
}

// StepResult indicates what should happen after Update
type StepResult int

const (
	// StepContinue means stay on current step
	StepContinue StepResult = iota
	// StepNext means move to next step
	StepNext
	// StepPrev means move to previous step
	StepPrev
)

// NavigationHelp returns standard navigation help text
func NavigationHelp() string {
	return "↑/↓: navigate  enter: next  esc: back  ctrl+c: quit"
}

// NavigationHelpWithInput returns navigation help for steps with text input
func NavigationHelpWithInput() string {
	return "tab: next field  enter: next step  esc: back  ctrl+c: quit"
}
