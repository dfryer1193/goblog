package service

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.yaml.in/yaml/v3"

	"github.com/dfryer1193/goblog/models"
)

// postsDir is the Content Repository folder holding Posts.
const postsDir = "posts"

// frontMatter is a Post's YAML front matter as the author wrote it.
type frontMatter struct {
	Title   string   `yaml:"title"`
	Date    string   `yaml:"date"`
	Tags    []string `yaml:"tags"`
	Summary string   `yaml:"summary"`
}

// readPosts reads and renders every Post in content's posts/ folder, deciding
// which are Drafts as of today. Posts it can't use come back as Problems.
func readPosts(content fs.FS, today models.Date) ([]*models.Post, []models.Problem, error) {
	entries, err := fs.ReadDir(content, postsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", postsDir, err)
	}

	md := goldmark.New(goldmark.WithExtensions(extension.GFM, &references{content: content}))
	var (
		posts    []*models.Post
		problems []models.Problem
	)
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".md" {
			continue
		}
		name := path.Join(postsDir, e.Name())
		post, perr := readPost(content, md, name, today)
		if perr != nil {
			problems = append(problems, models.Problem{Path: name, Err: perr})
			continue
		}
		posts = append(posts, post)
	}
	return posts, problems, nil
}

// readPost reads, validates and renders the Post at name.
func readPost(content fs.FS, md goldmark.Markdown, name string, today models.Date) (*models.Post, error) {
	src, err := fs.ReadFile(content, name)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}
	fm, body, err := splitFrontMatter(src)
	if err != nil {
		return nil, err
	}

	var meta frontMatter
	if err = yaml.Unmarshal(fm, &meta); err != nil {
		return nil, fmt.Errorf("invalid front matter: %w", err)
	}
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		return nil, errors.New("front matter has no title")
	}
	post := &models.Post{
		Path:    name,
		Slug:    strings.TrimSuffix(path.Base(name), ".md"),
		Title:   title,
		Tags:    meta.Tags,
		Summary: meta.Summary,
	}
	if post.PublishDate, err = parseDate(meta.Date); err != nil {
		return nil, err
	}
	post.Draft = post.PublishDate.IsZero() || post.PublishDate.Compare(today) > 0

	pc, refs := newReferencing(name)
	doc := md.Parser().Parse(text.NewReader(body), parser.WithContext(pc))
	if err = refs.err(); err != nil {
		return nil, err
	}
	if post.Summary == "" {
		post.Summary = firstParagraph(doc, body)
	}
	var html bytes.Buffer
	if err = md.Renderer().Render(&html, body, doc); err != nil {
		return nil, fmt.Errorf("rendering Markdown: %w", err)
	}
	post.Body = template.HTML(html.String()) //nolint:gosec // goldmark escapes the author's text and drops raw HTML
	return post, nil
}

// firstParagraph returns the plain text of the first paragraph in doc, with
// its line breaks turned into spaces. It's "" if doc has no paragraph.
func firstParagraph(doc ast.Node, src []byte) string {
	var para ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindParagraph {
			para = n
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if para == nil {
		return ""
	}

	var b strings.Builder
	_ = ast.Walk(para, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Text:
			b.Write(n.Segment.Value(src))
			if n.SoftLineBreak() || n.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(n.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}

// splitFrontMatter splits a Post into its YAML front matter, fenced by "---"
// lines at the top of the file, and its Markdown body.
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

// parseDate parses a front-matter date. An empty date is the zero Date.
func parseDate(s string) (models.Date, error) {
	if s == "" {
		return models.Date{}, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return models.Date{}, fmt.Errorf("date %q is not a YYYY-MM-DD date", s)
	}
	return models.DateOf(t), nil
}
