package transport

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"sync"
)

type StdioTransport struct {
	stdin  *bufio.Reader
	stdout io.Writer
	mu     sync.Mutex
}

func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		stdin:  bufio.NewReader(os.Stdin),
		stdout: os.Stdout,
	}
}

// Read reads a single JSON-RPC message from stdin
func (t *StdioTransport) Read() ([]byte, error) {
	// Read line-delimited JSON
	line, err := t.stdin.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	// Trim whitespace (including newline)
	trimmed := bytes.TrimSpace(line)
	// Skip empty lines
	if len(trimmed) == 0 {
		return t.Read() // Recursive call to get next non-empty line
	}
	return trimmed, nil
}

// Write writes a JSON-RPC message to stdout
func (t *StdioTransport) Write(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, err := t.stdout.Write(data)
	if err != nil {
		return err
	}

	// Ensure newline termination
	if len(data) > 0 && data[len(data)-1] != '\n' {
		_, err = t.stdout.Write([]byte{'\n'})
	}

	return err
}

// Close implements graceful shutdown
func (t *StdioTransport) Close() error {
	// Stdio doesn't need explicit close
	return nil
}
