package fileops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ====================
// Conflict Resolution Tests
// ====================

func TestResolveConflict_NoConflict(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.txt")

	result, err := ResolveConflict(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != path {
		t.Errorf("expected %s, got %s", path, result)
	}
}

func TestResolveConflict_SingleExtension(t *testing.T) {
	tempDir := t.TempDir()

	// Create existing file
	existing := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(existing, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := ResolveConflict(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tempDir, "file-1.txt")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestResolveConflict_MultipleConflicts(t *testing.T) {
	tempDir := t.TempDir()

	// Create file.txt, file-1.txt, file-2.txt
	for _, name := range []string{"file.txt", "file-1.txt", "file-2.txt"} {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	existing := filepath.Join(tempDir, "file.txt")
	result, err := ResolveConflict(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tempDir, "file-3.txt")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestResolveConflict_CompoundExtension(t *testing.T) {
	tempDir := t.TempDir()

	// Create existing file with compound extension
	existing := filepath.Join(tempDir, "archive.tar.gz")
	if err := os.WriteFile(existing, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := ResolveConflict(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tempDir, "archive-1.tar.gz")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestResolveConflict_Directory(t *testing.T) {
	tempDir := t.TempDir()

	// Create existing directory
	existing := filepath.Join(tempDir, "mydir")
	if err := os.Mkdir(existing, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	result, err := ResolveConflict(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tempDir, "mydir-1")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestResolveConflict_NoExtension(t *testing.T) {
	tempDir := t.TempDir()

	existing := filepath.Join(tempDir, "README")
	if err := os.WriteFile(existing, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := ResolveConflict(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tempDir, "README-1")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestSplitNameAndExtensions(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantExt  string
	}{
		{"file.md", "file", ".md"},
		{"file.txt", "file", ".txt"},
		{"archive.tar.gz", "archive", ".tar.gz"},
		{"archive.tar.bz2", "archive", ".tar.bz2"},
		{"README", "README", ""},
		{".hidden", ".hidden", ""},
		{".hidden.txt", ".hidden", ".txt"},
		{"multi.part.name.txt", "multi.part.name", ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, ext := splitNameAndExtensions(tt.input)
			if name != tt.wantName {
				t.Errorf("name: got %s, want %s", name, tt.wantName)
			}
			if ext != tt.wantExt {
				t.Errorf("ext: got %s, want %s", ext, tt.wantExt)
			}
		})
	}
}

func TestHasConflictSuffix(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"file.txt", false},
		{"file-1.txt", true},
		{"file-123.txt", true},
		{"archive-1.tar.gz", true},
		{"dir-2", true},
		{"file-abc.txt", false},
		{"file-.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := HasConflictSuffix(tt.path)
			if got != tt.want {
				t.Errorf("HasConflictSuffix(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestGetConflictNumber(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"file.txt", 0},
		{"file-1.txt", 1},
		{"file-42.txt", 42},
		{"archive-100.tar.gz", 100},
		{"dir-5", 5},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := GetConflictNumber(tt.path)
			if got != tt.want {
				t.Errorf("GetConflictNumber(%q) = %d, want %d", tt.path, got, tt.want)
			}
		})
	}
}

// ====================
// Path Utilities Tests
// ====================

func TestIsInDirectory(t *testing.T) {
	tests := []struct {
		name string
		path string
		dir  string
		want bool
	}{
		{"file in dir", "/home/user/file.txt", "/home/user", true},
		{"nested file", "/home/user/subdir/file.txt", "/home/user", true},
		{"same dir", "/home/user", "/home/user", true},
		{"different dir", "/home/other/file.txt", "/home/user", false},
		{"sibling", "/home/user2", "/home/user", false},
		{"parent", "/home", "/home/user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsInDirectory(tt.path, tt.dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("IsInDirectory(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"simple path", "file.txt", false},
		{"nested path", "subdir/file.txt", false},
		{"empty", "", true},
		{"parent ref at start", "../file.txt", true},
		{"parent ref in middle", "subdir/../file.txt", true},
		{"null byte", "file\x00.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSubdirectory(t *testing.T) {
	tests := []struct {
		name    string
		subdir  string
		wantErr bool
	}{
		{"empty", "", false},
		{"simple", "subdir", false},
		{"nested", "subdir/nested", false},
		{"absolute", "/absolute/path", true},
		{"parent ref", "../escape", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubdirectory(tt.subdir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSubdirectory(%q) error = %v, wantErr %v", tt.subdir, err, tt.wantErr)
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create new directory
	newDir := filepath.Join(tempDir, "new", "nested", "dir")
	if err := EnsureDir(newDir); err != nil {
		t.Fatalf("failed to create new directory: %v", err)
	}

	if !IsDir(newDir) {
		t.Error("directory was not created")
	}

	// Calling again should be no-op
	if err := EnsureDir(newDir); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Creating over a file should fail
	filePath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := EnsureDir(filePath); err == nil {
		t.Error("expected error when creating dir over file")
	}
}

func TestRelativePath(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		target  string
		want    string
		wantErr bool
	}{
		{"simple", "/home/user", "/home/user/file.txt", "file.txt", false},
		{"nested", "/home/user", "/home/user/a/b/file.txt", "a/b/file.txt", false},
		{"same", "/home/user", "/home/user", ".", false},
		{"outside", "/home/user", "/home/other/file.txt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RelativePath(tt.base, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"no tilde", "/absolute/path", "/absolute/path", false},
		{"tilde only", "~", home, false},
		{"tilde slash", "~/subdir", filepath.Join(home, "subdir"), false},
		{"tilde user", "~other", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandHome(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathChecks(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file
	filePath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create test directory
	dirPath := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	nonexistent := filepath.Join(tempDir, "nonexistent")

	// PathExists
	if !PathExists(filePath) {
		t.Error("PathExists should return true for existing file")
	}
	if !PathExists(dirPath) {
		t.Error("PathExists should return true for existing dir")
	}
	if PathExists(nonexistent) {
		t.Error("PathExists should return false for nonexistent")
	}

	// IsFile
	if !IsFile(filePath) {
		t.Error("IsFile should return true for file")
	}
	if IsFile(dirPath) {
		t.Error("IsFile should return false for directory")
	}
	if IsFile(nonexistent) {
		t.Error("IsFile should return false for nonexistent")
	}

	// IsDir
	if IsDir(filePath) {
		t.Error("IsDir should return false for file")
	}
	if !IsDir(dirPath) {
		t.Error("IsDir should return true for directory")
	}
	if IsDir(nonexistent) {
		t.Error("IsDir should return false for nonexistent")
	}
}

// ====================
// Copy Tests
// ====================

func TestCopy_File(t *testing.T) {
	tempDir := t.TempDir()
	content := []byte("test content")

	// Create source file
	src := filepath.Join(tempDir, "source.txt")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Copy to new location
	dst := filepath.Join(tempDir, "dest.txt")
	result, err := Copy(src, dst, false)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Verify result
	if result.Src != src {
		t.Errorf("result.Src = %s, want %s", result.Src, src)
	}
	if result.Dst != dst {
		t.Errorf("result.Dst = %s, want %s", result.Dst, dst)
	}
	if result.IsDir {
		t.Error("result.IsDir should be false")
	}
	if result.FileCount != 1 {
		t.Errorf("result.FileCount = %d, want 1", result.FileCount)
	}
	if result.Size != int64(len(content)) {
		t.Errorf("result.Size = %d, want %d", result.Size, len(content))
	}
	if result.Renamed {
		t.Error("result.Renamed should be false")
	}

	// Verify content
	gotContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dest: %v", err)
	}
	if string(gotContent) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", gotContent, content)
	}
}

func TestCopy_FileConflict(t *testing.T) {
	tempDir := t.TempDir()

	// Create source and existing dest
	src := filepath.Join(tempDir, "source.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	dst := filepath.Join(tempDir, "dest.txt")
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing dest: %v", err)
	}

	// Copy without force (should rename)
	result, err := Copy(src, dst, false)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if !result.Renamed {
		t.Error("result.Renamed should be true")
	}

	expectedDst := filepath.Join(tempDir, "dest-1.txt")
	if result.Dst != expectedDst {
		t.Errorf("result.Dst = %s, want %s", result.Dst, expectedDst)
	}

	// Verify original is unchanged
	origContent, _ := os.ReadFile(dst)
	if string(origContent) != "existing" {
		t.Error("original file was modified")
	}
}

func TestCopy_FileForce(t *testing.T) {
	tempDir := t.TempDir()

	// Create source and existing dest
	src := filepath.Join(tempDir, "source.txt")
	if err := os.WriteFile(src, []byte("new content"), 0644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	dst := filepath.Join(tempDir, "dest.txt")
	if err := os.WriteFile(dst, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create existing dest: %v", err)
	}

	// Copy with force (should overwrite)
	result, err := Copy(src, dst, true)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if result.Renamed {
		t.Error("result.Renamed should be false with force")
	}
	if result.Dst != dst {
		t.Errorf("result.Dst = %s, want %s", result.Dst, dst)
	}

	// Verify content was overwritten
	gotContent, _ := os.ReadFile(dst)
	if string(gotContent) != "new content" {
		t.Errorf("content not overwritten: got %q", gotContent)
	}
}

func TestCopy_Directory(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory structure
	srcDir := filepath.Join(tempDir, "source")
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

	// Copy directory
	dstDir := filepath.Join(tempDir, "dest")
	result, err := Copy(srcDir, dstDir, false)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if !result.IsDir {
		t.Error("result.IsDir should be true")
	}
	if result.FileCount != 2 {
		t.Errorf("result.FileCount = %d, want 2", result.FileCount)
	}

	// Verify structure
	if !IsFile(filepath.Join(dstDir, "file1.txt")) {
		t.Error("file1.txt not copied")
	}
	if !IsFile(filepath.Join(dstDir, "subdir", "file2.txt")) {
		t.Error("subdir/file2.txt not copied")
	}
}

func TestCopyBatch(t *testing.T) {
	tempDir := t.TempDir()

	// Create source files
	src1 := filepath.Join(tempDir, "file1.txt")
	src2 := filepath.Join(tempDir, "file2.txt")
	os.WriteFile(src1, []byte("content1"), 0644)
	os.WriteFile(src2, []byte("content2"), 0644)

	destDir := filepath.Join(tempDir, "dest")
	os.Mkdir(destDir, 0755)

	items := []CopyItem{
		{Src: src1, Dst: filepath.Join(destDir, "file1.txt")},
		{Src: src2, Dst: filepath.Join(destDir, "file2.txt")},
	}

	results, errors := CopyBatch(items, false)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	for i, err := range errors {
		if err != nil {
			t.Errorf("item %d failed: %v", i, err)
		}
	}
}

func TestCopyBatch_PartialFailure(t *testing.T) {
	tempDir := t.TempDir()

	// Create only one source file
	src1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(src1, []byte("content1"), 0644)

	destDir := filepath.Join(tempDir, "dest")
	os.Mkdir(destDir, 0755)

	items := []CopyItem{
		{Src: src1, Dst: filepath.Join(destDir, "file1.txt")},
		{Src: filepath.Join(tempDir, "nonexistent.txt"), Dst: filepath.Join(destDir, "file2.txt")},
	}

	results, errors := CopyBatch(items, false)

	// First should succeed
	if errors[0] != nil {
		t.Errorf("first item should succeed: %v", errors[0])
	}

	// Second should fail
	if errors[1] == nil {
		t.Error("second item should fail")
	}

	// First result should be valid
	if results[0].FileCount != 1 {
		t.Error("first result should have FileCount=1")
	}
}

func TestCopyToDir(t *testing.T) {
	tempDir := t.TempDir()

	src := filepath.Join(tempDir, "source.txt")
	os.WriteFile(src, []byte("content"), 0644)

	destDir := filepath.Join(tempDir, "dest")
	os.Mkdir(destDir, 0755)

	result, err := CopyToDir(src, destDir, false)
	if err != nil {
		t.Fatalf("CopyToDir failed: %v", err)
	}

	expected := filepath.Join(destDir, "source.txt")
	if result.Dst != expected {
		t.Errorf("result.Dst = %s, want %s", result.Dst, expected)
	}
}

// ====================
// Move Tests
// ====================

func TestMove_File(t *testing.T) {
	tempDir := t.TempDir()
	content := []byte("test content")

	// Create source file
	src := filepath.Join(tempDir, "source.txt")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Move to new location
	dst := filepath.Join(tempDir, "dest.txt")
	result, err := Move(src, dst)
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	// Verify result
	if result.Src != src {
		t.Errorf("result.Src = %s, want %s", result.Src, src)
	}
	if result.Dst != dst {
		t.Errorf("result.Dst = %s, want %s", result.Dst, dst)
	}
	if result.IsDir {
		t.Error("result.IsDir should be false")
	}
	if result.FileCount != 1 {
		t.Errorf("result.FileCount = %d, want 1", result.FileCount)
	}

	// Verify source is gone
	if PathExists(src) {
		t.Error("source should be deleted")
	}

	// Verify destination exists with correct content
	gotContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dest: %v", err)
	}
	if string(gotContent) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", gotContent, content)
	}
}

func TestMove_FileConflict(t *testing.T) {
	tempDir := t.TempDir()

	// Create source and existing dest
	src := filepath.Join(tempDir, "source.txt")
	if err := os.WriteFile(src, []byte("source content"), 0644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	dst := filepath.Join(tempDir, "dest.txt")
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing dest: %v", err)
	}

	// Move (should rename)
	result, err := Move(src, dst)
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	if !result.Renamed {
		t.Error("result.Renamed should be true")
	}

	expectedDst := filepath.Join(tempDir, "dest-1.txt")
	if result.Dst != expectedDst {
		t.Errorf("result.Dst = %s, want %s", result.Dst, expectedDst)
	}

	// Verify original dest is unchanged
	origContent, _ := os.ReadFile(dst)
	if string(origContent) != "existing" {
		t.Error("original file was modified")
	}

	// Verify source moved to new location
	movedContent, _ := os.ReadFile(expectedDst)
	if string(movedContent) != "source content" {
		t.Error("content not moved correctly")
	}
}

func TestMove_Directory(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory structure
	srcDir := filepath.Join(tempDir, "source")
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

	// Move directory
	dstDir := filepath.Join(tempDir, "dest")
	result, err := Move(srcDir, dstDir)
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	if !result.IsDir {
		t.Error("result.IsDir should be true")
	}
	if result.FileCount != 2 {
		t.Errorf("result.FileCount = %d, want 2", result.FileCount)
	}

	// Verify source is gone
	if PathExists(srcDir) {
		t.Error("source directory should be deleted")
	}

	// Verify structure at destination
	if !IsFile(filepath.Join(dstDir, "file1.txt")) {
		t.Error("file1.txt not moved")
	}
	if !IsFile(filepath.Join(dstDir, "subdir", "file2.txt")) {
		t.Error("subdir/file2.txt not moved")
	}
}

func TestMoveBatch(t *testing.T) {
	tempDir := t.TempDir()

	// Create source files
	src1 := filepath.Join(tempDir, "file1.txt")
	src2 := filepath.Join(tempDir, "file2.txt")
	os.WriteFile(src1, []byte("content1"), 0644)
	os.WriteFile(src2, []byte("content2"), 0644)

	destDir := filepath.Join(tempDir, "dest")
	os.Mkdir(destDir, 0755)

	items := []MoveItem{
		{Src: src1, Dst: filepath.Join(destDir, "file1.txt")},
		{Src: src2, Dst: filepath.Join(destDir, "file2.txt")},
	}

	results, errors := MoveBatch(items)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	for i, err := range errors {
		if err != nil {
			t.Errorf("item %d failed: %v", i, err)
		}
	}

	// Verify sources are gone
	if PathExists(src1) || PathExists(src2) {
		t.Error("source files should be deleted")
	}
}

func TestMoveToDir(t *testing.T) {
	tempDir := t.TempDir()

	src := filepath.Join(tempDir, "source.txt")
	os.WriteFile(src, []byte("content"), 0644)

	destDir := filepath.Join(tempDir, "dest")
	os.Mkdir(destDir, 0755)

	result, err := MoveToDir(src, destDir)
	if err != nil {
		t.Fatalf("MoveToDir failed: %v", err)
	}

	expected := filepath.Join(destDir, "source.txt")
	if result.Dst != expected {
		t.Errorf("result.Dst = %s, want %s", result.Dst, expected)
	}

	if PathExists(src) {
		t.Error("source should be deleted")
	}
}

func TestMove_NonexistentSource(t *testing.T) {
	tempDir := t.TempDir()

	src := filepath.Join(tempDir, "nonexistent.txt")
	dst := filepath.Join(tempDir, "dest.txt")

	_, err := Move(src, dst)
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "source not found") {
		t.Errorf("error should mention source not found: %v", err)
	}
}

func TestCopy_NonexistentSource(t *testing.T) {
	tempDir := t.TempDir()

	src := filepath.Join(tempDir, "nonexistent.txt")
	dst := filepath.Join(tempDir, "dest.txt")

	_, err := Copy(src, dst, false)
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "source not found") {
		t.Errorf("error should mention source not found: %v", err)
	}
}
