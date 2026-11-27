package app_test

import (
	"net/http"
	"regexp"
	"testing"
)

// visitorID is the Visitor ID on line, or "" if it has none.
func visitorID(t *testing.T, line map[string]any) string {
	t.Helper()
	id, ok := line["visitor_id"]
	if !ok {
		return ""
	}
	s, ok := id.(string)
	if !ok {
		t.Fatalf("visitor_id = %v, want a string; line: %v", id, line)
	}
	return s
}

// visit is a Page View of /about/ with header. It waits for the Page View's
// line, so lines come in the order of the visits, and returns it.
func visit(t *testing.T, g *goblog, header http.Header) map[string]any {
	t.Helper()
	before := len(g.stats.lines())
	assertStatus(t, "/about/", g.request(t, http.MethodGet, "/about/", header), http.StatusOK)
	return g.statsLines(t, before+1)[before]
}

// visitFrom is visit from ip, as the gateway forwards it, with userAgent,
// and returns the line's Visitor ID.
func visitFrom(t *testing.T, g *goblog, ip, userAgent string) string {
	t.Helper()
	return visitorID(t, visit(t, g, http.Header{"X-Forwarded-For": {ip}, "User-Agent": {userAgent}}))
}

const (
	firefox = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"
	safari  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) " +
		"Version/18.0 Safari/605.1.15"
)

func TestTheSameIPAndUserAgentGetTheSameVisitorIDAndOthersDont(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	ids := []string{
		visitFrom(t, g, "203.0.113.7", firefox),
		visitFrom(t, g, "203.0.113.7", firefox),
		visitFrom(t, g, "203.0.113.8", firefox),
		visitFrom(t, g, "203.0.113.7", safari),
	}

	for i, id := range ids {
		if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
			t.Errorf("visit %d: visitor_id = %q, want 16 hex digits", i, id)
		}
	}
	if ids[0] != ids[1] {
		t.Errorf("the same IP and User-Agent got Visitor IDs %q and %q, want the same", ids[0], ids[1])
	}
	if ids[2] == ids[0] {
		t.Errorf("a different IP got the same Visitor ID %q", ids[2])
	}
	if ids[3] == ids[0] {
		t.Errorf("a different User-Agent got the same Visitor ID %q", ids[3])
	}
}
