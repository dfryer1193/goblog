package service

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"

	"github.com/dfryer1193/goblog/models"
)

// pagesDir is the Content Repository folder holding Pages.
const pagesDir = "pages"

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
		data := pageData{Title: p.Title, Label: p.Name + markdownExt, WindowTitle: p.Name + markdownExt, Page: p}
		if err := b.out.page(indexFileFor(p.URL()), "page", data); err != nil {
			return fmt.Errorf("rendering %s: %w", p.Path, err)
		}
	}
	return nil
}

// readPage reads, validates and renders the Page at name. A Page's front
// matter needs only a title; other fields are ignored.
func readPage(content fs.FS, md goldmark.Markdown, name string) (*models.Page, error) {
	pageName := fileStem(name)
	if err := checkPageName(pageName); err != nil {
		return nil, err
	}
	var meta titleFrontMatter
	body, err := readFrontMatter(content, name, &meta)
	if err != nil {
		return nil, err
	}
	d, err := parseMarkdown(md, name, body)
	if err != nil {
		return nil, err
	}
	page := &models.Page{Path: name, Name: pageName, Title: meta.Title}
	if page.Body, err = d.render(md); err != nil {
		return nil, err
	}
	return page, nil
}

// reservedPaths are the top-level site paths that goblog itself uses, so no
// Page may take them. They're compared case-insensitively, so that the output
// also works on case-insensitive filesystems.
var reservedPaths = map[string]bool{ //nolint:gochecknoglobals // fixed set
	// Rendered from the Content Repository.
	"index.html": true, postsDir: true, tagsSection: true, "images": true,
	"feed.xml": true, "sitemap.xml": true, "404.html": true,
	// The stylesheet, fonts and their licences, and room for more.
	"static": true, assetsSection: true, "css": true, "fonts": true, "js": true, licensesSection: true,
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
