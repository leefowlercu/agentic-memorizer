package registry

import (
	"context"
	"database/sql"
	"fmt"
)

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
		Description: "Create schema_migrations table",
		Up: `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				description TEXT NOT NULL,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`,
	},
}

// Migrate runs all pending migrations on the database.
func Migrate(ctx context.Context, db *sql.DB) error {
	// Ensure schema_migrations table exists first
	_, err := db.ExecContext(ctx, `
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
	currentVersion, err := getCurrentVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get current version; %w", err)
	}

	// Run pending migrations
	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}

		if err := runMigration(ctx, db, m); err != nil {
			return fmt.Errorf("failed to run migration %d (%s); %w", m.Version, m.Description, err)
		}
	}

	return nil
}

// getCurrentVersion returns the highest applied migration version.
func getCurrentVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// runMigration executes a single migration within a transaction.
func runMigration(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
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
func GetSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	return getCurrentVersion(ctx, db)
}
