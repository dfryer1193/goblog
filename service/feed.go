package service

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dfryer1193/goblog/models"
)

// feedFile is the Atom feed's output file, at /feed.xml.
const feedFile = "feed.xml"

// atomNS is the Atom 1.0 (RFC 4287) XML namespace.
const atomNS = "http://www.w3.org/2005/Atom"

type atomFeed struct {
	XMLName  xml.Name    `xml:"feed"`
	NS       string      `xml:"xmlns,attr"`
	ID       string      `xml:"id"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle,omitempty"`
	Updated  string      `xml:"updated"`
	Author   atomPerson  `xml:"author"`
	Links    []atomLink  `xml:"link"`
	Entries  []atomEntry `xml:"entry"`
}

type atomPerson struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	ID         string         `xml:"id"`
	Title      string         `xml:"title"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Links      []atomLink     `xml:"link"`
	Categories []atomCategory `xml:"category"`
	Summary    string         `xml:"summary,omitempty"`
	Content    atomText       `xml:"content"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

type atomText struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

// renderFeed renders /feed.xml, an Atom feed of the Published Posts, newest
// first, with their full content. Drafts are never in it, even when the build
// includes them. It needs the base URL, so without one there's no feed.
func renderFeed(b *siteBuild) error {
	if b.settings.URL == "" {
		return nil
	}
	f := atomFeed{
		NS:       atomNS,
		ID:       b.absURL("/"),
		Title:    b.settings.Title,
		Subtitle: b.settings.Description,
		// Atom needs an author; the site's title stands in for a blog with
		// no named author.
		Author: atomPerson{Name: cmp.Or(b.settings.Author, b.settings.Title)},
		Links: []atomLink{
			{Rel: "self", Type: "application/atom+xml", Href: b.absURL("/" + feedFile)},
			{Rel: "alternate", Type: "text/html", Href: b.absURL("/")},
		},
		// With no Posts, the feed last changed when the site was built.
		Updated: b.now.In(b.tz).Format(time.RFC3339),
	}
	for _, p := range publishedPosts(b.posts) {
		updated := b.publishTime(p.PublishDate)
		if len(f.Entries) == 0 {
			f.Updated = updated
		}
		e := atomEntry{
			ID:        b.absURL(p.URL()),
			Title:     p.Title,
			Published: updated,
			Updated:   updated,
			Links:     []atomLink{{Rel: "alternate", Type: "text/html", Href: b.absURL(p.URL())}},
			Summary:   p.Summary,
			Content:   atomText{Type: "html", Text: b.absoluteLinks(string(p.Body))},
		}
		for _, tag := range p.Tags {
			e.Categories = append(e.Categories, atomCategory{Term: tag})
		}
		f.Entries = append(f.Entries, e)
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(f); err != nil {
		return fmt.Errorf("rendering %s: %w", feedFile, err)
	}
	buf.WriteByte('\n')
	b.out.files[feedFile] = buf.Bytes()
	return nil
}

// publishedPosts returns the Published Posts among posts, in the same order.
func publishedPosts(posts []*models.Post) []*models.Post {
	return listedPosts(posts, false)
}

// publishTime is the moment a Post with Publish Date d is published:
// midnight at the start of d in the build's timezone, in RFC 3339.
func (b *siteBuild) publishTime(d models.Date) string {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, b.tz).Format(time.RFC3339)
}

// absURL turns the site path p, e.g. /posts/x/, into an absolute URL under
// the base URL, escaping it as needed.
func (b *siteBuild) absURL(p string) string {
	return b.settings.URL + (&url.URL{Path: p}).EscapedPath()
}

// siteRelativeAttr matches an href or src attribute holding a site-absolute
// path such as /posts/x/ (but not a //host URL) in a rendered Post. Goldmark
// writes every attribute in double quotes and escapes quotes in text, and it
// drops raw HTML, so this only ever matches the attributes it wrote.
var siteRelativeAttr = regexp.MustCompile(`\s(?:href|src)="/(?:[^/"][^"]*)?"`)

// absoluteLinks rewrites the site-absolute links and images in a rendered
// Post's HTML to absolute URLs, so that they work in a feed reader. Links to
// other sites, fragments and the like are left alone. The paths are already
// escaped.
func (b *siteBuild) absoluteLinks(html string) string {
	return siteRelativeAttr.ReplaceAllStringFunc(html, func(attr string) string {
		i := strings.IndexByte(attr, '"') + 1
		return attr[:i] + b.settings.URL + attr[i:]
	})
}
