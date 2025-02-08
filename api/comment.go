package api

type Comment struct {
	ID          int       `json:"id"`
	Content     string    `json:"content"`
	AuthorEmail string    `json:"author_email"`
	CreatedAt   string    `json:"created_at"`
	Children    []Comment `json:"children"`
}
