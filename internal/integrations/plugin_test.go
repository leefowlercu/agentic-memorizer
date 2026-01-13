package integrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginIntegration(t *testing.T) {
	t.Run("NewPluginIntegration", func(t *testing.T) {
		files := []PluginFile{
			{TargetName: "init.lua", Content: []byte("return {}"), Mode: 0644},
		}

		integration := NewPluginIntegration(
			"test-plugin",
			"test-harness",
			"Test plugin integration",
			"testbin",
			"~/.test/plugins/memorizer",
			files,
		)

		if integration.Name() != "test-plugin" {
			t.Errorf("Name() = %q, want %q", integration.Name(), "test-plugin")
		}
		if integration.Harness() != "test-harness" {
			t.Errorf("Harness() = %q, want %q", integration.Harness(), "test-harness")
		}
		if integration.Type() != IntegrationTypePlugin {
			t.Errorf("Type() = %q, want %q", integration.Type(), IntegrationTypePlugin)
		}
		if integration.Description() != "Test plugin integration" {
			t.Errorf("Description() = %q, want %q", integration.Description(), "Test plugin integration")
		}
	})

	t.Run("SetupAndTeardown", func(t *testing.T) {
		tmpDir := t.TempDir()
		pluginDir := filepath.Join(tmpDir, "plugins", "memorizer")

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		files := []PluginFile{
			{TargetName: "init.lua", Content: []byte("-- Memorizer plugin\nreturn {}"), Mode: 0644},
			{TargetName: "utils.lua", Content: []byte("-- Utils\nreturn {}"), Mode: 0644},
		}

		integration := NewPluginIntegration(
			"test-plugin",
			"test-harness",
			"Test",
			"testbin",
			pluginDir,
			files,
		)

		ctx := context.Background()

		// Setup
		err := integration.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify files installed
		initPath := filepath.Join(pluginDir, "init.lua")
		data, err := os.ReadFile(initPath)
		if err != nil {
			t.Fatalf("init.lua not found: %v", err)
		}
		if string(data) != "-- Memorizer plugin\nreturn {}" {
			t.Errorf("init.lua content = %q, want %q", string(data), "-- Memorizer plugin\nreturn {}")
		}

		utilsPath := filepath.Join(pluginDir, "utils.lua")
		if _, err := os.Stat(utilsPath); os.IsNotExist(err) {
			t.Error("utils.lua not found")
		}

		// IsInstalled should return true
		installed, err := integration.IsInstalled()
		if err != nil {
			t.Fatalf("IsInstalled failed: %v", err)
		}
		if !installed {
			t.Error("IsInstalled should return true after setup")
		}

		// Teardown
		err = integration.Teardown(ctx)
		if err != nil {
			t.Fatalf("Teardown failed: %v", err)
		}

		// Verify files removed
		if _, err := os.Stat(initPath); !os.IsNotExist(err) {
			t.Error("init.lua should be removed after teardown")
		}
		if _, err := os.Stat(utilsPath); !os.IsNotExist(err) {
			t.Error("utils.lua should be removed after teardown")
		}

		// IsInstalled should return false
		installed, err = integration.IsInstalled()
		if err != nil {
			t.Fatalf("IsInstalled failed: %v", err)
		}
		if installed {
			t.Error("IsInstalled should return false after teardown")
		}
	})

	t.Run("SetupBackupsExistingFiles", func(t *testing.T) {
		tmpDir := t.TempDir()
		pluginDir := filepath.Join(tmpDir, "plugins", "memorizer")
		_ = os.MkdirAll(pluginDir, 0755)

		// Create existing file
		existingPath := filepath.Join(pluginDir, "init.lua")
		_ = os.WriteFile(existingPath, []byte("-- Existing content"), 0644)

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		files := []PluginFile{
			{TargetName: "init.lua", Content: []byte("-- New content"), Mode: 0644},
		}

		integration := NewPluginIntegration(
			"test-plugin",
			"test",
			"Test",
			"testbin",
			pluginDir,
			files,
		)

		ctx := context.Background()
		err := integration.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify backup exists
		backupPath := existingPath + ".backup"
		backupData, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Backup not created: %v", err)
		}
		if string(backupData) != "-- Existing content" {
			t.Errorf("Backup content = %q, want %q", string(backupData), "-- Existing content")
		}

		// Verify new content installed
		newData, _ := os.ReadFile(existingPath)
		if string(newData) != "-- New content" {
			t.Errorf("New content = %q, want %q", string(newData), "-- New content")
		}
	})

	t.Run("TeardownRestoresBackup", func(t *testing.T) {
		tmpDir := t.TempDir()
		pluginDir := filepath.Join(tmpDir, "plugins", "memorizer")
		_ = os.MkdirAll(pluginDir, 0755)

		// Create current and backup files
		currentPath := filepath.Join(pluginDir, "init.lua")
		backupPath := currentPath + ".backup"
		_ = os.WriteFile(currentPath, []byte("-- Current"), 0644)
		_ = os.WriteFile(backupPath, []byte("-- Backup original"), 0644)

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		files := []PluginFile{
			{TargetName: "init.lua", Content: []byte("content"), Mode: 0644},
		}

		integration := NewPluginIntegration(
			"test-plugin",
			"test",
			"Test",
			"testbin",
			pluginDir,
			files,
		)

		ctx := context.Background()
		err := integration.Teardown(ctx)
		if err != nil {
			t.Fatalf("Teardown failed: %v", err)
		}

		// Verify backup was restored
		data, err := os.ReadFile(currentPath)
		if err != nil {
			t.Fatalf("File not restored: %v", err)
		}
		if string(data) != "-- Backup original" {
			t.Errorf("Restored content = %q, want %q", string(data), "-- Backup original")
		}
	})

	t.Run("ValidateMissingBinary", func(t *testing.T) {
		integration := NewPluginIntegration(
			"test",
			"test",
			"Test",
			"nonexistent-binary-12345",
			"/tmp/plugins/memorizer",
			nil,
		)

		err := integration.Validate()
		if err == nil {
			t.Error("Validate should fail for missing binary")
		}
	})

	t.Run("StatusMissingHarness", func(t *testing.T) {
		integration := NewPluginIntegration(
			"test",
			"test",
			"Test",
			"nonexistent-binary-12345",
			"/tmp/plugins/memorizer",
			nil,
		)

		status, err := integration.Status()
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if status.Status != StatusMissingHarness {
			t.Errorf("Status = %q, want %q", status.Status, StatusMissingHarness)
		}
	})

	t.Run("IsInstalledPartiallyInstalled", func(t *testing.T) {
		tmpDir := t.TempDir()
		pluginDir := filepath.Join(tmpDir, "plugins", "memorizer")
		_ = os.MkdirAll(pluginDir, 0755)

		// Only create one of two files
		_ = os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("content"), 0644)

		files := []PluginFile{
			{TargetName: "init.lua", Content: []byte("content")},
			{TargetName: "utils.lua", Content: []byte("content")},
		}

		integration := NewPluginIntegration(
			"test-plugin",
			"test",
			"Test",
			"testbin",
			pluginDir,
			files,
		)

		installed, _ := integration.IsInstalled()
		if installed {
			t.Error("IsInstalled should return false for partially installed plugin")
		}
	})
}
