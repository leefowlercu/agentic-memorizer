package steps

import (
	"errors"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"

	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

const defaultHTTPPort = 7600

// HTTPPortStep handles HTTP API port configuration.
type HTTPPortStep struct {
	BaseStep

	portInput components.TextInput
	err       error
}

// NewHTTPPortStep creates a new HTTP port configuration step.
func NewHTTPPortStep() *HTTPPortStep {
	return &HTTPPortStep{
		BaseStep:  NewBaseStep("HTTP Port"),
		portInput: components.NewTextInput("Port:", strconv.Itoa(defaultHTTPPort)),
	}
}

// Init initializes the step.
func (s *HTTPPortStep) Init(cfg *viper.Viper) tea.Cmd {
	// Pre-fill from existing config
	if port := cfg.GetInt("http.port"); port != 0 {
		s.portInput.SetValue(strconv.Itoa(port))
	}

	s.portInput.Focus()
	s.err = nil

	return nil
}

// Update handles input.
func (s *HTTPPortStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, StepContinue
	}

	switch keyMsg.Type {
	case tea.KeyEnter:
		if err := s.Validate(); err != nil {
			s.err = err
			return nil, StepContinue
		}
		s.err = nil
		return nil, StepNext

	case tea.KeyEsc:
		return nil, StepPrev

	default:
		s.portInput, _ = s.portInput.Update(msg)
		// Clear error on input change
		s.err = nil
		return nil, StepContinue
	}
}

// View renders the step UI.
func (s *HTTPPortStep) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Secondary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Daemon HTTP Port"))
	b.WriteString("\n\n")

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	b.WriteString(mutedStyle.Render("Configure the HTTP API port for the daemon:"))
	b.WriteString("\n\n")

	b.WriteString(s.portInput.View())
	b.WriteString("\n\n")

	b.WriteString(mutedStyle.Render("Default: 7600. The daemon listens on this port for API requests."))

	// Show error if any
	if s.err != nil {
		b.WriteString("\n\n")
		b.WriteString(FormatError(s.err))
	}

	b.WriteString("\n\n")
	b.WriteString(NavigationHelpWithInput())

	return b.String()
}

// Validate checks the port configuration.
func (s *HTTPPortStep) Validate() error {
	portStr := strings.TrimSpace(s.portInput.Value())
	if portStr == "" {
		return errors.New("port is required")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return errors.New("port must be a number")
	}

	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	return nil
}

// Apply writes the port configuration.
func (s *HTTPPortStep) Apply(cfg *viper.Viper) error {
	portStr := strings.TrimSpace(s.portInput.Value())
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = defaultHTTPPort
	}

	cfg.Set("http.port", port)

	return nil
}
