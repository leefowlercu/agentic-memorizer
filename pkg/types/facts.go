package types

import "time"

// ============================================================================
// Facts Types
// These types represent user-defined facts that are stored in the knowledge
// graph and injected into AI agent contexts via hooks.
// ============================================================================

// Fact represents a single user-defined fact in the knowledge graph
type Fact struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Source    string    `json:"source"`
}

// FactsIndex represents the output format for facts
// This structure is used when outputting facts via CLI or hooks
type FactsIndex struct {
	Generated time.Time `json:"generated"`
	Facts     []Fact    `json:"facts"`
	Stats     FactStats `json:"stats"`
}

// FactStats provides summary statistics for facts
type FactStats struct {
	TotalFacts int `json:"total_facts"`
	MaxFacts   int `json:"max_facts"`
}
