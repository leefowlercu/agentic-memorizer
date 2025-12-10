package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestNewGeminiCLIHookAdapter(t *testing.T) {
	adapter := NewGeminiCLIHookAdapter()

	if adapter == nil {
		t.Fatal("NewGeminiCLIHookAdapter() returned nil")
	}

	if adapter.GetName() != HookIntegrationName {
		t.Errorf("GetName() = %q, want %q", adapter.GetName(), HookIntegrationName)
	}

	if adapter.GetVersion() != HookIntegrationVersion {
		t.Errorf("GetVersion() = %q, want %q", adapter.GetVersion(), HookIntegrationVersion)
	}

	if adapter.outputFormat != integrations.FormatXML {
		t.Errorf("Default output format = %v, want %v", adapter.outputFormat, integrations.FormatXML)
	}
}

func TestGeminiCLIHookAdapter_GetCommand(t *testing.T) {
	adapter := NewGeminiCLIHookAdapter()

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
			want:       "/usr/local/bin/memorizer read --format xml --integration gemini-cli-hook",
		},
		{
			name:       "Markdown format",
			binaryPath: "/usr/local/bin/memorizer",
			format:     integrations.FormatMarkdown,
			want:       "/usr/local/bin/memorizer read --format markdown --integration gemini-cli-hook",
		},
		{
			name:       "JSON format",
			binaryPath: "/usr/local/bin/memorizer",
			format:     integrations.FormatJSON,
			want:       "/usr/local/bin/memorizer read --format json --integration gemini-cli-hook",
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

func TestGeminiCLIHookAdapter_SetupAndRemove(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.json")

	adapter := NewGeminiCLIHookAdapter()
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
	settings, _, err := readHookSettings(settingsPath)
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
			if hook.Name != HookName {
				t.Errorf("Hook name = %q, want %q", hook.Name, HookName)
			}
			if hook.Type != "command" {
				t.Errorf("Hook type = %q, want %q", hook.Type, "command")
			}
			if hook.Description != HookDescription {
				t.Errorf("Hook description = %q, want %q", hook.Description, HookDescription)
			}
			expectedCommand := adapter.GetCommand(binaryPath, integrations.FormatXML)
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
	settings, _, err = readHookSettings(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings after remove: %v", err)
	}

	sessionStartEvents, ok = settings.Hooks[SessionStartEvent]
	if ok && len(sessionStartEvents) > 0 {
		// Check if any memorizer hooks remain
		for _, event := range sessionStartEvents {
			for _, hook := range event.Hooks {
				if hook.Command == adapter.GetCommand(binaryPath, integrations.FormatXML) {
					t.Error("Memorizer hook still exists after Remove()")
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

func TestGeminiCLIHookAdapter_Detect(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	geminiDir := filepath.Join(tempDir, ".gemini")

	adapter := NewGeminiCLIHookAdapter()
	adapter.settingsPath = filepath.Join(geminiDir, "settings.json")

	// Test when directory doesn't exist
	detected, err := adapter.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if detected {
		t.Error("Detect() = true, want false when directory doesn't exist")
	}

	// Create directory
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
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

func TestGeminiCLIHookAdapter_FormatOutput(t *testing.T) {
	adapter := NewGeminiCLIHookAdapter()

	index := &types.GraphIndex{
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
	var result GeminiHookOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify structure
	if result.HookSpecificOutput == nil {
		t.Fatal("HookSpecificOutput should not be nil")
	}
	if result.HookSpecificOutput.HookEventName != SessionStartEvent {
		t.Errorf("HookEventName = %q, want %q", result.HookSpecificOutput.HookEventName, SessionStartEvent)
	}
}

func TestGeminiCLIHookAdapter_Reload(t *testing.T) {
	adapter := NewGeminiCLIHookAdapter()

	// Test reloading output format
	config := integrations.IntegrationConfig{
		OutputFormat: "markdown",
	}

	if err := adapter.Reload(config); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	if adapter.outputFormat != integrations.FormatMarkdown {
		t.Errorf("Output format after reload = %v, want %v", adapter.outputFormat, integrations.FormatMarkdown)
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

func TestGeminiCLIHookAdapter_ValidateErrors(t *testing.T) {
	adapter := NewGeminiCLIHookAdapter()
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

	emptySettings := GeminiSettings{
		Hooks: make(map[string][]GeminiHookEvent),
	}
	fullSettings := make(map[string]any)
	if err := writeHookSettings(settingsPath, &emptySettings, fullSettings); err != nil {
		t.Fatalf("Failed to write empty settings: %v", err)
	}

	// Test with no SessionStart hooks
	err = adapter.Validate()
	if err == nil {
		t.Error("Validate() should return error when no SessionStart hooks configured")
	}
}
