package persistence

import (
	"database/sql"
	"fmt"

	"github.com/dfryer1193/goblog/blog/domain"
	"github.com/jmoiron/sqlx"
)

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
func (r *SQLitePostRepository) UpsertPost(p *domain.Post) error {
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

	_, err := r.db.Exec(query,
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
func (r *SQLitePostRepository) GetPost(id string) (*domain.Post, error) {
	if id == "" {
		return nil, fmt.Errorf("post ID cannot be empty")
	}

	query := getPostQuery

	// Use intermediate struct with NullTime for scanning
	var row struct {
		ID          string       `db:"id"`
		Title       string       `db:"title"`
		Snippet     string       `db:"snippet"`
		HTMLPath    string       `db:"html_path"`
		UpdatedAt   sql.NullTime `db:"updated_at"`
		PublishedAt sql.NullTime `db:"published_at"`
		CreatedAt   sql.NullTime `db:"created_at"`
	}

	err := r.db.Get(&row, query, id)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("post not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	// Convert to domain.Post
	post := &domain.Post{
		ID:       row.ID,
		Title:    row.Title,
		Snippet:  row.Snippet,
		HTMLPath: row.HTMLPath,
	}

	if row.UpdatedAt.Valid {
		post.UpdatedAt = row.UpdatedAt.Time
	}
	if row.PublishedAt.Valid {
		post.PublishedAt = row.PublishedAt.Time
	}
	if row.CreatedAt.Valid {
		post.CreatedAt = row.CreatedAt.Time
	}

	return post, nil
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
func (r *SQLitePostRepository) ListPublishedPosts(limit, offset int) ([]*domain.Post, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if offset < 0 {
		offset = 0
	}

	query := listPublishedPostsQuery

	// Use intermediate struct with NullTime for scanning
	var rows []struct {
		ID          string       `db:"id"`
		Title       string       `db:"title"`
		Snippet     string       `db:"snippet"`
		HTMLPath    string       `db:"html_path"`
		UpdatedAt   sql.NullTime `db:"updated_at"`
		PublishedAt sql.NullTime `db:"published_at"`
		CreatedAt   sql.NullTime `db:"created_at"`
	}

	err := r.db.Select(&rows, query, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to list published posts: %w", err)
	}

	// Convert to domain.Post slice
	posts := make([]*domain.Post, 0, len(rows))
	for _, row := range rows {
		post := &domain.Post{
			ID:       row.ID,
			Title:    row.Title,
			Snippet:  row.Snippet,
			HTMLPath: row.HTMLPath,
		}

		if row.UpdatedAt.Valid {
			post.UpdatedAt = row.UpdatedAt.Time
		}
		if row.PublishedAt.Valid {
			post.PublishedAt = row.PublishedAt.Time
		}
		if row.CreatedAt.Valid {
			post.CreatedAt = row.CreatedAt.Time
		}

		posts = append(posts, post)
	}

	return posts, nil
}
