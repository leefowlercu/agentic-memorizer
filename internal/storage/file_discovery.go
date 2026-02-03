package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"
)

// UpdateDiscoveryState upserts a discovery record for a file.
func (s *Storage) UpdateDiscoveryState(ctx context.Context, path string, contentHash string, size int64, modTime time.Time) error {
	path = filepath.Clean(path)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO file_discovery (path, content_hash, size, mod_time, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET
		   content_hash = excluded.content_hash,
		   size = excluded.size,
		   mod_time = excluded.mod_time,
		   updated_at = CURRENT_TIMESTAMP`,
		path, contentHash, size, modTime,
	)
	if err != nil {
		return fmt.Errorf("failed to update discovery state; %w", err)
	}

	return nil
}

// DeleteDiscoveryState removes the discovery record for a given path.
func (s *Storage) DeleteDiscoveryState(ctx context.Context, path string) error {
	path = filepath.Clean(path)

	result, err := s.db.ExecContext(ctx,
		"DELETE FROM file_discovery WHERE path = ?",
		path,
	)
	if err != nil {
		return fmt.Errorf("failed to delete discovery state; %w", err)
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

// DeleteDiscoveryStatesForPath removes all discovery records under a parent path.
func (s *Storage) DeleteDiscoveryStatesForPath(ctx context.Context, parentPath string) error {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	_, err := s.db.ExecContext(ctx,
		`DELETE FROM file_discovery WHERE path LIKE ? OR path = ?`,
		prefix+"%", parentPath,
	)
	if err != nil {
		return fmt.Errorf("failed to delete discovery states; %w", err)
	}

	return nil
}

// ListDiscoveryStates returns discovery records under a parent path.
func (s *Storage) ListDiscoveryStates(ctx context.Context, parentPath string) ([]FileDiscovery, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, path, content_hash, size, mod_time, created_at, updated_at
		 FROM file_discovery
		 WHERE path LIKE ? OR path = ?`,
		prefix+"%", parentPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query discovery states; %w", err)
	}
	defer rows.Close()

	var states []FileDiscovery
	for rows.Next() {
		var state FileDiscovery
		var contentHash sql.NullString
		if err := rows.Scan(&state.ID, &state.Path, &contentHash, &state.Size, &state.ModTime, &state.CreatedAt, &state.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan discovery state; %w", err)
		}
		if contentHash.Valid {
			state.ContentHash = contentHash.String
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate discovery states; %w", err)
	}

	return states, nil
}

// CountDiscoveredFiles returns the count of discovered files under a parent path.
func (s *Storage) CountDiscoveredFiles(ctx context.Context, parentPath string) (int, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_discovery WHERE path LIKE ? OR path = ?`,
		prefix+"%", parentPath,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count discovery states; %w", err)
	}

	return count, nil
}
