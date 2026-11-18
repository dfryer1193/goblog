package service_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/models"
	"github.com/dfryer1193/goblog/service"
)

func TestWithNoBaseURLTheSiteHasNoFeedOrSitemapAndThatIsAProblem(t *testing.T) {
	repo := contentRepo(map[string]string{
		"site.yaml":  "title: No URL\n",
		"posts/x.md": "---\ntitle: X\ndate: 2026-01-01\n---\nBody\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	if len(site.Problems) != 1 || site.Problems[0].String() !=
		"site.yaml: has no url, so the site has no feed.xml or sitemap.xml" {
		t.Fatalf("got problems %v, want one saying site.yaml has no url", site.Problems)
	}
	assertNoFile(t, site, "feed.xml")
	assertNoFile(t, site, "sitemap.xml")
	page(t, site, "posts/x/index.html")
	if text := page(t, site, "index.html"); strings.Contains(text, "application/atom+xml") {
		t.Errorf("index links to a feed that doesn't exist:\n%s", text)
	}
}

// atomFeed is the part of an Atom feed the tests check.
type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Author  string      `xml:"author>name"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	ID      string     `xml:"id"`
	Title   string     `xml:"title"`
	Updated string     `xml:"updated"`
	Links   []atomLink `xml:"link"`
	Content struct {
		Type string `xml:"type,attr"`
		Text string `xml:",chardata"`
	} `xml:"content"`
}

// feed parses the site's feed.xml, failing the test if it isn't Atom.
func feed(t *testing.T, site *models.Site) atomFeed {
	t.Helper()
	var f atomFeed
	if err := xml.Unmarshal([]byte(page(t, site, "feed.xml")), &f); err != nil {
		t.Fatalf("feed.xml isn't an Atom feed: %v\n%s", err, page(t, site, "feed.xml"))
	}
	return f
}

func TestFeedHasThePublishedPostsNewestFirstWithTheirFullContent(t *testing.T) {
	repo := contentRepo(map[string]string{
		"site.yaml": "title: Werewolf Notes\nauthor: D. Author\nurl: https://blog.example.com/\n" +
			"timezone: America/New_York\n",
		"posts/old.md":      "---\ntitle: Old\ndate: 2025-12-31\n---\nFirst *post*.\n\nSecond paragraph.\n",
		"posts/new.md":      "---\ntitle: New & shiny\ndate: 2026-03-01\n---\nHello.\n",
		"posts/tomorrow.md": "---\ntitle: Tomorrow\ndate: 2026-03-11\n---\nNot yet.\n",
		"posts/undated.md":  "---\ntitle: Undated\n---\nWork in progress.\n",
	})

	for _, includeDrafts := range []bool{false, true} {
		site := build(t, repo, now, service.BuildOptions{IncludeDrafts: includeDrafts})

		assertNoProblems(t, site)
		f := feed(t, site)
		if f.Title != "Werewolf Notes" || f.Author != "D. Author" || f.ID != "https://blog.example.com/" {
			t.Errorf("feed title, author, id = %q, %q, %q", f.Title, f.Author, f.ID)
		}
		if f.Updated != "2026-03-01T00:00:00-05:00" {
			t.Errorf("feed updated = %q, want the newest Post's Publish Date", f.Updated)
		}
		assertLink(t, "feed", f.Links, "self", "https://blog.example.com/feed.xml")
		assertLink(t, "feed", f.Links, "alternate", "https://blog.example.com/")

		if len(f.Entries) != 2 {
			t.Fatalf("drafts=%v: feed has %d entries, want the 2 Published Posts: %+v",
				includeDrafts, len(f.Entries), f.Entries)
		}
		newPost, oldPost := f.Entries[0], f.Entries[1]
		if newPost.Title != "New & shiny" || oldPost.Title != "Old" {
			t.Errorf("entries are %q, %q; want New & shiny, then Old", newPost.Title, oldPost.Title)
		}
		if newPost.ID != "https://blog.example.com/posts/new/" || newPost.Updated != "2026-03-01T00:00:00-05:00" {
			t.Errorf("new entry id, updated = %q, %q", newPost.ID, newPost.Updated)
		}
		assertLink(t, "new entry", newPost.Links, "alternate", "https://blog.example.com/posts/new/")
		if oldPost.Content.Type != "html" ||
			oldPost.Content.Text != "<p>First <em>post</em>.</p>\n<p>Second paragraph.</p>\n" {
			t.Errorf("old entry content (%s) = %q, want the whole Post as HTML",
				oldPost.Content.Type, oldPost.Content.Text)
		}
	}
}

func assertLink(t *testing.T, name string, links []atomLink, rel, href string) {
	t.Helper()
	for _, l := range links {
		if l.Rel == rel && l.Href == href {
			return
		}
	}
	t.Errorf("%s has no %s link to %s: %+v", name, rel, href, links)
}
