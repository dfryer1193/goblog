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
			expected: "Title with spaces",
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
			name:     "First paragraph after title",
			markdown: []byte("# Title\nThis is the first paragraph\n\nMore content"),
			expected: "This is the first paragraph",
		},
		{
			name:     "Multi-line first paragraph",
			markdown: []byte("# Title\nFirst line of paragraph.\nSecond line of paragraph.\n\nSecond paragraph"),
			expected: "First line of paragraph. Second line of paragraph.",
		},
		{
			name:     "Skip empty lines after title",
			markdown: []byte("# Title\n\n\nThis is the content after blank lines"),
			expected: "This is the content after blank lines",
		},
		{
			name:     "Multiple headings",
			markdown: []byte("# Title\n## Subtitle\nFirst paragraph content"),
			expected: "First paragraph content",
		},
		{
			name:     "Stop at code block",
			markdown: []byte("# Title\nFirst paragraph\n```\ncode\n```"),
			expected: "First paragraph",
		},
		{
			name:     "Stop at list",
			markdown: []byte("# Title\nIntro text\n- List item"),
			expected: "Intro text",
		},
		{
			name:     "Stop at horizontal rule",
			markdown: []byte("# Title\nContent before rule\n---\nAfter"),
			expected: "Content before rule",
		},
		{
			name:     "Stop at table",
			markdown: []byte("# Title\nIntro\n| Col1 | Col2 |"),
			expected: "Intro",
		},
		{
			name:     "Truncate long paragraph",
			markdown: []byte("# Title\nThis is a very long paragraph that exceeds the maximum length limit and should be truncated at a word boundary to ensure that the snippet looks clean and professional without cutting words in the middle which would look unprofessional."),
			expected: "This is a very long paragraph that exceeds the maximum length limit and should be truncated at a word boundary to ensure that the snippet looks clean and professional without cutting words in the...",
		},
		{
			name:     "Only title, no content",
			markdown: []byte("# Title"),
			expected: "",
		},
		{
			name:     "Empty markdown",
			markdown: []byte(""),
			expected: "",
		},
		{
			name:     "No title, direct content",
			markdown: []byte("This is content without a title.\nSecond line."),
			expected: "This is content without a title. Second line.",
		},
		{
			name:     "Paragraph with inline formatting",
			markdown: []byte("# Title\nThis has **bold** and *italic* text."),
			expected: "This has **bold** and *italic* text.",
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
		name          string
		basename      string
		markdown      []byte
		expectedTitle string
		expectedSnip  string
		expectedHTML  string
		shouldError   bool
	}{
		{
			name:          "Basic markdown rendering",
			basename:      "test.md",
			markdown:      []byte("# Hello World\nThis is a test paragraph.\n\nSome **bold** text"),
			expectedTitle: "Hello World",
			expectedSnip:  "This is a test paragraph.",
			expectedHTML:  "test.html",
			shouldError:   false,
		},
		{
			name:          "Markdown without title",
			basename:      "notitle.md",
			markdown:      []byte("Just some content here.\nMore content on line two."),
			expectedTitle: "Untitled Post",
			expectedSnip:  "Just some content here. More content on line two.",
			expectedHTML:  "notitle.html",
			shouldError:   false,
		},
		{
			name:          "Complex markdown with GFM features",
			basename:      "complex.md",
			markdown:      []byte("# Complex Post\nThis is my introduction paragraph.\n\n- [ ] Task 1\n- [x] Task 2\n\n| Col1 | Col2 |\n|------|------|\n| A    | B    |"),
			expectedTitle: "Complex Post",
			expectedSnip:  "This is my introduction paragraph.",
			expectedHTML:  "complex.html",
			shouldError:   false,
		},
		{
			name:          "Multi-line paragraph",
			basename:      "multiline.md",
			markdown:      []byte("# Post Title\nFirst line of intro.\nSecond line of intro.\n\nSecond paragraph"),
			expectedTitle: "Post Title",
			expectedSnip:  "First line of intro. Second line of intro.",
			expectedHTML:  "multiline.html",
			shouldError:   false,
		},
		{
			name:          "Markdown with only title",
			basename:      "titleonly.md",
			markdown:      []byte("# Only a Title"),
			expectedTitle: "Only a Title",
			expectedSnip:  "",
			expectedHTML:  "titleonly.html",
			shouldError:   false,
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

func TestIsRelativeLink(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Absolute HTTP URL",
			url:      "http://example.com/page",
			expected: false,
		},
		{
			name:     "Absolute HTTPS URL",
			url:      "https://example.com/page",
			expected: false,
		},
		{
			name:     "Protocol-relative URL",
			url:      "//example.com/page",
			expected: false,
		},
		{
			name:     "Mailto link",
			url:      "mailto:user@example.com",
			expected: false,
		},
		{
			name:     "Tel link",
			url:      "tel:+1234567890",
			expected: false,
		},
		{
			name:     "Data URI",
			url:      "data:image/png;base64,iVBOR...",
			expected: false,
		},
		{
			name:     "JavaScript URI",
			url:      "javascript:alert('test')",
			expected: false,
		},
		{
			name:     "Absolute path",
			url:      "/about/contact",
			expected: true,
		},
		{
			name:     "Relative path with ./",
			url:      "./images/photo.jpg",
			expected: true,
		},
		{
			name:     "Relative path with ../",
			url:      "../docs/readme.md",
			expected: true,
		},
		{
			name:     "Simple filename",
			url:      "image.png",
			expected: true,
		},
		{
			name:     "Relative path",
			url:      "posts/my-post.html",
			expected: true,
		},
		{
			name:     "Empty string",
			url:      "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRelativeLink(tt.url)
			if result != tt.expected {
				t.Errorf("isRelativeLink(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestRelativeLinkTransformer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "link-transformer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	renderer := NewMarkdownRenderer(tmpDir)

	tests := []struct {
		name           string
		markdown       string
		expectedInHTML []string
		notInHTML      []string
	}{
		{
			name: "Relative link transformation",
			markdown: `# Test
Intro

[Link to about](/about)`,
			expectedInHTML: []string{
				`href="https://blog.werewolves.fyi/about"`,
			},
		},
		{
			name: "Relative image transformation",
			markdown: `# Test
Intro

![Alt text](photo.jpg)`,
			expectedInHTML: []string{
				`src="https://blog.werewolves.fyi/images/photo.jpg"`,
			},
		},
		{
			name: "Absolute link unchanged",
			markdown: `# Test
Intro

[External](https://example.com/page)`,
			expectedInHTML: []string{
				`href="https://example.com/page"`,
			},
			notInHTML: []string{
				"blog.werewolves.fyi",
			},
		},
		{
			name: "Absolute image unchanged",
			markdown: `# Test
Intro

![Image](https://cdn.example.com/image.jpg)`,
			expectedInHTML: []string{
				`src="https://cdn.example.com/image.jpg"`,
			},
		},
		{
			name: "Protocol-relative URL unchanged",
			markdown: `# Test
Intro

[Link](//example.com/page)`,
			expectedInHTML: []string{
				`href="//example.com/page"`,
			},
		},
		{
			name: "Mailto unchanged",
			markdown: `# Test
Intro

[Email](mailto:test@example.com)`,
			expectedInHTML: []string{
				`href="mailto:test@example.com"`,
			},
		},
		{
			name: "Mixed links",
			markdown: `# Test
Intro

[Relative](/contact)
[Absolute](https://google.com)
![Relative Image](logo.png)
![Absolute Image](https://example.com/img.jpg)`,
			expectedInHTML: []string{
				`href="https://blog.werewolves.fyi/contact"`,
				`href="https://google.com"`,
				`src="https://blog.werewolves.fyi/images/logo.png"`,
				`src="https://example.com/img.jpg"`,
			},
		},
		{
			name: "Relative path with directory",
			markdown: `# Test
Intro

[Link](posts/my-post.md)`,
			expectedInHTML: []string{
				`href="https://blog.werewolves.fyi/my-post.md"`,
			},
		},
		{
			name: "Relative path with parent directory",
			markdown: `# Test
Intro

[Link](../other/page.html)`,
			expectedInHTML: []string{
				`href="https://blog.werewolves.fyi/page.html"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basename := "test-" + strings.ReplaceAll(tt.name, " ", "-") + ".md"
			result, err := renderer.Render(basename, []byte(tt.markdown))
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			htmlPath := filepath.Join(tmpDir, result.HTMLPath)
			content, err := os.ReadFile(htmlPath)
			if err != nil {
				t.Fatalf("Failed to read HTML file: %v", err)
			}

			html := string(content)

			for _, expected := range tt.expectedInHTML {
				if !strings.Contains(html, expected) {
					t.Errorf("HTML does not contain expected string %q\nHTML:\n%s", expected, html)
				}
			}

			for _, notExpected := range tt.notInHTML {
				if strings.Contains(html, notExpected) && len(tt.expectedInHTML) > 0 {
					// Only fail if we're explicitly checking something shouldn't be there
					// and we have other expectations that should be there
					t.Errorf("HTML contains unexpected string %q\nHTML:\n%s", notExpected, html)
				}
			}
		})
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
