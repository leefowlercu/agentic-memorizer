// Package initialize provides the TUI wizard for first-time configuration.
package initialize

import (
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/steps"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// Re-export step types for convenience.
type Step = steps.Step
type StepResult = steps.StepResult

const (
	StepContinue = steps.StepContinue
	StepNext     = steps.StepNext
	StepPrev     = steps.StepPrev
)

// WizardResult holds the outcome of the initialization wizard.
type WizardResult struct {
	// Config contains the final configuration if confirmed.
	Config *config.Config
	// Confirmed indicates whether the user confirmed the configuration.
	Confirmed bool
	// Cancelled indicates whether the user cancelled the wizard.
	Cancelled bool
	// Err contains any error that occurred during the wizard.
	Err error
}

// WizardModel is the main model for the initialization wizard.
type WizardModel struct {
	steps       []Step
	currentStep int
	config      *config.Config
	progress    components.Progress
	err         error
	cancelled   bool
	confirmed   bool
	quitting    bool
	width       int
	height      int
}

// NewWizard creates a new wizard with the given configuration and steps.
func NewWizard(cfg *config.Config, stepList []Step) WizardModel {
	// Build step titles for progress indicator
	titles := make([]string, len(stepList))
	for i, s := range stepList {
		titles[i] = s.Title()
	}

	slog.Debug("creating wizard model", "step_count", len(stepList))

	return WizardModel{
		steps:       stepList,
		currentStep: 0,
		config:      cfg,
		progress:    components.NewProgress(titles),
	}
}

// Init initializes the wizard and the first step.
func (m WizardModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		slog.Warn("wizard initialized with no steps")
		return tea.Quit
	}
	slog.Debug("initializing first step", "step", m.steps[0].Title())
	return m.steps[0].Init(m.config)
}

// Update handles input messages and delegates to the current step.
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			slog.Debug("wizard cancelled via Ctrl+C")
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	// Delegate to current step
	if m.currentStep >= 0 && m.currentStep < len(m.steps) {
		cmd, result := m.steps[m.currentStep].Update(msg)

		switch result {
		case StepNext:
			return m.nextStep()
		case StepPrev:
			return m.prevStep()
		}

		return m, cmd
	}

	return m, nil
}

// View renders the wizard UI.
func (m WizardModel) View() string {
	if m.quitting {
		if m.cancelled {
			return styles.ErrorText.Render("Initialization cancelled.") + "\n"
		}
		return ""
	}

	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Primary).
		MarginBottom(1).
		Render("Memorizer Setup Wizard")

	b.WriteString(header)
	b.WriteString("\n\n")

	// Progress indicator
	b.WriteString(m.progress.View())
	b.WriteString("\n")

	// Current step content
	if m.currentStep >= 0 && m.currentStep < len(m.steps) {
		b.WriteString(m.steps[m.currentStep].View())
	}

	// Error message if any
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(styles.ErrorText.Render(m.err.Error()))
	}

	// Apply padding around entire UI
	return styles.Container.Render(b.String())
}

// nextStep validates and applies the current step, then advances to the next.
func (m WizardModel) nextStep() (tea.Model, tea.Cmd) {
	if m.currentStep >= len(m.steps) {
		return m, nil
	}

	currentStep := m.steps[m.currentStep]
	slog.Debug("validating step", "step", currentStep.Title())

	// Validate current step
	if err := currentStep.Validate(); err != nil {
		slog.Debug("step validation failed", "step", currentStep.Title(), "error", err)
		m.err = err
		return m, nil
	}

	// Apply current step's configuration
	slog.Debug("applying step configuration", "step", currentStep.Title())
	if err := currentStep.Apply(m.config); err != nil {
		slog.Error("step apply failed", "step", currentStep.Title(), "error", err)
		m.err = err
		return m, nil
	}

	m.err = nil
	slog.Debug("step completed successfully", "step", currentStep.Title())

	// Check if this is the last step
	if m.currentStep >= len(m.steps)-1 {
		slog.Info("wizard completed, all steps finished")
		m.confirmed = true
		m.quitting = true
		return m, tea.Quit
	}

	// Advance to next step
	m.currentStep++
	m.progress.SetCurrent(m.currentStep)
	slog.Debug("advancing to next step", "step", m.steps[m.currentStep].Title(), "step_index", m.currentStep)

	// Initialize next step
	return m, m.steps[m.currentStep].Init(m.config)
}

// prevStep goes back to the previous step.
func (m WizardModel) prevStep() (tea.Model, tea.Cmd) {
	if m.currentStep <= 0 {
		slog.Debug("at first step, cannot go back")
		return m, nil
	}

	m.err = nil
	m.currentStep--
	m.progress.SetCurrent(m.currentStep)
	slog.Debug("going back to previous step", "step", m.steps[m.currentStep].Title(), "step_index", m.currentStep)

	// Re-initialize the previous step
	return m, m.steps[m.currentStep].Init(m.config)
}

// Result returns the wizard result after completion.
func (m WizardModel) Result() WizardResult {
	return WizardResult{
		Config:    m.config,
		Confirmed: m.confirmed,
		Cancelled: m.cancelled,
		Err:       m.err,
	}
}

// RunWizard runs the wizard and returns the result.
func RunWizard(cfg *config.Config, stepList []Step) (WizardResult, error) {
	wizard := NewWizard(cfg, stepList)

	p := tea.NewProgram(wizard, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return WizardResult{Err: err}, err
	}

	if m, ok := finalModel.(WizardModel); ok {
		return m.Result(), nil
	}

	return WizardResult{}, nil
}
