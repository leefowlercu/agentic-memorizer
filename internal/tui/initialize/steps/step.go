// Package steps provides wizard step implementations for the initialization wizard.
package steps

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

// StepResult indicates the result of a step update.
type StepResult int

const (
	// StepContinue indicates the step should continue processing.
	StepContinue StepResult = iota
	// StepNext indicates the wizard should advance to the next step.
	StepNext
	// StepPrev indicates the wizard should go back to the previous step.
	StepPrev
)

// Step is the interface that all wizard steps must implement.
type Step interface {
	// Init initializes the step with the current configuration.
	// It is called when the step becomes active.
	Init(cfg *config.Config) tea.Cmd

	// Update handles input messages and returns the result.
	Update(msg tea.Msg) (tea.Cmd, StepResult)

	// View renders the step's UI.
	View() string

	// Title returns the step's title for the progress indicator.
	Title() string

	// Validate checks if the step's input is valid.
	// Returns nil if valid, error otherwise.
	Validate() error

	// Apply writes the step's configuration values to the config.
	// This is called when advancing from this step.
	Apply(cfg *config.Config) error
}

// BaseStep provides common functionality for steps.
type BaseStep struct {
	title string
}

// NewBaseStep creates a new base step with the given title.
func NewBaseStep(title string) BaseStep {
	return BaseStep{title: title}
}

// Title returns the step's title.
func (b BaseStep) Title() string {
	return b.title
}
