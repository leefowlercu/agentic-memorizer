package gemini

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/shared"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const (
	// HookIntegrationName is the unique identifier for this integration
	HookIntegrationName = "gemini-cli-hook"

	// HookIntegrationVersion is the adapter version
	HookIntegrationVersion = "2.0.0"

	// HookName is the name field for the hook
	HookName = "memorizer-hook"

	// HookDescription is the description field for the hook
	HookDescription = "Load agentic memory index"

	// FactsHookName is the name field for the facts hook
	FactsHookName = "memorizer-facts-hook"

	// FactsHookDescription is the description field for the facts hook
	FactsHookDescription = "Load user-defined facts"
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
	return "Gemini CLI hooks integration (SessionStart for files, BeforeAgent for facts)"
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
// Returns true only if BOTH SessionStart AND BeforeAgent hooks are installed
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

	// Check both hook types
	hasSessionStart := a.hasMemorizerHook(settings, SessionStartEvent)
	hasBeforeAgent := a.hasMemorizerHook(settings, BeforeAgentEvent)

	// Both hooks must be present for integration to be considered enabled
	return hasSessionStart && hasBeforeAgent, nil
}

// hasMemorizerHook checks if a specific event type has a memorizer hook installed
func (a *GeminiCLIHookAdapter) hasMemorizerHook(settings *GeminiSettings, eventType string) bool {
	events, ok := settings.Hooks[eventType]
	if !ok || len(events) == 0 {
		return false
	}

	for _, event := range events {
		for _, hook := range event.Hooks {
			// Reject old binary name
			if shared.ContainsOldBinaryName(hook.Command) {
				return false
			}
			if shared.ContainsMemorizer(hook.Command) {
				return true
			}
		}
	}

	return false
}

// Setup configures the Gemini CLI integration
// Installs both SessionStart (for files) and BeforeAgent (for facts) hooks
// Uses transactional semantics: if BeforeAgent fails, SessionStart is rolled back
func (a *GeminiCLIHookAdapter) Setup(binaryPath string) error {
	settings, fullSettings, err := readHookSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings; %w", err)
	}

	// Save original state for rollback
	originalSessionStart := cloneGeminiHookEvents(settings.Hooks[SessionStartEvent])
	originalBeforeAgent := cloneGeminiHookEvents(settings.Hooks[BeforeAgentEvent])

	// Step 1: Install SessionStart hooks (for files)
	filesCommand := shared.GetFilesCommand(binaryPath, a.outputFormat, HookIntegrationName)
	sessionStartEvents := settings.Hooks[SessionStartEvent]
	if sessionStartEvents == nil {
		sessionStartEvents = []GeminiHookEvent{}
	}

	for _, matcher := range a.matchers {
		sessionStartEvents = addOrUpdateGeminiHook(sessionStartEvents, matcher, filesCommand, HookName, HookDescription)
	}
	settings.Hooks[SessionStartEvent] = sessionStartEvents

	if err := writeHookSettings(a.settingsPath, settings, fullSettings); err != nil {
		return fmt.Errorf("failed to write SessionStart hooks; %w", err)
	}

	// Step 2: Install BeforeAgent hook (for facts)
	// Note: BeforeAgent doesn't use matchers - it fires before every agent invocation
	factsCommand := shared.GetFactsCommand(binaryPath, a.outputFormat, HookIntegrationName)
	beforeAgentEvents := settings.Hooks[BeforeAgentEvent]
	if beforeAgentEvents == nil {
		beforeAgentEvents = []GeminiHookEvent{}
	}

	// Add or update single hook without matcher
	beforeAgentEvents = addOrUpdateGeminiHookNoMatcher(beforeAgentEvents, factsCommand, FactsHookName, FactsHookDescription)
	settings.Hooks[BeforeAgentEvent] = beforeAgentEvents

	if err := writeHookSettings(a.settingsPath, settings, fullSettings); err != nil {
		// Rollback SessionStart hooks
		settings.Hooks[SessionStartEvent] = originalSessionStart
		settings.Hooks[BeforeAgentEvent] = originalBeforeAgent
		_ = writeHookSettings(a.settingsPath, settings, fullSettings)
		return fmt.Errorf("failed to write BeforeAgent hooks (rolled back SessionStart); %w", err)
	}

	return nil
}

// cloneGeminiHookEvents creates a deep copy of hook events for rollback
func cloneGeminiHookEvents(events []GeminiHookEvent) []GeminiHookEvent {
	if events == nil {
		return nil
	}
	cloned := make([]GeminiHookEvent, len(events))
	for i, event := range events {
		cloned[i] = GeminiHookEvent{
			Matcher: event.Matcher,
			Hooks:   make([]GeminiHook, len(event.Hooks)),
		}
		copy(cloned[i].Hooks, event.Hooks)
	}
	return cloned
}

// Update updates the integration configuration
func (a *GeminiCLIHookAdapter) Update(binaryPath string) error {
	// For Gemini CLI, update is the same as setup
	return a.Setup(binaryPath)
}

// Remove removes the integration configuration
// Removes both SessionStart and BeforeAgent hooks
// Continues removing remaining hooks even if one fails, returning aggregated error
func (a *GeminiCLIHookAdapter) Remove() error {
	settings, fullSettings, err := readHookSettings(a.settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings; %w", err)
	}

	var errors []string

	// Remove memorizer hooks from SessionStart
	if err := a.removeHooksFromEvent(settings, SessionStartEvent); err != nil {
		errors = append(errors, fmt.Sprintf("SessionStart: %v", err))
	}

	// Remove memorizer hooks from BeforeAgent
	if err := a.removeHooksFromEvent(settings, BeforeAgentEvent); err != nil {
		errors = append(errors, fmt.Sprintf("BeforeAgent: %v", err))
	}

	// Write settings even if some removals failed (to persist partial removal)
	if err := writeHookSettings(a.settingsPath, settings, fullSettings); err != nil {
		return fmt.Errorf("failed to write settings; %w", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("partial removal errors: %s", shared.AggregateErrors(errors))
	}

	return nil
}

// removeHooksFromEvent removes memorizer hooks from a specific event type
func (a *GeminiCLIHookAdapter) removeHooksFromEvent(settings *GeminiSettings, eventType string) error {
	events, ok := settings.Hooks[eventType]
	if !ok {
		return nil // Nothing to remove
	}

	// Filter out memorizer and old agentic-memorizer hooks
	filtered := []GeminiHookEvent{}
	for _, event := range events {
		filteredHooks := []GeminiHook{}
		for _, hook := range event.Hooks {
			if !shared.ContainsMemorizer(hook.Command) {
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
func (a *GeminiCLIHookAdapter) GetCommand(binaryPath string, format integrations.OutputFormat) string {
	return shared.GetFilesCommand(binaryPath, format, HookIntegrationName)
}

// FormatOutput formats the file index for Gemini CLI (SessionStart JSON wrapper)
func (a *GeminiCLIHookAdapter) FormatOutput(index *types.FileIndex, format integrations.OutputFormat) (string, error) {
	return formatGeminiHookJSON(index, format)
}

// FormatFactsOutput formats the facts index for Gemini CLI (BeforeAgent JSON wrapper)
func (a *GeminiCLIHookAdapter) FormatFactsOutput(facts *types.FactsIndex, format integrations.OutputFormat) (string, error) {
	return formatBeforeAgentJSON(facts, format)
}

// Validate checks the health of the integration
// Reports per-hook status for SessionStart and BeforeAgent
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

	// Check for old binary name in any hook type
	if hasOldBinaryName(settings, SessionStartEvent) || hasOldBinaryName(settings, BeforeAgentEvent) {
		return fmt.Errorf("integration uses old binary name 'agentic-memorizer'; run 'memorizer integrations remove %s && memorizer integrations setup %s'",
			HookIntegrationName, HookIntegrationName)
	}

	// Check each hook type
	hasSessionStart := a.hasMemorizerHook(settings, SessionStartEvent)
	hasBeforeAgent := a.hasMemorizerHook(settings, BeforeAgentEvent)

	// Report detailed status
	if !hasSessionStart && !hasBeforeAgent {
		return fmt.Errorf("no memorizer hooks configured; SessionStart: missing, BeforeAgent: missing")
	}

	if !hasSessionStart {
		return fmt.Errorf("partially configured; SessionStart: missing, BeforeAgent: installed")
	}

	if !hasBeforeAgent {
		return fmt.Errorf("partially configured; SessionStart: installed, BeforeAgent: missing")
	}

	return nil
}

// hasOldBinaryName checks if any hooks in the event type use the old binary name
func hasOldBinaryName(settings *GeminiSettings, eventType string) bool {
	events, ok := settings.Hooks[eventType]
	if !ok {
		return false
	}

	for _, event := range events {
		for _, hook := range event.Hooks {
			if shared.ContainsOldBinaryName(hook.Command) {
				return true
			}
		}
	}

	return false
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
func addOrUpdateGeminiHook(events []GeminiHookEvent, matcher, command, name, description string) []GeminiHookEvent {
	// Find existing matcher index
	matcherIdx := -1
	for i, event := range events {
		if event.Matcher == matcher {
			matcherIdx = i
			break
		}
	}

	newHook := GeminiHook{
		Name:        name,
		Type:        "command",
		Command:     command,
		Description: description,
	}

	if matcherIdx >= 0 {
		// Update existing matcher - look for hook with same name or memorizer command
		hookExists := false
		for i, hook := range events[matcherIdx].Hooks {
			if hook.Name == name || shared.ContainsMemorizer(hook.Command) {
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

// addOrUpdateGeminiHookNoMatcher adds or updates a hook without a matcher
// Used for events like BeforeAgent that don't use matchers
func addOrUpdateGeminiHookNoMatcher(events []GeminiHookEvent, command, name, description string) []GeminiHookEvent {
	newHook := GeminiHook{
		Name:        name,
		Type:        "command",
		Command:     command,
		Description: description,
	}

	// Look for existing event with memorizer hook (no matcher or empty matcher)
	for i, event := range events {
		// Look for memorizer hook in this event
		for j, hook := range event.Hooks {
			if hook.Name == name || shared.ContainsMemorizer(hook.Command) {
				events[i].Hooks[j] = newHook
				return events
			}
		}
	}

	// No existing memorizer hook found - add new event without matcher
	events = append(events, GeminiHookEvent{
		// Matcher is empty - omitempty will exclude it from JSON
		Hooks: []GeminiHook{newHook},
	})

	return events
}
