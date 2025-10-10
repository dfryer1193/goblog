package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/dfryer1193/goblog/blog/domain"
	_ "modernc.org/sqlite"
)

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
	ctx := context.Background()

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

	err := repo.UpsertPost(ctx, post)
	if err != nil {
		t.Fatalf("UpsertPost failed: %v", err)
	}

	retrieved, err := repo.GetPost(ctx, "001")
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
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	post := &domain.Post{
		ID:        "001",
		Title:     "Original Title",
		Snippet:   "Original snippet",
		HTMLPath:  "/posts/001.html",
		UpdatedAt: now,
		CreatedAt: now,
	}

	err := repo.UpsertPost(ctx, post)
	if err != nil {
		t.Fatalf("UpsertPost (insert) failed: %v", err)
	}

	laterTime := now.Add(1 * time.Hour)
	post.Title = "Updated Title"
	post.Snippet = "Updated snippet"
	post.UpdatedAt = laterTime

	err = repo.UpsertPost(ctx, post)
	if err != nil {
		t.Fatalf("UpsertPost (update) failed: %v", err)
	}

	retrieved, err := repo.GetPost(ctx, "001")
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
	if !retrieved.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt should not change on update")
	}
}

func TestPostRepository_UpsertPost_NilPost(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	err := repo.UpsertPost(ctx, nil)
	if err == nil {
		t.Error("UpsertPost should return error for nil post")
	}
}

func TestPostRepository_GetPost_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	_, err := repo.GetPost(ctx, "nonexistent")
	if err == nil {
		t.Error("GetPost should return error for non-existent post")
	}
}

func TestPostRepository_GetPost_EmptyID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	_, err := repo.GetPost(ctx, "")
	if err == nil {
		t.Error("GetPost should return error for empty ID")
	}
}

func TestPostRepository_PublishAndUnpublish(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	post := &domain.Post{
		ID:        "001",
		Title:     "To Be Published",
		Snippet:   "A post to test publishing",
		HTMLPath:  "/posts/001.html",
		UpdatedAt: now,
		CreatedAt: now,
	}

	err := repo.UpsertPost(ctx, post)
	if err != nil {
		t.Fatalf("UpsertPost failed: %v", err)
	}

	retrieved, err := repo.GetPost(ctx, "001")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}
	if !retrieved.PublishedAt.IsZero() {
		t.Error("Post should not be published initially")
	}

	err = repo.Publish(ctx, "001")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	published, err := repo.GetPost(ctx, "001")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}
	if published.PublishedAt.IsZero() {
		t.Error("Post should be published")
	}
	if !published.UpdatedAt.After(now) {
		t.Error("UpdatedAt should be updated after publishing")
	}

	err = repo.Unpublish(ctx, "001")
	if err != nil {
		t.Fatalf("Unpublish failed: %v", err)
	}

	unpublished, err := repo.GetPost(ctx, "001")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}
	if !unpublished.PublishedAt.IsZero() {
		t.Error("Post should be unpublished")
	}
	if !unpublished.UpdatedAt.After(published.UpdatedAt) {
		t.Error("UpdatedAt should be updated after unpublishing")
	}
}

func TestPostRepository_ListPublishedPosts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	posts := []*domain.Post{
		{ID: "001", Title: "First", PublishedAt: baseTime.Add(1 * time.Hour), CreatedAt: baseTime},
		{ID: "002", Title: "Second", PublishedAt: baseTime.Add(2 * time.Hour), CreatedAt: baseTime},
		{ID: "003", Title: "Third", PublishedAt: baseTime.Add(3 * time.Hour), CreatedAt: baseTime},
		{ID: "004", Title: "Unpublished", CreatedAt: baseTime}, // Not published
	}

	for _, p := range posts {
		p.HTMLPath = "/path"
		p.Snippet = "snippet"
		err := repo.UpsertPost(ctx, p)
		if err != nil {
			t.Fatalf("UpsertPost failed: %v", err)
		}
	}

	retrieved, err := repo.ListPublishedPosts(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}
	if len(retrieved) != 3 {
		t.Fatalf("ListPublishedPosts should return 3 posts, got %d", len(retrieved))
	}

	if retrieved[0].ID != "003" {
		t.Errorf("Expected first post to be 003, got %s", retrieved[0].ID)
	}
	if retrieved[1].ID != "002" {
		t.Errorf("Expected second post to be 002, got %s", retrieved[1].ID)
	}
	if retrieved[2].ID != "001" {
		t.Errorf("Expected third post to be 001, got %s", retrieved[2].ID)
	}
}

func TestPostRepository_ListPublishedPosts_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 5; i++ {
		post := &domain.Post{
			ID:          fmt.Sprintf("%03d", i),
			Title:       fmt.Sprintf("Post %d", i),
			Snippet:     "snippet",
			HTMLPath:    "/path",
			PublishedAt: baseTime.Add(time.Duration(i) * time.Hour),
			CreatedAt:   baseTime,
		}
		err := repo.UpsertPost(ctx, post)
		if err != nil {
			t.Fatalf("UpsertPost failed: %v", err)
		}
	}

	page1, err := repo.ListPublishedPosts(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("Expected 2 posts, got %d", len(page1))
	}
	if page1[0].ID != "005" {
		t.Errorf("Expected first post to be 005, got %s", page1[0].ID)
	}
	if page1[1].ID != "004" {
		t.Errorf("Expected second post to be 004, got %s", page1[1].ID)
	}

	page2, err := repo.ListPublishedPosts(ctx, 2, 2)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("Expected 2 posts, got %d", len(page2))
	}
	if page2[0].ID != "003" {
		t.Errorf("Expected first post to be 003, got %s", page2[0].ID)
	}
	if page2[1].ID != "002" {
		t.Errorf("Expected second post to be 002, got %s", page2[1].ID)
	}

	page3, err := repo.ListPublishedPosts(ctx, 2, 4)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(page3))
	}
	if page3[0].ID != "001" {
		t.Errorf("Expected first post to be 001, got %s", page3[0].ID)
	}
}

func TestPostRepository_ListPublishedPosts_EmptyResult(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	posts, err := repo.ListPublishedPosts(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}
	if posts == nil {
		t.Fatal("ListPublishedPosts should return empty slice, not nil")
	}
	if len(posts) != 0 {
		t.Errorf("Expected 0 posts, got %d", len(posts))
	}
}

func TestPostRepository_ListPublishedPosts_DefaultLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 15; i++ {
		post := &domain.Post{
			ID:          fmt.Sprintf("%03d", i),
			Title:       fmt.Sprintf("Post %d", i),
			Snippet:     "snippet",
			HTMLPath:    "/path",
			PublishedAt: baseTime.Add(time.Duration(i) * time.Hour),
			CreatedAt:   baseTime,
		}
		err := repo.UpsertPost(ctx, post)
		if err != nil {
			t.Fatalf("UpsertPost failed: %v", err)
		}
	}

	posts, err := repo.ListPublishedPosts(ctx, 0, 0)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}
	if len(posts) != 10 {
		t.Errorf("ListPublishedPosts with limit 0 should use default of 10, got %d", len(posts))
	}
}

func TestPostRepository_ListPublishedPosts_NegativeOffset(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	post := &domain.Post{
		ID:          "001",
		Title:       "Test Post",
		Snippet:     "Test",
		HTMLPath:    "/posts/001.html",
		PublishedAt: time.Now(),
		CreatedAt:   time.Now(),
	}
	err := repo.UpsertPost(ctx, post)
	if err != nil {
		t.Fatalf("UpsertPost failed: %v", err)
	}

	posts, err := repo.ListPublishedPosts(ctx, 10, -5)
	if err != nil {
		t.Fatalf("ListPublishedPosts failed: %v", err)
	}
	if len(posts) != 1 {
		t.Errorf("ListPublishedPosts with negative offset should be treated as 0, got %d", len(posts))
	}
}

func TestPostRepository_GetLatestUpdatedTime(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewPostRepository(db)
	ctx := context.Background()

	// Test with no posts
	latestTime, err := repo.GetLatestUpdatedTime(ctx)
	if err != nil {
		t.Fatalf("GetLatestUpdatedTime failed: %v", err)
	}
	if !latestTime.IsZero() {
		t.Errorf("Expected zero time, got %v", latestTime)
	}

	// Test with one post
	time1 := time.Now().UTC().Truncate(time.Second)
	post1 := &domain.Post{ID: "001", UpdatedAt: time1, CreatedAt: time1, Title: "title", Snippet: "snippet", HTMLPath: "/path"}
	err = repo.UpsertPost(ctx, post1)
	if err != nil {
		t.Fatalf("UpsertPost failed: %v", err)
	}

	latestTime, err = repo.GetLatestUpdatedTime(ctx)
	if err != nil {
		t.Fatalf("GetLatestUpdatedTime failed: %v", err)
	}
	if !latestTime.Equal(time1) {
		t.Errorf("Expected latest time to be %v, got %v", time1, latestTime)
	}

	// Test with many posts
	time2 := time1.Add(1 * time.Hour)
	time3 := time1.Add(-1 * time.Hour)

	posts := []*domain.Post{
		{ID: "002", UpdatedAt: time2, CreatedAt: time1, Title: "title", Snippet: "snippet", HTMLPath: "/path"}, // most recent
		{ID: "003", UpdatedAt: time3, CreatedAt: time1, Title: "title", Snippet: "snippet", HTMLPath: "/path"},
	}

	for _, p := range posts {
		err := repo.UpsertPost(ctx, p)
		if err != nil {
			t.Fatalf("UpsertPost failed: %v", err)
		}
	}

	latestTime, err = repo.GetLatestUpdatedTime(ctx)
	if err != nil {
		t.Fatalf("GetLatestUpdatedTime failed: %v", err)
	}
	if !latestTime.Equal(time2) {
		t.Errorf("Expected latest time to be %v, got %v", time2, latestTime)
	}
}

func TestPostRepository_InterfaceCompliance(t *testing.T) {
	var _ domain.PostRepository = (*SQLitePostRepository)(nil)
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
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
