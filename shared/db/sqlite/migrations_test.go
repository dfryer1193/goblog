package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestRunMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &SQLiteConfig{
		Path: dbPath,
	}

	database := NewSQLiteDB(cfg)
	err := database.Connect()
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer database.Close()

	db := database.DB()

	// Verify schema_migrations table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check schema_migrations table: %v", err)
	}
	if count != 1 {
		t.Errorf("schema_migrations table not created")
	}

	// Verify posts table exists
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='posts'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check posts table: %v", err)
	}
	if count != 1 {
		t.Errorf("posts table not created")
	}

	// Verify index exists
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_posts_published_at'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check index: %v", err)
	}
	if count != 1 {
		t.Errorf("idx_posts_published_at index not created")
	}

	// Verify migration was recorded
	var version int
	var name string
	err = db.QueryRow("SELECT version, name FROM schema_migrations WHERE version = 1").Scan(&version, &name)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
	if name != "create_posts_table" {
		t.Errorf("name = %q, want %q", name, "create_posts_table")
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &SQLiteConfig{
		Path: dbPath,
	}

	// Connect first time
	database := NewSQLiteDB(cfg)
	err := database.Connect()
	if err != nil {
		t.Fatalf("First Connect() error = %v", err)
	}
	database.Close()

	// Connect second time - migrations should not fail
	database = NewSQLiteDB(cfg)
	err = database.Connect()
	if err != nil {
		t.Fatalf("Second Connect() error = %v", err)
	}
	defer database.Close()

	db := database.DB()

	// Verify migration was only recorded once
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("migration recorded %d times, want 1", count)
	}
}

func TestPostsTableSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &SQLiteConfig{
		Path: dbPath,
	}

	database := NewSQLiteDB(cfg)
	err := database.Connect()
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer database.Close()

	db := database.DB()

	// Test inserting a post
	_, err = db.Exec(`
		INSERT INTO posts (id, title, snippet, html_path, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, "001", "Test Post", "Test snippet", "/posts/001.html")
	if err != nil {
		t.Fatalf("Failed to insert post: %v", err)
	}

	// Test querying the post
	var id, title, snippet, htmlPath string
	var updatedAt, publishedAt, createdAt sql.NullTime
	err = db.QueryRow("SELECT id, title, snippet, html_path, updated_at, published_at, created_at FROM posts WHERE id = ?", "001").
		Scan(&id, &title, &snippet, &htmlPath, &updatedAt, &publishedAt, &createdAt)
	if err != nil {
		t.Fatalf("Failed to query post: %v", err)
	}

	if id != "001" {
		t.Errorf("id = %q, want %q", id, "001")
	}
	if title != "Test Post" {
		t.Errorf("title = %q, want %q", title, "Test Post")
	}
	if !createdAt.Valid {
		t.Error("created_at should not be NULL")
	}
	if updatedAt.Valid {
		t.Error("updated_at should be NULL")
	}
	if publishedAt.Valid {
		t.Error("published_at should be NULL")
	}
}

