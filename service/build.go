package service

import (
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
	// Timezone decides which day "today" is when comparing Publish Dates.
	// Nil means UTC.
	Timezone *time.Location
}

// Build renders the whole site from the Content Repository files in content,
// as of now.
//
// It reads only content and does no other I/O. Files the build has to skip,
// such as a Post with invalid front matter, are reported in the returned
// Site's Problems while the rest of the site still builds. An error means the
// site as a whole couldn't be built.
func Build(content fs.FS, now time.Time, opts BuildOptions) (*models.Site, error) {
	tz := opts.Timezone
	if tz == nil {
		tz = time.UTC
	}
	today := models.DateOf(now.In(tz))

	posts, problems, err := readPosts(content, today)
	if err != nil {
		return nil, err
	}
	listed := publishedPosts(posts)
	slices.SortFunc(listed, newestFirst)

	out, err := newOutput()
	if err != nil {
		return nil, err
	}
	if err = out.page("index.html", "index", pageData{Posts: listed}); err != nil {
		return nil, err
	}
	for _, p := range listed {
		if err = out.page("posts/"+p.Slug+"/index.html", "post", pageData{Title: p.Title, Post: p}); err != nil {
			return nil, fmt.Errorf("rendering %s: %w", p.Path, err)
		}
	}

	return &models.Site{Files: out.files, Problems: problems}, nil
}

// publishedPosts returns the posts that the site lists.
func publishedPosts(posts []*models.Post) []*models.Post {
	var listed []*models.Post
	for _, p := range posts {
		if !p.Draft {
			listed = append(listed, p)
		}
	}
	return listed
}

// newestFirst orders Posts by Publish Date, newest first, then by filename so
// that the order never changes between builds.
func newestFirst(a, b *models.Post) int {
	if c := b.PublishDate.Compare(a.PublishDate); c != 0 {
		return c
	}
	return strings.Compare(a.Path, b.Path)
}
