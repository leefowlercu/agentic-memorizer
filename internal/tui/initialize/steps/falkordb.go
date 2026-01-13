package steps

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"

	"github.com/leefowlercu/agentic-memorizer/internal/container"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// FalkorDB step phases.
type falkorDBPhase int

const (
	phaseRuntimeSelect falkorDBPhase = iota
	phaseStarting
	phaseCustomEntry
	phaseComplete
)

// Option value for existing FalkorDB instance.
const optionExisting = "existing"

// containerProgressMsg is sent during container startup with status updates.
type containerProgressMsg struct {
	progress container.StartProgress
}

// delayCompleteMsg is sent after the success display delay completes.
type delayCompleteMsg struct{}

// FalkorDBStep handles FalkorDB deployment configuration.
type FalkorDBStep struct {
	BaseStep

	phase         falkorDBPhase
	radio         components.RadioGroup
	hostInput     components.TextInput
	portInput     components.TextInput
	runtimes      []container.Runtime
	startErr      error
	useExisting   bool
	spinner       spinner.Model
	statusMessage string
	progressChan  chan container.StartProgress
}

// NewFalkorDBStep creates a new FalkorDB configuration step.
func NewFalkorDBStep() *FalkorDBStep {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Primary)

	return &FalkorDBStep{
		BaseStep:  NewBaseStep("FalkorDB"),
		hostInput: components.NewTextInput("Host:", "localhost"),
		portInput: components.NewTextInput("Port:", "6379"),
		spinner:   s,
	}
}

// Init initializes the step with runtime detection.
func (s *FalkorDBStep) Init(cfg *viper.Viper) tea.Cmd {
	slog.Debug("initializing FalkorDB step")

	// Detect available container runtimes
	s.runtimes = container.AvailableRuntimes()
	slog.Debug("detected container runtimes", "count", len(s.runtimes))

	// Build radio options based on available runtimes
	var options []components.RadioOption

	for _, rt := range s.runtimes {
		options = append(options, components.RadioOption{
			Label:       fmt.Sprintf("Start FalkorDB in %s", rt.DisplayName()),
			Value:       rt.String(),
			Description: fmt.Sprintf("Automatically start a FalkorDB container using %s", rt.DisplayName()),
		})
	}

	// Always offer existing instance option
	options = append(options, components.RadioOption{
		Label:       "Use existing FalkorDB instance",
		Value:       optionExisting,
		Description: "Connect to an existing FalkorDB server",
	})

	s.radio = components.NewRadioGroup(options)
	s.phase = phaseRuntimeSelect

	// Pre-fill with existing config values if available
	if host := cfg.GetString("graph.host"); host != "" {
		s.hostInput.SetValue(host)
		slog.Debug("pre-filled host from config", "host", host)
	}
	if port := cfg.GetInt("graph.port"); port != 0 {
		s.portInput.SetValue(strconv.Itoa(port))
		slog.Debug("pre-filled port from config", "port", port)
	}

	return nil
}

// Update handles input and manages container startup.
func (s *FalkorDBStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	switch msg := msg.(type) {
	case delayCompleteMsg:
		slog.Debug("delay complete, advancing to next step")
		return nil, StepNext

	case containerProgressMsg:
		s.statusMessage = msg.progress.Message

		switch msg.progress.Phase {
		case container.PhaseComplete:
			s.phase = phaseComplete
			slog.Debug("container startup complete, showing success for 1.5 seconds")
			return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
				return delayCompleteMsg{}
			}), StepContinue
		case container.PhaseFailed:
			s.startErr = msg.progress.Err
			s.phase = phaseRuntimeSelect
			return nil, StepContinue
		default:
			// Continue waiting for more progress updates
			return s.waitForNextProgress(), StepContinue
		}

	case spinner.TickMsg:
		if s.phase == phaseStarting {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return cmd, StepContinue
		}
		return nil, StepContinue

	case tea.KeyMsg:
		switch s.phase {
		case phaseRuntimeSelect:
			return s.handleRuntimeSelect(msg)
		case phaseCustomEntry:
			return s.handleCustomEntry(msg)
		case phaseStarting:
			// Ignore input while starting
			return nil, StepContinue
		}
	}

	return nil, StepContinue
}

func (s *FalkorDBStep) handleRuntimeSelect(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		selected := s.radio.Selected()
		slog.Debug("runtime option selected", "selection", selected)

		if selected == optionExisting {
			slog.Debug("user selected existing FalkorDB instance")
			s.useExisting = true
			s.phase = phaseCustomEntry
			s.hostInput.Focus()
			return nil, StepContinue
		}

		// Start container with selected runtime
		slog.Info("starting FalkorDB container", "runtime", selected)
		s.phase = phaseStarting
		s.startErr = nil
		return s.startContainer(container.Runtime(selected)), StepContinue

	case tea.KeyEsc:
		slog.Debug("user pressed escape in runtime selection")
		return nil, StepPrev

	default:
		s.radio, _ = s.radio.Update(msg)
		return nil, StepContinue
	}
}

func (s *FalkorDBStep) handleCustomEntry(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		if err := s.Validate(); err == nil {
			return nil, StepNext
		}
		return nil, StepContinue

	case tea.KeyEsc:
		s.phase = phaseRuntimeSelect
		s.hostInput.Blur()
		s.portInput.Blur()
		return nil, StepContinue

	case tea.KeyTab, tea.KeyShiftTab:
		// Toggle between host and port inputs
		if s.hostInput.Focused() {
			s.hostInput.Blur()
			s.portInput.Focus()
		} else {
			s.portInput.Blur()
			s.hostInput.Focus()
		}
		return nil, StepContinue

	default:
		// Update the focused input
		if s.hostInput.Focused() {
			s.hostInput, _ = s.hostInput.Update(msg)
		} else if s.portInput.Focused() {
			s.portInput, _ = s.portInput.Update(msg)
		}
		return nil, StepContinue
	}
}

func (s *FalkorDBStep) startContainer(runtime container.Runtime) tea.Cmd {
	slog.Debug("initiating container startup", "runtime", runtime)
	s.progressChan = make(chan container.StartProgress)

	go container.StartFalkorDBWithProgress(runtime, container.StartOptions{
		Port:   6379,
		Detach: true,
	}, s.progressChan)

	return tea.Batch(
		s.spinner.Tick,
		s.waitForNextProgress(),
	)
}

func (s *FalkorDBStep) waitForNextProgress() tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-s.progressChan
		if !ok {
			return containerProgressMsg{progress: container.StartProgress{
				Phase:   container.PhaseComplete,
				Message: "Container startup complete",
			}}
		}
		return containerProgressMsg{progress: progress}
	}
}

// View renders the step UI.
func (s *FalkorDBStep) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Secondary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("FalkorDB Graph Database"))
	b.WriteString("\n\n")

	switch s.phase {
	case phaseRuntimeSelect:
		b.WriteString(s.viewRuntimeSelect())
	case phaseStarting:
		b.WriteString(s.viewStarting())
	case phaseCustomEntry:
		b.WriteString(s.viewCustomEntry())
	case phaseComplete:
		b.WriteString(FormatSuccess("FalkorDB is ready"))
	}

	b.WriteString("\n")

	// Show error if any
	if s.startErr != nil {
		b.WriteString("\n")
		b.WriteString(FormatError(s.startErr))
		b.WriteString("\n")
	}

	// Navigation help
	b.WriteString("\n")
	if s.phase == phaseCustomEntry {
		b.WriteString(NavigationHelpWithInput())
	} else {
		b.WriteString(NavigationHelp())
	}

	return b.String()
}

func (s *FalkorDBStep) viewRuntimeSelect() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	b.WriteString(mutedStyle.Render("Select how to connect to FalkorDB:"))
	b.WriteString("\n\n")

	b.WriteString(s.radio.View())

	// Show runtime detection status
	if len(s.runtimes) == 0 {
		b.WriteString("\n")
		b.WriteString(FormatWarning("No container runtime detected (Docker or Podman)"))
	}

	return b.String()
}

func (s *FalkorDBStep) viewStarting() string {
	var b strings.Builder

	b.WriteString(s.spinner.View())
	b.WriteString(" ")

	if s.statusMessage != "" {
		b.WriteString(s.statusMessage)
	} else {
		b.WriteString("Starting FalkorDB...")
	}
	b.WriteString("\n")

	return b.String()
}

func (s *FalkorDBStep) viewCustomEntry() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	b.WriteString(mutedStyle.Render("Enter FalkorDB connection details:"))
	b.WriteString("\n\n")

	b.WriteString(s.hostInput.View())
	b.WriteString("\n\n")
	b.WriteString(s.portInput.View())

	return b.String()
}

// Validate checks the step configuration.
func (s *FalkorDBStep) Validate() error {
	if s.phase == phaseCustomEntry || s.useExisting {
		host := strings.TrimSpace(s.hostInput.Value())
		if host == "" {
			return errors.New("host is required")
		}

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
	}

	return nil
}

// Apply writes the FalkorDB configuration.
func (s *FalkorDBStep) Apply(cfg *viper.Viper) error {
	host := strings.TrimSpace(s.hostInput.Value())
	if host == "" {
		host = "localhost"
	}

	portStr := strings.TrimSpace(s.portInput.Value())
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 6379
	}

	slog.Debug("applying FalkorDB configuration", "host", host, "port", port)
	cfg.Set("graph.host", host)
	cfg.Set("graph.port", port)

	return nil
}
