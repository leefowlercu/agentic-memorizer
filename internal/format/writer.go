package format

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Writer provides buffered output with error handling
type Writer interface {
	// Write writes content without a newline
	Write(content string) error

	// WriteLine writes content with a newline
	WriteLine(content string) error

	// Flush flushes any buffered data
	Flush() error

	// Close flushes and closes the writer
	Close() error
}

// BufferedWriter implements Writer with buffered I/O
type BufferedWriter struct {
	writer *bufio.Writer
	output io.Writer
}

// NewBufferedWriter creates a new buffered writer
func NewBufferedWriter(w io.Writer) *BufferedWriter {
	return &BufferedWriter{
		writer: bufio.NewWriter(w),
		output: w,
	}
}

// NewStdoutWriter creates a buffered writer for stdout
func NewStdoutWriter() *BufferedWriter {
	return NewBufferedWriter(os.Stdout)
}

// NewFileWriter creates a buffered writer for a file
func NewFileWriter(path string) (*BufferedWriter, error) {
	// Clean the path
	cleanPath := filepath.Clean(path)

	// Ensure parent directory exists
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory; %w", err)
	}

	// Create the file with 0644 permissions
	file, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create file; %w", err)
	}

	return &BufferedWriter{
		writer: bufio.NewWriter(file),
		output: file,
	}, nil
}

// Write writes content without a newline
func (w *BufferedWriter) Write(content string) error {
	_, err := w.writer.WriteString(content)
	if err != nil {
		return fmt.Errorf("write failed; %w", err)
	}
	return nil
}

// WriteLine writes content with a newline
func (w *BufferedWriter) WriteLine(content string) error {
	if err := w.Write(content); err != nil {
		return err
	}
	if err := w.Write("\n"); err != nil {
		return err
	}
	return nil
}

// Flush flushes any buffered data
func (w *BufferedWriter) Flush() error {
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("flush failed; %w", err)
	}
	return nil
}

// Close flushes and closes the writer
func (w *BufferedWriter) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}

	// Close the underlying writer if it's a closer
	if closer, ok := w.output.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("close failed; %w", err)
		}
	}

	return nil
}
