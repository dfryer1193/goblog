package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dfryer1193/goblog/shared/db"
)

func TestNewSQLiteDB(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "env variable",
			envValue: "/tmp/env.db",
			want:     "/tmp/env.db",
		},
		{
			name: "default path",
			want: "./goblog.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("SQLITE_DB_PATH", tt.envValue)
				defer os.Unsetenv("SQLITE_DB_PATH")
			} else {
				os.Unsetenv("SQLITE_DB_PATH")
			}

			cfg := NewSQLiteConfig()

			database := NewSQLiteDB(cfg)
			
			if database.dbPath != tt.want {
				t.Errorf("dbPath = %v, want %v", database.dbPath, tt.want)
			}
		})
	}
}

func TestNewSQLiteDBWithExplicitPath(t *testing.T) {
	cfg := &SQLiteConfig{
		Path: "/tmp/test.db",
	}

	database := NewSQLiteDB(cfg)
	
	if database.dbPath != "/tmp/test.db" {
		t.Errorf("dbPath = %v, want %v", database.dbPath, "/tmp/test.db")
	}
}

func TestSQLiteDB_Connect(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &SQLiteConfig{
		Path: dbPath,
	}

	database := NewSQLiteDB(cfg)

	// Test successful connection
	err := database.Connect()
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer database.Close()

	// Verify DB() returns non-nil
	if database.DB() == nil {
		t.Error("DB() returned nil after Connect()")
	}

	// Test that connecting again returns an error
	err = database.Connect()
	if err == nil {
		t.Error("Connect() should return error when already connected")
	}
}

func TestSQLiteDB_Close(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &SQLiteConfig{
		Path: dbPath,
	}

	database := NewSQLiteDB(cfg)

	// Close without connecting should not error
	err := database.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Connect and close
	err = database.Connect()
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	err = database.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify DB() returns nil after close
	if database.DB() != nil {
		t.Error("DB() should return nil after Close()")
	}
}

func TestSQLiteDB_BasicOperations(t *testing.T) {
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

	sqlDB := database.DB()

	// Create a test table
	_, err = sqlDB.Exec(`
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert data
	result, err := sqlDB.Exec("INSERT INTO test_table (name) VALUES (?)", "test")
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get last insert id: %v", err)
	}

	if id != 1 {
		t.Errorf("Expected id = 1, got %d", id)
	}

	// Query data
	var name string
	err = sqlDB.QueryRow("SELECT name FROM test_table WHERE id = ?", id).Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query data: %v", err)
	}

	if name != "test" {
		t.Errorf("Expected name = 'test', got %q", name)
	}
}

func TestSQLiteDB_InterfaceCompliance(t *testing.T) {
	var _ db.Database = (*SQLiteDB)(nil)
}
