package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dfryer1193/goblog/blog/domain"
	"github.com/jmoiron/sqlx"
)

var _ domain.PostRepository = (*SQLitePostRepository)(nil)

// SQLitePostRepository implements domain.PostRepository using SQL database (SQLite)
type SQLitePostRepository struct {
	db *sqlx.DB
}

// NewPostRepository creates a new SQLitePostRepository from a standard sql.DB
func NewPostRepository(db *sql.DB) *SQLitePostRepository {
	return &SQLitePostRepository{
		db: sqlx.NewDb(db, "sqlite"),
	}
}

const upsertPostQuery = `
		INSERT INTO posts (id, title, snippet, html_path, updated_at, published_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			snippet = excluded.snippet,
			html_path = excluded.html_path,
			updated_at = excluded.updated_at,
			published_at = excluded.published_at,
			created_at = COALESCE(posts.created_at, excluded.created_at)
`

// UpsertPost inserts or updates a post in the database
// Uses INSERT ... ON CONFLICT for SQLite (requires SQLite 3.24.0+)
func (r *SQLitePostRepository) UpsertPost(ctx context.Context, p *domain.Post) error {
	if p == nil {
		return fmt.Errorf("post cannot be nil")
	}

	// Convert time.Time to nullable types for SQL
	// Zero time values should be stored as NULL
	var updatedAt, publishedAt, createdAt interface{}

	if !p.UpdatedAt.IsZero() {
		updatedAt = p.UpdatedAt
	}

	if !p.PublishedAt.IsZero() {
		publishedAt = p.PublishedAt
	}

	if !p.CreatedAt.IsZero() {
		createdAt = p.CreatedAt
	}

	query := upsertPostQuery

	_, err := r.db.ExecContext(ctx, query,
		p.ID,
		p.Title,
		p.Snippet,
		p.HTMLPath,
		updatedAt,
		publishedAt,
		createdAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert post: %w", err)
	}

	return nil
}

const getPostQuery = `
		SELECT id, title, snippet, html_path, updated_at, published_at, created_at
		FROM posts
		WHERE id = ?
`

// GetPost retrieves a single post by ID
func (r *SQLitePostRepository) GetPost(ctx context.Context, id string) (*domain.Post, error) {
	if id == "" {
		return nil, fmt.Errorf("post ID cannot be empty")
	}

	var row postRow
	err := r.db.GetContext(ctx, &row, getPostQuery, id)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("post not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	return row.toDomain(), nil
}

const getLatestUpdatedTimeQuery = `
		SELECT updated_at FROM posts WHERE updated_at IS NOT NULL ORDER BY updated_at DESC LIMIT 1
`

// GetLatestUpdatedTime returns the latest updated_at time across all posts
func (r *SQLitePostRepository) GetLatestUpdatedTime(ctx context.Context) (time.Time, error) {
	query := getLatestUpdatedTimeQuery

	var latestUpdated sql.NullTime
	err := r.db.GetContext(ctx, &latestUpdated, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to get latest updated time: %w", err)
	}

	if !latestUpdated.Valid {
		return time.Time{}, nil
	}

	return latestUpdated.Time, nil
}

const listPublishedPostsQuery = `
		SELECT id, title, snippet, html_path, updated_at, published_at, created_at
		FROM posts
		WHERE published_at IS NOT NULL
		ORDER BY published_at DESC
		LIMIT ? OFFSET ?
`

// ListPublishedPosts retrieves published posts ordered by publish date descending
// Only returns posts where published_at is not NULL
func (r *SQLitePostRepository) ListPublishedPosts(ctx context.Context, limit, offset int) ([]*domain.Post, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if offset < 0 {
		offset = 0
	}

	var rows []postRow
	err := r.db.SelectContext(ctx, &rows, listPublishedPostsQuery, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to list published posts: %w", err)
	}

	// Convert to domain.Post slice
	posts := make([]*domain.Post, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, row.toDomain())
	}

	return posts, nil
}

const publishPostQuery = `
		UPDATE posts
		SET published_at = ?, updated_at = ?
		WHERE id = ?
`

const unpublishPostQuery = `
		UPDATE posts
		SET published_at = NULL, updated_at = ?
		WHERE id = ?
`

// Publish sets the published_at timestamp for a post
func (r *SQLitePostRepository) Publish(ctx context.Context, postID string) error {
	if postID == "" {
		return fmt.Errorf("post ID cannot be empty")
	}

	now := time.Now().UTC()
	query := publishPostQuery
	_, err := r.db.ExecContext(ctx, query, now, now, postID)
	if err != nil {
		return fmt.Errorf("failed to publish post: %w", err)
	}

	return nil
}

// Unpublish sets the published_at timestamp to NULL for a post
func (r *SQLitePostRepository) Unpublish(ctx context.Context, postID string) error {
	if postID == "" {
		return fmt.Errorf("post ID cannot be empty")
	}

	now := time.Now().UTC()
	query := unpublishPostQuery
	_, err := r.db.ExecContext(ctx, query, now, postID)
	if err != nil {
		return fmt.Errorf("failed to unpublish post: %w", err)
	}

	return nil
}

// postRow is a private struct used to scan database rows
// It uses sql.NullTime to handle nullable timestamp fields
// and provides a method to convert to the domain.Post model
type postRow struct {
	ID          string       `db:"id"`
	Title       string       `db:"title"`
	Snippet     string       `db:"snippet"`
	HTMLPath    string       `db:"html_path"`
	UpdatedAt   sql.NullTime `db:"updated_at"`
	PublishedAt sql.NullTime `db:"published_at"`
	CreatedAt   sql.NullTime `db:"created_at"`
}

// toDomain converts a postRow to a domain.Post, handling nullable times
func (pr *postRow) toDomain() *domain.Post {
	post := &domain.Post{
		ID:       pr.ID,
		Title:    pr.Title,
		Snippet:  pr.Snippet,
		HTMLPath: pr.HTMLPath,
	}

	if pr.UpdatedAt.Valid {
		post.UpdatedAt = pr.UpdatedAt.Time
	}
	if pr.PublishedAt.Valid {
		post.PublishedAt = pr.PublishedAt.Time
	}
	if pr.CreatedAt.Valid {
		post.CreatedAt = pr.CreatedAt.Time
	}

	return post
}
