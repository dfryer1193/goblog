package domain

import (
	"context"
	"time"
)

// Image represents an image file stored from the repository
type Image struct {
	Path      string
	Hash      string
	Content   []byte
	UpdatedAt time.Time
	CreatedAt time.Time
}

type ImageRepository interface {
	// SaveImage saves an image to both filesystem and database
	SaveImage(ctx context.Context, img *Image) error
	
	// GetImage retrieves an image record from the database
	GetImage(ctx context.Context, path string) (*Image, error)
	
	// DeleteImage removes an image from both filesystem and database
	DeleteImage(ctx context.Context, path string) error
}
