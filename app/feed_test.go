package app_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestFeedIsServedAsAtomAndTheSitemapAsXML(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{
		"site.yaml":      "title: Test\nurl: https://blog.example.com\n",
		"posts/hello.md": post("Hello", "2026-03-01") + strings.Repeat("Enough words to be worth compressing. ", 20),
	})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	gzip := http.Header{"Accept-Encoding": {"gzip"}}
	for _, tc := range []struct {
		path   string
		header http.Header
		want   string
	}{
		{"/feed.xml", nil, "application/atom+xml; charset=utf-8"},
		{"/feed.xml", gzip, "application/atom+xml; charset=utf-8"},
		{"/sitemap.xml", nil, "application/xml; charset=utf-8"},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			resp := g.request(t, method, tc.path, tc.header)

			assertStatus(t, tc.path, resp, http.StatusOK)
			if got := resp.header.Get("Content-Type"); got != tc.want {
				t.Errorf("%s %s (Accept-Encoding %q) Content-Type = %q, want %q",
					method, tc.path, tc.header.Get("Accept-Encoding"), got, tc.want)
			}
		}
	}
	if got := g.request(t, http.MethodGet, "/feed.xml", gzip).header.Get("Content-Encoding"); got != "gzip" {
		t.Errorf("GET /feed.xml Content-Encoding = %q, want the gzip variant served", got)
	}
}
