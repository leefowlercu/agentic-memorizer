// Package storage provides consolidated SQLite storage for the memorizer daemon.
// It manages remembered paths, file state, critical events, and the persistence queue
// in a single database file.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// Storage provides access to the consolidated SQLite database.
type Storage struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

// Open creates a new Storage instance with the given database path.
// It creates the directory structure if needed and runs migrations.
func Open(ctx context.Context, dbPath string) (*Storage, error) {
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

	// Serialize access to avoid SQLite write contention.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Configure busy timeout and enable foreign keys/WAL mode for better concurrency
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy timeout; %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys; %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode; %w", err)
	}

	s := &Storage{
		db:     db,
		dbPath: dbPath,
	}

	// Run migrations
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations; %w", err)
	}

	return s, nil
}

// DB returns the underlying database connection.
// Use with care; prefer using Storage methods.
func (s *Storage) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// Path returns the database file path.
func (s *Storage) Path() string {
	return s.dbPath
}

// migrate runs all pending migrations on the database.
func (s *Storage) migrate(ctx context.Context) error {
	// Ensure schema_migrations table exists first
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table; %w", err)
	}

	// Get current version
	currentVersion, err := s.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version; %w", err)
	}

	// Run pending migrations
	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}

		if err := s.runMigration(ctx, m); err != nil {
			return fmt.Errorf("failed to run migration %d (%s); %w", m.Version, m.Description, err)
		}
	}

	return nil
}

// getCurrentVersion returns the highest applied migration version.
func (s *Storage) getCurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// runMigration executes a single migration within a transaction.
func (s *Storage) runMigration(ctx context.Context, m Migration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction; %w", err)
	}
	defer tx.Rollback()

	// Execute the migration
	if _, err := tx.ExecContext(ctx, m.Up); err != nil {
		return fmt.Errorf("failed to execute migration; %w", err)
	}

	// Record the migration
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
		m.Version, m.Description,
	); err != nil {
		return fmt.Errorf("failed to record migration; %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction; %w", err)
	}

	return nil
}

// GetSchemaVersion returns the current schema version.
func (s *Storage) GetSchemaVersion(ctx context.Context) (int, error) {
	return s.getCurrentVersion(ctx)
}

// Migration represents a database schema migration.
type Migration struct {
	Version     int
	Description string
	Up          string
}

// migrations contains all schema migrations in order.
var migrations = []Migration{
	{
		Version:     1,
		Description: "Create remembered_paths table",
		Up: `
			CREATE TABLE IF NOT EXISTS remembered_paths (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				path TEXT UNIQUE NOT NULL,
				config_json TEXT,
				last_walk_at TIMESTAMP,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_remembered_paths_path ON remembered_paths(path);
		`,
	},
	{
		Version:     2,
		Description: "Create file_state table with analysis tracking",
		Up: `
			CREATE TABLE IF NOT EXISTS file_state (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				path TEXT UNIQUE NOT NULL,
				content_hash TEXT NOT NULL,
				metadata_hash TEXT NOT NULL,
				size INTEGER NOT NULL,
				mod_time TIMESTAMP NOT NULL,
				last_analyzed_at TIMESTAMP,
				analysis_version TEXT,
				-- Granular analysis state tracking
				metadata_analyzed_at TIMESTAMP,
				semantic_analyzed_at TIMESTAMP,
				semantic_error TEXT,
				semantic_retry_count INTEGER DEFAULT 0,
				embeddings_analyzed_at TIMESTAMP,
				embeddings_error TEXT,
				embeddings_retry_count INTEGER DEFAULT 0,
				-- Timestamps
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_file_state_content_hash ON file_state(content_hash);
			CREATE INDEX IF NOT EXISTS idx_file_state_path ON file_state(path);
			CREATE INDEX IF NOT EXISTS idx_file_state_semantic_error ON file_state(semantic_error);
			CREATE INDEX IF NOT EXISTS idx_file_state_embeddings_error ON file_state(embeddings_error);
		`,
	},
	{
		Version:     3,
		Description: "Create critical_events table",
		Up: `
			CREATE TABLE IF NOT EXISTS critical_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				event_type TEXT NOT NULL,
				payload BLOB NOT NULL,
				created_at TIMESTAMP NOT NULL
			);
		`,
	},
	{
		Version:     4,
		Description: "Create persistence_queue table",
		Up: `
			CREATE TABLE IF NOT EXISTS persistence_queue (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				file_path TEXT NOT NULL,
				content_hash TEXT NOT NULL,
				result_json BLOB NOT NULL,
				status TEXT NOT NULL DEFAULT 'pending',
				retry_count INTEGER DEFAULT 0,
				last_error TEXT,
				enqueued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				started_at TIMESTAMP,
				completed_at TIMESTAMP,
				UNIQUE(file_path, content_hash)
			);

			CREATE INDEX IF NOT EXISTS idx_persistence_queue_status ON persistence_queue(status);
			CREATE INDEX IF NOT EXISTS idx_persistence_queue_enqueued_at ON persistence_queue(enqueued_at);
		`,
	},
	{
		Version:     5,
		Description: "Create file_discovery table",
		Up: `
			CREATE TABLE IF NOT EXISTS file_discovery (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				path TEXT UNIQUE NOT NULL,
				content_hash TEXT,
				size INTEGER NOT NULL,
				mod_time TIMESTAMP NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_file_discovery_path ON file_discovery(path);
			CREATE INDEX IF NOT EXISTS idx_file_discovery_content_hash ON file_discovery(content_hash);
		`,
	},
}
