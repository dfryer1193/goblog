package app_test

import (
	"net/http"
	"regexp"
	"testing"
)

func TestStylesheetAndFontsAreCachedForGood(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	index := g.get(t, "/")
	css := regexp.MustCompile(`<link rel="stylesheet" href="(/assets/[^"]+)">`).FindStringSubmatch(index.body)
	if css == nil {
		t.Fatalf("index links no stylesheet:\n%s", index.body)
	}
	stylesheet := g.get(t, css[1])
	font := regexp.MustCompile(`url\("(/fonts/[^"]+\.woff2)"\)`).FindStringSubmatch(stylesheet.body)
	if font == nil {
		t.Fatalf("stylesheet loads no font:\n%s", stylesheet.body)
	}
	licence := regexp.MustCompile(`/licenses/[^ ]+\.txt`).FindString(stylesheet.body)

	const immutable, short = "public, max-age=31536000, immutable", "public, max-age=60"
	for path, want := range map[string]string{css[1]: immutable, font[1]: immutable, licence: short} {
		resp := g.get(t, path)
		assertStatus(t, path, resp, http.StatusOK)
		if got := resp.header.Get("Cache-Control"); got != want {
			t.Errorf("GET %s Cache-Control = %q, want %q", path, got, want)
		}
	}
}
