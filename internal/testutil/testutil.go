// Package testutil provides testing utilities for isolated test environments.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

// TestEnv provides an isolated test environment with its own config directory.
type TestEnv struct {
	t         *testing.T
	ConfigDir string
}

// NewTestEnv creates an isolated test environment.
// It uses environment variables to override all paths, ensuring complete
// isolation even when tests run in parallel across packages.
// Cleanup is automatic via t.Cleanup.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	// Create temp directory for this test's config
	configDir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create test config dir: %v", err)
	}

	// Use t.Setenv for automatic cleanup - this is test-scoped
	// These env vars override viper settings via AutomaticEnv()
	t.Setenv("MEMORIZER_CONFIG_DIR", configDir)
	t.Setenv("MEMORIZER_DATABASE_REGISTRY_PATH", filepath.Join(configDir, "registry.db"))
	t.Setenv("MEMORIZER_LOG_FILE", filepath.Join(configDir, "memorizer.log"))
	t.Setenv("MEMORIZER_DAEMON_PID_FILE", filepath.Join(configDir, "daemon.pid"))
	t.Setenv("MEMORIZER_CACHE_BASE_DIR", filepath.Join(configDir, "cache"))

	// Reset and reinitialize config with new env vars
	config.Reset()
	if err := config.Init(); err != nil {
		t.Fatalf("failed to initialize test config: %v", err)
	}

	env := &TestEnv{
		t:         t,
		ConfigDir: configDir,
	}

	// Register cleanup to reset config state
	t.Cleanup(func() {
		config.Reset()
	})

	return env
}

// RegistryPath returns the path where the test registry database will be created.
func (e *TestEnv) RegistryPath() string {
	return filepath.Join(e.ConfigDir, "registry.db")
}

// CreateTestDir creates a test directory within the test environment's temp space.
// Returns the absolute path to the created directory.
func (e *TestEnv) CreateTestDir(name string) string {
	e.t.Helper()

	// Use a separate temp dir for test data (not inside config dir)
	testDataDir := filepath.Join(e.t.TempDir(), "testdata", name)
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		e.t.Fatalf("failed to create test dir %s: %v", name, err)
	}
	return testDataDir
}

// CreateTestFile creates a test file with the given content.
// Returns the absolute path to the created file.
func (e *TestEnv) CreateTestFile(dir, name, content string) string {
	e.t.Helper()

	filePath := filepath.Join(dir, name)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		e.t.Fatalf("failed to create test file %s: %v", filePath, err)
	}
	return filePath
}
