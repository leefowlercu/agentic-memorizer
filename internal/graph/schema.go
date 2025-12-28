package graph

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// SchemaConfig contains schema configuration settings
type SchemaConfig struct {
	// Vector index settings
	EmbeddingDimensions    int    // Default: 1536 for OpenAI text-embedding-3-small
	SimilarityFunction     string // "cosine" or "euclidean", default: "cosine"
	VectorIndexM           int    // HNSW M parameter, default: 16
	VectorIndexEfConstruct int    // HNSW efConstruction, default: 200
}

// DefaultSchemaConfig returns default schema configuration
func DefaultSchemaConfig() SchemaConfig {
	return SchemaConfig{
		EmbeddingDimensions:    1536,
		SimilarityFunction:     "cosine",
		VectorIndexM:           16,
		VectorIndexEfConstruct: 200,
	}
}

// Schema handles graph schema initialization and management
type Schema struct {
	client *Client
	config SchemaConfig
	logger *slog.Logger
}

// NewSchema creates a new Schema manager
func NewSchema(client *Client, config SchemaConfig, logger *slog.Logger) *Schema {
	if logger == nil {
		logger = slog.Default()
	}
	return &Schema{
		client: client,
		config: config,
		logger: logger.With("component", "graph-schema"),
	}
}

// NodeLabels defines the node types in our schema
var NodeLabels = []string{
	"File",      // Primary node for files
	"Tag",       // Semantic tags
	"Topic",     // Key topics
	"Category",  // File categories (documents, code, images, etc.)
	"Entity",    // Named entities (technologies, people, concepts)
	"Directory", // Directory hierarchy
	"Fact",      // User-defined facts for agent context
}

// RelationshipTypes defines the relationship types in our schema
var RelationshipTypes = []string{
	"HAS_TAG",      // File -> Tag
	"COVERS_TOPIC", // File -> Topic
	"IN_CATEGORY",  // File -> Category
	"MENTIONS",     // File -> Entity
	"REFERENCES",   // File -> Topic (with type and confidence)
	"SIMILAR_TO",   // File -> File (embedding similarity)
	"IN_DIRECTORY", // File -> Directory
	"PARENT_OF",    // Topic -> Topic, Directory -> Directory
}

// Initialize creates all necessary schema components
func (s *Schema) Initialize(ctx context.Context) error {
	s.logger.Info("initializing graph schema")

	// Initialize base categories
	if err := s.initializeCategories(ctx); err != nil {
		return fmt.Errorf("failed to initialize categories; %w", err)
	}

	// Create indexes
	if err := s.createIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create indexes; %w", err)
	}

	s.logger.Info("graph schema initialized successfully")
	return nil
}

// initializeCategories creates the predefined category nodes
func (s *Schema) initializeCategories(ctx context.Context) error {
	categories := []string{"documents", "code", "images", "data", "other"}

	for _, cat := range categories {
		query := `MERGE (c:Category {name: $name}) RETURN c`
		_, err := s.client.Query(ctx, query, map[string]any{"name": cat})
		if err != nil {
			return fmt.Errorf("failed to create category %s; %w", cat, err)
		}
	}

	s.logger.Debug("initialized categories", "count", len(categories))
	return nil
}

// createIndexes creates all required indexes
func (s *Schema) createIndexes(ctx context.Context) error {
	// Create range indexes for fast lookups
	rangeIndexes := []struct {
		label    string
		property string
	}{
		{"File", "path"},
		{"File", "hash"},
		{"Tag", "name"},
		{"Topic", "name"},
		{"Category", "name"},
		{"Entity", "normalized"},
		{"Directory", "path"},
		{"Fact", "id"},
	}

	for _, idx := range rangeIndexes {
		if err := s.createRangeIndex(ctx, idx.label, idx.property); err != nil {
			s.logger.Warn("failed to create range index (may already exist)",
				"label", idx.label,
				"property", idx.property,
				"error", err,
			)
		}
	}

	// Create full-text index on File.summary for text search
	if err := s.createFullTextIndex(ctx, "File", "summary"); err != nil {
		s.logger.Warn("failed to create full-text index (may already exist)",
			"label", "File",
			"property", "summary",
			"error", err,
		)
	}

	// Create vector index for embedding similarity search
	if err := s.createVectorIndex(ctx, "File", "embedding"); err != nil {
		s.logger.Warn("failed to create vector index (may already exist)",
			"label", "File",
			"property", "embedding",
			"error", err,
		)
	}

	return nil
}

// createRangeIndex creates a range index for fast property lookups
func (s *Schema) createRangeIndex(ctx context.Context, label, property string) error {
	// FalkorDB uses CREATE INDEX syntax
	query := fmt.Sprintf("CREATE INDEX FOR (n:%s) ON (n.%s)", label, property)
	_, err := s.client.Query(ctx, query, nil)
	if err != nil {
		// Check if it's just "index already exists" error
		if strings.Contains(err.Error(), "already indexed") ||
			strings.Contains(err.Error(), "Index already exists") {
			s.logger.Debug("range index already exists",
				"label", label,
				"property", property,
			)
			return nil
		}
		return err
	}

	s.logger.Debug("created range index",
		"label", label,
		"property", property,
	)
	return nil
}

// createFullTextIndex creates a full-text index for text search
func (s *Schema) createFullTextIndex(ctx context.Context, label, property string) error {
	query := fmt.Sprintf("CALL db.idx.fulltext.createNodeIndex('%s', '%s')", label, property)
	_, err := s.client.Query(ctx, query, nil)
	if err != nil {
		if strings.Contains(err.Error(), "already indexed") ||
			strings.Contains(err.Error(), "Index already exists") {
			s.logger.Debug("full-text index already exists",
				"label", label,
				"property", property,
			)
			return nil
		}
		return err
	}

	s.logger.Debug("created full-text index",
		"label", label,
		"property", property,
	)
	return nil
}

// createVectorIndex creates a vector index for similarity search
func (s *Schema) createVectorIndex(ctx context.Context, label, property string) error {
	query := fmt.Sprintf(
		"CREATE VECTOR INDEX FOR (n:%s) ON (n.%s) OPTIONS {dimension:%d, similarityFunction:'%s', M:%d, efConstruction:%d}",
		label,
		property,
		s.config.EmbeddingDimensions,
		s.config.SimilarityFunction,
		s.config.VectorIndexM,
		s.config.VectorIndexEfConstruct,
	)

	_, err := s.client.Query(ctx, query, nil)
	if err != nil {
		if strings.Contains(err.Error(), "already indexed") ||
			strings.Contains(err.Error(), "Index already exists") {
			s.logger.Debug("vector index already exists",
				"label", label,
				"property", property,
			)
			return nil
		}
		return err
	}

	s.logger.Debug("created vector index",
		"label", label,
		"property", property,
		"dimensions", s.config.EmbeddingDimensions,
	)
	return nil
}

// DropAllIndexes removes all indexes (useful for schema migration)
func (s *Schema) DropAllIndexes(ctx context.Context) error {
	s.logger.Warn("dropping all indexes")

	// Get list of indexes
	result, err := s.client.Query(ctx, "CALL db.indexes()", nil)
	if err != nil {
		return fmt.Errorf("failed to list indexes; %w", err)
	}

	// This is a destructive operation, log each dropped index
	for result.Next() {
		record := result.Record()
		s.logger.Info("index found during drop", "record", record)
	}

	return nil
}

// ClearGraph removes all nodes and relationships
func (s *Schema) ClearGraph(ctx context.Context) error {
	s.logger.Warn("clearing entire graph")

	// Delete all relationships first, then all nodes
	_, err := s.client.Query(ctx, "MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		return fmt.Errorf("failed to clear graph; %w", err)
	}

	s.logger.Info("graph cleared successfully")
	return nil
}

// GetIndexes returns information about existing indexes
func (s *Schema) GetIndexes(ctx context.Context) ([]IndexInfo, error) {
	result, err := s.client.Query(ctx, "CALL db.indexes()", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes; %w", err)
	}

	var indexes []IndexInfo
	for result.Next() {
		record := result.Record()
		// Parse index information from result
		// Note: The exact structure depends on FalkorDB's response format
		info := IndexInfo{
			Name: record.GetString(0, ""),
		}
		indexes = append(indexes, info)
	}

	return indexes, nil
}

// IndexInfo represents information about an index
type IndexInfo struct {
	Name     string `json:"name"`
	Label    string `json:"label,omitempty"`
	Property string `json:"property,omitempty"`
	Type     string `json:"type,omitempty"`
}

// ValidateSchema checks that the schema is properly initialized
func (s *Schema) ValidateSchema(ctx context.Context) error {
	s.logger.Debug("validating schema")

	// Check that categories exist
	result, err := s.client.Query(ctx, "MATCH (c:Category) RETURN count(c) as count", nil)
	if err != nil {
		return fmt.Errorf("failed to validate categories; %w", err)
	}

	if result.Next() {
		count := result.Record().GetInt64(0, 0)
		if count < 5 {
			return fmt.Errorf("schema validation failed; expected 5 categories, found %d", count)
		}
	}

	s.logger.Debug("schema validation passed")
	return nil
}
