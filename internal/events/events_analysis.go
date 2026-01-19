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

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
