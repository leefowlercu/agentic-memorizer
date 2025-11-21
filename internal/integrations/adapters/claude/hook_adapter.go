package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const (
	// IntegrationName is the unique identifier for this integration
	IntegrationName = "claude-code-hook"

	// IntegrationVersion is the adapter version
	IntegrationVersion = "1.0.1"

	// SessionStartEvent is the hook event name for Claude Code
	SessionStartEvent = "SessionStart"
)

// Default matchers for SessionStart hooks
var defaultMatchers = []string{"startup", "resume", "clear", "compact"}

// ClaudeCodeAdapter implements the Integration interface for Claude Code
type ClaudeCodeAdapter struct {
	settingsPath string
	matchers     []string
	outputFormat integrations.OutputFormat
}

// NewClaudeCodeAdapter creates a new Claude Code integration adapter
func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{
		settingsPath: getDefaultSettingsPath(),
		matchers:     defaultMatchers,
		outputFormat: integrations.FormatXML, // Default to XML
	}
}

// GetName returns the integration name
func (a *ClaudeCodeAdapter) GetName() string {
	return IntegrationName
}

// GetDescription returns a human-readable description
func (a *ClaudeCodeAdapter) GetDescription() string {
	return "Claude Code SessionStart hook integration"
}

// GetVersion returns the adapter version
func (a *ClaudeCodeAdapter) GetVersion() string {
	return IntegrationVersion
}

// Detect checks if Claude Code is installed on the system
func (a *ClaudeCodeAdapter) Detect() (bool, error) {
	// Check if ~/.claude directory exists
	claudeDir := filepath.Dir(a.settingsPath)
	info, err := os.Stat(claudeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check Claude directory: %w", err)
	}

	return info.IsDir(), nil
}

// IsEnabled checks if the integration is currently configured
func (a *ClaudeCodeAdapter) IsEnabled() (bool, error) {
	// Check if settings file exists
	if _, err := os.Stat(a.settingsPath); os.IsNotExist(err) {
		return false, nil
	}

	// Read settings and check for agentic-memorizer hooks
	settings, _, err := readSettings(a.settingsPath)
	if err != nil {
		return false, fmt.Errorf("failed to read settings: %w", err)
	}

	// Check if SessionStart hooks exist with agentic-memorizer command
	sessionStartEvents, ok := settings.Hooks[SessionStartEvent]
	if !ok || len(sessionStartEvents) == 0 {
		return false, nil
	}

	// Look for at least one agentic-memorizer hook
	for _, event := range sessionStartEvents {
		for _, hook := range event.Hooks {
			if strings.Contains(hook.Command, "agentic-memorizer") {
				return true, nil
			}
		}
	}

	return false, nil
}

// Setup configures the Claude Code integration
func (a *ClaudeCodeAdapter) Setup(binaryPath string) error {
	settings, fullSettings, err := readSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}

	command := a.GetCommand(binaryPath, a.outputFormat)

	// Setup hooks for all matchers
	sessionStartEvents := settings.Hooks[SessionStartEvent]
	if sessionStartEvents == nil {
		sessionStartEvents = []HookEvent{}
	}

	for _, matcher := range a.matchers {
		sessionStartEvents = addOrUpdateHook(sessionStartEvents, matcher, command)
	}

	settings.Hooks[SessionStartEvent] = sessionStartEvents

	if err := writeSettings(a.settingsPath, settings, fullSettings); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	return nil
}

// Update updates the integration configuration
func (a *ClaudeCodeAdapter) Update(binaryPath string) error {
	// For Claude Code, update is the same as setup
	return a.Setup(binaryPath)
}

// Remove removes the integration configuration
func (a *ClaudeCodeAdapter) Remove() error {
	settings, fullSettings, err := readSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}

	// Remove agentic-memorizer hooks from SessionStart
	sessionStartEvents, ok := settings.Hooks[SessionStartEvent]
	if !ok {
		return nil // Nothing to remove
	}

	// Filter out agentic-memorizer hooks
	filtered := []HookEvent{}
	for _, event := range sessionStartEvents {
		filteredHooks := []Hook{}
		for _, hook := range event.Hooks {
			if !strings.Contains(hook.Command, "agentic-memorizer") {
				filteredHooks = append(filteredHooks, hook)
			}
		}
		if len(filteredHooks) > 0 {
			event.Hooks = filteredHooks
			filtered = append(filtered, event)
		}
	}

	if len(filtered) == 0 {
		delete(settings.Hooks, SessionStartEvent)
	} else {
		settings.Hooks[SessionStartEvent] = filtered
	}

	if err := writeSettings(a.settingsPath, settings, fullSettings); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	return nil
}

// GetCommand returns the command that should be executed by the hook
func (a *ClaudeCodeAdapter) GetCommand(binaryPath string, format integrations.OutputFormat) string {
	return fmt.Sprintf("%s read --format %s --integration %s", binaryPath, format, IntegrationName)
}

// FormatOutput formats the index for Claude Code (SessionStart JSON wrapper)
func (a *ClaudeCodeAdapter) FormatOutput(index *types.Index, format integrations.OutputFormat) (string, error) {
	return formatSessionStartJSON(index, format)
}

// Validate checks the health of the integration
func (a *ClaudeCodeAdapter) Validate() error {
	// Check if settings file exists
	if _, err := os.Stat(a.settingsPath); os.IsNotExist(err) {
		return fmt.Errorf("settings file not found at %s", a.settingsPath)
	}

	// Try to read settings
	settings, _, err := readSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}

	// Check if hooks are configured
	sessionStartEvents, ok := settings.Hooks[SessionStartEvent]
	if !ok || len(sessionStartEvents) == 0 {
		return fmt.Errorf("no SessionStart hooks configured")
	}

	// Verify agentic-memorizer hooks exist
	foundHooks := 0
	for _, event := range sessionStartEvents {
		for _, hook := range event.Hooks {
			if strings.Contains(hook.Command, "agentic-memorizer") {
				foundHooks++
			}
		}
	}

	if foundHooks == 0 {
		return fmt.Errorf("no agentic-memorizer hooks found in SessionStart events")
	}

	return nil
}

// Reload applies configuration changes
func (a *ClaudeCodeAdapter) Reload(newConfig integrations.IntegrationConfig) error {
	// Update output format if changed
	if newConfig.OutputFormat != "" {
		format, err := integrations.ParseOutputFormat(newConfig.OutputFormat)
		if err != nil {
			return fmt.Errorf("invalid output format: %w", err)
		}
		a.outputFormat = format
	}

	// Update matchers if provided
	if matchersRaw, ok := newConfig.Settings["matchers"]; ok {
		if matchers, ok := matchersRaw.([]string); ok {
			a.matchers = matchers
		}
	}

	// Update settings path if provided
	if settingsPath, ok := newConfig.Settings["settings_path"].(string); ok && settingsPath != "" {
		expanded, err := integrations.ExpandPath(settingsPath)
		if err != nil {
			return fmt.Errorf("invalid settings path: %w", err)
		}
		a.settingsPath = expanded
	}

	return nil
}

// getDefaultSettingsPath returns the default path to Claude Code settings
func getDefaultSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// addOrUpdateHook adds or updates a hook for a specific matcher
func addOrUpdateHook(events []HookEvent, matcher, command string) []HookEvent {
	// Find existing matcher index
	matcherIdx := -1
	for i, event := range events {
		if event.Matcher == matcher {
			matcherIdx = i
			break
		}
	}

	newHook := Hook{
		Type:    "command",
		Command: command,
	}

	if matcherIdx >= 0 {
		// Update existing matcher
		hookExists := false
		for i, hook := range events[matcherIdx].Hooks {
			if strings.Contains(hook.Command, "agentic-memorizer") {
				events[matcherIdx].Hooks[i] = newHook
				hookExists = true
				break
			}
		}
		if !hookExists {
			events[matcherIdx].Hooks = append(events[matcherIdx].Hooks, newHook)
		}
	} else {
		// Add new matcher
		events = append(events, HookEvent{
			Matcher: matcher,
			Hooks:   []Hook{newHook},
		})
	}

	return events
}
