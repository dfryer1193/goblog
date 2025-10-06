package persistence

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/dfryer1193/goblog/blog/domain"
	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create the posts table
	_, err = db.Exec(`
		CREATE TABLE posts (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			snippet TEXT NOT NULL,
			html_path TEXT NOT NULL,
			updated_at TIMESTAMP,
			published_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create posts table: %v", err)
	}

	// Create index
	_, err = db.Exec(`
		CREATE INDEX idx_posts_published_at
		ON posts(published_at DESC)
		WHERE published_at IS NOT NULL
	`)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	return db
}

func TestNewPostRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)
	if repo == nil {
		t.Fatal("NewPostRepository returned nil")
	}
	if repo.db == nil {
		t.Error("repository db field not set correctly")
	}
}

func TestPostRepository_UpsertPost_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	post := &domain.Post{
		ID:          "001",
		Title:       "Test Post",
		Snippet:     "This is a test post",
		HTMLPath:    "/posts/001.html",
		UpdatedAt:   now,
		PublishedAt: now,
		CreatedAt:   now,
	}

	err := repo.UpsertPost(post)
	if err != nil {
		t.Fatalf("UpsertPost failed: %v", err)
	}

	// Verify the post was inserted
	retrieved, err := repo.GetPost("001")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}

	if retrieved.ID != post.ID {
		t.Errorf("ID = %v, want %v", retrieved.ID, post.ID)
	}
	if retrieved.Title != post.Title {
		t.Errorf("Title = %v, want %v", retrieved.Title, post.Title)
	}
	if retrieved.Snippet != post.Snippet {
		t.Errorf("Snippet = %v, want %v", retrieved.Snippet, post.Snippet)
	}
	if retrieved.HTMLPath != post.HTMLPath {
		t.Errorf("HTMLPath = %v, want %v", retrieved.HTMLPath, post.HTMLPath)
	}
	if !retrieved.UpdatedAt.Equal(post.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", retrieved.UpdatedAt, post.UpdatedAt)
	}
	if !retrieved.PublishedAt.Equal(post.PublishedAt) {
		t.Errorf("PublishedAt = %v, want %v", retrieved.PublishedAt, post.PublishedAt)
	}
	if !retrieved.CreatedAt.Equal(post.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", retrieved.CreatedAt, post.CreatedAt)
	}
}

func TestPostRepository_UpsertPost_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	post := &domain.Post{
		ID:          "001",
		Title:       "Original Title",
		Snippet:     "Original snippet",
		HTMLPath:    "/posts/001.html",
		UpdatedAt:   now,
		PublishedAt: now,
		CreatedAt:   now,
	}

	// Insert the post
	err := repo.UpsertPost(post)
	if err != nil {
		t.Fatalf("UpsertPost (insert) failed: %v", err)
	}

	// Update the post
	laterTime := now.Add(1 * time.Hour)
	post.Title = "Updated Title"
	post.Snippet = "Updated snippet"
	post.UpdatedAt = laterTime

	err = repo.UpsertPost(post)
	if err != nil {
		t.Fatalf("UpsertPost (update) failed: %v", err)
	}

	// Verify the post was updated
	retrieved, err := repo.GetPost("001")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}

	if retrieved.Title != "Updated Title" {
		t.Errorf("Title = %v, want %v", retrieved.Title, "Updated Title")
	}
	if retrieved.Snippet != "Updated snippet" {
		t.Errorf("Snippet = %v, want %v", retrieved.Snippet, "Updated snippet")
	}
	if !retrieved.UpdatedAt.Equal(laterTime) {
		t.Errorf("UpdatedAt = %v, want %v", retrieved.UpdatedAt, laterTime)
	}
	// CreatedAt should remain unchanged
	if !retrieved.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v (should not change on update)", retrieved.CreatedAt, now)
	}
}

func TestPostRepository_UpsertPost_UnpublishPost(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	post := &domain.Post{
		ID:          "001",
		Title:       "Published Post",
		Snippet:     "This post is published",
		HTMLPath:    "/posts/001.html",
		UpdatedAt:   now,
		PublishedAt: now,
		CreatedAt:   now,
	}

	// Insert the published post
	err := repo.UpsertPost(post)
	if err != nil {
		t.Fatalf("UpsertPost failed: %v", err)
	}

	// Unpublish the post by setting PublishedAt to zero value
	post.PublishedAt = time.Time{}
	post.UpdatedAt = now.Add(1 * time.Hour)

	err = repo.UpsertPost(post)
	if err != nil {
		t.Fatalf("UpsertPost (unpublish) failed: %v", err)
	}

	// Verify the post was unpublished
	retrieved, err := repo.GetPost("001")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}

	if !retrieved.PublishedAt.IsZero() {
		t.Errorf("PublishedAt = %v, want zero value", retrieved.PublishedAt)
	}
}

func TestPostRepository_UpsertPost_NilPost(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	err := repo.UpsertPost(nil)
	if err == nil {
		t.Error("UpsertPost should return error for nil post")
	}
}

func TestPostRepository_GetPost_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	_, err := repo.GetPost("nonexistent")
	if err == nil {
		t.Error("GetPost should return error for non-existent post")
	}
}

func TestPostRepository_GetPost_EmptyID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	_, err := repo.GetPost("")
	if err == nil {
		t.Error("GetPost should return error for empty ID")
	}
}

func TestPostRepository_ListPublishedPosts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	// Create test posts with different publish dates
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	posts := []*domain.Post{
		{
			ID:          "001",
			Title:       "First Post",
			Snippet:     "First",
			HTMLPath:    "/posts/001.html",
			UpdatedAt:   baseTime,
			PublishedAt: baseTime.Add(1 * time.Hour),
			CreatedAt:   baseTime,
		},
		{
			ID:          "002",
			Title:       "Second Post",
			Snippet:     "Second",
			HTMLPath:    "/posts/002.html",
			UpdatedAt:   baseTime,
			PublishedAt: baseTime.Add(2 * time.Hour),
			CreatedAt:   baseTime,
		},
		{
			ID:          "003",
			Title:       "Third Post",
			Snippet:     "Third",
			HTMLPath:    "/posts/003.html",
			UpdatedAt:   baseTime,
			PublishedAt: baseTime.Add(3 * time.Hour),
			CreatedAt:   baseTime,
		},
		{
			ID:          "004",
			Title:       "Unpublished Post",
			Snippet:     "Unpublished",
			HTMLPath:    "/posts/004.html",
			UpdatedAt:   baseTime,
			PublishedAt: time.Time{}, // Not published
			CreatedAt:   baseTime,
		},
	}

	// Insert all posts
	for _, post := range posts {
		err := repo.UpsertPost(post)
		if err != nil {
			t.Fatalf("UpsertPost failed: %v", err)
		}
	}

	// List published posts
	retrieved, err := repo.ListPublishedPosts(10, 0)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}

	// Should return 3 published posts, ordered by publish date descending
	if len(retrieved) != 3 {
		t.Fatalf("ListPublishedPosts returned %d posts, want 3", len(retrieved))
	}

	// Verify order (most recent first)
	if retrieved[0].ID != "003" {
		t.Errorf("First post ID = %v, want 003", retrieved[0].ID)
	}
	if retrieved[1].ID != "002" {
		t.Errorf("Second post ID = %v, want 002", retrieved[1].ID)
	}
	if retrieved[2].ID != "001" {
		t.Errorf("Third post ID = %v, want 001", retrieved[2].ID)
	}
}

func TestPostRepository_ListPublishedPosts_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	// Create 5 published posts
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 5; i++ {
		post := &domain.Post{
			ID:          fmt.Sprintf("%03d", i),
			Title:       fmt.Sprintf("Post %d", i),
			Snippet:     fmt.Sprintf("Snippet %d", i),
			HTMLPath:    fmt.Sprintf("/posts/%03d.html", i),
			UpdatedAt:   baseTime,
			PublishedAt: baseTime.Add(time.Duration(i) * time.Hour),
			CreatedAt:   baseTime,
		}
		err := repo.UpsertPost(post)
		if err != nil {
			t.Fatalf("UpsertPost failed: %v", err)
		}
	}

	// Test first page (limit 2, offset 0)
	page1, err := repo.ListPublishedPosts(2, 0)
	if err != nil {
		t.Fatalf("ListPublishedPosts page 1 failed: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("Page 1 length = %d, want 2", len(page1))
	}
	if page1[0].ID != "005" {
		t.Errorf("Page 1 first post ID = %v, want 005", page1[0].ID)
	}
	if page1[1].ID != "004" {
		t.Errorf("Page 1 second post ID = %v, want 004", page1[1].ID)
	}

	// Test second page (limit 2, offset 2)
	page2, err := repo.ListPublishedPosts(2, 2)
	if err != nil {
		t.Fatalf("ListPublishedPosts page 2 failed: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("Page 2 length = %d, want 2", len(page2))
	}
	if page2[0].ID != "003" {
		t.Errorf("Page 2 first post ID = %v, want 003", page2[0].ID)
	}
	if page2[1].ID != "002" {
		t.Errorf("Page 2 second post ID = %v, want 002", page2[1].ID)
	}

	// Test third page (limit 2, offset 4) - should have 1 post
	page3, err := repo.ListPublishedPosts(2, 4)
	if err != nil {
		t.Fatalf("ListPublishedPosts page 3 failed: %v", err)
	}
	if len(page3) != 1 {
		t.Errorf("Page 3 length = %d, want 1", len(page3))
	}
	if page3[0].ID != "001" {
		t.Errorf("Page 3 first post ID = %v, want 001", page3[0].ID)
	}
}

func TestPostRepository_ListPublishedPosts_EmptyResult(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	// List posts when there are none
	posts, err := repo.ListPublishedPosts(10, 0)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}

	if posts == nil {
		t.Error("ListPublishedPosts should return empty slice, not nil")
	}
	if len(posts) != 0 {
		t.Errorf("ListPublishedPosts returned %d posts, want 0", len(posts))
	}
}

func TestPostRepository_ListPublishedPosts_DefaultLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	// Create 15 published posts
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 15; i++ {
		post := &domain.Post{
			ID:          fmt.Sprintf("%03d", i),
			Title:       fmt.Sprintf("Post %d", i),
			Snippet:     fmt.Sprintf("Snippet %d", i),
			HTMLPath:    fmt.Sprintf("/posts/%03d.html", i),
			UpdatedAt:   baseTime,
			PublishedAt: baseTime.Add(time.Duration(i) * time.Hour),
			CreatedAt:   baseTime,
		}
		err := repo.UpsertPost(post)
		if err != nil {
			t.Fatalf("UpsertPost failed: %v", err)
		}
	}

	// Test with limit 0 (should use default of 10)
	posts, err := repo.ListPublishedPosts(0, 0)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}

	if len(posts) != 10 {
		t.Errorf("ListPublishedPosts with limit 0 returned %d posts, want 10 (default)", len(posts))
	}
}

func TestPostRepository_ListPublishedPosts_NegativeOffset(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewPostRepository(db)

	// Create a published post
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	post := &domain.Post{
		ID:          "001",
		Title:       "Test Post",
		Snippet:     "Test",
		HTMLPath:    "/posts/001.html",
		UpdatedAt:   baseTime,
		PublishedAt: baseTime,
		CreatedAt:   baseTime,
	}
	err := repo.UpsertPost(post)
	if err != nil {
		t.Fatalf("UpsertPost failed: %v", err)
	}

	// Test with negative offset (should be treated as 0)
	posts, err := repo.ListPublishedPosts(10, -5)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}

	if len(posts) != 1 {
		t.Errorf("ListPublishedPosts with negative offset returned %d posts, want 1", len(posts))
	}
}

func TestPostRepository_InterfaceCompliance(t *testing.T) {
	var _ domain.PostRepository = (*SQLitePostRepository)(nil)
}
