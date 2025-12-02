package graph

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
)

// ClusterDetection provides file clustering capabilities
type ClusterDetection struct {
	client *Client
	logger *slog.Logger
}

// NewClusterDetection creates a new ClusterDetection handler
func NewClusterDetection(client *Client, logger *slog.Logger) *ClusterDetection {
	if logger == nil {
		logger = slog.Default()
	}
	return &ClusterDetection{
		client: client,
		logger: logger.With("component", "graph-clusters"),
	}
}

// Cluster represents a group of related files
type Cluster struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	FileCount       int           `json:"file_count"`
	Files           []ClusterFile `json:"files"`
	DominantTags    []string      `json:"dominant_tags"`
	DominantTopics  []string      `json:"dominant_topics"`
	DominantEntities []string     `json:"dominant_entities,omitempty"`
	Categories      []string      `json:"categories"`
}

// ClusterFile represents a file within a cluster
type ClusterFile struct {
	Path       string  `json:"path"`
	Name       string  `json:"name"`
	Summary    string  `json:"summary"`
	Category   string  `json:"category"`
	Centrality float64 `json:"centrality"` // How central this file is to the cluster
}

// DetectTopicClusters finds clusters of files based on shared topics
func (c *ClusterDetection) DetectTopicClusters(ctx context.Context, minSize int) ([]Cluster, error) {
	c.logger.Debug("detecting topic clusters", "min_size", minSize)

	// Find topics that connect multiple files
	query := `
		MATCH (t:Topic)<-[:COVERS_TOPIC]-(f:File)
		WITH t, collect(f) as files, count(f) as fileCount
		WHERE fileCount >= $minSize
		RETURN t.name, fileCount,
		       [f IN files | {path: f.path, name: f.name, summary: f.summary, category: f.category}] as fileData
		ORDER BY fileCount DESC
	`
	result, err := c.client.Query(ctx, query, map[string]any{"minSize": minSize})
	if err != nil {
		return nil, fmt.Errorf("topic cluster detection failed; %w", err)
	}

	var clusters []Cluster
	clusterID := 0
	for result.Next() {
		record := result.Record()
		topicName := record.GetString(0, "")
		fileCount := record.GetInt64(1, 0)

		// Parse file data
		filesData := record.GetByIndex(2)
		files := c.parseClusterFiles(filesData)

		// Calculate centrality for each file (based on other connections)
		for i := range files {
			files[i].Centrality = 1.0 / float64(i+1) // Simple positional centrality
		}

		// Collect unique categories
		catMap := make(map[string]bool)
		for _, f := range files {
			if f.Category != "" {
				catMap[f.Category] = true
			}
		}
		categories := make([]string, 0, len(catMap))
		for cat := range catMap {
			categories = append(categories, cat)
		}

		clusters = append(clusters, Cluster{
			ID:             fmt.Sprintf("topic-%d", clusterID),
			Name:           topicName,
			FileCount:      int(fileCount),
			Files:          files,
			DominantTopics: []string{topicName},
			Categories:     categories,
		})
		clusterID++
	}

	return clusters, nil
}

// DetectEntityClusters finds clusters of files based on shared entities
func (c *ClusterDetection) DetectEntityClusters(ctx context.Context, minSize int) ([]Cluster, error) {
	c.logger.Debug("detecting entity clusters", "min_size", minSize)

	query := `
		MATCH (e:Entity)<-[:MENTIONS]-(f:File)
		WITH e, collect(f) as files, count(f) as fileCount
		WHERE fileCount >= $minSize
		RETURN e.name, e.type, fileCount,
		       [f IN files | {path: f.path, name: f.name, summary: f.summary, category: f.category}] as fileData
		ORDER BY fileCount DESC
	`
	result, err := c.client.Query(ctx, query, map[string]any{"minSize": minSize})
	if err != nil {
		return nil, fmt.Errorf("entity cluster detection failed; %w", err)
	}

	var clusters []Cluster
	clusterID := 0
	for result.Next() {
		record := result.Record()
		entityName := record.GetString(0, "")
		entityType := record.GetString(1, "")
		fileCount := record.GetInt64(2, 0)

		filesData := record.GetByIndex(3)
		files := c.parseClusterFiles(filesData)

		for i := range files {
			files[i].Centrality = 1.0 / float64(i+1)
		}

		catMap := make(map[string]bool)
		for _, f := range files {
			if f.Category != "" {
				catMap[f.Category] = true
			}
		}
		categories := make([]string, 0, len(catMap))
		for cat := range catMap {
			categories = append(categories, cat)
		}

		clusters = append(clusters, Cluster{
			ID:               fmt.Sprintf("entity-%d", clusterID),
			Name:             fmt.Sprintf("%s (%s)", entityName, entityType),
			FileCount:        int(fileCount),
			Files:            files,
			DominantEntities: []string{entityName},
			Categories:       categories,
		})
		clusterID++
	}

	return clusters, nil
}

// DetectTagClusters finds clusters of files based on shared tags
func (c *ClusterDetection) DetectTagClusters(ctx context.Context, minSize int) ([]Cluster, error) {
	c.logger.Debug("detecting tag clusters", "min_size", minSize)

	query := `
		MATCH (t:Tag)<-[:HAS_TAG]-(f:File)
		WITH t, collect(f) as files, count(f) as fileCount
		WHERE fileCount >= $minSize
		RETURN t.name, fileCount,
		       [f IN files | {path: f.path, name: f.name, summary: f.summary, category: f.category}] as fileData
		ORDER BY fileCount DESC
	`
	result, err := c.client.Query(ctx, query, map[string]any{"minSize": minSize})
	if err != nil {
		return nil, fmt.Errorf("tag cluster detection failed; %w", err)
	}

	var clusters []Cluster
	clusterID := 0
	for result.Next() {
		record := result.Record()
		tagName := record.GetString(0, "")
		fileCount := record.GetInt64(1, 0)

		filesData := record.GetByIndex(2)
		files := c.parseClusterFiles(filesData)

		for i := range files {
			files[i].Centrality = 1.0 / float64(i+1)
		}

		catMap := make(map[string]bool)
		for _, f := range files {
			if f.Category != "" {
				catMap[f.Category] = true
			}
		}
		categories := make([]string, 0, len(catMap))
		for cat := range catMap {
			categories = append(categories, cat)
		}

		clusters = append(clusters, Cluster{
			ID:           fmt.Sprintf("tag-%d", clusterID),
			Name:         tagName,
			FileCount:    int(fileCount),
			Files:        files,
			DominantTags: []string{tagName},
			Categories:   categories,
		})
		clusterID++
	}

	return clusters, nil
}

// DetectCategoryClusters finds clusters based on file categories
func (c *ClusterDetection) DetectCategoryClusters(ctx context.Context) ([]Cluster, error) {
	c.logger.Debug("detecting category clusters")

	query := `
		MATCH (f:File)-[:IN_CATEGORY]->(cat:Category)
		WITH cat, collect(f) as files, count(f) as fileCount
		WHERE fileCount > 0
		OPTIONAL MATCH (f:File)-[:IN_CATEGORY]->(cat)
		OPTIONAL MATCH (f)-[:HAS_TAG]->(t:Tag)
		OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(tp:Topic)
		WITH cat, files, fileCount,
		     collect(DISTINCT t.name) as tags,
		     collect(DISTINCT tp.name) as topics
		RETURN cat.name, fileCount,
		       [f IN files | {path: f.path, name: f.name, summary: f.summary, category: f.category}] as fileData,
		       [t IN tags WHERE t IS NOT NULL][0..5] as topTags,
		       [tp IN topics WHERE tp IS NOT NULL][0..5] as topTopics
		ORDER BY fileCount DESC
	`
	result, err := c.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("category cluster detection failed; %w", err)
	}

	var clusters []Cluster
	clusterID := 0
	for result.Next() {
		record := result.Record()
		catName := record.GetString(0, "")
		fileCount := record.GetInt64(1, 0)

		filesData := record.GetByIndex(2)
		files := c.parseClusterFiles(filesData)

		topTags := parseStringArray(record.GetByIndex(3))
		topTopics := parseStringArray(record.GetByIndex(4))

		for i := range files {
			files[i].Centrality = 1.0 / float64(i+1)
		}

		clusters = append(clusters, Cluster{
			ID:             fmt.Sprintf("category-%d", clusterID),
			Name:           fmt.Sprintf("Category: %s", catName),
			FileCount:      int(fileCount),
			Files:          files,
			DominantTags:   topTags,
			DominantTopics: topTopics,
			Categories:     []string{catName},
		})
		clusterID++
	}

	return clusters, nil
}

// DetectOverlappingClusters finds files that belong to multiple clusters
// These are potential integration points or hub files
func (c *ClusterDetection) DetectOverlappingClusters(ctx context.Context, limit int) ([]OverlapFile, error) {
	c.logger.Debug("detecting cluster overlaps", "limit", limit)

	query := `
		MATCH (f:File)
		OPTIONAL MATCH (f)-[:HAS_TAG]->(t:Tag)
		OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(tp:Topic)
		OPTIONAL MATCH (f)-[:MENTIONS]->(e:Entity)
		WITH f,
		     count(DISTINCT t) as tagCount,
		     count(DISTINCT tp) as topicCount,
		     count(DISTINCT e) as entityCount,
		     collect(DISTINCT t.name) as tags,
		     collect(DISTINCT tp.name) as topics,
		     collect(DISTINCT e.name) as entities
		WHERE tagCount > 2 OR topicCount > 2 OR entityCount > 2
		RETURN f.path, f.name, f.summary, f.category,
		       tagCount, topicCount, entityCount,
		       [t IN tags WHERE t IS NOT NULL][0..5] as topTags,
		       [tp IN topics WHERE tp IS NOT NULL][0..5] as topTopics,
		       [e IN entities WHERE e IS NOT NULL][0..5] as topEntities
		ORDER BY tagCount + topicCount + entityCount DESC
		LIMIT $limit
	`
	result, err := c.client.Query(ctx, query, map[string]any{"limit": limit})
	if err != nil {
		return nil, fmt.Errorf("overlap detection failed; %w", err)
	}

	var overlaps []OverlapFile
	for result.Next() {
		record := result.Record()
		overlaps = append(overlaps, OverlapFile{
			Path:        record.GetString(0, ""),
			Name:        record.GetString(1, ""),
			Summary:     record.GetString(2, ""),
			Category:    record.GetString(3, ""),
			TagCount:    record.GetInt64(4, 0),
			TopicCount:  record.GetInt64(5, 0),
			EntityCount: record.GetInt64(6, 0),
			Tags:        parseStringArray(record.GetByIndex(7)),
			Topics:      parseStringArray(record.GetByIndex(8)),
			Entities:    parseStringArray(record.GetByIndex(9)),
		})
	}

	return overlaps, nil
}

// OverlapFile represents a file that spans multiple clusters
type OverlapFile struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	Summary     string   `json:"summary"`
	Category    string   `json:"category"`
	TagCount    int64    `json:"tag_count"`
	TopicCount  int64    `json:"topic_count"`
	EntityCount int64    `json:"entity_count"`
	Tags        []string `json:"tags"`
	Topics      []string `json:"topics"`
	Entities    []string `json:"entities"`
}

// GetClusterSummary returns a high-level summary of knowledge clusters
func (c *ClusterDetection) GetClusterSummary(ctx context.Context) (*ClusterSummary, error) {
	c.logger.Debug("generating cluster summary")

	summary := &ClusterSummary{}

	// Count topics by file coverage
	topicQuery := `
		MATCH (t:Topic)<-[:COVERS_TOPIC]-(f:File)
		WITH t.name as name, count(f) as fileCount
		RETURN name, fileCount
		ORDER BY fileCount DESC
		LIMIT 10
	`
	topicResult, err := c.client.Query(ctx, topicQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic stats; %w", err)
	}
	for topicResult.Next() {
		record := topicResult.Record()
		summary.TopTopics = append(summary.TopTopics, TopicStats{
			Name:      record.GetString(0, ""),
			FileCount: record.GetInt64(1, 0),
		})
	}

	// Count tags by file coverage
	tagQuery := `
		MATCH (t:Tag)<-[:HAS_TAG]-(f:File)
		WITH t.name as name, count(f) as fileCount
		RETURN name, fileCount
		ORDER BY fileCount DESC
		LIMIT 10
	`
	tagResult, err := c.client.Query(ctx, tagQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag stats; %w", err)
	}
	for tagResult.Next() {
		record := tagResult.Record()
		summary.TopTags = append(summary.TopTags, TagStats{
			Name:      record.GetString(0, ""),
			FileCount: record.GetInt64(1, 0),
		})
	}

	// Count entities by file coverage
	entityQuery := `
		MATCH (e:Entity)<-[:MENTIONS]-(f:File)
		WITH e.name as name, e.type as type, count(f) as fileCount
		RETURN name, type, fileCount
		ORDER BY fileCount DESC
		LIMIT 10
	`
	entityResult, err := c.client.Query(ctx, entityQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity stats; %w", err)
	}
	for entityResult.Next() {
		record := entityResult.Record()
		summary.TopEntities = append(summary.TopEntities, EntityStats{
			Name:      record.GetString(0, ""),
			Type:      record.GetString(1, ""),
			FileCount: record.GetInt64(2, 0),
		})
	}

	// Category distribution
	catQuery := `
		MATCH (f:File)-[:IN_CATEGORY]->(c:Category)
		WITH c.name as category, count(f) as fileCount
		RETURN category, fileCount
		ORDER BY fileCount DESC
	`
	catResult, err := c.client.Query(ctx, catQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get category stats; %w", err)
	}
	summary.CategoryDistribution = make(map[string]int64)
	for catResult.Next() {
		record := catResult.Record()
		summary.CategoryDistribution[record.GetString(0, "")] = record.GetInt64(1, 0)
	}

	return summary, nil
}

// ClusterSummary provides high-level statistics about knowledge clusters
type ClusterSummary struct {
	TopTopics            []TopicStats     `json:"top_topics"`
	TopTags              []TagStats       `json:"top_tags"`
	TopEntities          []EntityStats    `json:"top_entities"`
	CategoryDistribution map[string]int64 `json:"category_distribution"`
}

// TopicStats represents topic statistics
type TopicStats struct {
	Name      string `json:"name"`
	FileCount int64  `json:"file_count"`
}

// TagStats represents tag statistics
type TagStats struct {
	Name      string `json:"name"`
	FileCount int64  `json:"file_count"`
}

// EntityStats represents entity statistics
type EntityStats struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	FileCount int64  `json:"file_count"`
}

// parseClusterFiles parses the file data from query results
func (c *ClusterDetection) parseClusterFiles(data any) []ClusterFile {
	if data == nil {
		return nil
	}

	arr, ok := data.([]any)
	if !ok {
		return nil
	}

	files := make([]ClusterFile, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		file := ClusterFile{}
		if v, ok := m["path"].(string); ok {
			file.Path = v
		}
		if v, ok := m["name"].(string); ok {
			file.Name = v
		}
		if v, ok := m["summary"].(string); ok {
			file.Summary = v
		}
		if v, ok := m["category"].(string); ok {
			file.Category = v
		}
		files = append(files, file)
	}

	return files
}

// FindFileClusterMembership returns all clusters a file belongs to
func (c *ClusterDetection) FindFileClusterMembership(ctx context.Context, filePath string) (*FileMembership, error) {
	membership := &FileMembership{
		FilePath: filePath,
	}

	// Get tags
	tagQuery := `
		MATCH (f:File {path: $filePath})-[:HAS_TAG]->(t:Tag)<-[:HAS_TAG]-(other:File)
		WHERE f <> other
		WITH t.name as tag, count(other) as otherCount
		RETURN tag, otherCount
		ORDER BY otherCount DESC
	`
	tagResult, err := c.client.Query(ctx, tagQuery, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get tag membership; %w", err)
	}
	for tagResult.Next() {
		record := tagResult.Record()
		membership.TagClusters = append(membership.TagClusters, ClusterMember{
			Name:           record.GetString(0, ""),
			SharedFileCount: record.GetInt64(1, 0),
		})
	}

	// Get topics
	topicQuery := `
		MATCH (f:File {path: $filePath})-[:COVERS_TOPIC]->(t:Topic)<-[:COVERS_TOPIC]-(other:File)
		WHERE f <> other
		WITH t.name as topic, count(other) as otherCount
		RETURN topic, otherCount
		ORDER BY otherCount DESC
	`
	topicResult, err := c.client.Query(ctx, topicQuery, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get topic membership; %w", err)
	}
	for topicResult.Next() {
		record := topicResult.Record()
		membership.TopicClusters = append(membership.TopicClusters, ClusterMember{
			Name:           record.GetString(0, ""),
			SharedFileCount: record.GetInt64(1, 0),
		})
	}

	// Get entities
	entityQuery := `
		MATCH (f:File {path: $filePath})-[:MENTIONS]->(e:Entity)<-[:MENTIONS]-(other:File)
		WHERE f <> other
		WITH e.name as entity, count(other) as otherCount
		RETURN entity, otherCount
		ORDER BY otherCount DESC
	`
	entityResult, err := c.client.Query(ctx, entityQuery, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get entity membership; %w", err)
	}
	for entityResult.Next() {
		record := entityResult.Record()
		membership.EntityClusters = append(membership.EntityClusters, ClusterMember{
			Name:           record.GetString(0, ""),
			SharedFileCount: record.GetInt64(1, 0),
		})
	}

	// Sort by shared file count
	sort.Slice(membership.TagClusters, func(i, j int) bool {
		return membership.TagClusters[i].SharedFileCount > membership.TagClusters[j].SharedFileCount
	})
	sort.Slice(membership.TopicClusters, func(i, j int) bool {
		return membership.TopicClusters[i].SharedFileCount > membership.TopicClusters[j].SharedFileCount
	})
	sort.Slice(membership.EntityClusters, func(i, j int) bool {
		return membership.EntityClusters[i].SharedFileCount > membership.EntityClusters[j].SharedFileCount
	})

	return membership, nil
}

// FileMembership represents which clusters a file belongs to
type FileMembership struct {
	FilePath       string          `json:"file_path"`
	TagClusters    []ClusterMember `json:"tag_clusters"`
	TopicClusters  []ClusterMember `json:"topic_clusters"`
	EntityClusters []ClusterMember `json:"entity_clusters"`
}

// ClusterMember represents membership in a cluster
type ClusterMember struct {
	Name           string `json:"name"`
	SharedFileCount int64 `json:"shared_file_count"`
}
