// Package events provides an in-process pub/sub event bus for cross-component
// communication within the memorizer daemon.
package events

import (
	"time"
)

// EventType identifies the type of event being published.
type EventType string

const (
	// FileDiscovered is published when a new file is found during a walk.
	FileDiscovered EventType = "file.discovered"

	// FileChanged is published when an existing file is modified.
	FileChanged EventType = "file.changed"

	// FileDeleted is published when a file is removed.
	FileDeleted EventType = "file.deleted"

	// AnalysisComplete is published when analysis finishes for a file.
	AnalysisComplete EventType = "analysis.complete"

	// AnalysisFailed is published when analysis fails for a file.
	AnalysisFailed EventType = "analysis.failed"

	// SemanticAnalysisFailed is published when semantic analysis fails for a file.
	SemanticAnalysisFailed EventType = "analysis.semantic_failed"

	// EmbeddingsGenerationFailed is published when embeddings generation fails for a file.
	EmbeddingsGenerationFailed EventType = "analysis.embeddings_failed"

	// GraphPersistenceFailed is published when writing analysis results to the graph fails.
	GraphPersistenceFailed EventType = "graph.persistence_failed"

	// ConfigReloaded is published when configuration is successfully reloaded.
	ConfigReloaded EventType = "config.reloaded"

	// ConfigReloadFailed is published when configuration reload fails.
	ConfigReloadFailed EventType = "config.reload_failed"
)

// Event represents a published event in the system.
type Event struct {
	// Type identifies the event type.
	Type EventType

	// Timestamp is when the event was created.
	Timestamp time.Time

	// Payload contains event-specific data.
	Payload any
}

// NewEvent creates a new event with the given type and payload.
func NewEvent(eventType EventType, payload any) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

// FileEvent contains data for file-related events (discovered, changed, deleted).
type FileEvent struct {
	// Path is the absolute path to the file.
	Path string

	// ContentHash is the SHA256 hash of the file content (empty for deleted files).
	ContentHash string

	// Size is the file size in bytes (0 for deleted files).
	Size int64

	// ModTime is the file modification time (zero for deleted files).
	ModTime time.Time

	// IsNew indicates if this is a newly discovered file (for FileDiscovered events).
	IsNew bool
}

// AnalysisEvent contains data for analysis-related events.
type AnalysisEvent struct {
	// Path is the absolute path to the analyzed file.
	Path string

	// ContentHash is the SHA256 hash of the analyzed content.
	ContentHash string

	// AnalysisType indicates what type of analysis was performed.
	AnalysisType AnalysisType

	// Duration is how long the analysis took.
	Duration time.Duration

	// Error contains the error message if analysis failed (for AnalysisFailed events).
	Error string
}

// GraphEvent contains data for graph-related events.
type GraphEvent struct {
	// Path is the absolute path to the file that couldn't be persisted.
	Path string

	// Operation describes what graph operation failed (e.g., "upsert_file", "set_tags").
	Operation string

	// Error contains the error message.
	Error string

	// Retries is the number of retry attempts made before giving up.
	Retries int
}

// AnalysisType identifies the type of analysis performed.
type AnalysisType string

const (
	// AnalysisMetadata indicates metadata-only analysis was performed.
	AnalysisMetadata AnalysisType = "metadata"

	// AnalysisSemantic indicates semantic analysis was performed.
	AnalysisSemantic AnalysisType = "semantic"

	// AnalysisEmbeddings indicates embeddings generation was performed.
	AnalysisEmbeddings AnalysisType = "embeddings"

	// AnalysisFull indicates full analysis (metadata + semantic + embeddings) was performed.
	AnalysisFull AnalysisType = "full"
)

// ConfigReloadEvent contains data for config reload events.
type ConfigReloadEvent struct {
	// ChangedSections lists which config sections were modified.
	ChangedSections []string

	// ReloadableChanges indicates if all changes are hot-reloadable.
	ReloadableChanges bool

	// Error contains the error message if reload failed (for ConfigReloadFailed events).
	Error string
}

// EventHandler is a function that processes events.
type EventHandler func(event Event)
