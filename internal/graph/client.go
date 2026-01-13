package graph

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RedisGraph/redisgraph-go"
	"github.com/gomodule/redigo/redis"
)

// Graph is the interface for graph operations.
type Graph interface {
	// Name returns the component name.
	Name() string

	// Start initializes the graph connection.
	Start(ctx context.Context) error

	// Stop closes the graph connection.
	Stop(ctx context.Context) error

	// UpsertFile creates or updates a file node.
	UpsertFile(ctx context.Context, file *FileNode) error

	// DeleteFile removes a file node and its relationships.
	DeleteFile(ctx context.Context, path string) error

	// GetFile retrieves a file node by path.
	GetFile(ctx context.Context, path string) (*FileNode, error)

	// UpsertDirectory creates or updates a directory node.
	UpsertDirectory(ctx context.Context, dir *DirectoryNode) error

	// DeleteDirectory removes a directory node and its relationships.
	DeleteDirectory(ctx context.Context, path string) error

	// UpsertChunk creates or updates a chunk node.
	UpsertChunk(ctx context.Context, chunk *ChunkNode) error

	// DeleteChunks removes all chunks for a file.
	DeleteChunks(ctx context.Context, filePath string) error

	// SetFileTags sets the tags for a file.
	SetFileTags(ctx context.Context, path string, tags []string) error

	// SetFileTopics sets the topics for a file.
	SetFileTopics(ctx context.Context, path string, topics []Topic) error

	// SetFileEntities sets the entities mentioned in a file.
	SetFileEntities(ctx context.Context, path string, entities []Entity) error

	// SetFileReferences sets the references from a file.
	SetFileReferences(ctx context.Context, path string, refs []Reference) error

	// Query executes a raw Cypher query.
	Query(ctx context.Context, cypher string) (*QueryResult, error)

	// HasEmbedding checks if an embedding exists for the given content hash and version.
	HasEmbedding(ctx context.Context, contentHash string, version int) (bool, error)

	// ExportSnapshot exports a complete snapshot of the graph.
	ExportSnapshot(ctx context.Context) (*GraphSnapshot, error)

	// GetFileWithRelations retrieves a file with all its related data.
	GetFileWithRelations(ctx context.Context, path string) (*FileWithRelations, error)

	// IsConnected returns true if connected to the database.
	IsConnected() bool
}

// Config contains graph connection configuration.
type Config struct {
	Host        string
	Port        int
	GraphName   string
	PasswordEnv string
	MaxRetries  int
	RetryDelay  time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host:        "localhost",
		Port:        6379,
		GraphName:   "memorizer",
		PasswordEnv: "MEMORIZER_GRAPH_PASSWORD",
		MaxRetries:  3,
		RetryDelay:  time.Second,
	}
}

// FalkorDBGraph implements Graph using FalkorDB/RedisGraph.
type FalkorDBGraph struct {
	mu        sync.RWMutex
	config    Config
	logger    *slog.Logger
	conn      redis.Conn
	graph     redisgraph.Graph
	connected bool

	// Write queue for graceful degradation
	writeQueue chan writeOp
	wg         sync.WaitGroup
	stopChan   chan struct{}
}

// writeOp represents a queued write operation.
type writeOp struct {
	query  string
	result chan error
}

// Option configures the FalkorDB graph client.
type Option func(*FalkorDBGraph)

// WithConfig sets the configuration.
func WithConfig(cfg Config) Option {
	return func(g *FalkorDBGraph) {
		g.config = cfg
	}
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) Option {
	return func(g *FalkorDBGraph) {
		g.logger = logger
	}
}

// NewFalkorDBGraph creates a new FalkorDB graph client.
func NewFalkorDBGraph(opts ...Option) *FalkorDBGraph {
	g := &FalkorDBGraph{
		config:     DefaultConfig(),
		logger:     slog.Default(),
		writeQueue: make(chan writeOp, 1000),
		stopChan:   make(chan struct{}),
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

// Name returns the component name.
func (g *FalkorDBGraph) Name() string {
	return "graph"
}

// Start initializes the graph connection.
func (g *FalkorDBGraph) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.connected {
		return nil
	}

	// Get password from environment
	password := os.Getenv(g.config.PasswordEnv)

	// Connect to FalkorDB (Redis protocol)
	addr := fmt.Sprintf("%s:%d", g.config.Host, g.config.Port)

	var dialOpts []redis.DialOption
	if password != "" {
		dialOpts = append(dialOpts, redis.DialPassword(password))
	}

	conn, err := redis.Dial("tcp", addr, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect to FalkorDB at %s; %w", addr, err)
	}

	g.conn = conn
	g.graph = redisgraph.GraphNew(g.config.GraphName, conn)
	g.connected = true

	// Create schema constraints
	if err := g.createSchema(ctx); err != nil {
		g.logger.Warn("failed to create schema constraints", "error", err)
	}

	// Start write queue processor
	g.wg.Add(1)
	go g.processWriteQueue()

	g.logger.Info("connected to FalkorDB",
		"host", g.config.Host,
		"port", g.config.Port,
		"graph", g.config.GraphName)

	return nil
}

// Stop closes the graph connection.
func (g *FalkorDBGraph) Stop(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.connected {
		return nil
	}

	// Signal write queue to stop
	close(g.stopChan)

	// Wait for pending writes with timeout
	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Debug("write queue drained")
	case <-ctx.Done():
		g.logger.Warn("write queue drain timed out")
	}

	// Close connection
	if g.conn != nil {
		g.conn.Close()
	}

	g.connected = false
	g.logger.Info("disconnected from FalkorDB")

	return nil
}

// IsConnected returns true if connected to the database.
func (g *FalkorDBGraph) IsConnected() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.connected
}

// createSchema creates indexes and constraints.
func (g *FalkorDBGraph) createSchema(ctx context.Context) error {
	queries := []string{
		// Create indexes for common lookups
		"CREATE INDEX FOR (f:File) ON (f.path)",
		"CREATE INDEX FOR (f:File) ON (f.content_hash)",
		"CREATE INDEX FOR (c:Chunk) ON (c.id)",
		"CREATE INDEX FOR (c:Chunk) ON (c.file_path)",
		"CREATE INDEX FOR (d:Directory) ON (d.path)",
		"CREATE INDEX FOR (t:Tag) ON (t.normalized_name)",
		"CREATE INDEX FOR (t:Topic) ON (t.normalized_name)",
		"CREATE INDEX FOR (e:Entity) ON (e.normalized_name)",
	}

	for _, q := range queries {
		if _, err := g.graph.Query(q); err != nil {
			// Ignore errors for existing indexes
			g.logger.Debug("schema query", "query", q, "error", err)
		}
	}

	return nil
}

// processWriteQueue handles queued write operations.
func (g *FalkorDBGraph) processWriteQueue() {
	defer g.wg.Done()

	for {
		select {
		case <-g.stopChan:
			// Drain remaining operations
			for {
				select {
				case op := <-g.writeQueue:
					g.executeWrite(op)
				default:
					return
				}
			}
		case op := <-g.writeQueue:
			g.executeWrite(op)
		}
	}
}

// executeWrite executes a write operation with retry.
func (g *FalkorDBGraph) executeWrite(op writeOp) {
	var err error
	for i := 0; i <= g.config.MaxRetries; i++ {
		_, err = g.graph.Query(op.query)
		if err == nil {
			if op.result != nil {
				op.result <- nil
			}
			return
		}

		if i < g.config.MaxRetries {
			time.Sleep(g.config.RetryDelay * time.Duration(1<<i))
		}
	}

	if op.result != nil {
		op.result <- err
	}
	g.logger.Error("write operation failed after retries", "error", err)
}

// queueWrite queues a write operation for async execution.
func (g *FalkorDBGraph) queueWrite(query string) error {
	select {
	case g.writeQueue <- writeOp{query: query}:
		return nil
	default:
		return fmt.Errorf("write queue full")
	}
}

// queueWriteSync queues a write operation and waits for completion.
func (g *FalkorDBGraph) queueWriteSync(query string) error {
	result := make(chan error, 1)
	select {
	case g.writeQueue <- writeOp{query: query, result: result}:
		return <-result
	default:
		return fmt.Errorf("write queue full")
	}
}

// UpsertFile creates or updates a file node and its directory relationship.
func (g *FalkorDBGraph) UpsertFile(ctx context.Context, file *FileNode) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	query := fmt.Sprintf(`
		MERGE (f:File {path: '%s'})
		SET f.name = '%s',
			f.extension = '%s',
			f.mime_type = '%s',
			f.language = '%s',
			f.size = %d,
			f.mod_time = %d,
			f.content_hash = '%s',
			f.metadata_hash = '%s',
			f.summary = '%s',
			f.complexity = %d,
			f.analyzed_at = %d,
			f.analysis_version = %d,
			f.updated_at = %d
	`, escapeString(file.Path),
		escapeString(file.Name),
		escapeString(file.Extension),
		escapeString(file.MIMEType),
		escapeString(file.Language),
		file.Size,
		file.ModTime.Unix(),
		escapeString(file.ContentHash),
		escapeString(file.MetadataHash),
		escapeString(file.Summary),
		file.Complexity,
		file.AnalyzedAt.Unix(),
		file.AnalysisVersion,
		time.Now().Unix())

	if err := g.queueWrite(query); err != nil {
		return err
	}

	// Create CONTAINS relationship from parent directory to file
	parentDir := filepath.Dir(file.Path)
	parentName := filepath.Base(parentDir)

	relQuery := fmt.Sprintf(`
		MERGE (d:Directory {path: '%s'})
		ON CREATE SET d.name = '%s', d.is_remembered = false, d.file_count = 0, d.created_at = %d
		SET d.updated_at = %d
		WITH d
		MATCH (f:File {path: '%s'})
		MERGE (d)-[:CONTAINS]->(f)
	`, escapeString(parentDir),
		escapeString(parentName),
		time.Now().Unix(),
		time.Now().Unix(),
		escapeString(file.Path))

	return g.queueWrite(relQuery)
}

// DeleteFile removes a file node and its relationships.
func (g *FalkorDBGraph) DeleteFile(ctx context.Context, path string) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	// Delete chunks first
	chunkQuery := fmt.Sprintf(`
		MATCH (c:Chunk {file_path: '%s'})
		DETACH DELETE c
	`, escapeString(path))
	if err := g.queueWriteSync(chunkQuery); err != nil {
		return err
	}

	// Delete file
	query := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})
		DETACH DELETE f
	`, escapeString(path))
	return g.queueWriteSync(query)
}

// GetFile retrieves a file node by path.
func (g *FalkorDBGraph) GetFile(ctx context.Context, path string) (*FileNode, error) {
	if !g.IsConnected() {
		return nil, fmt.Errorf("not connected to graph database")
	}

	query := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})
		RETURN f.path, f.name, f.extension, f.mime_type, f.language,
			   f.size, f.mod_time, f.content_hash, f.metadata_hash,
			   f.summary, f.complexity, f.analyzed_at, f.analysis_version
	`, escapeString(path))

	result, err := g.graph.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed; %w", err)
	}

	if result.Empty() {
		return nil, nil // Not found
	}

	if !result.Next() {
		return nil, nil
	}

	record := result.Record()
	return parseFileFromRecord(record)
}

// UpsertDirectory creates or updates a directory node.
func (g *FalkorDBGraph) UpsertDirectory(ctx context.Context, dir *DirectoryNode) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	query := fmt.Sprintf(`
		MERGE (d:Directory {path: '%s'})
		SET d.name = '%s',
			d.is_remembered = %t,
			d.file_count = %d,
			d.updated_at = %d
	`, escapeString(dir.Path),
		escapeString(dir.Name),
		dir.IsRemembered,
		dir.FileCount,
		time.Now().Unix())

	return g.queueWrite(query)
}

// DeleteDirectory removes a directory node and its relationships.
func (g *FalkorDBGraph) DeleteDirectory(ctx context.Context, path string) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	query := fmt.Sprintf(`
		MATCH (d:Directory {path: '%s'})
		DETACH DELETE d
	`, escapeString(path))
	return g.queueWriteSync(query)
}

// UpsertChunk creates or updates a chunk node.
func (g *FalkorDBGraph) UpsertChunk(ctx context.Context, chunk *ChunkNode) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	query := fmt.Sprintf(`
		MERGE (c:Chunk {id: '%s'})
		SET c.file_path = '%s',
			c.index = %d,
			c.content_hash = '%s',
			c.start_offset = %d,
			c.end_offset = %d,
			c.chunk_type = '%s',
			c.function_name = '%s',
			c.class_name = '%s',
			c.heading = '%s',
			c.heading_level = %d,
			c.summary = '%s',
			c.embedding_version = %d,
			c.token_count = %d,
			c.updated_at = %d
	`, escapeString(chunk.ID),
		escapeString(chunk.FilePath),
		chunk.Index,
		escapeString(chunk.ContentHash),
		chunk.StartOffset,
		chunk.EndOffset,
		escapeString(chunk.ChunkType),
		escapeString(chunk.FunctionName),
		escapeString(chunk.ClassName),
		escapeString(chunk.Heading),
		chunk.HeadingLevel,
		escapeString(chunk.Summary),
		chunk.EmbeddingVersion,
		chunk.TokenCount,
		time.Now().Unix())

	if err := g.queueWrite(query); err != nil {
		return err
	}

	// Create relationship to file
	relQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})
		MATCH (c:Chunk {id: '%s'})
		MERGE (f)-[:HAS_CHUNK]->(c)
	`, escapeString(chunk.FilePath), escapeString(chunk.ID))

	return g.queueWrite(relQuery)
}

// DeleteChunks removes all chunks for a file.
func (g *FalkorDBGraph) DeleteChunks(ctx context.Context, filePath string) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	query := fmt.Sprintf(`
		MATCH (c:Chunk {file_path: '%s'})
		DETACH DELETE c
	`, escapeString(filePath))
	return g.queueWriteSync(query)
}

// SetFileTags sets the tags for a file.
func (g *FalkorDBGraph) SetFileTags(ctx context.Context, path string, tags []string) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	// First remove existing tag relationships
	removeQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})-[r:HAS_TAG]->()
		DELETE r
	`, escapeString(path))
	if err := g.queueWriteSync(removeQuery); err != nil {
		return err
	}

	// Add new tags
	for _, tag := range tags {
		query := fmt.Sprintf(`
			MATCH (f:File {path: '%s'})
			MERGE (t:Tag {normalized_name: '%s'})
			ON CREATE SET t.name = '%s', t.usage_count = 1, t.created_at = %d
			ON MATCH SET t.usage_count = t.usage_count + 1
			MERGE (f)-[:HAS_TAG]->(t)
		`, escapeString(path),
			escapeString(normalizeString(tag)),
			escapeString(tag),
			time.Now().Unix())

		if err := g.queueWrite(query); err != nil {
			return err
		}
	}

	return nil
}

// SetFileTopics sets the topics for a file.
func (g *FalkorDBGraph) SetFileTopics(ctx context.Context, path string, topics []Topic) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	// First remove existing topic relationships
	removeQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})-[r:COVERS_TOPIC]->()
		DELETE r
	`, escapeString(path))
	if err := g.queueWriteSync(removeQuery); err != nil {
		return err
	}

	// Add new topics
	for _, topic := range topics {
		query := fmt.Sprintf(`
			MATCH (f:File {path: '%s'})
			MERGE (t:Topic {normalized_name: '%s'})
			ON CREATE SET t.name = '%s', t.usage_count = 1, t.created_at = %d
			ON MATCH SET t.usage_count = t.usage_count + 1
			MERGE (f)-[:COVERS_TOPIC {confidence: %f}]->(t)
		`, escapeString(path),
			escapeString(normalizeString(topic.Name)),
			escapeString(topic.Name),
			time.Now().Unix(),
			topic.Confidence)

		if err := g.queueWrite(query); err != nil {
			return err
		}
	}

	return nil
}

// SetFileEntities sets the entities mentioned in a file.
func (g *FalkorDBGraph) SetFileEntities(ctx context.Context, path string, entities []Entity) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	// First remove existing entity relationships
	removeQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})-[r:MENTIONS]->()
		DELETE r
	`, escapeString(path))
	if err := g.queueWriteSync(removeQuery); err != nil {
		return err
	}

	// Add new entities
	for _, entity := range entities {
		query := fmt.Sprintf(`
			MATCH (f:File {path: '%s'})
			MERGE (e:Entity {normalized_name: '%s', type: '%s'})
			ON CREATE SET e.name = '%s', e.usage_count = 1, e.created_at = %d
			ON MATCH SET e.usage_count = e.usage_count + 1
			MERGE (f)-[:MENTIONS]->(e)
		`, escapeString(path),
			escapeString(normalizeString(entity.Name)),
			escapeString(entity.Type),
			escapeString(entity.Name),
			time.Now().Unix())

		if err := g.queueWrite(query); err != nil {
			return err
		}
	}

	return nil
}

// SetFileReferences sets the references from a file.
func (g *FalkorDBGraph) SetFileReferences(ctx context.Context, path string, refs []Reference) error {
	if !g.IsConnected() {
		return fmt.Errorf("not connected to graph database")
	}

	// First remove existing reference relationships
	removeQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})-[r:REFERENCES]->()
		DELETE r
	`, escapeString(path))
	if err := g.queueWriteSync(removeQuery); err != nil {
		return err
	}

	// Add new references
	for _, ref := range refs {
		if ref.Type == "file" {
			// Reference to another file
			query := fmt.Sprintf(`
				MATCH (f:File {path: '%s'})
				MERGE (t:File {path: '%s'})
				MERGE (f)-[:REFERENCES {type: 'file'}]->(t)
			`, escapeString(path), escapeString(ref.Target))

			if err := g.queueWrite(query); err != nil {
				return err
			}
		}
		// Other reference types can be stored as properties on the relationship
	}

	return nil
}

// Query executes a raw Cypher query.
func (g *FalkorDBGraph) Query(ctx context.Context, cypher string) (*QueryResult, error) {
	if !g.IsConnected() {
		return nil, fmt.Errorf("not connected to graph database")
	}

	result, err := g.graph.Query(cypher)
	if err != nil {
		return nil, fmt.Errorf("query failed; %w", err)
	}

	return convertQueryResult(result), nil
}

// HasEmbedding checks if an embedding exists for the given content hash and version.
func (g *FalkorDBGraph) HasEmbedding(ctx context.Context, contentHash string, version int) (bool, error) {
	if !g.IsConnected() {
		return false, fmt.Errorf("not connected to graph database")
	}

	query := fmt.Sprintf(`
		MATCH (c:Chunk {content_hash: '%s', embedding_version: %d})
		WHERE c.embedding IS NOT NULL
		RETURN count(c)
	`, escapeString(contentHash), version)

	result, err := g.graph.Query(query)
	if err != nil {
		return false, fmt.Errorf("query failed; %w", err)
	}

	if result.Empty() || !result.Next() {
		return false, nil
	}

	record := result.Record()
	countVal := record.GetByIndex(0)
	if countVal == nil {
		return false, nil
	}

	count, ok := countVal.(int64)
	if !ok {
		return false, nil
	}

	return count > 0, nil
}

// ExportSnapshot exports a complete snapshot of the graph.
func (g *FalkorDBGraph) ExportSnapshot(ctx context.Context) (*GraphSnapshot, error) {
	if !g.IsConnected() {
		return nil, fmt.Errorf("not connected to graph database")
	}

	snapshot := &GraphSnapshot{
		ExportedAt: time.Now(),
		Version:    1,
	}

	// Export files
	files, err := g.exportFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export files; %w", err)
	}
	snapshot.Files = files

	// Export directories
	dirs, err := g.exportDirectories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export directories; %w", err)
	}
	snapshot.Directories = dirs

	// Export tags
	tags, err := g.exportTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export tags; %w", err)
	}
	snapshot.Tags = tags

	// Export topics
	topics, err := g.exportTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export topics; %w", err)
	}
	snapshot.Topics = topics

	// Export entities
	entities, err := g.exportEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to export entities; %w", err)
	}
	snapshot.Entities = entities

	// Get counts
	snapshot.TotalChunks, _ = g.countNodes(ctx, LabelChunk)
	snapshot.TotalRelationships, _ = g.countRelationships(ctx)

	return snapshot, nil
}

// GetFileWithRelations retrieves a file with all its related data.
func (g *FalkorDBGraph) GetFileWithRelations(ctx context.Context, path string) (*FileWithRelations, error) {
	if !g.IsConnected() {
		return nil, fmt.Errorf("not connected to graph database")
	}

	file, err := g.GetFile(ctx, path)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, nil
	}

	result := &FileWithRelations{File: *file}

	// Get tags
	tagQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})-[:HAS_TAG]->(t:Tag)
		RETURN t.name
	`, escapeString(path))
	tagResult, err := g.graph.Query(tagQuery)
	if err == nil {
		for tagResult.Next() {
			record := tagResult.Record()
			if name := getStringFromRecord(record, 0); name != "" {
				result.Tags = append(result.Tags, name)
			}
		}
	}

	// Get topics
	topicQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})-[r:COVERS_TOPIC]->(t:Topic)
		RETURN t.name, r.confidence
	`, escapeString(path))
	topicResult, err := g.graph.Query(topicQuery)
	if err == nil {
		for topicResult.Next() {
			record := topicResult.Record()
			name := getStringFromRecord(record, 0)
			confidence := getFloatFromRecord(record, 1)
			if name != "" {
				result.Topics = append(result.Topics, Topic{Name: name, Confidence: confidence})
			}
		}
	}

	// Get entities
	entityQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})-[:MENTIONS]->(e:Entity)
		RETURN e.name, e.type
	`, escapeString(path))
	entityResult, err := g.graph.Query(entityQuery)
	if err == nil {
		for entityResult.Next() {
			record := entityResult.Record()
			name := getStringFromRecord(record, 0)
			entityType := getStringFromRecord(record, 1)
			if name != "" {
				result.Entities = append(result.Entities, Entity{Name: name, Type: entityType})
			}
		}
	}

	// Get chunk count
	countQuery := fmt.Sprintf(`
		MATCH (f:File {path: '%s'})-[:HAS_CHUNK]->(c:Chunk)
		RETURN count(c)
	`, escapeString(path))
	countResult, err := g.graph.Query(countQuery)
	if err == nil && countResult.Next() {
		result.ChunkCount = getIntFromRecord(countResult.Record(), 0)
	}

	return result, nil
}

// Helper functions for export

func (g *FalkorDBGraph) exportFiles(ctx context.Context) ([]FileNode, error) {
	query := `
		MATCH (f:File)
		RETURN f.path, f.name, f.extension, f.mime_type, f.language,
			   f.size, f.mod_time, f.content_hash, f.metadata_hash,
			   f.summary, f.complexity, f.analyzed_at, f.analysis_version
	`
	result, err := g.graph.Query(query)
	if err != nil {
		return nil, err
	}

	var files []FileNode
	for result.Next() {
		file, err := parseFileFromRecord(result.Record())
		if err != nil {
			continue
		}
		files = append(files, *file)
	}

	return files, nil
}

func (g *FalkorDBGraph) exportDirectories(ctx context.Context) ([]DirectoryNode, error) {
	query := `
		MATCH (d:Directory)
		RETURN d.path, d.name, d.is_remembered, d.file_count
	`
	result, err := g.graph.Query(query)
	if err != nil {
		return nil, err
	}

	var dirs []DirectoryNode
	for result.Next() {
		record := result.Record()
		dirs = append(dirs, DirectoryNode{
			Path:         getStringFromRecord(record, 0),
			Name:         getStringFromRecord(record, 1),
			IsRemembered: getBoolFromRecord(record, 2),
			FileCount:    getIntFromRecord(record, 3),
		})
	}

	return dirs, nil
}

func (g *FalkorDBGraph) exportTags(ctx context.Context) ([]TagNode, error) {
	query := `
		MATCH (t:Tag)
		RETURN t.name, t.normalized_name, t.usage_count
	`
	result, err := g.graph.Query(query)
	if err != nil {
		return nil, err
	}

	var tags []TagNode
	for result.Next() {
		record := result.Record()
		tags = append(tags, TagNode{
			Name:           getStringFromRecord(record, 0),
			NormalizedName: getStringFromRecord(record, 1),
			UsageCount:     getIntFromRecord(record, 2),
		})
	}

	return tags, nil
}

func (g *FalkorDBGraph) exportTopics(ctx context.Context) ([]TopicNode, error) {
	query := `
		MATCH (t:Topic)
		RETURN t.name, t.normalized_name, t.usage_count
	`
	result, err := g.graph.Query(query)
	if err != nil {
		return nil, err
	}

	var topics []TopicNode
	for result.Next() {
		record := result.Record()
		topics = append(topics, TopicNode{
			Name:           getStringFromRecord(record, 0),
			NormalizedName: getStringFromRecord(record, 1),
			UsageCount:     getIntFromRecord(record, 2),
		})
	}

	return topics, nil
}

func (g *FalkorDBGraph) exportEntities(ctx context.Context) ([]EntityNode, error) {
	query := `
		MATCH (e:Entity)
		RETURN e.name, e.type, e.normalized_name, e.usage_count
	`
	result, err := g.graph.Query(query)
	if err != nil {
		return nil, err
	}

	var entities []EntityNode
	for result.Next() {
		record := result.Record()
		entities = append(entities, EntityNode{
			Name:           getStringFromRecord(record, 0),
			Type:           getStringFromRecord(record, 1),
			NormalizedName: getStringFromRecord(record, 2),
			UsageCount:     getIntFromRecord(record, 3),
		})
	}

	return entities, nil
}

func (g *FalkorDBGraph) countNodes(ctx context.Context, label string) (int, error) {
	query := fmt.Sprintf("MATCH (n:%s) RETURN count(n)", label)
	result, err := g.graph.Query(query)
	if err != nil {
		return 0, err
	}

	if result.Next() {
		return getIntFromRecord(result.Record(), 0), nil
	}

	return 0, nil
}

func (g *FalkorDBGraph) countRelationships(ctx context.Context) (int, error) {
	result, err := g.graph.Query("MATCH ()-[r]->() RETURN count(r)")
	if err != nil {
		return 0, err
	}

	if result.Next() {
		return getIntFromRecord(result.Record(), 0), nil
	}

	return 0, nil
}

// convertQueryResult converts RedisGraph result to our QueryResult type.
func convertQueryResult(result *redisgraph.QueryResult) *QueryResult {
	qr := &QueryResult{
		Stats: QueryStats{
			NodesCreated:     result.NodesCreated(),
			NodesDeleted:     result.NodesDeleted(),
			RelationsCreated: result.RelationshipsCreated(),
			RelationsDeleted: result.RelationshipsDeleted(),
			PropertiesSet:    result.PropertiesSet(),
			ExecutionTimeMs:  float64(result.RunTime()),
		},
	}

	for result.Next() {
		record := result.Record()
		values := record.Values()
		row := make([]any, len(values))
		copy(row, values)
		qr.Rows = append(qr.Rows, row)
	}

	return qr
}

// parseFileFromRecord parses a file node from query result record.
func parseFileFromRecord(record *redisgraph.Record) (*FileNode, error) {
	file := &FileNode{
		Path:            getStringFromRecord(record, 0),
		Name:            getStringFromRecord(record, 1),
		Extension:       getStringFromRecord(record, 2),
		MIMEType:        getStringFromRecord(record, 3),
		Language:        getStringFromRecord(record, 4),
		Size:            int64(getIntFromRecord(record, 5)),
		ModTime:         time.Unix(int64(getIntFromRecord(record, 6)), 0),
		ContentHash:     getStringFromRecord(record, 7),
		MetadataHash:    getStringFromRecord(record, 8),
		Summary:         getStringFromRecord(record, 9),
		Complexity:      getIntFromRecord(record, 10),
		AnalyzedAt:      time.Unix(int64(getIntFromRecord(record, 11)), 0),
		AnalysisVersion: getIntFromRecord(record, 12),
	}

	return file, nil
}

// Helper functions for type conversions from record

func getStringFromRecord(record *redisgraph.Record, index int) string {
	val := record.GetByIndex(index)
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

func getIntFromRecord(record *redisgraph.Record, index int) int {
	val := record.GetByIndex(index)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func getFloatFromRecord(record *redisgraph.Record, index int) float64 {
	val := record.GetByIndex(index)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func getBoolFromRecord(record *redisgraph.Record, index int) bool {
	val := record.GetByIndex(index)
	if val == nil {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	if s, ok := val.(string); ok {
		return s == "true"
	}
	return false
}

// escapeString escapes single quotes for Cypher queries.
func escapeString(s string) string {
	result := ""
	for _, c := range s {
		if c == '\'' {
			result += "\\'"
		} else if c == '\\' {
			result += "\\\\"
		} else {
			result += string(c)
		}
	}
	return result
}

// normalizeString converts a string to lowercase for matching.
func normalizeString(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else {
			result += string(c)
		}
	}
	return result
}
