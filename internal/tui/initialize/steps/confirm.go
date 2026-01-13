package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// ConfirmStep displays a configuration summary and asks for confirmation.
type ConfirmStep struct {
	BaseStep

	cfg *config.Config
}

// NewConfirmStep creates a new confirmation step.
func NewConfirmStep() *ConfirmStep {
	return &ConfirmStep{
		BaseStep: NewBaseStep("Confirm"),
	}
}

// Init initializes the step with the current configuration.
func (s *ConfirmStep) Init(cfg *config.Config) tea.Cmd {
	s.cfg = cfg
	return nil
}

// Update handles input.
func (s *ConfirmStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, StepContinue
	}

	switch keyMsg.Type {
	case tea.KeyEnter:
		return nil, StepNext

	case tea.KeyEsc:
		return nil, StepPrev
	}

	return nil, StepContinue
}

// View renders the configuration summary.
func (s *ConfirmStep) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Secondary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Configuration Summary"))
	b.WriteString("\n\n")

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	b.WriteString(mutedStyle.Render("Review your settings before saving:"))
	b.WriteString("\n\n")

	// Build configuration summary
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Primary).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	// FalkorDB
	b.WriteString(s.formatSection("FalkorDB"))
	b.WriteString(s.formatRow(labelStyle, valueStyle, "Host", s.cfg.Graph.Host))
	b.WriteString(s.formatRow(labelStyle, valueStyle, "Port", fmt.Sprintf("%d", s.cfg.Graph.Port)))
	b.WriteString("\n")

	// Semantic Provider
	b.WriteString(s.formatSection("Semantic Analysis"))
	b.WriteString(s.formatRow(labelStyle, valueStyle, "Provider", s.cfg.Semantic.Provider))
	b.WriteString(s.formatRow(labelStyle, valueStyle, "Model", s.cfg.Semantic.Model))
	if s.cfg.Semantic.APIKey != nil && *s.cfg.Semantic.APIKey != "" {
		b.WriteString(s.formatRow(labelStyle, valueStyle, "API Key", "********"))
	}
	b.WriteString("\n")

	// Embeddings
	b.WriteString(s.formatSection("Vector Embeddings"))
	if s.cfg.Embeddings.Enabled {
		b.WriteString(s.formatRow(labelStyle, valueStyle, "Enabled", "Yes"))
		b.WriteString(s.formatRow(labelStyle, valueStyle, "Provider", s.cfg.Embeddings.Provider))
		b.WriteString(s.formatRow(labelStyle, valueStyle, "Model", s.cfg.Embeddings.Model))
	} else {
		b.WriteString(s.formatRow(labelStyle, valueStyle, "Enabled", "No"))
	}
	b.WriteString("\n")

	// HTTP Port
	b.WriteString(s.formatSection("Daemon"))
	b.WriteString(s.formatRow(labelStyle, valueStyle, "HTTP Port", fmt.Sprintf("%d", s.cfg.Daemon.HTTPPort)))
	b.WriteString("\n")

	// Confirmation prompt
	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Success)

	b.WriteString(successStyle.Render("Press Enter to save and start the daemon."))
	b.WriteString("\n\n")

	b.WriteString(NavigationHelp())

	return b.String()
}

func (s *ConfirmStep) formatSection(title string) string {
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Secondary).
		MarginBottom(1)

	return sectionStyle.Render(title) + "\n"
}

func (s *ConfirmStep) formatRow(labelStyle, valueStyle lipgloss.Style, label, value string) string {
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value) + "\n"
}

// Validate always passes for the confirm step.
func (s *ConfirmStep) Validate() error {
	return nil
}

// Apply is a no-op for the confirm step.
func (s *ConfirmStep) Apply(cfg *config.Config) error {
	return nil
}
