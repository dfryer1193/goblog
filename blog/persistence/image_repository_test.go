package persistence

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/dfryer1193/goblog/blog/domain"
	_ "modernc.org/sqlite"
)

func setupTestImageDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create images table
	_, err = db.Exec(`
		CREATE TABLE images (
			path TEXT PRIMARY KEY,
			hash TEXT NOT NULL,
			updated_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create images table: %v", err)
	}

	return db
}

func TestImageRepository_SaveImage(t *testing.T) {
	db := setupTestImageDB(t)
	defer db.Close()

	repo := NewImageRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	img := &domain.Image{
		Path:      "images/test.jpg",
		Hash:      "abc123",
		Content:   []byte("fake image content"),
		UpdatedAt: now,
		CreatedAt: now,
	}

	// Test insert
	err := repo.SaveImage(ctx, img)
	if err != nil {
		t.Fatalf("Failed to insert image: %v", err)
	}

	// Test update
	img.Hash = "def456"
	img.Content = []byte("updated content")
	img.UpdatedAt = now.Add(time.Hour)
	err = repo.SaveImage(ctx, img)
	if err != nil {
		t.Fatalf("Failed to update image: %v", err)
	}

	// Verify the update
	retrieved, err := repo.GetImage(ctx, img.Path)
	if err != nil {
		t.Fatalf("Failed to get image: %v", err)
	}

	if retrieved.Hash != "def456" {
		t.Errorf("Hash = %q, want %q", retrieved.Hash, "def456")
	}
}

func TestImageRepository_GetImage(t *testing.T) {
	db := setupTestImageDB(t)
	defer db.Close()

	repo := NewImageRepository(db)
	ctx := context.Background()

	// Test getting non-existent image
	_, err := repo.GetImage(ctx, "nonexistent.jpg")
	if err == nil {
		t.Error("Expected error for non-existent image, got nil")
	}

	// Insert an image
	now := time.Now().UTC()
	img := &domain.Image{
		Path:      "images/test.png",
		Hash:      "xyz789",
		Content:   []byte("test content"),
		UpdatedAt: now,
		CreatedAt: now,
	}
	err = repo.SaveImage(ctx, img)
	if err != nil {
		t.Fatalf("Failed to insert image: %v", err)
	}

	// Test getting existing image
	retrieved, err := repo.GetImage(ctx, img.Path)
	if err != nil {
		t.Fatalf("Failed to get image: %v", err)
	}

	if retrieved.Path != img.Path {
		t.Errorf("Path = %q, want %q", retrieved.Path, img.Path)
	}
	if retrieved.Hash != img.Hash {
		t.Errorf("Hash = %q, want %q", retrieved.Hash, img.Hash)
	}
}

func TestImageRepository_DeleteImage(t *testing.T) {
	db := setupTestImageDB(t)
	defer db.Close()

	repo := NewImageRepository(db)
	ctx := context.Background()

	// Insert an image
	now := time.Now().UTC()
	img := &domain.Image{
		Path:      "images/todelete.gif",
		Hash:      "hash123",
		Content:   []byte("content to delete"),
		UpdatedAt: now,
		CreatedAt: now,
	}
	err := repo.SaveImage(ctx, img)
	if err != nil {
		t.Fatalf("Failed to insert image: %v", err)
	}

	// Delete the image
	err = repo.DeleteImage(ctx, img.Path)
	if err != nil {
		t.Fatalf("Failed to delete image: %v", err)
	}

	// Verify deletion
	_, err = repo.GetImage(ctx, img.Path)
	if err == nil {
		t.Error("Expected error for deleted image, got nil")
	}
}

func TestImageRepository_SaveImage_NilImage(t *testing.T) {
	db := setupTestImageDB(t)
	defer db.Close()

	repo := NewImageRepository(db)
	ctx := context.Background()

	err := repo.SaveImage(ctx, nil)
	if err == nil {
		t.Error("Expected error for nil image, got nil")
	}
}

func TestImageRepository_SaveImage_EmptyPath(t *testing.T) {
	db := setupTestImageDB(t)
	defer db.Close()

	repo := NewImageRepository(db)
	ctx := context.Background()

	img := &domain.Image{
		Path: "",
		Hash: "hash",
	}

	err := repo.SaveImage(ctx, img)
	if err == nil {
		t.Error("Expected error for empty path, got nil")
	}
}
