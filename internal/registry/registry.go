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
		        last_analyzed_at, analysis_version, created_at, updated_at
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
		                         last_analyzed_at, analysis_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET
		   content_hash = excluded.content_hash,
		   metadata_hash = excluded.metadata_hash,
		   size = excluded.size,
		   mod_time = excluded.mod_time,
		   last_analyzed_at = excluded.last_analyzed_at,
		   analysis_version = excluded.analysis_version,
		   updated_at = CURRENT_TIMESTAMP`,
		state.Path, state.ContentHash, state.MetadataHash, state.Size, state.ModTime,
		state.LastAnalyzedAt, state.AnalysisVersion,
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
		        last_analyzed_at, analysis_version, created_at, updated_at
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

	err := row.Scan(&s.ID, &s.Path, &s.ContentHash, &s.MetadataHash, &s.Size, &s.ModTime,
		&lastAnalyzedAt, &analysisVersion, &s.CreatedAt, &s.UpdatedAt)
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

	return &s, nil
}

// scanFileStateRows scans rows into a FileState.
func scanFileStateRows(rows *sql.Rows) (*FileState, error) {
	var s FileState
	var lastAnalyzedAt sql.NullTime
	var analysisVersion sql.NullString

	err := rows.Scan(&s.ID, &s.Path, &s.ContentHash, &s.MetadataHash, &s.Size, &s.ModTime,
		&lastAnalyzedAt, &analysisVersion, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan file state; %w", err)
	}

	if lastAnalyzedAt.Valid {
		s.LastAnalyzedAt = &lastAnalyzedAt.Time
	}
	if analysisVersion.Valid {
		s.AnalysisVersion = analysisVersion.String
	}

	return &s, nil
}
