package app_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// assertPageView fails the test unless line is a Page View of path, of kind,
// answered 200, at the test clock's start.
func assertPageView(t *testing.T, line map[string]any, path, kind string) {
	t.Helper()
	want := map[string]any{
		"stream": "goblog_stats",
		"time":   "2026-03-10T15:00:00Z",
		"class":  "page_view",
		"path":   path,
		"status": float64(http.StatusOK),
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
	assertPageView(t, lines[0], "/posts/hello/", "post")
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
			assertPageView(t, lines[0], path, kind)
		})
	}
}

func TestRequestsThatArentPageViewsEmitNoLine(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	// Finding the stylesheet reads the index: one Page View.
	asset := assetPath(t, g)

	for _, tc := range []struct {
		name, method, path string
		header             http.Header
	}{
		{"HEAD", http.MethodHead, "/posts/hello/", nil},
		{"Sec-Purpose prefetch", http.MethodGet, "/posts/hello/", http.Header{"Sec-Purpose": {"prefetch"}}},
		{"Sec-Purpose prerender", http.MethodGet, "/posts/hello/", http.Header{"Sec-Purpose": {"prefetch;prerender"}}},
		{"Purpose prefetch", http.MethodGet, "/posts/hello/", http.Header{"Purpose": {"prefetch"}}},
		{"redirect", http.MethodGet, "/posts/hello", nil},
		{"image", http.MethodGet, "/images/cat.png", nil},
		{"asset", http.MethodGet, asset, nil},
		{"feed", http.MethodGet, "/feed.xml", nil},
		{"sitemap", http.MethodGet, "/sitemap.xml", nil},
		{"404", http.MethodGet, "/nope/", nil},
	} {
		resp := g.request(t, tc.method, tc.path, tc.header)
		if resp.status >= http.StatusInternalServerError {
			t.Fatalf("%s: %s %s status = %d", tc.name, tc.method, tc.path, resp.status)
		}
	}
	// Lines are written in order, so once this Page View's line is out,
	// any line for the requests above would be too.
	g.get(t, "/about/")

	lines := g.statsLines(t, 2)
	assertPageView(t, lines[0], "/", "index")
	assertPageView(t, lines[1], "/about/", "page")
}

// assetPath is the path of the site's stylesheet, which has a content hash
// in its name.
func assetPath(t *testing.T, g *goblog) string {
	t.Helper()
	match := regexp.MustCompile(`<link rel="stylesheet" href="(/assets/[^"]+)">`).FindStringSubmatch(g.get(t, "/").body)
	if match == nil {
		t.Fatal("the index links no stylesheet")
	}
	return match[1]
}

func TestMetricsCountPageViewsAndOtherRequestsByKindOfPage(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	asset := assetPath(t, g) // a Page View of the index

	for _, path := range []string{"/posts/hello/", "/posts/hello/", "/about/", "/tags/k8s/"} {
		g.get(t, path)
	}
	g.request(t, http.MethodHead, "/posts/hello/", nil)
	for _, path := range []string{"/images/cat.png", asset, "/feed.xml", "/sitemap.xml", "/nope/", "/posts/hello"} {
		g.get(t, path)
	}

	for sample, want := range map[string]float64{
		`goblog_page_views_total{kind="index"}`:       1,
		`goblog_page_views_total{kind="post"}`:        2,
		`goblog_page_views_total{kind="page"}`:        1,
		`goblog_page_views_total{kind="tag"}`:         1,
		`goblog_page_views_total{kind="image"}`:       0,
		`goblog_other_requests_total{kind="post"}`:    1,
		`goblog_other_requests_total{kind="image"}`:   1,
		`goblog_other_requests_total{kind="asset"}`:   1,
		`goblog_other_requests_total{kind="feed"}`:    1,
		`goblog_other_requests_total{kind="sitemap"}`: 1,
		`goblog_other_requests_total{kind="missing"}`: 1,
		`goblog_other_requests_total{kind="other"}`:   1,
	} {
		if got := g.metric(t, sample); got != want {
			t.Errorf("%s = %v, want %v", sample, got, want)
		}
	}
}

func TestMetricsAreNotOnTheSitesRouter(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.get(t, "/")

	resp := g.get(t, "/metrics")

	assertStatus(t, "/metrics", resp, http.StatusNotFound)
	if strings.Contains(resp.body, "goblog_page_views_total") {
		t.Errorf("GET /metrics on the site serves metrics:\n%s", resp.body)
	}
}
