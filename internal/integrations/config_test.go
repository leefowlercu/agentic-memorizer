package integrations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/.config/test", filepath.Join(home, ".config/test")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		result := expandPath(tt.input)
		if result != tt.expected {
			t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestReadJSONConfig(t *testing.T) {
	t.Run("NonexistentFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")

		config, err := ReadJSONConfig(path)
		if err != nil {
			t.Fatalf("ReadJSONConfig failed: %v", err)
		}

		if config.Content == nil {
			t.Error("Content should not be nil")
		}

		if len(config.Content) != 0 {
			t.Errorf("Content should be empty for nonexistent file, got %d entries", len(config.Content))
		}
	})

	t.Run("ValidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.json")

		content := map[string]any{
			"key1": "value1",
			"key2": float64(42),
		}
		data, _ := json.Marshal(content)
		_ = os.WriteFile(path, data, 0644)

		config, err := ReadJSONConfig(path)
		if err != nil {
			t.Fatalf("ReadJSONConfig failed: %v", err)
		}

		if config.Content["key1"] != "value1" {
			t.Errorf("key1 = %v, want %v", config.Content["key1"], "value1")
		}

		if config.Content["key2"] != float64(42) {
			t.Errorf("key2 = %v, want %v", config.Content["key2"], float64(42))
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "invalid.json")

		_ = os.WriteFile(path, []byte("invalid json{"), 0644)

		_, err := ReadJSONConfig(path)
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("EmptyFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "empty.json")

		_ = os.WriteFile(path, []byte(""), 0644)

		config, err := ReadJSONConfig(path)
		if err != nil {
			t.Fatalf("ReadJSONConfig failed: %v", err)
		}

		if config.Content == nil {
			t.Error("Content should not be nil for empty file")
		}
	})
}

func TestWriteJSONConfig(t *testing.T) {
	t.Run("WriteNewFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "new.json")

		content := map[string]any{
			"key": "value",
		}

		err := WriteJSONConfig(path, content)
		if err != nil {
			t.Fatalf("WriteJSONConfig failed: %v", err)
		}

		// Verify file was created
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read written file: %v", err)
		}

		var read map[string]any
		if err := json.Unmarshal(data, &read); err != nil {
			t.Fatalf("Written file is not valid JSON: %v", err)
		}

		if read["key"] != "value" {
			t.Errorf("key = %v, want %v", read["key"], "value")
		}
	})

	t.Run("CreateParentDirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nested", "dir", "config.json")

		content := map[string]any{"test": true}

		err := WriteJSONConfig(path, content)
		if err != nil {
			t.Fatalf("WriteJSONConfig failed: %v", err)
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("File was not created")
		}
	})
}

func TestBackupConfig(t *testing.T) {
	t.Run("BackupExisting", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.json")

		// Create original file
		_ = os.WriteFile(path, []byte(`{"original": true}`), 0644)

		backupPath, err := BackupConfig(path)
		if err != nil {
			t.Fatalf("BackupConfig failed: %v", err)
		}

		if backupPath == "" {
			t.Fatal("Backup path should not be empty")
		}

		// Verify backup exists
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("Backup file was not created")
		}

		// Verify backup content
		backupData, _ := os.ReadFile(backupPath)
		if string(backupData) != `{"original": true}` {
			t.Errorf("Backup content = %q, want %q", string(backupData), `{"original": true}`)
		}
	})

	t.Run("BackupNonexistent", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")

		backupPath, err := BackupConfig(path)
		if err != nil {
			t.Fatalf("BackupConfig failed: %v", err)
		}

		if backupPath != "" {
			t.Error("Backup path should be empty for nonexistent file")
		}
	})
}

func TestRestoreBackup(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := filepath.Join(tmpDir, "config.json")
	backupPath := filepath.Join(tmpDir, "config.backup.json")

	// Create backup
	_ = os.WriteFile(backupPath, []byte(`{"backup": true}`), 0644)

	// Create modified original
	_ = os.WriteFile(originalPath, []byte(`{"modified": true}`), 0644)

	// Restore
	err := RestoreBackup(backupPath, originalPath)
	if err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	// Verify restored content
	data, _ := os.ReadFile(originalPath)
	if string(data) != `{"backup": true}` {
		t.Errorf("Restored content = %q, want %q", string(data), `{"backup": true}`)
	}
}

func TestConfigExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Existing file
	existingPath := filepath.Join(tmpDir, "exists.json")
	_ = os.WriteFile(existingPath, []byte("{}"), 0644)

	if !ConfigExists(existingPath) {
		t.Error("ConfigExists should return true for existing file")
	}

	// Nonexistent file
	nonexistentPath := filepath.Join(tmpDir, "nonexistent.json")
	if ConfigExists(nonexistentPath) {
		t.Error("ConfigExists should return false for nonexistent file")
	}
}
