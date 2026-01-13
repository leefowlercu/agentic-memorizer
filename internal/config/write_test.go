package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewDefaultConfig()
	err := Write(&cfg, configPath)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Write() did not create config file")
	}
}

func TestWrite_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "nested", "config.yaml")

	cfg := NewDefaultConfig()
	err := Write(&cfg, configPath)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Verify directory was created
	dir := filepath.Dir(configPath)
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		t.Error("Write() did not create directory")
	}
	if !info.IsDir() {
		t.Error("Write() directory is not a directory")
	}
}

func TestWrite_DirectoryPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "newdir", "config.yaml")

	cfg := NewDefaultConfig()
	err := Write(&cfg, configPath)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	dir := filepath.Dir(configPath)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("failed to stat directory; %v", err)
	}

	// Check permissions (0700)
	perms := info.Mode().Perm()
	if perms != 0700 {
		t.Errorf("directory permissions = %o, want 0700", perms)
	}
}

func TestWrite_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewDefaultConfig()
	err := Write(&cfg, configPath)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat file; %v", err)
	}

	// Check permissions (0600)
	perms := info.Mode().Perm()
	if perms != 0600 {
		t.Errorf("file permissions = %o, want 0600", perms)
	}
}

func TestWrite_IncludesHeaderComment(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewDefaultConfig()
	err := Write(&cfg, configPath)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file; %v", err)
	}

	// Verify header comment
	if !strings.HasPrefix(string(content), "# Memorizer configuration") {
		t.Error("Write() did not include header comment")
	}
	if !strings.Contains(string(content), "Generated:") {
		t.Error("Write() did not include generation timestamp")
	}
}

func TestWrite_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewDefaultConfig()
	err := Write(&cfg, configPath)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Try to load it back
	loaded, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v; written YAML is not valid", err)
	}

	// Verify some values round-tripped correctly
	if loaded.LogLevel != cfg.LogLevel {
		t.Errorf("LogLevel = %q, want %q", loaded.LogLevel, cfg.LogLevel)
	}
	if loaded.Daemon.HTTPPort != cfg.Daemon.HTTPPort {
		t.Errorf("Daemon.HTTPPort = %d, want %d", loaded.Daemon.HTTPPort, cfg.Daemon.HTTPPort)
	}
	if loaded.Semantic.Provider != cfg.Semantic.Provider {
		t.Errorf("Semantic.Provider = %q, want %q", loaded.Semantic.Provider, cfg.Semantic.Provider)
	}
}

func TestWrite_ExpandsTilde(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := NewDefaultConfig()
	err := Write(&cfg, "~/config.yaml")
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "config.yaml")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Write() did not expand tilde; file not at %s", expectedPath)
	}
}

func TestConfigExists_FileExists_ReturnsTrue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".config", "memorizer")
	os.MkdirAll(configDir, 0700)
	configPath := filepath.Join(configDir, "config.yaml")
	os.WriteFile(configPath, []byte("log_level: info\n"), 0600)

	if !ConfigExists() {
		t.Error("ConfigExists() = false, want true when file exists")
	}
}

func TestConfigExists_FileNotExists_ReturnsFalse(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if ConfigExists() {
		t.Error("ConfigExists() = true, want false when file doesn't exist")
	}
}

func TestConfigExistsAt_FileExists_ReturnsTrue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte("log_level: info\n"), 0600)

	if !ConfigExistsAt(configPath) {
		t.Error("ConfigExistsAt() = false, want true when file exists")
	}
}

func TestConfigExistsAt_FileNotExists_ReturnsFalse(t *testing.T) {
	if ConfigExistsAt("/nonexistent/path/config.yaml") {
		t.Error("ConfigExistsAt() = true, want false when file doesn't exist")
	}
}

func TestDefaultConfigPath_ReturnsExpectedPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	expected := filepath.Join(tmpDir, ".config", "memorizer", "config.yaml")
	got := DefaultConfigPath()
	if got != expected {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, expected)
	}
}

func TestConfigDir_ReturnsExpectedPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	expected := filepath.Join(tmpDir, ".config", "memorizer")
	got := ConfigDir()
	if got != expected {
		t.Errorf("ConfigDir() = %q, want %q", got, expected)
	}
}
