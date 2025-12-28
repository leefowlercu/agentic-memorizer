package graph

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Fact constraints
const (
	MinFactContentLength = 10
	MaxFactContentLength = 500
	MaxTotalFacts        = 50
)

// FactNode represents a Fact node in the graph
type FactNode struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Source    string    `json:"source"`
}

// Facts handles Fact node CRUD operations
type Facts struct {
	client *Client
	logger *slog.Logger
}

// NewFacts creates a new Facts handler
func NewFacts(client *Client, logger *slog.Logger) *Facts {
	if logger == nil {
		logger = slog.Default()
	}
	return &Facts{
		client: client,
		logger: logger.With("component", "graph-facts"),
	}
}

// Create creates a new Fact node with a generated UUID
func (f *Facts) Create(ctx context.Context, content, source string) (*FactNode, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		CREATE (fact:Fact {
			id: $id,
			content: $content,
			created_at: $created_at,
			source: $source
		})
		RETURN fact.id, fact.content, fact.created_at, fact.source
	`

	params := map[string]any{
		"id":         id,
		"content":    content,
		"created_at": now.Format(time.RFC3339),
		"source":     source,
	}

	result, err := f.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create fact; %w", err)
	}

	if !result.Next() {
		return nil, fmt.Errorf("failed to create fact; no result returned")
	}

	fact := &FactNode{
		ID:        id,
		Content:   content,
		CreatedAt: now,
		Source:    source,
	}

	f.logger.Debug("created fact", "id", id)
	return fact, nil
}

// Update updates an existing Fact node's content
func (f *Facts) Update(ctx context.Context, id, content string) (*FactNode, error) {
	now := time.Now()

	query := `
		MATCH (fact:Fact {id: $id})
		SET fact.content = $content,
		    fact.updated_at = $updated_at
		RETURN fact.id, fact.content, fact.created_at, fact.updated_at, fact.source
	`

	params := map[string]any{
		"id":         id,
		"content":    content,
		"updated_at": now.Format(time.RFC3339),
	}

	result, err := f.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update fact; %w", err)
	}

	if !result.Next() {
		return nil, fmt.Errorf("fact with id %q not found", id)
	}

	record := result.Record()
	fact := &FactNode{
		ID:      record.GetString(0, ""),
		Content: record.GetString(1, ""),
		Source:  record.GetString(4, ""),
	}

	// Parse created_at
	if createdStr := record.GetString(2, ""); createdStr != "" {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			fact.CreatedAt = t
		}
	}

	// Parse updated_at
	if updatedStr := record.GetString(3, ""); updatedStr != "" {
		if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			fact.UpdatedAt = t
		}
	}

	f.logger.Debug("updated fact", "id", id)
	return fact, nil
}

// GetByID retrieves a Fact node by its ID
func (f *Facts) GetByID(ctx context.Context, id string) (*FactNode, error) {
	query := `
		MATCH (fact:Fact {id: $id})
		RETURN fact.id, fact.content, fact.created_at, fact.updated_at, fact.source
	`

	result, err := f.client.Query(ctx, query, map[string]any{"id": id})
	if err != nil {
		return nil, fmt.Errorf("failed to get fact; %w", err)
	}

	if !result.Next() {
		return nil, nil // Not found
	}

	record := result.Record()
	fact := &FactNode{
		ID:      record.GetString(0, ""),
		Content: record.GetString(1, ""),
		Source:  record.GetString(4, ""),
	}

	// Parse created_at
	if createdStr := record.GetString(2, ""); createdStr != "" {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			fact.CreatedAt = t
		}
	}

	// Parse updated_at
	if updatedStr := record.GetString(3, ""); updatedStr != "" {
		if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			fact.UpdatedAt = t
		}
	}

	return fact, nil
}

// GetByContent retrieves a Fact node by its content (for duplicate detection)
func (f *Facts) GetByContent(ctx context.Context, content string) (*FactNode, error) {
	query := `
		MATCH (fact:Fact {content: $content})
		RETURN fact.id, fact.content, fact.created_at, fact.updated_at, fact.source
	`

	result, err := f.client.Query(ctx, query, map[string]any{"content": content})
	if err != nil {
		return nil, fmt.Errorf("failed to get fact by content; %w", err)
	}

	if !result.Next() {
		return nil, nil // Not found
	}

	record := result.Record()
	fact := &FactNode{
		ID:      record.GetString(0, ""),
		Content: record.GetString(1, ""),
		Source:  record.GetString(4, ""),
	}

	// Parse created_at
	if createdStr := record.GetString(2, ""); createdStr != "" {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			fact.CreatedAt = t
		}
	}

	// Parse updated_at
	if updatedStr := record.GetString(3, ""); updatedStr != "" {
		if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			fact.UpdatedAt = t
		}
	}

	return fact, nil
}

// List returns all Fact nodes ordered by creation time (newest first)
func (f *Facts) List(ctx context.Context) ([]FactNode, error) {
	query := `
		MATCH (fact:Fact)
		RETURN fact.id, fact.content, fact.created_at, fact.updated_at, fact.source
		ORDER BY fact.created_at DESC
	`

	result, err := f.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list facts; %w", err)
	}

	var facts []FactNode
	for result.Next() {
		record := result.Record()
		fact := FactNode{
			ID:      record.GetString(0, ""),
			Content: record.GetString(1, ""),
			Source:  record.GetString(4, ""),
		}

		// Parse created_at
		if createdStr := record.GetString(2, ""); createdStr != "" {
			if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
				fact.CreatedAt = t
			}
		}

		// Parse updated_at
		if updatedStr := record.GetString(3, ""); updatedStr != "" {
			if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
				fact.UpdatedAt = t
			}
		}

		facts = append(facts, fact)
	}

	return facts, nil
}

// Delete removes a Fact node by its ID
func (f *Facts) Delete(ctx context.Context, id string) error {
	query := `
		MATCH (fact:Fact {id: $id})
		DELETE fact
	`

	_, err := f.client.Query(ctx, query, map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("failed to delete fact; %w", err)
	}

	f.logger.Debug("deleted fact", "id", id)
	return nil
}

// Count returns the total number of Fact nodes
func (f *Facts) Count(ctx context.Context) (int64, error) {
	query := `MATCH (fact:Fact) RETURN count(fact) as count`

	result, err := f.client.Query(ctx, query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count facts; %w", err)
	}

	if result.Next() {
		return result.Record().GetInt64(0, 0), nil
	}

	return 0, nil
}

// Exists checks if a fact exists by ID
func (f *Facts) Exists(ctx context.Context, id string) (bool, error) {
	query := `MATCH (fact:Fact {id: $id}) RETURN count(fact) > 0 as exists`

	result, err := f.client.Query(ctx, query, map[string]any{"id": id})
	if err != nil {
		return false, fmt.Errorf("failed to check fact existence; %w", err)
	}

	if result.Next() {
		val := result.Record().GetByIndex(0)
		if b, ok := val.(bool); ok {
			return b, nil
		}
	}

	return false, nil
}
