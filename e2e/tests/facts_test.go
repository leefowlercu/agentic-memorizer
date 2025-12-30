//go:build e2e

package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestFacts_RememberFact tests successful fact creation
func TestFacts_RememberFact(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Remember a fact
	factContent := "This is a test fact that should be remembered"
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", factContent)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	// Output contains checkmark and ID
	harness.AssertContains(t, stdout, "Fact created")
	harness.AssertContains(t, stdout, "ID:")
}

// TestFacts_RememberFactContentTooShort tests validation for short content
func TestFacts_RememberFactContentTooShort(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try to remember a fact with content too short (< 10 characters)
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", "short")

	// Should fail with validation error
	if exitCode == 0 {
		t.Error("Expected command to fail for short content")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "at least 10 characters")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestFacts_RememberFactContentTooLong tests validation for long content
func TestFacts_RememberFactContentTooLong(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create content longer than 500 characters
	longContent := strings.Repeat("This is a very long fact. ", 25) // ~650 characters

	stdout, stderr, exitCode := h.RunCommand("remember", "fact", longContent)

	// Should fail with validation error
	if exitCode == 0 {
		t.Error("Expected command to fail for content too long")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "500 characters")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestFacts_RememberFactDuplicate tests duplicate content detection
func TestFacts_RememberFactDuplicate(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	factContent := "This is a unique fact for duplicate testing"

	// Create first fact
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", factContent)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Try to create duplicate
	stdout, stderr, exitCode = h.RunCommand("remember", "fact", factContent)

	// Should fail with duplicate error
	if exitCode == 0 {
		t.Error("Expected command to fail for duplicate content")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "already exists")
}

// TestFacts_RememberFactUpdate tests updating an existing fact
func TestFacts_RememberFactUpdate(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Create initial fact
	initialContent := "Initial fact content for update test"
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", initialContent)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Extract ID from creation output (format: "✓ Fact created with ID: <uuid>")
	factID := extractFactID(stdout)
	if factID == "" {
		t.Skip("Could not extract fact ID from creation output")
	}

	// Update the fact
	updatedContent := "Updated fact content for testing purposes"
	stdout, stderr, exitCode = h.RunCommand("remember", "fact", updatedContent, "--id", factID)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "updated")
}

// extractFactID extracts the UUID from command output
func extractFactID(output string) string {
	// Look for "ID: <uuid>" pattern
	idIndex := strings.Index(output, "ID: ")
	if idIndex == -1 {
		return ""
	}
	idStart := idIndex + 4
	// Find end of UUID (next whitespace or newline)
	remaining := output[idStart:]
	idEnd := strings.IndexAny(remaining, " \n\t")
	if idEnd == -1 {
		idEnd = len(remaining)
	}
	return strings.TrimSpace(remaining[:idEnd])
}

// TestFacts_RememberFactUpdateNonExistent tests updating a non-existent fact
func TestFacts_RememberFactUpdateNonExistent(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Try to update a non-existent fact
	nonExistentID := "00000000-0000-0000-0000-000000000000"
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", "Updated content", "--id", nonExistentID)

	// Should fail
	if exitCode == 0 {
		t.Error("Expected command to fail for non-existent fact ID")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "not found")
}

// TestFacts_RememberFactInvalidUUID tests validation for invalid UUID format
func TestFacts_RememberFactInvalidUUID(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try to update with invalid UUID format
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", "Some content here", "--id", "not-a-valid-uuid")

	// Should fail with validation error
	if exitCode == 0 {
		t.Error("Expected command to fail for invalid UUID")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "invalid")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestFacts_ReadFactsEmpty tests reading facts when none exist
func TestFacts_ReadFactsEmpty(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Read facts (should be empty in fresh test environment)
	stdout, stderr, exitCode := h.RunCommand("read", "facts")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	// Should indicate no facts or show empty list
	output := stdout + stderr
	t.Logf("Read facts (empty) output: %s", output)
}

// TestFacts_ReadFactsList tests reading all facts
func TestFacts_ReadFactsList(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Create some facts
	facts := []string{
		"First fact for reading test purposes",
		"Second fact for reading test purposes",
		"Third fact for reading test purposes",
	}

	for _, fact := range facts {
		stdout, stderr, exitCode := h.RunCommand("remember", "fact", fact)
		harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	}

	// Read all facts
	stdout, stderr, exitCode := h.RunCommand("read", "facts")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Verify facts are in output
	for _, fact := range facts {
		harness.AssertContains(t, stdout, fact)
	}
}

// TestFacts_ReadFactsFormats tests reading facts in different formats
func TestFacts_ReadFactsFormats(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Create a fact
	factContent := "Test fact for format testing purposes"
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", factContent)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	tests := []struct {
		name     string
		format   string
		contains string
	}{
		{"JSON", "json", `"content"`},
		{"XML", "xml", "<facts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := h.RunCommand("read", "facts", "--format", tt.format)
			harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
			harness.AssertContains(t, stdout, tt.contains)
		})
	}
}

// TestFacts_ForgetFact tests successful fact deletion
func TestFacts_ForgetFact(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Create a fact
	factContent := "Fact to be forgotten in testing"
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", factContent)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Extract ID from creation output
	factID := extractFactID(stdout)
	if factID == "" {
		t.Skip("Could not extract fact ID from creation output")
	}

	// Forget the fact
	stdout, stderr, exitCode = h.RunCommand("forget", "fact", factID)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "deleted")

	// Verify fact is gone
	stdout, stderr, exitCode = h.RunCommand("read", "facts", "--format", "json")
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	if strings.Contains(stdout, factContent) {
		t.Error("Fact should have been deleted but still appears in output")
	}
}

// TestFacts_ForgetFactNonExistent tests forgetting a non-existent fact
func TestFacts_ForgetFactNonExistent(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify graph is available
	ctx := context.Background()
	if _, err := h.GraphClient.Query(ctx, "RETURN 1"); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Try to forget a non-existent fact
	nonExistentID := "00000000-0000-0000-0000-000000000000"
	stdout, stderr, exitCode := h.RunCommand("forget", "fact", nonExistentID)

	// Should fail
	if exitCode == 0 {
		t.Error("Expected command to fail for non-existent fact")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "not found")
}

// TestFacts_ForgetFactInvalidUUID tests validation for invalid UUID format
func TestFacts_ForgetFactInvalidUUID(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try to forget with invalid UUID format
	stdout, stderr, exitCode := h.RunCommand("forget", "fact", "not-a-valid-uuid")

	// Should fail with validation error
	if exitCode == 0 {
		t.Error("Expected command to fail for invalid UUID")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "invalid")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestFacts_GraphUnavailable tests commands when graph is not running
func TestFacts_GraphUnavailable(t *testing.T) {
	t.Skip("Requires stopping FalkorDB during test - may interfere with other tests")

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Configure to use a port where no FalkorDB is running
	// (Would need to modify harness to support this)

	// Test remember fact
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", "Test fact content")
	if exitCode == 0 {
		t.Error("Expected command to fail when graph unavailable")
	}
	output := stdout + stderr
	if !strings.Contains(output, "FalkorDB") && !strings.Contains(output, "graph") {
		t.Errorf("Expected error message about graph, got: %s", output)
	}

	// Test read facts
	stdout, stderr, exitCode = h.RunCommand("read", "facts")
	if exitCode == 0 {
		t.Error("Expected command to fail when graph unavailable")
	}

	// Test forget fact
	stdout, stderr, exitCode = h.RunCommand("forget", "fact", "00000000-0000-0000-0000-000000000000")
	if exitCode == 0 {
		t.Error("Expected command to fail when graph unavailable")
	}
}

// TestFacts_CommandHelp tests help output for facts commands
func TestFacts_CommandHelp(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	tests := []struct {
		name string
		args []string
	}{
		{"remember help", []string{"remember", "--help"}},
		{"remember fact help", []string{"remember", "fact", "--help"}},
		{"read facts help", []string{"read", "facts", "--help"}},
		{"forget help", []string{"forget", "--help"}},
		{"forget fact help", []string{"forget", "fact", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := h.RunCommand(tt.args...)
			harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
			harness.AssertContains(t, stdout, "Usage:")
		})
	}
}

// TestFacts_ReadFilesStillWorks tests that read files command still works after refactoring
func TestFacts_ReadFilesStillWorks(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add a test file to memory
	if err := h.AddMemoryFile("test.md", "# Test File\n\nThis is a test."); err != nil {
		t.Fatalf("Failed to add memory file: %v", err)
	}

	// Verify read files command works
	stdout, stderr, _ := h.RunCommand("read", "files")

	// May fail if no index exists yet, but should not have syntax errors
	output := stdout + stderr
	if strings.Contains(output, "unknown flag") || strings.Contains(output, "unknown command") {
		t.Errorf("Unexpected command error: %s", output)
	}
}
