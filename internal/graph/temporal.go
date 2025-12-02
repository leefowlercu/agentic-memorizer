package graph

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// TemporalTracking provides file modification pattern analysis
type TemporalTracking struct {
	client *Client
	logger *slog.Logger
}

// NewTemporalTracking creates a new TemporalTracking handler
func NewTemporalTracking(client *Client, logger *slog.Logger) *TemporalTracking {
	if logger == nil {
		logger = slog.Default()
	}
	return &TemporalTracking{
		client: client,
		logger: logger.With("component", "graph-temporal"),
	}
}

// ModificationEvent represents a file modification event
type ModificationEvent struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Modified time.Time `json:"modified"`
	Category string    `json:"category"`
}

// CoModifiedPair represents two files frequently modified together
type CoModifiedPair struct {
	File1Path    string  `json:"file1_path"`
	File1Name    string  `json:"file1_name"`
	File2Path    string  `json:"file2_path"`
	File2Name    string  `json:"file2_name"`
	CoModCount   int64   `json:"co_modification_count"`
	LastCoMod    string  `json:"last_co_modification"`
	Relationship string  `json:"relationship"` // same_topic, same_entity, same_category, etc.
}

// GetRecentModifications returns files modified within a time window
func (t *TemporalTracking) GetRecentModifications(ctx context.Context, since time.Time, limit int) ([]ModificationEvent, error) {
	query := `
		MATCH (f:File)
		WHERE f.modified >= $since
		RETURN f.path, f.name, f.modified, f.category
		ORDER BY f.modified DESC
		LIMIT $limit
	`
	params := map[string]any{
		"since": since.Format(time.RFC3339),
		"limit": limit,
	}

	result, err := t.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent modifications; %w", err)
	}

	var events []ModificationEvent
	for result.Next() {
		record := result.Record()
		modStr := record.GetString(2, "")
		modTime, _ := time.Parse(time.RFC3339, modStr)

		events = append(events, ModificationEvent{
			Path:     record.GetString(0, ""),
			Name:     record.GetString(1, ""),
			Modified: modTime,
			Category: record.GetString(3, ""),
		})
	}

	return events, nil
}

// FindCoModifiedFiles finds files that were modified within the same time window
// windowMinutes defines how close modifications must be to be considered "co-modified"
func (t *TemporalTracking) FindCoModifiedFiles(ctx context.Context, windowMinutes int, limit int) ([]CoModifiedPair, error) {
	t.logger.Debug("finding co-modified files",
		"window_minutes", windowMinutes,
		"limit", limit,
	)

	// Find files modified within the window of each other that share some connection
	query := `
		MATCH (f1:File), (f2:File)
		WHERE f1 <> f2 AND id(f1) < id(f2)
		AND abs(duration.between(datetime(f1.modified), datetime(f2.modified)).minutes) <= $windowMinutes

		// Check for shared connections
		OPTIONAL MATCH (f1)-[:COVERS_TOPIC]->(t:Topic)<-[:COVERS_TOPIC]-(f2)
		OPTIONAL MATCH (f1)-[:HAS_TAG]->(tag:Tag)<-[:HAS_TAG]-(f2)
		OPTIONAL MATCH (f1)-[:MENTIONS]->(e:Entity)<-[:MENTIONS]-(f2)
		OPTIONAL MATCH (f1)-[:IN_CATEGORY]->(c:Category)<-[:IN_CATEGORY]-(f2)

		WITH f1, f2,
		     count(DISTINCT t) as sharedTopics,
		     count(DISTINCT tag) as sharedTags,
		     count(DISTINCT e) as sharedEntities,
		     count(DISTINCT c) as sharedCategories

		WHERE sharedTopics > 0 OR sharedTags > 0 OR sharedEntities > 0 OR sharedCategories > 0

		RETURN f1.path, f1.name, f2.path, f2.name,
		       CASE
		         WHEN sharedEntities > 0 THEN 'shared_entity'
		         WHEN sharedTopics > 0 THEN 'shared_topic'
		         WHEN sharedTags > 0 THEN 'shared_tag'
		         ELSE 'shared_category'
		       END as relationship,
		       f1.modified as lastMod
		ORDER BY lastMod DESC
		LIMIT $limit
	`
	params := map[string]any{
		"windowMinutes": windowMinutes,
		"limit":         limit,
	}

	result, err := t.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find co-modified files; %w", err)
	}

	var pairs []CoModifiedPair
	for result.Next() {
		record := result.Record()
		pairs = append(pairs, CoModifiedPair{
			File1Path:    record.GetString(0, ""),
			File1Name:    record.GetString(1, ""),
			File2Path:    record.GetString(2, ""),
			File2Name:    record.GetString(3, ""),
			Relationship: record.GetString(4, ""),
			LastCoMod:    record.GetString(5, ""),
			CoModCount:   1, // Would need historical data to track actual count
		})
	}

	return pairs, nil
}

// GetModificationTimeline returns a timeline of modifications grouped by day
func (t *TemporalTracking) GetModificationTimeline(ctx context.Context, days int) ([]DayActivity, error) {
	cutoff := time.Now().AddDate(0, 0, -days)

	query := `
		MATCH (f:File)
		WHERE f.modified >= $cutoff
		WITH date(datetime(f.modified)) as modDate, count(f) as fileCount,
		     collect(f.category) as categories
		RETURN modDate, fileCount, categories
		ORDER BY modDate DESC
	`
	result, err := t.client.Query(ctx, query, map[string]any{
		"cutoff": cutoff.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get modification timeline; %w", err)
	}

	var timeline []DayActivity
	for result.Next() {
		record := result.Record()

		// Parse categories
		catMap := make(map[string]int)
		if cats, ok := record.GetByIndex(2).([]any); ok {
			for _, c := range cats {
				if cs, ok := c.(string); ok && cs != "" {
					catMap[cs]++
				}
			}
		}

		timeline = append(timeline, DayActivity{
			Date:               record.GetString(0, ""),
			FileCount:          record.GetInt64(1, 0),
			CategoryBreakdown:  catMap,
		})
	}

	return timeline, nil
}

// DayActivity represents modification activity for a single day
type DayActivity struct {
	Date              string         `json:"date"`
	FileCount         int64          `json:"file_count"`
	CategoryBreakdown map[string]int `json:"category_breakdown"`
}

// GetFileModificationHistory returns the modification pattern for a specific file
func (t *TemporalTracking) GetFileModificationHistory(ctx context.Context, filePath string) (*FileHistory, error) {
	// Get the file's current info
	query := `
		MATCH (f:File {path: $filePath})
		RETURN f.modified, f.category
	`
	result, err := t.client.Query(ctx, query, map[string]any{"filePath": filePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get file history; %w", err)
	}

	if !result.Next() {
		return nil, fmt.Errorf("file not found; %s", filePath)
	}

	record := result.Record()
	modStr := record.GetString(0, "")
	modTime, _ := time.Parse(time.RFC3339, modStr)

	history := &FileHistory{
		Path:         filePath,
		LastModified: modTime,
		Category:     record.GetString(1, ""),
	}

	// Find files modified around the same time (potential co-edits)
	coEditQuery := `
		MATCH (f:File {path: $filePath}), (other:File)
		WHERE f <> other
		AND abs(duration.between(datetime(f.modified), datetime(other.modified)).minutes) <= 60
		OPTIONAL MATCH (f)-[:COVERS_TOPIC]->(t:Topic)<-[:COVERS_TOPIC]-(other)
		OPTIONAL MATCH (f)-[:MENTIONS]->(e:Entity)<-[:MENTIONS]-(other)
		WITH other, count(t) + count(e) as connectionStrength
		WHERE connectionStrength > 0
		RETURN other.path, other.name, other.modified, connectionStrength
		ORDER BY connectionStrength DESC
		LIMIT 10
	`
	coEditResult, err := t.client.Query(ctx, coEditQuery, map[string]any{"filePath": filePath})
	if err != nil {
		t.logger.Warn("failed to get co-edited files", "error", err)
	} else {
		for coEditResult.Next() {
			rec := coEditResult.Record()
			coModStr := rec.GetString(2, "")
			coModTime, _ := time.Parse(time.RFC3339, coModStr)

			history.CoEditedFiles = append(history.CoEditedFiles, CoEditedFile{
				Path:               rec.GetString(0, ""),
				Name:               rec.GetString(1, ""),
				Modified:           coModTime,
				ConnectionStrength: rec.GetInt64(3, 0),
			})
		}
	}

	return history, nil
}

// FileHistory represents the modification history of a file
type FileHistory struct {
	Path          string         `json:"path"`
	LastModified  time.Time      `json:"last_modified"`
	Category      string         `json:"category"`
	CoEditedFiles []CoEditedFile `json:"co_edited_files,omitempty"`
}

// CoEditedFile represents a file that was edited around the same time
type CoEditedFile struct {
	Path               string    `json:"path"`
	Name               string    `json:"name"`
	Modified           time.Time `json:"modified"`
	ConnectionStrength int64     `json:"connection_strength"`
}

// GetActivityHotspots finds files or areas with high modification activity
func (t *TemporalTracking) GetActivityHotspots(ctx context.Context, days int, limit int) ([]ActivityHotspot, error) {
	cutoff := time.Now().AddDate(0, 0, -days)

	// For now, identify "hotspots" based on directory activity
	query := `
		MATCH (f:File)-[:IN_DIRECTORY]->(d:Directory)
		WHERE f.modified >= $cutoff
		WITH d, count(f) as modCount, max(f.modified) as lastMod
		WHERE modCount >= 2
		RETURN d.path, modCount, lastMod
		ORDER BY modCount DESC
		LIMIT $limit
	`
	result, err := t.client.Query(ctx, query, map[string]any{
		"cutoff": cutoff.Format(time.RFC3339),
		"limit":  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get activity hotspots; %w", err)
	}

	var hotspots []ActivityHotspot
	for result.Next() {
		record := result.Record()
		lastModStr := record.GetString(2, "")
		lastModTime, _ := time.Parse(time.RFC3339, lastModStr)

		hotspots = append(hotspots, ActivityHotspot{
			Path:            record.GetString(0, ""),
			ModificationCount: record.GetInt64(1, 0),
			LastModified:    lastModTime,
		})
	}

	return hotspots, nil
}

// ActivityHotspot represents an area with high modification activity
type ActivityHotspot struct {
	Path              string    `json:"path"`
	ModificationCount int64     `json:"modification_count"`
	LastModified      time.Time `json:"last_modified"`
}

// GetStaleFiles finds files that haven't been modified in a long time
func (t *TemporalTracking) GetStaleFiles(ctx context.Context, staleDays int, limit int) ([]StaleFile, error) {
	cutoff := time.Now().AddDate(0, 0, -staleDays)

	query := `
		MATCH (f:File)
		WHERE f.modified < $cutoff
		RETURN f.path, f.name, f.modified, f.category
		ORDER BY f.modified ASC
		LIMIT $limit
	`
	result, err := t.client.Query(ctx, query, map[string]any{
		"cutoff": cutoff.Format(time.RFC3339),
		"limit":  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get stale files; %w", err)
	}

	var staleFiles []StaleFile
	now := time.Now()
	for result.Next() {
		record := result.Record()
		modStr := record.GetString(2, "")
		modTime, _ := time.Parse(time.RFC3339, modStr)

		staleFiles = append(staleFiles, StaleFile{
			Path:        record.GetString(0, ""),
			Name:        record.GetString(1, ""),
			Modified:    modTime,
			Category:    record.GetString(3, ""),
			DaysSinceModification: int(now.Sub(modTime).Hours() / 24),
		})
	}

	return staleFiles, nil
}

// StaleFile represents a file that hasn't been modified recently
type StaleFile struct {
	Path                  string    `json:"path"`
	Name                  string    `json:"name"`
	Modified              time.Time `json:"modified"`
	Category              string    `json:"category"`
	DaysSinceModification int       `json:"days_since_modification"`
}

// GetTemporalStats returns overall temporal statistics
func (t *TemporalTracking) GetTemporalStats(ctx context.Context) (*TemporalStats, error) {
	stats := &TemporalStats{}

	// Get oldest and newest files
	rangeQuery := `
		MATCH (f:File)
		RETURN min(f.modified) as oldest, max(f.modified) as newest, count(f) as total
	`
	rangeResult, err := t.client.Query(ctx, rangeQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get temporal range; %w", err)
	}
	if rangeResult.Next() {
		record := rangeResult.Record()
		oldestStr := record.GetString(0, "")
		newestStr := record.GetString(1, "")

		if oldestStr != "" {
			oldest, _ := time.Parse(time.RFC3339, oldestStr)
			stats.OldestFile = oldest
		}
		if newestStr != "" {
			newest, _ := time.Parse(time.RFC3339, newestStr)
			stats.NewestFile = newest
		}
		stats.TotalFiles = record.GetInt64(2, 0)
	}

	// Count files modified in different time windows
	now := time.Now()
	windows := []struct {
		name   string
		cutoff time.Time
	}{
		{"last_day", now.AddDate(0, 0, -1)},
		{"last_week", now.AddDate(0, 0, -7)},
		{"last_month", now.AddDate(0, -1, 0)},
	}

	stats.ActivityWindows = make(map[string]int64)
	for _, w := range windows {
		windowQuery := `
			MATCH (f:File)
			WHERE f.modified >= $cutoff
			RETURN count(f)
		`
		windowResult, err := t.client.Query(ctx, windowQuery, map[string]any{
			"cutoff": w.cutoff.Format(time.RFC3339),
		})
		if err != nil {
			continue
		}
		if windowResult.Next() {
			stats.ActivityWindows[w.name] = windowResult.Record().GetInt64(0, 0)
		}
	}

	return stats, nil
}

// TemporalStats contains overall temporal statistics
type TemporalStats struct {
	TotalFiles      int64            `json:"total_files"`
	OldestFile      time.Time        `json:"oldest_file"`
	NewestFile      time.Time        `json:"newest_file"`
	ActivityWindows map[string]int64 `json:"activity_windows"`
}
