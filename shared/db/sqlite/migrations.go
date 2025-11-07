package sqlite

import (
	"database/sql"
	"fmt"
)

// migration represents a single database migration
type migration struct {
	version int
	name    string
	up      string
}

// migrations is the ordered list of all database migrations
// Each migration should be idempotent and safe to run multiple times
var migrations = []migration{
	{
		version: 1,
		name:    "create_posts_table",
		up: `
			CREATE TABLE IF NOT EXISTS posts (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL,
				snippet TEXT NOT NULL,
				html_path TEXT NOT NULL,
				updated_at TIMESTAMP,
				published_at TIMESTAMP,
				created_at TIMESTAMP NOT NULL
			);

			CREATE INDEX IF NOT EXISTS idx_posts_published_at 
			ON posts(published_at DESC)
			WHERE published_at IS NOT NULL;
		`,
	},
	{
		version: 2,
		name:    "create_images_table",
		up: `
			CREATE TABLE IF NOT EXISTS images (
				path TEXT PRIMARY KEY,
				hash TEXT NOT NULL,
				updated_at TIMESTAMP,
				created_at TIMESTAMP NOT NULL
			);

			CREATE INDEX IF NOT EXISTS idx_images_updated_at 
			ON images(updated_at DESC);
		`,
	},
}

// runMigrations executes all pending migrations
func runMigrations(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	currentVersion := 0
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %w", err)
	}

	// Run pending migrations
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue // Already applied
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", m.version, err)
		}

		_, err = tx.Exec(m.up)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d (%s): %w", m.version, m.name, err)
		}

		_, err = tx.Exec(
			"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
			m.version,
			m.name,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", m.version, err)
		}
	}

	return nil
}
