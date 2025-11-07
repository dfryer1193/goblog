package domain

import (
	"context"
	"time"
)

// Post represents a blog post
// A post is created from a Markdown file, and the resulting HTML is stored at HTMLPath.
// Posts become published when they are merged to main.
type Post struct {
	ID          string
	Title       string
	Snippet     string
	HTMLPath    string
	HTMLContent []byte
	UpdatedAt   time.Time
	PublishedAt time.Time
	CreatedAt   time.Time
}

type PostRepository interface {
	// SavePost saves a post to both filesystem and database
	SavePost(ctx context.Context, p *Post) error
	
	GetPost(ctx context.Context, id string) (*Post, error)
	GetLatestUpdatedTime(ctx context.Context) (time.Time, error)
	ListPublishedPosts(ctx context.Context, limit int, offset int) ([]*Post, error)

	Publish(ctx context.Context, postID string) error
	Unpublish(ctx context.Context, postID string) error
}
