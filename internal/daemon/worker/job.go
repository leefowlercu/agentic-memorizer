package worker

import (
	"os"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Job represents a file processing job
type Job struct {
	Path     string
	Info     os.FileInfo
	Priority int // Higher priority = process first
}

// JobResult represents the result of processing a job
type JobResult struct {
	Entry     types.IndexEntry
	Embedding []float32 // Optional embedding for the file's summary
	Error     error
}
