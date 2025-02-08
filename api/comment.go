package api

type Comment struct {
	ID          int       `json:"id"`
	Content     string    `json:"content"`
	AuthorEmail string    `json:"author_email"`
	CreatedAt   string    `json:"created_at"`
	Children    []Comment `json:"children"`
}

type CommentProto struct {
	PostID      string `json:"post_id"`
	AuthorEmail string `json:"author_email"`
	Content     string `json:"content"`
	InReplyToID int    `json:"in_reply_to"`
}
