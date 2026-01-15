package steps

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/servicemanager"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

const (
	optionYes = "yes"
	optionNo  = "no"

	healthCheckTimeout  = 30 * time.Second
	healthCheckInterval = 500 * time.Millisecond
)

// installState represents the state of the installation process.
type installState int

const (
	stateSelecting installState = iota
	stateInstalling
	stateStarting
	stateWaitingHealth
	stateDone
	stateFailed
	stateSkipped
)

// ServiceInstallStep handles daemon service installation after config is saved.
type ServiceInstallStep struct {
	BaseStep

	radio         components.RadioGroup
	state         installState
	progressMsg   string
	resultMsg     string
	err           error
	cfg           *config.Config
	managerFactory func() (servicemanager.DaemonManager, error)
}

// NewServiceInstallStep creates a new service installation step.
func NewServiceInstallStep() *ServiceInstallStep {
	options := []components.RadioOption{
		{
			Label:       "Yes, install and start",
			Value:       optionYes,
			Description: "Install as system service and start the daemon now",
		},
		{
			Label:       "No, I'll start manually",
			Value:       optionNo,
			Description: "Skip service installation; start with 'memorizer daemon start'",
		},
	}

	return &ServiceInstallStep{
		BaseStep:       NewBaseStep("Start Daemon"),
		radio:          components.NewRadioGroup(options),
		state:          stateSelecting,
		managerFactory: servicemanager.NewDaemonManager,
	}
}

// Init initializes the step.
func (s *ServiceInstallStep) Init(cfg *config.Config) tea.Cmd {
	s.cfg = cfg
	s.state = stateSelecting
	s.progressMsg = ""
	s.resultMsg = ""
	s.err = nil
	return nil
}

// Update handles input.
func (s *ServiceInstallStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	// Handle async messages
	switch msg := msg.(type) {
	case installProgressMsg:
		s.progressMsg = msg.message
		s.state = msg.state
		return nil, StepContinue

	case installDoneMsg:
		s.state = stateDone
		s.resultMsg = msg.message
		return nil, StepContinue

	case installErrorMsg:
		s.state = stateFailed
		s.err = msg.err
		s.resultMsg = msg.message
		return nil, StepContinue
	}

	// In selecting state, handle navigation
	if s.state == stateSelecting {
		keyMsg, ok := msg.(tea.KeyMsg)
		if !ok {
			return nil, StepContinue
		}

		switch keyMsg.Type {
		case tea.KeyEnter:
			if s.radio.Selected() == optionYes {
				// Start installation
				return s.startInstallation(), StepContinue
			}
			// User selected No - mark as skipped
			s.state = stateSkipped
			return nil, StepContinue

		case tea.KeyEsc:
			return nil, StepPrev

		default:
			s.radio, _ = s.radio.Update(msg)
			return nil, StepContinue
		}
	}

	// In done/failed/skipped states, Enter advances
	if s.state == stateDone || s.state == stateFailed || s.state == stateSkipped {
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok && keyMsg.Type == tea.KeyEnter {
			return nil, StepNext
		}
	}

	return nil, StepContinue
}

// View renders the step UI.
func (s *ServiceInstallStep) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Secondary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Start Daemon Service"))
	b.WriteString("\n\n")

	switch s.state {
	case stateSelecting:
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		b.WriteString(mutedStyle.Render("Would you like to install and start the daemon as a system service?"))
		b.WriteString("\n\n")
		b.WriteString(s.radio.View())
		b.WriteString("\n\n")
		b.WriteString(NavigationHelp())

	case stateInstalling, stateStarting, stateWaitingHealth:
		b.WriteString(s.renderProgress())

	case stateDone:
		b.WriteString(FormatSuccess(s.resultMsg))
		b.WriteString("\n\n")
		b.WriteString(s.renderServiceInfo())
		b.WriteString("\n\n")
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		b.WriteString(mutedStyle.Render("Press Enter to finish."))

	case stateFailed:
		b.WriteString(FormatWarning(s.resultMsg))
		if s.err != nil {
			b.WriteString("\n")
			errStyle := lipgloss.NewStyle().Foreground(styles.Muted)
			b.WriteString(errStyle.Render(fmt.Sprintf("Details: %v", s.err)))
		}
		b.WriteString("\n\n")
		b.WriteString(s.renderTroubleshooting())
		b.WriteString("\n\n")
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		b.WriteString(mutedStyle.Render("Configuration was saved successfully. Press Enter to finish."))

	case stateSkipped:
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		b.WriteString(mutedStyle.Render("Service installation skipped."))
		b.WriteString("\n\n")
		b.WriteString(s.renderManualStart())
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("Press Enter to finish."))
	}

	return b.String()
}

func (s *ServiceInstallStep) renderProgress() string {
	var b strings.Builder

	steps := []struct {
		state installState
		label string
	}{
		{stateInstalling, "Installing service..."},
		{stateStarting, "Starting daemon..."},
		{stateWaitingHealth, "Waiting for health check..."},
	}

	for _, step := range steps {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(styles.Muted)

		if step.state < s.state {
			// Completed
			prefix = styles.SuccessText.Render("✓ ")
			style = lipgloss.NewStyle().Foreground(styles.Success)
		} else if step.state == s.state {
			// Current
			prefix = styles.Cursor.Render("▸ ")
			style = lipgloss.NewStyle().Foreground(styles.Highlight)
		}

		b.WriteString(prefix)
		b.WriteString(style.Render(step.label))
		b.WriteString("\n")
	}

	if s.progressMsg != "" {
		b.WriteString("\n")
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		b.WriteString(mutedStyle.Render(s.progressMsg))
	}

	return b.String()
}

func (s *ServiceInstallStep) renderServiceInfo() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Secondary)

	b.WriteString(labelStyle.Render("Service Information"))
	b.WriteString("\n")

	platform := servicemanager.DetectPlatform()
	switch platform {
	case servicemanager.PlatformMacOS:
		b.WriteString(mutedStyle.Render("  Service: com.leefowlercu.memorizer"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  Stop: launchctl stop com.leefowlercu.memorizer"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  Logs: Console.app or ~/.config/memorizer/memorizer.log"))
	case servicemanager.PlatformLinux:
		b.WriteString(mutedStyle.Render("  Service: memorizer.service"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  Stop: systemctl --user stop memorizer.service"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  Logs: journalctl --user -u memorizer.service"))
	}

	return b.String()
}

func (s *ServiceInstallStep) renderTroubleshooting() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Secondary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	b.WriteString(labelStyle.Render("Troubleshooting"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  - Check logs: ~/.config/memorizer/memorizer.log"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  - Try manual start: memorizer daemon start"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  - Check daemon status: memorizer daemon status"))

	return b.String()
}

func (s *ServiceInstallStep) renderManualStart() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Secondary)
	cmdStyle := lipgloss.NewStyle().Foreground(styles.Highlight)

	b.WriteString(labelStyle.Render("To start the daemon manually:"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(cmdStyle.Render("memorizer daemon start"))

	return b.String()
}

// Validate always passes for the service install step.
func (s *ServiceInstallStep) Validate() error {
	return nil
}

// Apply ensures configuration is written to disk.
// Config may already have been written during installation, but writing again is harmless.
func (s *ServiceInstallStep) Apply(cfg *config.Config) error {
	// Always ensure config is saved. This handles:
	// - User skipped installation (stateSkipped)
	// - Installation failed before config write (stateFailed with early error)
	// - Installation succeeded (stateDone) - harmless re-write
	if err := s.writeConfig(); err != nil {
		return fmt.Errorf("failed to save configuration; %w", err)
	}
	return nil
}

// Message types for async operations.
type installProgressMsg struct {
	state   installState
	message string
}

type installDoneMsg struct {
	message string
}

type installErrorMsg struct {
	err     error
	message string
}

// startInstallation begins the installation process.
func (s *ServiceInstallStep) startInstallation() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Write config to disk first (daemon needs it)
		if err := s.writeConfig(); err != nil {
			return installErrorMsg{
				err:     err,
				message: "Failed to save configuration",
			}
		}

		// Get daemon manager
		manager, err := s.managerFactory()
		if err != nil {
			return installErrorMsg{
				err:     err,
				message: "Failed to initialize service manager",
			}
		}

		// Check if already installed and stop if running
		installed, _ := manager.IsInstalled()
		if installed {
			status, _ := manager.Status(ctx)
			if status.IsRunning {
				_ = manager.StopDaemon(ctx)
				time.Sleep(500 * time.Millisecond)
			}
		}

		// Install service
		if err := manager.Install(ctx); err != nil {
			return installErrorMsg{
				err:     err,
				message: "Failed to install service",
			}
		}

		// Start daemon
		if err := manager.StartDaemon(ctx); err != nil {
			return installErrorMsg{
				err:     err,
				message: "Failed to start daemon",
			}
		}

		// Wait for health check
		if err := s.waitForHealth(ctx); err != nil {
			return installErrorMsg{
				err:     err,
				message: "Daemon started but health check failed",
			}
		}

		return installDoneMsg{
			message: "Daemon installed and running",
		}
	}
}

// waitForHealth polls the health endpoint until healthy or timeout.
func (s *ServiceInstallStep) waitForHealth(ctx context.Context) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("config not initialized")
	}

	url := fmt.Sprintf("http://%s:%d/readyz", cfg.Daemon.HTTPBind, cfg.Daemon.HTTPPort)

	deadline := time.Now().Add(healthCheckTimeout)
	client := &http.Client{Timeout: 5 * time.Second}

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			time.Sleep(healthCheckInterval)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(healthCheckInterval)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		time.Sleep(healthCheckInterval)
	}

	return fmt.Errorf("health check timed out after %v", healthCheckTimeout)
}

// SetManagerFactory sets a custom manager factory (for testing).
func (s *ServiceInstallStep) SetManagerFactory(factory func() (servicemanager.DaemonManager, error)) {
	s.managerFactory = factory
}

// writeConfig writes the configuration to disk.
func (s *ServiceInstallStep) writeConfig() error {
	if s.cfg == nil {
		return fmt.Errorf("config not initialized")
	}

	configPath := config.GetConfigPath()
	return config.Write(s.cfg, configPath)
}
