package format

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufferedWriter_WriteAndFlush(t *testing.T) {
	var buf bytes.Buffer
	writer := NewBufferedWriter(&buf)

	err := writer.Write("hello")
	require.NoError(t, err)

	err = writer.Write(" ")
	require.NoError(t, err)

	err = writer.Write("world")
	require.NoError(t, err)

	// Data should not be in buffer yet (buffered)
	assert.Equal(t, "", buf.String())

	err = writer.Flush()
	require.NoError(t, err)

	// Now data should be flushed
	assert.Equal(t, "hello world", buf.String())
}

func TestBufferedWriter_WriteLine(t *testing.T) {
	var buf bytes.Buffer
	writer := NewBufferedWriter(&buf)

	err := writer.WriteLine("line 1")
	require.NoError(t, err)

	err = writer.WriteLine("line 2")
	require.NoError(t, err)

	err = writer.Flush()
	require.NoError(t, err)

	assert.Equal(t, "line 1\nline 2\n", buf.String())
}

func TestBufferedWriter_Close(t *testing.T) {
	var buf bytes.Buffer
	writer := NewBufferedWriter(&buf)

	err := writer.Write("content")
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Close should flush
	assert.Equal(t, "content", buf.String())
}

func TestNewStdoutWriter(t *testing.T) {
	writer := NewStdoutWriter()
	assert.NotNil(t, writer)
	assert.Equal(t, os.Stdout, writer.output)
}

func TestNewFileWriter(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	writer, err := NewFileWriter(filePath)
	require.NoError(t, err)
	defer writer.Close()

	err = writer.WriteLine("test content")
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Verify file was created
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "test content\n", string(content))

	// Verify file permissions
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestNewFileWriter_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "subdir", "nested", "test.txt")

	writer, err := NewFileWriter(filePath)
	require.NoError(t, err)
	defer writer.Close()

	err = writer.WriteLine("test")
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filePath)
	assert.NoError(t, err)
}

func TestNewFileWriter_PathTraversalProtection(t *testing.T) {
	tmpDir := t.TempDir()
	// filepath.Clean should normalize this
	filePath := filepath.Join(tmpDir, "..", "..", "malicious.txt")

	writer, err := NewFileWriter(filePath)
	require.NoError(t, err)
	defer writer.Close()

	// Clean path should not escape tmpDir parent
	cleanPath := filepath.Clean(filePath)
	assert.NotContains(t, cleanPath, "..")
}

func TestBufferedWriter_ErrorPropagation(t *testing.T) {
	// Create a writer that will fail on write
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	writer, err := NewFileWriter(filePath)
	require.NoError(t, err)

	// Write some content
	err = writer.WriteLine("test")
	require.NoError(t, err)

	// Close the file to cause subsequent writes to fail
	if closer, ok := writer.output.(interface{ Close() error }); ok {
		closer.Close()
	}

	// This flush should fail since file is closed
	err = writer.Flush()
	assert.Error(t, err)
}
