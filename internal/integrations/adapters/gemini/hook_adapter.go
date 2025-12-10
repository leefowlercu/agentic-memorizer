package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const (
	// HookIntegrationName is the unique identifier for this integration
	HookIntegrationName = "gemini-cli-hook"

	// HookIntegrationVersion is the adapter version
	HookIntegrationVersion = "1.0.0"

	// HookName is the name field for the hook
	HookName = "memorizer-hook"

	// HookDescription is the description field for the hook
	HookDescription = "Load agentic memory index"
)

// Default matchers for SessionStart hooks
var defaultHookMatchers = []string{"startup", "resume", "clear"}

// GeminiCLIHookAdapter implements the Integration interface for Gemini CLI SessionStart hooks
type GeminiCLIHookAdapter struct {
	settingsPath string
	matchers     []string
	outputFormat integrations.OutputFormat
}

// NewGeminiCLIHookAdapter creates a new Gemini CLI hook integration adapter
func NewGeminiCLIHookAdapter() *GeminiCLIHookAdapter {
	return &GeminiCLIHookAdapter{
		settingsPath: getDefaultHookSettingsPath(),
		matchers:     defaultHookMatchers,
		outputFormat: integrations.FormatXML, // Default to XML
	}
}

// GetName returns the integration name
func (a *GeminiCLIHookAdapter) GetName() string {
	return HookIntegrationName
}

// GetDescription returns a human-readable description
func (a *GeminiCLIHookAdapter) GetDescription() string {
	return "Gemini CLI SessionStart hook integration"
}

// GetVersion returns the adapter version
func (a *GeminiCLIHookAdapter) GetVersion() string {
	return HookIntegrationVersion
}

// Detect checks if Gemini CLI is installed on the system
func (a *GeminiCLIHookAdapter) Detect() (bool, error) {
	// Check if ~/.gemini directory exists
	geminiDir := filepath.Dir(a.settingsPath)
	info, err := os.Stat(geminiDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check Gemini directory; %w", err)
	}

	return info.IsDir(), nil
}

// IsEnabled checks if the integration is currently configured
func (a *GeminiCLIHookAdapter) IsEnabled() (bool, error) {
	// Check if settings file exists
	if _, err := os.Stat(a.settingsPath); os.IsNotExist(err) {
		return false, nil
	}

	// Read settings and check for memorizer hooks
	settings, _, err := readHookSettings(a.settingsPath)
	if err != nil {
		return false, fmt.Errorf("failed to read settings; %w", err)
	}

	// Check if SessionStart hooks exist with memorizer command
	sessionStartEvents, ok := settings.Hooks[SessionStartEvent]
	if !ok || len(sessionStartEvents) == 0 {
		return false, nil
	}

	// Look for memorizer hook (not old agentic-memorizer)
	for _, event := range sessionStartEvents {
		for _, hook := range event.Hooks {
			if strings.Contains(hook.Command, "agentic-memorizer") {
				// Found old binary name - report as not enabled
				return false, nil
			}
			if strings.Contains(hook.Command, "memorizer") {
				return true, nil
			}
		}
	}

	return false, nil
}

// Setup configures the Gemini CLI integration
func (a *GeminiCLIHookAdapter) Setup(binaryPath string) error {
	settings, fullSettings, err := readHookSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings; %w", err)
	}

	command := a.GetCommand(binaryPath, a.outputFormat)

	// Setup hooks for all matchers
	sessionStartEvents := settings.Hooks[SessionStartEvent]
	if sessionStartEvents == nil {
		sessionStartEvents = []GeminiHookEvent{}
	}

	for _, matcher := range a.matchers {
		sessionStartEvents = addOrUpdateGeminiHook(sessionStartEvents, matcher, command)
	}

	settings.Hooks[SessionStartEvent] = sessionStartEvents

	if err := writeHookSettings(a.settingsPath, settings, fullSettings); err != nil {
		return fmt.Errorf("failed to write settings; %w", err)
	}

	return nil
}

// Update updates the integration configuration
func (a *GeminiCLIHookAdapter) Update(binaryPath string) error {
	// For Gemini CLI, update is the same as setup
	return a.Setup(binaryPath)
}

// Remove removes the integration configuration
func (a *GeminiCLIHookAdapter) Remove() error {
	settings, fullSettings, err := readHookSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings; %w", err)
	}

	// Remove memorizer hooks from SessionStart
	sessionStartEvents, ok := settings.Hooks[SessionStartEvent]
	if !ok {
		return nil // Nothing to remove
	}

	// Filter out memorizer and old agentic-memorizer hooks
	filtered := []GeminiHookEvent{}
	for _, event := range sessionStartEvents {
		filteredHooks := []GeminiHook{}
		for _, hook := range event.Hooks {
			if !strings.Contains(hook.Command, "agentic-memorizer") && !strings.Contains(hook.Command, "memorizer") {
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

	if err := writeHookSettings(a.settingsPath, settings, fullSettings); err != nil {
		return fmt.Errorf("failed to write settings; %w", err)
	}

	return nil
}

// GetCommand returns the command that should be executed by the hook
func (a *GeminiCLIHookAdapter) GetCommand(binaryPath string, format integrations.OutputFormat) string {
	return fmt.Sprintf("%s read --format %s --integration %s", binaryPath, format, HookIntegrationName)
}

// FormatOutput formats the graph index for Gemini CLI (SessionStart JSON wrapper)
func (a *GeminiCLIHookAdapter) FormatOutput(index *types.GraphIndex, format integrations.OutputFormat) (string, error) {
	return formatGeminiHookJSON(index, format)
}

// Validate checks the health of the integration
func (a *GeminiCLIHookAdapter) Validate() error {
	// Check if settings file exists
	if _, err := os.Stat(a.settingsPath); os.IsNotExist(err) {
		return fmt.Errorf("settings file not found at %s", a.settingsPath)
	}

	// Try to read settings
	settings, _, err := readHookSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings; %w", err)
	}

	// Check if hooks are configured
	sessionStartEvents, ok := settings.Hooks[SessionStartEvent]
	if !ok || len(sessionStartEvents) == 0 {
		return fmt.Errorf("no SessionStart hooks configured")
	}

	// Check for old binary name and reject
	for _, event := range sessionStartEvents {
		for _, hook := range event.Hooks {
			if strings.Contains(hook.Command, "agentic-memorizer") {
				return fmt.Errorf("integration uses old binary name 'agentic-memorizer'; run 'memorizer integrations remove %s && memorizer integrations setup %s'",
					HookIntegrationName, HookIntegrationName)
			}
		}
	}

	// Verify memorizer hooks exist
	foundHooks := 0
	for _, event := range sessionStartEvents {
		for _, hook := range event.Hooks {
			if strings.Contains(hook.Command, "memorizer") {
				foundHooks++
			}
		}
	}

	if foundHooks == 0 {
		return fmt.Errorf("no memorizer hooks found in SessionStart events")
	}

	return nil
}

// Reload applies configuration changes
func (a *GeminiCLIHookAdapter) Reload(newConfig integrations.IntegrationConfig) error {
	// Update output format if changed
	if newConfig.OutputFormat != "" {
		format, err := integrations.ParseOutputFormat(newConfig.OutputFormat)
		if err != nil {
			return fmt.Errorf("invalid output format; %w", err)
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
			return fmt.Errorf("invalid settings path; %w", err)
		}
		a.settingsPath = expanded
	}

	return nil
}

// getDefaultHookSettingsPath returns the default path to Gemini CLI settings
func getDefaultHookSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gemini", "settings.json")
}

// addOrUpdateGeminiHook adds or updates a hook for a specific matcher
func addOrUpdateGeminiHook(events []GeminiHookEvent, matcher, command string) []GeminiHookEvent {
	// Find existing matcher index
	matcherIdx := -1
	for i, event := range events {
		if event.Matcher == matcher {
			matcherIdx = i
			break
		}
	}

	newHook := GeminiHook{
		Name:        HookName,
		Type:        "command",
		Command:     command,
		Description: HookDescription,
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
		events = append(events, GeminiHookEvent{
			Matcher: matcher,
			Hooks:   []GeminiHook{newHook},
		})
	}

	return events
}
