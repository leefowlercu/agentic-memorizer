package subcommands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFile_NonExistentPath(t *testing.T) {
	// Reset flags
	rememberFileDir = ""
	rememberFileForce = false
	rememberFileDryRun = false

	cmd := FileCmd
	args := []string{"/nonexistent/path/file.md"}

	err := validateFile(cmd, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestValidateFile_ValidFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Reset flags
	rememberFileDir = ""
	rememberFileForce = false
	rememberFileDryRun = false

	cmd := FileCmd
	args := []string{testFile}

	err = validateFile(cmd, args)
	assert.NoError(t, err)
}

func TestValidateFile_ValidDirectory(t *testing.T) {
	// Create temp directory with files
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(testDir, "test.md")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Reset flags
	rememberFileDir = ""
	rememberFileForce = false
	rememberFileDryRun = false

	cmd := FileCmd
	args := []string{testDir}

	err = validateFile(cmd, args)
	assert.NoError(t, err)
}

func TestValidateFile_InvalidDir(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Set invalid --dir flag
	rememberFileDir = "../escape"
	rememberFileForce = false
	rememberFileDryRun = false

	cmd := FileCmd
	args := []string{testFile}

	err = validateFile(cmd, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --dir value")

	// Clean up flag
	rememberFileDir = ""
}

func TestValidateFile_LargeBatchWarning(t *testing.T) {
	// Create temp directory with many files
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "largedir")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	// Create more than largeFileBatchThreshold files
	for i := 0; i < largeFileBatchThreshold+10; i++ {
		testFile := filepath.Join(testDir, fmt.Sprintf("file%d.md", i))
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)
	}

	// Reset flags (not forcing)
	rememberFileDir = ""
	rememberFileForce = false
	rememberFileDryRun = false

	cmd := FileCmd
	args := []string{testDir}

	err = validateFile(cmd, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "large batch detected")
}

func TestValidateFile_LargeBatchWithForce(t *testing.T) {
	// Create temp directory with many files
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "largedir")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	// Create more than largeFileBatchThreshold files
	for i := 0; i < largeFileBatchThreshold+10; i++ {
		testFile := filepath.Join(testDir, fmt.Sprintf("file%d.md", i))
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)
	}

	// Set force flag
	rememberFileDir = ""
	rememberFileForce = true
	rememberFileDryRun = false

	cmd := FileCmd
	args := []string{testDir}

	err = validateFile(cmd, args)
	assert.NoError(t, err) // Should pass with --force

	// Clean up flag
	rememberFileForce = false
}

func TestValidateFile_UnreadableFile(t *testing.T) {
	// Create temp file with no read permissions
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "unreadable.md")
	err := os.WriteFile(testFile, []byte("test content"), 0000)
	require.NoError(t, err)

	// Reset flags
	rememberFileDir = ""
	rememberFileForce = false
	rememberFileDryRun = false

	cmd := FileCmd
	args := []string{testFile}

	err = validateFile(cmd, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read file")

	// Clean up permissions for deletion
	_ = os.Chmod(testFile, 0644)
}

func TestCountFilesInPath_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	count, err := countFilesInPath(testFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCountFilesInPath_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	err := os.MkdirAll(filepath.Join(testDir, "subdir"), 0755)
	require.NoError(t, err)

	// Create 3 files
	for _, name := range []string{"file1.md", "file2.md", "subdir/file3.md"} {
		err = os.WriteFile(filepath.Join(testDir, name), []byte("content"), 0644)
		require.NoError(t, err)
	}

	count, err := countFilesInPath(testDir)
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountFilesInPath_NonExistent(t *testing.T) {
	_, err := countFilesInPath("/nonexistent/path")
	assert.Error(t, err)
}

func TestHasSemanticHandler(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".md", true},
		{".go", true},
		{".py", true},
		{".json", true},
		{".docx", true},
		{".pdf", true},
		{".png", true},
		{".unknown", false},
		{".xyz", false},
		{".zip", false},
		{".exe", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := hasSemanticHandler(nil, tt.ext)
			assert.Equal(t, tt.expected, result, "extension %s", tt.ext)
		})
	}
}

func TestFileCmd_HasCorrectFlags(t *testing.T) {
	// Check that all flags are defined
	dirFlag := FileCmd.Flags().Lookup("dir")
	assert.NotNil(t, dirFlag)
	assert.Equal(t, "string", dirFlag.Value.Type())

	forceFlag := FileCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "bool", forceFlag.Value.Type())

	dryRunFlag := FileCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag)
	assert.Equal(t, "bool", dryRunFlag.Value.Type())
}

func TestFileCmd_RequiresAtLeastOneArg(t *testing.T) {
	// FileCmd.Args is cobra.MinimumNArgs(1)
	assert.NotNil(t, FileCmd.Args)
}
