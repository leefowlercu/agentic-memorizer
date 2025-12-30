package graph

import (
	"context"
	"testing"
	"time"
)

func TestExtractDirName(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "/Users/lee/.memorizer/memory",
			expected: "memory",
		},
		{
			name:     "path with trailing slash",
			path:     "/Users/lee/.memorizer/memory/",
			expected: "memory",
		},
		{
			name:     "nested path",
			path:     "/Users/lee/.memorizer/memory/documents/work",
			expected: "work",
		},
		{
			name:     "root path",
			path:     "/",
			expected: "/",
		},
		{
			name:     "single directory",
			path:     "/home",
			expected: "home",
		},
		{
			name:     "no leading slash",
			path:     "documents/notes",
			expected: "notes",
		},
		{
			name:     "single name no slash",
			path:     "documents",
			expected: "documents",
		},
		{
			name:     "path with spaces",
			path:     "/Users/lee/My Documents/Work",
			expected: "Work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDirName(tt.path)
			if result != tt.expected {
				t.Errorf("extractDirName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// Integration test - requires running FalkorDB
func TestEdges_CleanupOrphanedNodes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient(ClientConfig{
		Host:     "localhost",
		Port:     6379,
		Database: "test_edges_cleanup",
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Skipf("Skipping test (requires FalkorDB); %v", err)
	}
	defer client.Close()

	// Clear any existing data
	_, _ = client.Query(ctx, "MATCH (n) DETACH DELETE n", nil)

	edges := NewEdges(client, nil)

	// Create orphaned nodes (no file connections)
	_, err := client.Query(ctx, "CREATE (:Tag {name: 'orphan-tag'})", nil)
	if err != nil {
		t.Fatalf("failed to create orphan tag; %v", err)
	}
	_, err = client.Query(ctx, "CREATE (:Topic {name: 'orphan-topic'})", nil)
	if err != nil {
		t.Fatalf("failed to create orphan topic; %v", err)
	}
	_, err = client.Query(ctx, "CREATE (:Entity {name: 'orphan-entity', type: 'test'})", nil)
	if err != nil {
		t.Fatalf("failed to create orphan entity; %v", err)
	}
	_, err = client.Query(ctx, "CREATE (:Directory {path: '/orphan/dir', name: 'dir'})", nil)
	if err != nil {
		t.Fatalf("failed to create orphan directory; %v", err)
	}

	// Create a file with connected nodes
	_, err = client.Query(ctx, `
		CREATE (f:File {path: '/test/file.txt', name: 'file.txt'})
		CREATE (t:Tag {name: 'connected-tag'})
		CREATE (f)-[:HAS_TAG]->(t)
	`, nil)
	if err != nil {
		t.Fatalf("failed to create file with connected tag; %v", err)
	}

	// Run cleanup
	removed, err := edges.CleanupOrphanedNodes(ctx)
	if err != nil {
		t.Fatalf("CleanupOrphanedNodes failed; %v", err)
	}

	// Should have removed 4 orphaned nodes (tag, topic, entity, directory)
	if removed != 4 {
		t.Errorf("expected 4 orphaned nodes removed, got %d", removed)
	}

	// Verify connected tag still exists
	result, err := client.Query(ctx, "MATCH (t:Tag {name: 'connected-tag'}) RETURN t", nil)
	if err != nil {
		t.Fatalf("failed to query connected tag; %v", err)
	}
	if !result.Next() {
		t.Error("connected tag should still exist after cleanup")
	}

	// Verify orphan tag was removed
	result, err = client.Query(ctx, "MATCH (t:Tag {name: 'orphan-tag'}) RETURN t", nil)
	if err != nil {
		t.Fatalf("failed to query orphan tag; %v", err)
	}
	if result.Next() {
		t.Error("orphan tag should have been removed")
	}

	// Cleanup test data
	_, _ = client.Query(ctx, "MATCH (n) DETACH DELETE n", nil)
}
