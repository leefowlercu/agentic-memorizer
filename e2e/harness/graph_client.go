package harness

import (
	"context"
	"fmt"

	"github.com/FalkorDB/falkordb-go/v2"
	"github.com/redis/go-redis/v9/maintnotifications"
)

// GraphClient provides a test client for FalkorDB operations
type GraphClient struct {
	host      string
	port      string
	graphName string
	db        *falkordb.FalkorDB
	graph     *falkordb.Graph
}

// NewGraphClient creates a new FalkorDB test client
func NewGraphClient(host, port, graphName string) *GraphClient {
	return &GraphClient{
		host:      host,
		port:      port,
		graphName: graphName,
	}
}

// Connect establishes connection to FalkorDB
func (c *GraphClient) Connect() error {
	addr := fmt.Sprintf("%s:%s", c.host, c.port)

	opts := &falkordb.ConnectionOption{
		Addr: addr,
		// Disable maintenance notifications (Redis Enterprise feature not supported by FalkorDB)
		// This prevents go-redis from attempting CLIENT MAINT_NOTIFICATIONS handshake
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	}

	db, err := falkordb.FalkorDBNew(opts)
	if err != nil {
		return fmt.Errorf("failed to connect to FalkorDB; %w", err)
	}

	c.db = db
	c.graph = db.SelectGraph(c.graphName)

	// Verify connection
	_, err = c.graph.Query("RETURN 1", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to verify connection; %w", err)
	}

	return nil
}

// Close closes the FalkorDB connection
func (c *GraphClient) Close() error {
	// FalkorDB client doesn't require explicit close
	// Connection is managed by the Redis client underneath
	c.db = nil
	c.graph = nil
	return nil
}

// Query executes a Cypher query
func (c *GraphClient) Query(ctx context.Context, query string) (falkordb.QueryResult, error) {
	if c.graph == nil {
		if err := c.Connect(); err != nil {
			return falkordb.QueryResult{}, err
		}
	}

	result, err := c.graph.Query(query, nil, nil)
	if err != nil {
		return falkordb.QueryResult{}, fmt.Errorf("query failed; %w", err)
	}

	if result == nil {
		return falkordb.QueryResult{}, fmt.Errorf("query returned nil result")
	}

	return *result, nil
}

// Clear clears all data from the test graph
func (c *GraphClient) Clear(ctx context.Context) error {
	if c.graph == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Delete all nodes and relationships
	_, err := c.graph.Query("MATCH (n) DETACH DELETE n", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to clear graph; %w", err)
	}

	return nil
}

// DropGraph completely drops the test graph (for test cleanup)
// Note: With unique graph names per test, simple Clear() is sufficient for cleanup
func (c *GraphClient) DropGraph(ctx context.Context) error {
	// Since each test uses a unique graph name, clearing all nodes is sufficient
	return c.Clear(ctx)
}

// CountNodes counts nodes of a specific label
func (c *GraphClient) CountNodes(ctx context.Context, label string) (int, error) {
	query := fmt.Sprintf("MATCH (n:%s) RETURN count(n) as count", label)
	result, err := c.Query(ctx, query)
	if err != nil {
		return 0, err
	}

	if !result.Next() {
		return 0, fmt.Errorf("no results from count query")
	}

	record := result.Record()
	countVal, ok := record.Get("count")
	if !ok {
		return 0, fmt.Errorf("count field not found in result")
	}

	count, ok := countVal.(int64)
	if !ok {
		return 0, fmt.Errorf("count is not an integer")
	}

	return int(count), nil
}

// FileExists checks if a file node exists in the graph
func (c *GraphClient) FileExists(ctx context.Context, path string) (bool, error) {
	query := fmt.Sprintf("MATCH (f:File {path: '%s'}) RETURN count(f) as count", path)
	result, err := c.Query(ctx, query)
	if err != nil {
		return false, err
	}

	if !result.Next() {
		return false, nil
	}

	record := result.Record()
	countVal, ok := record.Get("count")
	if !ok {
		return false, nil
	}

	count, ok := countVal.(int64)
	if !ok {
		return false, nil
	}

	return count > 0, nil
}

// GetFileTags retrieves tags for a specific file
func (c *GraphClient) GetFileTags(ctx context.Context, path string) ([]string, error) {
	query := fmt.Sprintf("MATCH (f:File {path: '%s'})-[:HAS_TAG]->(t:Tag) RETURN t.name as tag", path)
	result, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var tags []string
	for result.Next() {
		record := result.Record()
		tagVal, ok := record.Get("tag")
		if ok {
			if tag, ok := tagVal.(string); ok {
				tags = append(tags, tag)
			}
		}
	}

	return tags, nil
}

// GetRelatedFiles finds files related to a given file via shared tags/topics
func (c *GraphClient) GetRelatedFiles(ctx context.Context, path string, limit int) ([]string, error) {
	query := fmt.Sprintf(`
		MATCH (f1:File {path: '%s'})-[:HAS_TAG|COVERS_TOPIC]->(shared)<-[:HAS_TAG|COVERS_TOPIC]-(f2:File)
		WHERE f1 <> f2
		RETURN DISTINCT f2.path as path
		LIMIT %d
	`, path, limit)

	result, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var relatedFiles []string
	for result.Next() {
		record := result.Record()
		pathVal, ok := record.Get("path")
		if ok {
			if filePath, ok := pathVal.(string); ok {
				relatedFiles = append(relatedFiles, filePath)
			}
		}
	}

	return relatedFiles, nil
}
