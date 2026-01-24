package steps

import (
	"net"
	"strconv"
	"testing"
)

func TestCheckPortInUse_PortInUse(t *testing.T) {
	// Start a listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer listener.Close()

	// Get the port that was assigned
	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	// Check that our function detects it as in use
	if !CheckPortInUse(port) {
		t.Errorf("expected port %d to be detected as in use", port)
	}
}

func TestCheckPortInUse_PortNotInUse(t *testing.T) {
	// Find a port that's not in use by trying to listen and immediately closing
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port
	listener.Close()

	// Now the port should be free
	if CheckPortInUse(port) {
		t.Errorf("expected port %d to be detected as not in use", port)
	}
}

func TestFormatWarning(t *testing.T) {
	result := FormatWarning("test warning")
	if result == "" {
		t.Error("expected non-empty warning string")
	}
	if !containsStr(result, "test warning") {
		t.Error("expected warning to contain the message")
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "with error",
			err:      strconv.ErrSyntax,
			expected: "Error:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatError(tt.err)
			if tt.expected == "" && result != "" {
				t.Errorf("expected empty string, got %q", result)
			}
			if tt.expected != "" && !containsStr(result, tt.expected) {
				t.Errorf("expected result to contain %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatSuccess(t *testing.T) {
	result := FormatSuccess("operation completed")
	if result == "" {
		t.Error("expected non-empty success string")
	}
	if !containsStr(result, "operation completed") {
		t.Error("expected success message to contain the text")
	}
}

// containsStr is a simple string contains check for tests.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
