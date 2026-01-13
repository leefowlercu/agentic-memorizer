package integrations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHookIntegration(t *testing.T) {
	t.Run("NewHookIntegration", func(t *testing.T) {
		hooks := []HookConfig{
			{HookType: "SessionStart", Matcher: ".*", Command: "test command", Timeout: 1000},
		}

		integration := NewHookIntegration(
			"test-hook",
			"test-harness",
			"Test hook integration",
			"testbin",
			"~/.test/settings.json",
			"hooks",
			hooks,
		)

		if integration.Name() != "test-hook" {
			t.Errorf("Name() = %q, want %q", integration.Name(), "test-hook")
		}
		if integration.Harness() != "test-harness" {
			t.Errorf("Harness() = %q, want %q", integration.Harness(), "test-harness")
		}
		if integration.Type() != IntegrationTypeHook {
			t.Errorf("Type() = %q, want %q", integration.Type(), IntegrationTypeHook)
		}
		if integration.Description() != "Test hook integration" {
			t.Errorf("Description() = %q, want %q", integration.Description(), "Test hook integration")
		}
	})

	t.Run("SetupAndTeardown", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "settings.json")

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		// Update PATH to include temp dir
		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		hooks := []HookConfig{
			{HookType: "SessionStart", Matcher: ".*", Command: "memorizer read", Timeout: 30000},
		}

		integration := NewHookIntegration(
			"test-hook",
			"test-harness",
			"Test",
			"testbin",
			configPath,
			"hooks",
			hooks,
		)

		ctx := context.Background()

		// Setup
		err := integration.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify hooks added
		data, _ := os.ReadFile(configPath)
		var config map[string]any
		_ = json.Unmarshal(data, &config)

		hooksSection, ok := config["hooks"].(map[string]any)
		if !ok {
			t.Fatal("hooks section not found")
		}

		if _, exists := hooksSection["memorizer-SessionStart"]; !exists {
			t.Error("memorizer-SessionStart hook not found")
		}

		// IsInstalled should return true
		installed, err := integration.IsInstalled()
		if err != nil {
			t.Fatalf("IsInstalled failed: %v", err)
		}
		if !installed {
			t.Error("IsInstalled should return true after setup")
		}

		// Teardown
		err = integration.Teardown(ctx)
		if err != nil {
			t.Fatalf("Teardown failed: %v", err)
		}

		// Verify hooks removed (use fresh map to avoid json.Unmarshal merge behavior)
		data, _ = os.ReadFile(configPath)
		var afterConfig map[string]any
		_ = json.Unmarshal(data, &afterConfig)

		if hooksSection, ok := afterConfig["hooks"].(map[string]any); ok {
			if _, exists := hooksSection["memorizer-SessionStart"]; exists {
				t.Error("memorizer-SessionStart hook should be removed after teardown")
			}
		}

		// IsInstalled should return false
		installed, err = integration.IsInstalled()
		if err != nil {
			t.Fatalf("IsInstalled failed: %v", err)
		}
		if installed {
			t.Error("IsInstalled should return false after teardown")
		}
	})

	t.Run("ValidateMissingBinary", func(t *testing.T) {
		integration := NewHookIntegration(
			"test",
			"test",
			"Test",
			"nonexistent-binary-12345",
			"/tmp/config.json",
			"hooks",
			nil,
		)

		err := integration.Validate()
		if err == nil {
			t.Error("Validate should fail for missing binary")
		}
	})

	t.Run("StatusMissingHarness", func(t *testing.T) {
		integration := NewHookIntegration(
			"test",
			"test",
			"Test",
			"nonexistent-binary-12345",
			"/tmp/config.json",
			"hooks",
			nil,
		)

		status, err := integration.Status()
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if status.Status != StatusMissingHarness {
			t.Errorf("Status = %q, want %q", status.Status, StatusMissingHarness)
		}
	})
}
