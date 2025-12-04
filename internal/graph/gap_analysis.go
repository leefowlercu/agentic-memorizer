package graph

import (
	"context"
	"fmt"
	"log/slog"
)

// GapAnalysis identifies gaps and areas needing more documentation
type GapAnalysis struct {
	client *Client
	logger *slog.Logger
}

// NewGapAnalysis creates a new GapAnalysis handler
func NewGapAnalysis(client *Client, logger *slog.Logger) *GapAnalysis {
	if logger == nil {
		logger = slog.Default()
	}
	return &GapAnalysis{
		client: client,
		logger: logger.With("component", "graph-gap-analysis"),
	}
}

// Gap represents a knowledge gap in the graph
type Gap struct {
	Type        string `json:"type"` // topic, entity, tag, category
	Name        string `json:"name"`
	FileCount   int64  `json:"file_count"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // low, medium, high
	Suggestion  string `json:"suggestion"`
}

// GapReport contains the full gap analysis results
type GapReport struct {
	UnderDocumentedTopics   []Gap `json:"under_documented_topics"`
	UnderDocumentedEntities []Gap `json:"under_documented_entities"`
	OrphanedTags            []Gap `json:"orphaned_tags"`
	OrphanedTopics          []Gap `json:"orphaned_topics"`
	OrphanedEntities        []Gap `json:"orphaned_entities"`
	IsolatedFiles           []Gap `json:"isolated_files"`
	EmptyCategories         []Gap `json:"empty_categories"`
	TotalGaps               int   `json:"total_gaps"`
}

// AnalyzeGaps performs a complete gap analysis
func (g *GapAnalysis) AnalyzeGaps(ctx context.Context) (*GapReport, error) {
	g.logger.Debug("performing gap analysis")

	report := &GapReport{}

	// Find under-documented topics (mentioned but with few files)
	topics, err := g.FindUnderDocumentedTopics(ctx, 2)
	if err != nil {
		g.logger.Warn("failed to find under-documented topics", "error", err)
	} else {
		report.UnderDocumentedTopics = topics
	}

	// Find under-documented entities
	entities, err := g.FindUnderDocumentedEntities(ctx, 1)
	if err != nil {
		g.logger.Warn("failed to find under-documented entities", "error", err)
	} else {
		report.UnderDocumentedEntities = entities
	}

	// Find orphaned tags (tags with only one file)
	orphanedTags, err := g.FindOrphanedTags(ctx)
	if err != nil {
		g.logger.Warn("failed to find orphaned tags", "error", err)
	} else {
		report.OrphanedTags = orphanedTags
	}

	// Find orphaned topics
	orphanedTopics, err := g.FindOrphanedTopics(ctx)
	if err != nil {
		g.logger.Warn("failed to find orphaned topics", "error", err)
	} else {
		report.OrphanedTopics = orphanedTopics
	}

	// Find orphaned entities
	orphanedEntities, err := g.FindOrphanedEntities(ctx)
	if err != nil {
		g.logger.Warn("failed to find orphaned entities", "error", err)
	} else {
		report.OrphanedEntities = orphanedEntities
	}

	// Find isolated files (files with no tags, topics, or entities)
	isolatedFiles, err := g.FindIsolatedFiles(ctx)
	if err != nil {
		g.logger.Warn("failed to find isolated files", "error", err)
	} else {
		report.IsolatedFiles = isolatedFiles
	}

	// Find empty categories
	emptyCategories, err := g.FindEmptyCategories(ctx)
	if err != nil {
		g.logger.Warn("failed to find empty categories", "error", err)
	} else {
		report.EmptyCategories = emptyCategories
	}

	// Calculate total gaps
	report.TotalGaps = len(report.UnderDocumentedTopics) +
		len(report.UnderDocumentedEntities) +
		len(report.OrphanedTags) +
		len(report.OrphanedTopics) +
		len(report.OrphanedEntities) +
		len(report.IsolatedFiles) +
		len(report.EmptyCategories)

	g.logger.Info("gap analysis complete", "total_gaps", report.TotalGaps)
	return report, nil
}

// FindUnderDocumentedTopics finds topics that have fewer than minFiles files
func (g *GapAnalysis) FindUnderDocumentedTopics(ctx context.Context, minFiles int) ([]Gap, error) {
	query := `
		MATCH (t:Topic)
		OPTIONAL MATCH (t)<-[:COVERS_TOPIC]-(f:File)
		WITH t, count(f) as fileCount
		WHERE fileCount > 0 AND fileCount < $minFiles
		RETURN t.name, fileCount
		ORDER BY fileCount ASC, t.name
		LIMIT 20
	`
	result, err := g.client.Query(ctx, query, map[string]any{"minFiles": minFiles})
	if err != nil {
		return nil, fmt.Errorf("failed to find under-documented topics; %w", err)
	}

	var gaps []Gap
	for result.Next() {
		record := result.Record()
		name := record.GetString(0, "")
		fileCount := record.GetInt64(1, 0)

		severity := "medium"
		if fileCount == 1 {
			severity = "high"
		}

		gaps = append(gaps, Gap{
			Type:        "topic",
			Name:        name,
			FileCount:   fileCount,
			Description: fmt.Sprintf("Topic '%s' is covered by only %d file(s)", name, fileCount),
			Severity:    severity,
			Suggestion:  fmt.Sprintf("Consider adding more documentation about '%s'", name),
		})
	}

	return gaps, nil
}

// FindUnderDocumentedEntities finds entities mentioned in very few files
func (g *GapAnalysis) FindUnderDocumentedEntities(ctx context.Context, minFiles int) ([]Gap, error) {
	query := `
		MATCH (e:Entity)
		OPTIONAL MATCH (e)<-[:MENTIONS]-(f:File)
		WITH e, count(f) as fileCount
		WHERE fileCount > 0 AND fileCount <= $minFiles
		RETURN e.name, e.type, fileCount
		ORDER BY fileCount ASC, e.name
		LIMIT 30
	`
	result, err := g.client.Query(ctx, query, map[string]any{"minFiles": minFiles})
	if err != nil {
		return nil, fmt.Errorf("failed to find under-documented entities; %w", err)
	}

	var gaps []Gap
	for result.Next() {
		record := result.Record()
		name := record.GetString(0, "")
		entType := record.GetString(1, "")
		fileCount := record.GetInt64(2, 0)

		gaps = append(gaps, Gap{
			Type:        "entity",
			Name:        fmt.Sprintf("%s (%s)", name, entType),
			FileCount:   fileCount,
			Description: fmt.Sprintf("Entity '%s' is only mentioned in %d file(s)", name, fileCount),
			Severity:    "low",
			Suggestion:  fmt.Sprintf("Consider documenting '%s' more thoroughly", name),
		})
	}

	return gaps, nil
}

// FindOrphanedTags finds tags that are only associated with one file
func (g *GapAnalysis) FindOrphanedTags(ctx context.Context) ([]Gap, error) {
	query := `
		MATCH (t:Tag)
		OPTIONAL MATCH (t)<-[:HAS_TAG]-(f:File)
		WITH t, count(f) as fileCount
		WHERE fileCount = 1
		RETURN t.name, fileCount
		ORDER BY t.name
		LIMIT 50
	`
	result, err := g.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find orphaned tags; %w", err)
	}

	var gaps []Gap
	for result.Next() {
		record := result.Record()
		name := record.GetString(0, "")

		gaps = append(gaps, Gap{
			Type:        "orphaned_tag",
			Name:        name,
			FileCount:   1,
			Description: fmt.Sprintf("Tag '%s' is only used by one file", name),
			Severity:    "low",
			Suggestion:  "Consider if this tag should be merged with a more general tag or if more files should use it",
		})
	}

	return gaps, nil
}

// FindOrphanedTopics finds topics with no file coverage
func (g *GapAnalysis) FindOrphanedTopics(ctx context.Context) ([]Gap, error) {
	query := `
		MATCH (t:Topic)
		WHERE NOT EXISTS((t)<-[:COVERS_TOPIC]-())
		RETURN t.name
		ORDER BY t.name
		LIMIT 20
	`
	result, err := g.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find orphaned topics; %w", err)
	}

	var gaps []Gap
	for result.Next() {
		record := result.Record()
		name := record.GetString(0, "")

		gaps = append(gaps, Gap{
			Type:        "orphaned_topic",
			Name:        name,
			FileCount:   0,
			Description: fmt.Sprintf("Topic '%s' has no files covering it", name),
			Severity:    "high",
			Suggestion:  "This topic may be stale or needs documentation",
		})
	}

	return gaps, nil
}

// FindOrphanedEntities finds entities with no file mentions
func (g *GapAnalysis) FindOrphanedEntities(ctx context.Context) ([]Gap, error) {
	query := `
		MATCH (e:Entity)
		WHERE NOT EXISTS((e)<-[:MENTIONS]-())
		RETURN e.name, e.type
		ORDER BY e.name
		LIMIT 20
	`
	result, err := g.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find orphaned entities; %w", err)
	}

	var gaps []Gap
	for result.Next() {
		record := result.Record()
		name := record.GetString(0, "")
		entType := record.GetString(1, "")

		gaps = append(gaps, Gap{
			Type:        "orphaned_entity",
			Name:        fmt.Sprintf("%s (%s)", name, entType),
			FileCount:   0,
			Description: fmt.Sprintf("Entity '%s' is not mentioned in any files", name),
			Severity:    "medium",
			Suggestion:  "This entity may be stale and can be removed",
		})
	}

	return gaps, nil
}

// FindIsolatedFiles finds files with no semantic connections
func (g *GapAnalysis) FindIsolatedFiles(ctx context.Context) ([]Gap, error) {
	query := `
		MATCH (f:File)
		WHERE NOT EXISTS((f)-[:HAS_TAG]->())
		  AND NOT EXISTS((f)-[:COVERS_TOPIC]->())
		  AND NOT EXISTS((f)-[:MENTIONS]->())
		RETURN f.path, f.name, f.category
		ORDER BY f.modified DESC
		LIMIT 30
	`
	result, err := g.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find isolated files; %w", err)
	}

	var gaps []Gap
	for result.Next() {
		record := result.Record()
		path := record.GetString(0, "")
		name := record.GetString(1, "")
		category := record.GetString(2, "")

		gaps = append(gaps, Gap{
			Type:        "isolated_file",
			Name:        name,
			FileCount:   0,
			Description: fmt.Sprintf("File '%s' (%s) has no tags, topics, or entities", name, category),
			Severity:    "medium",
			Suggestion:  fmt.Sprintf("Re-analyze %s to extract semantic information", path),
		})
	}

	return gaps, nil
}

// FindEmptyCategories finds categories with no files
func (g *GapAnalysis) FindEmptyCategories(ctx context.Context) ([]Gap, error) {
	query := `
		MATCH (c:Category)
		WHERE NOT EXISTS((c)<-[:IN_CATEGORY]-())
		RETURN c.name
	`
	result, err := g.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find empty categories; %w", err)
	}

	var gaps []Gap
	for result.Next() {
		record := result.Record()
		name := record.GetString(0, "")

		gaps = append(gaps, Gap{
			Type:        "empty_category",
			Name:        name,
			FileCount:   0,
			Description: fmt.Sprintf("Category '%s' has no files", name),
			Severity:    "low",
			Suggestion:  "This category is unused",
		})
	}

	return gaps, nil
}

// FindMissingConnections finds files that should probably be connected based on content similarity
func (g *GapAnalysis) FindMissingConnections(ctx context.Context, limit int) ([]MissingConnection, error) {
	// Find files in the same category that share no common tags/topics/entities
	// but might be related based on their summaries
	query := `
		MATCH (f1:File)-[:IN_CATEGORY]->(c:Category)<-[:IN_CATEGORY]-(f2:File)
		WHERE f1 <> f2 AND id(f1) < id(f2)
		AND NOT EXISTS((f1)-[:HAS_TAG]->()<-[:HAS_TAG]-(f2))
		AND NOT EXISTS((f1)-[:COVERS_TOPIC]->()<-[:COVERS_TOPIC]-(f2))
		AND NOT EXISTS((f1)-[:MENTIONS]->()<-[:MENTIONS]-(f2))
		RETURN f1.path, f1.name, f2.path, f2.name, c.name
		LIMIT $limit
	`
	result, err := g.client.Query(ctx, query, map[string]any{"limit": limit})
	if err != nil {
		return nil, fmt.Errorf("failed to find missing connections; %w", err)
	}

	var connections []MissingConnection
	for result.Next() {
		record := result.Record()
		connections = append(connections, MissingConnection{
			File1Path:     record.GetString(0, ""),
			File1Name:     record.GetString(1, ""),
			File2Path:     record.GetString(2, ""),
			File2Name:     record.GetString(3, ""),
			SharedContext: record.GetString(4, ""),
			Reason:        "Same category but no shared connections",
		})
	}

	return connections, nil
}

// MissingConnection represents two files that might need to be connected
type MissingConnection struct {
	File1Path     string `json:"file1_path"`
	File1Name     string `json:"file1_name"`
	File2Path     string `json:"file2_path"`
	File2Name     string `json:"file2_name"`
	SharedContext string `json:"shared_context"`
	Reason        string `json:"reason"`
}

// GetCoverageStats returns statistics about knowledge coverage
func (g *GapAnalysis) GetCoverageStats(ctx context.Context) (*CoverageStats, error) {
	stats := &CoverageStats{}

	// Total files
	fileQuery := `MATCH (f:File) RETURN count(f)`
	fileResult, err := g.client.Query(ctx, fileQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count files; %w", err)
	}
	if fileResult.Next() {
		stats.TotalFiles = fileResult.Record().GetInt64(0, 0)
	}

	// Files with tags
	tagQuery := `MATCH (f:File)-[:HAS_TAG]->() RETURN count(DISTINCT f)`
	tagResult, err := g.client.Query(ctx, tagQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count tagged files; %w", err)
	}
	if tagResult.Next() {
		stats.FilesWithTags = tagResult.Record().GetInt64(0, 0)
	}

	// Files with topics
	topicQuery := `MATCH (f:File)-[:COVERS_TOPIC]->() RETURN count(DISTINCT f)`
	topicResult, err := g.client.Query(ctx, topicQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count topic files; %w", err)
	}
	if topicResult.Next() {
		stats.FilesWithTopics = topicResult.Record().GetInt64(0, 0)
	}

	// Files with entities
	entityQuery := `MATCH (f:File)-[:MENTIONS]->() RETURN count(DISTINCT f)`
	entityResult, err := g.client.Query(ctx, entityQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count entity files; %w", err)
	}
	if entityResult.Next() {
		stats.FilesWithEntities = entityResult.Record().GetInt64(0, 0)
	}

	// Files with embeddings
	embeddingQuery := `MATCH (f:File) WHERE f.embedding IS NOT NULL RETURN count(f)`
	embeddingResult, err := g.client.Query(ctx, embeddingQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count embedded files; %w", err)
	}
	if embeddingResult.Next() {
		stats.FilesWithEmbeddings = embeddingResult.Record().GetInt64(0, 0)
	}

	// Calculate coverage percentages
	if stats.TotalFiles > 0 {
		stats.TagCoverage = float64(stats.FilesWithTags) / float64(stats.TotalFiles) * 100
		stats.TopicCoverage = float64(stats.FilesWithTopics) / float64(stats.TotalFiles) * 100
		stats.EntityCoverage = float64(stats.FilesWithEntities) / float64(stats.TotalFiles) * 100
		stats.EmbeddingCoverage = float64(stats.FilesWithEmbeddings) / float64(stats.TotalFiles) * 100
	}

	// Calculate full coverage (files with all semantic info)
	fullQuery := `
		MATCH (f:File)
		WHERE EXISTS((f)-[:HAS_TAG]->())
		  AND EXISTS((f)-[:COVERS_TOPIC]->())
		RETURN count(f)
	`
	fullResult, err := g.client.Query(ctx, fullQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count fully covered files; %w", err)
	}
	if fullResult.Next() {
		stats.FullyCoveredFiles = fullResult.Record().GetInt64(0, 0)
	}
	if stats.TotalFiles > 0 {
		stats.FullCoverage = float64(stats.FullyCoveredFiles) / float64(stats.TotalFiles) * 100
	}

	return stats, nil
}

// CoverageStats represents knowledge coverage statistics
type CoverageStats struct {
	TotalFiles          int64   `json:"total_files"`
	FilesWithTags       int64   `json:"files_with_tags"`
	FilesWithTopics     int64   `json:"files_with_topics"`
	FilesWithEntities   int64   `json:"files_with_entities"`
	FilesWithEmbeddings int64   `json:"files_with_embeddings"`
	FullyCoveredFiles   int64   `json:"fully_covered_files"`
	TagCoverage         float64 `json:"tag_coverage_percent"`
	TopicCoverage       float64 `json:"topic_coverage_percent"`
	EntityCoverage      float64 `json:"entity_coverage_percent"`
	EmbeddingCoverage   float64 `json:"embedding_coverage_percent"`
	FullCoverage        float64 `json:"full_coverage_percent"`
}
