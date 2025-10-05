package domain

import "time"

// Post represents a blog post
// A post is created from a Markdown file, and the resulting HTML is stored at HTMLPath.
// Posts become published when they are merged to main.
type Post struct {
	ID          string
	Title       string
	Snippet     string
	HTMLPath    string
	UpdatedAt   time.Time
	PublishedAt time.Time
	CreatedAt   time.Time
}

type PostRepository interface {
	UpsertPost(p *Post) error
}
