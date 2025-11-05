package sqlite

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/dfryer1193/goblog/shared/db"
	_ "modernc.org/sqlite"
)

const (
	// DefaultPath is the default path for the SQLite database
	defaultPath = "./goblog.db"
)

type SQLiteConfig struct {
	Path string
}

func NewSQLiteConfig() *SQLiteConfig {
	path := os.Getenv("SQLITE_DB_PATH")
	if path == "" {
		path = defaultPath
	}

	return &SQLiteConfig{
		Path: path,
	}
}

// SQLiteDB implements the db.Database interface for SQLite
type SQLiteDB struct {
	dbPath string
	db     *sql.DB
}

// NewSQLiteDB creates a new SQLite database instance
// If dbPath is empty, it will use the SQLITE_DB_PATH environment variable
// If that's also empty, it defaults to "./goblog.db"
func NewSQLiteDB(cfg *SQLiteConfig) db.Database {
	return &SQLiteDB{
		dbPath: cfg.Path,
	}
}

// Connect opens a connection to the SQLite database
func (s *SQLiteDB) Connect() error {
	if s.db != nil {
		return fmt.Errorf("database already connected")
	}

	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Set some recommended SQLite pragmas for better performance and reliability
	pragmas := []string{
		"PRAGMA journal_mode=WAL",   // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL", // Balance between safety and performance
		"PRAGMA foreign_keys=ON",    // Enable foreign key constraints
		"PRAGMA busy_timeout=5000",  // Wait up to 5 seconds if database is locked
		"PRAGMA cache_size=-64000",  // Use 64MB cache (negative means KB)
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return fmt.Errorf("failed to set pragma %q: %w", pragma, err)
		}
	}

	s.db = db

	// Run migrations
	if err := runMigrations(db); err != nil {
		db.Close()
		s.db = nil
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	if s.db == nil {
		return nil
	}

	err := s.db.Close()
	s.db = nil
	return err
}

// DB returns the underlying *sql.DB instance
func (s *SQLiteDB) DB() *sql.DB {
	return s.db
}
