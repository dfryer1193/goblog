package app_test

import (
	"net/http"
	"testing"
)

func TestCacheControlByKindOfFile(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{
		"posts/hello.md":   post("Hello", "2026-03-01"),
		"images/photo.png": "\x89PNG not really",
	})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	const short, day = "public, max-age=60", "public, max-age=86400"
	gzip := http.Header{"Accept-Encoding": {"gzip"}}
	for _, tc := range []struct {
		path   string
		header http.Header
		status int
		want   string
	}{
		{"/", nil, http.StatusOK, short},
		{"/", gzip, http.StatusOK, short},
		{"/posts/hello/", nil, http.StatusOK, short},
		{"/images/photo.png", nil, http.StatusOK, day},
		{"/posts/hello", nil, http.StatusMovedPermanently, short},
		{"/nope", nil, http.StatusNotFound, short},
		{"/images/missing.png", nil, http.StatusNotFound, short},
	} {
		resp := g.request(t, http.MethodGet, tc.path, tc.header)

		assertStatus(t, tc.path, resp, tc.status)
		if got := resp.header.Get("Cache-Control"); got != tc.want {
			t.Errorf("GET %s (Accept-Encoding %q) Cache-Control = %q, want %q",
				tc.path, tc.header.Get("Accept-Encoding"), got, tc.want)
		}
	}
}

func TestUnreadySiteIsNotCached(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)

	resp := g.get(t, "/")

	assertStatus(t, "/", resp, http.StatusServiceUnavailable)
	if got := resp.header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}
