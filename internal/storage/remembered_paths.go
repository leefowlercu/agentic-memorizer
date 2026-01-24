package storage

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
)

// ErrPathNotFound is returned when a path is not found in the registry.
var ErrPathNotFound = errors.New("path not found")

// ErrPathExists is returned when attempting to add a path that already exists.
var ErrPathExists = errors.New("path already exists")

// AddPath adds a new remembered path to the registry.
func (s *Storage) AddPath(ctx context.Context, path string, config *PathConfig) error {
	path = filepath.Clean(path)

	var configJSON *string
	if config != nil {
		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config; %w", err)
		}
		str := string(data)
		configJSON = &str
	}

	_, err := s.db.ExecContext(ctx,
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
func (s *Storage) RemovePath(ctx context.Context, path string) error {
	path = filepath.Clean(path)

	result, err := s.db.ExecContext(ctx,
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
func (s *Storage) GetPath(ctx context.Context, path string) (*RememberedPath, error) {
	path = filepath.Clean(path)

	row := s.db.QueryRowContext(ctx,
		`SELECT id, path, config_json, last_walk_at, created_at, updated_at
		 FROM remembered_paths WHERE path = ?`,
		path,
	)

	return scanRememberedPath(row)
}

// ListPaths returns all remembered paths.
func (s *Storage) ListPaths(ctx context.Context) ([]RememberedPath, error) {
	rows, err := s.db.QueryContext(ctx,
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
func (s *Storage) UpdatePathConfig(ctx context.Context, path string, config *PathConfig) error {
	path = filepath.Clean(path)

	var configJSON *string
	if config != nil {
		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config; %w", err)
		}
		str := string(data)
		configJSON = &str
	}

	result, err := s.db.ExecContext(ctx,
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
func (s *Storage) UpdatePathLastWalk(ctx context.Context, path string, lastWalk time.Time) error {
	path = filepath.Clean(path)

	result, err := s.db.ExecContext(ctx,
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
func (s *Storage) FindContainingPath(ctx context.Context, filePath string) (*RememberedPath, error) {
	filePath = filepath.Clean(filePath)

	// Get all remembered paths and find the closest ancestor
	paths, err := s.ListPaths(ctx)
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
func (s *Storage) GetEffectiveConfig(ctx context.Context, filePath string) (*PathConfig, error) {
	rp, err := s.FindContainingPath(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return rp.Config, nil
}

// CheckPathHealth validates all remembered paths and returns their status.
// This method does not modify the registry - it only reports current status.
func (s *Storage) CheckPathHealth(ctx context.Context) ([]PathStatus, error) {
	paths, err := s.ListPaths(ctx)
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
func (s *Storage) ValidateAndCleanPaths(ctx context.Context) ([]string, error) {
	statuses, err := s.CheckPathHealth(ctx)
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, status := range statuses {
		if status.Status != PathStatusMissing {
			continue
		}

		// Delete all file_state entries under this path (best effort - ignore errors)
		_ = s.DeleteFileStatesForPath(ctx, status.Path)

		// Remove the remembered path itself
		if err := s.RemovePath(ctx, status.Path); err != nil {
			if !errors.Is(err, ErrPathNotFound) {
				return removed, fmt.Errorf("failed to remove path %s; %w", status.Path, err)
			}
			// Path already removed - continue
		}

		removed = append(removed, status.Path)
	}

	return removed, nil
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
