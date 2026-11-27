package app_test

import (
	"net/http"
	"strings"
	"testing"
)

// getFrom requests path from goblog as the client at ip, which the gateway
// names in X-Forwarded-For.
func (g *goblog) getFrom(t *testing.T, ip, path string) response {
	t.Helper()
	return g.request(t, http.MethodGet, path, http.Header{"X-Forwarded-For": {ip}})
}

func TestRequestsFromAnExcludedNetworkEmitNoLinesOrMetrics(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.statsConfig.setExcludedNetworks("203.0.113.0/24", "2001:db8::/32")
	g.sync(t)

	for _, ip := range []string{"203.0.113.7", "2001:db8::1", "::ffff:203.0.113.8"} {
		resp := g.getFrom(t, ip, "/posts/hello/")
		assertStatus(t, "/posts/hello/", resp, http.StatusOK)
		assertContains(t, "/posts/hello/", resp.body, "Hello body.")
		assertStatus(t, "/images/cat.png", g.getFrom(t, ip, "/images/cat.png"), http.StatusOK)
	}
	// Lines are written in order, so once this Page View's line is out,
	// any line for the requests above would be too.
	g.getFrom(t, "198.51.100.1", "/about/")

	lines := g.statsLines(t, 1)
	assertPageView(t, lines[0], "/about/", "page")
	for sample, want := range map[string]float64{
		`goblog_page_views_total{kind="post"}`:      0,
		`goblog_page_views_total{kind="page"}`:      1,
		`goblog_other_requests_total{kind="image"}`: 0,
	} {
		if got := g.metric(t, sample); got != want {
			t.Errorf("%s = %v, want %v", sample, got, want)
		}
	}
}

func TestChangingTheExcludedNetworksAppliesWithoutARestart(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.getFrom(t, "203.0.113.7", "/posts/hello/")
	g.statsLines(t, 1)

	g.statsConfig.setExcludedNetworks("203.0.113.0/24")
	g.getFrom(t, "203.0.113.7", "/posts/hello/")
	g.getFrom(t, "198.51.100.1", "/tags/")

	g.statsConfig.setExcludedNetworks()
	g.getFrom(t, "203.0.113.7", "/about/")

	lines := g.statsLines(t, 3)
	assertPageView(t, lines[0], "/posts/hello/", "post")
	assertPageView(t, lines[1], "/tags/", "tag")
	assertPageView(t, lines[2], "/about/", "page")
}

func TestAnInvalidExcludedNetworkIsSkippedWithOneWarning(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.statsConfig.setExcludedNetworks("not-a-cidr", "203.0.113.0/24")
	g.sync(t)

	for range 3 {
		g.getFrom(t, "203.0.113.7", "/posts/hello/")
	}
	g.getFrom(t, "198.51.100.1", "/about/")

	lines := g.statsLines(t, 1)
	assertPageView(t, lines[0], "/about/", "page")
	warnings := g.statsWarnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "not-a-cidr") {
		t.Fatalf("warnings = %q, want one naming not-a-cidr", warnings)
	}

	// A new list warns about its new invalid entry, once, and not again
	// about the old one.
	g.statsConfig.setExcludedNetworks("not-a-cidr", "203.0.113.0/33", "203.0.113.0/24")
	for range 3 {
		g.getFrom(t, "203.0.113.7", "/posts/hello/")
	}
	warnings = g.statsWarnings()
	if len(warnings) != 2 || !strings.Contains(warnings[1], "203.0.113.0/33") {
		t.Fatalf("warnings = %q, want a second naming 203.0.113.0/33", warnings)
	}
}
