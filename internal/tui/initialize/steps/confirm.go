package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// ConfirmStep shows a summary and asks for confirmation
type ConfirmStep struct {
	radio                *components.RadioGroup
	config               *config.Config
	selectedIntegrations []string
	err                  error
}

// NewConfirmStep creates a new confirmation step
func NewConfirmStep() *ConfirmStep {
	return &ConfirmStep{}
}

// Title returns the step title
func (s *ConfirmStep) Title() string {
	return "Confirm"
}

// Init initializes the step
func (s *ConfirmStep) Init(cfg *config.Config) tea.Cmd {
	return s.InitWithIntegrations(cfg, nil)
}

// InitWithIntegrations initializes with explicit integration list
func (s *ConfirmStep) InitWithIntegrations(cfg *config.Config, selectedIntegrations []string) tea.Cmd {
	s.config = cfg
	s.selectedIntegrations = selectedIntegrations
	s.err = nil

	// Generate intelligent description based on NEW integrations being set up
	description := "Write config, setup app directory"

	// Check if any integrations are newly selected (not already configured)
	hasNewIntegrations := false
	if len(selectedIntegrations) > 0 {
		registry := integrations.GlobalRegistry()
		for _, name := range selectedIntegrations {
			integration, err := registry.Get(name)
			if err != nil {
				continue
			}
			// Check if this integration is NOT already configured
			alreadyConfigured, _ := integration.IsEnabled()
			if !alreadyConfigured {
				hasNewIntegrations = true
				break
			}
		}
	}

	if hasNewIntegrations {
		description += ", and enable integrations"
	}

	options := []components.RadioOption{
		{Label: "Yes, create configuration", Description: description},
		{Label: "No, go back", Description: "Return to previous steps"},
	}

	s.radio = components.NewRadioGroup(options)
	s.radio.Focus()

	return nil
}

// Update handles input
func (s *ConfirmStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if s.radio.Selected() == 0 {
				return nil, StepNext // Confirmed
			}
			return nil, StepPrev // Go back

		case "esc":
			return nil, StepPrev
		}
	}

	s.radio.Update(msg)
	return nil, StepContinue
}

// View renders the step
func (s *ConfirmStep) View() string {
	var b strings.Builder

	b.WriteString(styles.Subtitle.Render("Review your configuration"))
	b.WriteString("\n\n")

	// Summary
	b.WriteString(s.renderSummary())
	b.WriteString("\n\n")

	// Confirmation
	b.WriteString(styles.Label.Render("Proceed with initialization?"))
	b.WriteString("\n\n")
	b.WriteString(s.radio.View())

	if s.err != nil {
		b.WriteString("\n\n")
		b.WriteString(styles.ErrorText.Render("Error: " + s.err.Error()))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.HelpText.Render(NavigationHelp()))

	return b.String()
}

// Validate validates the step
func (s *ConfirmStep) Validate() error {
	return nil
}

// Apply applies the step values to config
func (s *ConfirmStep) Apply(cfg *config.Config) error {
	return nil
}

// IsConfirmed returns true if user selected "Yes"
func (s *ConfirmStep) IsConfirmed() bool {
	return s.radio.Selected() == 0
}

func (s *ConfirmStep) renderSummary() string {
	var b strings.Builder

	// Memory root
	b.WriteString(s.summaryLine("Memory Directory", s.config.MemoryRoot))

	// Semantic Provider
	if s.config.Semantic.Enabled && s.config.Semantic.Provider != "" {
		providerDisplay := s.config.Semantic.Provider
		if s.config.Semantic.Model != "" {
			providerDisplay += " (" + s.config.Semantic.Model + ")"
		}
		b.WriteString(s.summaryLine("Semantic Provider", providerDisplay))

		if s.config.Semantic.APIKey != "" {
			b.WriteString(s.summaryLine("API Key", s.maskKey(s.config.Semantic.APIKey)))
		} else {
			b.WriteString(s.summaryLine("API Key", "Using environment variable"))
		}
	} else {
		b.WriteString(s.summaryLine("Semantic Analysis", "Disabled"))
	}

	// HTTP API
	if s.config.Daemon.HTTPPort > 0 {
		b.WriteString(s.summaryLine("HTTP API", fmt.Sprintf("Enabled (port %d)", s.config.Daemon.HTTPPort)))
	} else {
		b.WriteString(s.summaryLine("HTTP API", "Disabled"))
	}

	// FalkorDB
	b.WriteString(s.summaryLine("FalkorDB", fmt.Sprintf("%s:%d", s.config.Graph.Host, s.config.Graph.Port)))

	// Embeddings
	if s.config.Embeddings.Enabled {
		if s.config.Embeddings.APIKey != "" {
			b.WriteString(s.summaryLine("Embeddings", "Enabled (key in config)"))
		} else {
			b.WriteString(s.summaryLine("Embeddings", "Enabled (using env var)"))
		}
	} else {
		b.WriteString(s.summaryLine("Embeddings", "Disabled"))
	}

	// Integrations
	if len(s.selectedIntegrations) > 0 {
		b.WriteString(s.summaryLine("Integrations", strings.Join(s.selectedIntegrations, ", ")))
	} else {
		b.WriteString(s.summaryLine("Integrations", "None selected"))
	}

	return b.String()
}

func (s *ConfirmStep) summaryLine(label, value string) string {
	labelStr := styles.Label.Render(fmt.Sprintf("  %-19s", label+":"))
	valueStr := styles.SuccessText.Render(value)
	return labelStr + valueStr + "\n"
}

func (s *ConfirmStep) maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
