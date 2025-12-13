package graph

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/FalkorDB/falkordb-go/v2"
	"github.com/redis/go-redis/v9/maintnotifications"
)

// ClientConfig contains FalkorDB connection settings
type ClientConfig struct {
	Host     string
	Port     int
	Database string
	Password string
}

// DefaultClientConfig returns default configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Host:     "localhost",
		Port:     6379,
		Database: "memorizer",
		Password: "",
	}
}

// Client wraps FalkorDB connection and provides graph operations
type Client struct {
	config   ClientConfig
	db       *falkordb.FalkorDB
	graph    *falkordb.Graph
	mu       sync.RWMutex
	logger   *slog.Logger
	closed   bool
	closedMu sync.RWMutex
}

// NewClient creates a new FalkorDB client
func NewClient(config ClientConfig, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		config: config,
		logger: logger.With("component", "graph-client"),
	}
}

// Connect establishes connection to FalkorDB
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	c.logger.Info("connecting to FalkorDB",
		"host", c.config.Host,
		"port", c.config.Port,
		"database", c.config.Database,
	)

	opts := &falkordb.ConnectionOption{
		Addr: addr,
		// Disable maintenance notifications (Redis Enterprise feature not supported by FalkorDB)
		// This prevents go-redis from attempting CLIENT MAINT_NOTIFICATIONS handshake
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	}

	if c.config.Password != "" {
		opts.Password = c.config.Password
	}

	db, err := falkordb.FalkorDBNew(opts)
	if err != nil {
		return fmt.Errorf("failed to connect to FalkorDB; %w", err)
	}

	c.db = db
	c.graph = db.SelectGraph(c.config.Database)

	// Verify connection with a ping query
	// FalkorDB uses lazy connection, so we need to execute a query to confirm connectivity
	_, err = c.graph.Query("RETURN 1", nil, nil)
	if err != nil {
		c.db = nil
		c.graph = nil
		return fmt.Errorf("failed to verify FalkorDB connection; %w", err)
	}

	c.logger.Info("connected to FalkorDB",
		"database", c.config.Database,
	)

	return nil
}

// Close closes the FalkorDB connection
func (c *Client) Close() error {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()

	if c.closed {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil && c.db.Conn != nil {
		if err := c.db.Conn.Close(); err != nil {
			c.logger.Error("error closing FalkorDB connection", "error", err)
			return fmt.Errorf("failed to close FalkorDB connection; %w", err)
		}
	}

	c.closed = true
	c.logger.Info("falkordb connection closed")
	return nil
}

// IsConnected checks if client is connected
func (c *Client) IsConnected() bool {
	c.closedMu.RLock()
	defer c.closedMu.RUnlock()

	if c.closed {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.db != nil
}

// Ping checks connectivity to FalkorDB
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return fmt.Errorf("not connected to FalkorDB")
	}

	// Execute a simple query to verify connectivity
	_, err := c.graph.Query("RETURN 1", nil, nil)
	if err != nil {
		return fmt.Errorf("FalkorDB ping failed; %w", err)
	}

	return nil
}

// Health returns health status information
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	status := &HealthStatus{
		Connected: c.IsConnected(),
		Database:  c.config.Database,
		Timestamp: time.Now(),
	}

	if !status.Connected {
		status.Error = "not connected"
		return status, nil
	}

	// Check ping
	if err := c.Ping(ctx); err != nil {
		status.Connected = false
		status.Error = err.Error()
		return status, nil
	}

	// Get graph statistics
	stats, err := c.GetGraphStats(ctx)
	if err != nil {
		c.logger.Warn("failed to get graph stats for health check", "error", err)
	} else {
		status.Stats = stats
	}

	return status, nil
}

// HealthStatus represents the health of the FalkorDB connection
type HealthStatus struct {
	Connected bool        `json:"connected"`
	Database  string      `json:"database"`
	Error     string      `json:"error,omitempty"`
	Stats     *GraphStats `json:"stats,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// GraphStats represents graph statistics
type GraphStats struct {
	NodeCount         int64 `json:"node_count"`
	RelationshipCount int64 `json:"relationship_count"`
	LabelCount        int   `json:"label_count"`
}

// GetGraphStats retrieves statistics about the graph
func (c *Client) GetGraphStats(ctx context.Context) (*GraphStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.graph == nil {
		return nil, fmt.Errorf("not connected to FalkorDB")
	}

	stats := &GraphStats{}

	// Get node count
	result, err := c.graph.Query("MATCH (n) RETURN count(n) as count", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get node count; %w", err)
	}
	if result.Next() {
		record := result.Record()
		if count, ok := record.GetByIndex(0).(int64); ok {
			stats.NodeCount = count
		}
	}

	// Get relationship count
	result, err = c.graph.Query("MATCH ()-[r]->() RETURN count(r) as count", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get relationship count; %w", err)
	}
	if result.Next() {
		record := result.Record()
		if count, ok := record.GetByIndex(0).(int64); ok {
			stats.RelationshipCount = count
		}
	}

	return stats, nil
}

// Query executes a Cypher query with parameters
func (c *Client) Query(ctx context.Context, query string, params map[string]any) (*QueryResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.graph == nil {
		return nil, fmt.Errorf("not connected to FalkorDB")
	}

	c.logger.Debug("executing query",
		"query", query,
		"params", params,
	)

	result, err := c.graph.Query(query, params, nil)
	if err != nil {
		return nil, fmt.Errorf("query failed; %w", err)
	}

	return &QueryResult{result: result}, nil
}

// QueryResult wraps FalkorDB query result for easier access
type QueryResult struct {
	result *falkordb.QueryResult
}

// Next advances to next record
func (qr *QueryResult) Next() bool {
	return qr.result.Next()
}

// Record returns current record
func (qr *QueryResult) Record() *Record {
	return &Record{record: qr.result.Record()}
}

// Empty returns true if result has no records
func (qr *QueryResult) Empty() bool {
	return qr.result.Empty()
}

// Record wraps FalkorDB record for easier access
type Record struct {
	record *falkordb.Record
}

// GetByIndex returns value at index
func (r *Record) GetByIndex(index int) any {
	return r.record.GetByIndex(index)
}

// Get returns value by key name
func (r *Record) Get(key string) (any, bool) {
	return r.record.Get(key)
}

// GetString returns string value at index, with default fallback
func (r *Record) GetString(index int, defaultVal string) string {
	val := r.record.GetByIndex(index)
	if s, ok := val.(string); ok {
		return s
	}
	return defaultVal
}

// GetInt64 returns int64 value at index, with default fallback
func (r *Record) GetInt64(index int, defaultVal int64) int64 {
	val := r.record.GetByIndex(index)
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	}
	return defaultVal
}

// GetFloat64 returns float64 value at index, with default fallback
func (r *Record) GetFloat64(index int, defaultVal float64) float64 {
	val := r.record.GetByIndex(index)
	switch v := val.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	}
	return defaultVal
}

// Graph returns the underlying FalkorDB graph for direct access when needed
func (c *Client) Graph() *falkordb.Graph {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.graph
}
