//go:build e2e

package tests

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestAPIFacts_GetFactsIndex tests GET /api/v1/facts/index
func TestAPIFacts_GetFactsIndex(t *testing.T) {
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

	// Enable HTTP server
	if err := h.EnableHTTPServer(8081); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Create some facts via CLI
	facts := []string{
		"This is the first test fact for API testing",
		"This is the second test fact for API testing",
		"This is the third test fact for API testing",
	}
	for _, fact := range facts {
		stdout, stderr, exitCode := h.RunCommand("remember", "fact", fact)
		harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	}

	// Start daemon
	daemonCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(daemonCtx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for daemon to initialize
	time.Sleep(2 * time.Second)

	// Get facts index via API
	result, err := h.HTTPClient.GetFactsIndex()
	if err != nil {
		t.Fatalf("GetFactsIndex failed: %v", err)
	}

	// Verify response structure
	if _, ok := result["facts"]; !ok {
		t.Error("Response missing 'facts' field")
	}
	if _, ok := result["count"]; !ok {
		t.Error("Response missing 'count' field")
	}
	if _, ok := result["stats"]; !ok {
		t.Error("Response missing 'stats' field")
	}

	// Verify facts count
	count, ok := result["count"].(float64)
	if !ok {
		t.Fatalf("Count is not a number: %T", result["count"])
	}
	if int(count) != len(facts) {
		t.Errorf("Expected %d facts, got %d", len(facts), int(count))
	}

	t.Logf("Facts index response: %+v", result)
}

// TestAPIFacts_GetFactsIndexEmpty tests GET /api/v1/facts/index with no facts
func TestAPIFacts_GetFactsIndexEmpty(t *testing.T) {
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

	// Enable HTTP server
	if err := h.EnableHTTPServer(8082); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon with no facts
	daemonCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(daemonCtx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for daemon to initialize
	time.Sleep(2 * time.Second)

	// Get facts index via API (should be empty)
	result, err := h.HTTPClient.GetFactsIndex()
	if err != nil {
		t.Fatalf("GetFactsIndex failed: %v", err)
	}

	// Verify count is 0
	count, ok := result["count"].(float64)
	if !ok {
		t.Fatalf("Count is not a number: %T", result["count"])
	}
	if int(count) != 0 {
		t.Errorf("Expected 0 facts, got %d", int(count))
	}

	t.Logf("Empty facts index response: %+v", result)
}

// TestAPIFacts_GetFact tests GET /api/v1/facts/{id}
func TestAPIFacts_GetFact(t *testing.T) {
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

	// Enable HTTP server
	if err := h.EnableHTTPServer(8083); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Create a fact via CLI
	factContent := "This is a specific fact for GET by ID testing"
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", factContent)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Extract fact ID
	factID := extractFactID(stdout)
	if factID == "" {
		t.Skip("Could not extract fact ID from creation output")
	}

	// Start daemon
	daemonCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(daemonCtx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for daemon to initialize
	time.Sleep(2 * time.Second)

	// Get fact by ID via API
	result, err := h.HTTPClient.GetFact(factID)
	if err != nil {
		t.Fatalf("GetFact failed: %v", err)
	}

	// Verify response contains the fact
	if _, ok := result["fact"]; !ok {
		t.Error("Response missing 'fact' field")
	}

	fact, ok := result["fact"].(map[string]any)
	if !ok {
		t.Fatalf("Fact is not a map: %T", result["fact"])
	}

	// Verify content matches
	if content, ok := fact["content"].(string); ok {
		if content != factContent {
			t.Errorf("Fact content mismatch; got %q, want %q", content, factContent)
		}
	} else {
		t.Error("Fact missing 'content' field")
	}

	// Verify ID matches
	if id, ok := fact["id"].(string); ok {
		if id != factID {
			t.Errorf("Fact ID mismatch; got %q, want %q", id, factID)
		}
	} else {
		t.Error("Fact missing 'id' field")
	}

	t.Logf("Get fact response: %+v", result)
}

// TestAPIFacts_GetFactNotFound tests GET /api/v1/facts/{id} for non-existent fact
func TestAPIFacts_GetFactNotFound(t *testing.T) {
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

	// Enable HTTP server
	if err := h.EnableHTTPServer(8084); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	daemonCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(daemonCtx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for daemon to initialize
	time.Sleep(2 * time.Second)

	// Try to get non-existent fact
	nonExistentID := "00000000-0000-0000-0000-000000000000"
	_, err := h.HTTPClient.GetFact(nonExistentID)
	if err == nil {
		t.Error("Expected error for non-existent fact, got nil")
	}

	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 404/not found error, got: %v", err)
	}
}

// TestAPIFacts_FactLifecycle tests the full lifecycle: create, get, delete via API
func TestAPIFacts_FactLifecycle(t *testing.T) {
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

	// Enable HTTP server
	if err := h.EnableHTTPServer(8085); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon first (to have HTTP API available)
	daemonCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(daemonCtx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for daemon to initialize
	time.Sleep(2 * time.Second)

	// Step 1: Verify no facts initially
	result, err := h.HTTPClient.GetFactsIndex()
	if err != nil {
		t.Fatalf("GetFactsIndex failed: %v", err)
	}
	initialCount := int(result["count"].(float64))
	t.Logf("Initial fact count: %d", initialCount)

	// Step 2: Create fact via CLI
	factContent := "This is a lifecycle test fact content"
	stdout, stderr, exitCode := h.RunCommand("remember", "fact", factContent)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	factID := extractFactID(stdout)
	if factID == "" {
		t.Skip("Could not extract fact ID from creation output")
	}
	t.Logf("Created fact with ID: %s", factID)

	// Step 3: Verify fact appears in index via API
	time.Sleep(500 * time.Millisecond) // Brief wait for graph update
	result, err = h.HTTPClient.GetFactsIndex()
	if err != nil {
		t.Fatalf("GetFactsIndex failed after create: %v", err)
	}
	afterCreateCount := int(result["count"].(float64))
	if afterCreateCount != initialCount+1 {
		t.Errorf("Expected fact count to increase by 1, got %d (was %d)", afterCreateCount, initialCount)
	}

	// Step 4: Get specific fact via API
	factResult, err := h.HTTPClient.GetFact(factID)
	if err != nil {
		t.Fatalf("GetFact failed: %v", err)
	}
	fact := factResult["fact"].(map[string]any)
	if fact["content"].(string) != factContent {
		t.Errorf("Fact content mismatch")
	}

	// Step 5: Delete fact via CLI
	stdout, stderr, exitCode = h.RunCommand("forget", "fact", factID)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "deleted")

	// Step 6: Verify fact is gone from index via API
	time.Sleep(500 * time.Millisecond) // Brief wait for graph update
	result, err = h.HTTPClient.GetFactsIndex()
	if err != nil {
		t.Fatalf("GetFactsIndex failed after delete: %v", err)
	}
	afterDeleteCount := int(result["count"].(float64))
	if afterDeleteCount != initialCount {
		t.Errorf("Expected fact count to return to %d, got %d", initialCount, afterDeleteCount)
	}

	// Step 7: Verify get by ID returns 404
	_, err = h.HTTPClient.GetFact(factID)
	if err == nil {
		t.Error("Expected error for deleted fact, got nil")
	}

	t.Log("Fact lifecycle test completed successfully")
}
