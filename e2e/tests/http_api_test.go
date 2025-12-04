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

// TestHTTPAPI_Search tests the semantic search endpoint
func TestHTTPAPI_Search(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test files
	if err := h.AddMemoryFile("search-test1.md", "# Test Document\n\nThis document contains search terms."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("search-test2.md", "# Another Document\n\nDifferent content here."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Test search
	result, err := h.HTTPClient.SearchFiles("document", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Search results: %+v", result)
}

// TestHTTPAPI_SearchEmpty tests search with no results
func TestHTTPAPI_SearchEmpty(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon with no files
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Test search on empty index
	result, err := h.HTTPClient.SearchFiles("nonexistent", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Empty search results: %+v", result)
}

// TestHTTPAPI_GetFile tests file metadata retrieval
func TestHTTPAPI_GetFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test file
	testFile := "api-test.md"
	if err := h.AddMemoryFile(testFile, "# API Test\n\nTest content for metadata."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Get file metadata
	metadata, err := h.HTTPClient.GetFileMetadata(testFile)
	if err != nil {
		t.Fatalf("GetFileMetadata failed: %v", err)
	}

	t.Logf("File metadata: %+v", metadata)
}

// TestHTTPAPI_GetFileNotFound tests 404 handling
func TestHTTPAPI_GetFileNotFound(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Try to get non-existent file
	_, err := h.HTTPClient.GetFileMetadata("nonexistent.md")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "not found") {
		t.Logf("Error message: %v", err)
	}
}

// TestHTTPAPI_RecentFiles tests recent files endpoint
func TestHTTPAPI_RecentFiles(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test files
	for i := 1; i <= 3; i++ {
		filename := "recent-" + string(rune('0'+i)) + ".md"
		content := "# Recent File " + string(rune('0'+i)) + "\n\nContent."
		if err := h.AddMemoryFile(filename, content); err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}
		time.Sleep(100 * time.Millisecond) // Ensure different timestamps
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// List recent files
	files, err := h.HTTPClient.ListRecentFiles(7, 10)
	if err != nil {
		t.Fatalf("ListRecentFiles failed: %v", err)
	}

	t.Logf("Recent files: %+v", files)
}

// TestHTTPAPI_RelatedFiles tests related files endpoint
func TestHTTPAPI_RelatedFiles(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add related files with similar content
	if err := h.AddMemoryFile("related1.md", "# Document 1\n\nContent about FalkorDB and graphs."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("related2.md", "# Document 2\n\nAlso discusses FalkorDB and graphs."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Get related files
	related, err := h.HTTPClient.GetRelatedFiles("related1.md", 5)
	if err != nil {
		t.Fatalf("GetRelatedFiles failed: %v", err)
	}

	t.Logf("Related files: %+v", related)
}

// TestHTTPAPI_EntitySearch tests entity search endpoint
func TestHTTPAPI_EntitySearch(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add files mentioning an entity
	if err := h.AddMemoryFile("entity1.md", "# Entity Test\n\nThis mentions FalkorDB extensively."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("entity2.md", "# Another Test\n\nFalkorDB is a graph database."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Search for entity
	result, err := h.HTTPClient.SearchEntities("FalkorDB", 10)
	if err != nil {
		t.Fatalf("SearchEntities failed: %v", err)
	}

	t.Logf("Entity search results: %+v", result)
}

// TestHTTPAPI_Rebuild tests rebuild endpoint
func TestHTTPAPI_Rebuild(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test file
	if err := h.AddMemoryFile("rebuild-test.md", "# Rebuild Test\n\nTest content."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Wait for initial indexing
	time.Sleep(5 * time.Second)

	// Trigger rebuild
	if err := h.HTTPClient.TriggerRebuild(false); err != nil {
		t.Fatalf("TriggerRebuild failed: %v", err)
	}

	t.Log("Rebuild triggered successfully")
}

// TestHTTPAPI_RebuildForce tests rebuild with force flag
func TestHTTPAPI_RebuildForce(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test file
	if err := h.AddMemoryFile("force-rebuild.md", "# Force Rebuild\n\nTest content."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Wait for initial indexing
	time.Sleep(5 * time.Second)

	// Trigger rebuild with force
	if err := h.HTTPClient.TriggerRebuild(true); err != nil {
		t.Fatalf("TriggerRebuild (force) failed: %v", err)
	}

	t.Log("Force rebuild triggered successfully")
}

// TestHTTPAPI_HealthWithGraph tests health endpoint with graph metrics
func TestHTTPAPI_HealthWithGraph(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Get health with graph metrics
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	// Verify expected fields
	if _, ok := health["status"]; !ok {
		t.Error("Health response missing 'status' field")
	}

	if graph, ok := health["graph"].(map[string]any); ok {
		if _, ok := graph["connected"]; !ok {
			t.Error("Graph health missing 'connected' field")
		}
		t.Logf("Graph status: %+v", graph)
	}

	t.Logf("Full health response: %+v", health)
}

// TestHTTPAPI_GetIndex tests the full index retrieval endpoint
func TestHTTPAPI_GetIndex(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test files
	if err := h.AddMemoryFile("index-test1.md", "# Index Test 1\n\nContent for index test."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("index-test2.md", "# Index Test 2\n\nMore content."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
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

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Get full index
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("GetIndex failed: %v", err)
	}

	t.Logf("Index: %+v", index)
}
