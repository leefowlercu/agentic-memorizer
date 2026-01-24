// Package registry provides SQLite-based storage for remembered paths and file state.
// This package wraps the internal/storage package and provides backward compatibility
// with existing code that uses the registry types.
package registry

import (
	"github.com/leefowlercu/agentic-memorizer/internal/storage"
)

// Re-export types from storage package for backwards compatibility.
// New code should prefer using the storage package directly.

// RememberedPath represents a directory that has been registered for tracking.
type RememberedPath = storage.RememberedPath

// PathConfig contains configuration for a remembered path.
type PathConfig = storage.PathConfig

// FileState tracks the state of a file for incremental processing.
type FileState = storage.FileState

// PathStatus represents the health status of a remembered path.
type PathStatus = storage.PathStatus

// Path status constants.
const (
	PathStatusOK      = storage.PathStatusOK
	PathStatusMissing = storage.PathStatusMissing
	PathStatusDenied  = storage.PathStatusDenied
	PathStatusError   = storage.PathStatusError
)
