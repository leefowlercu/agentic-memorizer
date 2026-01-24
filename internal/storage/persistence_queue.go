package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// DurablePersistenceQueue defines the interface for durable storage of analysis results
// that could not be persisted to the graph due to connectivity issues.
type DurablePersistenceQueue interface {
	// Enqueue adds an analysis result to the queue for later persistence.
	// Uses upsert semantics: if an entry exists for the same file path and content hash,
	// it is replaced with the new result.
	Enqueue(ctx context.Context, filePath, contentHash string, resultJSON []byte) error

	// DequeueBatch retrieves up to n pending items and atomically transitions them
	// to inflight status. Returns an empty slice if no items are pending.
	DequeueBatch(ctx context.Context, n int) ([]*QueuedResult, error)

	// Complete marks an item as successfully processed.
	Complete(ctx context.Context, id int64) error

	// Fail increments the retry count and records the error. If max retries is exceeded,
	// the item transitions to failed status.
	Fail(ctx context.Context, id int64, maxRetries int, errMsg string) error

	// Stats returns current queue statistics for health reporting.
	Stats(ctx context.Context) (*QueueStats, error)

	// Purge removes completed items older than completedOlderThan and failed items
	// older than failedOlderThan. Returns the count of purged items.
	Purge(ctx context.Context, completedOlderThan, failedOlderThan time.Duration) (int64, error)
}

// PersistenceQueue implements DurablePersistenceQueue using SQLite.
type PersistenceQueue struct {
	db *sql.DB
}

// NewPersistenceQueue creates a new PersistenceQueue using the provided database connection.
func NewPersistenceQueue(db *sql.DB) *PersistenceQueue {
	return &PersistenceQueue{db: db}
}

// Enqueue adds an analysis result to the queue for later persistence.
// Uses INSERT OR REPLACE for upsert semantics.
func (q *PersistenceQueue) Enqueue(ctx context.Context, filePath, contentHash string, resultJSON []byte) error {
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO persistence_queue (file_path, content_hash, result_json, status, enqueued_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(file_path, content_hash) DO UPDATE SET
			result_json = excluded.result_json,
			status = 'pending',
			retry_count = 0,
			last_error = NULL,
			enqueued_at = excluded.enqueued_at,
			started_at = NULL,
			completed_at = NULL
	`, filePath, contentHash, resultJSON, QueueStatusPending, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to enqueue item; %w", err)
	}
	return nil
}

// DequeueBatch retrieves up to n pending items and atomically transitions them to inflight.
func (q *PersistenceQueue) DequeueBatch(ctx context.Context, n int) ([]*QueuedResult, error) {
	if n <= 0 {
		return nil, nil
	}

	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction; %w", err)
	}
	defer tx.Rollback()

	// Select pending items ordered by enqueue time (FIFO)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, file_path, content_hash, result_json, status, retry_count,
		       last_error, enqueued_at, started_at, completed_at
		FROM persistence_queue
		WHERE status = ?
		ORDER BY enqueued_at ASC
		LIMIT ?
	`, QueueStatusPending, n)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending items; %w", err)
	}

	var results []*QueuedResult
	var ids []int64
	for rows.Next() {
		var r QueuedResult
		var status string
		var lastError sql.NullString
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&r.ID, &r.FilePath, &r.ContentHash, &r.ResultJSON, &status,
			&r.RetryCount, &lastError, &r.EnqueuedAt, &startedAt, &completedAt,
		)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan row; %w", err)
		}

		r.Status = QueueStatus(status)
		if lastError.Valid {
			r.LastError = lastError.String
		}
		if startedAt.Valid {
			r.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			r.CompletedAt = &completedAt.Time
		}

		results = append(results, &r)
		ids = append(ids, r.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows; %w", err)
	}
	rows.Close()

	if len(ids) == 0 {
		return nil, nil
	}

	// Update status to inflight for all selected items
	now := time.Now().UTC()
	for _, id := range ids {
		_, err := tx.ExecContext(ctx, `
			UPDATE persistence_queue
			SET status = ?, started_at = ?
			WHERE id = ?
		`, QueueStatusInflight, now, id)
		if err != nil {
			return nil, fmt.Errorf("failed to update item %d to inflight; %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction; %w", err)
	}

	// Update the results to reflect the new status
	for _, r := range results {
		r.Status = QueueStatusInflight
		r.StartedAt = &now
	}

	return results, nil
}

// Complete marks an item as successfully processed.
func (q *PersistenceQueue) Complete(ctx context.Context, id int64) error {
	result, err := q.db.ExecContext(ctx, `
		UPDATE persistence_queue
		SET status = ?, completed_at = ?
		WHERE id = ?
	`, QueueStatusCompleted, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("failed to complete item; %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected; %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("item %d not found", id)
	}

	return nil
}

// Fail increments the retry count and records the error.
// If retry_count exceeds maxRetries, the item transitions to failed status.
func (q *PersistenceQueue) Fail(ctx context.Context, id int64, maxRetries int, errMsg string) error {
	// First, get current retry count
	var retryCount int
	err := q.db.QueryRowContext(ctx, `
		SELECT retry_count FROM persistence_queue WHERE id = ?
	`, id).Scan(&retryCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("item %d not found", id)
		}
		return fmt.Errorf("failed to get retry count; %w", err)
	}

	newRetryCount := retryCount + 1
	now := time.Now().UTC()

	var newStatus QueueStatus
	if newRetryCount >= maxRetries {
		newStatus = QueueStatusFailed
	} else {
		// Return to pending for retry
		newStatus = QueueStatusPending
	}

	_, err = q.db.ExecContext(ctx, `
		UPDATE persistence_queue
		SET status = ?, retry_count = ?, last_error = ?, started_at = NULL,
		    completed_at = CASE WHEN ? = 'failed' THEN ? ELSE NULL END
		WHERE id = ?
	`, newStatus, newRetryCount, errMsg, newStatus, now, id)
	if err != nil {
		return fmt.Errorf("failed to update failed item; %w", err)
	}

	return nil
}

// Stats returns current queue statistics.
func (q *PersistenceQueue) Stats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{}

	// Count by status
	rows, err := q.db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM persistence_queue GROUP BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query stats; %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan stats row; %w", err)
		}

		switch QueueStatus(status) {
		case QueueStatusPending:
			stats.Pending = count
		case QueueStatusInflight:
			stats.Inflight = count
		case QueueStatusCompleted:
			stats.Completed = count
		case QueueStatusFailed:
			stats.Failed = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating stats rows; %w", err)
	}

	// Get oldest pending item
	var oldestPendingStr sql.NullString
	err = q.db.QueryRowContext(ctx, `
		SELECT MIN(enqueued_at) FROM persistence_queue WHERE status = ?
	`, QueueStatusPending).Scan(&oldestPendingStr)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get oldest pending; %w", err)
	}
	if oldestPendingStr.Valid && oldestPendingStr.String != "" {
		// Parse timestamp - SQLite MIN returns Go's default time format
		// Format: "2006-01-02 15:04:05.999999999 -0700 MST"
		t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", oldestPendingStr.String)
		if err != nil {
			// Try format with +0000 UTC instead of -0700 MST
			t, err = time.Parse("2006-01-02 15:04:05.999999999 +0000 UTC", oldestPendingStr.String)
			if err != nil {
				// Try RFC3339Nano (for direct queries)
				t, err = time.Parse(time.RFC3339Nano, oldestPendingStr.String)
			}
		}
		if err == nil {
			stats.OldestPending = &t
		}
	}

	return stats, nil
}

// Purge removes old completed and failed items.
func (q *PersistenceQueue) Purge(ctx context.Context, completedOlderThan, failedOlderThan time.Duration) (int64, error) {
	now := time.Now().UTC()
	completedCutoff := now.Add(-completedOlderThan)
	failedCutoff := now.Add(-failedOlderThan)

	result, err := q.db.ExecContext(ctx, `
		DELETE FROM persistence_queue
		WHERE (status = ? AND completed_at < ?)
		   OR (status = ? AND completed_at < ?)
	`, QueueStatusCompleted, completedCutoff, QueueStatusFailed, failedCutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to purge items; %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected; %w", err)
	}

	return affected, nil
}

// PersistenceQueueFromStorage creates a PersistenceQueue from a Storage instance.
func (s *Storage) PersistenceQueue() *PersistenceQueue {
	return NewPersistenceQueue(s.db)
}

// MarshalAnalysisResult serializes an analysis result to JSON for queue storage.
// This is a helper function for callers that need to serialize results before enqueueing.
func MarshalAnalysisResult(result any) ([]byte, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analysis result; %w", err)
	}
	return data, nil
}

// UnmarshalAnalysisResult deserializes an analysis result from JSON.
// This is a helper function for callers that need to deserialize results after dequeueing.
// The result parameter should be a pointer to the target type.
func UnmarshalAnalysisResult(data []byte, result any) error {
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("failed to unmarshal analysis result; %w", err)
	}
	return nil
}
