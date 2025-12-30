package integrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "path without tilde",
			path:    "/usr/local/bin",
			wantErr: false,
		},
		{
			name:    "path with tilde only",
			path:    "~",
			wantErr: false,
		},
		{
			name:    "path with tilde and subdir",
			path:    "~/Documents/test",
			wantErr: false,
		},
		{
			name:    "relative path",
			path:    "relative/path",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded, err := ExpandPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if expanded == "" {
					t.Error("ExpandPath() returned empty string")
				}

				// If path starts with ~, result should not contain ~
				if len(tt.path) > 0 && tt.path[0] == '~' {
					if len(expanded) > 0 && expanded[0] == '~' {
						t.Errorf("ExpandPath() did not expand tilde: got %q", expanded)
					}
				}

				// If path doesn't start with ~, result should match input
				if len(tt.path) > 0 && tt.path[0] != '~' {
					if expanded != tt.path {
						t.Errorf("ExpandPath() = %q, want %q", expanded, tt.path)
					}
				}
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test directory
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing file",
			path: testFile,
			want: true,
		},
		{
			name: "non-existent file",
			path: filepath.Join(tmpDir, "nonexistent.txt"),
			want: false,
		},
		{
			name: "directory (not a file)",
			path: testDir,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileExists(tt.path)
			if got != tt.want {
				t.Errorf("FileExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing directory",
			path: testDir,
			want: true,
		},
		{
			name: "non-existent directory",
			path: filepath.Join(tmpDir, "nonexistent"),
			want: false,
		},
		{
			name: "file (not a directory)",
			path: testFile,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DirExists(tt.path)
			if got != tt.want {
				t.Errorf("DirExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "create single directory",
			path:    filepath.Join(tmpDir, "single"),
			wantErr: false,
		},
		{
			name:    "create nested directories",
			path:    filepath.Join(tmpDir, "nested/deep/path"),
			wantErr: false,
		},
		{
			name:    "already exists",
			path:    tmpDir,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureDir(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify directory was created
				if !DirExists(tt.path) {
					t.Errorf("EnsureDir() did not create directory %q", tt.path)
				}
			}
		})
	}
}

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{
			name:    "xml is valid",
			format:  "xml",
			wantErr: false,
		},
		{
			name:    "json is valid",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "invalid format",
			format:  "yaml",
			wantErr: true,
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		want    OutputFormat
		wantErr bool
	}{
		{
			name:    "xml format",
			format:  "xml",
			want:    FormatXML,
			wantErr: false,
		},
		{
			name:    "json format",
			format:  "json",
			want:    FormatJSON,
			wantErr: false,
		},
		{
			name:    "invalid format",
			format:  "yaml",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOutputFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOutputFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseOutputFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDefaultOutputFormat(t *testing.T) {
	got := GetDefaultOutputFormat()
	if got != FormatXML {
		t.Errorf("GetDefaultOutputFormat() = %v, want %v", got, FormatXML)
	}
}

func TestIsExecutable(t *testing.T) {
	tmpDir := t.TempDir()

	// Create executable file
	execFile := filepath.Join(tmpDir, "executable")
	if err := os.WriteFile(execFile, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create executable: %v", err)
	}

	// Create non-executable file
	nonExecFile := filepath.Join(tmpDir, "nonexecutable")
	if err := os.WriteFile(nonExecFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create non-executable: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "executable file",
			path: execFile,
			want: true,
		},
		{
			name: "non-executable file",
			path: nonExecFile,
			want: false,
		},
		{
			name: "non-existent file",
			path: filepath.Join(tmpDir, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsExecutable(tt.path)
			if got != tt.want {
				t.Errorf("IsExecutable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateBinaryPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid executable
	validExec := filepath.Join(tmpDir, "valid")
	if err := os.WriteFile(validExec, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create valid executable: %v", err)
	}

	// Create non-executable file
	nonExec := filepath.Join(tmpDir, "nonexec")
	if err := os.WriteFile(nonExec, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create non-executable: %v", err)
	}

	// Create directory
	dirPath := filepath.Join(tmpDir, "dir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid executable",
			path:    validExec,
			wantErr: false,
		},
		{
			name:    "non-existent path",
			path:    filepath.Join(tmpDir, "nonexistent"),
			wantErr: true,
		},
		{
			name:    "non-executable file",
			path:    nonExec,
			wantErr: true,
		},
		{
			name:    "directory path",
			path:    dirPath,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBinaryPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBinaryPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
