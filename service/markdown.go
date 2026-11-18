package service

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.yaml.in/yaml/v3"

	"github.com/dfryer1193/goblog/models"
)

// markdownExt is the extension of Post and Page files.
const markdownExt = ".md"

// fileStem is the name of the Post or Page file at name without its folder
// and extension: its slug or Page name, e.g. "picluster" for
// posts/picluster.md.
func fileStem(name string) string {
	return strings.TrimSuffix(path.Base(name), markdownExt)
}

// newMarkdown is the Markdown converter for Posts and Pages in content: GFM,
// with references resolved and code blocks highlighted.
func newMarkdown(content fs.FS) goldmark.Markdown {
	return goldmark.New(goldmark.WithExtensions(extension.GFM, &references{content: content}, highlightCode()))
}

// titleFrontMatter is the front matter every Post and Page has: a required
// title. It's all of a Page's; other fields are ignored.
type titleFrontMatter struct {
	Title string `yaml:"title"`
}

// titled is the front matter of a Post or Page, which embeds
// titleFrontMatter.
type titled interface {
	titleFields() *titleFrontMatter
}

func (m *titleFrontMatter) titleFields() *titleFrontMatter {
	return m
}

// readFrontMatter reads the Post or Page file at name, decodes its front
// matter into meta and checks that it has a title, which it trims. It
// returns the file's Markdown body.
func readFrontMatter(content fs.FS, name string, meta titled) ([]byte, error) {
	src, err := fs.ReadFile(content, name)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}
	fm, body, err := splitFrontMatter(src)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(fm, meta); err != nil {
		return nil, fmt.Errorf("invalid front matter: %w", err)
	}
	title := meta.titleFields()
	title.Title = strings.TrimSpace(title.Title)
	if title.Title == "" {
		return nil, errors.New("front matter has no title")
	}
	return body, nil
}

// splitFrontMatter splits a Post or Page into its YAML front matter, fenced
// by "---" lines at the top of the file, and its Markdown body.
func splitFrontMatter(src []byte) ([]byte, []byte, error) {
	const fence = "---"
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(src, []byte(fence+"\n"))
	if !ok {
		return nil, nil, errors.New("no front matter: the file must start with a --- line")
	}
	for offset := 0; offset <= len(rest); {
		line, _, _ := bytes.Cut(rest[offset:], []byte("\n"))
		if string(bytes.TrimRight(line, " \t")) == fence {
			bodyStart := min(offset+len(line)+1, len(rest))
			return rest[:offset], rest[bodyStart:], nil
		}
		offset += len(line) + 1
	}
	return nil, nil, errors.New("front matter has no closing --- line")
}

// document is the parsed Markdown body of a Post or Page, with its
// references resolved.
type document struct {
	// src is the Markdown the AST's segments point into.
	src []byte
	doc ast.Node
	// links are its links to Posts and Pages.
	links []siteLink
}

// parseMarkdown parses body, the Markdown of the Content Repository file at
// name, resolving its references. A reference that can't be resolved is an
// error.
func parseMarkdown(md goldmark.Markdown, name string, body []byte) (*document, error) {
	pc, refs := newReferencing(name)
	doc := md.Parser().Parse(text.NewReader(body), parser.WithContext(pc))
	if err := refs.err(); err != nil {
		return nil, err
	}
	return &document{src: body, doc: doc, links: refs.links}, nil
}

// unlinkMissing turns each of d's links to a Post or Page that isn't onSite
// into its plain text (keeping any markup inside it), and returns a Warning
// for each, as from the Content Repository file name.
func (d *document) unlinkMissing(name string, onSite map[string]bool) []models.Warning {
	var warnings []models.Warning
	for _, l := range d.links {
		if onSite[l.target] {
			continue
		}
		parent := l.node.Parent()
		for c := l.node.FirstChild(); c != nil; c = l.node.FirstChild() {
			parent.InsertBefore(parent, l.node, c)
		}
		parent.RemoveChild(parent, l.node)
		warnings = append(warnings, models.Warning{
			Path:    name,
			Message: fmt.Sprintf("link %q points at %s, which isn't published, so it's plain text", l.dest, l.target),
		})
	}
	d.links = nil
	return warnings
}

// render renders d as HTML.
func (d *document) render(md goldmark.Markdown) (template.HTML, error) {
	var html bytes.Buffer
	if err := md.Renderer().Render(&html, d.src, d.doc); err != nil {
		return "", fmt.Errorf("rendering Markdown: %w", err)
	}
	return template.HTML(html.String()), nil //nolint:gosec // goldmark escapes the author's text and drops raw HTML
}
