package steps

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

const defaultHTTPPort = 7600

// HTTPPortStep handles HTTP API port configuration
type HTTPPortStep struct {
	radio      *components.RadioGroup
	portInput  *components.TextInput
	err        error
	focusIndex int // 0 = radio, 1 = port input (when visible)
}

// NewHTTPPortStep creates a new HTTP port configuration step
func NewHTTPPortStep() *HTTPPortStep {
	return &HTTPPortStep{}
}

// Title returns the step title
func (s *HTTPPortStep) Title() string {
	return "HTTP API"
}

// Init initializes the step
func (s *HTTPPortStep) Init(cfg *config.Config) tea.Cmd {
	s.err = nil
	s.focusIndex = 0

	options := []components.RadioOption{
		{Label: fmt.Sprintf("Enable (port %d)", defaultHTTPPort), Description: "Recommended - default"},
		{Label: "Enable (custom port)", Description: "Specify a different port"},
		{Label: "Disable", Description: "No HTTP API"},
	}

	s.radio = components.NewRadioGroup(options)
	s.radio.Focus()

	s.portInput = components.NewTextInput("Port").
		WithPlaceholder("1024-65535").
		WithWidth(10)

	// Pre-select based on existing config
	// Default to "Enable (port 7600)" unless explicitly configured otherwise
	if cfg.Daemon.HTTPPort > 0 && cfg.Daemon.HTTPPort != defaultHTTPPort {
		s.radio.SetSelected(1)
		s.portInput.SetValue(strconv.Itoa(cfg.Daemon.HTTPPort))
	} else {
		// Default to recommended option (Enable with default port)
		s.radio.SetSelected(0)
	}

	return nil
}

// Update handles input
func (s *HTTPPortStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
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
			if s.radio.Selected() == 1 {
				s.focusIndex = (s.focusIndex + 1) % 2
				if s.focusIndex == 0 {
					s.radio.Focus()
					s.portInput.Blur()
				} else {
					s.radio.Blur()
					return s.portInput.Focus(), StepContinue
				}
			}
			return nil, StepContinue

		case "shift+tab":
			if s.radio.Selected() == 1 && s.focusIndex == 1 {
				s.focusIndex = 0
				s.radio.Focus()
				s.portInput.Blur()
			}
			return nil, StepContinue
		}
	}

	// Delegate to focused component
	if s.focusIndex == 1 && s.radio.Selected() == 1 {
		cmd := s.portInput.Update(msg)
		return cmd, StepContinue
	}

	// Handle radio selection change
	oldSelected := s.radio.Selected()
	s.radio.Update(msg)
	newSelected := s.radio.Selected()

	if oldSelected != 1 && newSelected == 1 {
		s.focusIndex = 1
		s.radio.Blur()
		return s.portInput.Focus(), StepContinue
	} else if oldSelected == 1 && newSelected != 1 {
		s.focusIndex = 0
		s.radio.Focus()
		s.portInput.Blur()
	}

	return nil, StepContinue
}

// View renders the step
func (s *HTTPPortStep) View() string {
	var b strings.Builder

	b.WriteString(styles.Subtitle.Render("Configure HTTP API for health checks and MCP notifications"))
	b.WriteString("\n\n")

	b.WriteString(s.radio.View())

	if s.radio.Selected() == 1 {
		b.WriteString("\n\n")
		b.WriteString(s.portInput.ViewInline())
	}

	if s.err != nil {
		b.WriteString("\n\n")
		b.WriteString(styles.ErrorText.Render("Error: " + s.err.Error()))
	}

	b.WriteString("\n\n")
	if s.radio.Selected() == 1 {
		b.WriteString(styles.HelpText.Render(NavigationHelpWithInput()))
	} else {
		b.WriteString(styles.HelpText.Render(NavigationHelp()))
	}

	return b.String()
}

// Validate validates the step
func (s *HTTPPortStep) Validate() error {
	if s.radio.Selected() == 1 {
		portStr := s.portInput.Value()
		if portStr == "" {
			return fmt.Errorf("port number is required")
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port number")
		}

		if port < 1 || port > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}

		if port < 1024 {
			// Warning but not error
			return nil
		}
	}
	return nil
}

// Apply applies the step values to config
func (s *HTTPPortStep) Apply(cfg *config.Config) error {
	switch s.radio.Selected() {
	case 0: // Default port
		cfg.Daemon.HTTPPort = defaultHTTPPort
		cfg.MCP.DaemonPort = defaultHTTPPort
		cfg.MCP.DaemonHost = "localhost"
	case 1: // Custom port
		port, _ := strconv.Atoi(s.portInput.Value())
		cfg.Daemon.HTTPPort = port
		cfg.MCP.DaemonPort = port
		cfg.MCP.DaemonHost = "localhost"
	case 2: // Disabled
		cfg.Daemon.HTTPPort = 0
		cfg.MCP.DaemonPort = 0
	}
	return nil
}
