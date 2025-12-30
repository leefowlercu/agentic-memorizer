package skip

import (
	"testing"
)

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		cfg      *Config
		expected bool
	}{
		{
			name:     "always skip .git",
			dirName:  ".git",
			cfg:      &Config{SkipHidden: false},
			expected: true,
		},
		{
			name:     "always skip .cache",
			dirName:  ".cache",
			cfg:      &Config{SkipHidden: false},
			expected: true,
		},
		{
			name:     "always skip .forgotten",
			dirName:  ".forgotten",
			cfg:      &Config{SkipHidden: false},
			expected: true,
		},
		{
			name:     "skip hidden directory when SkipHidden true",
			dirName:  ".hidden",
			cfg:      &Config{SkipHidden: true},
			expected: true,
		},
		{
			name:     "allow hidden directory when SkipHidden false",
			dirName:  ".hidden",
			cfg:      &Config{SkipHidden: false},
			expected: false,
		},
		{
			name:     "skip user-configured directory",
			dirName:  "node_modules",
			cfg:      &Config{SkipHidden: true, SkipDirs: []string{"node_modules", "vendor"}},
			expected: true,
		},
		{
			name:     "allow non-configured directory",
			dirName:  "src",
			cfg:      &Config{SkipHidden: true, SkipDirs: []string{"node_modules"}},
			expected: false,
		},
		{
			name:     "allow regular directory with default config",
			dirName:  "documents",
			cfg:      &Config{SkipHidden: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSkipDir(tt.dirName, tt.cfg)
			if result != tt.expected {
				t.Errorf("ShouldSkipDir(%q) = %v, want %v", tt.dirName, result, tt.expected)
			}
		})
	}
}

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		cfg      *Config
		expected bool
	}{
		{
			name:     "skip hidden file when SkipHidden true",
			fileName: ".gitignore",
			cfg:      &Config{SkipHidden: true},
			expected: true,
		},
		{
			name:     "allow hidden file when SkipHidden false",
			fileName: ".gitignore",
			cfg:      &Config{SkipHidden: false},
			expected: false,
		},
		{
			name:     "skip user-configured file",
			fileName: "Thumbs.db",
			cfg:      &Config{SkipHidden: true, SkipFiles: []string{"Thumbs.db", ".DS_Store"}},
			expected: true,
		},
		{
			name:     "allow non-configured file",
			fileName: "readme.txt",
			cfg:      &Config{SkipHidden: true, SkipFiles: []string{"Thumbs.db"}},
			expected: false,
		},
		{
			name:     "skip file by extension",
			fileName: "backup.bak",
			cfg:      &Config{SkipHidden: true, SkipExtensions: []string{".bak", ".tmp"}},
			expected: true,
		},
		{
			name:     "skip file by another extension",
			fileName: "cache.tmp",
			cfg:      &Config{SkipHidden: true, SkipExtensions: []string{".bak", ".tmp"}},
			expected: true,
		},
		{
			name:     "allow file with non-skipped extension",
			fileName: "document.txt",
			cfg:      &Config{SkipHidden: true, SkipExtensions: []string{".bak", ".tmp"}},
			expected: false,
		},
		{
			name:     "allow regular file with default config",
			fileName: "main.go",
			cfg:      &Config{SkipHidden: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSkipFile(tt.fileName, tt.cfg)
			if result != tt.expected {
				t.Errorf("ShouldSkipFile(%q) = %v, want %v", tt.fileName, result, tt.expected)
			}
		})
	}
}

func TestShouldSkip(t *testing.T) {
	cfg := &Config{
		SkipHidden:     true,
		SkipDirs:       []string{"vendor"},
		SkipFiles:      []string{"Thumbs.db"},
		SkipExtensions: []string{".bak"},
	}

	tests := []struct {
		name     string
		path     string
		isDir    bool
		expected bool
	}{
		{
			name:     "directory - always skip .git",
			path:     ".git",
			isDir:    true,
			expected: true,
		},
		{
			name:     "directory - user configured",
			path:     "vendor",
			isDir:    true,
			expected: true,
		},
		{
			name:     "directory - regular",
			path:     "src",
			isDir:    true,
			expected: false,
		},
		{
			name:     "file - hidden",
			path:     ".env",
			isDir:    false,
			expected: true,
		},
		{
			name:     "file - user configured",
			path:     "Thumbs.db",
			isDir:    false,
			expected: true,
		},
		{
			name:     "file - skipped extension",
			path:     "data.bak",
			isDir:    false,
			expected: true,
		},
		{
			name:     "file - regular",
			path:     "main.go",
			isDir:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSkip(tt.path, tt.isDir, cfg)
			if result != tt.expected {
				t.Errorf("ShouldSkip(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, result, tt.expected)
			}
		})
	}
}

func TestAlwaysSkipDirs(t *testing.T) {
	expected := []string{".git", ".cache", ".forgotten"}

	if len(AlwaysSkipDirs) != len(expected) {
		t.Errorf("AlwaysSkipDirs has %d entries, want %d", len(AlwaysSkipDirs), len(expected))
	}

	for i, dir := range expected {
		if AlwaysSkipDirs[i] != dir {
			t.Errorf("AlwaysSkipDirs[%d] = %q, want %q", i, AlwaysSkipDirs[i], dir)
		}
	}
}

func TestNilConfig(t *testing.T) {
	// Test that functions handle nil config gracefully by panicking with nil pointer
	// This is expected behavior - callers must provide a valid config
	defer func() {
		if r := recover(); r == nil {
			t.Error("ShouldSkipDir with nil config should panic")
		}
	}()

	ShouldSkipDir("test", nil)
}

func TestEmptyConfig(t *testing.T) {
	cfg := &Config{}

	t.Run("empty config allows regular directory", func(t *testing.T) {
		if ShouldSkipDir("src", cfg) {
			t.Error("empty config should allow regular directories")
		}
	})

	t.Run("empty config allows regular file", func(t *testing.T) {
		if ShouldSkipFile("main.go", cfg) {
			t.Error("empty config should allow regular files")
		}
	})

	t.Run("empty config still skips always-skip dirs", func(t *testing.T) {
		if !ShouldSkipDir(".git", cfg) {
			t.Error("empty config should still skip .git")
		}
		if !ShouldSkipDir(".cache", cfg) {
			t.Error("empty config should still skip .cache")
		}
		if !ShouldSkipDir(".forgotten", cfg) {
			t.Error("empty config should still skip .forgotten")
		}
	})

	t.Run("empty config allows hidden files (SkipHidden is false)", func(t *testing.T) {
		if ShouldSkipFile(".env", cfg) {
			t.Error("empty config (SkipHidden=false) should allow hidden files")
		}
	})
}
