package hooks

import (
	"encoding/json"
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
	settings, fullSettings, err := ReadSettings(settingsPath)
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

	err = WriteSettings(settingsPath, settings, fullSettings)
	if err != nil {
		t.Fatalf("Failed to write settings: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("Settings file was not created")
	}

	// Read back and verify
	readSettings, _, err := ReadSettings(settingsPath)
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
				expectedCommand := binaryPath + " read --format xml --wrap-json"
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

	err := WriteSettings(settingsPath, existingSettings, make(map[string]any))
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

func TestSetupSessionStartHooksPreservesSettings(t *testing.T) {
	binaryPath := "/test/path/agentic-memorizer"
	expectedCommand := binaryPath + " read --format xml --wrap-json"

	tests := []struct {
		name                string
		initialSettings     string
		expectUpdatedCount  int
		verifyFn            func(t *testing.T, settingsPath string)
	}{
		{
			name: "no hooks section",
			initialSettings: `{
  "awsCredentialExport": "doormat aws json",
  "includeCoAuthoredBy": false,
  "permissions": {
    "allow": ["WebSearch"],
    "deny": [],
    "ask": []
  },
  "alwaysThinkingEnabled": true
}`,
			expectUpdatedCount: 4,
			verifyFn: func(t *testing.T, settingsPath string) {
				// Read back settings
				data, err := os.ReadFile(settingsPath)
				if err != nil {
					t.Fatalf("Failed to read settings: %v", err)
				}

				var fullSettings map[string]any
				if err := json.Unmarshal(data, &fullSettings); err != nil {
					t.Fatalf("Failed to parse settings: %v", err)
				}

				// Verify all original fields preserved
				if fullSettings["awsCredentialExport"] != "doormat aws json" {
					t.Error("awsCredentialExport not preserved")
				}
				if fullSettings["includeCoAuthoredBy"] != false {
					t.Error("includeCoAuthoredBy not preserved")
				}
				if fullSettings["alwaysThinkingEnabled"] != true {
					t.Error("alwaysThinkingEnabled not preserved")
				}
				if fullSettings["permissions"] == nil {
					t.Error("permissions not preserved")
				}

				// Verify hooks were added
				hooks, ok := fullSettings["hooks"].(map[string]any)
				if !ok || hooks == nil {
					t.Fatal("hooks section not created")
				}
			},
		},
		{
			name: "empty hooks section",
			initialSettings: `{
  "awsCredentialExport": "doormat aws json",
  "permissions": {
    "allow": ["WebSearch"]
  },
  "hooks": {}
}`,
			expectUpdatedCount: 4,
			verifyFn: func(t *testing.T, settingsPath string) {
				data, err := os.ReadFile(settingsPath)
				if err != nil {
					t.Fatalf("Failed to read settings: %v", err)
				}

				var fullSettings map[string]any
				if err := json.Unmarshal(data, &fullSettings); err != nil {
					t.Fatalf("Failed to parse settings: %v", err)
				}

				// Verify original fields preserved
				if fullSettings["awsCredentialExport"] != "doormat aws json" {
					t.Error("awsCredentialExport not preserved")
				}

				// Verify SessionStart hooks added
				hooks := fullSettings["hooks"].(map[string]any)
				if hooks["SessionStart"] == nil {
					t.Error("SessionStart hooks not added")
				}
			},
		},
		{
			name: "pre-existing unrelated hooks",
			initialSettings: `{
  "awsCredentialExport": "doormat aws json",
  "permissions": {
    "allow": ["WebSearch"],
    "defaultMode": "plan"
  },
  "hooks": {
    "PromptSubmit": [
      {
        "matcher": "commit",
        "hooks": [
          {
            "type": "command",
            "command": "git status"
          }
        ]
      }
    ],
    "OtherEvent": [
      {
        "matcher": "test",
        "hooks": [
          {
            "type": "command",
            "command": "echo test"
          }
        ]
      }
    ]
  }
}`,
			expectUpdatedCount: 4,
			verifyFn: func(t *testing.T, settingsPath string) {
				data, err := os.ReadFile(settingsPath)
				if err != nil {
					t.Fatalf("Failed to read settings: %v", err)
				}

				var fullSettings map[string]any
				if err := json.Unmarshal(data, &fullSettings); err != nil {
					t.Fatalf("Failed to parse settings: %v", err)
				}

				// Verify original fields preserved
				if fullSettings["awsCredentialExport"] != "doormat aws json" {
					t.Error("awsCredentialExport not preserved")
				}

				// Verify other hooks preserved
				hooks := fullSettings["hooks"].(map[string]any)
				if hooks["PromptSubmit"] == nil {
					t.Error("PromptSubmit hooks not preserved")
				}
				if hooks["OtherEvent"] == nil {
					t.Error("OtherEvent hooks not preserved")
				}

				// Verify SessionStart hooks added
				if hooks["SessionStart"] == nil {
					t.Error("SessionStart hooks not added")
				}
			},
		},
		{
			name: "hooks already setup",
			initialSettings: `{
  "awsCredentialExport": "doormat aws json",
  "permissions": {
    "allow": ["WebSearch"]
  },
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "/test/path/agentic-memorizer read --format xml --wrap-json"
          }
        ]
      },
      {
        "matcher": "resume",
        "hooks": [
          {
            "type": "command",
            "command": "/test/path/agentic-memorizer read --format xml --wrap-json"
          }
        ]
      },
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "/test/path/agentic-memorizer read --format xml --wrap-json"
          }
        ]
      },
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "/test/path/agentic-memorizer read --format xml --wrap-json"
          }
        ]
      }
    ]
  }
}`,
			expectUpdatedCount: 0,
			verifyFn: func(t *testing.T, settingsPath string) {
				data, err := os.ReadFile(settingsPath)
				if err != nil {
					t.Fatalf("Failed to read settings: %v", err)
				}

				var fullSettings map[string]any
				if err := json.Unmarshal(data, &fullSettings); err != nil {
					t.Fatalf("Failed to parse settings: %v", err)
				}

				// Verify original fields preserved
				if fullSettings["awsCredentialExport"] != "doormat aws json" {
					t.Error("awsCredentialExport not preserved")
				}

				// Verify hooks unchanged
				hooks := fullSettings["hooks"].(map[string]any)
				sessionStart := hooks["SessionStart"].([]any)
				if len(sessionStart) != 4 {
					t.Errorf("Expected 4 SessionStart matchers, got %d", len(sessionStart))
				}
			},
		},
		{
			name: "hook command differs - needs update",
			initialSettings: `{
  "awsCredentialExport": "doormat aws json",
  "permissions": {
    "allow": ["WebSearch"]
  },
  "alwaysThinkingEnabled": true,
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "/test/path/agentic-memorizer --format json"
          }
        ]
      },
      {
        "matcher": "resume",
        "hooks": [
          {
            "type": "command",
            "command": "/test/path/agentic-memorizer --format json"
          }
        ]
      },
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "/test/path/agentic-memorizer --format json"
          }
        ]
      },
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "/test/path/agentic-memorizer --format json"
          }
        ]
      }
    ]
  }
}`,
			expectUpdatedCount: 4,
			verifyFn: func(t *testing.T, settingsPath string) {
				data, err := os.ReadFile(settingsPath)
				if err != nil {
					t.Fatalf("Failed to read settings: %v", err)
				}

				var fullSettings map[string]any
				if err := json.Unmarshal(data, &fullSettings); err != nil {
					t.Fatalf("Failed to parse settings: %v", err)
				}

				// Verify original fields preserved
				if fullSettings["awsCredentialExport"] != "doormat aws json" {
					t.Error("awsCredentialExport not preserved")
				}
				if fullSettings["alwaysThinkingEnabled"] != true {
					t.Error("alwaysThinkingEnabled not preserved")
				}

				// Verify hooks were updated
				settings, _, err := ReadSettings(settingsPath)
				if err != nil {
					t.Fatalf("Failed to read settings: %v", err)
				}

				for _, matcher := range []string{"startup", "resume", "clear", "compact"} {
					found := false
					for _, event := range settings.Hooks["SessionStart"] {
						if event.Matcher == matcher {
							for _, hook := range event.Hooks {
								if hook.Command == expectedCommand {
									found = true
									break
								}
							}
						}
					}
					if !found {
						t.Errorf("Hook command for matcher %s was not updated to: %s", matcher, expectedCommand)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test
			tmpDir := t.TempDir()
			settingsPath := filepath.Join(tmpDir, "settings.json")

			// Write initial settings
			if err := os.WriteFile(settingsPath, []byte(tt.initialSettings), 0644); err != nil {
				t.Fatalf("Failed to write initial settings: %v", err)
			}

			// Override the settings path for this test
			originalGetPath := GetClaudeSettingsPath
			GetClaudeSettingsPath = func() (string, error) {
				return settingsPath, nil
			}
			defer func() {
				GetClaudeSettingsPath = originalGetPath
			}()

			// Run setup
			_, updated, err := SetupSessionStartHooks(binaryPath)
			if err != nil {
				t.Fatalf("Failed to setup hooks: %v", err)
			}

			// Verify expected update count
			if len(updated) != tt.expectUpdatedCount {
				t.Errorf("Expected %d matchers to be updated, got %d", tt.expectUpdatedCount, len(updated))
			}

			// Run test-specific verification
			tt.verifyFn(t, settingsPath)
		})
	}
}
