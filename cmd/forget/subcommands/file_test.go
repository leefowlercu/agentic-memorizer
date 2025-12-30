package subcommands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (appDir, memoryRoot string, cleanup func()) {
	t.Helper()

	// Create temp directories
	tmpDir := t.TempDir()
	appDir = filepath.Join(tmpDir, ".memorizer")
	memoryRoot = filepath.Join(appDir, "memory")

	err := os.MkdirAll(memoryRoot, 0755)
	require.NoError(t, err)

	// Set environment variable for config
	oldAppDir := os.Getenv("MEMORIZER_APP_DIR")
	os.Setenv("MEMORIZER_APP_DIR", appDir)

	// Create config file
	configPath := filepath.Join(appDir, "config.yaml")
	configContent := "memory:\n  root: " + memoryRoot + "\n"
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Reset config for testing
	config.ResetForTesting()

	cleanup = func() {
		os.Setenv("MEMORIZER_APP_DIR", oldAppDir)
		config.ResetForTesting()
	}

	return appDir, memoryRoot, cleanup
}

func TestValidateForgetFile_NonExistentPath(t *testing.T) {
	_, memoryRoot, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Reset flags
	fileDryRun = false

	cmd := FileCmd
	args := []string{filepath.Join(memoryRoot, "nonexistent.md")}

	err := validateForgetFile(cmd, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestValidateForgetFile_ValidFile(t *testing.T) {
	_, memoryRoot, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test file in memory directory
	testFile := filepath.Join(memoryRoot, "test.md")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Reset flags
	fileDryRun = false

	cmd := FileCmd
	args := []string{testFile}

	err = validateForgetFile(cmd, args)
	assert.NoError(t, err)
}

func TestValidateForgetFile_PathOutsideMemory(t *testing.T) {
	_, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test file outside memory directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "outside.md")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Reset flags
	fileDryRun = false

	cmd := FileCmd
	args := []string{testFile}

	err = validateForgetFile(cmd, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in memory directory")
}

func TestValidateForgetFile_ValidDirectory(t *testing.T) {
	_, memoryRoot, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test directory in memory
	testDir := filepath.Join(memoryRoot, "testdir")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(testDir, "test.md")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Reset flags
	fileDryRun = false

	cmd := FileCmd
	args := []string{testDir}

	err = validateForgetFile(cmd, args)
	assert.NoError(t, err)
}

func TestFileCmd_HasCorrectFlags(t *testing.T) {
	// Check that dry-run flag is defined
	dryRunFlag := FileCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag)
	assert.Equal(t, "bool", dryRunFlag.Value.Type())
}

func TestFileCmd_RequiresAtLeastOneArg(t *testing.T) {
	// FileCmd.Args is cobra.MinimumNArgs(1)
	assert.NotNil(t, FileCmd.Args)
}

func TestGetForgottenDir(t *testing.T) {
	// Test with custom app dir
	tmpDir := t.TempDir()
	oldAppDir := os.Getenv("MEMORIZER_APP_DIR")
	os.Setenv("MEMORIZER_APP_DIR", tmpDir)
	defer os.Setenv("MEMORIZER_APP_DIR", oldAppDir)

	forgottenDir, err := config.GetForgottenDir()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, ".forgotten"), forgottenDir)
}
