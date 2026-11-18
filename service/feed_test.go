package service_test

import (
	"strings"
	"testing"

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
