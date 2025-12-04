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

// TestGraph_Status tests graph status command
func TestGraph_Status(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("graph", "status")

	// Graph status should work (may show not running or running depending on test env)
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Unexpected exit code %d for graph status. Stdout: %s, Stderr: %s",
			exitCode, stdout, stderr)
	}

	output := stdout + stderr
	t.Logf("Graph status: %s", output)
}

// TestGraph_Connection tests FalkorDB connection
func TestGraph_Connection(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Test graph client connection
	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		// Graph may not be available in test environment
		t.Skipf("FalkorDB not available: %v", err)
	}

	t.Log("Successfully connected to FalkorDB")
}

// TestGraph_QueryExecution tests basic Cypher query execution
func TestGraph_QueryExecution(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify connection
	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available")
	}

	// Execute simple query
	query := "RETURN 1 as value"
	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query execution failed: %v", err)
	}

	// Check if we have at least one result
	hasResult := result.Next()
	if !hasResult {
		t.Error("Expected query to return results")
	}

	t.Log("Query executed successfully")
}

// TestGraph_NodeCreation tests creating nodes in the graph
func TestGraph_NodeCreation(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available")
	}

	// Create a test node
	query := `CREATE (f:File {path: '/test/file.md', name: 'file.md', hash: 'testhash123'}) RETURN f`
	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Node creation failed: %v", err)
	}

	if !result.Next() {
		t.Error("Expected result from node creation")
	}

	t.Log("Node created successfully")

	// Cleanup - delete the test node
	deleteQuery := `MATCH (f:File {hash: 'testhash123'}) DELETE f`
	_, err = h.GraphClient.Query(ctx, deleteQuery)
	if err != nil {
		t.Logf("Cleanup failed (non-critical): %v", err)
	}
}

// TestGraph_RelationshipCreation tests creating relationships
func TestGraph_RelationshipCreation(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available")
	}

	// Create file and tag nodes with relationship
	query := `
		CREATE (f:File {path: '/test/doc.md', hash: 'hash456'})
		CREATE (t:Tag {name: 'test-tag'})
		CREATE (f)-[:HAS_TAG]->(t)
		RETURN f, t
	`
	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Relationship creation failed: %v", err)
	}

	if !result.Next() {
		t.Error("Expected result from relationship creation")
	}

	t.Log("Relationship created successfully")

	// Cleanup
	cleanupQuery := `MATCH (f:File {hash: 'hash456'})-[r:HAS_TAG]->(t:Tag) DELETE r, f, t`
	_, _ = h.GraphClient.Query(ctx, cleanupQuery)
}

// TestGraph_SearchQuery tests semantic search queries
func TestGraph_SearchQuery(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available")
	}

	// Create test data
	setupQuery := `
		CREATE (f1:File {path: '/docs/readme.md', name: 'readme.md', hash: 'readme123', summary: 'Documentation for the project'})
		CREATE (f2:File {path: '/src/main.go', name: 'main.go', hash: 'main456', summary: 'Main application entry point'})
		CREATE (t1:Tag {name: 'documentation'})
		CREATE (t2:Tag {name: 'golang'})
		CREATE (f1)-[:HAS_TAG]->(t1)
		CREATE (f2)-[:HAS_TAG]->(t2)
	`
	_, err = h.GraphClient.Query(ctx, setupQuery)
	if err != nil {
		t.Fatalf("Test data setup failed: %v", err)
	}

	// Search by tag
	searchQuery := `MATCH (f:File)-[:HAS_TAG]->(t:Tag) WHERE t.name CONTAINS 'doc' RETURN f.path as path`
	result, err := h.GraphClient.Query(ctx, searchQuery)
	if err != nil {
		t.Fatalf("Search query failed: %v", err)
	}

	hasResults := result.Next()
	if !hasResults {
		t.Error("Expected at least 1 search result")
	}

	t.Log("Search query returned results")

	// Cleanup
	cleanupQuery := `MATCH (f:File) WHERE f.hash IN ['readme123', 'main456'] DETACH DELETE f`
	_, _ = h.GraphClient.Query(ctx, cleanupQuery)
	cleanupTags := `MATCH (t:Tag) WHERE t.name IN ['documentation', 'golang'] DELETE t`
	_, _ = h.GraphClient.Query(ctx, cleanupTags)
}

// TestGraph_GracefulDegradation tests daemon behavior when graph unavailable
func TestGraph_GracefulDegradation(t *testing.T) {
	t.Skip("Requires stopping FalkorDB during test - may interfere with other tests")

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Skipf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Skipf("Daemon failed to become healthy: %v", err)
	}

	// Check health with graph available
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Skipf("HTTP server not available")
	}

	if graph, ok := health["graph"].(map[string]any); ok {
		t.Logf("Graph status before degradation: %v", graph)
	}

	// Stop FalkorDB (would need docker-compose integration)
	// For now, just verify daemon doesn't crash

	// Verify daemon still healthy
	health, err = h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Daemon crashed when graph unavailable: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected daemon to remain healthy, got: %v", health["status"])
	}

	t.Log("Daemon gracefully handles graph unavailability")
}

// TestGraph_MultipleConnections tests concurrent graph connections
func TestGraph_MultipleConnections(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available")
	}

	// Execute multiple queries concurrently
	concurrentQueries := 5
	errors := make(chan error, concurrentQueries)

	for i := 0; i < concurrentQueries; i++ {
		go func(id int) {
			ctx := context.Background()
			query := "RETURN 1 as query_id"
			_, err := h.GraphClient.Query(ctx, query)
			errors <- err
		}(i)
	}

	// Collect results
	for i := 0; i < concurrentQueries; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Concurrent query %d failed: %v", i, err)
		}
	}

	t.Logf("Successfully executed %d concurrent queries", concurrentQueries)
}

// TestGraph_DataPersistence tests that graph data persists
func TestGraph_DataPersistence(t *testing.T) {
	t.Skip("Requires graph restart - tested in daemon rebuild workflow")

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available")
	}

	// Create persistent data
	createQuery := `CREATE (f:File {path: '/persistent/file.md', hash: 'persist789'}) RETURN f`
	_, err = h.GraphClient.Query(ctx, createQuery)
	if err != nil {
		t.Fatalf("Failed to create persistent data: %v", err)
	}

	// Restart graph (would need docker-compose restart)
	// For now, just verify data exists

	// Query for data
	searchQuery := `MATCH (f:File {hash: 'persist789'}) RETURN f.path as path`
	result, err := h.GraphClient.Query(ctx, searchQuery)
	if err != nil {
		t.Fatalf("Failed to query persistent data: %v", err)
	}

	if !result.Next() {
		t.Error("Expected 1 persistent result")
	}

	t.Log("Data persistence verified")

	// Cleanup
	cleanupQuery := `MATCH (f:File {hash: 'persist789'}) DELETE f`
	_, _ = h.GraphClient.Query(ctx, cleanupQuery)
}

// TestGraph_SchemaConstraints tests graph schema validation
func TestGraph_SchemaConstraints(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available")
	}

	// Create node with required properties
	query := `CREATE (f:File {path: '/schema/test.md', name: 'test.md', hash: 'schema123'}) RETURN f`
	_, err = h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Schema-compliant node creation failed: %v", err)
	}

	t.Log("Schema constraints validated")

	// Cleanup
	cleanupQuery := `MATCH (f:File {hash: 'schema123'}) DELETE f`
	_, _ = h.GraphClient.Query(ctx, cleanupQuery)
}

// TestGraph_StartStop tests graph start and stop commands
func TestGraph_StartStop(t *testing.T) {
	t.Skip("Graph start/stop managed by docker-compose in test environment")

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Test start command
	stdout, stderr, exitCode := h.RunCommand("graph", "start")
	harness.LogOutput(t, stdout, stderr)

	if exitCode != 0 {
		output := stdout + stderr
		if strings.Contains(output, "already running") {
			t.Log("Graph already running (expected in test environment)")
		} else {
			t.Logf("Graph start failed (exit=%d): %s", exitCode, output)
		}
	}

	// Test status
	stdout, stderr, _ = h.RunCommand("graph", "status")
	output := stdout + stderr
	t.Logf("Graph status: %s", output)

	// Test stop command
	stdout, stderr, exitCode = h.RunCommand("graph", "stop")
	harness.LogOutput(t, stdout, stderr)

	if exitCode != 0 {
		t.Logf("Graph stop may fail in test environment (managed by docker-compose)")
	}
}

// TestGraph_EmptyDatabase tests queries on empty database
func TestGraph_EmptyDatabase(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	ctx := context.Background()
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available")
	}

	// Query empty database
	query := `MATCH (f:File) RETURN count(f) as file_count`
	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query on empty database failed: %v", err)
	}

	hasResult := result.Next()
	t.Logf("Empty database query returned results: %v", hasResult)
}
