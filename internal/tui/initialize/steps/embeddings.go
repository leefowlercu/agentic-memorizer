package steps

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// EmbeddingsStep handles embeddings configuration
type EmbeddingsStep struct {
	enableRadio  *components.RadioGroup
	keyRadio     *components.RadioGroup
	keyInput     *components.TextInput
	envKeyFound  bool
	err          error
	focusIndex   int // 0 = enable radio, 1 = key radio, 2 = key input
	showKeySetup bool
}

// NewEmbeddingsStep creates a new embeddings configuration step
func NewEmbeddingsStep() *EmbeddingsStep {
	return &EmbeddingsStep{}
}

// Title returns the step title
func (s *EmbeddingsStep) Title() string {
	return "Embeddings"
}

// Init initializes the step
func (s *EmbeddingsStep) Init(cfg *config.Config) tea.Cmd {
	s.envKeyFound = os.Getenv(config.EmbeddingsAPIKeyEnv) != ""
	s.err = nil
	s.focusIndex = 0
	s.showKeySetup = false

	// Enable/disable options
	var enableOptions []components.RadioOption
	if s.envKeyFound {
		enableOptions = []components.RadioOption{
			{Label: "Enable embeddings", Description: config.EmbeddingsAPIKeyEnv + " detected"},
			{Label: "Disable embeddings", Description: "Skip vector similarity search"},
		}
	} else {
		enableOptions = []components.RadioOption{
			{Label: "Enable embeddings", Description: "Requires OpenAI API key"},
			{Label: "Disable embeddings", Description: "Skip vector similarity search"},
		}
	}

	s.enableRadio = components.NewRadioGroup(enableOptions)
	s.enableRadio.Focus()

	// Key configuration options
	keyOptions := []components.RadioOption{
		{Label: "Use environment variable", Description: "Set " + config.EmbeddingsAPIKeyEnv},
		{Label: "Enter API key directly", Description: "Will be stored in config file"},
	}
	s.keyRadio = components.NewRadioGroup(keyOptions)

	s.keyInput = components.NewTextInput("OpenAI API Key").
		WithPlaceholder("sk-...").
		WithMasked().
		WithWidth(60)

	// Default to "Enable embeddings" (recommended)
	s.enableRadio.SetSelected(0)
	s.showKeySetup = true

	// Pre-populate API key if already configured
	if cfg.Embeddings.APIKey != "" {
		s.keyRadio.SetSelected(1)
		s.keyInput.SetValue(cfg.Embeddings.APIKey)
	}

	return nil
}

// Update handles input
func (s *EmbeddingsStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
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
			return s.nextFocus(), StepContinue

		case "shift+tab":
			return s.prevFocus(), StepContinue
		}
	}

	// Delegate to focused component
	switch s.focusIndex {
	case 0:
		oldSelected := s.enableRadio.Selected()
		s.enableRadio.Update(msg)
		newSelected := s.enableRadio.Selected()

		// Toggle key setup visibility
		if newSelected == 0 {
			s.showKeySetup = true
		} else {
			s.showKeySetup = false
		}

		// Auto-advance to key setup
		if oldSelected != 0 && newSelected == 0 && !s.envKeyFound {
			s.focusIndex = 1
			s.enableRadio.Blur()
			s.keyRadio.Focus()
		}

	case 1:
		if s.showKeySetup {
			oldSelected := s.keyRadio.Selected()
			s.keyRadio.Update(msg)
			newSelected := s.keyRadio.Selected()

			if oldSelected != 1 && newSelected == 1 {
				s.focusIndex = 2
				s.keyRadio.Blur()
				return s.keyInput.Focus(), StepContinue
			}
		}

	case 2:
		if s.showKeySetup && s.keyRadio.Selected() == 1 {
			cmd := s.keyInput.Update(msg)
			return cmd, StepContinue
		}
	}

	return nil, StepContinue
}

// View renders the step
func (s *EmbeddingsStep) View() string {
	var b strings.Builder

	b.WriteString(styles.Subtitle.Render("Configure vector embeddings for similarity search"))
	b.WriteString("\n\n")

	b.WriteString(s.enableRadio.View())

	if s.showKeySetup && !s.envKeyFound {
		b.WriteString("\n\n")
		b.WriteString(styles.Label.Render("API Key Configuration:"))
		b.WriteString("\n")
		b.WriteString(s.keyRadio.View())

		if s.keyRadio.Selected() == 1 {
			b.WriteString("\n\n")
			b.WriteString(s.keyInput.View())
		}
	} else if s.showKeySetup && s.envKeyFound {
		b.WriteString("\n\n")
		b.WriteString(styles.SuccessText.Render("Using " + config.EmbeddingsAPIKeyEnv + " from environment"))
	}

	if s.err != nil {
		b.WriteString("\n\n")
		b.WriteString(styles.ErrorText.Render("Error: " + s.err.Error()))
	}

	b.WriteString("\n\n")
	if s.showKeySetup && s.keyRadio.Selected() == 1 {
		b.WriteString(styles.HelpText.Render(NavigationHelpWithInput()))
	} else {
		b.WriteString(styles.HelpText.Render(NavigationHelp()))
	}

	return b.String()
}

// Validate validates the step
func (s *EmbeddingsStep) Validate() error {
	// All options are valid
	return nil
}

// Apply applies the step values to config
func (s *EmbeddingsStep) Apply(cfg *config.Config) error {
	if s.enableRadio.Selected() == 0 {
		cfg.Embeddings.Enabled = true
		if s.keyRadio.Selected() == 1 {
			cfg.Embeddings.APIKey = s.keyInput.Value()
		} else {
			cfg.Embeddings.APIKey = ""
		}
	} else {
		cfg.Embeddings.Enabled = false
		cfg.Embeddings.APIKey = ""
	}
	return nil
}

// Helper methods

func (s *EmbeddingsStep) nextFocus() tea.Cmd {
	if !s.showKeySetup {
		return nil
	}

	maxFocus := 1
	if s.keyRadio.Selected() == 1 {
		maxFocus = 2
	}

	s.focusIndex++
	if s.focusIndex > maxFocus {
		s.focusIndex = 0
	}

	return s.updateFocus()
}

func (s *EmbeddingsStep) prevFocus() tea.Cmd {
	if !s.showKeySetup {
		return nil
	}

	s.focusIndex--
	if s.focusIndex < 0 {
		maxFocus := 1
		if s.keyRadio.Selected() == 1 {
			maxFocus = 2
		}
		s.focusIndex = maxFocus
	}

	return s.updateFocus()
}

func (s *EmbeddingsStep) updateFocus() tea.Cmd {
	s.enableRadio.Blur()
	s.keyRadio.Blur()
	s.keyInput.Blur()

	switch s.focusIndex {
	case 0:
		s.enableRadio.Focus()
		return nil
	case 1:
		s.keyRadio.Focus()
		return nil
	case 2:
		return s.keyInput.Focus()
	}
	return nil
}
