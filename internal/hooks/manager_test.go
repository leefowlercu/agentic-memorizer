package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindBinaryPath(t *testing.T) {
	// This test just ensures the function doesn't panic
	// Actual path detection will vary by environment
	path, err := FindBinaryPath()

	// Either we find it or we don't, both are valid depending on environment
	if err != nil {
		t.Logf("Binary path not found (expected in some environments): %v", err)
	} else {
		t.Logf("Found binary path: %s", path)

		// Verify it's a valid path
		if !filepath.IsAbs(path) {
			t.Errorf("Expected absolute path, got: %s", path)
		}
	}
}

func TestReadWriteSettings(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Test reading non-existent file
	settings, err := ReadSettings(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read non-existent settings: %v", err)
	}

	if settings.Hooks == nil {
		t.Error("Expected hooks map to be initialized")
	}

	// Test writing settings
	settings.Hooks["TestEvent"] = []HookEvent{
		{
			Matcher: "test",
			Hooks: []Hook{
				{
					Type:    "command",
					Command: "echo test",
				},
			},
		},
	}

	err = WriteSettings(settingsPath, settings)
	if err != nil {
		t.Fatalf("Failed to write settings: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("Settings file was not created")
	}

	// Read back and verify
	readSettings, err := ReadSettings(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read written settings: %v", err)
	}

	if len(readSettings.Hooks["TestEvent"]) != 1 {
		t.Errorf("Expected 1 TestEvent hook, got %d", len(readSettings.Hooks["TestEvent"]))
	}
}

func TestSetupSessionStartHooks(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Override the settings path for this test
	originalGetPath := GetClaudeSettingsPath
	GetClaudeSettingsPath = func() (string, error) {
		return settingsPath, nil
	}
	defer func() {
		GetClaudeSettingsPath = originalGetPath
	}()

	binaryPath := "/test/path/agentic-memorizer"

	// Run setup
	settings, updated, err := SetupSessionStartHooks(binaryPath)
	if err != nil {
		t.Fatalf("Failed to setup hooks: %v", err)
	}

	// Verify all matchers were added
	expectedMatchers := []string{"startup", "resume", "clear", "compact"}
	if len(updated) != len(expectedMatchers) {
		t.Errorf("Expected %d matchers to be updated, got %d", len(expectedMatchers), len(updated))
	}

	// Verify SessionStart exists
	sessionStart, exists := settings.Hooks["SessionStart"]
	if !exists {
		t.Fatal("SessionStart hook not found")
	}

	// Verify all matchers are present
	for _, matcher := range expectedMatchers {
		found := false
		for _, event := range sessionStart {
			if event.Matcher == matcher {
				found = true
				// Verify our command is in the hooks
				commandFound := false
				expectedCommand := binaryPath + " --format json"
				for _, hook := range event.Hooks {
					if hook.Command == expectedCommand {
						commandFound = true
						break
					}
				}
				if !commandFound {
					t.Errorf("Expected command '%s' not found in matcher '%s'", expectedCommand, matcher)
				}
				break
			}
		}
		if !found {
			t.Errorf("Matcher '%s' not found in SessionStart hooks", matcher)
		}
	}

	// Run setup again to test idempotency
	_, updated2, err := SetupSessionStartHooks(binaryPath)
	if err != nil {
		t.Fatalf("Failed second setup: %v", err)
	}

	// Should not update anything since hooks already exist
	if len(updated2) != 0 {
		t.Errorf("Expected no updates on second run, got %d updates", len(updated2))
	}
}

func TestPreserveExistingHooks(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Create existing settings with a custom hook
	existingSettings := &Settings{
		Hooks: map[string][]HookEvent{
			"SessionStart": {
				{
					Matcher: "startup",
					Hooks: []Hook{
						{
							Type:    "command",
							Command: "echo 'existing hook'",
						},
					},
				},
			},
		},
	}

	err := WriteSettings(settingsPath, existingSettings)
	if err != nil {
		t.Fatalf("Failed to write existing settings: %v", err)
	}

	// Override the settings path for this test
	originalGetPath := GetClaudeSettingsPath
	GetClaudeSettingsPath = func() (string, error) {
		return settingsPath, nil
	}
	defer func() {
		GetClaudeSettingsPath = originalGetPath
	}()

	binaryPath := "/test/path/agentic-memorizer"

	// Run setup
	settings, _, err := SetupSessionStartHooks(binaryPath)
	if err != nil {
		t.Fatalf("Failed to setup hooks: %v", err)
	}

	// Verify existing hook is still there
	startupEvents := settings.Hooks["SessionStart"]
	var startupEvent *HookEvent
	for i := range startupEvents {
		if startupEvents[i].Matcher == "startup" {
			startupEvent = &startupEvents[i]
			break
		}
	}

	if startupEvent == nil {
		t.Fatal("startup matcher not found")
	}

	// Should have 2 hooks: the existing one and our new one
	if len(startupEvent.Hooks) != 2 {
		t.Errorf("Expected 2 hooks in startup matcher, got %d", len(startupEvent.Hooks))
	}

	// Verify existing hook is preserved
	existingFound := false
	for _, hook := range startupEvent.Hooks {
		if hook.Command == "echo 'existing hook'" {
			existingFound = true
			break
		}
	}

	if !existingFound {
		t.Error("Existing hook was not preserved")
	}
}
