package app_test

import (
	"net/http"
	"testing"
)

// assertPageView fails the test unless line is a Page View of path, of kind,
// answered with status, at the test clock's start.
func assertPageView(t *testing.T, line map[string]any, path, kind string, status int) {
	t.Helper()
	want := map[string]any{
		"stream": "goblog_stats",
		"time":   "2026-03-10T15:00:00Z",
		"class":  "page_view",
		"path":   path,
		"status": float64(status),
		"kind":   kind,
	}
	for field, value := range want {
		if line[field] != value {
			t.Errorf("Page View line for %s: %s = %v, want %v; line: %v", path, field, line[field], value, line)
		}
	}
}

func TestGettingAPostEmitsAPageViewLine(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	assertStatus(t, "/posts/hello/", g.get(t, "/posts/hello/"), http.StatusOK)

	lines := g.statsLines(t, 1)
	assertPageView(t, lines[0], "/posts/hello/", "post", http.StatusOK)
}

// statsSite is a Content Repository with one of each kind of page.
func statsSite(t *testing.T) *contentRepo {
	t.Helper()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{
		"site.yaml":      "title: Test\nurl: https://blog.example.com\n",
		"posts/hello.md": "---\ntitle: Hello\ndate: 2026-03-01\ntags: [k8s]\n---\nHello body. ![a cat](../images/cat.png)\n",
		"pages/about.md": "---\ntitle: About\n---\nAbout body.\n",
		"images/cat.png": "not really a PNG",
	})
	return repo
}

func TestGettingAPageTheIndexOrATagPageEmitsAPageViewLine(t *testing.T) {
	t.Parallel()
	for path, kind := range map[string]string{
		"/about/":    "page",
		"/":          "index",
		"/tags/":     "tag",
		"/tags/k8s/": "tag",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			g := startGoblog(t, statsSite(t), nil)
			g.sync(t)

			assertStatus(t, path, g.get(t, path), http.StatusOK)

			lines := g.statsLines(t, 1)
			assertPageView(t, lines[0], path, kind, http.StatusOK)
		})
	}
}
