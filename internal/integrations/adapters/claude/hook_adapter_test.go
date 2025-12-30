package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/shared"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestNewClaudeCodeHookAdapter(t *testing.T) {
	adapter := NewClaudeCodeHookAdapter()

	if adapter == nil {
		t.Fatal("NewClaudeCodeHookAdapter() returned nil")
	}

	if adapter.GetName() != IntegrationName {
		t.Errorf("GetName() = %q, want %q", adapter.GetName(), IntegrationName)
	}

	if adapter.GetVersion() != IntegrationVersion {
		t.Errorf("GetVersion() = %q, want %q", adapter.GetVersion(), IntegrationVersion)
	}

	if adapter.outputFormat != integrations.FormatXML {
		t.Errorf("Default output format = %v, want %v", adapter.outputFormat, integrations.FormatXML)
	}

	// Check default matchers
	expectedMatchers := []string{"startup", "resume", "clear", "compact"}
	if len(adapter.matchers) != len(expectedMatchers) {
		t.Errorf("Default matchers count = %d, want %d", len(adapter.matchers), len(expectedMatchers))
	}
}

func TestClaudeCodeHookAdapter_GetDescription(t *testing.T) {
	adapter := NewClaudeCodeHookAdapter()

	desc := adapter.GetDescription()
	if desc == "" {
		t.Error("GetDescription() returned empty string")
	}
}

func TestClaudeCodeHookAdapter_GetCommand(t *testing.T) {
	adapter := NewClaudeCodeHookAdapter()

	tests := []struct {
		name       string
		binaryPath string
		format     integrations.OutputFormat
		want       string
	}{
		{
			name:       "XML format",
			binaryPath: "/usr/local/bin/memorizer",
			format:     integrations.FormatXML,
			want:       "/usr/local/bin/memorizer read files --format xml --integration claude-code-hook",
		},
		{
			name:       "JSON format",
			binaryPath: "/usr/local/bin/memorizer",
			format:     integrations.FormatJSON,
			want:       "/usr/local/bin/memorizer read files --format json --integration claude-code-hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.GetCommand(tt.binaryPath, tt.format)
			if got != tt.want {
				t.Errorf("GetCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClaudeCodeHookAdapter_SetupAndRemove(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.json")

	adapter := NewClaudeCodeHookAdapter()
	adapter.settingsPath = settingsPath

	binaryPath := "/usr/local/bin/memorizer"

	// Test Setup
	if err := adapter.Setup(binaryPath); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	// Verify settings file was created
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Fatal("Settings file was not created")
	}

	// Read and verify settings content
	settings, _, err := readSettings(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings: %v", err)
	}

	// Verify SessionStart hooks exist
	sessionStartEvents, ok := settings.Hooks[SessionStartEvent]
	if !ok {
		t.Fatal("SessionStart hooks not found")
	}

	// Verify all matchers are configured
	expectedMatchers := map[string]bool{
		"startup": false,
		"resume":  false,
		"clear":   false,
		"compact": false,
	}

	for _, event := range sessionStartEvents {
		if _, ok := expectedMatchers[event.Matcher]; ok {
			expectedMatchers[event.Matcher] = true

			// Verify hook configuration
			if len(event.Hooks) == 0 {
				t.Errorf("No hooks found for matcher %q", event.Matcher)
				continue
			}

			hook := event.Hooks[0]
			if hook.Type != "command" {
				t.Errorf("Hook type = %q, want %q", hook.Type, "command")
			}
			expectedCommand := shared.GetFilesCommand(binaryPath, integrations.FormatXML, IntegrationName)
			if hook.Command != expectedCommand {
				t.Errorf("Hook command = %q, want %q", hook.Command, expectedCommand)
			}
		}
	}

	for matcher, found := range expectedMatchers {
		if !found {
			t.Errorf("Matcher %q not found in settings", matcher)
		}
	}

	// Verify UserPromptSubmit hook exists
	userPromptSubmitEvents, ok := settings.Hooks[UserPromptSubmitEvent]
	if !ok {
		t.Fatal("UserPromptSubmit hooks not found")
	}

	if len(userPromptSubmitEvents) == 0 {
		t.Fatal("No UserPromptSubmit events found")
	}

	// Verify UserPromptSubmit hook configuration
	foundFactsHook := false
	for _, event := range userPromptSubmitEvents {
		for _, hook := range event.Hooks {
			if hook.Type == "command" {
				expectedFactsCommand := shared.GetFactsCommand(binaryPath, integrations.FormatXML, IntegrationName)
				if hook.Command == expectedFactsCommand {
					foundFactsHook = true
				}
			}
		}
	}
	if !foundFactsHook {
		t.Error("Facts hook not found in UserPromptSubmit events")
	}

	// Test IsEnabled (should be true after setup)
	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if !enabled {
		t.Error("IsEnabled() = false, want true after setup")
	}

	// Test Validate
	if err := adapter.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	// Test Remove
	if err := adapter.Remove(); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify hooks were removed
	settings, _, err = readSettings(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings after remove: %v", err)
	}

	// Check SessionStart hooks removed
	sessionStartEvents, ok = settings.Hooks[SessionStartEvent]
	if ok && len(sessionStartEvents) > 0 {
		for _, event := range sessionStartEvents {
			for _, hook := range event.Hooks {
				if hook.Command == shared.GetFilesCommand(binaryPath, integrations.FormatXML, IntegrationName) {
					t.Error("Files hook still exists in SessionStart after Remove()")
				}
			}
		}
	}

	// Check UserPromptSubmit hooks removed
	userPromptSubmitEvents, ok = settings.Hooks[UserPromptSubmitEvent]
	if ok && len(userPromptSubmitEvents) > 0 {
		for _, event := range userPromptSubmitEvents {
			for _, hook := range event.Hooks {
				if hook.Command == shared.GetFactsCommand(binaryPath, integrations.FormatXML, IntegrationName) {
					t.Error("Facts hook still exists in UserPromptSubmit after Remove()")
				}
			}
		}
	}

	// Test IsEnabled (should be false after remove)
	enabled, err = adapter.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if enabled {
		t.Error("IsEnabled() = true, want false after remove")
	}
}

func TestClaudeCodeHookAdapter_Detect(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")

	adapter := NewClaudeCodeHookAdapter()
	adapter.settingsPath = filepath.Join(claudeDir, "settings.json")

	// Test when directory doesn't exist
	detected, err := adapter.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if detected {
		t.Error("Detect() = true, want false when directory doesn't exist")
	}

	// Create directory
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Test when directory exists
	detected, err = adapter.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !detected {
		t.Error("Detect() = false, want true when directory exists")
	}
}

func TestClaudeCodeHookAdapter_FormatOutput(t *testing.T) {
	adapter := NewClaudeCodeHookAdapter()

	index := &types.FileIndex{
		MemoryRoot: "/test/path",
		Stats: types.IndexStats{
			TotalFiles:    1,
			TotalSize:     1024,
			CachedFiles:   1,
			AnalyzedFiles: 0,
		},
		Files: []types.FileEntry{
			{
				Path:     "/test/file.txt",
				Name:     "file.txt",
				Category: "documents",
				Size:     1024,
				Type:     "txt",
				Summary:  "Test file",
				Tags:     []string{"test"},
			},
		},
	}

	output, err := adapter.FormatOutput(index, integrations.FormatXML)
	if err != nil {
		t.Fatalf("FormatOutput() error = %v", err)
	}

	// Verify output is valid JSON
	var result SessionStartOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify structure
	if result.HookSpecific == nil {
		t.Fatal("HookSpecific should not be nil")
	}
	if result.HookSpecific.HookEventName != SessionStartEvent {
		t.Errorf("HookEventName = %q, want %q", result.HookSpecific.HookEventName, SessionStartEvent)
	}
	if !result.Continue {
		t.Error("Continue should be true")
	}
	if !result.SuppressOutput {
		t.Error("SuppressOutput should be true")
	}
}

func TestClaudeCodeHookAdapter_FormatFactsOutput(t *testing.T) {
	adapter := NewClaudeCodeHookAdapter()

	facts := &types.FactsIndex{
		Stats: types.FactStats{
			TotalFacts: 1,
			MaxFacts:   50,
		},
		Facts: []types.Fact{
			{
				ID:      "test-id",
				Content: "Test fact content",
			},
		},
	}

	output, err := adapter.FormatFactsOutput(facts, integrations.FormatXML)
	if err != nil {
		t.Fatalf("FormatFactsOutput() error = %v", err)
	}

	// Verify output is valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify the output has expected structure
	if _, ok := result["continue"]; !ok {
		t.Error("Output should have 'continue' field")
	}
}

func TestClaudeCodeHookAdapter_Reload(t *testing.T) {
	adapter := NewClaudeCodeHookAdapter()

	// Test reloading output format
	config := integrations.IntegrationConfig{
		OutputFormat: "json",
	}

	if err := adapter.Reload(config); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	if adapter.outputFormat != integrations.FormatJSON {
		t.Errorf("Output format after reload = %v, want %v", adapter.outputFormat, integrations.FormatJSON)
	}

	// Test reloading matchers
	config = integrations.IntegrationConfig{
		Settings: map[string]any{
			"matchers": []string{"startup", "resume"},
		},
	}

	if err := adapter.Reload(config); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	if len(adapter.matchers) != 2 {
		t.Errorf("Matchers count after reload = %d, want 2", len(adapter.matchers))
	}

	// Test invalid output format
	config = integrations.IntegrationConfig{
		OutputFormat: "invalid",
	}

	if err := adapter.Reload(config); err == nil {
		t.Error("Reload() with invalid format should return error")
	}
}

func TestClaudeCodeHookAdapter_ValidateErrors(t *testing.T) {
	adapter := NewClaudeCodeHookAdapter()
	adapter.settingsPath = "/nonexistent/path/settings.json"

	// Test when settings file doesn't exist
	err := adapter.Validate()
	if err == nil {
		t.Error("Validate() should return error when settings file doesn't exist")
	}

	// Create temp directory with settings but no hooks
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.json")
	adapter.settingsPath = settingsPath

	emptySettings := Settings{
		Hooks: make(map[string][]HookEvent),
	}
	fullSettings := make(map[string]any)
	if err := writeSettings(settingsPath, &emptySettings, fullSettings); err != nil {
		t.Fatalf("Failed to write empty settings: %v", err)
	}

	// Test with no hooks configured
	err = adapter.Validate()
	if err == nil {
		t.Error("Validate() should return error when no hooks configured")
	}
}

func TestClaudeCodeHookAdapter_PartialConfiguration(t *testing.T) {
	// Test when only SessionStart hook is installed
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.json")

	adapter := NewClaudeCodeHookAdapter()
	adapter.settingsPath = settingsPath

	// Write settings with only SessionStart hook
	settings := &Settings{
		Hooks: map[string][]HookEvent{
			SessionStartEvent: {
				{
					Matcher: "startup",
					Hooks: []Hook{
						{Type: "command", Command: "/usr/local/bin/memorizer read files --format xml --integration claude-code-hook"},
					},
				},
			},
		},
	}
	if err := writeSettings(settingsPath, settings, make(map[string]any)); err != nil {
		t.Fatalf("Failed to write settings: %v", err)
	}

	// IsEnabled should be false (requires both hooks)
	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if enabled {
		t.Error("IsEnabled() = true, want false when only SessionStart is configured")
	}

	// Validate should report partial configuration
	err = adapter.Validate()
	if err == nil {
		t.Error("Validate() should return error for partial configuration")
	}
}

func TestClaudeCodeHookAdapter_OldBinaryName(t *testing.T) {
	// Test detection of old binary name 'agentic-memorizer'
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.json")

	adapter := NewClaudeCodeHookAdapter()
	adapter.settingsPath = settingsPath

	// Write settings with old binary name
	settings := &Settings{
		Hooks: map[string][]HookEvent{
			SessionStartEvent: {
				{
					Matcher: "startup",
					Hooks: []Hook{
						{Type: "command", Command: "/usr/local/bin/agentic-memorizer read files"},
					},
				},
			},
		},
	}
	if err := writeSettings(settingsPath, settings, make(map[string]any)); err != nil {
		t.Fatalf("Failed to write settings: %v", err)
	}

	// IsEnabled should be false (old binary name rejected)
	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if enabled {
		t.Error("IsEnabled() = true, want false when old binary name is used")
	}

	// Validate should report old binary name error
	err = adapter.Validate()
	if err == nil {
		t.Error("Validate() should return error for old binary name")
	}
}

func TestClaudeCodeHookAdapter_Update(t *testing.T) {
	// Test that Update behaves same as Setup
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.json")

	adapter := NewClaudeCodeHookAdapter()
	adapter.settingsPath = settingsPath

	binaryPath := "/usr/local/bin/memorizer"

	// Update should work on fresh install
	if err := adapter.Update(binaryPath); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify hooks are installed
	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if !enabled {
		t.Error("IsEnabled() = false after Update()")
	}
}

func TestClaudeCodeHookAdapter_InterfaceCompliance(t *testing.T) {
	adapter := NewClaudeCodeHookAdapter()

	// Verify adapter implements the Integration interface
	var _ integrations.Integration = adapter
}
