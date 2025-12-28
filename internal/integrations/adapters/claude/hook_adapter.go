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
	IntegrationVersion = "3.0.0"

	// SessionStartEvent is the hook event name for file index injection
	SessionStartEvent = "SessionStart"

	// UserPromptSubmitEvent is the hook event name for facts injection
	UserPromptSubmitEvent = "UserPromptSubmit"
)

// Default matchers for SessionStart hooks
var defaultMatchers = []string{"startup", "resume", "clear", "compact"}

// ClaudeCodeHookAdapter implements the Integration interface for Claude Code
type ClaudeCodeHookAdapter struct {
	settingsPath string
	matchers     []string
	outputFormat integrations.OutputFormat
}

// NewClaudeCodeHookAdapter creates a new Claude Code hook integration adapter
func NewClaudeCodeHookAdapter() *ClaudeCodeHookAdapter {
	return &ClaudeCodeHookAdapter{
		settingsPath: getDefaultSettingsPath(),
		matchers:     defaultMatchers,
		outputFormat: integrations.FormatXML, // Default to XML
	}
}

// GetName returns the integration name
func (a *ClaudeCodeHookAdapter) GetName() string {
	return IntegrationName
}

// GetDescription returns a human-readable description
func (a *ClaudeCodeHookAdapter) GetDescription() string {
	return "Claude Code hooks integration (SessionStart for files, UserPromptSubmit for facts)"
}

// GetVersion returns the adapter version
func (a *ClaudeCodeHookAdapter) GetVersion() string {
	return IntegrationVersion
}

// Detect checks if Claude Code is installed on the system
func (a *ClaudeCodeHookAdapter) Detect() (bool, error) {
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
// Returns true only if BOTH SessionStart AND UserPromptSubmit hooks are installed
func (a *ClaudeCodeHookAdapter) IsEnabled() (bool, error) {
	// Check if settings file exists
	if _, err := os.Stat(a.settingsPath); os.IsNotExist(err) {
		return false, nil
	}

	// Read settings and check for memorizer hooks
	settings, _, err := readSettings(a.settingsPath)
	if err != nil {
		return false, fmt.Errorf("failed to read settings: %w", err)
	}

	// Check both hook types
	hasSessionStart := a.hasMemorizerHook(settings, SessionStartEvent)
	hasUserPromptSubmit := a.hasMemorizerHook(settings, UserPromptSubmitEvent)

	// Both hooks must be present for integration to be considered enabled
	return hasSessionStart && hasUserPromptSubmit, nil
}

// hasMemorizerHook checks if a specific event type has a memorizer hook installed
func (a *ClaudeCodeHookAdapter) hasMemorizerHook(settings *Settings, eventType string) bool {
	events, ok := settings.Hooks[eventType]
	if !ok || len(events) == 0 {
		return false
	}

	for _, event := range events {
		for _, hook := range event.Hooks {
			// Reject old binary name
			if strings.Contains(hook.Command, "agentic-memorizer") {
				return false
			}
			if strings.Contains(hook.Command, "memorizer") {
				return true
			}
		}
	}

	return false
}

// Setup configures the Claude Code integration
// Installs both SessionStart (for files) and UserPromptSubmit (for facts) hooks
// Uses transactional semantics: if UserPromptSubmit fails, SessionStart is rolled back
func (a *ClaudeCodeHookAdapter) Setup(binaryPath string) error {
	settings, fullSettings, err := readSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}

	// Save original state for rollback
	originalSessionStart := cloneHookEvents(settings.Hooks[SessionStartEvent])
	originalUserPromptSubmit := cloneHookEvents(settings.Hooks[UserPromptSubmitEvent])

	// Step 1: Install SessionStart hooks (for files)
	filesCommand := a.getFilesCommand(binaryPath, a.outputFormat)
	sessionStartEvents := settings.Hooks[SessionStartEvent]
	if sessionStartEvents == nil {
		sessionStartEvents = []HookEvent{}
	}

	for _, matcher := range a.matchers {
		sessionStartEvents = addOrUpdateHook(sessionStartEvents, matcher, filesCommand)
	}
	settings.Hooks[SessionStartEvent] = sessionStartEvents

	if err := writeSettings(a.settingsPath, settings, fullSettings); err != nil {
		return fmt.Errorf("failed to write SessionStart hooks: %w", err)
	}

	// Step 2: Install UserPromptSubmit hook (for facts)
	// Note: UserPromptSubmit doesn't use matchers - it fires on every prompt
	factsCommand := a.getFactsCommand(binaryPath, a.outputFormat)
	userPromptSubmitEvents := settings.Hooks[UserPromptSubmitEvent]
	if userPromptSubmitEvents == nil {
		userPromptSubmitEvents = []HookEvent{}
	}

	// Add or update single hook without matcher
	userPromptSubmitEvents = addOrUpdateHookNoMatcher(userPromptSubmitEvents, factsCommand)
	settings.Hooks[UserPromptSubmitEvent] = userPromptSubmitEvents

	if err := writeSettings(a.settingsPath, settings, fullSettings); err != nil {
		// Rollback SessionStart hooks
		settings.Hooks[SessionStartEvent] = originalSessionStart
		settings.Hooks[UserPromptSubmitEvent] = originalUserPromptSubmit
		_ = writeSettings(a.settingsPath, settings, fullSettings)
		return fmt.Errorf("failed to write UserPromptSubmit hooks (rolled back SessionStart): %w", err)
	}

	return nil
}

// cloneHookEvents creates a deep copy of hook events for rollback
func cloneHookEvents(events []HookEvent) []HookEvent {
	if events == nil {
		return nil
	}
	cloned := make([]HookEvent, len(events))
	for i, event := range events {
		cloned[i] = HookEvent{
			Matcher: event.Matcher,
			Hooks:   make([]Hook, len(event.Hooks)),
		}
		copy(cloned[i].Hooks, event.Hooks)
	}
	return cloned
}

// getFilesCommand returns the command for SessionStart hook (file index)
func (a *ClaudeCodeHookAdapter) getFilesCommand(binaryPath string, format integrations.OutputFormat) string {
	return fmt.Sprintf("%s read files --format %s --integration %s", binaryPath, format, IntegrationName)
}

// getFactsCommand returns the command for UserPromptSubmit hook (facts)
func (a *ClaudeCodeHookAdapter) getFactsCommand(binaryPath string, format integrations.OutputFormat) string {
	return fmt.Sprintf("%s read facts --format %s --integration %s", binaryPath, format, IntegrationName)
}

// Update updates the integration configuration
func (a *ClaudeCodeHookAdapter) Update(binaryPath string) error {
	// For Claude Code, update is the same as setup
	return a.Setup(binaryPath)
}

// Remove removes the integration configuration
// Removes both SessionStart and UserPromptSubmit hooks
// Continues removing remaining hooks even if one fails, returning aggregated error
func (a *ClaudeCodeHookAdapter) Remove() error {
	settings, fullSettings, err := readSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}

	var errors []string

	// Remove memorizer hooks from SessionStart
	if err := a.removeHooksFromEvent(settings, SessionStartEvent); err != nil {
		errors = append(errors, fmt.Sprintf("SessionStart: %v", err))
	}

	// Remove memorizer hooks from UserPromptSubmit
	if err := a.removeHooksFromEvent(settings, UserPromptSubmitEvent); err != nil {
		errors = append(errors, fmt.Sprintf("UserPromptSubmit: %v", err))
	}

	// Write settings even if some removals failed (to persist partial removal)
	if err := writeSettings(a.settingsPath, settings, fullSettings); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("partial removal errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// removeHooksFromEvent removes memorizer hooks from a specific event type
func (a *ClaudeCodeHookAdapter) removeHooksFromEvent(settings *Settings, eventType string) error {
	events, ok := settings.Hooks[eventType]
	if !ok {
		return nil // Nothing to remove
	}

	// Filter out memorizer and old agentic-memorizer hooks
	filtered := []HookEvent{}
	for _, event := range events {
		filteredHooks := []Hook{}
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
		delete(settings.Hooks, eventType)
	} else {
		settings.Hooks[eventType] = filtered
	}

	return nil
}

// GetCommand returns the command that should be executed by the hook
// Returns the SessionStart command for backwards compatibility
func (a *ClaudeCodeHookAdapter) GetCommand(binaryPath string, format integrations.OutputFormat) string {
	return a.getFilesCommand(binaryPath, format)
}

// FormatOutput formats the file index for Claude Code (SessionStart JSON wrapper)
func (a *ClaudeCodeHookAdapter) FormatOutput(index *types.FileIndex, format integrations.OutputFormat) (string, error) {
	return formatSessionStartJSON(index, format)
}

// FormatFactsOutput formats the facts index for Claude Code (UserPromptSubmit JSON wrapper)
func (a *ClaudeCodeHookAdapter) FormatFactsOutput(facts *types.FactsIndex, format integrations.OutputFormat) (string, error) {
	return formatUserPromptSubmitJSON(facts, format)
}

// Validate checks the health of the integration
// Reports per-hook status for SessionStart and UserPromptSubmit
func (a *ClaudeCodeHookAdapter) Validate() error {
	// Check if settings file exists
	if _, err := os.Stat(a.settingsPath); os.IsNotExist(err) {
		return fmt.Errorf("settings file not found at %s", a.settingsPath)
	}

	// Try to read settings
	settings, _, err := readSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}

	// Check for old binary name in any hook type
	if hasOldBinaryName(settings, SessionStartEvent) || hasOldBinaryName(settings, UserPromptSubmitEvent) {
		return fmt.Errorf("integration uses old binary name 'agentic-memorizer'; run 'memorizer integrations remove %s && memorizer integrations setup %s'",
			IntegrationName, IntegrationName)
	}

	// Check each hook type
	hasSessionStart := a.hasMemorizerHook(settings, SessionStartEvent)
	hasUserPromptSubmit := a.hasMemorizerHook(settings, UserPromptSubmitEvent)

	// Report detailed status
	if !hasSessionStart && !hasUserPromptSubmit {
		return fmt.Errorf("no memorizer hooks configured; SessionStart: missing, UserPromptSubmit: missing")
	}

	if !hasSessionStart {
		return fmt.Errorf("partially configured; SessionStart: missing, UserPromptSubmit: installed")
	}

	if !hasUserPromptSubmit {
		return fmt.Errorf("partially configured; SessionStart: installed, UserPromptSubmit: missing")
	}

	return nil
}

// hasOldBinaryName checks if any hooks in the event type use the old binary name
func hasOldBinaryName(settings *Settings, eventType string) bool {
	events, ok := settings.Hooks[eventType]
	if !ok {
		return false
	}

	for _, event := range events {
		for _, hook := range event.Hooks {
			if strings.Contains(hook.Command, "agentic-memorizer") {
				return true
			}
		}
	}

	return false
}

// Reload applies configuration changes
func (a *ClaudeCodeHookAdapter) Reload(newConfig integrations.IntegrationConfig) error {
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
		// Update existing matcher - look for both old and new binary names
		hookExists := false
		for i, hook := range events[matcherIdx].Hooks {
			if strings.Contains(hook.Command, "memorizer") {
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

// addOrUpdateHookNoMatcher adds or updates a hook without a matcher
// Used for events like UserPromptSubmit that don't use matchers
func addOrUpdateHookNoMatcher(events []HookEvent, command string) []HookEvent {
	newHook := Hook{
		Type:    "command",
		Command: command,
	}

	// Look for existing event with memorizer hook (no matcher or empty matcher)
	for i, event := range events {
		// Look for memorizer hook in this event
		for j, hook := range event.Hooks {
			if strings.Contains(hook.Command, "memorizer") {
				events[i].Hooks[j] = newHook
				return events
			}
		}
	}

	// No existing memorizer hook found - add new event without matcher
	events = append(events, HookEvent{
		// Matcher is empty - omitempty will exclude it from JSON
		Hooks: []Hook{newHook},
	})

	return events
}
