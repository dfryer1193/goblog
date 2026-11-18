package app_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// removeSite404 deletes the 404 page (and its gzip variant) from the site
// goblog is serving, as if the render had produced none.
func (g *goblog) removeSite404(t *testing.T) {
	t.Helper()
	pages, err := filepath.Glob(filepath.Join(g.workDir, "output", "site-*", "404.html*"))
	if err != nil || len(pages) == 0 {
		t.Fatalf("finding the rendered 404 page: %v (found %d)", err, len(pages))
	}
	for _, p := range pages {
		if err = os.Remove(p); err != nil {
			t.Fatalf("removing %s: %v", p, err)
		}
	}
}

// assertBuiltInPage fails the test unless resp is one of goblog's built-in
// pages with the given pane label.
func assertBuiltInPage(t *testing.T, path string, resp response, label string) {
	t.Helper()
	assertContains(t, path, resp.body, `<span class="label">`+label+`</span>`)
	assertContains(t, path, resp.body, "<style>")
	if got := resp.header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("%s Content-Type = %q, want HTML", path, got)
	}
}

func TestUnknownPathsGetTheBuiltIn404WhenTheSiteHasNone(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	g.removeSite404(t)

	for _, path := range []string{"/nope", "/posts/gone/", "/posts"} {
		resp := g.get(t, path)
		assertStatus(t, path, resp, http.StatusNotFound)
		assertBuiltInPage(t, path, resp, "[no name]")
		assertContains(t, path, resp.body, "no such file or directory")
		assertContains(t, path, resp.body, `href="/"`)
		if got := resp.header.Get("Cache-Control"); got != "public, max-age=60" {
			t.Errorf("GET %s Cache-Control = %q, want the short rule", path, got)
		}
	}
}
