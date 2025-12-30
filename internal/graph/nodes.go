package graph

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Nodes handles node CRUD operations
type Nodes struct {
	client *Client
	logger *slog.Logger
}

// NewNodes creates a new Nodes handler
func NewNodes(client *Client, logger *slog.Logger) *Nodes {
	if logger == nil {
		logger = slog.Default()
	}
	return &Nodes{
		client: client,
		logger: logger.With("component", "graph-nodes"),
	}
}

// FileNode represents a File node in the graph
type FileNode struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Category     string    `json:"category"`
	Size         int64     `json:"size"`
	Modified     time.Time `json:"modified"`
	Summary      string    `json:"summary,omitempty"`
	DocumentType string    `json:"document_type,omitempty"`
	Confidence   float64   `json:"confidence,omitempty"`
	Embedding    []float32 `json:"embedding,omitempty"`
}

// FileNodeFromEntry creates a FileNode from an IndexEntry
func FileNodeFromEntry(entry types.IndexEntry) FileNode {
	node := FileNode{
		Path:     entry.Metadata.Path,
		Hash:     entry.Metadata.Hash,
		Name:     extractFilename(entry.Metadata.Path),
		Type:     entry.Metadata.Type,
		Category: entry.Metadata.Category,
		Size:     entry.Metadata.Size,
		Modified: entry.Metadata.Modified,
	}

	if entry.Semantic != nil {
		node.Summary = entry.Semantic.Summary
		node.DocumentType = entry.Semantic.DocumentType
		node.Confidence = entry.Semantic.Confidence
	}

	return node
}

// extractFilename extracts the filename from a path
func extractFilename(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx == -1 {
		return path
	}
	return path[idx+1:]
}

// UpsertFile creates or updates a File node
func (n *Nodes) UpsertFile(ctx context.Context, file FileNode) error {
	query := `
		MERGE (f:File {path: $path})
		SET f.hash = $hash,
		    f.name = $name,
		    f.type = $type,
		    f.category = $category,
		    f.size = $size,
		    f.modified = $modified,
		    f.summary = $summary,
		    f.document_type = $document_type,
		    f.confidence = $confidence
		RETURN f
	`

	params := map[string]any{
		"path":          file.Path,
		"hash":          file.Hash,
		"name":          file.Name,
		"type":          file.Type,
		"category":      file.Category,
		"size":          file.Size,
		"modified":      file.Modified.Format(time.RFC3339),
		"summary":       file.Summary,
		"document_type": file.DocumentType,
		"confidence":    file.Confidence,
	}

	_, err := n.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to upsert file node; %w", err)
	}

	n.logger.Debug("upserted file node", "path", file.Path)
	return nil
}

// UpsertFileWithEmbedding creates or updates a File node including embedding
func (n *Nodes) UpsertFileWithEmbedding(ctx context.Context, file FileNode, embedding []float32) error {
	query := `
		MERGE (f:File {path: $path})
		SET f.hash = $hash,
		    f.name = $name,
		    f.type = $type,
		    f.category = $category,
		    f.size = $size,
		    f.modified = $modified,
		    f.summary = $summary,
		    f.document_type = $document_type,
		    f.confidence = $confidence,
		    f.embedding = vecf32($embedding)
		RETURN f
	`

	params := map[string]any{
		"path":          file.Path,
		"hash":          file.Hash,
		"name":          file.Name,
		"type":          file.Type,
		"category":      file.Category,
		"size":          file.Size,
		"modified":      file.Modified.Format(time.RFC3339),
		"summary":       file.Summary,
		"document_type": file.DocumentType,
		"confidence":    file.Confidence,
		"embedding":     embedding,
	}

	_, err := n.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to upsert file node with embedding; %w", err)
	}

	n.logger.Debug("upserted file node with embedding", "path", file.Path)
	return nil
}

// GetFile retrieves a File node by path or filename
// If path is absolute, searches by exact path match
// If path is relative (no leading /), searches by filename
func (n *Nodes) GetFile(ctx context.Context, path string) (*FileNode, error) {
	// Try exact path match first
	if strings.HasPrefix(path, "/") {
		return n.getFileByPath(ctx, path)
	}

	// Path doesn't start with / - try filename search
	return n.GetFileByFilename(ctx, path)
}

// getFileByPath retrieves a File node by exact path match
func (n *Nodes) getFileByPath(ctx context.Context, path string) (*FileNode, error) {
	query := `
		MATCH (f:File {path: $path})
		RETURN f.path, f.hash, f.name, f.type, f.category, f.size, f.modified,
		       f.summary, f.document_type, f.confidence
	`

	result, err := n.client.Query(ctx, query, map[string]any{"path": path})
	if err != nil {
		return nil, fmt.Errorf("failed to get file node; %w", err)
	}

	if !result.Next() {
		return nil, nil // Not found
	}

	record := result.Record()
	file := &FileNode{
		Path:         record.GetString(0, ""),
		Hash:         record.GetString(1, ""),
		Name:         record.GetString(2, ""),
		Type:         record.GetString(3, ""),
		Category:     record.GetString(4, ""),
		Size:         record.GetInt64(5, 0),
		Summary:      record.GetString(7, ""),
		DocumentType: record.GetString(8, ""),
		Confidence:   record.GetFloat64(9, 0),
	}

	// Parse modified time
	if modStr := record.GetString(6, ""); modStr != "" {
		if t, err := time.Parse(time.RFC3339, modStr); err == nil {
			file.Modified = t
		}
	}

	return file, nil
}

// GetFileByFilename retrieves a File node by filename (searches by name field)
func (n *Nodes) GetFileByFilename(ctx context.Context, filename string) (*FileNode, error) {
	query := `
		MATCH (f:File {name: $filename})
		RETURN f.path, f.hash, f.name, f.type, f.category, f.size, f.modified,
		       f.summary, f.document_type, f.confidence
		LIMIT 1
	`

	result, err := n.client.Query(ctx, query, map[string]any{"filename": filename})
	if err != nil {
		return nil, fmt.Errorf("failed to get file by filename; %w", err)
	}

	if !result.Next() {
		return nil, nil // Not found
	}

	record := result.Record()
	file := &FileNode{
		Path:         record.GetString(0, ""),
		Hash:         record.GetString(1, ""),
		Name:         record.GetString(2, ""),
		Type:         record.GetString(3, ""),
		Category:     record.GetString(4, ""),
		Size:         record.GetInt64(5, 0),
		Summary:      record.GetString(7, ""),
		DocumentType: record.GetString(8, ""),
		Confidence:   record.GetFloat64(9, 0),
	}

	// Parse modified time
	if modStr := record.GetString(6, ""); modStr != "" {
		if t, err := time.Parse(time.RFC3339, modStr); err == nil {
			file.Modified = t
		}
	}

	return file, nil
}

// DeleteFile removes a File node and all its relationships
func (n *Nodes) DeleteFile(ctx context.Context, path string) error {
	query := `
		MATCH (f:File {path: $path})
		DETACH DELETE f
	`

	_, err := n.client.Query(ctx, query, map[string]any{"path": path})
	if err != nil {
		return fmt.Errorf("failed to delete file node; %w", err)
	}

	n.logger.Debug("deleted file node", "path", path)
	return nil
}

// GetOrCreateTag gets or creates a Tag node
func (n *Nodes) GetOrCreateTag(ctx context.Context, name string) error {
	query := `MERGE (t:Tag {name: $name}) RETURN t`
	_, err := n.client.Query(ctx, query, map[string]any{"name": strings.ToLower(name)})
	if err != nil {
		return fmt.Errorf("failed to get/create tag; %w", err)
	}
	return nil
}

// GetOrCreateTopic gets or creates a Topic node
func (n *Nodes) GetOrCreateTopic(ctx context.Context, name string) error {
	query := `MERGE (t:Topic {name: $name}) RETURN t`
	_, err := n.client.Query(ctx, query, map[string]any{"name": strings.ToLower(name)})
	if err != nil {
		return fmt.Errorf("failed to get/create topic; %w", err)
	}
	return nil
}

// GetOrCreateEntity gets or creates an Entity node
func (n *Nodes) GetOrCreateEntity(ctx context.Context, name, entityType string) error {
	query := `
		MERGE (e:Entity {normalized: $normalized})
		ON CREATE SET e.name = $name, e.type = $type
		RETURN e
	`
	params := map[string]any{
		"name":       name,
		"type":       entityType,
		"normalized": strings.ToLower(name),
	}
	_, err := n.client.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to get/create entity; %w", err)
	}
	return nil
}

// GetOrCreateDirectory gets or creates a Directory node
func (n *Nodes) GetOrCreateDirectory(ctx context.Context, path string) error {
	query := `MERGE (d:Directory {path: $path}) RETURN d`
	_, err := n.client.Query(ctx, query, map[string]any{"path": path})
	if err != nil {
		return fmt.Errorf("failed to get/create directory; %w", err)
	}
	return nil
}

// GetCategory retrieves a Category node by name
func (n *Nodes) GetCategory(ctx context.Context, name string) (bool, error) {
	query := `MATCH (c:Category {name: $name}) RETURN c`
	result, err := n.client.Query(ctx, query, map[string]any{"name": name})
	if err != nil {
		return false, fmt.Errorf("failed to get category; %w", err)
	}
	return result.Next(), nil
}

// ListFiles returns all File nodes
func (n *Nodes) ListFiles(ctx context.Context) ([]FileNode, error) {
	query := `
		MATCH (f:File)
		RETURN f.path, f.hash, f.name, f.type, f.category, f.size, f.modified,
		       f.summary, f.document_type, f.confidence
		ORDER BY f.modified DESC
	`

	result, err := n.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list files; %w", err)
	}

	var files []FileNode
	for result.Next() {
		record := result.Record()
		file := FileNode{
			Path:         record.GetString(0, ""),
			Hash:         record.GetString(1, ""),
			Name:         record.GetString(2, ""),
			Type:         record.GetString(3, ""),
			Category:     record.GetString(4, ""),
			Size:         record.GetInt64(5, 0),
			Summary:      record.GetString(7, ""),
			DocumentType: record.GetString(8, ""),
			Confidence:   record.GetFloat64(9, 0),
		}

		if modStr := record.GetString(6, ""); modStr != "" {
			if t, err := time.Parse(time.RFC3339, modStr); err == nil {
				file.Modified = t
			}
		}

		files = append(files, file)
	}

	return files, nil
}

// ListTags returns all Tag names
func (n *Nodes) ListTags(ctx context.Context) ([]string, error) {
	query := `MATCH (t:Tag) RETURN t.name ORDER BY t.name`
	result, err := n.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags; %w", err)
	}

	var tags []string
	for result.Next() {
		tags = append(tags, result.Record().GetString(0, ""))
	}
	return tags, nil
}

// ListTopics returns all Topic names
func (n *Nodes) ListTopics(ctx context.Context) ([]string, error) {
	query := `MATCH (t:Topic) RETURN t.name ORDER BY t.name`
	result, err := n.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list topics; %w", err)
	}

	var topics []string
	for result.Next() {
		topics = append(topics, result.Record().GetString(0, ""))
	}
	return topics, nil
}

// ListEntities returns all Entity nodes
func (n *Nodes) ListEntities(ctx context.Context) ([]EntityNode, error) {
	query := `MATCH (e:Entity) RETURN e.name, e.type, e.normalized ORDER BY e.name`
	result, err := n.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities; %w", err)
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

// EntityNode represents an Entity in the graph
type EntityNode struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Normalized string `json:"normalized"`
}

// CountFiles returns the total number of File nodes
func (n *Nodes) CountFiles(ctx context.Context) (int64, error) {
	query := `MATCH (f:File) RETURN count(f) as count`
	result, err := n.client.Query(ctx, query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count files; %w", err)
	}

	if result.Next() {
		return result.Record().GetInt64(0, 0), nil
	}
	return 0, nil
}

// FileExists checks if a file exists by path
func (n *Nodes) FileExists(ctx context.Context, path string) (bool, error) {
	query := `MATCH (f:File {path: $path}) RETURN count(f) > 0 as exists`
	result, err := n.client.Query(ctx, query, map[string]any{"path": path})
	if err != nil {
		return false, fmt.Errorf("failed to check file existence; %w", err)
	}

	if result.Next() {
		val := result.Record().GetByIndex(0)
		if b, ok := val.(bool); ok {
			return b, nil
		}
	}
	return false, nil
}

// ListFilePaths returns all File node paths (efficient for stale detection)
func (n *Nodes) ListFilePaths(ctx context.Context) ([]string, error) {
	query := `MATCH (f:File) RETURN f.path`
	result, err := n.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list file paths; %w", err)
	}

	var paths []string
	for result.Next() {
		paths = append(paths, result.Record().GetString(0, ""))
	}
	return paths, nil
}

// GetFileByHash retrieves a File node by content hash
func (n *Nodes) GetFileByHash(ctx context.Context, hash string) (*FileNode, error) {
	query := `
		MATCH (f:File {hash: $hash})
		RETURN f.path, f.hash, f.name, f.type, f.category, f.size, f.modified,
		       f.summary, f.document_type, f.confidence
		LIMIT 1
	`

	result, err := n.client.Query(ctx, query, map[string]any{"hash": hash})
	if err != nil {
		return nil, fmt.Errorf("failed to get file by hash; %w", err)
	}

	if !result.Next() {
		return nil, nil // Not found
	}

	record := result.Record()
	file := &FileNode{
		Path:         record.GetString(0, ""),
		Hash:         record.GetString(1, ""),
		Name:         record.GetString(2, ""),
		Type:         record.GetString(3, ""),
		Category:     record.GetString(4, ""),
		Size:         record.GetInt64(5, 0),
		Summary:      record.GetString(7, ""),
		DocumentType: record.GetString(8, ""),
		Confidence:   record.GetFloat64(9, 0),
	}

	if modStr := record.GetString(6, ""); modStr != "" {
		if t, err := time.Parse(time.RFC3339, modStr); err == nil {
			file.Modified = t
		}
	}

	return file, nil
}
