package models

import (
	"fmt"
	"html/template"
	"time"
)

// Date is a calendar date with no time of day or timezone, such as a Publish
// Date. The zero Date means "no date".
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the calendar date of t in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// IsZero reports whether d is the zero Date, i.e. no date at all.
func (d Date) IsZero() bool {
	return d == Date{}
}

// Compare returns -1 if d is before other, 0 if they're the same day and +1
// if d is after other.
func (d Date) Compare(other Date) int {
	return d.time().Compare(other.time())
}

// Display formats d the way the site shows dates: YYYY.MM.DD.
func (d Date) Display() string {
	return d.time().Format("2006.01.02")
}

// String formats d as YYYY-MM-DD, the front-matter format.
func (d Date) String() string {
	return d.time().Format(time.DateOnly)
}

func (d Date) time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// Post is one Post from the Content Repository's posts/ folder, with its
// front matter parsed and its Markdown rendered.
type Post struct {
	// Path is the Post's file in the Content Repository, e.g. posts/picluster.md.
	Path string
	// Slug is the filename without .md; the Post lives at /posts/<Slug>/.
	Slug string
	// Title is the required front-matter title.
	Title string
	// PublishDate is the front-matter date. It's zero when the Post has none,
	// which makes it a Draft.
	PublishDate Date
	// Tags are the front-matter tags, in the order the author wrote them.
	Tags []string
	// Summary is the front-matter summary, or the plain text of the Post's
	// first paragraph when it has none.
	Summary string
	// Draft reports whether the Post is a Draft at the time of the build.
	Draft bool
	// Body is the Post's rendered Markdown.
	Body template.HTML
	// Words is how many words the Post's Markdown has: everything it shows,
	// code included, split at whitespace.
	Words int
}

// URL is the Post's site path, e.g. /posts/picluster/.
func (p *Post) URL() string {
	return PostURL(p.Slug)
}

// PostURL is the site path of the Post with the given slug, e.g.
// /posts/picluster/. It's the one place Post URLs are made.
func PostURL(slug string) string {
	return "/" + PostsSection + "/" + slug + "/"
}

// Problem is something wrong with one file in the Content Repository that
// made the site build skip it. The rest of the site still builds. Compare
// Warning, for something the build works around.
type Problem struct {
	// Path is the offending file in the Content Repository, e.g. posts/x.md.
	Path string
	// Err says what's wrong.
	Err error
}

// String formats the Problem as "<path>: <what's wrong>".
func (p Problem) String() string {
	return fmt.Sprintf("%s: %v", p.Path, p.Err)
}

// Page is one standalone Page from the Content Repository's pages/ folder,
// such as About. It has no Publish Date and isn't listed with the Posts.
type Page struct {
	// Path is the Page's file in the Content Repository, e.g. pages/about.md.
	Path string
	// Name is the filename without .md; the Page lives at /<Name>/.
	Name string
	// Title is the required front-matter title.
	Title string
	// Body is the Page's rendered Markdown.
	Body template.HTML
}

// URL is the Page's site path, e.g. /about/.
func (p *Page) URL() string {
	return PageURL(p.Name)
}

// PageURL is the site path of the Page with the given name, e.g. /about/.
// It's the one place Page URLs are made.
func PageURL(name string) string {
	return "/" + name + "/"
}

// Warning is something wrong with one file in the Content Repository that
// the site build worked around: unlike a Problem, the file is still on the
// site. For example, a link to a Post that isn't published is rendered as
// plain text.
type Warning struct {
	// Path is the file in the Content Repository, e.g. posts/x.md.
	Path string
	// Message says what's wrong and what the build did about it.
	Message string
}

// String formats the Warning as "<path>: <message>".
func (w Warning) String() string {
	return w.Path + ": " + w.Message
}
