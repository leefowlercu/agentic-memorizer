package config

import (
	"slices"
	"strings"
	"testing"
)

func TestDefaultSkipExtensions(t *testing.T) {
	// Verify slice is not empty
	if len(DefaultSkipExtensions) == 0 {
		t.Error("DefaultSkipExtensions should not be empty")
	}

	// Verify all extensions start with a dot
	for i, ext := range DefaultSkipExtensions {
		if !strings.HasPrefix(ext, ".") {
			t.Errorf("DefaultSkipExtensions[%d] = %q should start with '.'", i, ext)
		}
	}

	// Verify expected common extensions are present
	expected := []string{".exe", ".dll", ".pyc", ".log", ".tmp"}
	for _, exp := range expected {
		if !slices.Contains(DefaultSkipExtensions, exp) {
			t.Errorf("DefaultSkipExtensions should contain %q", exp)
		}
	}
}

func TestDefaultSkipDirectories(t *testing.T) {
	// Verify slice is not empty
	if len(DefaultSkipDirectories) == 0 {
		t.Error("DefaultSkipDirectories should not be empty")
	}

	// Verify expected common directories are present
	expected := []string{".git", "node_modules", "__pycache__", ".vscode"}
	for _, exp := range expected {
		if !slices.Contains(DefaultSkipDirectories, exp) {
			t.Errorf("DefaultSkipDirectories should contain %q", exp)
		}
	}
}

func TestDefaultSkipFiles(t *testing.T) {
	// Verify slice is not empty
	if len(DefaultSkipFiles) == 0 {
		t.Error("DefaultSkipFiles should not be empty")
	}

	// Verify expected common files are present
	expected := []string{".DS_Store", "package-lock.json", "*.min.js"}
	for _, exp := range expected {
		if !slices.Contains(DefaultSkipFiles, exp) {
			t.Errorf("DefaultSkipFiles should contain %q", exp)
		}
	}

	// Verify editor artifacts are present
	editorArtifacts := []string{"4913", "#*", "*~"}
	for _, artifact := range editorArtifacts {
		if !slices.Contains(DefaultSkipFiles, artifact) {
			t.Errorf("DefaultSkipFiles should contain editor artifact %q", artifact)
		}
	}
}

func TestDefaultSkipHidden(t *testing.T) {
	if DefaultSkipHidden != true {
		t.Errorf("DefaultSkipHidden = %v, want true", DefaultSkipHidden)
	}
}

func TestDefaultsConfig_SkipArraysNotNil(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.Defaults.Skip.Extensions == nil {
		t.Error("Defaults.Skip.Extensions should not be nil")
	}
	if cfg.Defaults.Skip.Directories == nil {
		t.Error("Defaults.Skip.Directories should not be nil")
	}
	if cfg.Defaults.Skip.Files == nil {
		t.Error("Defaults.Skip.Files should not be nil")
	}
}

func TestDefaultsConfig_IncludeArraysEmpty(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.Defaults.Include.Extensions == nil {
		t.Error("Defaults.Include.Extensions should not be nil (should be empty slice)")
	}
	if len(cfg.Defaults.Include.Extensions) != 0 {
		t.Errorf("Defaults.Include.Extensions should be empty, got %v", cfg.Defaults.Include.Extensions)
	}

	if cfg.Defaults.Include.Directories == nil {
		t.Error("Defaults.Include.Directories should not be nil (should be empty slice)")
	}
	if len(cfg.Defaults.Include.Directories) != 0 {
		t.Errorf("Defaults.Include.Directories should be empty, got %v", cfg.Defaults.Include.Directories)
	}

	if cfg.Defaults.Include.Files == nil {
		t.Error("Defaults.Include.Files should not be nil (should be empty slice)")
	}
	if len(cfg.Defaults.Include.Files) != 0 {
		t.Errorf("Defaults.Include.Files should be empty, got %v", cfg.Defaults.Include.Files)
	}
}

func TestDefaultSkipExtensions_NoMinifiedPatterns(t *testing.T) {
	// Verify .min.js and .min.css are NOT in extensions (they don't work with filepath.Ext)
	for _, ext := range DefaultSkipExtensions {
		if ext == ".min.js" || ext == ".min.css" {
			t.Errorf("DefaultSkipExtensions should not contain %q (use glob in files instead)", ext)
		}
	}
}

func TestDefaultSkipFiles_HasMinifiedGlobs(t *testing.T) {
	// Verify minified bundle globs are in files
	expected := []string{"*.min.js", "*.min.css", "*.bundle.js"}
	for _, exp := range expected {
		if !slices.Contains(DefaultSkipFiles, exp) {
			t.Errorf("DefaultSkipFiles should contain glob pattern %q", exp)
		}
	}
}
