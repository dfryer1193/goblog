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

// readPages reads each Page in the Content Repository's pages/ folder,
// sorted by name, and keeps each one's Markdown in b.docs to render later.
// Pages it can't use become Problems.
func (b *siteBuild) readPages() ([]*models.Page, error) {
	entries, err := fs.ReadDir(b.content, pagesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", pagesDir, err)
	}

	var pages []*models.Page
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != markdownExt {
			continue
		}
		name := path.Join(pagesDir, e.Name())
		p, d, perr := readPage(b.content, b.md, name)
		if perr != nil {
			b.problems = append(b.problems, models.Problem{Path: name, Err: perr})
			continue
		}
		b.docs[name] = d
		pages = append(pages, p)
	}
	return pages, nil
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

// readPage reads and validates the Page at name, and parses its Markdown,
// which the caller renders once it knows which Pages are on the site. The
// Page has no Body yet. A Page's front matter needs only a title; other
// fields are ignored.
func readPage(content fs.FS, md goldmark.Markdown, name string) (*models.Page, *document, error) {
	pageName := fileStem(name)
	if err := checkPageName(pageName); err != nil {
		return nil, nil, err
	}
	var meta titleFrontMatter
	body, err := readFrontMatter(content, name, &meta)
	if err != nil {
		return nil, nil, err
	}
	d, err := parseMarkdown(md, name, body)
	if err != nil {
		return nil, nil, err
	}
	return &models.Page{Path: name, Name: pageName, Title: meta.Title}, d, nil
}

// reservedPaths are the top-level site paths that goblog itself uses, so no
// Page may take them. They're compared case-insensitively, so that the output
// also works on case-insensitive filesystems.
var reservedPaths = map[string]bool{ //nolint:gochecknoglobals // fixed set
	// Rendered from the Content Repository.
	models.IndexFile: true, models.PostsSection: true, models.TagsSection: true, models.ImagesSection: true,
	models.FeedFile: true, models.SitemapFile: true, models.NotFoundFile: true,
	// The stylesheet, fonts and their licences, and room for more.
	"static": true, models.AssetsSection: true, "css": true, models.FontsSection: true, "js": true,
	models.LicensesSection: true,
	"favicon.ico":          true, "robots.txt": true,
	// Served by goblog itself: mjolnir's probes and the webhook (api's
	// WebhookPath, which service can't import).
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
