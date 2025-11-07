package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dfryer1193/goblog/blog/domain"
	"github.com/dfryer1193/goblog/shared/db"
)

var _ domain.ImageRepository = (*SQLiteImageRepository)(nil)

const imageDir = "./images"

// SQLiteImageRepository implements domain.ImageRepository using SQL database (SQLite)
type SQLiteImageRepository struct {
	db *sql.DB
}

// NewImageRepository creates a new SQLiteImageRepository from a standard sql.DB
func NewImageRepository(sqlDB *sql.DB) *SQLiteImageRepository {
	return &SQLiteImageRepository{
		db: sqlDB,
	}
}

const upsertImageQuery = `
	INSERT INTO images (path, hash, updated_at, created_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		hash = excluded.hash,
		updated_at = excluded.updated_at,
		created_at = COALESCE(images.created_at, excluded.created_at)
`

// SaveImage saves an image to both filesystem and database within a transaction
func (r *SQLiteImageRepository) SaveImage(ctx context.Context, img *domain.Image) error {
	if img == nil {
		return fmt.Errorf("image cannot be nil")
	}

	if img.Path == "" {
		return fmt.Errorf("image path cannot be empty")
	}

	// Run filesystem and database operations in a transaction
	return db.RunInTransaction(ctx, r.db, func(txCtx context.Context) error {
		// Upsert to database first
		var updatedAt, createdAt any

		if !img.UpdatedAt.IsZero() {
			updatedAt = img.UpdatedAt
		}

		if !img.CreatedAt.IsZero() {
			createdAt = img.CreatedAt
		}

		executor := db.GetExecutor(txCtx, r.db)
		_, err := executor.ExecContext(txCtx, upsertImageQuery,
			img.Path,
			img.Hash,
			updatedAt,
			createdAt,
		)

		if err != nil {
			return fmt.Errorf("failed to upsert image record: %w", err)
		}

		// Then write to filesystem - if this fails, transaction rolls back
		if err := os.MkdirAll(imageDir, 0755); err != nil {
			return fmt.Errorf("failed to create image directory: %w", err)
		}

		filename := filepath.Base(img.Path)
		localPath := filepath.Join(imageDir, filename)

		if err := os.WriteFile(localPath, img.Content, 0644); err != nil {
			return fmt.Errorf("failed to write image file: %w", err)
		}

		return nil
	})
}

const getImageQuery = `
	SELECT path, hash, updated_at, created_at
	FROM images
	WHERE path = ?
`

// GetImage retrieves a single image by path
func (r *SQLiteImageRepository) GetImage(ctx context.Context, path string) (*domain.Image, error) {
	if path == "" {
		return nil, fmt.Errorf("image path cannot be empty")
	}

	var row imageRow
	err := r.db.QueryRowContext(ctx, getImageQuery, path).Scan(
		&row.Path,
		&row.Hash,
		&row.UpdatedAt,
		&row.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("image not found: %s", path)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	return row.toDomain(), nil
}

const deleteImageQuery = `
	DELETE FROM images WHERE path = ?
`

// DeleteImage removes an image from both filesystem and database within a transaction
func (r *SQLiteImageRepository) DeleteImage(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("image path cannot be empty")
	}

	// Run database and filesystem operations in a transaction
	return db.RunInTransaction(ctx, r.db, func(txCtx context.Context) error {
		// Delete from database first
		executor := db.GetExecutor(txCtx, r.db)
		_, err := executor.ExecContext(txCtx, deleteImageQuery, path)
		if err != nil {
			return fmt.Errorf("failed to delete image record: %w", err)
		}

		// Then remove from filesystem - if this fails, transaction rolls back
		filename := filepath.Base(path)
		localPath := filepath.Join(imageDir, filename)

		if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove image file: %w", err)
		}

		return nil
	})
}

// imageRow is a private struct used to scan database rows
type imageRow struct {
	Path      string       `db:"path"`
	Hash      string       `db:"hash"`
	UpdatedAt sql.NullTime `db:"updated_at"`
	CreatedAt sql.NullTime `db:"created_at"`
}

// toDomain converts an imageRow to a domain.Image, handling nullable times
func (ir *imageRow) toDomain() *domain.Image {
	img := &domain.Image{
		Path: ir.Path,
		Hash: ir.Hash,
	}

	if ir.UpdatedAt.Valid {
		img.UpdatedAt = ir.UpdatedAt.Time
	}
	if ir.CreatedAt.Valid {
		img.CreatedAt = ir.CreatedAt.Time
	}

	return img
}
