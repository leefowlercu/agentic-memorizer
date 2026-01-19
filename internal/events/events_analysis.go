package events

import "time"

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

// GraphFatalEvent contains data for fatal graph errors.
type GraphFatalEvent struct {
	// Error contains the error message.
	Error string
}

// GraphConnectionEvent contains data for graph connection events.
type GraphConnectionEvent struct {
	// Connected indicates if the connection was established (true) or lost (false).
	Connected bool

	// Endpoint is the graph database endpoint.
	Endpoint string

	// Error contains the error message if connection failed.
	Error string
}

// GraphBackpressureEvent contains data for graph write queue pressure events.
type GraphBackpressureEvent struct {
	// QueueDepth is the current number of items in the write queue.
	QueueDepth int

	// QueueCapacity is the maximum write queue size.
	QueueCapacity int
}

// QueueDegradationEvent contains data for queue degradation mode changes.
type QueueDegradationEvent struct {
	// PreviousMode is the previous degradation mode ("full", "no_embed", "metadata").
	PreviousMode string

	// CurrentMode is the new degradation mode.
	CurrentMode string

	// Reason describes what triggered the transition.
	Reason string

	// QueueDepth is the current queue depth at the time of transition.
	QueueDepth int
}

// IngestDecisionEvent contains data for files that were skipped or received limited analysis.
type IngestDecisionEvent struct {
	// Path is the absolute path to the file.
	Path string

	// Decision indicates the analysis decision ("metadata_only", "skipped").
	Decision string

	// Reason explains why the decision was made.
	Reason string
}

// NewAnalysisComplete creates an AnalysisComplete event.
func NewAnalysisComplete(path, contentHash string, analysisType AnalysisType, duration time.Duration) Event {
	return NewEvent(AnalysisComplete, &AnalysisEvent{
		Path:         path,
		ContentHash:  contentHash,
		AnalysisType: analysisType,
		Duration:     duration,
	})
}

// NewAnalysisFailed creates an AnalysisFailed event.
func NewAnalysisFailed(path string, err error) Event {
	return NewEvent(AnalysisFailed, &AnalysisEvent{
		Path:  path,
		Error: errorString(err),
	})
}

// NewSemanticAnalysisFailed creates a SemanticAnalysisFailed event.
func NewSemanticAnalysisFailed(path string, err error) Event {
	return NewEvent(SemanticAnalysisFailed, &AnalysisEvent{
		Path:         path,
		AnalysisType: AnalysisSemantic,
		Error:        errorString(err),
	})
}

// NewEmbeddingsGenerationFailed creates an EmbeddingsGenerationFailed event.
func NewEmbeddingsGenerationFailed(path string, err error) Event {
	return NewEvent(EmbeddingsGenerationFailed, &AnalysisEvent{
		Path:         path,
		AnalysisType: AnalysisEmbeddings,
		Error:        errorString(err),
	})
}

// NewGraphPersistenceFailed creates a GraphPersistenceFailed event.
func NewGraphPersistenceFailed(path, operation string, err error, retries int) Event {
	return NewEvent(GraphPersistenceFailed, &GraphEvent{
		Path:      path,
		Operation: operation,
		Error:     errorString(err),
		Retries:   retries,
	})
}

// NewGraphFatal creates a GraphFatal event.
func NewGraphFatal(err error) Event {
	return NewEvent(GraphFatal, &GraphFatalEvent{
		Error: errorString(err),
	})
}

// NewGraphConnected creates a GraphConnected event.
func NewGraphConnected(endpoint string) Event {
	return NewEvent(GraphConnected, &GraphConnectionEvent{
		Connected: true,
		Endpoint:  endpoint,
	})
}

// NewGraphDisconnected creates a GraphDisconnected event.
func NewGraphDisconnected(endpoint string, err error) Event {
	return NewEvent(GraphDisconnected, &GraphConnectionEvent{
		Connected: false,
		Endpoint:  endpoint,
		Error:     errorString(err),
	})
}

// NewGraphWriteQueueFull creates a GraphWriteQueueFull event.
func NewGraphWriteQueueFull(queueDepth, queueCapacity int) Event {
	return NewEvent(GraphWriteQueueFull, &GraphBackpressureEvent{
		QueueDepth:    queueDepth,
		QueueCapacity: queueCapacity,
	})
}

// NewQueueDegradationChanged creates a QueueDegradationChanged event.
func NewQueueDegradationChanged(previousMode, currentMode, reason string, queueDepth int) Event {
	return NewEvent(QueueDegradationChanged, &QueueDegradationEvent{
		PreviousMode: previousMode,
		CurrentMode:  currentMode,
		Reason:       reason,
		QueueDepth:   queueDepth,
	})
}

// NewAnalysisSkipped creates an AnalysisSkipped event.
func NewAnalysisSkipped(path, decision, reason string) Event {
	return NewEvent(AnalysisSkipped, &IngestDecisionEvent{
		Path:     path,
		Decision: decision,
		Reason:   reason,
	})
}

// NewAnalysisSemanticComplete creates an AnalysisSemanticComplete event.
func NewAnalysisSemanticComplete(path, contentHash string, duration time.Duration) Event {
	return NewEvent(AnalysisSemanticComplete, &AnalysisEvent{
		Path:         path,
		ContentHash:  contentHash,
		AnalysisType: AnalysisSemantic,
		Duration:     duration,
	})
}

// NewAnalysisEmbeddingsComplete creates an AnalysisEmbeddingsComplete event.
func NewAnalysisEmbeddingsComplete(path, contentHash string, duration time.Duration) Event {
	return NewEvent(AnalysisEmbeddingsComplete, &AnalysisEvent{
		Path:         path,
		ContentHash:  contentHash,
		AnalysisType: AnalysisEmbeddings,
		Duration:     duration,
	})
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
