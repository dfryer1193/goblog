package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractPostTitle(t *testing.T) {
	tests := []struct {
		name     string
		markdown []byte
		expected string
	}{
		{
			name:     "Valid title",
			markdown: []byte("# My Blog Post\nSome content"),
			expected: "My Blog Post",
		},
		{
			name:     "Title with extra spaces",
			markdown: []byte("#   Title with spaces   \nContent"),
			expected: "  Title with spaces",
		},
		{
			name:     "No title",
			markdown: []byte("Some content without title"),
			expected: "Untitled Post",
		},
		{
			name:     "Empty markdown",
			markdown: []byte(""),
			expected: "Untitled Post",
		},
		{
			name:     "Just newlines",
			markdown: []byte("\n\n"),
			expected: "Untitled Post",
		},
		{
			name:     "Missing hash symbol",
			markdown: []byte("Not a title\nContent"),
			expected: "Untitled Post",
		},
		{
			name:     "Hash without space",
			markdown: []byte("#NoSpace\nContent"),
			expected: "Untitled Post",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPostTitle(tt.markdown)
			if result != tt.expected {
				t.Errorf("extractPostTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractSnippet(t *testing.T) {
	tests := []struct {
		name     string
		markdown []byte
		expected string
	}{
		{
			name:     "Valid snippet",
			markdown: []byte("# Title\nThis is the snippet\nMore content"),
			expected: "This is the snippet",
		},
		{
			name:     "Snippet with spaces",
			markdown: []byte("# Title\n  This is a snippet with spaces  \nMore"),
			expected: "This is a snippet with spaces",
		},
		{
			name:     "Empty second line",
			markdown: []byte("# Title\n\nContent"),
			expected: "",
		},
		{
			name:     "Only one line",
			markdown: []byte("# Title"),
			expected: "",
		},
		{
			name:     "Empty markdown",
			markdown: []byte(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSnippet(tt.markdown)
			if result != tt.expected {
				t.Errorf("extractSnippet() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMarkdownRendererImpl_Render(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	renderer := NewMarkdownRenderer(tmpDir)

	tests := []struct {
		name           string
		basename       string
		markdown       []byte
		expectedTitle  string
		expectedSnip   string
		expectedHTML   string
		shouldError    bool
	}{
		{
			name:           "Basic markdown rendering",
			basename:       "test.md",
			markdown:       []byte("# Hello World\nThis is a test\n\nSome **bold** text"),
			expectedTitle:  "Hello World",
			expectedSnip:   "This is a test",
			expectedHTML:   "test.html",
			shouldError:    false,
		},
		{
			name:           "Markdown without title",
			basename:       "notitle.md",
			markdown:       []byte("Just some content\nNo title here"),
			expectedTitle:  "Untitled Post",
			expectedSnip:   "No title here",
			expectedHTML:   "notitle.html",
			shouldError:    false,
		},
		{
			name:           "Complex markdown with GFM features",
			basename:       "complex.md",
			markdown:       []byte("# Complex Post\nA snippet\n\n- [ ] Task 1\n- [x] Task 2\n\n| Col1 | Col2 |\n|------|------|\n| A    | B    |"),
			expectedTitle:  "Complex Post",
			expectedSnip:   "A snippet",
			expectedHTML:   "complex.html",
			shouldError:    false,
		},
		{
			name:           "Markdown with only title",
			basename:       "titleonly.md",
			markdown:       []byte("# Only a Title"),
			expectedTitle:  "Only a Title",
			expectedSnip:   "",
			expectedHTML:   "titleonly.html",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderer.Render(tt.basename, tt.markdown)

			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
				return
			}

			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.shouldError {
				return
			}

			if result.Title != tt.expectedTitle {
				t.Errorf("Title = %q, want %q", result.Title, tt.expectedTitle)
			}

			if result.Snippet != tt.expectedSnip {
				t.Errorf("Snippet = %q, want %q", result.Snippet, tt.expectedSnip)
			}

			if result.HTMLPath != tt.expectedHTML {
				t.Errorf("HTMLPath = %q, want %q", result.HTMLPath, tt.expectedHTML)
			}

			htmlPath := filepath.Join(tmpDir, tt.expectedHTML)
			if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
				t.Errorf("HTML file was not created at %s", htmlPath)
			}

			content, err := os.ReadFile(htmlPath)
			if err != nil {
				t.Errorf("Failed to read HTML file: %v", err)
			}
			if len(tt.markdown) > 0 && len(content) == 0 {
				t.Error("HTML file is empty")
			}
		})
	}
}

func TestMarkdownRendererImpl_Render_FileWriteError(t *testing.T) {
	invalidDir := "/nonexistent/invalid/path"
	renderer := NewMarkdownRenderer(invalidDir)

	markdown := []byte("# Test\nContent")
	_, err := renderer.Render("test.md", markdown)

	if err == nil {
		t.Error("Expected error when writing to invalid directory, got nil")
	}
}

func TestNewMarkdownRenderer(t *testing.T) {
	postDir := "/tmp/test-posts"
	renderer := NewMarkdownRenderer(postDir)

	if renderer == nil {
		t.Fatal("NewMarkdownRenderer returned nil")
	}

	impl, ok := renderer.(*MarkdownRendererImpl)
	if !ok {
		t.Fatal("NewMarkdownRenderer did not return *MarkdownRendererImpl")
	}

	if impl.postDir != postDir {
		t.Errorf("postDir = %q, want %q", impl.postDir, postDir)
	}

	if impl.renderer == nil {
		t.Error("renderer is nil")
	}
}

func TestMarkdownRendererImpl_Render_HTMLOutput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-html-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	renderer := NewMarkdownRenderer(tmpDir)

	tests := []struct {
		name           string
		markdown       []byte
		expectedInHTML []string
	}{
		{
			name:     "Bold text conversion",
			markdown: []byte("# Test\nTest\n\n**bold text**"),
			expectedInHTML: []string{
				"<strong>bold text</strong>",
			},
		},
		{
			name:     "Link conversion",
			markdown: []byte("# Test\nSnippet\n\n[Link](https://example.com)"),
			expectedInHTML: []string{
				"<a href=\"https://example.com\">Link</a>",
			},
		},
		{
			name:     "Code block conversion",
			markdown: []byte("# Test\nSnippet\n\n```go\nfunc main() {}\n```"),
			expectedInHTML: []string{
				"<code",
			},
		},
		{
			name:     "Strikethrough (GFM extension)",
			markdown: []byte("# Test\nSnippet\n\n~~strikethrough~~"),
			expectedInHTML: []string{
				"<del>strikethrough</del>",
			},
		},
		{
			name: "Table (GFM extension)",
			markdown: []byte(`# Test
Snippet

| Header1 | Header2 |
|---------|---------|
| Cell1   | Cell2   |`),
			expectedInHTML: []string{
				"<table>",
				"<thead>",
				"<tbody>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basename := "test-" + tt.name + ".md"
			result, err := renderer.Render(basename, tt.markdown)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			htmlPath := filepath.Join(tmpDir, result.HTMLPath)
			content, err := os.ReadFile(htmlPath)
			if err != nil {
				t.Fatalf("Failed to read HTML file: %v", err)
			}

			htmlStr := string(content)
			for _, expected := range tt.expectedInHTML {
				if !strings.Contains(htmlStr, expected) {
					t.Errorf("HTML does not contain expected string %q", expected)
				}
			}
		})
	}
}
