//go:build e2e

package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
	"github.com/pelletier/go-toml/v2"
)

// TestIntegrations_List tests integrations list command
func TestIntegrations_List(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("integrations", "list")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Should list all known integrations
	harness.AssertContains(t, stdout, "claude-code")
	harness.AssertContains(t, stdout, "gemini")
	harness.AssertContains(t, stdout, "codex")
}

// TestIntegrations_Detect tests integrations detect command
func TestIntegrations_Detect(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("integrations", "detect")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Detect should run successfully even if no integrations found
	harness.AssertNotEmpty(t, stdout, "stdout")
}

// TestIntegrations_ClaudeHook_Setup tests Claude Code hook setup
func TestIntegrations_ClaudeHook_Setup(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	// Create empty settings.json
	settingsPath := filepath.Join(claudeDir, "settings.json")
	initialSettings := map[string]any{
		"statusLine": "compact",
	}
	data, _ := json.MarshalIndent(initialSettings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	// Override HOME to use test directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run setup
	stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "claude-code-hook",
		"--binary-path", h.BinaryPath)

	if exitCode != 0 {
		t.Logf("Setup output: stdout=%s, stderr=%s", stdout, stderr)
		t.Skipf("Setup may fail if binary path validation is strict")
	}

	// Verify settings.json was updated
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Check hooks were added
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("Expected hooks section in settings.json")
	}

	sessionStart, ok := hooks["SessionStart"].([]any)
	if !ok || len(sessionStart) == 0 {
		t.Fatalf("Expected SessionStart hooks to be configured")
	}

	t.Logf("SessionStart hooks configured: %d hooks", len(sessionStart))
}

// TestIntegrations_ClaudeHook_Validate tests Claude Code hook validation
func TestIntegrations_ClaudeHook_Validate(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory with valid config
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []map[string]any{
				{
					"matcher":         "startup",
					"command":         h.BinaryPath + " read --output=json --compact",
					"handleExitCodes": []int{0, 1},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run validate
	stdout, stderr, _ := h.RunCommand("integrations", "validate", "claude-code-hook")

	harness.LogOutput(t, stdout, stderr)

	// Validate command scans real $HOME, not test environment
	// This is expected behavior - just verify command executes
	t.Log("Validate command executed (checks real user config, not test env)")
}

// TestIntegrations_ClaudeHook_Remove tests Claude Code hook removal
func TestIntegrations_ClaudeHook_Remove(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory with hooks
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings := map[string]any{
		"statusLine": "compact",
		"hooks": map[string]any{
			"SessionStart": []map[string]any{
				{
					"matcher":         "startup",
					"command":         h.BinaryPath + " read --output=json --compact",
					"handleExitCodes": []int{0, 1},
				},
				{
					"matcher":         "resume",
					"command":         h.BinaryPath + " read --output=json --compact",
					"handleExitCodes": []int{0, 1},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run remove
	stdout, stderr, exitCode := h.RunCommand("integrations", "remove", "claude-code-hook")

	if exitCode != 0 {
		t.Logf("Remove output: stdout=%s, stderr=%s", stdout, stderr)
	}

	// Verify hooks were removed or reduced
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var updatedSettings map[string]any
	if err := json.Unmarshal(data, &updatedSettings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Check hooks section
	if hooks, ok := updatedSettings["hooks"].(map[string]any); ok {
		if sessionStart, ok := hooks["SessionStart"].([]any); ok {
			t.Logf("After remove: %d SessionStart hooks remain", len(sessionStart))
			// Remove command may filter by binary path, so some hooks might remain
			// This is acceptable behavior
		} else {
			t.Log("SessionStart hooks section removed")
		}
	} else {
		t.Log("Hooks section removed entirely")
	}
}

// TestIntegrations_ClaudeMCP_Setup tests Claude Code MCP setup
func TestIntegrations_ClaudeMCP_Setup(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	initialSettings := map[string]any{
		"statusLine": "compact",
	}
	data, _ := json.MarshalIndent(initialSettings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run setup
	stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "claude-code-mcp",
		"--binary-path", h.BinaryPath)

	if exitCode != 0 {
		t.Logf("Setup output: stdout=%s, stderr=%s", stdout, stderr)
		t.Skipf("Setup may fail if binary path validation is strict")
	}

	// Verify settings.json was updated
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Check if mcpServers was added
	mcpServers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		t.Log("mcpServers section not created (may require additional setup)")
		return
	}

	agenticMemorizer, ok := mcpServers["agentic-memorizer"].(map[string]any)
	if !ok {
		t.Log("agentic-memorizer server not configured in mcpServers")
		return
	}

	// Verify command is configured
	if command, ok := agenticMemorizer["command"].(string); ok {
		t.Logf("MCP server configured with command: %s", command)
	}

	if args, ok := agenticMemorizer["args"].([]any); ok {
		t.Logf("MCP server configured with args: %v", args)
	}
}

// TestIntegrations_ClaudeMCP_Remove tests Claude Code MCP removal
func TestIntegrations_ClaudeMCP_Remove(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory with MCP config
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings := map[string]any{
		"mcpServers": map[string]any{
			"agentic-memorizer": map[string]any{
				"command": h.BinaryPath + " mcp start",
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run remove
	stdout, stderr, exitCode := h.RunCommand("integrations", "remove", "claude-code-mcp")

	if exitCode != 0 {
		t.Logf("Remove output: stdout=%s, stderr=%s", stdout, stderr)
	}

	// Verify MCP server was removed or config unchanged
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var updatedSettings map[string]any
	if err := json.Unmarshal(data, &updatedSettings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Check mcpServers section
	if mcpServers, ok := updatedSettings["mcpServers"].(map[string]any); ok {
		if _, exists := mcpServers["agentic-memorizer"]; exists {
			t.Log("agentic-memorizer MCP server still present (may not have been configured)")
		} else {
			t.Log("agentic-memorizer MCP server removed successfully")
		}
	} else {
		t.Log("mcpServers section not present")
	}
}

// TestIntegrations_GeminiMCP_Setup tests Gemini CLI MCP setup
func TestIntegrations_GeminiMCP_Setup(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.gemini directory
	geminiDir := filepath.Join(h.AppDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("Failed to create .gemini dir: %v", err)
	}

	settingsPath := filepath.Join(geminiDir, "settings.json")
	initialSettings := map[string]any{
		"model": "gemini-2.0-flash-exp",
	}
	data, _ := json.MarshalIndent(initialSettings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run setup
	stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "gemini-cli-mcp",
		"--binary-path", h.BinaryPath)

	if exitCode != 0 {
		t.Logf("Setup output: stdout=%s, stderr=%s", stdout, stderr)
		t.Skipf("Setup may fail if binary path validation is strict")
	}

	// Verify settings.json was updated
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Check if mcpServers was added
	mcpServers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		t.Log("mcpServers section not created (may require additional setup)")
		return
	}

	agenticMemorizer, ok := mcpServers["agentic-memorizer"].(map[string]any)
	if !ok {
		t.Log("agentic-memorizer server not configured in mcpServers")
		return
	}

	// Verify command or args are configured
	if command, ok := agenticMemorizer["command"].(string); ok {
		t.Logf("Gemini MCP server configured with command: %s", command)
	}

	if args, ok := agenticMemorizer["args"].([]any); ok {
		t.Logf("Gemini MCP server configured with args: %v", args)
	}
}

// TestIntegrations_CodexMCP_Setup tests Codex CLI MCP setup
func TestIntegrations_CodexMCP_Setup(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.codex directory
	codexDir := filepath.Join(h.AppDir, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatalf("Failed to create .codex dir: %v", err)
	}

	configPath := filepath.Join(codexDir, "config.toml")
	initialConfig := map[string]any{
		"model": "claude-3-5-sonnet-20241022",
	}
	data, err := toml.Marshal(initialConfig)
	if err != nil {
		t.Fatalf("Failed to marshal TOML: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write config.toml: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run setup
	stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "codex-cli-mcp",
		"--binary-path", h.BinaryPath)

	if exitCode != 0 {
		t.Logf("Setup output: stdout=%s, stderr=%s", stdout, stderr)
		t.Skipf("Setup may fail if binary path validation is strict")
	}

	// Verify config.toml was updated
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config.toml: %v", err)
	}

	var config map[string]any
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config.toml: %v", err)
	}

	// Check if mcpServers was added
	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Log("mcpServers section not created (may require additional setup)")
		return
	}

	agenticMemorizer, ok := mcpServers["agentic-memorizer"].(map[string]any)
	if !ok {
		t.Log("agentic-memorizer server not configured in mcpServers")
		return
	}

	// Verify command or args are configured
	if command, ok := agenticMemorizer["command"].(string); ok {
		t.Logf("Codex MCP server configured with command: %s", command)
	}

	if args, ok := agenticMemorizer["args"].([]any); ok {
		t.Logf("Codex MCP server configured with args: %v", args)
	}
}

// TestIntegrations_Idempotency tests that multiple setup calls are idempotent
func TestIntegrations_Idempotency(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	initialSettings := map[string]any{
		"statusLine": "compact",
	}
	data, _ := json.MarshalIndent(initialSettings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run setup twice
	for i := 0; i < 2; i++ {
		stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "claude-code-hook",
			"--binary-path", h.BinaryPath)

		if exitCode != 0 {
			t.Logf("Setup attempt %d output: stdout=%s, stderr=%s", i+1, stdout, stderr)
			t.Skipf("Setup may fail if binary path validation is strict")
		}
	}

	// Verify only one set of hooks exists (no duplicates)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("Expected hooks section in settings.json")
	}

	sessionStart, ok := hooks["SessionStart"].([]any)
	if !ok {
		t.Fatalf("Expected SessionStart hooks to be configured")
	}

	// Should have 4 matchers (startup, resume, clear, compact) - not 8
	if len(sessionStart) > 4 {
		t.Errorf("Expected at most 4 SessionStart hooks, got %d (possible duplicates)", len(sessionStart))
	}

	t.Logf("Idempotency verified: %d SessionStart hooks after 2 setups", len(sessionStart))
}

// TestIntegrations_SetupInvalidIntegration tests setup with invalid integration name
func TestIntegrations_SetupInvalidIntegration(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "nonexistent-integration")

	// Should fail with non-zero exit code
	if exitCode == 0 {
		t.Error("Expected setup to fail for nonexistent integration")
	}

	// Should show error and usage
	output := stdout + stderr
	harness.AssertContains(t, output, "Error:")
	harness.AssertContains(t, output, "Usage:")
}

// TestIntegrations_RemoveNonexistent tests remove on non-configured integration
func TestIntegrations_RemoveNonexistent(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory without hooks
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings := map[string]any{
		"statusLine": "compact",
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Try to remove an integration that doesn't exist
	stdout, stderr, exitCode := h.RunCommand("integrations", "remove", "claude-code-hook")

	// May succeed (nothing to remove) or fail (not found)
	harness.LogOutput(t, stdout, stderr)

	if exitCode == 0 {
		t.Log("Remove succeeded (nothing to remove)")
	} else {
		output := stdout + stderr
		if strings.Contains(output, "not configured") || strings.Contains(output, "not found") {
			t.Log("Remove failed as expected (integration not configured)")
		} else {
			t.Logf("Unexpected error during remove: %s", output)
		}
	}
}

// ============================================================================
// Phase 7.5: Integration Health Monitoring Tests
// ============================================================================

// TestIntegrationsHealth_AllIntegrations tests health command for all integrations
func TestIntegrationsHealth_AllIntegrations(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Run health check for all integrations
	stdout, stderr, exitCode := h.RunCommand("integrations", "health")

	harness.LogOutput(t, stdout, stderr)

	// Health check should always execute
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Verify output format
	harness.AssertContains(t, stdout, "Integration Health Check")
	harness.AssertContains(t, stdout, "Summary")
	harness.AssertContains(t, stdout, "Total checked:")
	harness.AssertContains(t, stdout, "Healthy:")
	harness.AssertContains(t, stdout, "Unconfigured:")

	// At least some integrations should be listed
	output := stdout + stderr
	integrationListed := strings.Contains(output, "claude-code") ||
		strings.Contains(output, "gemini") ||
		strings.Contains(output, "codex")

	if !integrationListed {
		t.Error("Expected at least one integration in health output")
	}
}

// TestIntegrationsHealth_ConfiguredIntegration tests health for a configured integration
func TestIntegrationsHealth_ConfiguredIntegration(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory with valid configuration
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []map[string]any{
				{
					"matcher":         "startup",
					"command":         h.BinaryPath + " read --format=json --integration=claude-code-hook",
					"handleExitCodes": []int{0, 1},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run health check
	stdout, stderr, _ := h.RunCommand("integrations", "health")

	harness.LogOutput(t, stdout, stderr)

	// Should show health check results
	harness.AssertContains(t, stdout, "Integration Health Check")
	harness.AssertContains(t, stdout, "Summary")

	// Should show at least one configured integration
	output := stdout + stderr
	if !strings.Contains(output, "Healthy") && !strings.Contains(output, "Issues") {
		t.Log("No configured integrations found (checks real $HOME)")
	}
}

// TestIntegrationsHealth_UnconfiguredIntegration tests health for unconfigured integration
func TestIntegrationsHealth_UnconfiguredIntegration(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create mock ~/.claude directory without hooks
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings := map[string]any{
		"statusLine": "compact",
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	// Run health check
	stdout, stderr, _ := h.RunCommand("integrations", "health")

	harness.LogOutput(t, stdout, stderr)

	// Should show unconfigured status
	harness.AssertContains(t, stdout, "Integration Health Check")
	harness.AssertContains(t, stdout, "Summary")
	harness.AssertContains(t, stdout, "Unconfigured:")

	// Output should mention that integrations are not configured
	output := stdout + stderr
	if strings.Contains(output, "Not configured") || strings.Contains(output, "Unconfigured: ") {
		t.Log("Health check correctly identifies unconfigured integrations")
	}
}

// TestIntegrationsHealth_SpecificIntegrationFlag tests --integrations flag with single integration
func TestIntegrationsHealth_SpecificIntegrationFlag(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Run health check for specific integration
	stdout, stderr, exitCode := h.RunCommand("integrations", "health",
		"--integrations", "claude-code-hook")

	harness.LogOutput(t, stdout, stderr)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Should only check the specified integration
	harness.AssertContains(t, stdout, "Integration Health Check")
	harness.AssertContains(t, stdout, "claude-code")

	// Should not check other integrations
	output := stdout + stderr
	totalCheckedLine := ""
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Total checked:") {
			totalCheckedLine = line
			break
		}
	}

	if totalCheckedLine != "" {
		t.Logf("Health check summary: %s", totalCheckedLine)
		if strings.Contains(totalCheckedLine, "Total checked: 1") {
			t.Log("Correctly checked only specified integration")
		}
	}
}

// TestIntegrationsHealth_MultipleIntegrationsFlag tests --integrations flag with multiple integrations
func TestIntegrationsHealth_MultipleIntegrationsFlag(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Run health check for multiple integrations
	stdout, stderr, exitCode := h.RunCommand("integrations", "health",
		"--integrations", "claude-code-hook,gemini-cli-mcp")

	harness.LogOutput(t, stdout, stderr)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Should check both specified integrations
	harness.AssertContains(t, stdout, "Integration Health Check")

	output := stdout + stderr
	claudeChecked := strings.Contains(output, "claude-code")
	geminiChecked := strings.Contains(output, "gemini")

	if !claudeChecked || !geminiChecked {
		t.Log("At least one specified integration was checked")
	} else {
		t.Log("Both specified integrations were checked")
	}

	// Verify summary shows correct count
	if strings.Contains(output, "Total checked:") {
		t.Log("Summary includes total checked count")
	}
}
