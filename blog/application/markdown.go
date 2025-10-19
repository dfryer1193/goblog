package application

import (
	"bytes"

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

// TODO: Use yuin/goldmark for markdown rendering

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
	var buf bytes.Buffer
	r.renderer.Convert(markdown, &buf)

	// TODO: pass filename in and convert it to html
	// TODO: Write html content to disk
	// TODO: Extract title
	// TODO: extract snippet
	// TODO: build & return MarkdownProcessingResult

	return nil, nil
}
