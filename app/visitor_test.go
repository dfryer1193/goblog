package app_test

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
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

func TestTheClientIPIsTheRightmostForwardedForEntryOrThePeer(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	rightmost := visitFrom(t, g, "203.0.113.7", firefox)
	behindAnotherProxy := visitFrom(t, g, "198.51.100.1, 203.0.113.7", firefox)
	leftmost := visitFrom(t, g, "198.51.100.1", firefox)
	// The test server's TCP peer is the test, on loopback.
	peer := visitFrom(t, g, "127.0.0.1", firefox)
	noHeader := visitorID(t, visit(t, g, http.Header{"User-Agent": {firefox}}))

	if behindAnotherProxy != rightmost {
		t.Errorf("X-Forwarded-For: 198.51.100.1, 203.0.113.7 got Visitor ID %q, want %q, as from 203.0.113.7",
			behindAnotherProxy, rightmost)
	}
	if leftmost == rightmost {
		t.Errorf("X-Forwarded-For: 198.51.100.1 got 203.0.113.7's Visitor ID %q", leftmost)
	}
	if noHeader != peer {
		t.Errorf("no X-Forwarded-For got Visitor ID %q, want %q, as from the TCP peer 127.0.0.1", noHeader, peer)
	}
}

func TestRewritingTheSaltFileChangesVisitorIDsWithoutARestart(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	before := visitFrom(t, g, "203.0.113.7", firefox)

	g.writeSalt(t, "the-next-days-salt-0123456789abc")
	// goblog checks the salt file at most every 5 seconds.
	g.clock.advance(5 * time.Second)

	after := visitFrom(t, g, "203.0.113.7", firefox)
	if after == "" || after == before {
		t.Errorf("after the salt changed, the same IP and User-Agent got Visitor ID %q, want a new one (was %q)",
			after, before)
	}
	if again := visitFrom(t, g, "203.0.113.7", firefox); again != after {
		t.Errorf(
			"under the new salt, the same IP and User-Agent got Visitor IDs %q and %q, want the same",
			after,
			again,
		)
	}
}

// assertSaltLoaded fails the test unless the salt-loaded gauge reads want.
func assertSaltLoaded(t *testing.T, g *goblog, want float64) {
	t.Helper()
	if got := g.metric(t, "goblog_stats_salt_loaded"); got != want {
		t.Errorf("goblog_stats_salt_loaded = %v, want %v", got, want)
	}
}

func TestWithoutAUsableSaltPageViewsHaveNoVisitorIDUntilItsRestored(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		breakSalt func(t *testing.T, g *goblog)
	}{
		{"missing", func(t *testing.T, g *goblog) { g.removeSalt(t) }},
		{"empty", func(t *testing.T, g *goblog) { g.writeSalt(t, "") }},
		{"shorter than 32 bytes", func(t *testing.T, g *goblog) { g.writeSalt(t, testSalt[:31]) }},
		{"unreadable", func(t *testing.T, g *goblog) {
			// A directory in its place can't be read, even by root.
			g.removeSalt(t)
			if err := os.Mkdir(g.saltFile, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := startGoblog(t, statsSite(t), nil)
			g.sync(t)
			withSalt := visitFrom(t, g, "203.0.113.7", firefox)
			assertSaltLoaded(t, g, 1)

			tc.breakSalt(t, g)
			for range 3 {
				g.clock.advance(5 * time.Second)
				line := visit(t, g, http.Header{"X-Forwarded-For": {"203.0.113.7"}, "User-Agent": {firefox}})
				if id, ok := line["visitor_id"]; ok {
					t.Errorf("with the salt file %s, the Page View has visitor_id %v; line: %v", tc.name, id, line)
				}
			}
			assertSaltLoaded(t, g, 0)
			if warnings := g.statsWarnings(); len(warnings) != 1 {
				t.Errorf("with the salt file %s for 3 checks, got %d warnings, want 1:\n%s",
					tc.name, len(warnings), strings.Join(warnings, "\n"))
			}

			_ = os.Remove(g.saltFile)
			g.writeSalt(t, testSalt)
			g.clock.advance(5 * time.Second)
			if restored := visitFrom(t, g, "203.0.113.7", firefox); restored != withSalt {
				t.Errorf("with the salt restored, Visitor ID = %q, want %q as before", restored, withSalt)
			}
			assertSaltLoaded(t, g, 1)
		})
	}
}

func TestWithoutASaltFileSetPageViewsHaveNoVisitorID(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), map[string]string{"GOBLOG_STATS_SALT_FILE": ""})
	g.sync(t)

	line := visit(t, g, http.Header{"User-Agent": {firefox}})

	if id, ok := line["visitor_id"]; ok {
		t.Errorf("without GOBLOG_STATS_SALT_FILE, the Page View has visitor_id %v", id)
	}
	assertSaltLoaded(t, g, 0)
	if warnings := g.statsWarnings(); len(warnings) != 1 {
		t.Errorf("without GOBLOG_STATS_SALT_FILE, got %d warnings, want 1:\n%s",
			len(warnings), strings.Join(warnings, "\n"))
	}
}

func TestNoLineOrLogContainsTheClientIPOrTheSalt(t *testing.T) {
	t.Parallel()
	const ip, nextSalt = "203.0.113.7", "the-next-days-salt-0123456789abc"
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	visitFrom(t, g, ip, firefox)
	g.writeSalt(t, nextSalt)
	g.clock.advance(5 * time.Second)
	visitFrom(t, g, ip, firefox)
	g.writeSalt(t, nextSalt[:10])
	g.clock.advance(5 * time.Second)
	visitFrom(t, g, ip, firefox)
	g.removeSalt(t)
	g.clock.advance(5 * time.Second)
	visitFrom(t, g, ip, firefox)
	g.writeSalt(t, testSalt)
	g.clock.advance(5 * time.Second)
	visitFrom(t, g, ip, firefox)

	for name, lines := range map[string][]string{"line": g.stats.lines(), "log": g.statsLog.lines()} {
		for _, line := range lines {
			for _, secret := range []string{ip, testSalt, nextSalt, nextSalt[:10]} {
				if strings.Contains(line, secret) {
					t.Errorf("a Site Statistics %s contains %q:\n%s", name, secret, line)
				}
			}
		}
	}
}
