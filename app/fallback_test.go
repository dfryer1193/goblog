package app_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

func TestBuiltIn404UnderHEADAndGzip(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	g.removeSite404(t)
	page := g.get(t, "/nope")

	head := g.request(t, http.MethodHead, "/nope", nil)
	assertStatus(t, "HEAD /nope", head, http.StatusNotFound)
	if head.body != "" {
		t.Errorf("HEAD /nope has a body:\n%s", head.body)
	}
	for _, key := range []string{"Content-Type", "Content-Length", "Cache-Control"} {
		if got, want := head.header.Get(key), page.header.Get(key); got != want {
			t.Errorf("HEAD /nope %s = %q, want GET's %q", key, got, want)
		}
	}
	if got, want := page.header.Get("Content-Length"), strconv.Itoa(len(page.body)); got != want {
		t.Errorf("GET /nope Content-Length = %q, want %q", got, want)
	}

	gz := g.request(t, http.MethodGet, "/nope", http.Header{"Accept-Encoding": {"gzip"}})
	assertStatus(t, "gzip /nope", gz, http.StatusNotFound)
	if gz.body != page.body {
		t.Errorf("gzip-accepting GET /nope got a different body:\n%s", gz.body)
	}
	// It has no gzip variant, so, like any such file, it isn't negotiated.
	for _, key := range []string{"Content-Encoding", "Vary"} {
		if got := gz.header.Get(key); got != "" {
			t.Errorf("gzip-accepting GET /nope %s = %q, want none", key, got)
		}
	}
}

// assertSelfContained fails the test if page loads anything (stylesheets,
// fonts, images, scripts) or runs JavaScript.
func assertSelfContained(t *testing.T, name, page string) {
	t.Helper()
	for _, bad := range []string{"<script", "<link", "src=", "url(", "@import", "javascript:"} {
		if strings.Contains(strings.ToLower(page), bad) {
			t.Errorf("%s contains %q:\n%s", name, bad, page)
		}
	}
	if handler := regexp.MustCompile(`(?i)\son[a-z]+\s*=`).FindString(page); handler != "" {
		t.Errorf("%s has an event handler %q", name, handler)
	}
	for _, href := range regexp.MustCompile(`href="([^"]*)"`).FindAllStringSubmatch(page, -1) {
		if href[1] != "/" {
			t.Errorf("%s links to %q; the built-in pages only link home", name, href[1])
		}
	}
}

func TestBuiltIn404IsSelfContained(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	g.removeSite404(t)

	assertSelfContained(t, "the built-in 404", g.get(t, "/nope").body)
}

// assertStartingPage fails the test unless resp is the built-in "starting
// up" 503 (with no body for HEAD).
func assertStartingPage(t *testing.T, name string, resp response, head bool) {
	t.Helper()
	assertStatus(t, name, resp, http.StatusServiceUnavailable)
	if got := resp.header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("%s Cache-Control = %q, want no-store", name, got)
	}
	if secs, err := strconv.Atoi(resp.header.Get("Retry-After")); err != nil || secs < 1 || secs > 60 {
		t.Errorf("%s Retry-After = %q, want a few seconds", name, resp.header.Get("Retry-After"))
	}
	if got := resp.header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("%s Content-Type = %q, want HTML", name, got)
	}
	if head {
		if resp.body != "" {
			t.Errorf("%s has a body:\n%s", name, resp.body)
		}
		return
	}
	assertBuiltInPage(t, name, resp, "[starting]")
	assertContains(t, name, resp.body, "being rendered")
	assertSelfContained(t, name, resp.body)
}

func TestEveryRequestGetsTheStartingPageBeforeTheFirstRender(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		for _, path := range []string{"/", "/posts/hello/", "/nope", "/assets/style.css"} {
			name := method + " " + path
			resp := g.request(t, method, path, http.Header{"Accept-Encoding": {"gzip"}})
			assertStartingPage(t, name, resp, method == http.MethodHead)
		}
	}

	assertStatus(t, "/livez", g.get(t, "/livez"), http.StatusOK)
	body := `{"ref":"refs/heads/trunk"}`
	assertStatus(t, "/webhook", g.webhook(t, "ping", body, sign(webhookSecret, body)), http.StatusNoContent)
}

func TestAFailedFirstRenderKeepsTheStartingPageUntilASyncSucceeds(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{
		"site.yaml":      "title: Test\nurl: https://blog.example.com\ntimezone: Not/A_Timezone\n",
		"posts/hello.md": post("Hello", "2026-03-01"),
	})
	g := startGoblog(t, repo, nil)

	g.syncFails(t)

	for _, path := range []string{"/", "/nope"} {
		assertStartingPage(t, path+" after a failed render", g.get(t, path), false)
	}

	repo.commit(t, map[string]string{"site.yaml": "title: Test\nurl: https://blog.example.com\n"})
	g.sync(t)

	assertStatus(t, "/posts/hello/", g.get(t, "/posts/hello/"), http.StatusOK)
	assertStatus(t, "/nope", g.get(t, "/nope"), http.StatusNotFound)
}

// scanPaths are requests that may hit any of goblog's 404 and 503 paths.
var scanPaths = []string{ //nolint:gochecknoglobals // test table
	"/", "/nope", "/posts", "/posts/", "/posts/gone/", "/404.html", "/images/missing.png", "/assets/nope.css",
	"/webhook", "/webhook/", "/webhook/x", "/livez/x", "/readyz/", "/startupz/x", "//", "/%2e%2e/etc/passwd",
}

// assertNoPlainGoErrors fails the test if any 404 or 503 response to
// scanPaths, by any method, is one of Go's plain-text error bodies rather
// than an HTML page.
func assertNoPlainGoErrors(t *testing.T, g *goblog, state string) {
	t.Helper()
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		for _, path := range scanPaths {
			resp := g.request(t, method, path, nil)
			if resp.status != http.StatusNotFound && resp.status != http.StatusServiceUnavailable {
				continue
			}
			name := state + ": " + method + " " + path
			contentType := resp.header.Get("Content-Type")
			if strings.Contains(resp.body, "404 page not found") || strings.HasPrefix(contentType, "text/plain") {
				t.Errorf("%s got Go's plain text (%d, %q):\n%s", name, resp.status, contentType, resp.body)
			}
			if got := resp.header.Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("%s Content-Type = %q, want HTML", name, got)
			}
		}
	}
}

func TestGoPlainTextErrorsNeverReachAClient(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})

	for _, env := range []map[string]string{nil, {"GOBLOG_WEBHOOK_SECRET": ""}} {
		g := startGoblog(t, repo, env)
		assertNoPlainGoErrors(t, g, "before the first render")
		g.sync(t)
		assertNoPlainGoErrors(t, g, "rendered")
		g.removeSite404(t)
		assertNoPlainGoErrors(t, g, "rendered without a 404 page")
	}
}

func TestUnknownPathsGetTheBuiltIn404WhenTheSites404CantBeRead(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	g.removeSite404(t)
	sites, err := filepath.Glob(filepath.Join(g.workDir, "output", "site-*"))
	if err != nil || len(sites) != 1 {
		t.Fatalf("finding the rendered site: %v (found %d)", err, len(sites))
	}
	for _, name := range []string{"404.html", "404.html.gz"} {
		if err = os.Mkdir(filepath.Join(sites[0], name), 0o750); err != nil {
			t.Fatal(err)
		}
	}

	for _, header := range []http.Header{nil, {"Accept-Encoding": {"gzip"}}} {
		resp := g.request(t, http.MethodGet, "/nope", header)
		assertStatus(t, "/nope", resp, http.StatusNotFound)
		assertBuiltInPage(t, "/nope", resp, "[no name]")
	}
}
