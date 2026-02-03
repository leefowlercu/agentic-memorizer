package registry

import (
	"context"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/storage"
)

// ErrPathNotFound is returned when a path is not found in the registry.
var ErrPathNotFound = storage.ErrPathNotFound

// ErrPathExists is returned when attempting to add a path that already exists.
var ErrPathExists = storage.ErrPathExists

// Registry manages remembered paths and file state in SQLite.
type Registry interface {
	// Path management
	AddPath(ctx context.Context, path string, config *PathConfig) error
	RemovePath(ctx context.Context, path string) error
	GetPath(ctx context.Context, path string) (*RememberedPath, error)
	ListPaths(ctx context.Context) ([]RememberedPath, error)
	UpdatePathConfig(ctx context.Context, path string, config *PathConfig) error
	UpdatePathLastWalk(ctx context.Context, path string, lastWalk time.Time) error

	// Path resolution
	FindContainingPath(ctx context.Context, filePath string) (*RememberedPath, error)
	GetEffectiveConfig(ctx context.Context, filePath string) (*PathConfig, error)

	// File state management
	GetFileState(ctx context.Context, path string) (*FileState, error)
	UpdateFileState(ctx context.Context, state *FileState) error
	DeleteFileState(ctx context.Context, path string) error
	ListFileStates(ctx context.Context, parentPath string) ([]FileState, error)
	DeleteFileStatesForPath(ctx context.Context, parentPath string) error
	CountFileStates(ctx context.Context, parentPath string) (int, error)
	CountAnalyzedFiles(ctx context.Context, parentPath string) (int, error)

	// Discovery state management
	UpdateDiscoveryState(ctx context.Context, path string, contentHash string, size int64, modTime time.Time) error
	DeleteDiscoveryState(ctx context.Context, path string) error
	DeleteDiscoveryStatesForPath(ctx context.Context, parentPath string) error
	ListDiscoveryStates(ctx context.Context, parentPath string) ([]FileDiscovery, error)
	CountDiscoveredFiles(ctx context.Context, parentPath string) (int, error)

	// Granular analysis state updates
	UpdateMetadataState(ctx context.Context, path string, contentHash string, metadataHash string, size int64, modTime time.Time) error
	UpdateSemanticState(ctx context.Context, path string, analysisVersion string, err error) error
	UpdateEmbeddingsState(ctx context.Context, path string, err error) error
	ClearAnalysisState(ctx context.Context, path string) error

	// Query methods for analysis scheduling
	ListFilesNeedingMetadata(ctx context.Context, parentPath string) ([]FileState, error)
	ListFilesNeedingSemantic(ctx context.Context, parentPath string, maxRetries int) ([]FileState, error)
	ListFilesNeedingEmbeddings(ctx context.Context, parentPath string, maxRetries int) ([]FileState, error)

	// Path health checking
	CheckPathHealth(ctx context.Context) ([]PathStatus, error)
	ValidateAndCleanPaths(ctx context.Context) ([]string, error)

	// Lifecycle
	Close() error
}

// SQLiteRegistry is the SQLite implementation of Registry.
// It wraps the consolidated storage package.
type SQLiteRegistry struct {
	storage *storage.Storage
}

// Open creates a new SQLiteRegistry with the given database path.
// This function maintains backward compatibility with existing code.
func Open(ctx context.Context, dbPath string) (*SQLiteRegistry, error) {
	s, err := storage.Open(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	return &SQLiteRegistry{storage: s}, nil
}

// NewFromStorage creates a new SQLiteRegistry from an existing Storage instance.
// This allows sharing the database connection with other components.
func NewFromStorage(s *storage.Storage) *SQLiteRegistry {
	return &SQLiteRegistry{storage: s}
}

// Storage returns the underlying storage instance.
func (r *SQLiteRegistry) Storage() *storage.Storage {
	return r.storage
}

// Close closes the database connection.
func (r *SQLiteRegistry) Close() error {
	return r.storage.Close()
}

// AddPath adds a new remembered path to the registry.
func (r *SQLiteRegistry) AddPath(ctx context.Context, path string, config *PathConfig) error {
	return r.storage.AddPath(ctx, path, config)
}

// RemovePath removes a remembered path from the registry.
func (r *SQLiteRegistry) RemovePath(ctx context.Context, path string) error {
	return r.storage.RemovePath(ctx, path)
}

// GetPath retrieves a remembered path by its path string.
func (r *SQLiteRegistry) GetPath(ctx context.Context, path string) (*RememberedPath, error) {
	return r.storage.GetPath(ctx, path)
}

// ListPaths returns all remembered paths.
func (r *SQLiteRegistry) ListPaths(ctx context.Context) ([]RememberedPath, error) {
	return r.storage.ListPaths(ctx)
}

// UpdatePathConfig updates the configuration for a remembered path.
func (r *SQLiteRegistry) UpdatePathConfig(ctx context.Context, path string, config *PathConfig) error {
	return r.storage.UpdatePathConfig(ctx, path, config)
}

// UpdatePathLastWalk updates the last walk timestamp for a remembered path.
func (r *SQLiteRegistry) UpdatePathLastWalk(ctx context.Context, path string, lastWalk time.Time) error {
	return r.storage.UpdatePathLastWalk(ctx, path, lastWalk)
}

// FindContainingPath finds the remembered path that contains the given file path.
func (r *SQLiteRegistry) FindContainingPath(ctx context.Context, filePath string) (*RememberedPath, error) {
	return r.storage.FindContainingPath(ctx, filePath)
}

// GetEffectiveConfig returns the effective configuration for a file path.
func (r *SQLiteRegistry) GetEffectiveConfig(ctx context.Context, filePath string) (*PathConfig, error) {
	return r.storage.GetEffectiveConfig(ctx, filePath)
}

// GetFileState retrieves the file state for a given path.
func (r *SQLiteRegistry) GetFileState(ctx context.Context, path string) (*FileState, error) {
	return r.storage.GetFileState(ctx, path)
}

// UpdateFileState creates or updates the file state for a given path.
func (r *SQLiteRegistry) UpdateFileState(ctx context.Context, state *FileState) error {
	return r.storage.UpdateFileState(ctx, state)
}

// DeleteFileState removes the file state for a given path.
func (r *SQLiteRegistry) DeleteFileState(ctx context.Context, path string) error {
	return r.storage.DeleteFileState(ctx, path)
}

// ListFileStates returns all file states under a given parent path.
func (r *SQLiteRegistry) ListFileStates(ctx context.Context, parentPath string) ([]FileState, error) {
	return r.storage.ListFileStates(ctx, parentPath)
}

// DeleteFileStatesForPath removes all file states under a given parent path.
func (r *SQLiteRegistry) DeleteFileStatesForPath(ctx context.Context, parentPath string) error {
	return r.storage.DeleteFileStatesForPath(ctx, parentPath)
}

// CountFileStates returns the count of discovered files under a parent path.
func (r *SQLiteRegistry) CountFileStates(ctx context.Context, parentPath string) (int, error) {
	return r.storage.CountFileStates(ctx, parentPath)
}

// CountDiscoveredFiles returns the count of discovered files under a parent path.
func (r *SQLiteRegistry) CountDiscoveredFiles(ctx context.Context, parentPath string) (int, error) {
	return r.storage.CountDiscoveredFiles(ctx, parentPath)
}

// CountAnalyzedFiles returns the count of files with completed semantic analysis under a parent path.
func (r *SQLiteRegistry) CountAnalyzedFiles(ctx context.Context, parentPath string) (int, error) {
	return r.storage.CountAnalyzedFiles(ctx, parentPath)
}

// UpdateMetadataState updates the metadata tracking fields for a file.
func (r *SQLiteRegistry) UpdateMetadataState(ctx context.Context, path string, contentHash string, metadataHash string, size int64, modTime time.Time) error {
	return r.storage.UpdateMetadataState(ctx, path, contentHash, metadataHash, size, modTime)
}

// UpdateSemanticState updates the semantic analysis tracking fields for a file.
func (r *SQLiteRegistry) UpdateSemanticState(ctx context.Context, path string, analysisVersion string, err error) error {
	return r.storage.UpdateSemanticState(ctx, path, analysisVersion, err)
}

// UpdateEmbeddingsState updates the embeddings generation tracking fields for a file.
func (r *SQLiteRegistry) UpdateEmbeddingsState(ctx context.Context, path string, err error) error {
	return r.storage.UpdateEmbeddingsState(ctx, path, err)
}

// UpdateDiscoveryState updates the discovery state for a file.
func (r *SQLiteRegistry) UpdateDiscoveryState(ctx context.Context, path string, contentHash string, size int64, modTime time.Time) error {
	return r.storage.UpdateDiscoveryState(ctx, path, contentHash, size, modTime)
}

// DeleteDiscoveryState removes a discovery record for a path.
func (r *SQLiteRegistry) DeleteDiscoveryState(ctx context.Context, path string) error {
	return r.storage.DeleteDiscoveryState(ctx, path)
}

// DeleteDiscoveryStatesForPath removes discovery records under a parent path.
func (r *SQLiteRegistry) DeleteDiscoveryStatesForPath(ctx context.Context, parentPath string) error {
	return r.storage.DeleteDiscoveryStatesForPath(ctx, parentPath)
}

// ListDiscoveryStates returns discovery records under a parent path.
func (r *SQLiteRegistry) ListDiscoveryStates(ctx context.Context, parentPath string) ([]FileDiscovery, error) {
	return r.storage.ListDiscoveryStates(ctx, parentPath)
}

// ClearAnalysisState clears all analysis state for a file, forcing reanalysis.
func (r *SQLiteRegistry) ClearAnalysisState(ctx context.Context, path string) error {
	return r.storage.ClearAnalysisState(ctx, path)
}

// ListFilesNeedingMetadata returns files that have not had metadata computed yet.
func (r *SQLiteRegistry) ListFilesNeedingMetadata(ctx context.Context, parentPath string) ([]FileState, error) {
	return r.storage.ListFilesNeedingMetadata(ctx, parentPath)
}

// ListFilesNeedingSemantic returns files that need semantic analysis.
func (r *SQLiteRegistry) ListFilesNeedingSemantic(ctx context.Context, parentPath string, maxRetries int) ([]FileState, error) {
	return r.storage.ListFilesNeedingSemantic(ctx, parentPath, maxRetries)
}

// ListFilesNeedingEmbeddings returns files that need embeddings generation.
func (r *SQLiteRegistry) ListFilesNeedingEmbeddings(ctx context.Context, parentPath string, maxRetries int) ([]FileState, error) {
	return r.storage.ListFilesNeedingEmbeddings(ctx, parentPath, maxRetries)
}

// CheckPathHealth validates all remembered paths and returns their status.
func (r *SQLiteRegistry) CheckPathHealth(ctx context.Context) ([]PathStatus, error) {
	return r.storage.CheckPathHealth(ctx)
}

// ValidateAndCleanPaths checks all remembered paths and removes those that no longer exist.
func (r *SQLiteRegistry) ValidateAndCleanPaths(ctx context.Context) ([]string, error) {
	return r.storage.ValidateAndCleanPaths(ctx)
}
