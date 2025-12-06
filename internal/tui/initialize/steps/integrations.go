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

// IntegrationsStep handles integration selection
type IntegrationsStep struct {
	checkbox    *components.CheckboxList
	available   []integrations.Integration
	noAvailable bool
	err         error
}

// NewIntegrationsStep creates a new integrations selection step
func NewIntegrationsStep() *IntegrationsStep {
	return &IntegrationsStep{}
}

// Title returns the step title
func (s *IntegrationsStep) Title() string {
	return "Integrations"
}

// Init initializes the step
func (s *IntegrationsStep) Init(cfg *config.Config) tea.Cmd {
	s.err = nil

	// Detect available integrations
	registry := integrations.GlobalRegistry()
	s.available = registry.DetectAvailable()

	if len(s.available) == 0 {
		s.noAvailable = true
		s.checkbox = nil
		return nil
	}

	s.noAvailable = false

	// Build checkbox items
	items := make([]components.CheckboxItem, len(s.available))
	for i, integration := range s.available {
		description := integration.GetDescription()

		// Check if integration is already configured
		alreadyConfigured, _ := integration.IsEnabled()
		if alreadyConfigured {
			description = description + " - Already configured"
		}

		items[i] = components.CheckboxItem{
			Label:       integration.GetName(),
			Description: description,
			Value:       integration.GetName(),
			Checked:     alreadyConfigured,
			Enabled:     !alreadyConfigured, // Disable selection for already-configured integrations
		}
	}

	s.checkbox = components.NewCheckboxList(items)
	s.checkbox.Focus()

	return nil
}

// Update handles input
func (s *IntegrationsStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
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
		}
	}

	if s.checkbox != nil {
		s.checkbox.Update(msg)
	}

	return nil, StepContinue
}

// countConfiguredIntegrations returns the number of already-configured integrations
func (s *IntegrationsStep) countConfiguredIntegrations() int {
	count := 0
	for _, integration := range s.available {
		enabled, _ := integration.IsEnabled()
		if enabled {
			count++
		}
	}
	return count
}

// getIntelligentSubtitle returns context-aware subtitle based on integration state
func (s *IntegrationsStep) getIntelligentSubtitle() string {
	if s.noAvailable {
		return "No agent frameworks detected"
	}

	alreadyConfigured := s.countConfiguredIntegrations()
	available := len(s.available)

	if alreadyConfigured == 0 {
		return "Select integrations to configure"
	} else if alreadyConfigured == available {
		return "Review configured integrations"
	} else {
		return "Review and configure integrations"
	}
}

// View renders the step
func (s *IntegrationsStep) View() string {
	var b strings.Builder

	// Use intelligent subtitle based on integration state
	b.WriteString(styles.Subtitle.Render(s.getIntelligentSubtitle()))
	b.WriteString("\n\n")

	if s.noAvailable {
		b.WriteString(styles.MutedText.Render("No agent frameworks detected on this system."))
		b.WriteString("\n\n")
		b.WriteString(styles.MutedText.Render("Supported integrations:"))
		b.WriteString("\n")
		b.WriteString(styles.MutedText.Render("  - Claude Code"))
		b.WriteString("\n")
		b.WriteString(styles.MutedText.Render("  - Gemini CLI"))
		b.WriteString("\n")
		b.WriteString(styles.MutedText.Render("  - Continue.dev"))
		b.WriteString("\n")
		b.WriteString(styles.MutedText.Render("  - Cline"))
		b.WriteString("\n\n")
		b.WriteString(styles.MutedText.Render("Install a framework and run 'agentic-memorizer integrations setup <name>' later."))
	} else {
		// Show note if some integrations are already configured
		alreadyConfigured := s.countConfiguredIntegrations()
		if alreadyConfigured > 0 {
			noteText := fmt.Sprintf("Note: %d integration(s) already configured (marked disabled).", alreadyConfigured)
			b.WriteString(styles.MutedText.Render(noteText))
			b.WriteString("\n\n")
		}

		b.WriteString(s.checkbox.View())
		b.WriteString("\n\n")
		b.WriteString(s.checkbox.ViewHelp())
	}

	if s.err != nil {
		b.WriteString("\n\n")
		b.WriteString(styles.ErrorText.Render("Error: " + s.err.Error()))
	}

	b.WriteString("\n\n")
	if s.noAvailable {
		b.WriteString(styles.HelpText.Render("enter: next  esc: back  ctrl+c: quit"))
	} else {
		b.WriteString(styles.HelpText.Render(NavigationHelp()))
	}

	return b.String()
}

// Validate validates the step
func (s *IntegrationsStep) Validate() error {
	// No validation needed - selecting no integrations is valid
	return nil
}

// Apply applies the step values to config
func (s *IntegrationsStep) Apply(cfg *config.Config) error {
	if s.checkbox == nil {
		cfg.Integrations.Enabled = []string{}
		return nil
	}

	checked := s.checkbox.CheckedValues()
	enabled := make([]string, len(checked))
	for i, v := range checked {
		enabled[i] = v.(string)
	}
	cfg.Integrations.Enabled = enabled

	return nil
}

// SelectedIntegrations returns the list of selected integration names
func (s *IntegrationsStep) SelectedIntegrations() []string {
	if s.checkbox == nil {
		return nil
	}

	checked := s.checkbox.CheckedValues()
	names := make([]string, len(checked))
	for i, v := range checked {
		names[i] = v.(string)
	}
	return names
}
