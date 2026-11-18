package service_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/models"
	"github.com/dfryer1193/goblog/service"
)

func TestWithoutSiteSettingsThereIsNoFeedOrSitemapAndNothingLinksToThem(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/x.md":     "---\ntitle: X\ndate: 2026-01-01\ntags: [t]\n---\nBody\n",
		"pages/about.md": "---\ntitle: About\n---\n",
	})
	delete(repo, "site.yaml")

	site := build(t, repo, now, service.BuildOptions{})

	if len(site.Problems) != 1 || !strings.HasPrefix(site.Problems[0].String(), "site.yaml: missing") {
		t.Fatalf("got problems %v, want only the one saying site.yaml is missing", site.Problems)
	}
	assertNoFile(t, site, "feed.xml")
	assertNoFile(t, site, "sitemap.xml")
	for _, name := range []string{
		"index.html", "posts/x/index.html", "about/index.html", "tags/index.html", "tags/t/index.html", "404.html",
	} {
		if text := page(t, site, name); strings.Contains(text, "feed.xml") || strings.Contains(text, "atom+xml") {
			t.Errorf("%s links to a feed that doesn't exist:\n%s", name, text)
		}
	}
	assertContains(t, "index", infoLine(t, page(t, site, "index.html")), "<span>1 post</span>")
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

func TestFeedContentLinksAndImagesAreAbsolute(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/x.md": "---\ntitle: X\ndate: 2026-01-01\n---\n" +
			"See [the other](./other.md#part), [about](../pages/about.md), [root](/), [far](https://elsewhere.example/a), " +
			"[mail](mailto:me@example.com), [here](#top) and [proto](//cdn.example/x).\n\n" +
			"![a pic](../images/a.png)\n\n" +
			"`<a href=\"/code\">` is code.\n",
		"posts/other.md": "---\ntitle: Other\ndate: 2026-01-01\n---\n",
		"pages/about.md": "---\ntitle: About\n---\n",
		"images/a.png":   "png",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	var content string
	for _, e := range feed(t, site).Entries {
		if e.Title == "X" {
			content = e.Content.Text
		}
	}
	for _, want := range []string{
		`href="https://blog.example.com/posts/other/#part"`,
		`href="https://blog.example.com/about/"`,
		`href="https://blog.example.com/"`,
		`href="https://elsewhere.example/a"`,
		`href="mailto:me@example.com"`,
		`href="#top"`,
		`href="//cdn.example/x"`,
		`src="https://blog.example.com/images/a.png"`,
		`<code>&lt;a href=&quot;/code&quot;&gt;</code>`,
	} {
		assertContains(t, "X's feed content", content, want)
	}
}

func TestEveryPageLinksToTheFeedForAutodiscovery(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/x.md":     "---\ntitle: X\ndate: 2026-01-01\ntags: [t]\n---\n",
		"pages/about.md": "---\ntitle: About\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	const want = `<link rel="alternate" type="application/atom+xml" title="Test blog" ` +
		`href="https://blog.example.com/feed.xml">`
	for _, name := range []string{
		"index.html", "posts/x/index.html", "about/index.html", "tags/index.html", "tags/t/index.html",
	} {
		assertContains(t, name, page(t, site, name), want)
	}
}
