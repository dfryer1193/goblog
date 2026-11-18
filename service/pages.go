package service

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.yaml.in/yaml/v3"

	"github.com/dfryer1193/goblog/models"
)

// pagesDir is the Content Repository folder holding Pages.
const pagesDir = "pages"

// pageFrontMatter is a Page's YAML front matter as the author wrote it.
// Pages need only a title; other fields are ignored.
type pageFrontMatter struct {
	Title string `yaml:"title"`
}

// readPages reads and renders each Page in the Content Repository's pages/
// folder, sorted by name. Pages it can't use come back as Problems.
func readPages(content fs.FS) ([]*models.Page, []models.Problem, error) {
	entries, err := fs.ReadDir(content, pagesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", pagesDir, err)
	}

	md := newMarkdown(content)
	var (
		pages    []*models.Page
		problems []models.Problem
	)
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != markdownExt {
			continue
		}
		name := path.Join(pagesDir, e.Name())
		p, perr := readPage(content, md, name)
		if perr != nil {
			problems = append(problems, models.Problem{Path: name, Err: perr})
			continue
		}
		pages = append(pages, p)
	}
	return pages, problems, nil
}

// renderPages renders each Page at /<name>/.
func renderPages(b *siteBuild) error {
	for _, p := range b.pages {
		data := pageData{Title: p.Title, Label: p.Name + markdownExt, Page: p}
		if err := b.out.page(p.Name+"/index.html", "page", data); err != nil {
			return fmt.Errorf("rendering %s: %w", p.Path, err)
		}
	}
	return nil
}

// readPage reads, validates and renders the Page at name.
func readPage(content fs.FS, md goldmark.Markdown, name string) (*models.Page, error) {
	pageName := strings.TrimSuffix(path.Base(name), markdownExt)
	if err := checkPageName(pageName); err != nil {
		return nil, err
	}
	src, err := fs.ReadFile(content, name)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}
	fm, body, err := splitFrontMatter(src)
	if err != nil {
		return nil, err
	}
	var meta pageFrontMatter
	if err = yaml.Unmarshal(fm, &meta); err != nil {
		return nil, fmt.Errorf("invalid front matter: %w", err)
	}
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		return nil, errors.New("front matter has no title")
	}
	pc, refs := newReferencing(name)
	doc := md.Parser().Parse(text.NewReader(body), parser.WithContext(pc))
	if err = refs.err(); err != nil {
		return nil, err
	}
	var html bytes.Buffer
	if err = md.Renderer().Render(&html, body, doc); err != nil {
		return nil, fmt.Errorf("rendering Markdown: %w", err)
	}
	return &models.Page{
		Path:  name,
		Name:  pageName,
		Title: title,
		Body:  template.HTML(html.String()), //nolint:gosec // goldmark escapes the author's text and drops raw HTML
	}, nil
}

// reservedPaths are the top-level site paths that goblog itself uses, so no
// Page may take them. They're compared case-insensitively, so that the output
// also works on case-insensitive filesystems.
var reservedPaths = map[string]bool{ //nolint:gochecknoglobals // fixed set
	// Rendered from the Content Repository.
	"index.html": true, postsDir: true, tagsSection: true, "images": true,
	"feed.xml": true, "sitemap.xml": true, "404.html": true,
	// Room for the stylesheet, fonts and other static files.
	"static": true, "assets": true, "css": true, "fonts": true, "js": true,
	"favicon.ico": true, "robots.txt": true,
	// Served by goblog itself: mjolnir's probes and the webhook.
	"livez": true, "readyz": true, "startupz": true, "webhook": true,
}

// pageNamePattern is what a Page's name may look like: a single URL path
// segment that needs no escaping.
var pageNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// checkPageName returns an error if a Page called name can't live at
// /<name>/.
func checkPageName(name string) error {
	if !pageNamePattern.MatchString(name) {
		return fmt.Errorf("the name %q must start with a letter or digit and have only letters, "+
			"digits, '.', '-' and '_'", name)
	}
	if reservedPaths[strings.ToLower(name)] {
		return fmt.Errorf("/%s/ is reserved for the site's own pages", name)
	}
	return nil
}
