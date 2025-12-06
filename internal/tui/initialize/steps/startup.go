package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/servicemanager"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// InstallChoice represents the user's choice for service installation
type InstallChoice int

const (
	InstallUser   InstallChoice = iota // Auto-install user-level
	InstallSystem                      // Show manual commands for system-level
	InstallSkip                        // Skip service setup
)

// StartChoice represents whether to start the daemon now or later
type StartChoice int

const (
	StartNow   StartChoice = iota // Start daemon immediately
	StartLater                    // Show start instructions for later
)

// StartupStep handles service manager configuration
type StartupStep struct {
	platform      servicemanager.Platform
	installRadio  *components.RadioGroup
	daemonRadio   *components.RadioGroup
	currentScreen int // 0 = install choice, 1 = daemon start choice
	installChoice InstallChoice
	startChoice   StartChoice
	installPath   string
	installError  error
	systemInstr   string // System-level installation instructions
}

// NewStartupStep creates a new startup configuration step
func NewStartupStep() *StartupStep {
	return &StartupStep{}
}

// Title returns the step title
func (s *StartupStep) Title() string {
	return "Automatic Startup"
}

// Init initializes the step
func (s *StartupStep) Init(cfg *config.Config) tea.Cmd {
	s.installError = nil
	s.currentScreen = 0
	s.platform = servicemanager.DetectPlatform()

	// Determine platform name for display
	var platformName string
	switch s.platform {
	case servicemanager.PlatformLinux:
		platformName = "systemd (Linux)"
	case servicemanager.PlatformDarwin:
		platformName = "launchd (macOS)"
	default:
		platformName = "unknown"
	}

	// Build install options
	installOptions := []components.RadioOption{
		{
			Label:       "Set up user-level service (recommended)",
			Description: fmt.Sprintf("Automatically configure %s for current user", platformName),
		},
		{
			Label:       "Set up system-level service (manual)",
			Description: "Display commands to configure as system service",
		},
		{
			Label:       "Skip automatic startup",
			Description: "Start daemon manually when needed",
		},
	}

	s.installRadio = components.NewRadioGroup(installOptions)
	s.installRadio.Focus()

	// Build daemon start options (will be used on screen 2)
	daemonOptions := []components.RadioOption{
		{
			Label:       "Yes, start the daemon",
			Description: "Start the daemon in the background immediately",
		},
		{
			Label:       "No, show me how to start it later",
			Description: "Display instructions for manual startup",
		},
	}

	s.daemonRadio = components.NewRadioGroup(daemonOptions)

	return nil
}

// Update handles input
func (s *StartupStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if s.currentScreen == 0 {
				// Screen 1: Install choice
				return s.handleInstallChoice()
			} else {
				// Screen 2: Daemon start choice
				s.startChoice = StartChoice(s.daemonRadio.Selected())
				return nil, StepNext
			}

		case "esc":
			if s.currentScreen == 1 {
				// Go back to screen 1
				s.currentScreen = 0
				s.installRadio.Focus()
				s.daemonRadio.Blur()
				return nil, StepContinue
			}
			return nil, StepPrev
		}
	}

	// Delegate to appropriate radio based on current screen
	if s.currentScreen == 0 {
		s.installRadio.Update(msg)
	} else {
		s.daemonRadio.Update(msg)
	}

	return nil, StepContinue
}

// View renders the step
func (s *StartupStep) View() string {
	var b strings.Builder

	if s.currentScreen == 0 {
		// Screen 1: Installation choice
		b.WriteString(styles.Subtitle.Render("Configure automatic daemon startup"))
		b.WriteString("\n\n")

		var platformName string
		switch s.platform {
		case servicemanager.PlatformLinux:
			platformName = "systemd"
		case servicemanager.PlatformDarwin:
			platformName = "launchd"
		default:
			platformName = "service manager"
		}

		b.WriteString(fmt.Sprintf("Choose how to manage the daemon with %s:\n\n", platformName))
		b.WriteString(s.installRadio.View())

		if s.installError != nil {
			b.WriteString("\n\n")
			b.WriteString(styles.ErrorText.Render("Error: " + s.installError.Error()))
		}

		b.WriteString("\n\n")
		b.WriteString(styles.HelpText.Render(NavigationHelp()))

	} else {
		// Screen 2: Daemon start choice (only after successful user-level install)
		b.WriteString(styles.Subtitle.Render("Daemon Startup"))
		b.WriteString("\n\n")

		if s.installChoice == InstallUser && s.installPath != "" {
			b.WriteString(styles.SuccessText.Render("✓ Service installed successfully"))
			b.WriteString("\n")
			b.WriteString(styles.MutedText.Render(fmt.Sprintf("  %s", s.installPath)))
			b.WriteString("\n\n")
		} else if s.systemInstr != "" {
			// Show truncated system instructions
			b.WriteString(styles.SuccessText.Render("System-level installation instructions:"))
			b.WriteString("\n\n")
			b.WriteString(styles.MutedText.Render("(Full instructions will be shown after wizard completion)"))
			b.WriteString("\n\n")
		}

		b.WriteString("Start the daemon now?\n\n")
		b.WriteString(s.daemonRadio.View())

		b.WriteString("\n\n")
		b.WriteString(styles.HelpText.Render(NavigationHelp()))
	}

	return b.String()
}

// Validate validates the step
func (s *StartupStep) Validate() error {
	// No validation needed - choice-based step
	return nil
}

// Apply applies the step values to config
func (s *StartupStep) Apply(cfg *config.Config) error {
	// Nothing to apply to config - installation happens in handleInstallChoice
	return nil
}

// Helper methods

// handleInstallChoice processes the user's installation choice
func (s *StartupStep) handleInstallChoice() (tea.Cmd, StepResult) {
	s.installChoice = InstallChoice(s.installRadio.Selected())

	switch s.installChoice {
	case InstallUser:
		// Perform user-level installation
		err := s.performUserInstall()
		if err != nil {
			s.installError = err
			return nil, StepContinue
		}

		// Move to screen 2 (daemon start choice)
		s.currentScreen = 1
		s.installRadio.Blur()
		s.daemonRadio.Focus()
		return nil, StepContinue

	case InstallSystem:
		// Generate system instructions
		err := s.generateSystemInstructions()
		if err != nil {
			s.installError = err
			return nil, StepContinue
		}

		// Show instructions in final screen, skip daemon start choice
		return nil, StepNext

	case InstallSkip:
		// Skip directly to next step
		return nil, StepNext
	}

	return nil, StepContinue
}

// performUserInstall installs the service file at user level
func (s *StartupStep) performUserInstall() error {
	// Get binary path
	binaryPath, err := servicemanager.GetBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to locate binary; %w", err)
	}

	// Get user and home
	user := os.Getenv("USER")
	if user == "" {
		return fmt.Errorf("USER environment variable not set")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory; %w", err)
	}

	// Load config for log file path (defensive)
	cfg, err := config.GetConfig()
	if err != nil || cfg == nil {
		cfg = &config.DefaultConfig
	}

	logFile := cfg.Daemon.LogFile
	if logFile == "" {
		logFile = filepath.Join(home, ".agentic-memorizer", "daemon.log")
	}

	// Install based on platform
	switch s.platform {
	case servicemanager.PlatformLinux:
		return s.installSystemdUser(binaryPath, user, home, logFile)
	case servicemanager.PlatformDarwin:
		return s.installLaunchdUser(binaryPath, user, home, logFile)
	default:
		return fmt.Errorf("unsupported platform: %s", s.platform)
	}
}

// installSystemdUser installs a systemd user-level unit
func (s *StartupStep) installSystemdUser(binaryPath, user, home, logFile string) error {
	cfg := servicemanager.SystemdConfig{
		BinaryPath: binaryPath,
		User:       user,
		Home:       home,
		LogFile:    logFile,
	}

	unitContent := servicemanager.GenerateUserUnit(cfg)
	err := servicemanager.InstallUserUnit(unitContent, home)
	if err != nil {
		return fmt.Errorf("failed to install systemd unit; %w", err)
	}

	// Store install path for display
	s.installPath, _ = servicemanager.GetUserUnitPath(home)
	return nil
}

// installLaunchdUser installs a launchd user-level agent
func (s *StartupStep) installLaunchdUser(binaryPath, user, home, logFile string) error {
	cfg := servicemanager.LaunchdConfig{
		BinaryPath: binaryPath,
		User:       user,
		Home:       home,
		LogFile:    logFile,
	}

	plistContent := servicemanager.GeneratePlist(cfg)
	plistPath, err := servicemanager.GetUserAgentPath(home, user)
	if err != nil {
		return fmt.Errorf("failed to get plist path; %w", err)
	}

	err = servicemanager.InstallUserAgent(plistContent, plistPath)
	if err != nil {
		return fmt.Errorf("failed to install launchd agent; %w", err)
	}

	// Store install path for display
	s.installPath = plistPath
	return nil
}

// generateSystemInstructions generates system-level installation instructions
func (s *StartupStep) generateSystemInstructions() error {
	// Get binary path
	binaryPath, err := servicemanager.GetBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to locate binary; %w", err)
	}

	// Get user and home
	user := os.Getenv("USER")
	if user == "" {
		return fmt.Errorf("USER environment variable not set")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory; %w", err)
	}

	// Load config
	cfg, err := config.GetConfig()
	if err != nil || cfg == nil {
		cfg = &config.DefaultConfig
	}

	logFile := cfg.Daemon.LogFile
	if logFile == "" {
		logFile = filepath.Join(home, ".agentic-memorizer", "daemon.log")
	}

	// Generate instructions based on platform
	switch s.platform {
	case servicemanager.PlatformLinux:
		systemdCfg := servicemanager.SystemdConfig{
			BinaryPath: binaryPath,
			User:       user,
			Home:       home,
			LogFile:    logFile,
		}
		systemUnit := servicemanager.GenerateSystemUnit(systemdCfg)
		s.systemInstr = servicemanager.GetSystemdSystemInstructions(systemUnit)

	case servicemanager.PlatformDarwin:
		launchdCfg := servicemanager.LaunchdConfig{
			BinaryPath: binaryPath,
			User:       user,
			Home:       home,
			LogFile:    logFile,
		}
		plistContent := servicemanager.GeneratePlist(launchdCfg)
		systemPath := servicemanager.GetSystemDaemonPath(user)
		s.systemInstr = servicemanager.GetLaunchdSystemInstructions(plistContent, systemPath)

	default:
		return fmt.Errorf("unsupported platform: %s", s.platform)
	}

	return nil
}

// GetInstallChoice returns the user's installation choice
func (s *StartupStep) GetInstallChoice() InstallChoice {
	return s.installChoice
}

// GetStartChoice returns the user's daemon start choice
func (s *StartupStep) GetStartChoice() StartChoice {
	return s.startChoice
}

// GetSystemInstructions returns the system-level installation instructions
func (s *StartupStep) GetSystemInstructions() string {
	return s.systemInstr
}

// GetInstallPath returns the path where the service was installed
func (s *StartupStep) GetInstallPath() string {
	return s.installPath
}
