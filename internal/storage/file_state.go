package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

// GetFileState retrieves the file state for a given path.
func (s *Storage) GetFileState(ctx context.Context, path string) (*FileState, error) {
	path = filepath.Clean(path)

	row := s.db.QueryRowContext(ctx,
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
func (s *Storage) UpdateFileState(ctx context.Context, state *FileState) error {
	state.Path = filepath.Clean(state.Path)

	_, err := s.db.ExecContext(ctx,
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
func (s *Storage) DeleteFileState(ctx context.Context, path string) error {
	path = filepath.Clean(path)

	result, err := s.db.ExecContext(ctx,
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
func (s *Storage) ListFileStates(ctx context.Context, parentPath string) ([]FileState, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := s.db.QueryContext(ctx,
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
		st, err := scanFileStateRows(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, *st)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file states; %w", err)
	}

	return states, nil
}

// DeleteFileStatesForPath removes all file states under a given parent path.
func (s *Storage) DeleteFileStatesForPath(ctx context.Context, parentPath string) error {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	_, err := s.db.ExecContext(ctx,
		"DELETE FROM file_state WHERE path LIKE ? OR path = ?",
		prefix+"%", parentPath,
	)
	if err != nil {
		return fmt.Errorf("failed to delete file states; %w", err)
	}

	return nil
}

// CountFileStates returns the count of discovered files under a parent path.
func (s *Storage) CountFileStates(ctx context.Context, parentPath string) (int, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_state WHERE path LIKE ? OR path = ?`,
		prefix+"%", parentPath,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count file states; %w", err)
	}

	return count, nil
}

// CountAnalyzedFiles returns the count of files with completed semantic analysis under a parent path.
func (s *Storage) CountAnalyzedFiles(ctx context.Context, parentPath string) (int, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_state
		 WHERE (path LIKE ? OR path = ?)
		   AND semantic_analyzed_at IS NOT NULL`,
		prefix+"%", parentPath,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count analyzed files; %w", err)
	}

	return count, nil
}

// CountEmbeddingsFiles returns the count of files with completed embeddings generation under a parent path.
func (s *Storage) CountEmbeddingsFiles(ctx context.Context, parentPath string) (int, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_state
		 WHERE (path LIKE ? OR path = ?)
		   AND embeddings_analyzed_at IS NOT NULL`,
		prefix+"%", parentPath,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count embeddings files; %w", err)
	}

	return count, nil
}

// UpdateMetadataState updates the metadata tracking fields for a file.
// This is called after computing content hash and file metadata.
func (s *Storage) UpdateMetadataState(ctx context.Context, path string, contentHash string, metadataHash string, size int64, modTime time.Time) error {
	path = filepath.Clean(path)
	now := time.Now()

	_, err := s.db.ExecContext(ctx,
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
func (s *Storage) UpdateSemanticState(ctx context.Context, path string, analysisVersion string, analysisErr error) error {
	path = filepath.Clean(path)
	now := time.Now()

	var errStr *string
	if analysisErr != nil {
		str := analysisErr.Error()
		errStr = &str
	}

	if analysisErr == nil {
		// Success: set timestamp, clear error and reset retry count
		_, err := s.db.ExecContext(ctx,
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
		_, err := s.db.ExecContext(ctx,
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
func (s *Storage) UpdateEmbeddingsState(ctx context.Context, path string, embeddingsErr error) error {
	path = filepath.Clean(path)
	now := time.Now()

	var errStr *string
	if embeddingsErr != nil {
		str := embeddingsErr.Error()
		errStr = &str
	}

	if embeddingsErr == nil {
		// Success: set timestamp, clear error and reset retry count
		_, err := s.db.ExecContext(ctx,
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
		_, err := s.db.ExecContext(ctx,
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
func (s *Storage) ClearAnalysisState(ctx context.Context, path string) error {
	path = filepath.Clean(path)

	_, err := s.db.ExecContext(ctx,
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
func (s *Storage) ListFilesNeedingMetadata(ctx context.Context, parentPath string) ([]FileState, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := s.db.QueryContext(ctx,
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
func (s *Storage) ListFilesNeedingSemantic(ctx context.Context, parentPath string, maxRetries int) ([]FileState, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := s.db.QueryContext(ctx,
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
func (s *Storage) ListFilesNeedingEmbeddings(ctx context.Context, parentPath string, maxRetries int) ([]FileState, error) {
	parentPath = filepath.Clean(parentPath)
	prefix := parentPath + string(filepath.Separator)

	rows, err := s.db.QueryContext(ctx,
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

// scanAllFileStates is a helper that scans all rows into FileState slice.
func scanAllFileStates(rows *sql.Rows) ([]FileState, error) {
	var states []FileState
	for rows.Next() {
		st, err := scanFileStateRows(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, *st)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file states; %w", err)
	}

	return states, nil
}

// scanFileState scans a single row into a FileState.
func scanFileState(row *sql.Row) (*FileState, error) {
	var st FileState
	var lastAnalyzedAt sql.NullTime
	var analysisVersion sql.NullString
	var metadataAnalyzedAt sql.NullTime
	var semanticAnalyzedAt sql.NullTime
	var semanticError sql.NullString
	var embeddingsAnalyzedAt sql.NullTime
	var embeddingsError sql.NullString

	err := row.Scan(&st.ID, &st.Path, &st.ContentHash, &st.MetadataHash, &st.Size, &st.ModTime,
		&lastAnalyzedAt, &analysisVersion,
		&metadataAnalyzedAt, &semanticAnalyzedAt, &semanticError, &st.SemanticRetryCount,
		&embeddingsAnalyzedAt, &embeddingsError, &st.EmbeddingsRetryCount,
		&st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPathNotFound
		}
		return nil, fmt.Errorf("failed to scan file state; %w", err)
	}

	if lastAnalyzedAt.Valid {
		st.LastAnalyzedAt = &lastAnalyzedAt.Time
	}
	if analysisVersion.Valid {
		st.AnalysisVersion = analysisVersion.String
	}
	if metadataAnalyzedAt.Valid {
		st.MetadataAnalyzedAt = &metadataAnalyzedAt.Time
	}
	if semanticAnalyzedAt.Valid {
		st.SemanticAnalyzedAt = &semanticAnalyzedAt.Time
	}
	if semanticError.Valid {
		st.SemanticError = &semanticError.String
	}
	if embeddingsAnalyzedAt.Valid {
		st.EmbeddingsAnalyzedAt = &embeddingsAnalyzedAt.Time
	}
	if embeddingsError.Valid {
		st.EmbeddingsError = &embeddingsError.String
	}

	return &st, nil
}

// scanFileStateRows scans rows into a FileState.
func scanFileStateRows(rows *sql.Rows) (*FileState, error) {
	var st FileState
	var lastAnalyzedAt sql.NullTime
	var analysisVersion sql.NullString
	var metadataAnalyzedAt sql.NullTime
	var semanticAnalyzedAt sql.NullTime
	var semanticError sql.NullString
	var embeddingsAnalyzedAt sql.NullTime
	var embeddingsError sql.NullString

	err := rows.Scan(&st.ID, &st.Path, &st.ContentHash, &st.MetadataHash, &st.Size, &st.ModTime,
		&lastAnalyzedAt, &analysisVersion,
		&metadataAnalyzedAt, &semanticAnalyzedAt, &semanticError, &st.SemanticRetryCount,
		&embeddingsAnalyzedAt, &embeddingsError, &st.EmbeddingsRetryCount,
		&st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan file state; %w", err)
	}

	if lastAnalyzedAt.Valid {
		st.LastAnalyzedAt = &lastAnalyzedAt.Time
	}
	if analysisVersion.Valid {
		st.AnalysisVersion = analysisVersion.String
	}
	if metadataAnalyzedAt.Valid {
		st.MetadataAnalyzedAt = &metadataAnalyzedAt.Time
	}
	if semanticAnalyzedAt.Valid {
		st.SemanticAnalyzedAt = &semanticAnalyzedAt.Time
	}
	if semanticError.Valid {
		st.SemanticError = &semanticError.String
	}
	if embeddingsAnalyzedAt.Valid {
		st.EmbeddingsAnalyzedAt = &embeddingsAnalyzedAt.Time
	}
	if embeddingsError.Valid {
		st.EmbeddingsError = &embeddingsError.String
	}

	return &st, nil
}
