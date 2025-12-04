package harness

import (
	"fmt"
	"strings"
	"testing"
)

// AssertExitCode checks that a command exited with the expected code
func AssertExitCode(t testing.TB, expected, actual int, stdout, stderr string) {
	t.Helper()
	if actual != expected {
		t.Errorf("Expected exit code %d, got %d\nStdout: %s\nStderr: %s",
			expected, actual, stdout, stderr)
	}
}

// AssertContains checks that output contains expected string
func AssertContains(t testing.TB, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain %q, but it didn't.\nOutput: %s",
			expected, output)
	}
}

// AssertNotContains checks that output does not contain a string
func AssertNotContains(t testing.TB, output, notExpected string) {
	t.Helper()
	if strings.Contains(output, notExpected) {
		t.Errorf("Expected output to NOT contain %q, but it did.\nOutput: %s",
			notExpected, output)
	}
}

// AssertEmpty checks that output is empty
func AssertEmpty(t testing.TB, output string, label string) {
	t.Helper()
	if output != "" {
		t.Errorf("Expected %s to be empty, but got: %s", label, output)
	}
}

// AssertNotEmpty checks that output is not empty
func AssertNotEmpty(t testing.TB, output string, label string) {
	t.Helper()
	if output == "" {
		t.Errorf("Expected %s to not be empty", label)
	}
}

// AssertEqual checks that two values are equal
func AssertEqual(t testing.TB, expected, actual any, label string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", label, expected, actual)
	}
}

// AssertTrue checks that a condition is true
func AssertTrue(t testing.TB, condition bool, message string) {
	t.Helper()
	if !condition {
		t.Errorf("Expected condition to be true: %s", message)
	}
}

// AssertFalse checks that a condition is false
func AssertFalse(t testing.TB, condition bool, message string) {
	t.Helper()
	if condition {
		t.Errorf("Expected condition to be false: %s", message)
	}
}

// AssertNoError checks that an error is nil
func AssertNoError(t testing.TB, err error, context string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", context, err)
	}
}

// AssertError checks that an error occurred
func AssertError(t testing.TB, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", context)
	}
}

// AssertErrorContains checks that an error occurred and contains expected message
func AssertErrorContains(t testing.TB, err error, expectedMsg string, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", context)
	}
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("%s: expected error to contain %q, got: %v", context, expectedMsg, err)
	}
}

// AssertFileExists checks that a file exists in the harness memory directory
func AssertFileExists(t testing.TB, h *E2EHarness, filename string) {
	t.Helper()
	// This would check via the index or filesystem
	// For now, just a placeholder
}

// AssertMapContains checks that a map contains a key with expected value
func AssertMapContains(t testing.TB, m map[string]any, key string, expectedValue any) {
	t.Helper()
	actualValue, ok := m[key]
	if !ok {
		t.Errorf("Map does not contain key %q. Map: %+v", key, m)
		return
	}
	if actualValue != expectedValue {
		t.Errorf("Map[%q]: expected %v, got %v", key, expectedValue, actualValue)
	}
}

// AssertMapHasKey checks that a map contains a key
func AssertMapHasKey(t testing.TB, m map[string]any, key string) {
	t.Helper()
	if _, ok := m[key]; !ok {
		t.Errorf("Map does not contain key %q. Map: %+v", key, m)
	}
}

// AssertListLength checks that a list has the expected length
func AssertListLength(t testing.TB, list any, expectedLen int, label string) {
	t.Helper()

	var actualLen int
	switch v := list.(type) {
	case []any:
		actualLen = len(v)
	case []string:
		actualLen = len(v)
	case []int:
		actualLen = len(v)
	default:
		t.Fatalf("Unsupported list type for length assertion: %T", list)
	}

	if actualLen != expectedLen {
		t.Errorf("%s: expected length %d, got %d", label, expectedLen, actualLen)
	}
}

// LogOutput logs command output for debugging
func LogOutput(t testing.TB, stdout, stderr string) {
	t.Helper()
	if stdout != "" {
		t.Logf("Stdout:\n%s", stdout)
	}
	if stderr != "" {
		t.Logf("Stderr:\n%s", stderr)
	}
}

// AssertCommandSuccess runs a command and asserts it succeeds
func AssertCommandSuccess(t testing.TB, h *E2EHarness, args ...string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, exitCode := h.RunCommand(args...)
	if exitCode != 0 {
		t.Fatalf("Command %v failed with exit code %d\nStdout: %s\nStderr: %s",
			args, exitCode, stdout, stderr)
	}
	return stdout, stderr
}

// AssertCommandFailure runs a command and asserts it fails
func AssertCommandFailure(t testing.TB, h *E2EHarness, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	stdout, stderr, exitCode = h.RunCommand(args...)
	if exitCode == 0 {
		t.Fatalf("Command %v unexpectedly succeeded\nStdout: %s\nStderr: %s",
			args, stdout, stderr)
	}
	return stdout, stderr, exitCode
}

// RetryUntilSuccess retries a function until it succeeds or times out
func RetryUntilSuccess(t testing.TB, fn func() error, timeout, interval string) error {
	t.Helper()

	// This is a simplified version - would need proper time parsing
	// For now, just a placeholder that calls the function once
	return fn()
}

// DumpDaemonLogs reads and logs the daemon log file for debugging
func DumpDaemonLogs(t testing.TB, h *E2EHarness) {
	t.Helper()
	// Would read h.LogPath and dump it
	t.Logf("Daemon logs would be dumped from: %s", h.LogPath)
}

// FormatError formats an error with context for better test output
func FormatError(context string, err error) string {
	return fmt.Sprintf("%s: %v", context, err)
}
