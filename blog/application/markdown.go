package application

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const (
	maxLength = 200
	blogURL   = "https://blog.werewolves.fyi"
)

// MarkdownProcessingResult contains the results of processing a markdown file
type MarkdownProcessingResult struct {
	Title       string
	Snippet     string
	HTMLContent []byte
}

type relativeLinkTransformer struct {
	domain string
}

func (t *relativeLinkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		link, linkOk := n.(*ast.Link)
		img, imgOk := n.(*ast.Image)
		if !linkOk && !imgOk {
			return ast.WalkContinue, nil
		}

		dest := ""
		if linkOk {
			dest = string(link.Destination)
		} else if imgOk {
			dest = string(img.Destination)
		}

		if isRelativeLink(dest) {
			destFile := path.Base(dest)
			if imgOk {
				img.Destination = []byte(t.domain + "/images/" + destFile)
			} else if linkOk {
				// Strip .md and .html extensions from links
				destFile = strings.TrimSuffix(destFile, ".md")
				destFile = strings.TrimSuffix(destFile, ".html")
				link.Destination = []byte(t.domain + "/" + destFile)
			}
		}

		return ast.WalkContinue, nil
	})
}

func isRelativeLink(dest string) bool {
	// Absolute path check
	if strings.HasPrefix(dest, "/") {
		if strings.HasPrefix(dest, "//") {
			return false
		}
		return true
	}

	if strings.HasPrefix(dest, "./") || strings.HasPrefix(dest, "../") {
		return true
	}

	if strings.Contains(dest, ":") {
		return false
	}

	return true
}

// MarkdownRenderer defines the interface for converting markdown to HTML.
type MarkdownRenderer interface {
	Render(markdown []byte) (*MarkdownProcessingResult, error)
}

type MarkdownRendererImpl struct {
	renderer goldmark.Markdown
}

func NewMarkdownRenderer() MarkdownRenderer {
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
			parser.WithASTTransformers(
				util.Prioritized(&relativeLinkTransformer{domain: blogURL}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)

	return &MarkdownRendererImpl{
		renderer: renderer,
	}
}

func (r *MarkdownRendererImpl) Render(markdown []byte) (*MarkdownProcessingResult, error) {
	title := extractPostTitle(markdown)
	snippet := extractSnippet(markdown)
	
	var buf bytes.Buffer
	err := r.renderer.Convert(markdown, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown to HTML: %w", err)
	}

	return &MarkdownProcessingResult{
		Title:       title,
		Snippet:     snippet,
		HTMLContent: buf.Bytes(),
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
