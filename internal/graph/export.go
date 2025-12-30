package graph

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Exporter handles exporting graph data to various formats
type Exporter struct {
	manager *Manager
	logger  *slog.Logger
}

// NewExporter creates a new Exporter
func NewExporter(manager *Manager, logger *slog.Logger) *Exporter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Exporter{
		manager: manager,
		logger:  logger.With("component", "graph-exporter"),
	}
}

// ToFileIndex exports the graph to a types.FileIndex (flattened format)
// This is the primary format with FileEntry instead of nested IndexEntry
// When verbose is true, includes related files per entry and graph insights
func (e *Exporter) ToFileIndex(ctx context.Context, memoryRoot string, verbose ...bool) (*types.FileIndex, error) {
	isVerbose := len(verbose) > 0 && verbose[0]
	e.logger.Debug("exporting graph to file index format", "verbose", isVerbose)

	if !e.manager.IsConnected() {
		return nil, fmt.Errorf("graph manager not connected")
	}

	// Get all files from the graph
	entries, err := e.manager.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all entries; %w", err)
	}

	// Get stats
	stats, err := e.manager.GetStats(ctx)
	if err != nil {
		e.logger.Warn("failed to get stats", "error", err)
		stats = &Stats{}
	}

	// Calculate coverage metrics
	coverage := calculateCoverageMetrics(entries)

	// Convert category counts to int
	byCategory := make(map[string]int)
	for k, v := range stats.Categories {
		byCategory[k] = int(v)
	}

	// Convert entries to FileEntry format
	files := make([]types.FileEntry, 0, len(entries))
	for _, entry := range entries {
		fe := convertToFileEntry(entry)

		// In verbose mode, fetch related files for each entry
		if isVerbose {
			relatedFiles, err := e.manager.GetRelatedFiles(ctx, entry.Metadata.Path, 5)
			if err != nil {
				e.logger.Warn("failed to get related files", "path", entry.Metadata.Path, "error", err)
			} else if len(relatedFiles) > 0 {
				fe.RelatedFiles = make([]types.RelatedFile, len(relatedFiles))
				for i, rf := range relatedFiles {
					fe.RelatedFiles[i] = types.RelatedFile{
						Path:  rf.Path,
						Name:  rf.Name,
						Via:   rf.ConnectionType,
						Score: float64(rf.Strength),
					}
				}
			}
		}

		files = append(files, fe)
	}

	totalSize := calculateTotalSize(entries)

	// Get knowledge summary (top tags, topics, entities)
	knowledge := e.getKnowledgeSummary(ctx, 10) // Top 10 of each

	// Build the file index
	index := &types.FileIndex{
		Generated:  time.Now(),
		MemoryRoot: memoryRoot,
		Files:      files,
		Stats: types.IndexStats{
			// File statistics
			TotalFiles:    int(stats.TotalFiles),
			TotalSize:     totalSize,
			AnalyzedFiles: countAnalyzedFiles(entries),
			CachedFiles:   0, // Not tracked in graph
			ErrorFiles:    countErrorFiles(entries),

			// Graph statistics
			TotalTags:     int(stats.TotalTags),
			TotalTopics:   int(stats.TotalTopics),
			TotalEntities: int(stats.TotalEntities),
			TotalEdges:    int(stats.TotalEdges),
			ByCategory:    byCategory,

			// Coverage metrics
			FilesWithSummary:  coverage.filesWithSummary,
			FilesWithTags:     coverage.filesWithTags,
			FilesWithTopics:   coverage.filesWithTopics,
			FilesWithEntities: coverage.filesWithEntities,
			AvgTagsPerFile:    coverage.avgTagsPerFile,
		},
		Knowledge:  knowledge,
		UsageGuide: defaultUsageGuide(),
	}

	// In verbose mode, add graph insights
	if isVerbose {
		insights := e.getInsights(ctx)
		if insights != nil {
			index.Insights = insights
		}
	}

	e.logger.Debug("exported graph to file index format",
		"files", len(files),
		"total_size", totalSize,
		"has_knowledge", knowledge != nil,
		"has_insights", index.Insights != nil,
		"verbose", isVerbose,
	)

	return index, nil
}

// ExportSummary exports a condensed summary suitable for context windows
type ExportSummary struct {
	Generated   time.Time        `json:"generated"`
	Root        string           `json:"root"`
	TotalFiles  int64            `json:"total_files"`
	Categories  map[string]int64 `json:"categories"`
	TopTags     []TagCount       `json:"top_tags"`
	TopTopics   []TopicCount     `json:"top_topics"`
	TopEntities []EntityCount    `json:"top_entities"`
	RecentFiles []FileSummary    `json:"recent_files,omitempty"`
}

// TagCount represents a tag with its usage count
type TagCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// TopicCount represents a topic with its usage count
type TopicCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// EntityCount represents an entity with its mention count
type EntityCount struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

// FileSummary is a condensed file representation
type FileSummary struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Category string    `json:"category"`
	Modified time.Time `json:"modified"`
	Summary  string    `json:"summary,omitempty"`
}

// ToSummary exports a condensed summary of the graph
func (e *Exporter) ToSummary(ctx context.Context, memoryRoot string, recentDays int, topN int) (*ExportSummary, error) {
	e.logger.Debug("exporting graph summary")

	if !e.manager.IsConnected() {
		return nil, fmt.Errorf("graph manager not connected")
	}

	// Get overview
	overview, err := e.manager.queries.GetGraphOverview(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get overview; %w", err)
	}

	summary := &ExportSummary{
		Generated:  time.Now(),
		Root:       memoryRoot,
		TotalFiles: overview.FileCount,
		Categories: overview.CategoryDistribution,
	}

	// Get top tags
	topTags, err := e.getTopTags(ctx, topN)
	if err != nil {
		e.logger.Warn("failed to get top tags", "error", err)
	} else {
		summary.TopTags = topTags
	}

	// Get top topics
	topTopics, err := e.getTopTopics(ctx, topN)
	if err != nil {
		e.logger.Warn("failed to get top topics", "error", err)
	} else {
		summary.TopTopics = topTopics
	}

	// Get top entities
	topEntities, err := e.getTopEntities(ctx, topN)
	if err != nil {
		e.logger.Warn("failed to get top entities", "error", err)
	} else {
		summary.TopEntities = topEntities
	}

	// Get recent files
	if recentDays > 0 {
		recentFiles, err := e.manager.GetRecentFiles(ctx, recentDays, topN)
		if err != nil {
			e.logger.Warn("failed to get recent files", "error", err)
		} else {
			for _, rf := range recentFiles {
				summary.RecentFiles = append(summary.RecentFiles, FileSummary{
					Path:     rf.Path,
					Name:     rf.Name,
					Category: rf.Category,
					Summary:  rf.Summary,
				})
			}
		}
	}

	return summary, nil
}

// getKnowledgeSummary builds a KnowledgeSummary from the graph
func (e *Exporter) getKnowledgeSummary(ctx context.Context, limit int) *types.KnowledgeSummary {
	// Get top tags
	graphTags, err := e.getTopTags(ctx, limit)
	if err != nil {
		e.logger.Warn("failed to get top tags", "error", err)
	}

	// Get top topics
	graphTopics, err := e.getTopTopics(ctx, limit)
	if err != nil {
		e.logger.Warn("failed to get top topics", "error", err)
	}

	// Get top entities
	graphEntities, err := e.getTopEntities(ctx, limit)
	if err != nil {
		e.logger.Warn("failed to get top entities", "error", err)
	}

	// If all failed, return nil
	if len(graphTags) == 0 && len(graphTopics) == 0 && len(graphEntities) == 0 {
		return nil
	}

	// Convert to types.* format
	summary := &types.KnowledgeSummary{}

	if len(graphTags) > 0 {
		summary.TopTags = make([]types.TagCount, len(graphTags))
		for i, t := range graphTags {
			summary.TopTags[i] = types.TagCount{
				Name:  t.Name,
				Count: int(t.Count),
			}
		}
	}

	if len(graphTopics) > 0 {
		summary.TopTopics = make([]types.TopicCount, len(graphTopics))
		for i, t := range graphTopics {
			summary.TopTopics[i] = types.TopicCount{
				Name:  t.Name,
				Count: int(t.Count),
			}
		}
	}

	if len(graphEntities) > 0 {
		summary.TopEntities = make([]types.EntityCount, len(graphEntities))
		for i, e := range graphEntities {
			summary.TopEntities[i] = types.EntityCount{
				Name:  e.Name,
				Type:  e.Type,
				Count: int(e.Count),
			}
		}
	}

	return summary
}

func (e *Exporter) getTopTags(ctx context.Context, limit int) ([]TagCount, error) {
	query := `
		MATCH (t:Tag)<-[:HAS_TAG]-(f:File)
		RETURN t.name, count(f) as count
		ORDER BY count DESC
		LIMIT $limit
	`
	result, err := e.manager.client.Query(ctx, query, map[string]any{"limit": limit})
	if err != nil {
		return nil, err
	}

	var tags []TagCount
	for result.Next() {
		record := result.Record()
		tags = append(tags, TagCount{
			Name:  record.GetString(0, ""),
			Count: record.GetInt64(1, 0),
		})
	}
	return tags, nil
}

func (e *Exporter) getTopTopics(ctx context.Context, limit int) ([]TopicCount, error) {
	query := `
		MATCH (t:Topic)<-[:COVERS_TOPIC]-(f:File)
		RETURN t.name, count(f) as count
		ORDER BY count DESC
		LIMIT $limit
	`
	result, err := e.manager.client.Query(ctx, query, map[string]any{"limit": limit})
	if err != nil {
		return nil, err
	}

	var topics []TopicCount
	for result.Next() {
		record := result.Record()
		topics = append(topics, TopicCount{
			Name:  record.GetString(0, ""),
			Count: record.GetInt64(1, 0),
		})
	}
	return topics, nil
}

func (e *Exporter) getTopEntities(ctx context.Context, limit int) ([]EntityCount, error) {
	query := `
		MATCH (e:Entity)<-[:MENTIONS]-(f:File)
		RETURN e.name, e.type, count(f) as count
		ORDER BY count DESC
		LIMIT $limit
	`
	result, err := e.manager.client.Query(ctx, query, map[string]any{"limit": limit})
	if err != nil {
		return nil, err
	}

	var entities []EntityCount
	for result.Next() {
		record := result.Record()
		entities = append(entities, EntityCount{
			Name:  record.GetString(0, ""),
			Type:  record.GetString(1, ""),
			Count: record.GetInt64(2, 0),
		})
	}
	return entities, nil
}

// getInsights builds graph insights (recommendations, clusters, gaps) for verbose mode
func (e *Exporter) getInsights(ctx context.Context) *types.IndexInsights {
	insights := &types.IndexInsights{}
	hasContent := false

	// Get top connected files as a form of recommendations
	if e.manager.recommendations != nil {
		connFiles, err := e.manager.Recommendations().TopConnectedFiles(ctx, 10)
		if err != nil {
			e.logger.Warn("failed to get top connected files", "error", err)
		} else if len(connFiles) > 0 {
			insights.Recommendations = make([]types.Recommendation, len(connFiles))
			for i, f := range connFiles {
				insights.Recommendations[i] = types.Recommendation{
					SourcePath: f.Path,
					TargetPath: f.Path,
					TargetName: f.Name,
					Reason:     fmt.Sprintf("Highly connected (%d tags, %d topics)", f.TagCount, f.TopicCount),
					Score:      float64(f.TotalConns),
				}
			}
			hasContent = true
		}
	}

	// Get topic clusters (groups of files sharing common topics)
	if e.manager.clusters != nil {
		clusters, err := e.manager.Clusters().DetectTopicClusters(ctx, 2)
		if err != nil {
			e.logger.Warn("failed to detect topic clusters", "error", err)
		} else if len(clusters) > 0 {
			// Limit to top 5 clusters
			maxClusters := 5
			if len(clusters) < maxClusters {
				maxClusters = len(clusters)
			}
			insights.TopicClusters = make([]types.Cluster, maxClusters)
			for i := 0; i < maxClusters; i++ {
				c := clusters[i]
				insights.TopicClusters[i] = types.Cluster{
					Name:       c.Name,
					FileCount:  c.FileCount,
					CommonTags: c.DominantTags,
				}
				// Add file paths (limit to 5)
				if len(c.Files) > 0 {
					maxFiles := 5
					if len(c.Files) < maxFiles {
						maxFiles = len(c.Files)
					}
					insights.TopicClusters[i].FilePaths = make([]string, maxFiles)
					for j := 0; j < maxFiles; j++ {
						insights.TopicClusters[i].FilePaths[j] = c.Files[j].Path
					}
				}
			}
			hasContent = true
		}
	}

	// Get coverage gaps
	if e.manager.gapAnalysis != nil {
		report, err := e.manager.gapAnalysis.AnalyzeGaps(ctx)
		if err != nil {
			e.logger.Warn("failed to analyze gaps", "error", err)
		} else if report != nil && report.TotalGaps > 0 {
			// Collect all gaps (limit to top 10)
			var allGaps []types.Gap

			// Add isolated files (high priority)
			for _, g := range report.IsolatedFiles {
				allGaps = append(allGaps, types.Gap{
					Type:        "isolated_file",
					Name:        g.Name,
					Description: g.Description,
					Severity:    g.Severity,
					Suggestion:  g.Suggestion,
				})
			}

			// Add under-documented topics
			for _, g := range report.UnderDocumentedTopics {
				allGaps = append(allGaps, types.Gap{
					Type:        "under_documented",
					Name:        g.Name,
					Description: g.Description,
					Severity:    g.Severity,
					Suggestion:  g.Suggestion,
				})
			}

			// Add orphaned tags
			for _, g := range report.OrphanedTags {
				allGaps = append(allGaps, types.Gap{
					Type:        "orphaned_tag",
					Name:        g.Name,
					Description: g.Description,
					Severity:    g.Severity,
					Suggestion:  g.Suggestion,
				})
			}

			if len(allGaps) > 10 {
				allGaps = allGaps[:10]
			}
			if len(allGaps) > 0 {
				insights.CoverageGaps = allGaps
				hasContent = true
			}
		}
	}

	if !hasContent {
		return nil
	}

	return insights
}

// Helper functions

func calculateTotalSize(entries []types.IndexEntry) int64 {
	var total int64
	for _, entry := range entries {
		total += entry.Metadata.Size
	}
	return total
}

func countAnalyzedFiles(entries []types.IndexEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.Semantic != nil {
			count++
		}
	}
	return count
}

func countErrorFiles(entries []types.IndexEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.Error != nil {
			count++
		}
	}
	return count
}

// coverageMetrics holds calculated coverage statistics
type coverageMetrics struct {
	filesWithSummary  int
	filesWithTags     int
	filesWithTopics   int
	filesWithEntities int
	avgTagsPerFile    float64
}

func calculateCoverageMetrics(entries []types.IndexEntry) coverageMetrics {
	metrics := coverageMetrics{}
	totalTags := 0

	for _, entry := range entries {
		if entry.Semantic != nil {
			if entry.Semantic.Summary != "" {
				metrics.filesWithSummary++
			}
			if len(entry.Semantic.Tags) > 0 {
				metrics.filesWithTags++
				totalTags += len(entry.Semantic.Tags)
			}
			if len(entry.Semantic.KeyTopics) > 0 {
				metrics.filesWithTopics++
			}
			if len(entry.Semantic.Entities) > 0 {
				metrics.filesWithEntities++
			}
		}
	}

	if len(entries) > 0 {
		metrics.avgTagsPerFile = float64(totalTags) / float64(len(entries))
	}

	return metrics
}

// convertToFileEntry converts a nested IndexEntry to a flattened FileEntry
func convertToFileEntry(entry types.IndexEntry) types.FileEntry {
	fe := types.FileEntry{
		// Identity
		Path: entry.Metadata.Path,
		Name: filepath.Base(entry.Metadata.Path),
		Hash: entry.Metadata.Hash,

		// Classification
		Type:     entry.Metadata.Type,
		Category: entry.Metadata.Category,

		// Physical attributes
		Size:       entry.Metadata.Size,
		SizeHuman:  formatSizeHuman(entry.Metadata.Size),
		Modified:   entry.Metadata.Modified,
		IsReadable: entry.Metadata.IsReadable,

		// Type-specific metadata
		WordCount:  entry.Metadata.WordCount,
		PageCount:  entry.Metadata.PageCount,
		SlideCount: entry.Metadata.SlideCount,
		Dimensions: entry.Metadata.Dimensions,
		Duration:   entry.Metadata.Duration,
		Language:   entry.Metadata.Language,
		Author:     entry.Metadata.Author,

		// Error
		Error: entry.Error,
	}

	// Populate semantic fields if available
	if entry.Semantic != nil {
		fe.Summary = entry.Semantic.Summary
		fe.DocumentType = entry.Semantic.DocumentType
		fe.Confidence = entry.Semantic.Confidence
		fe.Tags = entry.Semantic.Tags
		fe.Topics = entry.Semantic.KeyTopics

		// Convert Entity to EntityRef
		if len(entry.Semantic.Entities) > 0 {
			fe.Entities = make([]types.EntityRef, len(entry.Semantic.Entities))
			for i, e := range entry.Semantic.Entities {
				fe.Entities[i] = types.EntityRef{
					Name: e.Name,
					Type: e.Type,
				}
			}
		}
	}

	return fe
}

// formatSizeHuman formats bytes into human-readable size string
func formatSizeHuman(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// defaultUsageGuide returns the default usage guide for the memory index
func defaultUsageGuide() *types.UsageGuide {
	return &types.UsageGuide{
		Description: "This is a curated collection of files with AI-generated semantic understanding. " +
			"Each file has been analyzed to extract summaries, tags, topics, and entities.",
		WhenToUse: "Reference this index when you need context about available files. " +
			"Read individual files when their content is relevant to the user's query.",
		DirectReadable:     "md, txt, json, yaml, vtt, go, py, js, ts, png, jpg",
		ExtractionRequired: "docx, pptx, pdf",
	}
}

// GetFileEntry retrieves a single file and returns it as a FileEntry with related files
func (e *Exporter) GetFileEntry(ctx context.Context, path string, relatedLimit int) (*types.FileEntry, error) {
	if !e.manager.IsConnected() {
		return nil, fmt.Errorf("graph manager not connected")
	}

	// Get the file as IndexEntry
	entry, err := e.manager.GetFile(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file; %w", err)
	}
	if entry == nil {
		return nil, nil
	}

	// Convert to FileEntry
	fe := convertToFileEntry(*entry)

	// Get related files
	if relatedLimit > 0 {
		relatedFiles, err := e.manager.GetRelatedFiles(ctx, path, relatedLimit)
		if err != nil {
			e.logger.Warn("failed to get related files", "path", path, "error", err)
		} else if len(relatedFiles) > 0 {
			fe.RelatedFiles = make([]types.RelatedFile, len(relatedFiles))
			for i, rf := range relatedFiles {
				fe.RelatedFiles[i] = types.RelatedFile{
					Path:  rf.Path,
					Name:  rf.Name,
					Via:   rf.ConnectionType,
					Score: float64(rf.Strength),
				}
			}
		}
	}

	return &fe, nil
}
