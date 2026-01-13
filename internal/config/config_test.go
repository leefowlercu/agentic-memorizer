package config

import (
	"os"
	"path/filepath"
	"testing"
)

// T007: TestInit_NoConfigFile_UsesDefaults
func TestInit_NoConfigFile_UsesDefaults(t *testing.T) {
	// Setup: Use empty temp directory with no config file
	// Also override HOME to prevent finding user's real config
	tmpDir := t.TempDir()
	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)

	// Change to temp dir to prevent finding config in original working dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(origDir) })

	// Clear any cached viper state
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error when no config file exists: %v", err)
	}

	// Verify config file path is empty (no file loaded)
	if path := ConfigFilePath(); path != "" {
		t.Errorf("ConfigFilePath() = %q, want empty string when no config file", path)
	}
}

// T008: TestInit_ConfigInEnvDir_LoadsFromEnvDir
func TestInit_ConfigInEnvDir_LoadsFromEnvDir(t *testing.T) {
	// Setup: Create config in env-specified directory
	envDir := t.TempDir()
	configPath := filepath.Join(envDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 9999\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", envDir)
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify config was loaded from env directory
	loadedPath := ConfigFilePath()
	if loadedPath != configPath {
		t.Errorf("ConfigFilePath() = %q, want %q", loadedPath, configPath)
	}
}

// T009: TestInit_ConfigInDefaultDir_LoadsFromDefaultDir
func TestInit_ConfigInDefaultDir_LoadsFromDefaultDir(t *testing.T) {
	// Setup: Create temp directory to simulate ~/.config/memorizer/
	// We override HOME to control the default path
	tmpHome := t.TempDir()
	defaultDir := filepath.Join(tmpHome, ".config", "memorizer")
	if err := os.MkdirAll(defaultDir, 0755); err != nil {
		t.Fatalf("failed to create default dir: %v", err)
	}

	configPath := filepath.Join(defaultDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 8888\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Unset MEMORIZER_CONFIG_DIR so default path is used
	t.Setenv("MEMORIZER_CONFIG_DIR", "")
	t.Setenv("HOME", tmpHome)
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify config was loaded from default directory
	loadedPath := ConfigFilePath()
	if loadedPath != configPath {
		t.Errorf("ConfigFilePath() = %q, want %q", loadedPath, configPath)
	}
}

// T010: TestInit_ConfigInCurrentDir_LoadsFromCurrentDir
func TestInit_ConfigInCurrentDir_LoadsFromCurrentDir(t *testing.T) {
	// Setup: Create config in a temp directory and change to it
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 7777\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Change to temp directory (current dir fallback)
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Unset env var and use non-existent HOME so only current dir matches
	t.Setenv("MEMORIZER_CONFIG_DIR", "")
	t.Setenv("HOME", "/nonexistent")
	Reset()

	err = Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify config was loaded from current directory
	// Resolve symlinks for comparison (macOS /var -> /private/var)
	loadedPath := ConfigFilePath()
	expectedPath, _ := filepath.EvalSymlinks(configPath)
	actualPath, _ := filepath.EvalSymlinks(loadedPath)
	if actualPath != expectedPath {
		t.Errorf("ConfigFilePath() = %q, want %q", loadedPath, configPath)
	}
}

// T011: TestInit_InvalidYAML_ReturnsFatalError
func TestInit_InvalidYAML_ReturnsFatalError(t *testing.T) {
	// Setup: Create config with invalid YAML
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	invalidYAML := "daemon:\n  port: [invalid yaml"
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	err := Init()
	if err == nil {
		t.Fatal("Init() should return error for invalid YAML, got nil")
	}
}

// T012: TestInit_UnreadableFile_ReturnsFatalError
func TestInit_UnreadableFile_ReturnsFatalError(t *testing.T) {
	// Skip on CI or if running as root (root can read anything)
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	// Setup: Create config with no read permissions
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 1234\n"), 0000); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	defer func() { _ = os.Chmod(configPath, 0644) }() // Restore permissions for cleanup

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	err := Init()
	if err == nil {
		t.Fatal("Init() should return error for unreadable file, got nil")
	}
}

// T013: TestInit_MultipleLocations_UsesFirstMatch
func TestInit_MultipleLocations_UsesFirstMatch(t *testing.T) {
	// Setup: Create config in both env dir and current dir
	envDir := t.TempDir()
	envConfigPath := filepath.Join(envDir, "config.yaml")
	if err := os.WriteFile(envConfigPath, []byte("daemon:\n  port: 1111\n"), 0644); err != nil {
		t.Fatalf("failed to write env config file: %v", err)
	}

	currentDir := t.TempDir()
	currentConfigPath := filepath.Join(currentDir, "config.yaml")
	if err := os.WriteFile(currentConfigPath, []byte("daemon:\n  port: 2222\n"), 0644); err != nil {
		t.Fatalf("failed to write current dir config file: %v", err)
	}

	// Change to current dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(currentDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Set env var - should take priority over current dir
	t.Setenv("MEMORIZER_CONFIG_DIR", envDir)
	t.Setenv("HOME", "/nonexistent")
	Reset()

	err = Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify env dir config was loaded (first match)
	loadedPath := ConfigFilePath()
	if loadedPath != envConfigPath {
		t.Errorf("ConfigFilePath() = %q, want %q (env dir should take priority)", loadedPath, envConfigPath)
	}
}

// =============================================================================
// User Story 2: Environment Variable Override Tests
// =============================================================================

// T023: TestEnvOverride_SimpleKey_OverridesFileValue
func TestEnvOverride_SimpleKey_OverridesFileValue(t *testing.T) {
	// Setup: Create config file with a value
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	// Set env var to override the file value
	t.Setenv("MEMORIZER_DAEMON_HTTP_PORT", "9999")
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify env var overrides file value
	cfg := Get()
	if cfg.Daemon.HTTPPort != 9999 {
		t.Errorf("Get().Daemon.HTTPPort = %d, want 9999 (env override)", cfg.Daemon.HTTPPort)
	}
}

// T024: TestEnvOverride_NestedKey_MapsCorrectly
func TestEnvOverride_NestedKey_MapsCorrectly(t *testing.T) {
	// Setup: Create config file with nested value
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  metrics:\n    collection_interval: 30\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	// Nested key: daemon.metrics.collection_interval -> MEMORIZER_DAEMON_METRICS_COLLECTION_INTERVAL
	t.Setenv("MEMORIZER_DAEMON_METRICS_COLLECTION_INTERVAL", "120")
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify nested env var overrides file value
	cfg := Get()
	if cfg.Daemon.Metrics.CollectionInterval != 120 {
		t.Errorf("Get().Daemon.Metrics.CollectionInterval = %d, want 120 (env override)", cfg.Daemon.Metrics.CollectionInterval)
	}
}

// T025: TestEnvOverride_NoFileValue_UsesEnvValue
func TestEnvOverride_NoFileValue_UsesEnvValue(t *testing.T) {
	// Setup: Empty config directory (no file)
	// Isolate from user's real config
	tmpDir := t.TempDir()
	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(origDir) })

	// Set env var with no corresponding file value
	t.Setenv("MEMORIZER_DAEMON_HTTP_BIND", "0.0.0.0")
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify env var is used when no file value exists
	cfg := Get()
	if cfg.Daemon.HTTPBind != "0.0.0.0" {
		t.Errorf("Get().Daemon.HTTPBind = %q, want 0.0.0.0 (env value)", cfg.Daemon.HTTPBind)
	}
}

// =============================================================================
// User Story 3: Typed Config Accessor Tests
// =============================================================================

// TestGet_ReturnsTypedConfig verifies the typed accessor returns valid config
func TestGet_ReturnsTypedConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `daemon:
  http_port: 8080
  http_bind: 127.0.0.1
graph:
  host: localhost
  port: 6379
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() returned nil")
	}

	if cfg.Daemon.HTTPPort != 8080 {
		t.Errorf("Get().Daemon.HTTPPort = %d, want 8080", cfg.Daemon.HTTPPort)
	}
	if cfg.Daemon.HTTPBind != "127.0.0.1" {
		t.Errorf("Get().Daemon.HTTPBind = %q, want 127.0.0.1", cfg.Daemon.HTTPBind)
	}
	if cfg.Graph.Host != "localhost" {
		t.Errorf("Get().Graph.Host = %q, want localhost", cfg.Graph.Host)
	}
	if cfg.Graph.Port != 6379 {
		t.Errorf("Get().Graph.Port = %d, want 6379", cfg.Graph.Port)
	}
}

// TestGet_BeforeInit_ReturnsNil verifies Get() returns nil before Init()
func TestGet_BeforeInit_ReturnsNil(t *testing.T) {
	Reset()
	if cfg := Get(); cfg != nil {
		t.Errorf("Get() before Init() = %v, want nil", cfg)
	}
}

// TestMustGet_BeforeInit_Panics verifies MustGet() panics before Init()
func TestMustGet_BeforeInit_Panics(t *testing.T) {
	Reset()
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGet() before Init() should panic")
		}
	}()
	_ = MustGet()
}

// =============================================================================
// User Story 4: Reload Configuration Tests
// =============================================================================

// T042: TestReload_ValidConfig_UpdatesValues
func TestReload_ValidConfig_UpdatesValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial config
	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify initial value
	cfg := Get()
	if cfg.Daemon.HTTPPort != 8080 {
		t.Errorf("Get().Daemon.HTTPPort = %d, want 8080", cfg.Daemon.HTTPPort)
	}

	// Update config file
	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 9999\n"), 0644); err != nil {
		t.Fatalf("failed to update config file: %v", err)
	}

	// Reload config
	if err := Reload(); err != nil {
		t.Fatalf("Reload() returned error: %v", err)
	}

	// Verify updated value
	cfg = Get()
	if cfg.Daemon.HTTPPort != 9999 {
		t.Errorf("Get().Daemon.HTTPPort = %d after reload, want 9999", cfg.Daemon.HTTPPort)
	}
}

// T043: TestReload_InvalidConfig_RetainsPreviousValues
func TestReload_InvalidConfig_RetainsPreviousValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial valid config
	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify initial value
	cfg := Get()
	if cfg.Daemon.HTTPPort != 8080 {
		t.Errorf("Get().Daemon.HTTPPort = %d, want 8080", cfg.Daemon.HTTPPort)
	}

	// Corrupt config file with invalid YAML
	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: [invalid yaml"), 0644); err != nil {
		t.Fatalf("failed to corrupt config file: %v", err)
	}

	// Reload should fail but retain previous values
	err := Reload()
	if err == nil {
		t.Error("Reload() should return error for invalid YAML")
	}

	// Verify previous value is retained
	cfg = Get()
	if cfg.Daemon.HTTPPort != 8080 {
		t.Errorf("Get().Daemon.HTTPPort = %d after failed reload, want 8080 (retained)", cfg.Daemon.HTTPPort)
	}
}

// T044: TestReload_UnreadableConfig_RetainsPreviousValues
func TestReload_UnreadableConfig_RetainsPreviousValues(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial valid config
	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify initial value
	cfg := Get()
	if cfg.Daemon.HTTPPort != 8080 {
		t.Errorf("Get().Daemon.HTTPPort = %d, want 8080", cfg.Daemon.HTTPPort)
	}

	// Make config unreadable
	if err := os.Chmod(configPath, 0000); err != nil {
		t.Fatalf("failed to chmod config file: %v", err)
	}
	defer func() { _ = os.Chmod(configPath, 0644) }()

	// Reload should fail but retain previous values
	err := Reload()
	if err == nil {
		t.Error("Reload() should return error for unreadable file")
	}

	// Verify previous value is retained
	cfg = Get()
	if cfg.Daemon.HTTPPort != 8080 {
		t.Errorf("Get().Daemon.HTTPPort = %d after failed reload, want 8080 (retained)", cfg.Daemon.HTTPPort)
	}
}

// =============================================================================
// Path Expansion Tests
// =============================================================================

func TestExpandHome(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME environment variable not set")
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"no tilde", "/absolute/path", "/absolute/path"},
		{"relative path", "relative/path", "relative/path"},
		{"tilde only", "~", home},
		{"tilde with slash", "~/config", filepath.Join(home, "config")},
		{"tilde with nested path", "~/.config/memorizer", filepath.Join(home, ".config/memorizer")},
		{"tilde not at start", "/path/to/~", "/path/to/~"},
		{"tilde without slash", "~invalid", "~invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandHome(tt.input)
			if got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandHome_NoHome(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", origHome) }()

	_ = os.Unsetenv("HOME")

	// With HOME unset, tilde paths should remain unchanged
	input := "~/.config/memorizer"
	got := expandHome(input)
	if got != input {
		t.Errorf("expandHome(%q) with no HOME = %q, want %q (unchanged)", input, got, input)
	}
}

func TestExpandPath_ExpandsTilde(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME environment variable not set")
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"tilde path", "~/.config/memorizer/app.log", filepath.Join(home, ".config/memorizer/app.log")},
		{"absolute path", "/var/log/memorizer.log", "/var/log/memorizer.log"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandPath(tt.input)
			if got != tt.want {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandPath_WithTypedConfig(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME environment variable not set")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("log_file: ~/.config/memorizer/app.log\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	cfg := Get()
	// Use ExpandPath to expand tilde from typed config field
	got := ExpandPath(cfg.LogFile)
	want := filepath.Join(home, ".config/memorizer/app.log")
	if got != want {
		t.Errorf("ExpandPath(cfg.LogFile) = %q, want %q", got, want)
	}
}
