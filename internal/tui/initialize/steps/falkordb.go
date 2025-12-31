package steps

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/container"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// FalkorDBStep handles FalkorDB configuration
type FalkorDBStep struct {
	radio             *components.RadioGroup
	hostInput         *components.TextInput
	portInput         *components.TextInput
	passwordInput     *components.TextInput
	dockerAvailable   bool
	podmanAvailable   bool
	falkorRunning     bool
	err               error
	focusIndex        int               // 0 = radio, 1-3 = inputs
	startingContainer bool              // True when container runtime is starting FalkorDB
	containerStarted  bool              // True when container was successfully started
	port              int               // Port from config for container startup
	selectedRuntime   container.Runtime // Runtime selected for starting FalkorDB
}

// NewFalkorDBStep creates a new FalkorDB configuration step
func NewFalkorDBStep() *FalkorDBStep {
	return &FalkorDBStep{}
}

// Title returns the step title
func (s *FalkorDBStep) Title() string {
	return "FalkorDB Configuration"
}

// Init initializes the step
func (s *FalkorDBStep) Init(cfg *config.Config) tea.Cmd {
	s.err = nil
	s.focusIndex = 0
	s.startingContainer = false
	s.containerStarted = false
	s.port = cfg.Graph.Port
	s.selectedRuntime = container.RuntimeNone

	// Check runtime availability
	s.dockerAvailable = container.IsDockerAvailable()
	s.podmanAvailable = container.IsPodmanAvailable()

	// Check if FalkorDB is already running
	s.falkorRunning = container.IsFalkorDBRunning(cfg.Graph.Port)

	// Build options based on available runtimes
	var options []components.RadioOption
	if s.falkorRunning {
		options = []components.RadioOption{
			{Label: "Use existing instance", Description: fmt.Sprintf("FalkorDB detected on port %d", cfg.Graph.Port)},
			{Label: "Custom configuration", Description: "Specify host, port, and password"},
		}
	} else if s.dockerAvailable && s.podmanAvailable {
		// Both runtimes available
		options = []components.RadioOption{
			{Label: "Start FalkorDB in Docker", Description: "Automatic setup using Docker"},
			{Label: "Start FalkorDB in Podman", Description: "Automatic setup using Podman"},
			{Label: "Use default settings", Description: fmt.Sprintf("localhost:%d", cfg.Graph.Port)},
			{Label: "Custom configuration", Description: "Specify host, port, and password"},
		}
	} else if s.dockerAvailable {
		// Docker only
		options = []components.RadioOption{
			{Label: "Start FalkorDB in Docker", Description: "Recommended - automatic setup"},
			{Label: "Use default settings", Description: fmt.Sprintf("localhost:%d", cfg.Graph.Port)},
			{Label: "Custom configuration", Description: "Specify host, port, and password"},
		}
	} else if s.podmanAvailable {
		// Podman only
		options = []components.RadioOption{
			{Label: "Start FalkorDB in Podman", Description: "Recommended - automatic setup"},
			{Label: "Use default settings", Description: fmt.Sprintf("localhost:%d", cfg.Graph.Port)},
			{Label: "Custom configuration", Description: "Specify host, port, and password"},
		}
	} else {
		// Neither available
		options = []components.RadioOption{
			{Label: "Use default settings", Description: fmt.Sprintf("localhost:%d", cfg.Graph.Port)},
			{Label: "Custom configuration", Description: "Specify host, port, and password"},
		}
	}

	s.radio = components.NewRadioGroup(options)
	s.radio.Focus()

	s.hostInput = components.NewTextInput("Host").
		WithPlaceholder("localhost").
		WithWidth(30)
	s.hostInput.SetValue(cfg.Graph.Host)

	s.portInput = components.NewTextInput("Port").
		WithPlaceholder("6379").
		WithWidth(10)
	s.portInput.SetValue(strconv.Itoa(cfg.Graph.Port))

	s.passwordInput = components.NewTextInput("Password").
		WithPlaceholder("optional").
		WithMasked().
		WithWidth(30)
	if cfg.Graph.Password != "" {
		s.passwordInput.SetValue(cfg.Graph.Password)
	}

	return nil
}

// Update handles input
func (s *FalkorDBStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	switch msg := msg.(type) {
	case containerStartedMsg:
		s.startingContainer = false
		if msg.err != nil {
			s.err = fmt.Errorf("failed to start FalkorDB; %w", msg.err)
			return nil, StepContinue
		}
		s.containerStarted = true
		s.falkorRunning = true
		s.err = nil
		// Auto-advance to next step after successful container start
		return nil, StepNext

	case tea.KeyMsg:
		if s.startingContainer {
			// Ignore input while starting container
			return nil, StepContinue
		}

		switch msg.String() {
		case "enter":
			// Check if we need to start a container
			runtime := s.needsContainerStart()
			if runtime != container.RuntimeNone && !s.containerStarted {
				s.startingContainer = true
				s.selectedRuntime = runtime
				return s.startFalkorDB(runtime), StepContinue
			}

			if err := s.Validate(); err != nil {
				s.err = err
				return nil, StepContinue
			}
			s.err = nil
			return nil, StepNext

		case "esc":
			return nil, StepPrev

		case "tab":
			if s.isCustomSelected() {
				s.focusIndex = (s.focusIndex + 1) % 4
				return s.updateFocus(), StepContinue
			}
			return nil, StepContinue

		case "shift+tab":
			if s.isCustomSelected() && s.focusIndex > 0 {
				s.focusIndex--
				return s.updateFocus(), StepContinue
			}
			return nil, StepContinue
		}
	}

	// Delegate to focused component
	if s.isCustomSelected() && s.focusIndex > 0 {
		var cmd tea.Cmd
		switch s.focusIndex {
		case 1:
			cmd = s.hostInput.Update(msg)
		case 2:
			cmd = s.portInput.Update(msg)
		case 3:
			cmd = s.passwordInput.Update(msg)
		}
		return cmd, StepContinue
	}

	s.radio.Update(msg)
	return nil, StepContinue
}

// View renders the step
func (s *FalkorDBStep) View() string {
	var b strings.Builder

	b.WriteString(styles.Subtitle.Render("Configure FalkorDB graph database connection"))
	b.WriteString("\n\n")

	if s.startingContainer {
		runtimeName := container.GetRuntime(s.selectedRuntime)
		b.WriteString(styles.Focused.Render(fmt.Sprintf("Starting FalkorDB in %s...", runtimeName)))
		b.WriteString("\n")
		return b.String()
	}

	if s.containerStarted {
		b.WriteString(styles.SuccessText.Render("FalkorDB started successfully"))
		b.WriteString("\n\n")
	}

	b.WriteString(s.radio.View())

	if s.isCustomSelected() {
		b.WriteString("\n\n")
		b.WriteString(s.hostInput.View())
		b.WriteString("\n")
		b.WriteString(s.portInput.View())
		b.WriteString("\n")
		b.WriteString(s.passwordInput.View())
	}

	if s.err != nil {
		b.WriteString("\n\n")
		b.WriteString(styles.ErrorText.Render("Error: " + s.err.Error()))
	}

	b.WriteString("\n\n")
	if s.isCustomSelected() {
		b.WriteString(styles.HelpText.Render(NavigationHelpWithInput()))
	} else {
		b.WriteString(styles.HelpText.Render(NavigationHelp()))
	}

	return b.String()
}

// Validate validates the step
func (s *FalkorDBStep) Validate() error {
	// If container start option selected but not started, this is an error
	if s.needsContainerStart() != container.RuntimeNone && !s.containerStarted {
		return fmt.Errorf("FalkorDB must be started before continuing")
	}

	if s.isCustomSelected() {
		host := s.hostInput.Value()
		if host == "" {
			return fmt.Errorf("host is required")
		}

		portStr := s.portInput.Value()
		if portStr == "" {
			return fmt.Errorf("port is required")
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port number")
		}

		if port < 1 || port > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
	}

	return nil
}

// Apply applies the step values to config
func (s *FalkorDBStep) Apply(cfg *config.Config) error {
	if s.isCustomSelected() {
		cfg.Graph.Host = s.hostInput.Value()
		port, _ := strconv.Atoi(s.portInput.Value())
		cfg.Graph.Port = port
		cfg.Graph.Password = s.passwordInput.Value()
	}
	// For other options, keep defaults or use Docker defaults
	return nil
}

// Helper methods

func (s *FalkorDBStep) isCustomSelected() bool {
	selected := s.radio.Selected()
	if s.falkorRunning {
		return selected == 1
	}
	if s.dockerAvailable && s.podmanAvailable {
		// Both available: Docker=0, Podman=1, Default=2, Custom=3
		return selected == 3
	}
	if s.dockerAvailable || s.podmanAvailable {
		// One available: Start=0, Default=1, Custom=2
		return selected == 2
	}
	// Neither available: Default=0, Custom=1
	return selected == 1
}

// needsContainerStart returns the runtime to use for starting FalkorDB, or RuntimeNone if no start needed
func (s *FalkorDBStep) needsContainerStart() container.Runtime {
	if s.falkorRunning {
		return container.RuntimeNone
	}

	selected := s.radio.Selected()

	if s.dockerAvailable && s.podmanAvailable {
		// Both available: Docker=0, Podman=1, Default=2, Custom=3
		switch selected {
		case 0:
			return container.RuntimeDocker
		case 1:
			return container.RuntimePodman
		}
	} else if s.dockerAvailable {
		// Docker only: Start=0, Default=1, Custom=2
		if selected == 0 {
			return container.RuntimeDocker
		}
	} else if s.podmanAvailable {
		// Podman only: Start=0, Default=1, Custom=2
		if selected == 0 {
			return container.RuntimePodman
		}
	}

	return container.RuntimeNone
}

func (s *FalkorDBStep) updateFocus() tea.Cmd {
	s.radio.Blur()
	s.hostInput.Blur()
	s.portInput.Blur()
	s.passwordInput.Blur()

	switch s.focusIndex {
	case 0:
		s.radio.Focus()
		return nil
	case 1:
		return s.hostInput.Focus()
	case 2:
		return s.portInput.Focus()
	case 3:
		return s.passwordInput.Focus()
	}
	return nil
}

// Container start message
type containerStartedMsg struct {
	err error
}

func (s *FalkorDBStep) startFalkorDB(runtime container.Runtime) tea.Cmd {
	return func() tea.Msg {
		// Get app directory for persistent data
		appDir, err := config.GetAppDir()
		if err != nil {
			return containerStartedMsg{err: fmt.Errorf("failed to get app directory; %w", err)}
		}

		opts := container.StartOptions{
			Port:    s.port,
			DataDir: fmt.Sprintf("%s/falkordb", appDir),
			Detach:  true,
		}
		err = container.StartFalkorDB(runtime, opts)
		return containerStartedMsg{err: err}
	}
}
