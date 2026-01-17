package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// CriticalQueue is a bounded durable queue for critical events.
type CriticalQueue interface {
	Enqueue(event Event) error
	Dequeue(ctx context.Context) (Event, error)
	Len() (int, error)
	Cap() int
	Close() error
}

// SQLiteCriticalQueue implements CriticalQueue using SQLite.
type SQLiteCriticalQueue struct {
	db     *sql.DB
	cap    int
	closed chan struct{}
}

type queuedEvent struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

// NewSQLiteCriticalQueue creates a bounded queue backed by SQLite.
func NewSQLiteCriticalQueue(path string, cap int) (*SQLiteCriticalQueue, error) {
	if cap <= 0 {
		cap = 1000
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite critical queue; %w", err)
	}
	schema := `
	CREATE TABLE IF NOT EXISTS critical_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		payload BLOB NOT NULL,
		created_at TIMESTAMP NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("init sqlite critical queue; %w", err)
	}
	return &SQLiteCriticalQueue{
		db:     db,
		cap:    cap,
		closed: make(chan struct{}),
	}, nil
}

// Enqueue adds an event, dropping oldest if at capacity.
func (q *SQLiteCriticalQueue) Enqueue(event Event) error {
	select {
	case <-q.closed:
		return fmt.Errorf("queue closed")
	default:
	}

	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM critical_events`).Scan(&count); err != nil {
		return err
	}
	if count >= q.cap {
		// Drop oldest
		if _, err := tx.Exec(`DELETE FROM critical_events WHERE id IN (SELECT id FROM critical_events ORDER BY id ASC LIMIT 1)`); err != nil {
			return err
		}
	}

	payload, err := json.Marshal(queuedEvent{
		Type:      event.Type,
		Timestamp: event.Timestamp,
		Payload:   event.Payload,
	})
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`INSERT INTO critical_events (event_type, payload, created_at) VALUES (?, ?, ?)`,
		event.Type, payload, time.Now()); err != nil {
		return err
	}

	return tx.Commit()
}

// Dequeue returns the oldest event, blocking until available or ctx cancellation.
func (q *SQLiteCriticalQueue) Dequeue(ctx context.Context) (Event, error) {
	for {
		select {
		case <-ctx.Done():
			return Event{}, ctx.Err()
		case <-q.closed:
			return Event{}, fmt.Errorf("queue closed")
		default:
		}

		var id int64
		var payload []byte
		err := q.db.QueryRow(`SELECT id, payload FROM critical_events ORDER BY id ASC LIMIT 1`).Scan(&id, &payload)
		if err == sql.ErrNoRows {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if err != nil {
			return Event{}, err
		}

		if _, err := q.db.Exec(`DELETE FROM critical_events WHERE id = ?`, id); err != nil {
			return Event{}, err
		}

		var qe queuedEvent
		if err := json.Unmarshal(payload, &qe); err != nil {
			return Event{}, err
		}
		return Event{
			Type:      qe.Type,
			Timestamp: qe.Timestamp,
			Payload:   qe.Payload,
		}, nil
	}
}

// Len returns the current queue size.
func (q *SQLiteCriticalQueue) Len() (int, error) {
	var count int
	err := q.db.QueryRow(`SELECT COUNT(*) FROM critical_events`).Scan(&count)
	return count, err
}

// Cap returns the maximum queue size.
func (q *SQLiteCriticalQueue) Cap() int {
	return q.cap
}

// Close closes the underlying DB.
func (q *SQLiteCriticalQueue) Close() error {
	select {
	case <-q.closed:
	default:
		close(q.closed)
	}
	return q.db.Close()
}
