package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrPathNotFound is returned when a path is not found in the registry.
var ErrPathNotFound = errors.New("path not found")

// ErrPathExists is returned when attempting to add a path that already exists.
var ErrPathExists = errors.New("path already exists")

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
type SQLiteRegistry struct {
	db *sql.DB
}

// Open creates a new SQLiteRegistry with the given database path.
func Open(ctx context.Context, dbPath string) (*SQLiteRegistry, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory; %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database; %w", err)
	}

	// Enable foreign keys and WAL mode for better concurrency
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys; %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode; %w", err)
	}

	// Run migrations
	if err := Migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations; %w", err)
	}

	return &SQLiteRegistry{db: db}, nil
}

// Close closes the database connection.
func (r *SQLiteRegistry) Close() error {
	return r.db.Close()
}

// AddPath adds a new remembered path to the registry.
func (r *SQLiteRegistry) AddPath(ctx context.Context, path string, config *PathConfig) error {
	path = filepath.Clean(path)

	var configJSON *string
	if config != nil {
		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config; %w", err)
		}
		s := string(data)
		configJSON = &s
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO remembered_paths (path, config_json, created_at, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		path, configJSON,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrPathExists
		}
		return fmt.Errorf("failed to add path; %w", err)
	}

	return nil
}

// RemovePath removes a remembered path from the registry.
func (r *SQLiteRegistry) RemovePath(ctx context.Context, path string) error {
	path = filepath.Clean(path)

	result, err := r.db.ExecContext(ctx,
		"DELETE FROM remembered_paths WHERE path = ?",
		path,
	)
	if err != nil {
		return fmt.Errorf("failed to remove path; %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected; %w", err)
	}
	if rows == 0 {
		return ErrPathNotFound
	}

	return nil
}

// GetPath retrieves a remembered path by its path string.
func (r *SQLiteRegistry) GetPath(ctx context.Context, path string) (*RememberedPath, error) {
	path = filepath.Clean(path)

	row := r.db.QueryRowContext(ctx,
		`SELECT id, path, config_json, last_walk_at, created_at, updated_at
		 FROM remembered_paths WHERE path = ?`,
		path,
	)

	return scanRememberedPath(row)
}

// ListPaths returns all remembered paths.
func (r *SQLiteRegistry) ListPaths(ctx context.Context) ([]RememberedPath, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, path, config_json, last_walk_at, created_at, updated_at
		 FROM remembered_paths ORDER BY path`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list paths; %w", err)
	}
	defer rows.Close()

	var paths []RememberedPath
	for rows.Next() {
		p, err := scanRememberedPathRows(rows)
		if err != nil {
			return nil, err
		}
		paths = append(paths, *p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating paths; %w", err)
	}

	return paths, nil
}

// UpdatePathConfig updates the configuration for a remembered path.
func (r *SQLiteRegistry) UpdatePathConfig(ctx context.Context, path string, config *PathConfig) error {
	path = filepath.Clean(path)

	var configJSON *string
	if config != nil {
		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config; %w", err)
		}
		s := string(data)
		configJSON = &s
	}

	result, err := r.db.ExecContext(ctx,
		`UPDATE remembered_paths SET config_json = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE path = ?`,
		configJSON, path,
	)
	if err != nil {
		return fmt.Errorf("failed to update path config; %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected; %w", err)
	}
	if rows == 0 {
		return ErrPathNotFound
	}

	return nil
}

// UpdatePathLastWalk updates the last walk timestamp for a remembered path.
func (r *SQLiteRegistry) UpdatePathLastWalk(ctx context.Context, path string, lastWalk time.Time) error {
	path = filepath.Clean(path)

	result, err := r.db.ExecContext(ctx,
		`UPDATE remembered_paths SET last_walk_at = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE path = ?`,
		lastWalk, path,
	)
	if err != nil {
		return fmt.Errorf("failed to update last walk; %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected; %w", err)
	}
	if rows == 0 {
		return ErrPathNotFound
	}

	return nil
}

// FindContainingPath finds the remembered path that contains the given file path.
// Returns the closest (deepest) remembered ancestor.
func (r *SQLiteRegistry) FindContainingPath(ctx context.Context, filePath string) (*RememberedPath, error) {
	filePath = filepath.Clean(filePath)

	// Get all remembered paths and find the closest ancestor
	paths, err := r.ListPaths(ctx)
	if err != nil {
		return nil, err
	}

	var closest *RememberedPath
	closestLen := 0

	for i := range paths {
		p := &paths[i]
		// Check if filePath is under this remembered path
		if strings.HasPrefix(filePath, p.Path+string(filepath.Separator)) || filePath == p.Path {
			if len(p.Path) > closestLen {
				closest = p
				closestLen = len(p.Path)
			}
		}
	}

	if closest == nil {
		return nil, ErrPathNotFound
	}

	return closest, nil
}

// GetEffectiveConfig returns the effective configuration for a file path.
func (r *SQLiteRegistry) GetEffectiveConfig(ctx context.Context, filePath string) (*PathConfig, error) {
	rp, err := r.FindContainingPath(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return rp.Config, nil
}

// GetFileState retrieves the file state for a given path.
func (r *SQLiteRegistry) GetFileState(ctx context.Context, path string) (*FileState, error) {
	path = filepath.Clean(path)

	row := r.db.QueryRowContext(ctx,
		`SELECT id, path, content_hash, metadata_hash, size, mod_time,
		        last_analyzed_at, analysis_version,
		        metadata_analyzed_at, semantic_analyzed_at, semantic_error, semantic_retry_count,
		        embeddings_analyzed_at, embeddings_error, embeddings_retry_count,
		        created_at, updated_at
		 FROM file_state WHERE path = ?`,
		path,
	)

	return scanFileState(row)
}

// UpdateFileState creates or updates the file state for a given path.
func (r *SQLiteRegistry) UpdateFileState(ctx context.Context, state *FileState) error {
	state.Path = filepath.Clean(state.Path)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO file_state (path, content_hash, metadata_hash, size, mod_time,
		                         last_analyzed_at, analysis_version,
		                         metadata_analyzed_at, semantic_analyzed_at, semantic_error, semantic_retry_count,
		                         embeddings_analyzed_at, embeddings_error, embeddings_retry_count,
		                         created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET
		   content_hash = excluded.content_hash,
		   metadata_hash = excluded.metadata_hash,
		   size = excluded.size,
		   mod_time = excluded.mod_time,
		   last_analyzed_at = excluded.last_analyzed_at,
		   analysis_version = excluded.analysis_version,
		   metadata_analyzed_at = excluded.metadata_analyzed_at,
		   semantic_analyzed_at = excluded.semantic_analyzed_at,
		   semantic_error = excluded.semantic_error,
		   semantic_retry_count = excluded.semantic_retry_count,
		   embeddings_analyzed_at = excluded.embeddings_analyzed_at,
		   embeddings_error = excluded.embeddings_error,
		   embeddings_retry_count = excluded.embeddings_retry_count,
		   updated_at = CURRENT_TIMESTAMP`,
		state.Path, state.ContentHash, state.MetadataHash, state.Size, state.ModTime,
		state.LastAnalyzedAt, state.AnalysisVersion,
		state.MetadataAnalyzedAt, state.SemanticAnalyzedAt, state.SemanticError, state.SemanticRetryCount,
		state.EmbeddingsAnalyzedAt, state.EmbeddingsError, state.EmbeddingsRetryCount,
	)
	if err != nil {
		return fmt.Errorf("failed to update file state; %w", err)
	}

	return nil
}

// DeleteFileState removes the file state for a given path.
func (r *SQLiteRegistry) DeleteFileState(ctx context.Context, path string) error {
	path = filepath.Clean(path)

	result, err := r.db.ExecContext(ctx,
		"DELETE FROM file_state WHERE path = ?",
		path,
	)
	if err != nil {
		return fmt.Errorf("failed to delete file state; %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected; %w", err)
	}
	if rows == 0 {
		return ErrPathNotFound
	}

	return nil
}

// ListFileStates returns all file states under a given parent path.
func (r *SQLiteRegistry) ListFileStates(ctx context.Context, parentPath string) ([]FileState, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, path, content_hash, metadata_hash, size, mod_time,
		        last_analyzed_at, analysis_version,
		        metadata_analyzed_at, semantic_analyzed_at, semantic_error, semantic_retry_count,
		        embeddings_analyzed_at, embeddings_error, embeddings_retry_count,
		        created_at, updated_at
		 FROM file_state
		 WHERE path LIKE ? OR path = ?
		 ORDER BY path`,
		prefix+"%", parentPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list file states; %w", err)
	}
	defer rows.Close()

	var states []FileState
	for rows.Next() {
		s, err := scanFileStateRows(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, *s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file states; %w", err)
	}

	return states, nil
}

// DeleteFileStatesForPath removes all file states under a given parent path.
func (r *SQLiteRegistry) DeleteFileStatesForPath(ctx context.Context, parentPath string) error {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	_, err := r.db.ExecContext(ctx,
		"DELETE FROM file_state WHERE path LIKE ? OR path = ?",
		prefix+"%", parentPath,
	)
	if err != nil {
		return fmt.Errorf("failed to delete file states; %w", err)
	}

	return nil
}

// UpdateMetadataState updates the metadata tracking fields for a file.
// This is called after computing content hash and file metadata.
func (r *SQLiteRegistry) UpdateMetadataState(ctx context.Context, path string, contentHash string, metadataHash string, size int64, modTime time.Time) error {
	path = filepath.Clean(path)
	now := time.Now()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO file_state (path, content_hash, metadata_hash, size, mod_time, metadata_analyzed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET
		   content_hash = excluded.content_hash,
		   metadata_hash = excluded.metadata_hash,
		   size = excluded.size,
		   mod_time = excluded.mod_time,
		   metadata_analyzed_at = excluded.metadata_analyzed_at,
		   updated_at = CURRENT_TIMESTAMP`,
		path, contentHash, metadataHash, size, modTime, now,
	)
	if err != nil {
		return fmt.Errorf("failed to update metadata state; %w", err)
	}

	return nil
}

// UpdateSemanticState updates the semantic analysis tracking fields for a file.
// Pass nil for err if analysis succeeded, otherwise pass the error.
func (r *SQLiteRegistry) UpdateSemanticState(ctx context.Context, path string, analysisVersion string, analysisErr error) error {
	path = filepath.Clean(path)
	now := time.Now()

	var errStr *string
	if analysisErr != nil {
		s := analysisErr.Error()
		errStr = &s
	}

	if analysisErr == nil {
		// Success: set timestamp, clear error and reset retry count
		_, err := r.db.ExecContext(ctx,
			`UPDATE file_state SET
			   semantic_analyzed_at = ?,
			   analysis_version = ?,
			   semantic_error = NULL,
			   semantic_retry_count = 0,
			   last_analyzed_at = ?,
			   updated_at = CURRENT_TIMESTAMP
			 WHERE path = ?`,
			now, analysisVersion, now, path,
		)
		if err != nil {
			return fmt.Errorf("failed to update semantic state; %w", err)
		}
	} else {
		// Failure: set error and increment retry count
		_, err := r.db.ExecContext(ctx,
			`UPDATE file_state SET
			   semantic_error = ?,
			   semantic_retry_count = semantic_retry_count + 1,
			   updated_at = CURRENT_TIMESTAMP
			 WHERE path = ?`,
			errStr, path,
		)
		if err != nil {
			return fmt.Errorf("failed to update semantic state; %w", err)
		}
	}

	return nil
}

// UpdateEmbeddingsState updates the embeddings generation tracking fields for a file.
// Pass nil for err if generation succeeded, otherwise pass the error.
func (r *SQLiteRegistry) UpdateEmbeddingsState(ctx context.Context, path string, embeddingsErr error) error {
	path = filepath.Clean(path)
	now := time.Now()

	var errStr *string
	if embeddingsErr != nil {
		s := embeddingsErr.Error()
		errStr = &s
	}

	if embeddingsErr == nil {
		// Success: set timestamp, clear error and reset retry count
		_, err := r.db.ExecContext(ctx,
			`UPDATE file_state SET
			   embeddings_analyzed_at = ?,
			   embeddings_error = NULL,
			   embeddings_retry_count = 0,
			   updated_at = CURRENT_TIMESTAMP
			 WHERE path = ?`,
			now, path,
		)
		if err != nil {
			return fmt.Errorf("failed to update embeddings state; %w", err)
		}
	} else {
		// Failure: set error and increment retry count
		_, err := r.db.ExecContext(ctx,
			`UPDATE file_state SET
			   embeddings_error = ?,
			   embeddings_retry_count = embeddings_retry_count + 1,
			   updated_at = CURRENT_TIMESTAMP
			 WHERE path = ?`,
			errStr, path,
		)
		if err != nil {
			return fmt.Errorf("failed to update embeddings state; %w", err)
		}
	}

	return nil
}

// ClearAnalysisState clears all analysis state for a file, forcing reanalysis.
// This is called when a file's content hash changes.
func (r *SQLiteRegistry) ClearAnalysisState(ctx context.Context, path string) error {
	path = filepath.Clean(path)

	_, err := r.db.ExecContext(ctx,
		`UPDATE file_state SET
		   last_analyzed_at = NULL,
		   analysis_version = NULL,
		   metadata_analyzed_at = NULL,
		   semantic_analyzed_at = NULL,
		   semantic_error = NULL,
		   semantic_retry_count = 0,
		   embeddings_analyzed_at = NULL,
		   embeddings_error = NULL,
		   embeddings_retry_count = 0,
		   updated_at = CURRENT_TIMESTAMP
		 WHERE path = ?`,
		path,
	)
	if err != nil {
		return fmt.Errorf("failed to clear analysis state; %w", err)
	}

	return nil
}

// ListFilesNeedingMetadata returns files that have not had metadata computed yet.
func (r *SQLiteRegistry) ListFilesNeedingMetadata(ctx context.Context, parentPath string) ([]FileState, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, path, content_hash, metadata_hash, size, mod_time,
		        last_analyzed_at, analysis_version,
		        metadata_analyzed_at, semantic_analyzed_at, semantic_error, semantic_retry_count,
		        embeddings_analyzed_at, embeddings_error, embeddings_retry_count,
		        created_at, updated_at
		 FROM file_state
		 WHERE (path LIKE ? OR path = ?)
		   AND metadata_analyzed_at IS NULL
		 ORDER BY path`,
		prefix+"%", parentPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list files needing metadata; %w", err)
	}
	defer rows.Close()

	return scanAllFileStates(rows)
}

// ListFilesNeedingSemantic returns files that need semantic analysis.
// Excludes files that have exceeded maxRetries.
func (r *SQLiteRegistry) ListFilesNeedingSemantic(ctx context.Context, parentPath string, maxRetries int) ([]FileState, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, path, content_hash, metadata_hash, size, mod_time,
		        last_analyzed_at, analysis_version,
		        metadata_analyzed_at, semantic_analyzed_at, semantic_error, semantic_retry_count,
		        embeddings_analyzed_at, embeddings_error, embeddings_retry_count,
		        created_at, updated_at
		 FROM file_state
		 WHERE (path LIKE ? OR path = ?)
		   AND metadata_analyzed_at IS NOT NULL
		   AND semantic_analyzed_at IS NULL
		   AND semantic_retry_count < ?
		 ORDER BY path`,
		prefix+"%", parentPath, maxRetries,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list files needing semantic analysis; %w", err)
	}
	defer rows.Close()

	return scanAllFileStates(rows)
}

// ListFilesNeedingEmbeddings returns files that need embeddings generation.
// Excludes files that have exceeded maxRetries.
func (r *SQLiteRegistry) ListFilesNeedingEmbeddings(ctx context.Context, parentPath string, maxRetries int) ([]FileState, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, path, content_hash, metadata_hash, size, mod_time,
		        last_analyzed_at, analysis_version,
		        metadata_analyzed_at, semantic_analyzed_at, semantic_error, semantic_retry_count,
		        embeddings_analyzed_at, embeddings_error, embeddings_retry_count,
		        created_at, updated_at
		 FROM file_state
		 WHERE (path LIKE ? OR path = ?)
		   AND semantic_analyzed_at IS NOT NULL
		   AND embeddings_analyzed_at IS NULL
		   AND embeddings_retry_count < ?
		 ORDER BY path`,
		prefix+"%", parentPath, maxRetries,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list files needing embeddings; %w", err)
	}
	defer rows.Close()

	return scanAllFileStates(rows)
}

// CheckPathHealth validates all remembered paths and returns their status.
// This method does not modify the registry - it only reports current status.
func (r *SQLiteRegistry) CheckPathHealth(ctx context.Context) ([]PathStatus, error) {
	paths, err := r.ListPaths(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list paths; %w", err)
	}

	statuses := make([]PathStatus, 0, len(paths))
	for _, p := range paths {
		status := PathStatus{Path: p.Path}

		_, err := os.Stat(p.Path)
		if err == nil {
			status.Status = PathStatusOK
		} else if os.IsNotExist(err) {
			status.Status = PathStatusMissing
			status.Error = err
		} else if os.IsPermission(err) {
			status.Status = PathStatusDenied
			status.Error = err
		} else {
			status.Status = PathStatusError
			status.Error = err
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// ValidateAndCleanPaths checks all remembered paths and removes those that
// no longer exist. Returns the list of removed paths.
// Only paths with "missing" status are removed; paths with permission or other
// errors are preserved.
func (r *SQLiteRegistry) ValidateAndCleanPaths(ctx context.Context) ([]string, error) {
	statuses, err := r.CheckPathHealth(ctx)
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, status := range statuses {
		if status.Status != PathStatusMissing {
			continue
		}

		// Delete all file_state entries under this path (best effort - ignore errors)
		_ = r.DeleteFileStatesForPath(ctx, status.Path)

		// Remove the remembered path itself
		if err := r.RemovePath(ctx, status.Path); err != nil {
			if !errors.Is(err, ErrPathNotFound) {
				return removed, fmt.Errorf("failed to remove path %s; %w", status.Path, err)
			}
			// Path already removed - continue
		}

		removed = append(removed, status.Path)
	}

	return removed, nil
}

// scanAllFileStates is a helper that scans all rows into FileState slice.
func scanAllFileStates(rows *sql.Rows) ([]FileState, error) {
	var states []FileState
	for rows.Next() {
		s, err := scanFileStateRows(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, *s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file states; %w", err)
	}

	return states, nil
}

// scanRememberedPath scans a single row into a RememberedPath.
func scanRememberedPath(row *sql.Row) (*RememberedPath, error) {
	var p RememberedPath
	var configJSON sql.NullString
	var lastWalkAt sql.NullTime

	err := row.Scan(&p.ID, &p.Path, &configJSON, &lastWalkAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPathNotFound
		}
		return nil, fmt.Errorf("failed to scan remembered path; %w", err)
	}

	if configJSON.Valid {
		var config PathConfig
		if err := json.Unmarshal([]byte(configJSON.String), &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config; %w", err)
		}
		p.Config = &config
	}

	if lastWalkAt.Valid {
		p.LastWalkAt = &lastWalkAt.Time
	}

	return &p, nil
}

// scanRememberedPathRows scans rows into a RememberedPath.
func scanRememberedPathRows(rows *sql.Rows) (*RememberedPath, error) {
	var p RememberedPath
	var configJSON sql.NullString
	var lastWalkAt sql.NullTime

	err := rows.Scan(&p.ID, &p.Path, &configJSON, &lastWalkAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan remembered path; %w", err)
	}

	if configJSON.Valid {
		var config PathConfig
		if err := json.Unmarshal([]byte(configJSON.String), &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config; %w", err)
		}
		p.Config = &config
	}

	if lastWalkAt.Valid {
		p.LastWalkAt = &lastWalkAt.Time
	}

	return &p, nil
}

// scanFileState scans a single row into a FileState.
func scanFileState(row *sql.Row) (*FileState, error) {
	var s FileState
	var lastAnalyzedAt sql.NullTime
	var analysisVersion sql.NullString
	var metadataAnalyzedAt sql.NullTime
	var semanticAnalyzedAt sql.NullTime
	var semanticError sql.NullString
	var embeddingsAnalyzedAt sql.NullTime
	var embeddingsError sql.NullString

	err := row.Scan(&s.ID, &s.Path, &s.ContentHash, &s.MetadataHash, &s.Size, &s.ModTime,
		&lastAnalyzedAt, &analysisVersion,
		&metadataAnalyzedAt, &semanticAnalyzedAt, &semanticError, &s.SemanticRetryCount,
		&embeddingsAnalyzedAt, &embeddingsError, &s.EmbeddingsRetryCount,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPathNotFound
		}
		return nil, fmt.Errorf("failed to scan file state; %w", err)
	}

	if lastAnalyzedAt.Valid {
		s.LastAnalyzedAt = &lastAnalyzedAt.Time
	}
	if analysisVersion.Valid {
		s.AnalysisVersion = analysisVersion.String
	}
	if metadataAnalyzedAt.Valid {
		s.MetadataAnalyzedAt = &metadataAnalyzedAt.Time
	}
	if semanticAnalyzedAt.Valid {
		s.SemanticAnalyzedAt = &semanticAnalyzedAt.Time
	}
	if semanticError.Valid {
		s.SemanticError = &semanticError.String
	}
	if embeddingsAnalyzedAt.Valid {
		s.EmbeddingsAnalyzedAt = &embeddingsAnalyzedAt.Time
	}
	if embeddingsError.Valid {
		s.EmbeddingsError = &embeddingsError.String
	}

	return &s, nil
}

// scanFileStateRows scans rows into a FileState.
func scanFileStateRows(rows *sql.Rows) (*FileState, error) {
	var s FileState
	var lastAnalyzedAt sql.NullTime
	var analysisVersion sql.NullString
	var metadataAnalyzedAt sql.NullTime
	var semanticAnalyzedAt sql.NullTime
	var semanticError sql.NullString
	var embeddingsAnalyzedAt sql.NullTime
	var embeddingsError sql.NullString

	err := rows.Scan(&s.ID, &s.Path, &s.ContentHash, &s.MetadataHash, &s.Size, &s.ModTime,
		&lastAnalyzedAt, &analysisVersion,
		&metadataAnalyzedAt, &semanticAnalyzedAt, &semanticError, &s.SemanticRetryCount,
		&embeddingsAnalyzedAt, &embeddingsError, &s.EmbeddingsRetryCount,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan file state; %w", err)
	}

	if lastAnalyzedAt.Valid {
		s.LastAnalyzedAt = &lastAnalyzedAt.Time
	}
	if analysisVersion.Valid {
		s.AnalysisVersion = analysisVersion.String
	}
	if metadataAnalyzedAt.Valid {
		s.MetadataAnalyzedAt = &metadataAnalyzedAt.Time
	}
	if semanticAnalyzedAt.Valid {
		s.SemanticAnalyzedAt = &semanticAnalyzedAt.Time
	}
	if semanticError.Valid {
		s.SemanticError = &semanticError.String
	}
	if embeddingsAnalyzedAt.Valid {
		s.EmbeddingsAnalyzedAt = &embeddingsAnalyzedAt.Time
	}
	if embeddingsError.Valid {
		s.EmbeddingsError = &embeddingsError.String
	}

	return &s, nil
}
