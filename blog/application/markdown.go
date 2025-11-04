package application

import (
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// MarkdownProcessingResult contains the results of processing a markdown file
type MarkdownProcessingResult struct {
	Title    string
	Snippet  string
	HTMLPath string
}

// MarkdownRenderer defines the interface for converting markdown to HTML.
type MarkdownRenderer interface {
	Render(basename string, markdown []byte) (*MarkdownProcessingResult, error)
}

type MarkdownRendererImpl struct {
	postDir  string
	renderer goldmark.Markdown
}

func NewMarkdownRenderer(postDir string) MarkdownRenderer {
	// TODO: Implement custom domains for relative links
	renderer := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)

	return &MarkdownRendererImpl{
		postDir:  postDir,
		renderer: renderer,
	}
}

func (r *MarkdownRendererImpl) Render(basename string, markdown []byte) (*MarkdownProcessingResult, error) {
	title := extractPostTitle(markdown)
	snippet := extractSnippet(markdown)
	htmlBasename := strings.Replace(basename, ".md", ".html", 1)
	postPath := r.postDir + "/" + htmlBasename

	file, err := os.Create(postPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	r.renderer.Convert(markdown, file)
	err = file.Sync()
	if err != nil {
		return nil, err
	}

	return &MarkdownProcessingResult{
		Title:    title,
		Snippet:  snippet,
		HTMLPath: htmlBasename,
	}, nil
}

func extractPostTitle(markdown []byte) string {
	lines := strings.SplitN(string(markdown), "\n", 2)
	if len(lines) == 0 {
		return "Untitled Post"
	}

	firstLine := strings.TrimSpace(lines[0])
	title, found := strings.CutPrefix(firstLine, "# ")
	if !found {
		return "Untitled Post"
	}

	return strings.TrimSpace(title)
}

func extractSnippet(markdown []byte) string {
	const maxLength = 200

	lines := strings.Split(string(markdown), "\n")
	var paragraphLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip headings before we find content
		if strings.HasPrefix(trimmed, "#") {
			if len(paragraphLines) > 0 {
				break
			}
			continue
		}

		// Empty line handling
		if trimmed == "" {
			if len(paragraphLines) > 0 {
				break // End of first paragraph
			}
			continue
		}

		// Stop at code blocks, horizontal rules, lists, tables
		if strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "---") ||
			strings.HasPrefix(trimmed, "***") ||
			strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "+ ") ||
			strings.HasPrefix(trimmed, "|") {
			if len(paragraphLines) > 0 {
				break
			}
			continue
		}

		// Collect paragraph content
		paragraphLines = append(paragraphLines, trimmed)
	}

	if len(paragraphLines) == 0 {
		return ""
	}

	snippet := strings.Join(paragraphLines, " ")

	// Truncate if too long
	if len(snippet) > maxLength {
		snippet = snippet[:maxLength]
		if lastSpace := strings.LastIndexAny(snippet, " \t"); lastSpace > 0 {
			snippet = snippet[:lastSpace]
		}
		snippet += "..."
	}

	return snippet
}
