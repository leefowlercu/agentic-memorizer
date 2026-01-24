package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CriticalEvent represents a serializable event for the critical queue.
type CriticalEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

// CriticalEventQueue provides bounded durable queue operations for critical events.
// It uses the shared Storage database rather than a separate database file.
type CriticalEventQueue struct {
	storage *Storage
	cap     int
	closed  chan struct{}
}

// NewCriticalEventQueue creates a new critical event queue backed by the storage database.
func NewCriticalEventQueue(storage *Storage, cap int) *CriticalEventQueue {
	if cap <= 0 {
		cap = 1000
	}
	return &CriticalEventQueue{
		storage: storage,
		cap:     cap,
		closed:  make(chan struct{}),
	}
}

// Enqueue adds an event, dropping oldest if at capacity.
func (q *CriticalEventQueue) Enqueue(event CriticalEvent) error {
	select {
	case <-q.closed:
		return fmt.Errorf("queue closed")
	default:
	}

	db := q.storage.DB()

	tx, err := db.Begin()
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

	payload, err := json.Marshal(event)
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
func (q *CriticalEventQueue) Dequeue(ctx context.Context) (CriticalEvent, error) {
	db := q.storage.DB()

	for {
		select {
		case <-ctx.Done():
			return CriticalEvent{}, ctx.Err()
		case <-q.closed:
			return CriticalEvent{}, fmt.Errorf("queue closed")
		default:
		}

		var id int64
		var payload []byte
		err := db.QueryRow(`SELECT id, payload FROM critical_events ORDER BY id ASC LIMIT 1`).Scan(&id, &payload)
		if err == sql.ErrNoRows {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if err != nil {
			return CriticalEvent{}, err
		}

		if _, err := db.Exec(`DELETE FROM critical_events WHERE id = ?`, id); err != nil {
			return CriticalEvent{}, err
		}

		var event CriticalEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return CriticalEvent{}, err
		}
		return event, nil
	}
}

// Len returns the current queue size.
func (q *CriticalEventQueue) Len() (int, error) {
	var count int
	err := q.storage.DB().QueryRow(`SELECT COUNT(*) FROM critical_events`).Scan(&count)
	return count, err
}

// Cap returns the maximum queue size.
func (q *CriticalEventQueue) Cap() int {
	return q.cap
}

// Close marks the queue as closed.
// Note: This does not close the underlying storage - that is managed separately.
func (q *CriticalEventQueue) Close() error {
	select {
	case <-q.closed:
	default:
		close(q.closed)
	}
	return nil
}
