package steps

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// APIKeyStep handles Claude API key configuration
type APIKeyStep struct {
	radio       *components.RadioGroup
	keyInput    *components.TextInput
	envKeyFound bool
	err         error
	focusIndex  int // 0 = radio, 1 = key input (when visible)
}

// NewAPIKeyStep creates a new API key configuration step
func NewAPIKeyStep() *APIKeyStep {
	return &APIKeyStep{}
}

// Title returns the step title
func (s *APIKeyStep) Title() string {
	return "Claude API Key"
}

// Init initializes the step
func (s *APIKeyStep) Init(cfg *config.Config) tea.Cmd {
	s.envKeyFound = os.Getenv(config.ClaudeAPIKeyEnv) != ""
	s.err = nil
	s.focusIndex = 0

	// Build options based on environment
	var options []components.RadioOption
	if s.envKeyFound {
		options = []components.RadioOption{
			{Label: "Use env. variable value", Description: config.ClaudeAPIKeyEnv + " detected"},
			{Label: "Enter API key directly", Description: "Will be stored in config file"},
			{Label: "Skip for now", Description: "Configure later"},
		}
	} else {
		// When env var not found, only offer direct entry or skip
		options = []components.RadioOption{
			{Label: "Enter API key directly", Description: "Will be stored in config file"},
			{Label: "Skip for now", Description: "Configure later"},
		}
	}

	s.radio = components.NewRadioGroup(options)
	s.radio.Focus()

	s.keyInput = components.NewTextInput("API Key").
		WithPlaceholder("sk-ant-...").
		WithMasked().
		WithWidth(60)

	// Pre-select based on existing config
	if cfg.Claude.APIKey != "" {
		if s.envKeyFound {
			s.radio.SetSelected(1) // "Enter directly" is option 1 when env var found
		} else {
			s.radio.SetSelected(0) // "Enter directly" is option 0 when env var not found
		}
		s.keyInput.SetValue(cfg.Claude.APIKey)
	} else if s.envKeyFound {
		s.radio.SetSelected(0) // "Use env. variable value"
	}

	return nil
}

// Update handles input
func (s *APIKeyStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if err := s.Validate(); err != nil {
				s.err = err
				return nil, StepContinue
			}
			s.err = nil
			return nil, StepNext

		case "esc":
			return nil, StepPrev

		case "tab":
			// Toggle focus between radio and input when input is visible
			// When env var found: "Enter directly" is option 1
			// When env var not found: "Enter directly" is option 0
			enterDirectlySelected := (s.envKeyFound && s.radio.Selected() == 1) || (!s.envKeyFound && s.radio.Selected() == 0)
			if enterDirectlySelected {
				s.focusIndex = (s.focusIndex + 1) % 2
				if s.focusIndex == 0 {
					s.radio.Focus()
					s.keyInput.Blur()
				} else {
					s.radio.Blur()
					return s.keyInput.Focus(), StepContinue
				}
			}
			return nil, StepContinue

		case "shift+tab":
			enterDirectlySelected := (s.envKeyFound && s.radio.Selected() == 1) || (!s.envKeyFound && s.radio.Selected() == 0)
			if enterDirectlySelected && s.focusIndex == 1 {
				s.focusIndex = 0
				s.radio.Focus()
				s.keyInput.Blur()
			}
			return nil, StepContinue
		}
	}

	// Delegate to focused component
	enterDirectlySelected := (s.envKeyFound && s.radio.Selected() == 1) || (!s.envKeyFound && s.radio.Selected() == 0)
	if s.focusIndex == 1 && enterDirectlySelected {
		cmd := s.keyInput.Update(msg)
		return cmd, StepContinue
	}

	// Handle radio selection change
	oldSelected := s.radio.Selected()
	s.radio.Update(msg)
	newSelected := s.radio.Selected()

	// Auto-focus input when "Enter directly" is selected
	oldEnterDirectly := (s.envKeyFound && oldSelected == 1) || (!s.envKeyFound && oldSelected == 0)
	newEnterDirectly := (s.envKeyFound && newSelected == 1) || (!s.envKeyFound && newSelected == 0)

	if !oldEnterDirectly && newEnterDirectly {
		s.focusIndex = 1
		s.radio.Blur()
		return s.keyInput.Focus(), StepContinue
	} else if oldEnterDirectly && !newEnterDirectly {
		s.focusIndex = 0
		s.radio.Focus()
		s.keyInput.Blur()
	}

	return nil, StepContinue
}

// View renders the step
func (s *APIKeyStep) View() string {
	var b strings.Builder

	b.WriteString(styles.Subtitle.Render("Configure how Claude API key is provided"))
	b.WriteString("\n\n")

	b.WriteString(s.radio.View())

	// Show input field when "Enter directly" is selected
	enterDirectlySelected := (s.envKeyFound && s.radio.Selected() == 1) || (!s.envKeyFound && s.radio.Selected() == 0)
	if enterDirectlySelected {
		b.WriteString("\n\n")
		b.WriteString(s.keyInput.View())
	}

	if s.err != nil {
		b.WriteString("\n\n")
		b.WriteString(styles.ErrorText.Render("Error: " + s.err.Error()))
	}

	b.WriteString("\n\n")
	if enterDirectlySelected {
		b.WriteString(styles.HelpText.Render(NavigationHelpWithInput()))
	} else {
		b.WriteString(styles.HelpText.Render(NavigationHelp()))
	}

	return b.String()
}

// Validate validates the step
func (s *APIKeyStep) Validate() error {
	// All options are valid - even skip
	// If entering directly, key can be empty (will use env var fallback)
	return nil
}

// Apply applies the step values to config
func (s *APIKeyStep) Apply(cfg *config.Config) error {
	if s.envKeyFound {
		// When env var found, options are: [Use env, Enter directly, Skip]
		switch s.radio.Selected() {
		case 0: // Use environment variable
			cfg.Claude.APIKey = os.Getenv(config.ClaudeAPIKeyEnv)
		case 1: // Enter directly
			cfg.Claude.APIKey = s.keyInput.Value()
		case 2: // Skip
			cfg.Claude.APIKey = ""
		}
	} else {
		// When env var not found, options are: [Enter directly, Skip]
		switch s.radio.Selected() {
		case 0: // Enter directly
			cfg.Claude.APIKey = s.keyInput.Value()
		case 1: // Skip
			cfg.Claude.APIKey = ""
		}
	}
	return nil
}
