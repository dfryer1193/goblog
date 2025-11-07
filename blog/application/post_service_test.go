package application

import (
	"testing"
)

func TestIsPostFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Valid post file",
			path:     "posts/001-my-post.md",
			expected: true,
		},
		{
			name:     "Valid post with longer ID",
			path:     "posts/123-another-post.md",
			expected: true,
		},
		{
			name:     "Not a post - wrong directory",
			path:     "articles/001-post.md",
			expected: false,
		},
		{
			name:     "Not a post - no ID",
			path:     "posts/my-post.md",
			expected: false,
		},
		{
			name:     "Not a post - wrong extension",
			path:     "posts/001-my-post.txt",
			expected: false,
		},
		{
			name:     "Not a post - image file",
			path:     "images/photo.jpg",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPostFile(tt.path)
			if result != tt.expected {
				t.Errorf("isPostFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Valid JPEG image",
			path:     "images/photo.jpg",
			expected: true,
		},
		{
			name:     "Valid JPEG image (jpeg extension)",
			path:     "images/photo.jpeg",
			expected: true,
		},
		{
			name:     "Valid PNG image",
			path:     "images/logo.png",
			expected: true,
		},
		{
			name:     "Valid GIF image",
			path:     "images/animation.gif",
			expected: true,
		},
		{
			name:     "Valid SVG image",
			path:     "images/vector.svg",
			expected: true,
		},
		{
			name:     "Valid WebP image",
			path:     "images/modern.webp",
			expected: true,
		},
		{
			name:     "Valid AVIF image",
			path:     "images/modern.avif",
			expected: true,
		},
		{
			name:     "Image in subdirectory",
			path:     "images/subfolder/photo.jpg",
			expected: true,
		},
		{
			name:     "Not an image - wrong directory",
			path:     "photos/image.jpg",
			expected: false,
		},
		{
			name:     "Not an image - wrong extension",
			path:     "images/document.pdf",
			expected: false,
		},
		{
			name:     "Not an image - post file",
			path:     "posts/001-post.md",
			expected: false,
		},
		{
			name:     "Not an image - no extension",
			path:     "images/photo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isImageFile(tt.path)
			if result != tt.expected {
				t.Errorf("isImageFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestExtractPostID(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Single digit ID",
			path:     "posts/1-post.md",
			expected: "1",
		},
		{
			name:     "Three digit ID with leading zeros",
			path:     "posts/001-my-post.md",
			expected: "001",
		},
		{
			name:     "Large ID",
			path:     "posts/9999-post.md",
			expected: "9999",
		},
		{
			name:     "Invalid - no ID",
			path:     "posts/my-post.md",
			expected: "",
		},
		{
			name:     "Invalid - not a post file",
			path:     "images/001-image.jpg",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPostID(tt.path)
			if result != tt.expected {
				t.Errorf("extractPostID(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestCalculateHash(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected string
	}{
		{
			name:     "Empty content",
			content:  []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "Simple text",
			content:  []byte("hello world"),
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "Binary data",
			content:  []byte{0x00, 0xFF, 0x42},
			expected: "f803bec586282caafe409609aae90eb09f6d4cddb6e04431ddf76d22e7dcacd6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateHash(tt.content)
			if result != tt.expected {
				t.Errorf("calculateHash(%q) = %q, want %q", tt.content, result, tt.expected)
			}
			// Verify it's deterministic
			result2 := calculateHash(tt.content)
			if result != result2 {
				t.Errorf("calculateHash is not deterministic")
			}
		})
	}
}
