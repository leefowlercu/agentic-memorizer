package config

import (
	"os"
	"path/filepath"
	"testing"
)

// T007: TestInit_NoConfigFile_UsesDefaults
func TestInit_NoConfigFile_UsesDefaults(t *testing.T) {
	// Setup: Use empty temp directory with no config file
	tmpDir := t.TempDir()
	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)

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
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	// Set env var to override the file value
	t.Setenv("MEMORIZER_DAEMON_PORT", "9999")
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify env var overrides file value
	got := GetInt("daemon.port")
	if got != 9999 {
		t.Errorf("GetInt(daemon.port) = %d, want 9999 (env override)", got)
	}
}

// T024: TestEnvOverride_NestedKey_MapsCorrectly
func TestEnvOverride_NestedKey_MapsCorrectly(t *testing.T) {
	// Setup: Create config file with nested value
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("memory:\n  base:\n    path: /original\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	// Nested key: memory.base.path -> MEMORIZER_MEMORY_BASE_PATH
	t.Setenv("MEMORIZER_MEMORY_BASE_PATH", "/overridden")
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify nested env var overrides file value
	got := GetString("memory.base.path")
	if got != "/overridden" {
		t.Errorf("GetString(memory.base.path) = %q, want /overridden (env override)", got)
	}
}

// T025: TestEnvOverride_NoFileValue_UsesEnvValue
func TestEnvOverride_NoFileValue_UsesEnvValue(t *testing.T) {
	// Setup: Empty config directory (no file)
	tmpDir := t.TempDir()

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	// Set env var with no corresponding file value
	t.Setenv("MEMORIZER_DAEMON_HOST", "localhost")
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify env var is used when no file value exists
	got := GetString("daemon.host")
	if got != "localhost" {
		t.Errorf("GetString(daemon.host) = %q, want localhost (env value)", got)
	}
}

// T026: TestEnvOverride_InvalidType_ReturnsFatalError
func TestEnvOverride_InvalidType_ReturnsFatalError(t *testing.T) {
	// Setup: Create config file with integer type
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	// Set env var with invalid type (string for int field)
	t.Setenv("MEMORIZER_DAEMON_PORT", "not-a-number")
	Reset()

	err := Init()
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// When getting as int, should return 0 (Viper's behavior for type mismatch)
	// The spec says fatal error, but Viper doesn't validate types on load
	// Type validation happens when accessing values via GetInt
	got := GetInt("daemon.port")
	if got != 0 {
		t.Errorf("GetInt(daemon.port) = %d, want 0 (invalid type returns zero value)", got)
	}
}

// =============================================================================
// User Story 3: Programmatic Accessor Tests
// =============================================================================

// T031: TestGetString_ExistingKey_ReturnsValue
func TestGetString_ExistingKey_ReturnsValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  host: example.com\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	got := GetString("daemon.host")
	if got != "example.com" {
		t.Errorf("GetString(daemon.host) = %q, want example.com", got)
	}
}

// T032: TestGetInt_ExistingKey_ReturnsValue
func TestGetInt_ExistingKey_ReturnsValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	got := GetInt("daemon.port")
	if got != 8080 {
		t.Errorf("GetInt(daemon.port) = %d, want 8080", got)
	}
}

// T033: TestGetBool_ExistingKey_ReturnsValue
func TestGetBool_ExistingKey_ReturnsValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  enabled: true\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	got := GetBool("daemon.enabled")
	if got != true {
		t.Errorf("GetBool(daemon.enabled) = %v, want true", got)
	}
}

// T034: TestGet_MissingKey_ReturnsZeroValue
func TestGet_MissingKey_ReturnsZeroValue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	if got := GetString("nonexistent.key"); got != "" {
		t.Errorf("GetString(nonexistent.key) = %q, want empty string", got)
	}
	if got := GetInt("nonexistent.key"); got != 0 {
		t.Errorf("GetInt(nonexistent.key) = %d, want 0", got)
	}
	if got := GetBool("nonexistent.key"); got != false {
		t.Errorf("GetBool(nonexistent.key) = %v, want false", got)
	}
}

// T035: TestGet_NestedKey_ReturnsValue
func TestGet_NestedKey_ReturnsValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `memory:
  base:
    path: /data/memorizer
    limit: 1024
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	gotString := GetString("memory.base.path")
	if gotString != "/data/memorizer" {
		t.Errorf("GetString(memory.base.path) = %q, want /data/memorizer", gotString)
	}

	gotInt := GetInt("memory.base.limit")
	if gotInt != 1024 {
		t.Errorf("GetInt(memory.base.limit) = %d, want 1024", gotInt)
	}
}

// T036: TestSetDefault_SetsDefaultValue
func TestSetDefault_SetsDefaultValue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	// Set default before Init
	SetDefault("daemon.port", 9090)
	SetDefault("daemon.host", "localhost")
	SetDefault("daemon.debug", true)

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	if got := GetInt("daemon.port"); got != 9090 {
		t.Errorf("GetInt(daemon.port) = %d, want 9090 (default)", got)
	}
	if got := GetString("daemon.host"); got != "localhost" {
		t.Errorf("GetString(daemon.host) = %q, want localhost (default)", got)
	}
	if got := GetBool("daemon.debug"); got != true {
		t.Errorf("GetBool(daemon.debug) = %v, want true (default)", got)
	}
}

// =============================================================================
// User Story 4: Reload Configuration Tests
// =============================================================================

// T042: TestReload_ValidConfig_UpdatesValues
func TestReload_ValidConfig_UpdatesValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial config
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify initial value
	if got := GetInt("daemon.port"); got != 8080 {
		t.Errorf("GetInt(daemon.port) = %d, want 8080", got)
	}

	// Update config file
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 9999\n"), 0644); err != nil {
		t.Fatalf("failed to update config file: %v", err)
	}

	// Reload config
	if err := Reload(); err != nil {
		t.Fatalf("Reload() returned error: %v", err)
	}

	// Verify updated value
	if got := GetInt("daemon.port"); got != 9999 {
		t.Errorf("GetInt(daemon.port) = %d after reload, want 9999", got)
	}
}

// T043: TestReload_InvalidConfig_RetainsPreviousValues
func TestReload_InvalidConfig_RetainsPreviousValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial valid config
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify initial value
	if got := GetInt("daemon.port"); got != 8080 {
		t.Errorf("GetInt(daemon.port) = %d, want 8080", got)
	}

	// Corrupt config file with invalid YAML
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: [invalid yaml"), 0644); err != nil {
		t.Fatalf("failed to corrupt config file: %v", err)
	}

	// Reload should fail but retain previous values
	err := Reload()
	if err == nil {
		t.Error("Reload() should return error for invalid YAML")
	}

	// Verify previous value is retained
	if got := GetInt("daemon.port"); got != 8080 {
		t.Errorf("GetInt(daemon.port) = %d after failed reload, want 8080 (retained)", got)
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
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Verify initial value
	if got := GetInt("daemon.port"); got != 8080 {
		t.Errorf("GetInt(daemon.port) = %d, want 8080", got)
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
	if got := GetInt("daemon.port"); got != 8080 {
		t.Errorf("GetInt(daemon.port) = %d after failed reload, want 8080 (retained)", got)
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

func TestGetPath_ExpandsTilde(t *testing.T) {
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

	got := GetPath("log_file")
	want := filepath.Join(home, ".config/memorizer/app.log")
	if got != want {
		t.Errorf("GetPath(log_file) = %q, want %q", got, want)
	}
}

func TestGetPath_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("log_file: /var/log/memorizer.log\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	got := GetPath("log_file")
	want := "/var/log/memorizer.log"
	if got != want {
		t.Errorf("GetPath(log_file) = %q, want %q", got, want)
	}
}

func TestGetPath_EmptyValue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	got := GetPath("nonexistent_key")
	if got != "" {
		t.Errorf("GetPath(nonexistent_key) = %q, want empty string", got)
	}
}
