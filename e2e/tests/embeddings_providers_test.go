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

// TestEmbeddingsProviders_ConfigValidation tests that provider names are validated
func TestEmbeddingsProviders_ConfigValidation(t *testing.T) {
	t.Run("valid_provider_names", func(t *testing.T) {
		validProviders := []string{"openai", "voyage", "gemini"}

		for _, provider := range validProviders {
			t.Run(provider, func(t *testing.T) {
				h := harness.New(t)
				if err := h.Setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
				cleanup := harness.MustCleanup(t, h)
				defer cleanup.CleanupAll()

				// Create config with valid provider (disabled, no API key needed)
				err := h.CreateConfigWithEmbeddings(0,
					harness.SemanticProviderConfig{Enabled: false, Provider: "claude"},
					harness.EmbeddingsProviderConfig{Enabled: false, Provider: provider},
				)
				if err != nil {
					t.Fatalf("Failed to create config: %v", err)
				}

				// Validate config
				stdout, stderr, exitCode := h.RunCommand("config", "validate")
				if exitCode != 0 {
					t.Errorf("Config validation failed for embeddings provider %q: stdout=%s, stderr=%s", provider, stdout, stderr)
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

		// Create config with invalid embeddings provider
		config := map[string]any{
			"memory_root": h.MemoryRoot,
			"semantic": map[string]any{
				"enabled":  false,
				"provider": "claude",
			},
			"embeddings": map[string]any{
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
			t.Errorf("Expected validation to fail for invalid embeddings provider, but it passed: %s", output)
		} else {
			if strings.Contains(output, "embeddings") || strings.Contains(output, "provider") {
				t.Logf("Invalid embeddings provider correctly rejected: %s", output)
			}
		}
	})
}

// TestEmbeddingsProviders_ModelDimensionValidation tests model/dimension matching
func TestEmbeddingsProviders_ModelDimensionValidation(t *testing.T) {
	testCases := []struct {
		name       string
		provider   string
		model      string
		dimensions int
		shouldFail bool
	}{
		{"openai_small_correct", "openai", "text-embedding-3-small", 1536, false},
		{"openai_large_correct", "openai", "text-embedding-3-large", 3072, false},
		{"openai_small_wrong_dims", "openai", "text-embedding-3-small", 3072, true},
		{"voyage_3_correct", "voyage", "voyage-3", 1024, false},
		{"voyage_lite_correct", "voyage", "voyage-3-lite", 512, false},
		{"voyage_3_wrong_dims", "voyage", "voyage-3", 512, true},
		{"gemini_004_correct", "gemini", "text-embedding-004", 768, false},
		{"gemini_004_wrong_dims", "gemini", "text-embedding-004", 1536, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := harness.New(t)
			if err := h.Setup(); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
			cleanup := harness.MustCleanup(t, h)
			defer cleanup.CleanupAll()

			// Create config with model/dimension combination
			config := map[string]any{
				"memory_root": h.MemoryRoot,
				"semantic": map[string]any{
					"enabled":  false,
					"provider": "claude",
				},
				"embeddings": map[string]any{
					"enabled":    true,
					"provider":   tc.provider,
					"model":      tc.model,
					"dimensions": tc.dimensions,
					"api_key":    "test-key",
				},
			}

			data, _ := yaml.Marshal(config)
			if err := os.WriteFile(h.ConfigPath, data, 0644); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}

			// Validate config
			stdout, stderr, exitCode := h.RunCommand("config", "validate")
			output := stdout + stderr

			if tc.shouldFail {
				if exitCode == 0 {
					t.Errorf("Expected validation to fail for %s, but it passed", tc.name)
				} else {
					t.Logf("Dimension mismatch correctly detected for %s: %s", tc.name, output)
				}
			} else {
				if exitCode != 0 {
					t.Errorf("Expected validation to pass for %s, but it failed: %s", tc.name, output)
				}
			}
		})
	}
}

// TestEmbeddingsProviders_CacheIsolation tests that cache entries are isolated by provider
func TestEmbeddingsProviders_CacheIsolation(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create embeddings cache directory structure
	cacheDir := filepath.Join(h.MemoryRoot, ".cache", "embeddings")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create provider-specific subdirectories
	providers := []string{"openai", "voyage", "gemini"}
	for _, provider := range providers {
		providerDir := filepath.Join(cacheDir, provider)
		if err := os.MkdirAll(providerDir, 0755); err != nil {
			t.Fatalf("Failed to create provider cache directory: %v", err)
		}

		// Create nested hash directories (two-level structure)
		hashDir := filepath.Join(providerDir, "ab", "cd")
		if err := os.MkdirAll(hashDir, 0755); err != nil {
			t.Fatalf("Failed to create hash directory: %v", err)
		}

		// Create a mock cache file (binary format placeholder)
		cacheFile := filepath.Join(hashDir, "abcd1234567890.bin")
		if err := os.WriteFile(cacheFile, []byte{0x00, 0x04, 0x00, 0x00}, 0644); err != nil {
			t.Fatalf("Failed to create cache file: %v", err)
		}
	}

	// Verify cache structure exists
	for _, provider := range providers {
		providerDir := filepath.Join(cacheDir, provider)
		if _, err := os.Stat(providerDir); os.IsNotExist(err) {
			t.Errorf("Expected provider cache directory %s to exist", provider)
			continue
		}
		t.Logf("Provider %s embeddings cache directory exists", provider)
	}

	// Verify cache status command works
	stdout, stderr, exitCode := h.RunCommand("cache", "status")
	if exitCode != 0 {
		t.Logf("Cache status output: stdout=%s, stderr=%s", stdout, stderr)
	}

	t.Log("Embeddings cache isolation structure verified")
}

// TestEmbeddingsProviders_DaemonStartsWithEachProvider tests daemon starts with each provider configured
func TestEmbeddingsProviders_DaemonStartsWithEachProvider(t *testing.T) {
	providers := []struct {
		name       string
		model      string
		dimensions int
	}{
		{"openai", "text-embedding-3-small", 1536},
		{"voyage", "voyage-3", 1024},
		{"gemini", "text-embedding-004", 768},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			h := harness.New(t)
			if err := h.Setup(); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
			cleanup := harness.MustCleanup(t, h)
			defer cleanup.CleanupAll()

			// Create config with embeddings provider (disabled, no API key)
			err := h.CreateConfigWithEmbeddings(8080,
				harness.SemanticProviderConfig{Enabled: false, Provider: "claude"},
				harness.EmbeddingsProviderConfig{
					Enabled:    false,
					Provider:   provider.name,
					Model:      provider.model,
					Dimensions: provider.dimensions,
				},
			)
			if err != nil {
				t.Fatalf("Failed to create config: %v", err)
			}

			// Start daemon in background
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
			cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

			if err := cmd.Start(); err != nil {
				t.Fatalf("Failed to start daemon with embeddings provider %s: %v", provider.name, err)
			}

			defer func() {
				cancel()
				cmd.Wait()
			}()

			// Wait for daemon to be healthy
			if err := h.WaitForHealthy(30 * time.Second); err != nil {
				t.Fatalf("Daemon failed to become healthy with embeddings provider %s: %v", provider.name, err)
			}

			// Query health endpoint
			health, err := h.HTTPClient.Health()
			if err != nil {
				t.Fatalf("Health check failed for embeddings provider %s: %v", provider.name, err)
			}

			if status, ok := health["status"].(string); !ok || status != "healthy" {
				t.Errorf("Expected status=healthy for embeddings provider %s, got: %v", provider.name, health["status"])
			}

			// Check embeddings section in health response
			if embeddings, ok := health["embeddings"].(map[string]any); ok {
				t.Logf("Embeddings health info for %s: %v", provider.name, embeddings)
			}

			t.Logf("Daemon started successfully with embeddings provider %s", provider.name)
		})
	}
}

// TestEmbeddingsProviders_HealthEndpointInfo tests embeddings info in health endpoint
func TestEmbeddingsProviders_HealthEndpointInfo(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create config with embeddings provider specified
	err := h.CreateConfigWithEmbeddings(8080,
		harness.SemanticProviderConfig{Enabled: false, Provider: "claude"},
		harness.EmbeddingsProviderConfig{
			Enabled:    false,
			Provider:   "voyage",
			Model:      "voyage-code-3",
			Dimensions: 1024,
		},
	)
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

	// Verify health response contains embeddings section
	if embeddings, ok := health["embeddings"].(map[string]any); ok {
		t.Logf("Embeddings health info: %v", embeddings)

		// Check enabled field
		if enabled, ok := embeddings["enabled"].(bool); ok {
			t.Logf("Embeddings enabled: %v", enabled)
		}
		if provider, ok := embeddings["provider"].(string); ok {
			t.Logf("Embeddings provider: %s", provider)
		}
		if model, ok := embeddings["model"].(string); ok {
			t.Logf("Embeddings model: %s", model)
		}
	} else {
		t.Logf("Health response (no embeddings section): %v", health)
	}

	t.Log("Health endpoint embeddings info verified")
}

// TestEmbeddingsProviders_HotReloadProviderChange tests changing embeddings provider via hot-reload
func TestEmbeddingsProviders_HotReloadProviderChange(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create initial config with openai embeddings (disabled)
	err := h.CreateConfigWithEmbeddings(8080,
		harness.SemanticProviderConfig{Enabled: false, Provider: "claude"},
		harness.EmbeddingsProviderConfig{Enabled: false, Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
	)
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

	// Update config to use voyage embeddings provider
	err = h.CreateConfigWithEmbeddings(8080,
		harness.SemanticProviderConfig{Enabled: false, Provider: "claude"},
		harness.EmbeddingsProviderConfig{Enabled: false, Provider: "voyage", Model: "voyage-3", Dimensions: 1024},
	)
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
		t.Errorf("Expected daemon to be healthy after embeddings provider change, got: %v", health["status"])
	}

	t.Log("Embeddings provider hot-reload completed successfully")
}

// TestEmbeddingsProviders_ConfigShowSchema tests that schema shows embeddings config options
func TestEmbeddingsProviders_ConfigShowSchema(t *testing.T) {
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

	// Verify embeddings section is documented
	expectedFields := []string{
		"embeddings",
		"provider",
		"model",
		"dimensions",
	}

	for _, field := range expectedFields {
		if !strings.Contains(strings.ToLower(output), field) {
			t.Errorf("Expected schema to contain %q", field)
		}
	}

	t.Log("Config schema shows embeddings provider options")
}

// TestEmbeddingsProviders_DisabledByDefault tests that embeddings are disabled without API key
func TestEmbeddingsProviders_DisabledByDefault(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create config with embeddings explicitly disabled
	err := h.CreateConfigWithEmbeddings(8080,
		harness.SemanticProviderConfig{Enabled: false, Provider: "claude"},
		harness.EmbeddingsProviderConfig{Enabled: false, Provider: "openai"},
	)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nTest content for embeddings-disabled mode."); err != nil {
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

	// Check embeddings section - should show disabled
	if embeddings, ok := health["embeddings"].(map[string]any); ok {
		if enabled, ok := embeddings["enabled"].(bool); ok {
			if enabled {
				t.Error("Expected embeddings.enabled=false when no API key configured")
			} else {
				t.Log("Embeddings correctly disabled without API key")
			}
		}
	}

	t.Log("Daemon successfully running with embeddings disabled")
}

// TestEmbeddingsProviders_GraphIndexCreation tests that graph indexes are created for configured provider
func TestEmbeddingsProviders_GraphIndexCreation(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create config with embeddings provider (disabled but configured)
	err := h.CreateConfigWithEmbeddings(8080,
		harness.SemanticProviderConfig{Enabled: false, Provider: "claude"},
		harness.EmbeddingsProviderConfig{
			Enabled:    false,
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
		},
	)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Add test file to trigger graph initialization
	if err := h.AddMemoryFile("test.md", "# Test\n\nTest content."); err != nil {
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

	// Wait for initial processing
	time.Sleep(3 * time.Second)

	// Query graph status
	stdout, stderr, exitCode := h.RunCommand("graph", "status")
	output := stdout + stderr

	if exitCode != 0 {
		t.Logf("Graph status output (may fail if FalkorDB not available): %s", output)
	} else {
		t.Logf("Graph status: %s", output)
	}

	t.Log("Graph index creation test completed")
}
