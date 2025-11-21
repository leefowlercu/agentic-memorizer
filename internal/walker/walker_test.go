package walker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWalk(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create files and directories
	testFiles := []string{
		"file1.txt",
		"file2.md",
		"subdir/file3.txt",
		"subdir/file4.go",
		"subdir/nested/file5.json",
		".hidden.txt",
		"subdir/.hidden_dir/secret.txt",
		".config/settings.json",
		"skip_me.txt",
		"cache/data.bin",
	}

	for _, f := range testFiles {
		path := filepath.Join(tmpDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	tests := []struct {
		name          string
		skipDirs      []string
		skipFiles     []string
		wantFiles     []string
		wantNotFiles  []string
	}{
		{
			name:      "no filters",
			skipDirs:  []string{},
			skipFiles: []string{},
			wantFiles: []string{
				"file1.txt",
				"file2.md",
				"subdir/file3.txt",
				"subdir/file4.go",
				"subdir/nested/file5.json",
				"skip_me.txt",
				"cache/data.bin",
			},
			wantNotFiles: []string{
				".hidden.txt",
				"subdir/.hidden_dir/secret.txt",
				".config/settings.json",
			},
		},
		{
			name:      "skip specific directory",
			skipDirs:  []string{"cache"},
			skipFiles: []string{},
			wantFiles: []string{
				"file1.txt",
				"file2.md",
				"subdir/file3.txt",
			},
			wantNotFiles: []string{
				"cache/data.bin",
				".hidden.txt",
			},
		},
		{
			name:      "skip specific file",
			skipDirs:  []string{},
			skipFiles: []string{"skip_me.txt"},
			wantFiles: []string{
				"file1.txt",
				"file2.md",
				"cache/data.bin",
			},
			wantNotFiles: []string{
				"skip_me.txt",
			},
		},
		{
			name:      "skip multiple dirs and files",
			skipDirs:  []string{"cache", "subdir/nested"},
			skipFiles: []string{"file1.txt", "file4.go"},
			wantFiles: []string{
				"file2.md",
				"subdir/file3.txt",
			},
			wantNotFiles: []string{
				"file1.txt",
				"subdir/file4.go",
				"cache/data.bin",
				"subdir/nested/file5.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var visitedFiles []string

			visitor := func(path string, info os.FileInfo) error {
				relPath, _ := filepath.Rel(tmpDir, path)
				visitedFiles = append(visitedFiles, relPath)
				return nil
			}

			err := Walk(tmpDir, tt.skipDirs, tt.skipFiles, visitor)
			if err != nil {
				t.Fatalf("Walk() error = %v", err)
			}

			// Check expected files were visited
			for _, want := range tt.wantFiles {
				found := false
				for _, visited := range visitedFiles {
					if visited == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to visit %q, but it was not visited. Visited: %v", want, visitedFiles)
				}
			}

			// Check unwanted files were not visited
			for _, unwanted := range tt.wantNotFiles {
				for _, visited := range visitedFiles {
					if visited == unwanted {
						t.Errorf("Did not expect to visit %q, but it was visited", unwanted)
					}
				}
			}
		})
	}
}

func TestWalk_VisitorError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files in a subdirectory so visitor gets called
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Visitor that returns an error
	testErr := os.ErrPermission
	visitor := func(path string, info os.FileInfo) error {
		if strings.HasSuffix(path, "test.txt") {
			return testErr
		}
		return nil
	}

	err := Walk(tmpDir, nil, nil, visitor)
	if err != testErr {
		t.Errorf("Walk() should propagate visitor error, got %v, want %v", err, testErr)
	}
}

func TestWalk_NonExistentDirectory(t *testing.T) {
	nonExistent := "/path/that/does/not/exist"

	visited := false
	visitor := func(path string, info os.FileInfo) error {
		visited = true
		return nil
	}

	err := Walk(nonExistent, nil, nil, visitor)

	// Walk swallows access errors and prints to stderr, so no error is returned
	// But it should still complete without visiting files
	if err != nil {
		t.Errorf("Walk() returned unexpected error: %v", err)
	}

	if visited {
		t.Error("Visitor should not be called for non-existent directory")
	}
}

func TestWalk_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	visitCount := 0
	visitor := func(path string, info os.FileInfo) error {
		visitCount++
		return nil
	}

	err := Walk(tmpDir, nil, nil, visitor)
	if err != nil {
		t.Errorf("Walk() error = %v", err)
	}

	if visitCount != 0 {
		t.Errorf("Walk() visited %d files in empty directory, want 0", visitCount)
	}
}

func TestWalk_RootSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file in root
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	visitedPaths := []string{}
	visitor := func(path string, info os.FileInfo) error {
		visitedPaths = append(visitedPaths, path)
		return nil
	}

	err := Walk(tmpDir, nil, nil, visitor)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	// Root itself should not be visited
	for _, path := range visitedPaths {
		if path == tmpDir {
			t.Error("Walk() should not visit root directory itself")
		}
	}

	// But file in root should be visited
	found := false
	for _, path := range visitedPaths {
		if path == testFile {
			found = true
			break
		}
	}
	if !found {
		t.Error("Walk() should visit files in root directory")
	}
}

func TestWalk_HiddenFilesAndDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create hidden files and directories
	hiddenFile := filepath.Join(tmpDir, ".hidden_file.txt")
	hiddenDir := filepath.Join(tmpDir, ".hidden_dir")
	fileInHiddenDir := filepath.Join(hiddenDir, "secret.txt")

	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0644); err != nil {
		t.Fatalf("Failed to create hidden file: %v", err)
	}
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("Failed to create hidden dir: %v", err)
	}
	if err := os.WriteFile(fileInHiddenDir, []byte("secret"), 0644); err != nil {
		t.Fatalf("Failed to create file in hidden dir: %v", err)
	}

	visitedFiles := []string{}
	visitor := func(path string, info os.FileInfo) error {
		visitedFiles = append(visitedFiles, filepath.Base(path))
		return nil
	}

	err := Walk(tmpDir, nil, nil, visitor)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	// Hidden file should not be visited
	for _, name := range visitedFiles {
		if name == ".hidden_file.txt" {
			t.Error("Walk() should not visit hidden files")
		}
		if name == ".hidden_dir" {
			t.Error("Walk() should not visit hidden directories")
		}
		if name == "secret.txt" {
			t.Error("Walk() should not visit files inside hidden directories")
		}
	}
}

func TestGetRelPath(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "simple relative path",
			root:    "/home/user",
			path:    "/home/user/documents/file.txt",
			want:    "documents/file.txt",
			wantErr: false,
		},
		{
			name:    "same directory",
			root:    "/home/user",
			path:    "/home/user",
			want:    ".",
			wantErr: false,
		},
		{
			name:    "nested path",
			root:    "/var/lib",
			path:    "/var/lib/app/data/config.json",
			want:    "app/data/config.json",
			wantErr: false,
		},
		{
			name:    "path outside root",
			root:    "/home/user/project",
			path:    "/home/user/other",
			want:    "../other",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetRelPath(tt.root, tt.path)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRelPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("GetRelPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetRelPath_RealPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create actual files
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	relPath, err := GetRelPath(tmpDir, testFile)
	if err != nil {
		t.Fatalf("GetRelPath() error = %v", err)
	}

	expected := filepath.Join("subdir", "test.txt")
	if relPath != expected {
		t.Errorf("GetRelPath() = %q, want %q", relPath, expected)
	}
}

func TestWalk_PathCleaning(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test with dirty path (extra slashes, dots, etc.)
	dirtyRoot := tmpDir + "/./"

	visitCount := 0
	visitor := func(path string, info os.FileInfo) error {
		visitCount++
		return nil
	}

	err := Walk(dirtyRoot, nil, nil, visitor)
	if err != nil {
		t.Errorf("Walk() error = %v", err)
	}

	if visitCount == 0 {
		t.Error("Walk() should handle dirty paths and still visit files")
	}
}
