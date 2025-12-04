package graph

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Edges handles edge (relationship) CRUD operations
type Edges struct {
	client *Client
	logger *slog.Logger
}

// NewEdges creates a new Edges handler
func NewEdges(client *Client, logger *slog.Logger) *Edges {
	if logger == nil {
		logger = slog.Default()
	}
	return &Edges{
		client: client,
		logger: logger.With("component", "graph-edges"),
	}
}

// LinkFileToTag creates a HAS_TAG relationship between File and Tag
func (e *Edges) LinkFileToTag(ctx context.Context, filePath, tagName string) error {
	query := `
		MATCH (f:File {path: $filePath})
		MERGE (t:Tag {name: $tagName})
		MERGE (f)-[:HAS_TAG]->(t)
	`
	params := map[string]any{
		"filePath": filePath,
		"tagName":  strings.ToLower(tagName),
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link file to tag; %w", err)
	}
	return nil
}

// LinkFileToTopic creates a COVERS_TOPIC relationship between File and Topic
func (e *Edges) LinkFileToTopic(ctx context.Context, filePath, topicName string) error {
	query := `
		MATCH (f:File {path: $filePath})
		MERGE (t:Topic {name: $topicName})
		MERGE (f)-[:COVERS_TOPIC]->(t)
	`
	params := map[string]any{
		"filePath":  filePath,
		"topicName": strings.ToLower(topicName),
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link file to topic; %w", err)
	}
	return nil
}

// LinkFileToCategory creates an IN_CATEGORY relationship between File and Category
func (e *Edges) LinkFileToCategory(ctx context.Context, filePath, categoryName string) error {
	query := `
		MATCH (f:File {path: $filePath})
		MATCH (c:Category {name: $categoryName})
		MERGE (f)-[:IN_CATEGORY]->(c)
	`
	params := map[string]any{
		"filePath":     filePath,
		"categoryName": strings.ToLower(categoryName),
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link file to category; %w", err)
	}
	return nil
}

// LinkFileToEntity creates a MENTIONS relationship between File and Entity
func (e *Edges) LinkFileToEntity(ctx context.Context, filePath, entityName, entityType string) error {
	// Apply normalization with alias resolution
	normalized := normalizeEntityForGraph(entityName)

	query := `
		MATCH (f:File {path: $filePath})
		MERGE (ent:Entity {normalized: $normalized})
		ON CREATE SET ent.name = $entityName, ent.type = $entityType
		MERGE (f)-[:MENTIONS]->(ent)
	`
	params := map[string]any{
		"filePath":   filePath,
		"entityName": entityName,
		"entityType": entityType,
		"normalized": normalized,
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link file to entity; %w", err)
	}
	return nil
}

// normalizeEntityForGraph applies entity normalization with common alias resolution
func normalizeEntityForGraph(name string) string {
	// Basic normalization
	normalized := strings.TrimSpace(strings.ToLower(name))

	// Common technology aliases
	aliases := map[string]string{
		"tf":       "terraform",
		"k8s":      "kubernetes",
		"js":       "javascript",
		"ts":       "typescript",
		"py":       "python",
		"go":       "golang",
		"pg":       "postgresql",
		"postgres": "postgresql",
		"mongo":    "mongodb",
		"node":     "nodejs",
		"gh":       "github",
		"gl":       "gitlab",
		"aws":      "amazon web services",
		"gcp":      "google cloud platform",
		"azure":    "microsoft azure",
		"mcp":      "model context protocol",
	}

	if canonical, ok := aliases[normalized]; ok {
		return canonical
	}
	return normalized
}

// LinkFileToReference creates a REFERENCES relationship between File and Topic
func (e *Edges) LinkFileToReference(ctx context.Context, filePath, topicName, refType string, confidence float64) error {
	query := `
		MATCH (f:File {path: $filePath})
		MERGE (t:Topic {name: $topicName})
		MERGE (f)-[r:REFERENCES]->(t)
		SET r.type = $refType, r.confidence = $confidence
	`
	params := map[string]any{
		"filePath":   filePath,
		"topicName":  strings.ToLower(topicName),
		"refType":    refType,
		"confidence": confidence,
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link file reference; %w", err)
	}
	return nil
}

// LinkFileToDirectory creates an IN_DIRECTORY relationship between File and Directory
func (e *Edges) LinkFileToDirectory(ctx context.Context, filePath, dirPath string) error {
	query := `
		MATCH (f:File {path: $filePath})
		MERGE (d:Directory {path: $dirPath})
		MERGE (f)-[:IN_DIRECTORY]->(d)
	`
	params := map[string]any{
		"filePath": filePath,
		"dirPath":  dirPath,
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link file to directory; %w", err)
	}
	return nil
}

// LinkSimilarFiles creates a SIMILAR_TO relationship between two files
func (e *Edges) LinkSimilarFiles(ctx context.Context, filePath1, filePath2 string, score float64) error {
	query := `
		MATCH (f1:File {path: $filePath1})
		MATCH (f2:File {path: $filePath2})
		MERGE (f1)-[r:SIMILAR_TO]->(f2)
		SET r.score = $score
	`
	params := map[string]any{
		"filePath1": filePath1,
		"filePath2": filePath2,
		"score":     score,
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link similar files; %w", err)
	}
	return nil
}

// LinkTopicParent creates a PARENT_OF relationship between topics
func (e *Edges) LinkTopicParent(ctx context.Context, parentTopic, childTopic string) error {
	query := `
		MERGE (parent:Topic {name: $parentTopic})
		MERGE (child:Topic {name: $childTopic})
		MERGE (parent)-[:PARENT_OF]->(child)
	`
	params := map[string]any{
		"parentTopic": strings.ToLower(parentTopic),
		"childTopic":  strings.ToLower(childTopic),
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link topic parent; %w", err)
	}
	return nil
}

// LinkDirectoryParent creates a PARENT_OF relationship between directories
func (e *Edges) LinkDirectoryParent(ctx context.Context, parentDir, childDir string) error {
	query := `
		MERGE (parent:Directory {path: $parentDir})
		MERGE (child:Directory {path: $childDir})
		MERGE (parent)-[:PARENT_OF]->(child)
	`
	params := map[string]any{
		"parentDir": parentDir,
		"childDir":  childDir,
	}

	_, err := e.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to link directory parent; %w", err)
	}
	return nil
}

// RemoveFileEdges removes all edges from a File node
func (e *Edges) RemoveFileEdges(ctx context.Context, filePath string) error {
	query := `
		MATCH (f:File {path: $filePath})-[r]-()
		DELETE r
	`
	_, err := e.client.Query(ctx, query, map[string]any{"filePath": filePath})
	if err != nil {
		return fmt.Errorf("failed to remove file edges; %w", err)
	}
	return nil
}

// GetFileTags returns all tags for a file
func (e *Edges) GetFileTags(ctx context.Context, filePath string) ([]string, error) {
	query := `
		MATCH (f:File {path: $filePath})-[:HAS_TAG]->(t:Tag)
		RETURN t.name
		ORDER BY t.name
	`
	result, err := e.client.Query(ctx, query, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file tags; %w", err)
	}

	var tags []string
	for result.Next() {
		tags = append(tags, result.Record().GetString(0, ""))
	}
	return tags, nil
}

// GetFileTopics returns all topics for a file
func (e *Edges) GetFileTopics(ctx context.Context, filePath string) ([]string, error) {
	query := `
		MATCH (f:File {path: $filePath})-[:COVERS_TOPIC]->(t:Topic)
		RETURN t.name
		ORDER BY t.name
	`
	result, err := e.client.Query(ctx, query, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file topics; %w", err)
	}

	var topics []string
	for result.Next() {
		topics = append(topics, result.Record().GetString(0, ""))
	}
	return topics, nil
}

// GetFileEntities returns all entities mentioned in a file
func (e *Edges) GetFileEntities(ctx context.Context, filePath string) ([]EntityNode, error) {
	query := `
		MATCH (f:File {path: $filePath})-[:MENTIONS]->(ent:Entity)
		RETURN ent.name, ent.type, ent.normalized
		ORDER BY ent.name
	`
	result, err := e.client.Query(ctx, query, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file entities; %w", err)
	}

	var entities []EntityNode
	for result.Next() {
		record := result.Record()
		entities = append(entities, EntityNode{
			Name:       record.GetString(0, ""),
			Type:       record.GetString(1, ""),
			Normalized: record.GetString(2, ""),
		})
	}
	return entities, nil
}

// GetFileReferences returns all topics referenced by a file
func (e *Edges) GetFileReferences(ctx context.Context, filePath string) ([]Reference, error) {
	query := `
		MATCH (f:File {path: $filePath})-[r:REFERENCES]->(t:Topic)
		RETURN t.name, r.type, r.confidence
		ORDER BY r.confidence DESC
	`
	result, err := e.client.Query(ctx, query, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file references; %w", err)
	}

	var refs []Reference
	for result.Next() {
		record := result.Record()
		refs = append(refs, Reference{
			Topic:      record.GetString(0, ""),
			RefType:    record.GetString(1, ""),
			Confidence: record.GetFloat64(2, 0),
		})
	}
	return refs, nil
}

// Reference represents a reference relationship
type Reference struct {
	Topic      string  `json:"topic"`
	RefType    string  `json:"ref_type"`
	Confidence float64 `json:"confidence"`
}

// GetSimilarFiles returns files similar to the given file
func (e *Edges) GetSimilarFiles(ctx context.Context, filePath string, limit int) ([]SimilarFile, error) {
	query := `
		MATCH (f:File {path: $filePath})-[r:SIMILAR_TO]->(similar:File)
		RETURN similar.path, similar.name, similar.summary, r.score
		ORDER BY r.score DESC
		LIMIT $limit
	`
	params := map[string]any{
		"filePath": filePath,
		"limit":    limit,
	}

	result, err := e.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get similar files; %w", err)
	}

	var files []SimilarFile
	for result.Next() {
		record := result.Record()
		files = append(files, SimilarFile{
			Path:    record.GetString(0, ""),
			Name:    record.GetString(1, ""),
			Summary: record.GetString(2, ""),
			Score:   record.GetFloat64(3, 0),
		})
	}
	return files, nil
}

// SimilarFile represents a file with similarity score
type SimilarFile struct {
	Path    string  `json:"path"`
	Name    string  `json:"name"`
	Summary string  `json:"summary"`
	Score   float64 `json:"score"`
}

// GetFilesWithTag returns all files with a specific tag
func (e *Edges) GetFilesWithTag(ctx context.Context, tagName string) ([]FileNode, error) {
	query := `
		MATCH (f:File)-[:HAS_TAG]->(t:Tag {name: $tagName})
		RETURN f.path, f.hash, f.name, f.type, f.category, f.size, f.modified,
		       f.summary, f.document_type, f.confidence
		ORDER BY f.modified DESC
	`
	result, err := e.client.Query(ctx, query, map[string]any{"tagName": strings.ToLower(tagName)})
	if err != nil {
		return nil, fmt.Errorf("failed to get files with tag; %w", err)
	}

	var files []FileNode
	for result.Next() {
		record := result.Record()
		files = append(files, FileNode{
			Path:         record.GetString(0, ""),
			Hash:         record.GetString(1, ""),
			Name:         record.GetString(2, ""),
			Type:         record.GetString(3, ""),
			Category:     record.GetString(4, ""),
			Size:         record.GetInt64(5, 0),
			Summary:      record.GetString(7, ""),
			DocumentType: record.GetString(8, ""),
			Confidence:   record.GetFloat64(9, 0),
		})
	}
	return files, nil
}

// GetFilesWithTopic returns all files covering a specific topic
func (e *Edges) GetFilesWithTopic(ctx context.Context, topicName string) ([]FileNode, error) {
	query := `
		MATCH (f:File)-[:COVERS_TOPIC]->(t:Topic {name: $topicName})
		RETURN f.path, f.hash, f.name, f.type, f.category, f.size, f.modified,
		       f.summary, f.document_type, f.confidence
		ORDER BY f.modified DESC
	`
	result, err := e.client.Query(ctx, query, map[string]any{"topicName": strings.ToLower(topicName)})
	if err != nil {
		return nil, fmt.Errorf("failed to get files with topic; %w", err)
	}

	var files []FileNode
	for result.Next() {
		record := result.Record()
		files = append(files, FileNode{
			Path:         record.GetString(0, ""),
			Hash:         record.GetString(1, ""),
			Name:         record.GetString(2, ""),
			Type:         record.GetString(3, ""),
			Category:     record.GetString(4, ""),
			Size:         record.GetInt64(5, 0),
			Summary:      record.GetString(7, ""),
			DocumentType: record.GetString(8, ""),
			Confidence:   record.GetFloat64(9, 0),
		})
	}
	return files, nil
}

// GetFilesMentioningEntity returns all files mentioning a specific entity
func (e *Edges) GetFilesMentioningEntity(ctx context.Context, entityName string) ([]FileNode, error) {
	query := `
		MATCH (f:File)-[:MENTIONS]->(ent:Entity {normalized: $normalized})
		RETURN f.path, f.hash, f.name, f.type, f.category, f.size, f.modified,
		       f.summary, f.document_type, f.confidence
		ORDER BY f.modified DESC
	`
	result, err := e.client.Query(ctx, query, map[string]any{"normalized": strings.ToLower(entityName)})
	if err != nil {
		return nil, fmt.Errorf("failed to get files mentioning entity; %w", err)
	}

	var files []FileNode
	for result.Next() {
		record := result.Record()
		files = append(files, FileNode{
			Path:         record.GetString(0, ""),
			Hash:         record.GetString(1, ""),
			Name:         record.GetString(2, ""),
			Type:         record.GetString(3, ""),
			Category:     record.GetString(4, ""),
			Size:         record.GetInt64(5, 0),
			Summary:      record.GetString(7, ""),
			DocumentType: record.GetString(8, ""),
			Confidence:   record.GetFloat64(9, 0),
		})
	}
	return files, nil
}

// CleanupOrphanedNodes removes nodes that are no longer connected to any File
func (e *Edges) CleanupOrphanedNodes(ctx context.Context) (int64, error) {
	// Remove orphaned tags
	query := `
		MATCH (t:Tag)
		WHERE NOT EXISTS((t)<-[:HAS_TAG]-())
		DELETE t
		RETURN count(t) as deleted
	`
	result, err := e.client.Query(ctx, query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup orphaned tags; %w", err)
	}

	var totalDeleted int64
	if result.Next() {
		totalDeleted += result.Record().GetInt64(0, 0)
	}

	// Remove orphaned topics (that aren't referenced)
	query = `
		MATCH (t:Topic)
		WHERE NOT EXISTS((t)<-[:COVERS_TOPIC]-())
		  AND NOT EXISTS((t)<-[:REFERENCES]-())
		  AND NOT EXISTS((t)<-[:PARENT_OF]-())
		DELETE t
		RETURN count(t) as deleted
	`
	result, err = e.client.Query(ctx, query, nil)
	if err != nil {
		return totalDeleted, fmt.Errorf("failed to cleanup orphaned topics; %w", err)
	}

	if result.Next() {
		totalDeleted += result.Record().GetInt64(0, 0)
	}

	// Remove orphaned entities
	query = `
		MATCH (ent:Entity)
		WHERE NOT EXISTS((ent)<-[:MENTIONS]-())
		DELETE ent
		RETURN count(ent) as deleted
	`
	result, err = e.client.Query(ctx, query, nil)
	if err != nil {
		return totalDeleted, fmt.Errorf("failed to cleanup orphaned entities; %w", err)
	}

	if result.Next() {
		totalDeleted += result.Record().GetInt64(0, 0)
	}

	e.logger.Debug("cleaned up orphaned nodes", "deleted", totalDeleted)
	return totalDeleted, nil
}
