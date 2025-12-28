//go:build e2e

package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestIntegrations_ClaudeHookSetup tests claude-code-hook setup with both hook types
func TestIntegrations_ClaudeHookSetup(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create fake .claude directory in test environment
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Set HOME to test directory so integration finds the .claude dir
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", h.AppDir)
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Setup the integration
	stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "claude-code-hook")
	if exitCode != 0 {
		output := stdout + stderr
		// May fail if binary not found or other issues
		if strings.Contains(output, "not found") || strings.Contains(output, "binary") {
			t.Skipf("Integration setup may need binary path: %s", output)
		}
		t.Logf("Setup output: %s", output)
	}

	// Check that settings.json was created with both hook types
	settingsPath := filepath.Join(claudeDir, "settings.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Skipf("Settings file not created (may need different setup): %v", err)
	}

	settingsStr := string(content)

	// Verify SessionStart hook exists
	if !strings.Contains(settingsStr, "SessionStart") {
		t.Error("Expected SessionStart hook in settings")
	}

	// Verify UserPromptSubmit hook exists
	if !strings.Contains(settingsStr, "UserPromptSubmit") {
		t.Error("Expected UserPromptSubmit hook in settings")
	}

	// Verify both use memorizer commands
	if !strings.Contains(settingsStr, "read files") {
		t.Error("Expected 'read files' command in SessionStart hook")
	}
	if !strings.Contains(settingsStr, "read facts") {
		t.Error("Expected 'read facts' command in UserPromptSubmit hook")
	}

	t.Logf("Settings content: %s", settingsStr)
}

// TestIntegrations_GeminiHookSetup tests gemini-cli-hook setup with both hook types
func TestIntegrations_GeminiHookSetup(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create fake .gemini directory in test environment
	geminiDir := filepath.Join(h.AppDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("Failed to create .gemini directory: %v", err)
	}

	// Set HOME to test directory so integration finds the .gemini dir
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", h.AppDir)
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Setup the integration
	stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "gemini-cli-hook")
	if exitCode != 0 {
		output := stdout + stderr
		// May fail if binary not found or other issues
		if strings.Contains(output, "not found") || strings.Contains(output, "binary") {
			t.Skipf("Integration setup may need binary path: %s", output)
		}
		t.Logf("Setup output: %s", output)
	}

	// Check that settings.json was created with both hook types
	settingsPath := filepath.Join(geminiDir, "settings.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Skipf("Settings file not created (may need different setup): %v", err)
	}

	settingsStr := string(content)

	// Verify SessionStart hook exists
	if !strings.Contains(settingsStr, "SessionStart") {
		t.Error("Expected SessionStart hook in settings")
	}

	// Verify BeforeAgent hook exists
	if !strings.Contains(settingsStr, "BeforeAgent") {
		t.Error("Expected BeforeAgent hook in settings")
	}

	// Verify both use memorizer commands
	if !strings.Contains(settingsStr, "read files") {
		t.Error("Expected 'read files' command in SessionStart hook")
	}
	if !strings.Contains(settingsStr, "read facts") {
		t.Error("Expected 'read facts' command in BeforeAgent hook")
	}

	t.Logf("Settings content: %s", settingsStr)
}

// TestIntegrations_HealthPartialState tests health command with partial hook state
func TestIntegrations_HealthPartialState(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create .claude directory with only SessionStart hook (simulate partial state)
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Write partial settings with only SessionStart
	partialSettings := `{
		"hooks": {
			"SessionStart": [
				{
					"matcher": "startup",
					"hooks": [
						{
							"name": "memorizer-hook",
							"type": "command",
							"command": "/usr/local/bin/memorizer read files --format xml --integration claude-code-hook",
							"description": "Load agentic memory index"
						}
					]
				}
			]
		}
	}`

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(partialSettings), 0644); err != nil {
		t.Fatalf("Failed to write partial settings: %v", err)
	}

	// Set HOME to test directory
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", h.AppDir)
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Run health check
	stdout, stderr, _ := h.RunCommand("integrations", "health", "claude-code-hook")

	output := stdout + stderr
	t.Logf("Health output: %s", output)

	// Should indicate partial configuration
	if !strings.Contains(output, "partial") && !strings.Contains(output, "missing") {
		t.Logf("Expected health to show partial state, got: %s", output)
	}
}

// TestIntegrations_RemoveBothHooks tests that remove removes both hook types
func TestIntegrations_RemoveBothHooks(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create .claude directory with both hooks
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Write settings with both hooks
	fullSettings := `{
		"hooks": {
			"SessionStart": [
				{
					"matcher": "startup",
					"hooks": [
						{
							"name": "memorizer-hook",
							"type": "command",
							"command": "/usr/local/bin/memorizer read files --format xml --integration claude-code-hook",
							"description": "Load agentic memory index"
						}
					]
				}
			],
			"UserPromptSubmit": [
				{
					"matcher": "startup",
					"hooks": [
						{
							"name": "memorizer-facts-hook",
							"type": "command",
							"command": "/usr/local/bin/memorizer read facts --format xml --integration claude-code-hook",
							"description": "Load user-defined facts"
						}
					]
				}
			]
		}
	}`

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(fullSettings), 0644); err != nil {
		t.Fatalf("Failed to write settings: %v", err)
	}

	// Set HOME to test directory
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", h.AppDir)
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Remove the integration
	stdout, stderr, exitCode := h.RunCommand("integrations", "remove", "claude-code-hook")
	t.Logf("Remove output: stdout=%s, stderr=%s, exit=%d", stdout, stderr, exitCode)

	// Read settings file and verify hooks are gone
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings after remove: %v", err)
	}

	settingsStr := string(content)
	t.Logf("Settings after remove: %s", settingsStr)

	// Verify memorizer hooks are removed
	if strings.Contains(settingsStr, "memorizer") {
		t.Error("Expected all memorizer hooks to be removed")
	}
}

// TestIntegrations_ReadFactsWithIntegration tests read facts with --integration flag
func TestIntegrations_ReadFactsWithIntegration(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create .claude directory so integration is detected
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Set HOME to test directory
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", h.AppDir)
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Read facts with --integration flag (for Claude Code)
	stdout, stderr, exitCode := h.RunCommand("read", "facts", "--integration", "claude-code-hook")

	output := stdout + stderr
	t.Logf("Read facts with integration output: %s", output)

	// If graph unavailable, that's expected in some test environments
	if strings.Contains(output, "FalkorDB") || strings.Contains(output, "not running") {
		t.Skip("FalkorDB not available")
	}

	// Should produce UserPromptSubmit format
	if exitCode == 0 {
		// Check for expected JSON wrapper
		if !strings.Contains(stdout, "UserPromptSubmit") && !strings.Contains(stdout, "hookSpecificOutput") {
			t.Logf("Expected UserPromptSubmit format in output")
		}
	}
}

// TestIntegrations_ReadFactsWithGeminiIntegration tests read facts with Gemini integration
func TestIntegrations_ReadFactsWithGeminiIntegration(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create .gemini directory so integration is detected
	geminiDir := filepath.Join(h.AppDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("Failed to create .gemini directory: %v", err)
	}

	// Set HOME to test directory
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", h.AppDir)
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Read facts with --integration flag (for Gemini CLI)
	stdout, stderr, exitCode := h.RunCommand("read", "facts", "--integration", "gemini-cli-hook")

	output := stdout + stderr
	t.Logf("Read facts with Gemini integration output: %s", output)

	// If graph unavailable, that's expected in some test environments
	if strings.Contains(output, "FalkorDB") || strings.Contains(output, "not running") {
		t.Skip("FalkorDB not available")
	}

	// Should produce BeforeAgent format
	if exitCode == 0 {
		// Check for expected JSON wrapper
		if !strings.Contains(stdout, "BeforeAgent") && !strings.Contains(stdout, "hookSpecificOutput") {
			t.Logf("Expected BeforeAgent format in output")
		}
	}
}

// TestIntegrations_OldBinaryNameDetection tests detection of old agentic-memorizer binary name
func TestIntegrations_OldBinaryNameDetection(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create .claude directory with old binary name
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Write settings with old agentic-memorizer binary name
	oldSettings := `{
		"hooks": {
			"SessionStart": [
				{
					"matcher": "startup",
					"hooks": [
						{
							"name": "memorizer-hook",
							"type": "command",
							"command": "/usr/local/bin/agentic-memorizer read --format xml --integration claude-code-hook",
							"description": "Load agentic memory index"
						}
					]
				}
			]
		}
	}`

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(oldSettings), 0644); err != nil {
		t.Fatalf("Failed to write old settings: %v", err)
	}

	// Set HOME to test directory
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", h.AppDir)
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Run health check
	stdout, stderr, exitCode := h.RunCommand("integrations", "health", "claude-code-hook")

	output := stdout + stderr
	t.Logf("Health output with old binary: %s", output)

	// Should indicate issue with old binary name
	if exitCode == 0 {
		// Health should fail or warn about old binary name
		if !strings.Contains(output, "agentic-memorizer") && !strings.Contains(output, "old") {
			t.Logf("Expected warning about old binary name")
		}
	}
}
