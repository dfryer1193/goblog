package service

import (
	"cmp"
	"fmt"
	"html/template"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/yuin/goldmark"

	"github.com/dfryer1193/goblog/models"
)

// BuildOptions are the site build's inputs besides the Content Repository and
// the current time. The zero value is a valid default.
type BuildOptions struct {
	// Timezone decides which day "today" is when comparing Publish Dates,
	// when site.yaml doesn't set a timezone. Nil means UTC.
	Timezone *time.Location
	// IncludeDrafts renders Drafts as well as Published Posts, marked as
	// Drafts, for previewing.
	IncludeDrafts bool
	// Commit is the full hash of the Content Repository commit being
	// rendered. Empty means unknown, e.g. a build from a local directory.
	Commit string
}

// siteBuild is the state of one site build, shared by its steps.
type siteBuild struct {
	// content is the Content Repository.
	content fs.FS
	// now is the time of the build.
	now  time.Time
	opts BuildOptions
	// settings are the site settings from site.yaml.
	settings models.SiteSettings
	// tz is the build's timezone, which decides when Publish Dates arrive.
	tz *time.Location
	// today is now's date in the build's timezone.
	today models.Date
	// posts are the Posts the site shows, newest first: the Published
	// Posts, plus the Drafts if the build includes them.
	posts []*models.Post
	// pages are the Pages, sorted by name.
	pages []*models.Page
	// tags are the tags of posts, sorted by slug.
	tags []*models.Tag
	// md is the Markdown converter for Posts and Pages.
	md goldmark.Markdown
	// docs are the parsed Markdown of the Posts and Pages, by Content
	// Repository path, e.g. posts/x.md.
	docs map[string]*document
	// out collects the output files.
	out *output
	// problems are the files the build skipped so far.
	problems []models.Problem
	// warnings are what the build worked around so far.
	warnings []models.Warning
}

// buildSteps render the site's output, in order, once the Posts are read.
// Each adds output files with b.out.page (or to b.out.files directly) and may
// report Problems.
var buildSteps = []func(b *siteBuild) error{ //nolint:gochecknoglobals // fixed list of steps
	renderBodies,
	renderIndex,
	renderPostPages,
	renderPages,
	renderTags,
	renderFeed,
	renderSitemap,
	renderNotFound,
	copyImages,
}

// Build renders the whole site from the Content Repository files in content,
// as of now.
//
// It reads only content and does no other I/O. Files the build has to skip,
// such as a Post with invalid front matter, are reported in the returned
// Site's Problems while the rest of the site still builds. An error means the
// site as a whole couldn't be built.
func Build(content fs.FS, now time.Time, opts BuildOptions) (*models.Site, error) {
	settings, problem, err := readSettings(content)
	if err != nil {
		return nil, err
	}
	tz := cmp.Or(settings.Timezone, opts.Timezone, time.UTC)
	out, err := newOutput(&settings)
	if err != nil {
		return nil, err
	}
	b := &siteBuild{
		content: content, now: now, opts: opts, settings: settings,
		tz: tz, today: models.DateOf(now.In(tz)), out: out,
		md: newMarkdown(content), docs: map[string]*document{},
	}
	if problem != nil {
		b.problems = append(b.problems, *problem)
	}

	posts, err := b.readPosts()
	if err != nil {
		return nil, err
	}
	b.posts = publishedPosts(posts)
	if opts.IncludeDrafts {
		b.posts = posts
	}
	slices.SortFunc(b.posts, newestFirst)
	b.tags = collectTags(b.posts)

	if b.pages, err = b.readPages(); err != nil {
		return nil, err
	}
	b.out.tree = makeTree(b.posts, b.tags, b.pages, b.out.feed)
	b.out.build = newBuildInfo(now, tz, settings, opts.Commit)

	for _, step := range buildSteps {
		if err = step(b); err != nil {
			return nil, err
		}
	}
	return &models.Site{
		Files: b.out.files, Problems: b.problems, Warnings: b.warnings, Timezone: tz, Today: b.today,
		URL: settings.URL,
	}, nil
}

// renderBodies renders the Markdown of every Post and Page the site shows.
// A link to a Post or Page that the site doesn't show, such as a Draft (in a
// build that doesn't include them) or a file skipped for a Problem, is
// rendered as its plain text, with a Warning.
func renderBodies(b *siteBuild) error {
	onSite := map[string]bool{}
	for _, p := range b.posts {
		onSite[p.Path] = true
	}
	for _, p := range b.pages {
		onSite[p.Path] = true
	}
	render := func(name string) (template.HTML, error) {
		d := b.docs[name]
		b.warnings = append(b.warnings, d.unlinkMissing(name, onSite)...)
		body, err := d.render(b.md)
		if err != nil {
			return "", fmt.Errorf("rendering %s: %w", name, err)
		}
		return body, nil
	}
	var err error
	for _, p := range b.posts {
		if p.Body, err = render(p.Path); err != nil {
			return err
		}
	}
	for _, p := range b.pages {
		if p.Body, err = render(p.Path); err != nil {
			return err
		}
	}
	return nil
}

// renderIndex renders the index: every listed Post, newest first.
func renderIndex(b *siteBuild) error {
	return b.out.page(models.IndexFile, "index", pageData{Label: postsWindow, WindowTitle: "index", Posts: b.posts})
}

// renderPostPages renders one page per listed Post, at /posts/<slug>/.
func renderPostPages(b *siteBuild) error {
	for _, p := range b.posts {
		if err := b.out.page(indexFileFor(p.URL()), "post", pageData{
			Title: p.Title, Label: p.Slug + markdownExt, WindowTitle: p.Slug + markdownExt, Post: p,
		}); err != nil {
			return fmt.Errorf("rendering %s: %w", p.Path, err)
		}
	}
	return nil
}

// renderNotFound renders /404.html, the page for paths the site doesn't have.
func renderNotFound(b *siteBuild) error {
	return b.out.page(models.NotFoundFile, "404", pageData{Title: "not found", Label: "[no name]", NotFound: true})
}

// publishedPosts returns the Published Posts among posts, in the same order:
// every Post that isn't a Draft.
func publishedPosts(posts []*models.Post) []*models.Post {
	var published []*models.Post
	for _, p := range posts {
		if !p.Draft {
			published = append(published, p)
		}
	}
	return published
}

// newestFirst orders Posts by Publish Date, newest first, then by filename so
// that the order never changes between builds. Posts with no Publish Date
// (Drafts) come before all others.
func newestFirst(a, b *models.Post) int {
	if a.PublishDate.IsZero() != b.PublishDate.IsZero() {
		if a.PublishDate.IsZero() {
			return -1
		}
		return 1
	}
	if c := b.PublishDate.Compare(a.PublishDate); c != 0 {
		return c
	}
	return strings.Compare(a.Path, b.Path)
}
