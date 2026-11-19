package service_test

import (
	"encoding/xml"
	"slices"
	"testing"

	"github.com/dfryer1193/goblog/models"
	"github.com/dfryer1193/goblog/service"
)

// sitemapLocs parses the site's sitemap.xml and returns its URLs.
func sitemapLocs(t *testing.T, site *models.Site) []string {
	t.Helper()
	var sitemap struct {
		XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		URLs    []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	text := page(t, site, "sitemap.xml")
	if err := xml.Unmarshal([]byte(text), &sitemap); err != nil {
		t.Fatalf("sitemap.xml isn't a sitemap: %v\n%s", err, text)
	}
	locs := make([]string, 0, len(sitemap.URLs))
	for _, u := range sitemap.URLs {
		locs = append(locs, u.Loc)
	}
	return locs
}

func TestSitemapListsEveryPublicPageAndNoDrafts(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/old.md":      "---\ntitle: Old\ndate: 2025-12-31\ntags: [homelab]\n---\n",
		"posts/new.md":      "---\ntitle: New\ndate: 2026-03-01\ntags: [Café, homelab]\n---\n",
		"posts/tomorrow.md": "---\ntitle: Tomorrow\ndate: 2026-03-11\ntags: [secret]\n---\n",
		"posts/undated.md":  "---\ntitle: Undated\ntags: [wip]\n---\n",
		"pages/about.md":    "---\ntitle: About\n---\n",
	})

	for _, includeDrafts := range []bool{false, true} {
		site := build(t, repo, now, service.BuildOptions{IncludeDrafts: includeDrafts})

		assertNoProblems(t, site)
		want := []string{
			"https://blog.example.com/",
			"https://blog.example.com/posts/new/",
			"https://blog.example.com/posts/old/",
			"https://blog.example.com/about/",
			"https://blog.example.com/tags/",
			"https://blog.example.com/tags/caf%C3%A9/",
			"https://blog.example.com/tags/homelab/",
		}
		if got := sitemapLocs(t, site); !slices.Equal(got, want) {
			t.Errorf("drafts=%v: sitemap has\n%q\nwant\n%q", includeDrafts, got, want)
		}
	}
}
