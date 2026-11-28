package app_test

import (
	"net/http"
	"regexp"
	"testing"
)

func TestAVisitorsGetForAMissingPathEmitsAVisitor404LineWithItsSource(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	resp := g.request(t, http.MethodGet, "/posts/gone/", http.Header{
		"Referer":         {"https://lobste.rs/s/abc"},
		"User-Agent":      {firefox},
		"X-Forwarded-For": {"203.0.113.7"},
	})
	assertStatus(t, "/posts/gone/", resp, http.StatusNotFound)

	lines := g.statsLines(t, 1)
	assertLine(t, lines[0], map[string]any{
		"class":         "not_found",
		"path":          "/posts/gone/",
		"status":        float64(http.StatusNotFound),
		"kind":          "missing",
		"browser":       "Firefox",
		"browser_major": "140",
		"os":            "Linux",
		"device":        "desktop",
	})
	assertSource(t, lines[0], sourceOf("site", "lobste.rs", true))
	if id := visitorID(t, lines[0]); !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
		t.Errorf("visitor_id = %q, want 16 hex digits", id)
	}
}

func TestABotsRequestForAMissingPathIsABotRequestAndNoVisitor404(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	resp := g.request(t, http.MethodGet, "/wp-login.php", http.Header{"User-Agent": {"curl/8.5.0"}})
	assertStatus(t, "/wp-login.php", resp, http.StatusNotFound)

	lines := g.statsLines(t, 1)
	assertLine(t, lines[0], map[string]any{
		"class":  "bot",
		"bot":    "unknown",
		"path":   "/wp-login.php",
		"status": float64(http.StatusNotFound),
		"kind":   "missing",
	})
	g.assertMetrics(t, map[string]float64{
		`goblog_bot_requests_total{bot="unknown",kind="missing",status_class="4xx"}`: 1,
		`goblog_visitor_not_found_total`:                                             0,
	})
}

func TestMetricsCountVisitor404sAndNeverAsPageViews(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	for _, path := range []string{"/nope/", "/posts/gone/", "/images/missing.png"} {
		g.get(t, path)
	}
	g.request(t, http.MethodHead, "/nope/", nil)
	g.request(t, http.MethodGet, "/nope/", http.Header{"Purpose": {"prefetch"}})

	g.assertMetrics(t, map[string]float64{
		`goblog_visitor_not_found_total`:              3,
		`goblog_other_requests_total{kind="missing"}`: 2,
		`goblog_page_views_total{kind="missing"}`:     0,
		`goblog_page_views_total{kind="post"}`:        0,
	})
}
