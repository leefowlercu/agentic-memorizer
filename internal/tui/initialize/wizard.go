package initialize

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/steps"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// WizardResult contains the outcome of the wizard
type WizardResult struct {
	Config    *config.Config
	Confirmed bool
	Cancelled bool
	Err       error
}

// WizardModel is the main Bubbletea model for the initialization wizard
type WizardModel struct {
	steps       []steps.Step
	stepNames   []string
	currentStep int
	config      *config.Config
	progress    *components.Progress
	width       int
	height      int
	quitting    bool
	confirmed   bool
	err         error
}

// NewWizard creates a new initialization wizard
func NewWizard(cfg *config.Config) *WizardModel {
	stepList := []steps.Step{
		steps.NewAPIKeyStep(),
		steps.NewHTTPPortStep(),
		steps.NewFalkorDBStep(),
		steps.NewEmbeddingsStep(),
		steps.NewIntegrationsStep(),
		steps.NewConfirmStep(),
	}

	stepNames := make([]string, len(stepList))
	for i, step := range stepList {
		stepNames[i] = step.Title()
	}

	return &WizardModel{
		steps:       stepList,
		stepNames:   stepNames,
		currentStep: 0,
		config:      cfg,
		progress:    components.NewProgress(stepNames),
		width:       80,
		height:      24,
	}
}

// Init implements tea.Model
func (m WizardModel) Init() tea.Cmd {
	// Initialize the first step
	return m.steps[0].Init(m.config)
}

// Update implements tea.Model
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	// Delegate to current step
	cmd, result := m.steps[m.currentStep].Update(msg)

	switch result {
	case steps.StepNext:
		return m.nextStep()
	case steps.StepPrev:
		return m.prevStep()
	}

	return m, cmd
}

// View implements tea.Model
func (m WizardModel) View() string {
	if m.quitting {
		return styles.MutedText.Render("Initialization cancelled.\n")
	}

	var b strings.Builder

	// Header
	header := styles.Title.Render("Agentic Memorizer Setup")
	b.WriteString(header)
	b.WriteString("\n\n")

	// Progress indicator
	b.WriteString(m.progress.View())
	b.WriteString("\n")

	// Current step content
	stepContent := m.steps[m.currentStep].View()
	b.WriteString(stepContent)

	// Apply container styling
	content := styles.Container.Render(b.String())

	// Center if terminal is wide enough
	if m.width > 100 {
		content = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content)
	}

	return content
}

// nextStep advances to the next step
func (m WizardModel) nextStep() (tea.Model, tea.Cmd) {
	// Apply current step's values to config
	if err := m.steps[m.currentStep].Apply(m.config); err != nil {
		m.err = err
		return m, nil
	}

	// Check if this was the last step
	if m.currentStep >= len(m.steps)-1 {
		// Check if user confirmed
		if confirmStep, ok := m.steps[m.currentStep].(*steps.ConfirmStep); ok {
			m.confirmed = confirmStep.IsConfirmed()
		}
		m.quitting = true
		return m, tea.Quit
	}

	// Advance to next step
	m.currentStep++
	m.progress.SetStep(m.currentStep)

	// Initialize the new step
	return m, m.steps[m.currentStep].Init(m.config)
}

// prevStep goes back to the previous step
func (m WizardModel) prevStep() (tea.Model, tea.Cmd) {
	if m.currentStep <= 0 {
		return m, nil
	}

	m.currentStep--
	m.progress.SetStep(m.currentStep)

	// Re-initialize the step with current config
	return m, m.steps[m.currentStep].Init(m.config)
}

// RunWizard runs the interactive initialization wizard
// Returns the completed config and whether the user confirmed
func RunWizard(initialConfig *config.Config) (*WizardResult, error) {
	model := NewWizard(initialConfig)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("wizard error; %w", err)
	}

	m, ok := finalModel.(WizardModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type; expected WizardModel, got %T", finalModel)
	}

	return &WizardResult{
		Config:    m.config,
		Confirmed: m.confirmed,
		Cancelled: m.quitting && !m.confirmed,
		Err:       m.err,
	}, nil
}

// GetSelectedIntegrations returns the integrations selected in the wizard
// Call this after RunWizard to get the list of integrations to setup
func GetSelectedIntegrations(wizardSteps []steps.Step) []string {
	for _, step := range wizardSteps {
		if intStep, ok := step.(*steps.IntegrationsStep); ok {
			return intStep.SelectedIntegrations()
		}
	}
	return nil
}
