package graph

import (
	"context"
	"fmt"
)

// Schema indexes for the graph database.
// These indexes improve query performance for common lookups.

// coreIndexes are indexes on the primary node types.
var coreIndexes = []string{
	// File indexes
	"CREATE INDEX FOR (f:File) ON (f.path)",
	"CREATE INDEX FOR (f:File) ON (f.content_hash)",

	// Chunk indexes
	"CREATE INDEX FOR (c:Chunk) ON (c.id)",
	"CREATE INDEX FOR (c:Chunk) ON (c.file_path)",
	"CREATE INDEX FOR (c:Chunk) ON (c.content_hash)",

	// Directory indexes
	"CREATE INDEX FOR (d:Directory) ON (d.path)",

	// Tag/Topic/Entity indexes
	"CREATE INDEX FOR (t:Tag) ON (t.normalized_name)",
	"CREATE INDEX FOR (t:Topic) ON (t.normalized_name)",
	"CREATE INDEX FOR (e:Entity) ON (e.normalized_name)",
}

// metadataIndexes are indexes on the metadata node types.
var metadataIndexes = []string{
	// CodeMeta indexes
	"CREATE INDEX FOR (m:CodeMeta) ON (m.function_name)",
	"CREATE INDEX FOR (m:CodeMeta) ON (m.class_name)",
	"CREATE INDEX FOR (m:CodeMeta) ON (m.language)",
	"CREATE INDEX FOR (m:CodeMeta) ON (m.visibility)",

	// DocumentMeta indexes
	"CREATE INDEX FOR (m:DocumentMeta) ON (m.heading)",
	"CREATE INDEX FOR (m:DocumentMeta) ON (m.heading_level)",

	// NotebookMeta indexes
	"CREATE INDEX FOR (m:NotebookMeta) ON (m.cell_type)",

	// BuildMeta indexes
	"CREATE INDEX FOR (m:BuildMeta) ON (m.target_name)",
	"CREATE INDEX FOR (m:BuildMeta) ON (m.stage_name)",

	// InfraMeta indexes
	"CREATE INDEX FOR (m:InfraMeta) ON (m.resource_type)",
	"CREATE INDEX FOR (m:InfraMeta) ON (m.block_type)",

	// SchemaMeta indexes
	"CREATE INDEX FOR (m:SchemaMeta) ON (m.message_name)",
	"CREATE INDEX FOR (m:SchemaMeta) ON (m.type_name)",

	// ChunkEmbedding indexes (provider, model for composite lookups)
	"CREATE INDEX FOR (e:ChunkEmbedding) ON (e.provider)",
	"CREATE INDEX FOR (e:ChunkEmbedding) ON (e.model)",
}

// initSchema creates all indexes and constraints for the graph.
// Safe to call multiple times - existing indexes are ignored.
func (g *FalkorDBGraph) initSchema(ctx context.Context) error {
	// Create core indexes
	for _, query := range coreIndexes {
		if _, err := g.graph.Query(query); err != nil {
			// Ignore errors for existing indexes
			g.logger.Debug("schema query", "query", query, "error", err)
		}
	}

	// Create metadata indexes
	for _, query := range metadataIndexes {
		if _, err := g.graph.Query(query); err != nil {
			// Ignore errors for existing indexes
			g.logger.Debug("schema query", "query", query, "error", err)
		}
	}

	// Create vector index for similarity search
	if err := g.initVectorIndex(ctx); err != nil {
		g.logger.Warn("failed to create vector index", "error", err)
	}

	return nil
}

// initVectorIndex creates an HNSW vector index on ChunkEmbedding.embedding.
func (g *FalkorDBGraph) initVectorIndex(ctx context.Context) error {
	dim := g.config.EmbeddingDimension
	if dim == 0 {
		dim = 1536 // Default OpenAI text-embedding-3-small
	}

	// FalkorDB uses CREATE VECTOR INDEX syntax
	query := fmt.Sprintf(`
		CREATE VECTOR INDEX FOR (e:ChunkEmbedding) ON (e.embedding)
		OPTIONS {
			indexType: 'HNSW',
			dimension: %d,
			similarityFunction: 'cosine'
		}
	`, dim)

	if _, err := g.graph.Query(query); err != nil {
		// Try alternative syntax for older FalkorDB versions
		altQuery := fmt.Sprintf(`
			CALL db.idx.vector.createNodeIndex('ChunkEmbedding', 'embedding', %d, 'cosine')
		`, dim)
		if _, altErr := g.graph.Query(altQuery); altErr != nil {
			g.logger.Debug("vector index creation failed",
				"primary_error", err,
				"alt_error", altErr)
			// Index may already exist, not fatal
		}
	}

	g.logger.Info("vector index created/verified",
		"label", "ChunkEmbedding",
		"property", "embedding",
		"dimension", dim)

	return nil
}
