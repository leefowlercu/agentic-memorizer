package graph

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// ManagerConfig contains configuration for the GraphManager
type ManagerConfig struct {
	Client     ClientConfig
	Schema     SchemaConfig
	MemoryRoot string
}

// DefaultManagerConfig returns default manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Client: DefaultClientConfig(),
		Schema: DefaultSchemaConfig(),
	}
}

// Manager orchestrates all graph operations
type Manager struct {
	client          *Client
	schema          *Schema
	nodes           *Nodes
	edges           *Edges
	queries         *Queries
	facts           *Facts
	disambiguation  *Disambiguation
	recommendations *Recommendations
	clusters        *ClusterDetection
	gapAnalysis     *GapAnalysis
	temporal        *TemporalTracking

	config    ManagerConfig
	logger    *slog.Logger
	mu        sync.RWMutex
	connected bool
}

// NewManager creates a new GraphManager
func NewManager(config ManagerConfig, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	client := NewClient(config.Client, logger)

	return &Manager{
		client: client,
		config: config,
		logger: logger.With("component", "graph-manager"),
	}
}

// Initialize connects to FalkorDB and sets up the schema
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("initializing graph manager")

	// Connect to FalkorDB
	if err := m.client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to FalkorDB; %w", err)
	}

	// Initialize sub-components in dependency order:
	// 1. Schema first (creates constraints and indexes)
	// 2. Nodes/Edges/Facts (core CRUD operations, depend on schema)
	// 3. Queries (read-only operations, depend on nodes/edges)
	// 4. Analytics (disambiguation, recommendations, clusters, gaps, temporal)
	m.schema = NewSchema(m.client, m.config.Schema, m.logger)
	m.nodes = NewNodes(m.client, m.logger)
	m.edges = NewEdges(m.client, m.logger)
	m.facts = NewFacts(m.client, m.logger)
	m.queries = NewQueries(m.client, m.logger)
	m.disambiguation = NewDisambiguation(m.client, m.logger)
	m.recommendations = NewRecommendations(m.client, m.logger)
	m.clusters = NewClusterDetection(m.client, m.logger)
	m.gapAnalysis = NewGapAnalysis(m.client, m.logger)
	m.temporal = NewTemporalTracking(m.client, m.logger)

	// Initialize schema
	if err := m.schema.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize schema; %w", err)
	}

	m.connected = true
	m.logger.Info("graph manager initialized successfully")
	return nil
}

// Close closes the graph connection
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connected = false
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// IsConnected returns whether the manager is connected
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected && m.client.IsConnected()
}

// Health returns health status
func (m *Manager) Health(ctx context.Context) (*HealthStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return &HealthStatus{
			Connected: false,
			Error:     "manager not initialized",
			Timestamp: time.Now(),
		}, nil
	}

	return m.client.Health(ctx)
}

// UpdateInfo provides context about the entry being updated
type UpdateInfo struct {
	WasAnalyzed bool // true if semantic analysis was performed (API call)
	WasCached   bool // true if cached analysis was used
	HadError    bool // true if there was an error processing this file
}

// UpdateResult contains information about what the update operation did
type UpdateResult struct {
	Added   bool // true if new entry was added (vs updated)
	Updated bool // true if existing entry was modified
}

// UpdateSingle updates a single file entry in the graph
func (m *Manager) UpdateSingle(ctx context.Context, entry types.IndexEntry, info UpdateInfo) (UpdateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := UpdateResult{}

	if !m.connected {
		return result, fmt.Errorf("graph manager not connected")
	}

	// Check if file exists
	exists, err := m.nodes.FileExists(ctx, entry.Metadata.Path)
	if err != nil {
		return result, fmt.Errorf("failed to check file existence; %w", err)
	}

	// Create/update file node
	fileNode := FileNodeFromEntry(entry)
	if err := m.nodes.UpsertFile(ctx, fileNode); err != nil {
		return result, fmt.Errorf("failed to upsert file node; %w", err)
	}

	// Remove existing edges before creating new ones (rebuild strategy).
	// Alternative: delta updates. Chosen: full rebuild is simpler, ensures consistency.
	// Tags/topics/entities may have changed during re-analysis, so clean slate is safer.
	if exists {
		if err := m.edges.RemoveFileEdges(ctx, entry.Metadata.Path); err != nil {
			m.logger.Warn("failed to remove existing edges", "path", entry.Metadata.Path, "error", err)
		}
		result.Updated = true
	} else {
		result.Added = true
	}

	// Link to category
	if entry.Metadata.Category != "" {
		if err := m.edges.LinkFileToCategory(ctx, entry.Metadata.Path, entry.Metadata.Category); err != nil {
			m.logger.Warn("failed to link file to category", "error", err)
		}
	}

	// Link to directory
	dirPath := filepath.Dir(entry.Metadata.Path)
	if err := m.edges.LinkFileToDirectory(ctx, entry.Metadata.Path, dirPath); err != nil {
		m.logger.Warn("failed to link file to directory", "error", err)
	}

	// Process semantic analysis
	if entry.Semantic != nil {
		// Link tags
		for _, tag := range entry.Semantic.Tags {
			if err := m.edges.LinkFileToTag(ctx, entry.Metadata.Path, tag); err != nil {
				m.logger.Warn("failed to link file to tag", "tag", tag, "error", err)
			}
		}

		// Link topics
		for _, topic := range entry.Semantic.KeyTopics {
			if err := m.edges.LinkFileToTopic(ctx, entry.Metadata.Path, topic); err != nil {
				m.logger.Warn("failed to link file to topic", "topic", topic, "error", err)
			}
		}

		// Link entities
		for _, entity := range entry.Semantic.Entities {
			if err := m.edges.LinkFileToEntity(ctx, entry.Metadata.Path, entity.Name, entity.Type); err != nil {
				m.logger.Warn("failed to link file to entity", "entity", entity.Name, "error", err)
			}
		}

		// Link references
		for _, ref := range entry.Semantic.References {
			if err := m.edges.LinkFileToReference(ctx, entry.Metadata.Path, ref.Topic, ref.Type, ref.Confidence); err != nil {
				m.logger.Warn("failed to link file reference", "topic", ref.Topic, "error", err)
			}
		}
	}

	m.logger.Debug("updated file in graph",
		"path", entry.Metadata.Path,
		"added", result.Added,
		"updated", result.Updated,
	)

	return result, nil
}

// UpdateSingleWithEmbedding updates a single file entry in the graph with an embedding vector
// The provider parameter determines which embedding property to set (e.g., "openai" -> "embedding_openai")
func (m *Manager) UpdateSingleWithEmbedding(ctx context.Context, entry types.IndexEntry, info UpdateInfo, embedding []float32, provider string) (UpdateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := UpdateResult{}

	if !m.connected {
		return result, fmt.Errorf("graph manager not connected")
	}

	// Check if file exists
	exists, err := m.nodes.FileExists(ctx, entry.Metadata.Path)
	if err != nil {
		return result, fmt.Errorf("failed to check file existence; %w", err)
	}

	// Create/update file node with embedding
	fileNode := FileNodeFromEntry(entry)
	if len(embedding) > 0 {
		if err := m.nodes.UpsertFileWithEmbedding(ctx, fileNode, embedding, provider); err != nil {
			return result, fmt.Errorf("failed to upsert file node with embedding; %w", err)
		}
	} else {
		if err := m.nodes.UpsertFile(ctx, fileNode); err != nil {
			return result, fmt.Errorf("failed to upsert file node; %w", err)
		}
	}

	// Remove existing edges for this file (to rebuild them)
	if exists {
		if err := m.edges.RemoveFileEdges(ctx, entry.Metadata.Path); err != nil {
			m.logger.Warn("failed to remove existing edges", "path", entry.Metadata.Path, "error", err)
		}
		result.Updated = true
	} else {
		result.Added = true
	}

	// Link to category
	if entry.Metadata.Category != "" {
		if err := m.edges.LinkFileToCategory(ctx, entry.Metadata.Path, entry.Metadata.Category); err != nil {
			m.logger.Warn("failed to link file to category", "error", err)
		}
	}

	// Link to directory
	dirPath := filepath.Dir(entry.Metadata.Path)
	if err := m.edges.LinkFileToDirectory(ctx, entry.Metadata.Path, dirPath); err != nil {
		m.logger.Warn("failed to link file to directory", "error", err)
	}

	// Process semantic analysis
	if entry.Semantic != nil {
		// Link tags
		for _, tag := range entry.Semantic.Tags {
			if err := m.edges.LinkFileToTag(ctx, entry.Metadata.Path, tag); err != nil {
				m.logger.Warn("failed to link file to tag", "tag", tag, "error", err)
			}
		}

		// Link topics
		for _, topic := range entry.Semantic.KeyTopics {
			if err := m.edges.LinkFileToTopic(ctx, entry.Metadata.Path, topic); err != nil {
				m.logger.Warn("failed to link file to topic", "topic", topic, "error", err)
			}
		}

		// Link entities
		for _, entity := range entry.Semantic.Entities {
			if err := m.edges.LinkFileToEntity(ctx, entry.Metadata.Path, entity.Name, entity.Type); err != nil {
				m.logger.Warn("failed to link file to entity", "entity", entity.Name, "error", err)
			}
		}

		// Link references
		for _, ref := range entry.Semantic.References {
			if err := m.edges.LinkFileToReference(ctx, entry.Metadata.Path, ref.Topic, ref.Type, ref.Confidence); err != nil {
				m.logger.Warn("failed to link file reference", "topic", ref.Topic, "error", err)
			}
		}
	}

	m.logger.Debug("updated file in graph with embedding",
		"path", entry.Metadata.Path,
		"added", result.Added,
		"updated", result.Updated,
		"has_embedding", len(embedding) > 0,
	)

	return result, nil
}

// UpdateSingleWithEntities updates a file with enhanced semantic analysis including entities
func (m *Manager) UpdateSingleWithEntities(ctx context.Context, entry types.IndexEntry, entities []Entity, references []ReferenceInfo) (UpdateResult, error) {
	// First do the basic update
	result, err := m.UpdateSingle(ctx, entry, UpdateInfo{WasAnalyzed: true})
	if err != nil {
		return result, err
	}

	// Link entities
	for _, entity := range entities {
		if err := m.edges.LinkFileToEntity(ctx, entry.Metadata.Path, entity.Name, entity.Type); err != nil {
			m.logger.Warn("failed to link file to entity", "entity", entity.Name, "error", err)
		}
	}

	// Link references
	for _, ref := range references {
		if err := m.edges.LinkFileToReference(ctx, entry.Metadata.Path, ref.Topic, ref.RefType, ref.Confidence); err != nil {
			m.logger.Warn("failed to link file reference", "topic", ref.Topic, "error", err)
		}
	}

	return result, nil
}

// Entity represents an extracted entity
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"` // technology, person, concept, organization
}

// ReferenceInfo represents a reference to a topic
type ReferenceInfo struct {
	Topic      string  `json:"topic"`
	RefType    string  `json:"ref_type"` // requires, extends, related-to
	Confidence float64 `json:"confidence"`
}

// RemoveFile removes a file from the graph
func (m *Manager) RemoveFile(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return fmt.Errorf("graph manager not connected")
	}

	// Delete file and all its edges
	if err := m.nodes.DeleteFile(ctx, path); err != nil {
		return fmt.Errorf("failed to delete file; %w", err)
	}

	m.logger.Debug("removed file from graph", "path", path)
	return nil
}

// RemoveStaleFiles removes graph nodes for files that no longer exist on disk.
// Takes a set of current filesystem paths and removes any graph nodes not in that set.
// Returns the number of stale files removed.
func (m *Manager) RemoveStaleFiles(ctx context.Context, currentPaths map[string]bool) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return 0, fmt.Errorf("graph manager not connected")
	}

	// Get all file paths from graph
	graphPaths, err := m.nodes.ListFilePaths(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list graph file paths; %w", err)
	}

	// Find stale paths (in graph but not on disk)
	var stalePaths []string
	for _, graphPath := range graphPaths {
		if !currentPaths[graphPath] {
			stalePaths = append(stalePaths, graphPath)
		}
	}

	if len(stalePaths) == 0 {
		return 0, nil
	}

	// Remove stale files
	var removed int64
	for _, path := range stalePaths {
		if err := m.nodes.DeleteFile(ctx, path); err != nil {
			m.logger.Warn("failed to delete stale file", "path", path, "error", err)
			continue
		}
		removed++
		m.logger.Debug("removed stale file from graph", "path", path)
	}

	m.logger.Info("removed stale files from graph", "count", removed)
	return removed, nil
}

// GetAll returns all file entries (similar to index.Manager.GetCurrent)
func (m *Manager) GetAll(ctx context.Context) ([]types.IndexEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	files, err := m.nodes.ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list files; %w", err)
	}

	entries := make([]types.IndexEntry, 0, len(files))
	for _, file := range files {
		entry := types.IndexEntry{
			Metadata: types.FileMetadata{
				FileInfo: types.FileInfo{
					Path:     file.Path,
					Hash:     file.Hash,
					Size:     file.Size,
					Modified: file.Modified,
					Type:     file.Type,
					Category: file.Category,
				},
			},
		}

		if file.Summary != "" {
			// Get tags and topics for this file
			tags, _ := m.edges.GetFileTags(ctx, file.Path)
			topics, _ := m.edges.GetFileTopics(ctx, file.Path)

			// Get entities and references for this file
			entityNodes, _ := m.edges.GetFileEntities(ctx, file.Path)
			entities := make([]types.Entity, 0, len(entityNodes))
			for _, en := range entityNodes {
				entities = append(entities, types.Entity{
					Name: en.Name,
					Type: en.Type,
				})
			}

			referenceNodes, _ := m.edges.GetFileReferences(ctx, file.Path)
			references := make([]types.Reference, 0, len(referenceNodes))
			for _, rn := range referenceNodes {
				references = append(references, types.Reference{
					Topic:      rn.Topic,
					Type:       rn.RefType,
					Confidence: rn.Confidence,
				})
			}

			entry.Semantic = &types.SemanticAnalysis{
				Summary:      file.Summary,
				DocumentType: file.DocumentType,
				Confidence:   file.Confidence,
				Tags:         tags,
				KeyTopics:    topics,
				Entities:     entities,
				References:   references,
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetFile returns a single file entry
func (m *Manager) GetFile(ctx context.Context, path string) (*types.IndexEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	file, err := m.nodes.GetFile(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file; %w", err)
	}
	if file == nil {
		return nil, nil
	}

	entry := &types.IndexEntry{
		Metadata: types.FileMetadata{
			FileInfo: types.FileInfo{
				Path:     file.Path,
				Hash:     file.Hash,
				Size:     file.Size,
				Modified: file.Modified,
				Type:     file.Type,
				Category: file.Category,
			},
		},
	}

	if file.Summary != "" {
		tags, _ := m.edges.GetFileTags(ctx, file.Path)
		topics, _ := m.edges.GetFileTopics(ctx, file.Path)

		// Get entities and references for this file
		entityNodes, _ := m.edges.GetFileEntities(ctx, file.Path)
		entities := make([]types.Entity, 0, len(entityNodes))
		for _, en := range entityNodes {
			entities = append(entities, types.Entity{
				Name: en.Name,
				Type: en.Type,
			})
		}

		referenceNodes, _ := m.edges.GetFileReferences(ctx, file.Path)
		references := make([]types.Reference, 0, len(referenceNodes))
		for _, rn := range referenceNodes {
			references = append(references, types.Reference{
				Topic:      rn.Topic,
				Type:       rn.RefType,
				Confidence: rn.Confidence,
			})
		}

		entry.Semantic = &types.SemanticAnalysis{
			Summary:      file.Summary,
			DocumentType: file.DocumentType,
			Confidence:   file.Confidence,
			Tags:         tags,
			KeyTopics:    topics,
			Entities:     entities,
			References:   references,
		}
	}

	return entry, nil
}

// GetStats returns graph statistics
func (m *Manager) GetStats(ctx context.Context) (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	overview, err := m.queries.GetGraphOverview(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph overview; %w", err)
	}

	return &Stats{
		TotalFiles:    overview.FileCount,
		TotalTags:     overview.TagCount,
		TotalTopics:   overview.TopicCount,
		TotalEntities: overview.EntityCount,
		TotalEdges:    overview.RelationshipCount,
		Categories:    overview.CategoryDistribution,
	}, nil
}

// Stats represents graph statistics
type Stats struct {
	TotalFiles    int64            `json:"total_files"`
	TotalTags     int64            `json:"total_tags"`
	TotalTopics   int64            `json:"total_topics"`
	TotalEntities int64            `json:"total_entities"`
	TotalEdges    int64            `json:"total_edges"`
	Categories    map[string]int64 `json:"categories"`
}

// Search performs a combined search across multiple dimensions
func (m *Manager) Search(ctx context.Context, query string, limit int, categoryFilter string) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	var allResults []SearchResult

	// Search by filename
	filenameResults, err := m.queries.SearchByFilename(ctx, query, limit)
	if err == nil {
		allResults = append(allResults, filenameResults...)
	}

	// Search by tag
	tagResults, err := m.queries.SearchByTag(ctx, query, limit)
	if err == nil {
		allResults = append(allResults, tagResults...)
	}

	// Search by topic
	topicResults, err := m.queries.SearchByTopic(ctx, query, limit)
	if err == nil {
		allResults = append(allResults, topicResults...)
	}

	// Search by entity
	entityResults, err := m.queries.SearchByEntity(ctx, query, limit)
	if err == nil {
		allResults = append(allResults, entityResults...)
	}

	// Deduplicate results by path
	seen := make(map[string]bool)
	var uniqueResults []SearchResult
	for _, r := range allResults {
		if !seen[r.Path] {
			// Apply category filter if specified
			if categoryFilter != "" && strings.ToLower(r.Category) != strings.ToLower(categoryFilter) {
				continue
			}
			seen[r.Path] = true
			uniqueResults = append(uniqueResults, r)
		}
	}

	// Limit results
	if len(uniqueResults) > limit {
		uniqueResults = uniqueResults[:limit]
	}

	return uniqueResults, nil
}

// VectorSearch performs vector similarity search
// The provider parameter determines which embedding property to search (e.g., "openai" -> "embedding_openai")
func (m *Manager) VectorSearch(ctx context.Context, embedding []float32, limit int, provider string) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.queries.VectorSearch(ctx, embedding, limit, provider)
}

// GetRecentFiles returns recently modified files
func (m *Manager) GetRecentFiles(ctx context.Context, days int, limit int) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.queries.GetRecentFiles(ctx, days, limit)
}

// GetRelatedFiles returns files related to the given file
func (m *Manager) GetRelatedFiles(ctx context.Context, filePath string, limit int) ([]RelatedFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.queries.GetRelatedFiles(ctx, filePath, limit)
}

// GetFileConnections returns all connections for a file
func (m *Manager) GetFileConnections(ctx context.Context, filePath string) (*FileConnections, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.queries.GetFileConnections(ctx, filePath)
}

// ClearGraph removes all data from the graph
func (m *Manager) ClearGraph(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return fmt.Errorf("graph manager not connected")
	}

	if err := m.schema.ClearGraph(ctx); err != nil {
		return fmt.Errorf("failed to clear graph; %w", err)
	}

	// Re-initialize categories
	if err := m.schema.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to reinitialize schema; %w", err)
	}

	return nil
}

// Cleanup removes orphaned nodes
func (m *Manager) Cleanup(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return 0, fmt.Errorf("graph manager not connected")
	}

	return m.edges.CleanupOrphanedNodes(ctx)
}

// Nodes returns the Nodes handler for direct node operations
func (m *Manager) Nodes() *Nodes {
	return m.nodes
}

// Edges returns the Edges handler for direct edge operations
func (m *Manager) Edges() *Edges {
	return m.edges
}

// Queries returns the Queries handler for direct query operations
func (m *Manager) Queries() *Queries {
	return m.queries
}

// Facts returns the Facts handler for direct fact operations
func (m *Manager) Facts() *Facts {
	return m.facts
}

// Client returns the underlying FalkorDB client
func (m *Manager) Client() *Client {
	return m.client
}

// Disambiguation returns the Disambiguation handler for entity operations
func (m *Manager) Disambiguation() *Disambiguation {
	return m.disambiguation
}

// NormalizeEntities runs entity normalization and deduplication across the graph
func (m *Manager) NormalizeEntities(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return 0, fmt.Errorf("graph manager not connected")
	}

	return m.disambiguation.NormalizeAllEntities(ctx)
}

// FindDuplicateEntities finds entities that might be duplicates
func (m *Manager) FindDuplicateEntities(ctx context.Context) ([]DuplicateEntity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.disambiguation.FindDuplicateEntities(ctx)
}

// MergeEntities merges one entity into another
func (m *Manager) MergeEntities(ctx context.Context, source, target string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return 0, fmt.Errorf("graph manager not connected")
	}

	return m.disambiguation.MergeEntities(ctx, source, target)
}

// GetEntityVariants returns all variant names for an entity
func (m *Manager) GetEntityVariants(ctx context.Context, entityName string) ([]EntityVariant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.disambiguation.GetEntityVariants(ctx, entityName)
}

// FindSimilarEntities finds entities similar to the given name
func (m *Manager) FindSimilarEntities(ctx context.Context, name string, limit int) ([]SimilarEntity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.disambiguation.FindSimilarEntities(ctx, name, limit)
}

// Recommendations returns the Recommendations handler for file recommendations
func (m *Manager) Recommendations() *Recommendations {
	return m.recommendations
}

// RecommendRelated recommends files related to the given file
func (m *Manager) RecommendRelated(ctx context.Context, filePath string, limit int) ([]Recommendation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.recommendations.RecommendRelated(ctx, filePath, limit)
}

// RecommendByContext recommends files based on multiple context files
func (m *Manager) RecommendByContext(ctx context.Context, contextFiles []string, limit int) ([]Recommendation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.recommendations.RecommendByContext(ctx, contextFiles, limit)
}

// TopConnectedFiles returns the most connected files in the graph
func (m *Manager) TopConnectedFiles(ctx context.Context, limit int) ([]ConnectedFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.recommendations.TopConnectedFiles(ctx, limit)
}

// Clusters returns the ClusterDetection handler for cluster operations
func (m *Manager) Clusters() *ClusterDetection {
	return m.clusters
}

// DetectTopicClusters finds clusters based on shared topics
func (m *Manager) DetectTopicClusters(ctx context.Context, minSize int) ([]Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.clusters.DetectTopicClusters(ctx, minSize)
}

// DetectEntityClusters finds clusters based on shared entities
func (m *Manager) DetectEntityClusters(ctx context.Context, minSize int) ([]Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.clusters.DetectEntityClusters(ctx, minSize)
}

// DetectTagClusters finds clusters based on shared tags
func (m *Manager) DetectTagClusters(ctx context.Context, minSize int) ([]Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.clusters.DetectTagClusters(ctx, minSize)
}

// GetClusterSummary returns high-level statistics about knowledge clusters
func (m *Manager) GetClusterSummary(ctx context.Context) (*ClusterSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.clusters.GetClusterSummary(ctx)
}

// FindFileClusterMembership returns which clusters a file belongs to
func (m *Manager) FindFileClusterMembership(ctx context.Context, filePath string) (*FileMembership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.clusters.FindFileClusterMembership(ctx, filePath)
}

// GapAnalysis returns the GapAnalysis handler for gap detection
func (m *Manager) GapAnalysis() *GapAnalysis {
	return m.gapAnalysis
}

// AnalyzeGaps performs a complete gap analysis
func (m *Manager) AnalyzeGaps(ctx context.Context) (*GapReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.gapAnalysis.AnalyzeGaps(ctx)
}

// GetCoverageStats returns knowledge coverage statistics
func (m *Manager) GetCoverageStats(ctx context.Context) (*CoverageStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.gapAnalysis.GetCoverageStats(ctx)
}

// FindIsolatedFiles finds files with no semantic connections
func (m *Manager) FindIsolatedFiles(ctx context.Context) ([]Gap, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.gapAnalysis.FindIsolatedFiles(ctx)
}

// FindMissingConnections finds files that might need to be connected
func (m *Manager) FindMissingConnections(ctx context.Context, limit int) ([]MissingConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.gapAnalysis.FindMissingConnections(ctx, limit)
}

// Temporal returns the TemporalTracking handler for temporal analysis
func (m *Manager) Temporal() *TemporalTracking {
	return m.temporal
}

// GetRecentModifications returns files modified within a time window
func (m *Manager) GetRecentModifications(ctx context.Context, since time.Time, limit int) ([]ModificationEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.temporal.GetRecentModifications(ctx, since, limit)
}

// FindCoModifiedFiles finds files that were modified within the same time window
func (m *Manager) FindCoModifiedFiles(ctx context.Context, windowMinutes int, limit int) ([]CoModifiedPair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.temporal.FindCoModifiedFiles(ctx, windowMinutes, limit)
}

// GetModificationTimeline returns a timeline of modifications grouped by day
func (m *Manager) GetModificationTimeline(ctx context.Context, days int) ([]DayActivity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.temporal.GetModificationTimeline(ctx, days)
}

// GetFileModificationHistory returns the modification pattern for a specific file
func (m *Manager) GetFileModificationHistory(ctx context.Context, filePath string) (*FileHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.temporal.GetFileModificationHistory(ctx, filePath)
}

// GetActivityHotspots finds files or areas with high modification activity
func (m *Manager) GetActivityHotspots(ctx context.Context, days int, limit int) ([]ActivityHotspot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.temporal.GetActivityHotspots(ctx, days, limit)
}

// GetStaleFiles finds files that haven't been modified in a long time
func (m *Manager) GetStaleFiles(ctx context.Context, staleDays int, limit int) ([]StaleFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.temporal.GetStaleFiles(ctx, staleDays, limit)
}

// GetTemporalStats returns overall temporal statistics
func (m *Manager) GetTemporalStats(ctx context.Context) (*TemporalStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected {
		return nil, fmt.Errorf("graph manager not connected")
	}

	return m.temporal.GetTemporalStats(ctx)
}
