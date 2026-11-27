package app_test

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
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
		{"HEAD 404", http.MethodHead, "/nope/", nil},
		{"prefetched 404", http.MethodGet, "/nope/", http.Header{"Sec-Purpose": {"prefetch"}}},
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
	g.request(t, http.MethodHead, "/nope/", nil)
	for _, path := range []string{"/images/cat.png", asset, "/feed.xml", "/sitemap.xml", "/posts/hello"} {
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

func TestDisablingSiteStatisticsStopsLinesAndMetricsButNotTheSite(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.get(t, "/posts/hello/")
	g.statsLines(t, 1)

	g.statsConfig.setEnabled(false)
	resp := g.get(t, "/posts/hello/")
	assertStatus(t, "/posts/hello/", resp, http.StatusOK)
	assertContains(t, "/posts/hello/", resp.body, "Hello body.")
	assertStatus(t, "/images/cat.png", g.get(t, "/images/cat.png"), http.StatusOK)

	g.statsConfig.setEnabled(true)
	g.get(t, "/about/")

	lines := g.statsLines(t, 2)
	assertPageView(t, lines[0], "/posts/hello/", "post")
	assertPageView(t, lines[1], "/about/", "page")
	for sample, want := range map[string]float64{
		`goblog_page_views_total{kind="post"}`:      1,
		`goblog_page_views_total{kind="page"}`:      1,
		`goblog_other_requests_total{kind="image"}`: 0,
	} {
		if got := g.metric(t, sample); got != want {
			t.Errorf("%s = %v, want %v", sample, got, want)
		}
	}
}

// stalledWriter is an output whose writes block until the test ends.
type stalledWriter struct {
	done <-chan struct{}
}

// Write blocks until the test ends, then fails.
func (w stalledWriter) Write([]byte) (int, error) {
	<-w.done
	return 0, io.ErrClosedPipe
}

func TestAStalledOutputDoesntDelayRequests(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	g := startGoblogWritingStatsTo(t, statsSite(t), nil, stalledWriter{done: done})
	// Registered after the server's cleanup, so it runs first: a handler
	// stuck behind the output is released, and the server can close.
	t.Cleanup(func() { close(done) })
	g.sync(t)

	client := g.srv.Client()
	client.Timeout = 2 * time.Second
	// Far more Page Views than any buffer for lines would hold.
	for range 3000 {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, g.srv.URL+"/posts/hello/", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /posts/hello/ with a stalled output: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /posts/hello/ with a stalled output: status %d", resp.StatusCode)
		}
	}
}

func TestAPageViewLineHasTheBrowserOSAndDeviceFamilies(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, userAgent                   string
		browser, browserMajor, os, device string
	}{
		{
			"Chrome on Windows",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
			"Chrome", "142", "Windows", "desktop",
		},
		{
			"Edge on Windows",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0",
			"Edge", "142", "Windows", "desktop",
		},
		{
			"Firefox on Linux",
			"Mozilla/5.0 (X11; Linux x86_64; rv:145.0) Gecko/20100101 Firefox/145.0",
			"Firefox", "145", "Linux", "desktop",
		},
		{
			"Safari on macOS",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.1 Safari/605.1.15",
			"Safari", "26", "macOS", "desktop",
		},
		{
			"Safari on an iPhone",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.1 Mobile/15E148 Safari/604.1",
			"Safari", "26", "iOS", "mobile",
		},
		{
			"Chrome on an iPhone",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/142.0.7444.77 Mobile/15E148 Safari/604.1",
			"Chrome", "142", "iOS", "mobile",
		},
		{
			"Chrome on an Android phone",
			"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Mobile Safari/537.36",
			"Chrome", "142", "Android", "mobile",
		},
		{
			"Firefox on an Android phone",
			"Mozilla/5.0 (Android 15; Mobile; rv:145.0) Gecko/145.0 Firefox/145.0",
			"Firefox", "145", "Android", "mobile",
		},
		{
			"Safari on an iPad",
			"Mozilla/5.0 (iPad; CPU OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.1 Mobile/15E148 Safari/604.1",
			"Safari", "26", "iOS", "tablet",
		},
		{
			"Chrome on an Android tablet",
			"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
			"Chrome", "142", "Android", "tablet",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := startGoblog(t, statsSite(t), nil)
			g.sync(t)

			g.request(t, http.MethodGet, "/posts/hello/", http.Header{"User-Agent": {tc.userAgent}})

			line := g.statsLines(t, 1)[0]
			assertPageView(t, line, "/posts/hello/", "post")
			for field, want := range map[string]any{
				"browser": tc.browser, "browser_major": tc.browserMajor, "os": tc.os, "device": tc.device,
			} {
				if line[field] != want {
					t.Errorf("%s = %v, want %v; line: %v", field, line[field], want, line)
				}
			}
		})
	}
}

func TestAnUnrecognisedUserAgentsPageViewHasOtherAndUnknownFamilies(t *testing.T) {
	t.Parallel()
	for _, userAgent := range []string{
		"totally unknown thing",
		"Mozilla/5.0 (Hurd; GNU) Wobble/7.3.1",
	} {
		t.Run(userAgent, func(t *testing.T) {
			t.Parallel()
			g := startGoblog(t, statsSite(t), nil)
			g.sync(t)

			g.request(t, http.MethodGet, "/posts/hello/", http.Header{"User-Agent": {userAgent}})

			line := g.statsLines(t, 1)[0]
			assertPageView(t, line, "/posts/hello/", "post")
			for field, want := range map[string]any{
				"browser": "other", "browser_major": nil, "os": "other", "device": "unknown",
			} {
				if line[field] != want {
					t.Errorf("%s = %v, want %v; line: %v", field, line[field], want, line)
				}
			}
		})
	}
}

func TestAPageViewLineHasNoRawUserAgentOrFullVersion(t *testing.T) {
	t.Parallel()
	const userAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 " +
		"(KHTML, like Gecko) CriOS/142.0.7444.77 Mobile/15E148 Safari/604.1"
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	g.request(t, http.MethodGet, "/posts/hello/", http.Header{"User-Agent": {userAgent}})

	line := g.statsLines(t, 1)[0]
	for field, value := range line {
		s, ok := value.(string)
		if !ok {
			continue
		}
		for _, leak := range []string{userAgent, "CriOS", "142.0", "7444", "18_6", "15E148"} {
			if strings.Contains(s, leak) {
				t.Errorf("%s = %q, which contains %q from the User-Agent", field, s, leak)
			}
		}
	}
}
