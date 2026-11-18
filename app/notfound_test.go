package app_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"testing"
)

func TestUnknownPathsGetTheSitesOwn404Page(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	for _, path := range []string{"/nope", "/posts/gone/", "/images/missing.png"} {
		resp := g.get(t, path)
		assertStatus(t, path, resp, http.StatusNotFound)
		assertContains(t, path, resp.body, `<span class="label">[no name]</span>`)
		if got := resp.header.Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("GET %s Content-Type = %q, want HTML", path, got)
		}
		if got := resp.header.Get("Cache-Control"); got != "public, max-age=60" {
			t.Errorf("GET %s Cache-Control = %q, want the short rule", path, got)
		}
	}

	resp := g.request(t, http.MethodGet, "/nope", http.Header{"Accept-Encoding": {"gzip"}})
	assertStatus(t, "/nope gzipped", resp, http.StatusNotFound)
	if got := resp.header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	zr, err := gzip.NewReader(bytes.NewReader([]byte(resp.body)))
	if err != nil {
		t.Fatalf("404 isn't gzip: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("reading gzipped 404: %v", err)
	}
	assertContains(t, "/nope gzipped", string(body), "[no name]")
}
