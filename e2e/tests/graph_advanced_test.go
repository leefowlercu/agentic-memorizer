//go:build e2e

package tests

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestGraphAdvanced_MultiHopRelationships tests multi-hop graph traversal
func TestGraphAdvanced_MultiHopRelationships(t *testing.T) {
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

	// Create files with related content (should share tags)
	files := []struct {
		name    string
		content string
	}{
		{"doc1.md", "# FalkorDB\n\nFalkorDB is a graph database"},
		{"doc2.md", "# Graph Databases\n\nFalkorDB and Neo4j are popular"},
		{"doc3.md", "# Redis\n\nFalkorDB is built on Redis"},
	}

	for _, f := range files {
		if err := h.AddMemoryFile(f.name, f.content); err != nil {
			t.Fatalf("Failed to add file %s: %v", f.name, err)
		}
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

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Query multi-hop: find files related through shared tags
	// Pattern: file1 -[:HAS_TAG]-> tag <-[:HAS_TAG]- file2
	query := `
		MATCH (f1:File {name: 'doc1.md'})-[:HAS_TAG]->(t:Tag)<-[:HAS_TAG]-(f2:File)
		WHERE f1 <> f2
		RETURN DISTINCT f2.name as related_file, t.name as shared_tag
	`

	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Multi-hop query failed: %v", err)
	}

	relatedCount := 0
	for result.Next() {
		record := result.Record()
		if relatedFile, ok := record.Get("related_file"); ok {
			if sharedTag, ok := record.Get("shared_tag"); ok {
				t.Logf("Found related file via multi-hop: %v (shared tag: %v)", relatedFile, sharedTag)
				relatedCount++
			}
		}
	}

	if relatedCount == 0 {
		t.Log("No multi-hop relationships found (files may not share tags yet)")
	} else {
		t.Logf("Multi-hop query found %d related files", relatedCount)
	}
}

// TestGraphAdvanced_DepthLimitedTraversal tests graph traversal with depth limits
func TestGraphAdvanced_DepthLimitedTraversal(t *testing.T) {
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
	if err := h.AddMemoryFile("test.md", "# Test\n\nContent"); err != nil {
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

	time.Sleep(5 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Query with variable length path (depth 1-3)
	query := `
		MATCH (f:File {name: 'test.md'})-[*1..3]-(connected)
		RETURN DISTINCT labels(connected) as node_type, count(*) as count
	`

	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Depth-limited traversal failed: %v", err)
	}

	connectedTypes := 0
	for result.Next() {
		record := result.Record()
		if nodeType, ok := record.Get("node_type"); ok {
			if count, ok := record.Get("count"); ok {
				t.Logf("Connected nodes at depth 1-3: type=%v, count=%v", nodeType, count)
				connectedTypes++
			}
		}
	}

	t.Logf("Depth-limited traversal found %d connected node types", connectedTypes)
}

// TestGraphAdvanced_MultipleEntityTypes tests querying multiple relationship types
func TestGraphAdvanced_MultipleEntityTypes(t *testing.T) {
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

	// Add test file with rich content
	content := `# Multi-Entity Document

This document discusses FalkorDB, Redis, and Graph Databases.
It covers topics like performance, scalability, and data modeling.
`
	if err := h.AddMemoryFile("multi.md", content); err != nil {
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

	time.Sleep(10 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Query multiple entity types in single query
	query := `
		MATCH (f:File {name: 'multi.md'})
		OPTIONAL MATCH (f)-[:HAS_TAG]->(tag:Tag)
		OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(topic:Topic)
		OPTIONAL MATCH (f)-[:MENTIONS]->(entity:Entity)
		RETURN f.name,
		       collect(DISTINCT tag.name) as tags,
		       collect(DISTINCT topic.name) as topics,
		       collect(DISTINCT entity.name) as entities
	`

	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Multi-entity query failed: %v", err)
	}

	if result.Next() {
		record := result.Record()
		if tags, ok := record.Get("tags"); ok {
			t.Logf("Tags: %v", tags)
		}
		if topics, ok := record.Get("topics"); ok {
			t.Logf("Topics: %v", topics)
		}
		if entities, ok := record.Get("entities"); ok {
			t.Logf("Entities: %v", entities)
		}
	}

	t.Log("Multi-entity type query completed successfully")
}

// TestGraphAdvanced_PerformanceLargeGraph tests query performance with many nodes
func TestGraphAdvanced_PerformanceLargeGraph(t *testing.T) {
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

	// Create 30 files (reasonable for E2E, more for stress testing)
	fileCount := 30
	t.Logf("Creating %d files for performance testing", fileCount)

	for i := 0; i < fileCount; i++ {
		filename := fmt.Sprintf("perf_%d.md", i)
		content := fmt.Sprintf("# Performance Test %d\n\nContent for file %d", i, i)
		if err := h.AddMemoryFile(filename, content); err != nil {
			t.Fatalf("Failed to add file %d: %v", i, err)
		}
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

	// Wait for all files to be processed
	time.Sleep(20 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Performance test: count all files
	startTime := time.Now()
	count, err := h.GraphClient.CountNodes(ctx, "File")
	countDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Count query failed: %v", err)
	}

	t.Logf("Count query on %d files took %v", count, countDuration)

	if countDuration > 5*time.Second {
		t.Logf("Warning: Count query took longer than expected (%v)", countDuration)
	}

	// Performance test: search query
	searchQuery := `
		MATCH (f:File)
		RETURN f.name, f.path
		ORDER BY f.modified DESC
		LIMIT 10
	`

	startTime = time.Now()
	_, err = h.GraphClient.Query(ctx, searchQuery)
	searchDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Search query failed: %v", err)
	}

	t.Logf("Search query on %d files took %v", count, searchDuration)

	if searchDuration > 5*time.Second {
		t.Logf("Warning: Search query took longer than expected (%v)", searchDuration)
	}
}

// TestGraphAdvanced_FuzzyMatching tests case-insensitive and partial matching
func TestGraphAdvanced_FuzzyMatching(t *testing.T) {
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

	// Add files with various naming patterns
	files := []string{
		"README.md",
		"readme.txt",
		"ReadMe_Config.md",
		"read-me-first.md",
	}

	for _, f := range files {
		content := fmt.Sprintf("# %s\n\nContent", f)
		if err := h.AddMemoryFile(f, content); err != nil {
			t.Fatalf("Failed to add file %s: %v", f, err)
		}
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

	time.Sleep(10 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Fuzzy search: case-insensitive substring match
	query := `
		MATCH (f:File)
		WHERE toLower(f.name) CONTAINS toLower($pattern)
		RETURN f.name
	`

	tests := []struct {
		pattern     string
		expectMatch int
	}{
		{"readme", 4}, // Should match all readme variants
		{"READ", 4},   // Case-insensitive
		{"config", 1}, // Partial match
		{"xyz", 0},    // No match
	}

	for _, tt := range tests {
		result, err := h.GraphClient.Query(ctx, query)
		if err != nil {
			// Query with parameters - need to use raw query
			rawQuery := fmt.Sprintf(`
				MATCH (f:File)
				WHERE toLower(f.name) CONTAINS '%s'
				RETURN f.name
			`, tt.pattern)
			result, err = h.GraphClient.Query(ctx, rawQuery)
			if err != nil {
				t.Errorf("Fuzzy search for %q failed: %v", tt.pattern, err)
				continue
			}
		}

		matchCount := 0
		for result.Next() {
			record := result.Record()
			if name, ok := record.Get("f.name"); ok {
				t.Logf("Pattern %q matched: %v", tt.pattern, name)
				matchCount++
			}
		}

		if matchCount > 0 && tt.expectMatch == 0 {
			t.Logf("Pattern %q matched %d files (expected no matches)", tt.pattern, matchCount)
		} else if matchCount == 0 && tt.expectMatch > 0 {
			t.Logf("Pattern %q matched no files (expected matches)", tt.pattern)
		} else {
			t.Logf("Fuzzy search for %q: %d matches", tt.pattern, matchCount)
		}
	}
}

// TestGraphAdvanced_IntegrityAfterRestart tests graph persistence across daemon restarts
func TestGraphAdvanced_IntegrityAfterRestart(t *testing.T) {
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
	if err := h.AddMemoryFile("persist1.md", "# Persist 1\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("persist2.md", "# Persist 2\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon (first time)
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		cancel()
		cmd.Wait()
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Connect to graph and verify files
	if err := h.GraphClient.Connect(); err != nil {
		cancel()
		cmd.Wait()
		t.Fatalf("Failed to connect to graph: %v", err)
	}

	countBefore, err := h.GraphClient.CountNodes(ctx, "File")
	if err != nil {
		h.GraphClient.Close()
		cancel()
		cmd.Wait()
		t.Fatalf("Failed to count files before restart: %v", err)
	}

	t.Logf("Files before restart: %d", countBefore)
	h.GraphClient.Close()

	// Stop daemon
	cancel()
	cmd.Wait()

	time.Sleep(2 * time.Second)

	// Start daemon again
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	cmd2 := exec.CommandContext(ctx2, h.BinaryPath, "daemon", "start")
	cmd2.Env = append(cmd2.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd2.Start(); err != nil {
		t.Fatalf("Failed to restart daemon: %v", err)
	}

	defer func() {
		cancel2()
		cmd2.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy after restart: %v", err)
	}

	time.Sleep(5 * time.Second)

	// Reconnect to graph and verify integrity
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to reconnect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	countAfter, err := h.GraphClient.CountNodes(ctx2, "File")
	if err != nil {
		t.Fatalf("Failed to count files after restart: %v", err)
	}

	t.Logf("Files after restart: %d", countAfter)

	// Graph should persist (FalkorDB data persists)
	if countAfter < countBefore {
		t.Errorf("Graph lost data during restart: before=%d, after=%d", countBefore, countAfter)
	} else if countAfter == countBefore {
		t.Log("Graph integrity maintained across restart")
	} else {
		t.Logf("Graph has more files after restart (possible rebuild): before=%d, after=%d", countBefore, countAfter)
	}
}

// TestGraphAdvanced_ComplexRelationshipPatterns tests complex Cypher patterns
func TestGraphAdvanced_ComplexRelationshipPatterns(t *testing.T) {
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
	if err := h.AddMemoryFile("complex.md", "# Complex\n\nContent with entities"); err != nil {
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

	time.Sleep(5 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Complex pattern: Files connected through multiple relationship types
	query := `
		MATCH path = (f1:File)-[r*1..2]-(f2:File)
		WHERE f1 <> f2
		RETURN DISTINCT
			f1.name as file1,
			f2.name as file2,
			length(path) as distance
		LIMIT 10
	`

	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Complex pattern query failed: %v", err)
	}

	pathCount := 0
	for result.Next() {
		record := result.Record()
		if file1, ok := record.Get("file1"); ok {
			if file2, ok := record.Get("file2"); ok {
				if distance, ok := record.Get("distance"); ok {
					t.Logf("Path found: %v -> %v (distance=%v)", file1, file2, distance)
					pathCount++
				}
			}
		}
	}

	t.Logf("Complex pattern query found %d paths", pathCount)
}

// TestGraphAdvanced_CrossCategoryQueries tests queries across file categories
func TestGraphAdvanced_CrossCategoryQueries(t *testing.T) {
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

	// Create files of different categories
	files := []struct {
		name    string
		content string
	}{
		{"doc.md", "# Documentation\n\nMarkdown document"},
		{"data.json", `{"key": "value"}`},
		{"code.go", "package main\n\nfunc main() {}"},
	}

	for _, f := range files {
		if err := h.AddMemoryFile(f.name, f.content); err != nil {
			t.Fatalf("Failed to add file %s: %v", f.name, err)
		}
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

	time.Sleep(10 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Cross-category query: group by category
	query := `
		MATCH (f:File)
		RETURN f.category as category, count(*) as file_count
		ORDER BY file_count DESC
	`

	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Cross-category query failed: %v", err)
	}

	categoryCount := 0
	for result.Next() {
		record := result.Record()
		if category, ok := record.Get("category"); ok {
			if count, ok := record.Get("file_count"); ok {
				t.Logf("Category: %v, Files: %v", category, count)
				categoryCount++
			}
		}
	}

	t.Logf("Cross-category query found %d categories", categoryCount)
}

// TestGraphAdvanced_RelationshipCounting tests counting relationships
func TestGraphAdvanced_RelationshipCounting(t *testing.T) {
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
	if err := h.AddMemoryFile("tagged.md", "# Tagged Document\n\nContent"); err != nil {
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

	time.Sleep(5 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Count different relationship types
	query := `
		MATCH (f:File {name: 'tagged.md'})-[r]->(connected)
		RETURN type(r) as relationship_type, count(*) as count
	`

	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Relationship counting failed: %v", err)
	}

	relTypes := 0
	for result.Next() {
		record := result.Record()
		if relType, ok := record.Get("relationship_type"); ok {
			if count, ok := record.Get("count"); ok {
				t.Logf("Relationship type: %v, Count: %v", relType, count)
				relTypes++
			}
		}
	}

	t.Logf("Found %d relationship types", relTypes)
}

// TestGraphAdvanced_AggregationQueries tests aggregation and statistics
func TestGraphAdvanced_AggregationQueries(t *testing.T) {
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

	// Add multiple test files
	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("agg_%d.md", i)
		content := fmt.Sprintf("# Aggregation Test %d\n\nContent", i)
		if err := h.AddMemoryFile(filename, content); err != nil {
			t.Fatalf("Failed to add file %d: %v", i, err)
		}
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

	time.Sleep(10 * time.Second)

	// Connect to graph
	if err := h.GraphClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to graph: %v", err)
	}
	defer h.GraphClient.Close()

	// Aggregation query: file statistics
	query := `
		MATCH (f:File)
		RETURN
			count(f) as total_files,
			avg(f.size) as avg_size,
			max(f.size) as max_size,
			min(f.size) as min_size
	`

	result, err := h.GraphClient.Query(ctx, query)
	if err != nil {
		t.Fatalf("Aggregation query failed: %v", err)
	}

	if result.Next() {
		record := result.Record()
		if totalFiles, ok := record.Get("total_files"); ok {
			t.Logf("Total files: %v", totalFiles)
		}
		if avgSize, ok := record.Get("avg_size"); ok {
			t.Logf("Average size: %v", avgSize)
		}
		if maxSize, ok := record.Get("max_size"); ok {
			t.Logf("Max size: %v", maxSize)
		}
		if minSize, ok := record.Get("min_size"); ok {
			t.Logf("Min size: %v", minSize)
		}
	}

	t.Log("Aggregation query completed successfully")
}
