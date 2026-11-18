package service

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"

	"github.com/dfryer1193/goblog/models"
)

// postsDir is the Content Repository folder holding Posts.
const postsDir = "posts"

// postFrontMatter is a Post's YAML front matter as the author wrote it.
type postFrontMatter struct {
	titleFrontMatter `yaml:",inline"`

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

	md := newMarkdown(content)
	var (
		posts    []*models.Post
		problems []models.Problem
	)
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != markdownExt {
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
	var meta postFrontMatter
	body, err := readFrontMatter(content, name, &meta)
	if err != nil {
		return nil, err
	}
	tags, err := parseTags(meta.Tags)
	if err != nil {
		return nil, err
	}
	post := &models.Post{
		Path:    name,
		Slug:    fileStem(name),
		Title:   meta.Title,
		Tags:    tags,
		Summary: meta.Summary,
	}
	if post.PublishDate, err = parseDate(meta.Date); err != nil {
		return nil, err
	}
	post.Draft = post.PublishDate.IsZero() || post.PublishDate.Compare(today) > 0

	d, err := parseMarkdown(md, name, body)
	if err != nil {
		return nil, err
	}
	if post.Summary == "" {
		post.Summary = firstParagraph(d.doc, d.src)
	}
	post.Words = countWords(d.doc, d.src)
	if post.Body, err = d.render(md); err != nil {
		return nil, err
	}
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

// countWords counts the words in doc, whose source is src: all the text it
// shows, code blocks included, split at whitespace. Inline markup doesn't
// split a word; block boundaries and line breaks do.
func countWords(doc ast.Node, src []byte) int {
	var b strings.Builder
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Type() == ast.TypeBlock {
			b.WriteByte(' ')
		}
		switch n := n.(type) {
		case *ast.Text:
			b.Write(n.Segment.Value(src))
			if n.SoftLineBreak() || n.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(n.Value)
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			lines := n.Lines()
			for i := range lines.Len() {
				line := lines.At(i)
				b.Write(line.Value(src))
			}
		}
		return ast.WalkContinue, nil
	})
	return len(strings.Fields(b.String()))
}

// parseTags trims the front-matter tags and checks that each has a URL.
func parseTags(raw []string) ([]string, error) {
	tags := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if models.TagSlug(t) == "" {
			return nil, fmt.Errorf("tag %q has no letters or digits", t)
		}
		tags = append(tags, t)
	}
	return tags, nil
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
