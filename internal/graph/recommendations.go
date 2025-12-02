package graph

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
)

// RecommendationWeights configures the importance of different graph signals
type RecommendationWeights struct {
	SharedTags     float64 // Weight for shared tags
	SharedTopics   float64 // Weight for shared topics
	SharedEntities float64 // Weight for shared entities
	SameCategory   float64 // Weight for same category
	SameDirectory  float64 // Weight for same directory
	Similarity     float64 // Weight for embedding similarity
}

// DefaultRecommendationWeights returns sensible default weights
func DefaultRecommendationWeights() RecommendationWeights {
	return RecommendationWeights{
		SharedTags:     1.0,
		SharedTopics:   1.5, // Topics slightly more important
		SharedEntities: 2.0, // Entities most important
		SameCategory:   0.5,
		SameDirectory:  0.3,
		Similarity:     1.0,
	}
}

// Recommendations provides file recommendation capabilities
type Recommendations struct {
	client  *Client
	logger  *slog.Logger
	weights RecommendationWeights
}

// NewRecommendations creates a new Recommendations handler
func NewRecommendations(client *Client, logger *slog.Logger) *Recommendations {
	if logger == nil {
		logger = slog.Default()
	}
	return &Recommendations{
		client:  client,
		logger:  logger.With("component", "graph-recommendations"),
		weights: DefaultRecommendationWeights(),
	}
}

// SetWeights updates the recommendation weights
func (r *Recommendations) SetWeights(weights RecommendationWeights) {
	r.weights = weights
}

// Recommendation represents a recommended file with explanation
type Recommendation struct {
	Path         string   `json:"path"`
	Name         string   `json:"name"`
	Summary      string   `json:"summary"`
	Category     string   `json:"category"`
	Score        float64  `json:"score"`
	Reasons      []string `json:"reasons"`
	SharedTags   []string `json:"shared_tags,omitempty"`
	SharedTopics []string `json:"shared_topics,omitempty"`
	SharedEnts   []string `json:"shared_entities,omitempty"`
}

// RecommendRelated recommends files related to the given file
// Uses graph proximity with weighted scoring
func (r *Recommendations) RecommendRelated(ctx context.Context, filePath string, limit int) ([]Recommendation, error) {
	r.logger.Debug("generating recommendations",
		"source_file", filePath,
		"limit", limit,
	)

	// Get source file info for context
	sourceQuery := `
		MATCH (f:File {path: $filePath})
		OPTIONAL MATCH (f)-[:IN_CATEGORY]->(c:Category)
		OPTIONAL MATCH (f)-[:IN_DIRECTORY]->(d:Directory)
		RETURN f.path, c.name, d.path
	`
	sourceResult, err := r.client.Query(ctx, sourceQuery, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get source file; %w", err)
	}
	if !sourceResult.Next() {
		return nil, fmt.Errorf("file not found; %s", filePath)
	}
	sourceRecord := sourceResult.Record()
	sourceCategory := sourceRecord.GetString(1, "")
	sourceDir := sourceRecord.GetString(2, "")

	// Build combined recommendation query
	// This query finds related files through multiple paths and aggregates signals
	query := `
		MATCH (f:File {path: $filePath})

		// Find files with shared tags
		OPTIONAL MATCH (f)-[:HAS_TAG]->(t:Tag)<-[:HAS_TAG]-(tagRel:File)
		WHERE f <> tagRel

		// Find files with shared topics
		OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(tp:Topic)<-[:COVERS_TOPIC]-(topicRel:File)
		WHERE f <> topicRel

		// Find files with shared entities
		OPTIONAL MATCH (f)-[:MENTIONS]->(e:Entity)<-[:MENTIONS]-(entityRel:File)
		WHERE f <> entityRel

		// Aggregate all related files
		WITH f,
		     collect(DISTINCT {file: tagRel, type: 'tag', name: t.name}) as tagRelations,
		     collect(DISTINCT {file: topicRel, type: 'topic', name: tp.name}) as topicRelations,
		     collect(DISTINCT {file: entityRel, type: 'entity', name: e.name}) as entityRelations

		// Combine all relations
		UNWIND (tagRelations + topicRelations + entityRelations) as rel
		WHERE rel.file IS NOT NULL

		// Group by related file and collect connection info
		WITH rel.file as related,
		     collect(CASE WHEN rel.type = 'tag' THEN rel.name END) as sharedTags,
		     collect(CASE WHEN rel.type = 'topic' THEN rel.name END) as sharedTopics,
		     collect(CASE WHEN rel.type = 'entity' THEN rel.name END) as sharedEntities

		// Get file details and category
		OPTIONAL MATCH (related)-[:IN_CATEGORY]->(c:Category)
		OPTIONAL MATCH (related)-[:IN_DIRECTORY]->(d:Directory)

		RETURN related.path, related.name, related.summary, c.name, d.path,
		       [x IN sharedTags WHERE x IS NOT NULL] as tags,
		       [x IN sharedTopics WHERE x IS NOT NULL] as topics,
		       [x IN sharedEntities WHERE x IS NOT NULL] as entities
		LIMIT $limit * 2
	`
	params := map[string]any{
		"filePath": filePath,
		"limit":    limit,
	}

	result, err := r.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("recommendation query failed; %w", err)
	}

	// Calculate scores and build recommendations
	recMap := make(map[string]*Recommendation)

	for result.Next() {
		record := result.Record()
		path := record.GetString(0, "")
		if path == "" || path == filePath {
			continue
		}

		// Parse shared items
		sharedTags := parseStringArray(record.GetByIndex(5))
		sharedTopics := parseStringArray(record.GetByIndex(6))
		sharedEntities := parseStringArray(record.GetByIndex(7))

		// Get or create recommendation
		rec, exists := recMap[path]
		if !exists {
			rec = &Recommendation{
				Path:     path,
				Name:     record.GetString(1, ""),
				Summary:  record.GetString(2, ""),
				Category: record.GetString(3, ""),
			}
			recMap[path] = rec
		}

		// Accumulate shared items (deduplicate)
		rec.SharedTags = mergeUnique(rec.SharedTags, sharedTags)
		rec.SharedTopics = mergeUnique(rec.SharedTopics, sharedTopics)
		rec.SharedEnts = mergeUnique(rec.SharedEnts, sharedEntities)

		// Calculate score
		relatedCategory := record.GetString(3, "")
		relatedDir := record.GetString(4, "")

		score := 0.0
		var reasons []string

		// Shared tags contribution
		tagScore := float64(len(rec.SharedTags)) * r.weights.SharedTags
		if tagScore > 0 {
			score += tagScore
			reasons = append(reasons, fmt.Sprintf("%d shared tags", len(rec.SharedTags)))
		}

		// Shared topics contribution
		topicScore := float64(len(rec.SharedTopics)) * r.weights.SharedTopics
		if topicScore > 0 {
			score += topicScore
			reasons = append(reasons, fmt.Sprintf("%d shared topics", len(rec.SharedTopics)))
		}

		// Shared entities contribution
		entityScore := float64(len(rec.SharedEnts)) * r.weights.SharedEntities
		if entityScore > 0 {
			score += entityScore
			reasons = append(reasons, fmt.Sprintf("%d shared entities", len(rec.SharedEnts)))
		}

		// Same category bonus
		if sourceCategory != "" && relatedCategory == sourceCategory {
			score += r.weights.SameCategory
			reasons = append(reasons, "same category")
		}

		// Same directory bonus
		if sourceDir != "" && relatedDir == sourceDir {
			score += r.weights.SameDirectory
			reasons = append(reasons, "same directory")
		}

		rec.Score = score
		rec.Reasons = reasons
	}

	// Convert map to sorted slice
	recommendations := make([]Recommendation, 0, len(recMap))
	for _, rec := range recMap {
		if rec.Score > 0 {
			recommendations = append(recommendations, *rec)
		}
	}

	// Sort by score descending
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	// Limit results
	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	r.logger.Debug("generated recommendations",
		"source_file", filePath,
		"count", len(recommendations),
	)

	return recommendations, nil
}

// RecommendByContext recommends files based on multiple context files
// Useful for "what should I read next" given current working context
func (r *Recommendations) RecommendByContext(ctx context.Context, contextFiles []string, limit int) ([]Recommendation, error) {
	if len(contextFiles) == 0 {
		return nil, fmt.Errorf("at least one context file required")
	}

	r.logger.Debug("generating context recommendations",
		"context_files", contextFiles,
		"limit", limit,
	)

	// Collect all entities, tags, and topics from context files
	query := `
		UNWIND $files as filePath
		MATCH (f:File {path: filePath})
		OPTIONAL MATCH (f)-[:HAS_TAG]->(t:Tag)
		OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(tp:Topic)
		OPTIONAL MATCH (f)-[:MENTIONS]->(e:Entity)
		WITH collect(DISTINCT t.name) as contextTags,
		     collect(DISTINCT tp.name) as contextTopics,
		     collect(DISTINCT e.normalized) as contextEntities

		// Find files that share these characteristics
		MATCH (rec:File)
		WHERE NOT rec.path IN $files

		OPTIONAL MATCH (rec)-[:HAS_TAG]->(rt:Tag)
		WHERE rt.name IN contextTags

		OPTIONAL MATCH (rec)-[:COVERS_TOPIC]->(rtp:Topic)
		WHERE rtp.name IN contextTopics

		OPTIONAL MATCH (rec)-[:MENTIONS]->(re:Entity)
		WHERE re.normalized IN contextEntities

		WITH rec,
		     collect(DISTINCT rt.name) as matchedTags,
		     collect(DISTINCT rtp.name) as matchedTopics,
		     collect(DISTINCT re.name) as matchedEntities

		WHERE size(matchedTags) > 0 OR size(matchedTopics) > 0 OR size(matchedEntities) > 0

		RETURN rec.path, rec.name, rec.summary, rec.category,
		       [x IN matchedTags WHERE x IS NOT NULL] as tags,
		       [x IN matchedTopics WHERE x IS NOT NULL] as topics,
		       [x IN matchedEntities WHERE x IS NOT NULL] as entities
		LIMIT $limit * 2
	`
	params := map[string]any{
		"files": contextFiles,
		"limit": limit,
	}

	result, err := r.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("context recommendation query failed; %w", err)
	}

	var recommendations []Recommendation
	seen := make(map[string]bool)

	for result.Next() {
		record := result.Record()
		path := record.GetString(0, "")
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true

		sharedTags := parseStringArray(record.GetByIndex(4))
		sharedTopics := parseStringArray(record.GetByIndex(5))
		sharedEntities := parseStringArray(record.GetByIndex(6))

		// Calculate score
		score := 0.0
		var reasons []string

		tagScore := float64(len(sharedTags)) * r.weights.SharedTags
		if tagScore > 0 {
			score += tagScore
			reasons = append(reasons, fmt.Sprintf("%d shared tags", len(sharedTags)))
		}

		topicScore := float64(len(sharedTopics)) * r.weights.SharedTopics
		if topicScore > 0 {
			score += topicScore
			reasons = append(reasons, fmt.Sprintf("%d shared topics", len(sharedTopics)))
		}

		entityScore := float64(len(sharedEntities)) * r.weights.SharedEntities
		if entityScore > 0 {
			score += entityScore
			reasons = append(reasons, fmt.Sprintf("%d shared entities", len(sharedEntities)))
		}

		recommendations = append(recommendations, Recommendation{
			Path:         path,
			Name:         record.GetString(1, ""),
			Summary:      record.GetString(2, ""),
			Category:     record.GetString(3, ""),
			Score:        score,
			Reasons:      reasons,
			SharedTags:   sharedTags,
			SharedTopics: sharedTopics,
			SharedEnts:   sharedEntities,
		})
	}

	// Sort by score descending
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	// Limit results
	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	return recommendations, nil
}

// TopConnectedFiles returns the most connected files in the graph
// These are files that have the most relationships to other entities
func (r *Recommendations) TopConnectedFiles(ctx context.Context, limit int) ([]ConnectedFile, error) {
	query := `
		MATCH (f:File)
		OPTIONAL MATCH (f)-[:HAS_TAG]->(t:Tag)
		OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(tp:Topic)
		OPTIONAL MATCH (f)-[:MENTIONS]->(e:Entity)
		WITH f,
		     count(DISTINCT t) as tagCount,
		     count(DISTINCT tp) as topicCount,
		     count(DISTINCT e) as entityCount
		RETURN f.path, f.name, f.summary, f.category,
		       tagCount, topicCount, entityCount,
		       tagCount + topicCount + entityCount as totalConnections
		ORDER BY totalConnections DESC
		LIMIT $limit
	`
	result, err := r.client.Query(ctx, query, map[string]any{"limit": limit})
	if err != nil {
		return nil, fmt.Errorf("connected files query failed; %w", err)
	}

	var files []ConnectedFile
	for result.Next() {
		record := result.Record()
		files = append(files, ConnectedFile{
			Path:        record.GetString(0, ""),
			Name:        record.GetString(1, ""),
			Summary:     record.GetString(2, ""),
			Category:    record.GetString(3, ""),
			TagCount:    record.GetInt64(4, 0),
			TopicCount:  record.GetInt64(5, 0),
			EntityCount: record.GetInt64(6, 0),
			TotalConns:  record.GetInt64(7, 0),
		})
	}

	return files, nil
}

// ConnectedFile represents a file with its connection counts
type ConnectedFile struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Category    string `json:"category"`
	TagCount    int64  `json:"tag_count"`
	TopicCount  int64  `json:"topic_count"`
	EntityCount int64  `json:"entity_count"`
	TotalConns  int64  `json:"total_connections"`
}

// Helper functions

func parseStringArray(v any) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			result = append(result, s)
		}
	}
	return result
}

func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool)
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if !seen[s] {
			a = append(a, s)
			seen[s] = true
		}
	}
	return a
}
