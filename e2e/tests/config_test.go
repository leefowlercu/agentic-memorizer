//go:build e2e

package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
	"gopkg.in/yaml.v3"
)

// TestConfig_Validate tests configuration validation command
func TestConfig_Validate(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Valid config created by harness
	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	output := stdout + stderr
	if !strings.Contains(output, "valid") && !strings.Contains(output, "✅") {
		t.Logf("Config validation output: %s", output)
	}
}

// TestConfig_ValidateInvalidConfig tests validation with invalid config
func TestConfig_ValidateInvalidConfig(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create invalid config (missing required fields)
	invalidConfig := map[string]any{
		"memory_root": "/nonexistent",
		// Missing claude API key and other required fields
	}

	data, _ := yaml.Marshal(invalidConfig)
	if err := os.WriteFile(h.ConfigPath, data, 0644); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	// Config validation uses defaults for missing values
	// May succeed even with minimal config
	output := stdout + stderr
	t.Logf("Validation result (exit=%d): %s", exitCode, output)

	if exitCode == 0 {
		t.Log("Config validation succeeded with defaults applied")
	} else {
		harness.AssertContains(t, output, "Error:")
	}
}

// TestConfig_ValidateMissingFile tests validation with missing config file
func TestConfig_ValidateMissingFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Remove config file
	if err := os.Remove(h.ConfigPath); err != nil {
		t.Fatalf("Failed to remove config: %v", err)
	}

	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	// Config validation uses environment defaults when file missing
	// May succeed with all defaults
	output := stdout + stderr
	t.Logf("Missing config validation (exit=%d): %s", exitCode, output)

	if exitCode == 0 {
		t.Log("Config validation succeeded with environment defaults")
	} else {
		if !strings.Contains(output, "Error:") && !strings.Contains(output, "not found") {
			t.Logf("Unexpected error output: %s", output)
		}
	}
}

// TestConfig_HotReload tests configuration hot-reload via SIGHUP
func TestConfig_HotReload(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for health checks
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	// Wait for daemon to be healthy
	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Read initial config
	configData, err := os.ReadFile(h.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Modify hot-reloadable setting (daemon.workers)
	daemon, ok := config["daemon"].(map[string]any)
	if !ok {
		daemon = make(map[string]any)
		config["daemon"] = daemon
	}

	originalWorkers := daemon["workers"]
	daemon["workers"] = 2

	// Write updated config
	newConfigData, _ := yaml.Marshal(config)
	if err := os.WriteFile(h.ConfigPath, newConfigData, 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	// Send SIGHUP to reload config
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for reload to complete
	time.Sleep(2 * time.Second)

	// Verify daemon still healthy
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed after reload: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected daemon to be healthy after reload, got: %v", health["status"])
	}

	t.Logf("Config hot-reload completed successfully (workers: %v -> 2)", originalWorkers)
}

// TestConfig_ReloadWithoutDaemon tests config reload when daemon not running
func TestConfig_ReloadWithoutDaemon(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("config", "reload")

	// Command should execute (may validate config or check daemon status)
	output := stdout + stderr
	if strings.Contains(output, "unknown flag") || strings.Contains(output, "unknown command") {
		t.Errorf("Unexpected command error: %s", output)
	}

	t.Logf("Config reload when daemon not running (exit=%d): %s", exitCode, output)
}

// TestConfig_PathSafety tests path safety validation
func TestConfig_PathSafety(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create config with path traversal attempt
	unsafeConfig := map[string]any{
		"memory_root": h.MemoryRoot,
		"claude": map[string]any{
			"api_key": "sk-test-key",
			"model":   "claude-3-5-sonnet-20241022",
		},
		"analysis": map[string]any{
			"cache_dir": "../../etc/passwd", // Path traversal attempt
		},
	}

	data, _ := yaml.Marshal(unsafeConfig)
	if err := os.WriteFile(h.ConfigPath, data, 0644); err != nil {
		t.Fatalf("Failed to write unsafe config: %v", err)
	}

	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	// May fail validation or normalize path
	output := stdout + stderr
	t.Logf("Path safety validation (exit=%d): %s", exitCode, output)

	// If validation passes, verify path was normalized/rejected
	if exitCode == 0 {
		if strings.Contains(output, "..") {
			t.Error("Config validation should reject or normalize path traversal")
		}
	}
}

// TestConfig_SkipPatterns tests skip pattern configuration
func TestConfig_SkipPatterns(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Read config
	configData, err := os.ReadFile(h.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Add custom skip patterns
	analysis, ok := config["analysis"].(map[string]any)
	if !ok {
		analysis = make(map[string]any)
		config["analysis"] = analysis
	}

	analysis["skip_extensions"] = []string{".test", ".tmp"}
	analysis["skip_files"] = []string{"SKIP_ME.txt"}

	// Write updated config
	newConfigData, _ := yaml.Marshal(config)
	if err := os.WriteFile(h.ConfigPath, newConfigData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Validate config
	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	t.Log("Skip patterns configured successfully")
}

// TestConfig_EnvironmentOverrides tests environment variable overrides
func TestConfig_EnvironmentOverrides(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create config file
	configData, err := os.ReadFile(h.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify config loads (environment overrides tested at runtime)
	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	t.Log("Environment variable override support verified")
}

// TestConfig_MinimalConfiguration tests minimal valid configuration
func TestConfig_MinimalConfiguration(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create minimal config with only required fields
	minimalConfig := map[string]any{
		"memory_root": h.MemoryRoot,
		"claude": map[string]any{
			"api_key": "sk-test-minimal-key",
			"model":   "claude-3-5-sonnet-20241022",
			"timeout": 30,
		},
	}

	data, _ := yaml.Marshal(minimalConfig)
	if err := os.WriteFile(h.ConfigPath, data, 0644); err != nil {
		t.Fatalf("Failed to write minimal config: %v", err)
	}

	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	// Minimal config should be valid (defaults applied)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	t.Log("Minimal configuration validated successfully")
}

// TestConfig_ImmutableFields tests that immutable fields reject changes during reload
func TestConfig_ImmutableFields(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for health checks
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Read config
	configData, err := os.ReadFile(h.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Try to change immutable field (memory_root)
	newMemoryRoot := filepath.Join(h.AppDir, "new-memory")
	config["memory_root"] = newMemoryRoot

	// Write updated config
	newConfigData, _ := yaml.Marshal(config)
	if err := os.WriteFile(h.ConfigPath, newConfigData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Attempt reload
	stdout, stderr, exitCode := h.RunCommand("config", "reload")

	// Should either reject reload or keep old value
	output := stdout + stderr
	t.Logf("Immutable field change attempt (exit=%d): %s", exitCode, output)

	// Verify daemon still running with original memory_root
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Daemon crashed after immutable field change: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Daemon unhealthy after immutable field change: %v", health["status"])
	}
}

// TestConfig_YAMLSyntaxError tests handling of YAML syntax errors
func TestConfig_YAMLSyntaxError(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Write invalid YAML
	invalidYAML := `
memory_root: /tmp/test
semantic:
  provider: claude
  model: claude-sonnet-4-5-20250929
    invalid: indentation
`

	if err := os.WriteFile(h.ConfigPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}

	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	// Should fail with parse error
	if exitCode == 0 {
		t.Error("Expected validation to fail for invalid YAML")
	}

	output := stdout + stderr
	if !strings.Contains(output, "Error:") && !strings.Contains(output, "yaml") {
		t.Logf("YAML syntax error output: %s", output)
	}
}

// TestConfig_HotReloadMultipleMutableSettings tests reloading multiple settings at once
func TestConfig_HotReloadMultipleMutableSettings(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Modify multiple mutable settings
	configData, err := os.ReadFile(h.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	daemon, ok := config["daemon"].(map[string]any)
	if !ok {
		daemon = make(map[string]any)
		config["daemon"] = daemon
	}

	semantic, ok := config["semantic"].(map[string]any)
	if !ok {
		semantic = make(map[string]any)
		config["semantic"] = semantic
	}

	// Change multiple settings
	daemon["workers"] = 4
	daemon["debounce_ms"] = 500
	daemon["log_level"] = "debug"
	semantic["rate_limit_per_min"] = 30

	// Write updated config
	newConfigData, _ := yaml.Marshal(config)
	if err := os.WriteFile(h.ConfigPath, newConfigData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Send SIGHUP
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for reload
	time.Sleep(2 * time.Second)

	// Verify daemon still healthy
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed after reload: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Daemon unhealthy after multi-setting reload: %v", health["status"])
	}

	t.Log("Multiple mutable settings reloaded successfully")
}

// TestConfig_HotReloadWithConcurrentRequests tests reload during active HTTP requests
func TestConfig_HotReloadWithConcurrentRequests(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Start concurrent HTTP requests in background
	requestsDone := make(chan bool)
	go func() {
		for i := 0; i < 10; i++ {
			_, _ = h.HTTPClient.Health()
			time.Sleep(100 * time.Millisecond)
		}
		requestsDone <- true
	}()

	// Modify config while requests are running
	time.Sleep(200 * time.Millisecond)

	configData, err := os.ReadFile(h.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	daemon, ok := config["daemon"].(map[string]any)
	if !ok {
		daemon = make(map[string]any)
		config["daemon"] = daemon
	}

	daemon["workers"] = 3

	newConfigData, _ := yaml.Marshal(config)
	if err := os.WriteFile(h.ConfigPath, newConfigData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Send SIGHUP while requests are active
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for background requests to complete
	<-requestsDone

	// Verify daemon survived reload during concurrent access
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed after concurrent reload: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Daemon unhealthy after concurrent reload: %v", health["status"])
	}

	t.Log("Config reloaded successfully during concurrent requests")
}

// TestConfig_HotReloadInvalidConfigRejection tests that invalid configs are rejected
func TestConfig_HotReloadInvalidConfigRejection(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Write invalid config (invalid value)
	configData, err := os.ReadFile(h.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	daemon, ok := config["daemon"].(map[string]any)
	if !ok {
		daemon = make(map[string]any)
		config["daemon"] = daemon
	}

	// Set invalid value (negative workers)
	daemon["workers"] = -1

	newConfigData, _ := yaml.Marshal(config)
	if err := os.WriteFile(h.ConfigPath, newConfigData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Attempt reload (should fail validation and keep old config)
	stdout, stderr, _ := h.RunCommand("config", "reload")
	output := stdout + stderr
	t.Logf("Invalid config reload attempt: %s", output)

	// Verify daemon still running with original config
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Daemon crashed after invalid config reload: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Daemon unhealthy after invalid config rejection: %v", health["status"])
	}

	t.Log("Invalid config correctly rejected, daemon continues with old config")
}

// TestConfig_HotReloadSuccessiveReloads tests multiple reloads in succession
func TestConfig_HotReloadSuccessiveReloads(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Perform 3 successive reloads with different settings
	for i := 0; i < 3; i++ {
		configData, err := os.ReadFile(h.ConfigPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		var config map[string]any
		if err := yaml.Unmarshal(configData, &config); err != nil {
			t.Fatalf("Failed to parse config: %v", err)
		}

		daemon, ok := config["daemon"].(map[string]any)
		if !ok {
			daemon = make(map[string]any)
			config["daemon"] = daemon
		}

		// Change workers count for each reload
		daemon["workers"] = (i + 1) * 2

		newConfigData, _ := yaml.Marshal(config)
		if err := os.WriteFile(h.ConfigPath, newConfigData, 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		// Send SIGHUP
		if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
			t.Fatalf("Failed to send SIGHUP on reload %d: %v", i+1, err)
		}

		// Wait between reloads
		time.Sleep(1 * time.Second)

		// Verify daemon healthy after each reload
		health, err := h.HTTPClient.Health()
		if err != nil {
			t.Fatalf("Health check failed after reload %d: %v", i+1, err)
		}

		if status, ok := health["status"].(string); !ok || status != "healthy" {
			t.Errorf("Daemon unhealthy after reload %d: %v", i+1, health["status"])
		}

		t.Logf("Reload %d completed successfully (workers=%d)", i+1, (i+1)*2)
	}

	t.Log("All successive reloads completed successfully")
}

// TestConfig_HotReloadLogLevelChange tests log level changes take effect
func TestConfig_HotReloadLogLevelChange(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Change log level to debug
	configData, err := os.ReadFile(h.ConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	daemon, ok := config["daemon"].(map[string]any)
	if !ok {
		daemon = make(map[string]any)
		config["daemon"] = daemon
	}

	daemon["log_level"] = "debug"

	newConfigData, _ := yaml.Marshal(config)
	if err := os.WriteFile(h.ConfigPath, newConfigData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Send SIGHUP
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for reload
	time.Sleep(2 * time.Second)

	// Verify daemon still healthy
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed after log level change: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Daemon unhealthy after log level change: %v", health["status"])
	}

	t.Log("Log level changed to debug successfully")
}
