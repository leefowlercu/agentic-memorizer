package graph

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Queries provides common query patterns for the knowledge graph
type Queries struct {
	client *Client
	logger *slog.Logger
}

// NewQueries creates a new Queries handler
func NewQueries(client *Client, logger *slog.Logger) *Queries {
	if logger == nil {
		logger = slog.Default()
	}
	return &Queries{
		client: client,
		logger: logger.With("component", "graph-queries"),
	}
}

// SearchResult represents a file search result
type SearchResult struct {
	Path         string   `json:"path"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Category     string   `json:"category"`
	Summary      string   `json:"summary"`
	DocumentType string   `json:"document_type"`
	Score        float64  `json:"score,omitempty"`
	MatchType    string   `json:"match_type,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Topics       []string `json:"topics,omitempty"`
}

// VectorSearch performs similarity search using embeddings
// The provider parameter determines which embedding property to search (e.g., "openai" -> "embedding_openai")
func (q *Queries) VectorSearch(ctx context.Context, embedding []float32, limit int, provider string) ([]SearchResult, error) {
	embeddingProp := EmbeddingPropertyName(provider)

	// Build query dynamically with provider-specific embedding property
	query := fmt.Sprintf(`
		CALL db.idx.vector.queryNodes('File', '%s', $limit, vecf32($embedding))
		YIELD node, score
		RETURN node.path, node.name, node.type, node.category, node.summary, node.document_type, score
		ORDER BY score DESC
	`, embeddingProp)

	// Convert []float32 to []interface{} for FalkorDB driver compatibility
	embeddingAny := make([]interface{}, len(embedding))
	for i, v := range embedding {
		embeddingAny[i] = float64(v)
	}

	params := map[string]any{
		"embedding": embeddingAny,
		"limit":     limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("vector search failed; %w", err)
	}

	var results []SearchResult
	for result.Next() {
		record := result.Record()
		results = append(results, SearchResult{
			Path:         record.GetString(0, ""),
			Name:         record.GetString(1, ""),
			Type:         record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Summary:      record.GetString(4, ""),
			DocumentType: record.GetString(5, ""),
			Score:        record.GetFloat64(6, 0),
			MatchType:    "vector_similarity",
		})
	}

	return results, nil
}

// FullTextSearch performs full-text search on file summaries
func (q *Queries) FullTextSearch(ctx context.Context, searchText string, limit int) ([]SearchResult, error) {
	query := `
		CALL db.idx.fulltext.queryNodes('File', $searchText)
		YIELD node, score
		RETURN node.path, node.name, node.type, node.category, node.summary, node.document_type, score
		ORDER BY score DESC
		LIMIT $limit
	`
	params := map[string]any{
		"searchText": searchText,
		"limit":      limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("full-text search failed; %w", err)
	}

	var results []SearchResult
	for result.Next() {
		record := result.Record()
		results = append(results, SearchResult{
			Path:         record.GetString(0, ""),
			Name:         record.GetString(1, ""),
			Type:         record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Summary:      record.GetString(4, ""),
			DocumentType: record.GetString(5, ""),
			Score:        record.GetFloat64(6, 0),
			MatchType:    "fulltext",
		})
	}

	return results, nil
}

// SearchByFilename searches files by filename substring
func (q *Queries) SearchByFilename(ctx context.Context, pattern string, limit int) ([]SearchResult, error) {
	query := `
		MATCH (f:File)
		WHERE toLower(f.name) CONTAINS toLower($pattern)
		RETURN f.path, f.name, f.type, f.category, f.summary, f.document_type
		ORDER BY f.modified DESC
		LIMIT $limit
	`
	params := map[string]any{
		"pattern": pattern,
		"limit":   limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("filename search failed; %w", err)
	}

	var results []SearchResult
	for result.Next() {
		record := result.Record()
		results = append(results, SearchResult{
			Path:         record.GetString(0, ""),
			Name:         record.GetString(1, ""),
			Type:         record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Summary:      record.GetString(4, ""),
			DocumentType: record.GetString(5, ""),
			MatchType:    "filename",
		})
	}

	return results, nil
}

// SearchByTag searches files by tag
func (q *Queries) SearchByTag(ctx context.Context, tagName string, limit int) ([]SearchResult, error) {
	query := `
		MATCH (f:File)-[:HAS_TAG]->(t:Tag)
		WHERE toLower(t.name) CONTAINS toLower($tagName)
		RETURN DISTINCT f.path, f.name, f.type, f.category, f.summary, f.document_type, t.name
		ORDER BY f.modified DESC
		LIMIT $limit
	`
	params := map[string]any{
		"tagName": tagName,
		"limit":   limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("tag search failed; %w", err)
	}

	var results []SearchResult
	for result.Next() {
		record := result.Record()
		results = append(results, SearchResult{
			Path:         record.GetString(0, ""),
			Name:         record.GetString(1, ""),
			Type:         record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Summary:      record.GetString(4, ""),
			DocumentType: record.GetString(5, ""),
			MatchType:    "tag",
		})
	}

	return results, nil
}

// SearchByTopic searches files by topic
func (q *Queries) SearchByTopic(ctx context.Context, topicName string, limit int) ([]SearchResult, error) {
	query := `
		MATCH (f:File)-[:COVERS_TOPIC]->(t:Topic)
		WHERE toLower(t.name) CONTAINS toLower($topicName)
		RETURN DISTINCT f.path, f.name, f.type, f.category, f.summary, f.document_type, t.name
		ORDER BY f.modified DESC
		LIMIT $limit
	`
	params := map[string]any{
		"topicName": topicName,
		"limit":     limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("topic search failed; %w", err)
	}

	var results []SearchResult
	for result.Next() {
		record := result.Record()
		results = append(results, SearchResult{
			Path:         record.GetString(0, ""),
			Name:         record.GetString(1, ""),
			Type:         record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Summary:      record.GetString(4, ""),
			DocumentType: record.GetString(5, ""),
			MatchType:    "topic",
		})
	}

	return results, nil
}

// SearchByEntity searches files mentioning an entity
func (q *Queries) SearchByEntity(ctx context.Context, entityName string, limit int) ([]SearchResult, error) {
	query := `
		MATCH (f:File)-[:MENTIONS]->(e:Entity)
		WHERE toLower(e.name) CONTAINS toLower($entityName)
		   OR e.normalized CONTAINS toLower($entityName)
		RETURN DISTINCT f.path, f.name, f.type, f.category, f.summary, f.document_type, e.name
		ORDER BY f.modified DESC
		LIMIT $limit
	`
	params := map[string]any{
		"entityName": entityName,
		"limit":      limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("entity search failed; %w", err)
	}

	var results []SearchResult
	for result.Next() {
		record := result.Record()
		results = append(results, SearchResult{
			Path:         record.GetString(0, ""),
			Name:         record.GetString(1, ""),
			Type:         record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Summary:      record.GetString(4, ""),
			DocumentType: record.GetString(5, ""),
			MatchType:    "entity",
		})
	}

	return results, nil
}

// SearchByCategory returns files in a category
func (q *Queries) SearchByCategory(ctx context.Context, category string, limit int) ([]SearchResult, error) {
	query := `
		MATCH (f:File)-[:IN_CATEGORY]->(c:Category {name: $category})
		RETURN f.path, f.name, f.type, f.category, f.summary, f.document_type
		ORDER BY f.modified DESC
		LIMIT $limit
	`
	params := map[string]any{
		"category": strings.ToLower(category),
		"limit":    limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("category search failed; %w", err)
	}

	var results []SearchResult
	for result.Next() {
		record := result.Record()
		results = append(results, SearchResult{
			Path:         record.GetString(0, ""),
			Name:         record.GetString(1, ""),
			Type:         record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Summary:      record.GetString(4, ""),
			DocumentType: record.GetString(5, ""),
			MatchType:    "category",
		})
	}

	return results, nil
}

// GetRecentFiles returns recently modified files
func (q *Queries) GetRecentFiles(ctx context.Context, days int, limit int) ([]SearchResult, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

	query := `
		MATCH (f:File)
		WHERE f.modified >= $cutoff
		RETURN f.path, f.name, f.type, f.category, f.summary, f.document_type
		ORDER BY f.modified DESC
		LIMIT $limit
	`
	params := map[string]any{
		"cutoff": cutoff,
		"limit":  limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("recent files query failed; %w", err)
	}

	var results []SearchResult
	for result.Next() {
		record := result.Record()
		results = append(results, SearchResult{
			Path:         record.GetString(0, ""),
			Name:         record.GetString(1, ""),
			Type:         record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Summary:      record.GetString(4, ""),
			DocumentType: record.GetString(5, ""),
			MatchType:    "recent",
		})
	}

	return results, nil
}

// GetRelatedFiles finds files related to a given file through shared tags/topics/entities
func (q *Queries) GetRelatedFiles(ctx context.Context, filePath string, limit int) ([]RelatedFile, error) {
	query := `
		MATCH (f:File {path: $filePath})
		OPTIONAL MATCH (f)-[:HAS_TAG]->(t:Tag)<-[:HAS_TAG]-(related:File)
		WHERE f <> related
		WITH related, count(t) as tagCount
		OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(topic:Topic)<-[:COVERS_TOPIC]-(related2:File)
		WHERE f <> related2
		WITH COALESCE(related, related2) as rel, tagCount, count(topic) as topicCount
		WHERE rel IS NOT NULL
		RETURN rel.path, rel.name, rel.summary,
		       tagCount + topicCount as strength,
		       CASE WHEN tagCount > 0 THEN 'shared_tags' ELSE 'shared_topics' END as connection_type
		ORDER BY strength DESC
		LIMIT $limit
	`
	params := map[string]any{
		"filePath": filePath,
		"limit":    limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("related files query failed; %w", err)
	}

	var results []RelatedFile
	for result.Next() {
		record := result.Record()
		results = append(results, RelatedFile{
			Path:           record.GetString(0, ""),
			Name:           record.GetString(1, ""),
			Summary:        record.GetString(2, ""),
			Strength:       record.GetInt64(3, 0),
			ConnectionType: record.GetString(4, ""),
		})
	}

	return results, nil
}

// RelatedFile represents a file related to another
type RelatedFile struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Summary        string `json:"summary"`
	Strength       int64  `json:"strength"`
	ConnectionType string `json:"connection_type"`
}

// GetFileConnections returns all connections for a file
func (q *Queries) GetFileConnections(ctx context.Context, filePath string) (*FileConnections, error) {
	conn := &FileConnections{
		FilePath: filePath,
	}

	// Get tags
	tagQuery := `
		MATCH (f:File {path: $filePath})-[:HAS_TAG]->(t:Tag)
		RETURN t.name
	`
	result, err := q.client.Query(ctx, tagQuery, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file tags; %w", err)
	}
	for result.Next() {
		conn.Tags = append(conn.Tags, result.Record().GetString(0, ""))
	}

	// Get topics
	topicQuery := `
		MATCH (f:File {path: $filePath})-[:COVERS_TOPIC]->(t:Topic)
		RETURN t.name
	`
	result, err = q.client.Query(ctx, topicQuery, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file topics; %w", err)
	}
	for result.Next() {
		conn.Topics = append(conn.Topics, result.Record().GetString(0, ""))
	}

	// Get entities
	entityQuery := `
		MATCH (f:File {path: $filePath})-[:MENTIONS]->(e:Entity)
		RETURN e.name, e.type
	`
	result, err = q.client.Query(ctx, entityQuery, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file entities; %w", err)
	}
	for result.Next() {
		record := result.Record()
		conn.Entities = append(conn.Entities, EntityInfo{
			Name: record.GetString(0, ""),
			Type: record.GetString(1, ""),
		})
	}

	// Get category
	categoryQuery := `
		MATCH (f:File {path: $filePath})-[:IN_CATEGORY]->(c:Category)
		RETURN c.name
	`
	result, err = q.client.Query(ctx, categoryQuery, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file category; %w", err)
	}
	if result.Next() {
		conn.Category = result.Record().GetString(0, "")
	}

	return conn, nil
}

// FileConnections represents all graph connections for a file
type FileConnections struct {
	FilePath string       `json:"file_path"`
	Tags     []string     `json:"tags"`
	Topics   []string     `json:"topics"`
	Entities []EntityInfo `json:"entities"`
	Category string       `json:"category"`
}

// EntityInfo represents basic entity information
type EntityInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// FindFilesWithSharedEntities finds files that share entities with the given file
func (q *Queries) FindFilesWithSharedEntities(ctx context.Context, filePath string, limit int) ([]SharedEntityFile, error) {
	query := `
		MATCH (f1:File {path: $filePath})-[:MENTIONS]->(e:Entity)<-[:MENTIONS]-(f2:File)
		WHERE f1 <> f2
		RETURN f2.path, f2.name, f2.summary, collect(e.name) as shared_entities, count(e) as strength
		ORDER BY strength DESC
		LIMIT $limit
	`
	params := map[string]any{
		"filePath": filePath,
		"limit":    limit,
	}

	result, err := q.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("shared entities query failed; %w", err)
	}

	var results []SharedEntityFile
	for result.Next() {
		record := result.Record()

		// Parse shared entities array
		var entities []string
		if arr, ok := record.GetByIndex(3).([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					entities = append(entities, s)
				}
			}
		}

		results = append(results, SharedEntityFile{
			Path:           record.GetString(0, ""),
			Name:           record.GetString(1, ""),
			Summary:        record.GetString(2, ""),
			SharedEntities: entities,
			Strength:       record.GetInt64(4, 0),
		})
	}

	return results, nil
}

// SharedEntityFile represents a file that shares entities with another
type SharedEntityFile struct {
	Path           string   `json:"path"`
	Name           string   `json:"name"`
	Summary        string   `json:"summary"`
	SharedEntities []string `json:"shared_entities"`
	Strength       int64    `json:"strength"`
}

// GetGraphOverview returns high-level statistics about the graph
func (q *Queries) GetGraphOverview(ctx context.Context) (*GraphOverview, error) {
	overview := &GraphOverview{}

	// Count files
	result, err := q.client.Query(ctx, "MATCH (f:File) RETURN count(f)", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count files; %w", err)
	}
	if result.Next() {
		overview.FileCount = result.Record().GetInt64(0, 0)
	}

	// Count tags
	result, err = q.client.Query(ctx, "MATCH (t:Tag) RETURN count(t)", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count tags; %w", err)
	}
	if result.Next() {
		overview.TagCount = result.Record().GetInt64(0, 0)
	}

	// Count topics
	result, err = q.client.Query(ctx, "MATCH (t:Topic) RETURN count(t)", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count topics; %w", err)
	}
	if result.Next() {
		overview.TopicCount = result.Record().GetInt64(0, 0)
	}

	// Count entities
	result, err = q.client.Query(ctx, "MATCH (e:Entity) RETURN count(e)", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count entities; %w", err)
	}
	if result.Next() {
		overview.EntityCount = result.Record().GetInt64(0, 0)
	}

	// Count relationships
	result, err = q.client.Query(ctx, "MATCH ()-[r]->() RETURN count(r)", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count relationships; %w", err)
	}
	if result.Next() {
		overview.RelationshipCount = result.Record().GetInt64(0, 0)
	}

	// Get category distribution
	result, err = q.client.Query(ctx, `
		MATCH (f:File)-[:IN_CATEGORY]->(c:Category)
		RETURN c.name, count(f) as count
		ORDER BY count DESC
	`, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get category distribution; %w", err)
	}
	overview.CategoryDistribution = make(map[string]int64)
	for result.Next() {
		record := result.Record()
		overview.CategoryDistribution[record.GetString(0, "")] = record.GetInt64(1, 0)
	}

	return overview, nil
}

// GraphOverview contains high-level graph statistics
type GraphOverview struct {
	FileCount            int64            `json:"file_count"`
	TagCount             int64            `json:"tag_count"`
	TopicCount           int64            `json:"topic_count"`
	EntityCount          int64            `json:"entity_count"`
	RelationshipCount    int64            `json:"relationship_count"`
	CategoryDistribution map[string]int64 `json:"category_distribution"`
}
