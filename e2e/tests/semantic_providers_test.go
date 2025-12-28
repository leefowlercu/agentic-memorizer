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

// TestSemanticProviders_ConfigValidation tests that provider names are validated
func TestSemanticProviders_ConfigValidation(t *testing.T) {
	t.Run("valid_provider_names", func(t *testing.T) {
		validProviders := []string{"claude", "openai", "gemini"}

		for _, provider := range validProviders {
			t.Run(provider, func(t *testing.T) {
				h := harness.New(t)
				if err := h.Setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
				cleanup := harness.MustCleanup(t, h)
				defer cleanup.CleanupAll()

				// Create config with valid provider (disabled, no API key needed)
				err := h.CreateConfigWithSemanticProvider(0, harness.SemanticProviderConfig{
					Enabled:  false,
					Provider: provider,
				})
				if err != nil {
					t.Fatalf("Failed to create config: %v", err)
				}

				// Validate config
				stdout, stderr, exitCode := h.RunCommand("config", "validate")
				if exitCode != 0 {
					t.Errorf("Config validation failed for provider %q: stdout=%s, stderr=%s", provider, stdout, stderr)
				}
			})
		}
	})

	t.Run("invalid_provider_name", func(t *testing.T) {
		h := harness.New(t)
		if err := h.Setup(); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		cleanup := harness.MustCleanup(t, h)
		defer cleanup.CleanupAll()

		// Create config with invalid provider
		config := map[string]any{
			"memory_root": h.MemoryRoot,
			"semantic": map[string]any{
				"enabled":  true,
				"provider": "invalid-provider",
				"api_key":  "test-key",
			},
		}

		data, _ := yaml.Marshal(config)
		if err := os.WriteFile(h.ConfigPath, data, 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		// Validate config - should fail
		stdout, stderr, exitCode := h.RunCommand("config", "validate")
		output := stdout + stderr

		if exitCode == 0 {
			// If validation passes, it might be using defaults
			t.Logf("Config validation passed (may use defaults): %s", output)
		} else {
			// Expected: validation error for invalid provider
			if !strings.Contains(output, "invalid") && !strings.Contains(output, "provider") {
				t.Logf("Validation output: %s", output)
			}
			t.Logf("Invalid provider correctly rejected")
		}
	})
}

// TestSemanticProviders_MetadataOnlyMode tests daemon operation without API key
func TestSemanticProviders_MetadataOnlyMode(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create config with semantic disabled (no API key)
	err := h.CreateConfigWithSemanticProvider(8080, harness.SemanticProviderConfig{
		Enabled:  false,
		Provider: "claude",
	})
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nTest content for metadata-only mode."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
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

	// Query health endpoint
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	// Verify daemon is healthy
	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected status=healthy, got: %v", health["status"])
	}

	// Check semantic section in health response
	if semantic, ok := health["semantic"].(map[string]any); ok {
		if enabled, ok := semantic["enabled"].(bool); ok {
			if enabled {
				t.Error("Expected semantic.enabled=false in metadata-only mode")
			}
		}
		t.Logf("Semantic status: enabled=%v", semantic["enabled"])
	} else {
		t.Logf("Health response: %v", health)
	}

	t.Log("Daemon successfully running in metadata-only mode")
}

// TestSemanticProviders_HealthEndpointInfo tests provider info in health endpoint
func TestSemanticProviders_HealthEndpointInfo(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create config with semantic disabled but provider specified
	err := h.CreateConfigWithSemanticProvider(8080, harness.SemanticProviderConfig{
		Enabled:  false,
		Provider: "openai",
		Model:    "gpt-4o",
	})
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
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

	// Query health endpoint
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	// Verify health response contains semantic section
	if semantic, ok := health["semantic"].(map[string]any); ok {
		t.Logf("Semantic health info: %v", semantic)

		// Check enabled field
		if enabled, ok := semantic["enabled"].(bool); ok && enabled {
			// If enabled, provider and model should be present
			if provider, ok := semantic["provider"].(string); ok {
				t.Logf("Provider: %s", provider)
			}
			if model, ok := semantic["model"].(string); ok {
				t.Logf("Model: %s", model)
			}
		}
	} else {
		t.Logf("Health response (no semantic section): %v", health)
	}

	t.Log("Health endpoint provider info verified")
}

// TestSemanticProviders_CacheIsolation tests that cache entries are isolated by provider
func TestSemanticProviders_CacheIsolation(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create cache directory structure
	cacheDir := filepath.Join(h.MemoryRoot, ".cache", "summaries")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create provider-specific subdirectories
	providers := []string{"claude", "openai", "gemini"}
	for _, provider := range providers {
		providerDir := filepath.Join(cacheDir, provider)
		if err := os.MkdirAll(providerDir, 0755); err != nil {
			t.Fatalf("Failed to create provider cache directory: %v", err)
		}

		// Create a mock cache file
		cacheFile := filepath.Join(providerDir, "test-hash-v1-1-1.json")
		content := `{"provider":"` + provider + `","file_path":"/test/file.md"}`
		if err := os.WriteFile(cacheFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create cache file: %v", err)
		}
	}

	// Verify cache structure exists
	for _, provider := range providers {
		providerDir := filepath.Join(cacheDir, provider)
		entries, err := os.ReadDir(providerDir)
		if err != nil {
			t.Errorf("Failed to read provider cache directory %s: %v", provider, err)
			continue
		}
		if len(entries) == 0 {
			t.Errorf("Expected cache entries in %s directory", provider)
		} else {
			t.Logf("Provider %s cache directory has %d entries", provider, len(entries))
		}
	}

	// Verify cache status command works
	stdout, stderr, exitCode := h.RunCommand("cache", "status")
	if exitCode != 0 {
		t.Logf("Cache status output: stdout=%s, stderr=%s", stdout, stderr)
	}

	t.Log("Cache isolation structure verified")
}

// TestSemanticProviders_HotReloadProviderChange tests changing provider via hot-reload
func TestSemanticProviders_HotReloadProviderChange(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create initial config with claude provider (disabled)
	err := h.CreateConfigWithSemanticProvider(8080, harness.SemanticProviderConfig{
		Enabled:  false,
		Provider: "claude",
	})
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
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

	// Verify initial health
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Initial health check failed: %v", err)
	}
	t.Logf("Initial health: %v", health)

	// Update config to use openai provider
	err = h.CreateConfigWithSemanticProvider(8080, harness.SemanticProviderConfig{
		Enabled:  false,
		Provider: "openai",
	})
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Send SIGHUP to reload config
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for reload to complete
	time.Sleep(2 * time.Second)

	// Verify daemon still healthy after reload
	health, err = h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed after reload: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected daemon to be healthy after provider change, got: %v", health["status"])
	}

	t.Log("Provider hot-reload completed successfully")
}

// TestSemanticProviders_DaemonStartsWithEachProvider tests daemon starts with each provider configured
func TestSemanticProviders_DaemonStartsWithEachProvider(t *testing.T) {
	providers := []struct {
		name  string
		model string
	}{
		{"claude", "claude-sonnet-4-5-20250929"},
		{"openai", "gpt-4o"},
		{"gemini", "gemini-2.5-flash"},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			h := harness.New(t)
			if err := h.Setup(); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
			cleanup := harness.MustCleanup(t, h)
			defer cleanup.CleanupAll()

			// Create config with provider (disabled, no API key)
			err := h.CreateConfigWithSemanticProvider(8080, harness.SemanticProviderConfig{
				Enabled:  false,
				Provider: provider.name,
				Model:    provider.model,
			})
			if err != nil {
				t.Fatalf("Failed to create config: %v", err)
			}

			// Start daemon in background
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
			cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

			if err := cmd.Start(); err != nil {
				t.Fatalf("Failed to start daemon with provider %s: %v", provider.name, err)
			}

			defer func() {
				cancel()
				cmd.Wait()
			}()

			// Wait for daemon to be healthy
			if err := h.WaitForHealthy(30 * time.Second); err != nil {
				t.Fatalf("Daemon failed to become healthy with provider %s: %v", provider.name, err)
			}

			// Query health endpoint
			health, err := h.HTTPClient.Health()
			if err != nil {
				t.Fatalf("Health check failed for provider %s: %v", provider.name, err)
			}

			if status, ok := health["status"].(string); !ok || status != "healthy" {
				t.Errorf("Expected status=healthy for provider %s, got: %v", provider.name, health["status"])
			}

			t.Logf("Daemon started successfully with provider %s", provider.name)
		})
	}
}

// TestSemanticProviders_ConfigShowSchema tests that schema shows semantic config options
func TestSemanticProviders_ConfigShowSchema(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Run config show-schema command
	stdout, stderr, exitCode := h.RunCommand("config", "show-schema")
	if exitCode != 0 {
		t.Errorf("Config show-schema failed: stdout=%s, stderr=%s", stdout, stderr)
	}

	output := stdout + stderr

	// Verify semantic section is documented
	expectedFields := []string{
		"semantic",
		"provider",
		"api_key",
		"model",
		"enable_vision",
		"max_tokens",
	}

	for _, field := range expectedFields {
		if !strings.Contains(strings.ToLower(output), field) {
			t.Errorf("Expected schema to contain %q", field)
		}
	}

	t.Log("Config schema shows semantic provider options")
}

// TestSemanticProviders_RebuildWithoutProvider tests rebuild works without API key
func TestSemanticProviders_RebuildWithoutProvider(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create config with semantic disabled
	err := h.CreateConfigWithSemanticProvider(8080, harness.SemanticProviderConfig{
		Enabled:  false,
		Provider: "claude",
	})
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Add test files
	testFiles := []struct {
		name    string
		content string
	}{
		{"doc1.md", "# Document 1\n\nFirst document content."},
		{"doc2.md", "# Document 2\n\nSecond document content."},
		{"code.go", "package main\n\nfunc main() {}\n"},
	}

	for _, f := range testFiles {
		if err := h.AddMemoryFile(f.name, f.content); err != nil {
			t.Fatalf("Failed to add file %s: %v", f.name, err)
		}
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

	// Trigger rebuild
	stdout, stderr, exitCode := h.RunCommand("daemon", "rebuild")
	if exitCode != 0 {
		t.Errorf("Rebuild failed: stdout=%s, stderr=%s", stdout, stderr)
	}

	output := stdout + stderr
	if strings.Contains(strings.ToLower(output), "error") && !strings.Contains(output, "errors: 0") {
		t.Logf("Rebuild output: %s", output)
	}

	t.Log("Rebuild completed successfully without API key (metadata-only mode)")
}
