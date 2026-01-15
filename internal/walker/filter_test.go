package walker

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

func TestFilter_ShouldProcessFile(t *testing.T) {
	tests := []struct {
		name   string
		config *registry.PathConfig
		path   string
		want   bool
	}{
		{
			name:   "nil config allows all",
			config: nil,
			path:   "/test/file.go",
			want:   true,
		},
		{
			name:   "empty config allows all",
			config: &registry.PathConfig{},
			path:   "/test/file.go",
			want:   true,
		},
		{
			name: "skip extension",
			config: &registry.PathConfig{
				SkipExtensions: []string{".log"},
			},
			path: "/test/debug.log",
			want: false,
		},
		{
			name: "skip extension without dot",
			config: &registry.PathConfig{
				SkipExtensions: []string{"log"},
			},
			path: "/test/debug.log",
			want: false,
		},
		{
			name: "skip extension case insensitive",
			config: &registry.PathConfig{
				SkipExtensions: []string{".LOG"},
			},
			path: "/test/debug.log",
			want: false,
		},
		{
			name: "allow non-skipped extension",
			config: &registry.PathConfig{
				SkipExtensions: []string{".log"},
			},
			path: "/test/main.go",
			want: true,
		},
		{
			name: "skip hidden file",
			config: &registry.PathConfig{
				SkipHidden: true,
			},
			path: "/test/.hidden",
			want: false,
		},
		{
			name: "allow non-hidden file when skip hidden",
			config: &registry.PathConfig{
				SkipHidden: true,
			},
			path: "/test/visible.txt",
			want: true,
		},
		{
			name: "allow hidden file when skip hidden false",
			config: &registry.PathConfig{
				SkipHidden: false,
			},
			path: "/test/.hidden",
			want: true,
		},
		{
			name: "skip file by name",
			config: &registry.PathConfig{
				SkipFiles: []string{"Makefile"},
			},
			path: "/test/Makefile",
			want: false,
		},
		{
			name: "skip file by glob pattern",
			config: &registry.PathConfig{
				SkipFiles: []string{"*.min.js"},
			},
			path: "/test/app.min.js",
			want: false,
		},
		{
			name: "include extension override",
			config: &registry.PathConfig{
				SkipExtensions:    []string{".go"},
				IncludeExtensions: []string{".go"},
			},
			path: "/test/main.go",
			want: true,
		},
		{
			name: "include file by name override",
			config: &registry.PathConfig{
				SkipFiles:    []string{"*.config"},
				IncludeFiles: []string{"app.config"},
			},
			path: "/test/app.config",
			want: true,
		},
		{
			name: "include only mode - matches",
			config: &registry.PathConfig{
				IncludeExtensions: []string{".go", ".py"},
			},
			path: "/test/main.go",
			want: true,
		},
		{
			name: "include only mode - no match",
			config: &registry.PathConfig{
				IncludeExtensions: []string{".go", ".py"},
			},
			path: "/test/data.json",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.config)
			got := f.ShouldProcessFile(tt.path)
			if got != tt.want {
				t.Errorf("ShouldProcessFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldProcessDir(t *testing.T) {
	tests := []struct {
		name   string
		config *registry.PathConfig
		path   string
		want   bool
	}{
		{
			name:   "nil config allows all",
			config: nil,
			path:   "/test/src",
			want:   true,
		},
		{
			name:   "empty config allows all",
			config: &registry.PathConfig{},
			path:   "/test/src",
			want:   true,
		},
		{
			name: "skip directory by name",
			config: &registry.PathConfig{
				SkipDirectories: []string{"node_modules"},
			},
			path: "/test/node_modules",
			want: false,
		},
		{
			name: "skip directory by glob",
			config: &registry.PathConfig{
				SkipDirectories: []string{"__*__"},
			},
			path: "/test/__pycache__",
			want: false,
		},
		{
			name: "allow non-skipped directory",
			config: &registry.PathConfig{
				SkipDirectories: []string{"node_modules"},
			},
			path: "/test/src",
			want: true,
		},
		{
			name: "skip hidden directory",
			config: &registry.PathConfig{
				SkipHidden: true,
			},
			path: "/test/.git",
			want: false,
		},
		{
			name: "allow non-hidden directory when skip hidden",
			config: &registry.PathConfig{
				SkipHidden: true,
			},
			path: "/test/src",
			want: true,
		},
		{
			name: "allow hidden directory when skip hidden false",
			config: &registry.PathConfig{
				SkipHidden: false,
			},
			path: "/test/.git",
			want: true,
		},
		{
			name: "include directory override",
			config: &registry.PathConfig{
				SkipDirectories:    []string{"vendor"},
				IncludeDirectories: []string{"vendor"},
			},
			path: "/test/vendor",
			want: true,
		},
		{
			name: "hidden directory in include overrides skip hidden",
			config: &registry.PathConfig{
				SkipHidden:         true,
				IncludeDirectories: []string{".github"},
			},
			path: "/test/.github",
			want: true,
		},
		{
			name: "hidden directory not in include is skipped",
			config: &registry.PathConfig{
				SkipHidden:         true,
				IncludeDirectories: []string{".github"},
			},
			path: "/test/.git",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.config)
			got := f.ShouldProcessDir(tt.path)
			if got != tt.want {
				t.Errorf("ShouldProcessDir(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeExt(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"go", ".go"},
		{".go", ".go"},
		{"GO", ".go"},
		{".GO", ".go"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeExt(tt.input)
			if got != tt.want {
				t.Errorf("normalizeExt(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"file.txt", "file.txt", true},
		{"file.txt", "other.txt", false},
		{"*.txt", "file.txt", true},
		{"*.txt", "file.go", false},
		{"test_*", "test_main", true},
		{"test_*", "main_test", false},
		{"*.min.*", "app.min.js", true},
		{"__*__", "__pycache__", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"/"+tt.name, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.name)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldProcessFile_HiddenFileInIncludeOverridesSkipHidden(t *testing.T) {
	// This is an important edge case: .env is hidden, but if it's in IncludeFiles,
	// it should still be processed even when SkipHidden is true.
	tests := []struct {
		name   string
		config *registry.PathConfig
		path   string
		want   bool
	}{
		{
			name: "hidden file in include files overrides skip hidden",
			config: &registry.PathConfig{
				SkipHidden:   true,
				IncludeFiles: []string{".env"},
			},
			path: "/test/.env",
			want: true,
		},
		{
			name: "hidden file not in include files is skipped",
			config: &registry.PathConfig{
				SkipHidden:   true,
				IncludeFiles: []string{".env"},
			},
			path: "/test/.secret",
			want: false,
		},
		{
			name: "hidden file with include extension overrides skip hidden",
			config: &registry.PathConfig{
				SkipHidden:        true,
				IncludeExtensions: []string{".envrc"},
			},
			path: "/test/.local.envrc",
			want: true,
		},
		{
			name: "dotenv pattern matches hidden env files",
			config: &registry.PathConfig{
				SkipHidden:   true,
				IncludeFiles: []string{".env*"},
			},
			path: "/test/.env.local",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.config)
			got := f.ShouldProcessFile(tt.path)
			if got != tt.want {
				t.Errorf("ShouldProcessFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldProcessFile_NoExtension(t *testing.T) {
	// Test files without extensions like Makefile, Dockerfile
	tests := []struct {
		name   string
		config *registry.PathConfig
		path   string
		want   bool
	}{
		{
			name:   "makefile with no config",
			config: &registry.PathConfig{},
			path:   "/test/Makefile",
			want:   true,
		},
		{
			name: "makefile in skip files",
			config: &registry.PathConfig{
				SkipFiles: []string{"Makefile"},
			},
			path: "/test/Makefile",
			want: false,
		},
		{
			name: "dockerfile in include files",
			config: &registry.PathConfig{
				IncludeFiles: []string{"Dockerfile", "Makefile"},
			},
			path: "/test/Dockerfile",
			want: true,
		},
		{
			name: "no extension file not matched by extension skip",
			config: &registry.PathConfig{
				SkipExtensions: []string{".go", ".py"},
			},
			path: "/test/Makefile",
			want: true,
		},
		{
			name: "include only mode - no extension file not in list",
			config: &registry.PathConfig{
				IncludeExtensions: []string{".go", ".py"},
			},
			path: "/test/Makefile",
			want: false, // Has no extension, doesn't match .go or .py
		},
		{
			name: "include only mode - no extension file in include files",
			config: &registry.PathConfig{
				IncludeExtensions: []string{".go"},
				IncludeFiles:      []string{"Makefile"},
			},
			path: "/test/Makefile",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.config)
			got := f.ShouldProcessFile(tt.path)
			if got != tt.want {
				t.Errorf("ShouldProcessFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFilter_HasIncludeRules(t *testing.T) {
	tests := []struct {
		name   string
		config *registry.PathConfig
		want   bool
	}{
		{
			name:   "nil config",
			config: nil,
			want:   false,
		},
		{
			name:   "empty config",
			config: &registry.PathConfig{},
			want:   false,
		},
		{
			name: "has include extensions",
			config: &registry.PathConfig{
				IncludeExtensions: []string{".go"},
			},
			want: true,
		},
		{
			name: "has include files",
			config: &registry.PathConfig{
				IncludeFiles: []string{"Makefile"},
			},
			want: true,
		},
		{
			name: "only skip rules - no include",
			config: &registry.PathConfig{
				SkipExtensions: []string{".log"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.config)
			got := f.hasIncludeRules()
			if got != tt.want {
				t.Errorf("hasIncludeRules() = %v, want %v", got, tt.want)
			}
		})
	}
}
