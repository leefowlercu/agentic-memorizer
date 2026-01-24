package steps

import (
	"errors"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

const defaultHTTPPort = 7600

// PortChecker is a function type for checking if a port is in use.
type PortChecker func(port int) bool

// HTTPPortStep handles HTTP API port configuration.
type HTTPPortStep struct {
	BaseStep

	portInput   components.TextInput
	err         error
	warning     string
	portChecker PortChecker
}

// NewHTTPPortStep creates a new HTTP port configuration step.
func NewHTTPPortStep() *HTTPPortStep {
	return &HTTPPortStep{
		BaseStep:    NewBaseStep("HTTP Port"),
		portInput:   components.NewTextInput("Port:", strconv.Itoa(defaultHTTPPort)),
		portChecker: CheckPortInUse,
	}
}

// SetPortChecker sets a custom port checker (for testing).
func (s *HTTPPortStep) SetPortChecker(checker PortChecker) {
	s.portChecker = checker
}

// Init initializes the step.
func (s *HTTPPortStep) Init(cfg *config.Config) tea.Cmd {
	// Pre-fill from existing config
	if cfg.Daemon.HTTPPort != 0 {
		s.portInput.SetValue(strconv.Itoa(cfg.Daemon.HTTPPort))
	}

	s.portInput.Focus()
	s.err = nil
	s.warning = ""

	// Check if initial port is in use
	s.checkPortAvailability()

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
		// Clear error on input change and recheck port availability
		s.err = nil
		s.checkPortAvailability()
		return nil, StepContinue
	}
}

// checkPortAvailability checks if the current port is in use and sets a warning.
func (s *HTTPPortStep) checkPortAvailability() {
	s.warning = ""

	portStr := strings.TrimSpace(s.portInput.Value())
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return
	}

	if s.portChecker != nil && s.portChecker(port) {
		s.warning = "Port " + portStr + " is currently in use. The daemon may fail to start if the port is not freed."
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

	// Show warning if port is in use
	if s.warning != "" {
		b.WriteString("\n\n")
		b.WriteString(FormatWarning(s.warning))
	}

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
func (s *HTTPPortStep) Apply(cfg *config.Config) error {
	portStr := strings.TrimSpace(s.portInput.Value())
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = defaultHTTPPort
	}

	cfg.Daemon.HTTPPort = port

	return nil
}
