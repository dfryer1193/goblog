package service

import (
	"cmp"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

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
	// today is now's date in the build's timezone.
	today models.Date
	// posts are the Posts the site shows, newest first: the Published
	// Posts, plus the Drafts if the build includes them.
	posts []*models.Post
	// out collects the output files.
	out *output
	// problems are the files the build skipped so far.
	problems []models.Problem
}

// buildSteps render the site's output, in order, once the Posts are read.
// Each adds output files with b.out.page (or to b.out.files directly) and may
// report Problems.
var buildSteps = []func(b *siteBuild) error{ //nolint:gochecknoglobals // fixed list of steps
	renderIndex,
	renderPostPages,
	renderPages,
	renderTags,
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
		today: models.DateOf(now.In(tz)), out: out,
	}
	if problem != nil {
		b.problems = append(b.problems, *problem)
	}

	posts, problems, err := readPosts(content, b.today)
	if err != nil {
		return nil, err
	}
	b.problems = append(b.problems, problems...)
	b.posts = listedPosts(posts, opts.IncludeDrafts)
	slices.SortFunc(b.posts, newestFirst)

	for _, step := range buildSteps {
		if err = step(b); err != nil {
			return nil, err
		}
	}
	return &models.Site{Files: b.out.files, Problems: b.problems, Timezone: tz, Today: b.today}, nil
}

// renderIndex renders the index: every listed Post, newest first.
func renderIndex(b *siteBuild) error {
	return b.out.page("index.html", "index", pageData{Posts: b.posts})
}

// renderPostPages renders one page per listed Post, at /posts/<slug>/.
func renderPostPages(b *siteBuild) error {
	for _, p := range b.posts {
		if err := b.out.page("posts/"+p.Slug+"/index.html", "post", pageData{Title: p.Title, Post: p}); err != nil {
			return fmt.Errorf("rendering %s: %w", p.Path, err)
		}
	}
	return nil
}

// listedPosts returns the posts that the site shows: the Published Posts,
// plus the Drafts if includeDrafts is set.
func listedPosts(posts []*models.Post, includeDrafts bool) []*models.Post {
	var listed []*models.Post
	for _, p := range posts {
		if includeDrafts || !p.Draft {
			listed = append(listed, p)
		}
	}
	return listed
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
