package app_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// googlebot is Googlebot's User-Agent.
const googlebot = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

// botFromIP is a Bot's request for /posts/hello/ from ip, as the gateway
// forwards it, and returns its line.
func botFromIP(t *testing.T, g *goblog, ip string) map[string]any {
	t.Helper()
	before := len(g.stats.lines())
	resp := g.request(t, http.MethodGet, "/posts/hello/", http.Header{
		"X-Forwarded-For": {ip},
		"User-Agent":      {googlebot},
	})
	assertStatus(t, "/posts/hello/", resp, http.StatusOK)
	line := g.statsLines(t, before+1)[before]
	if line["class"] != "bot" {
		t.Fatalf("class = %v, want bot; line: %v", line["class"], line)
	}
	return line
}

// assertNetwork fails the test unless line says the AS number asn and the
// organisation org, the number absent if it's 0 and the organisation if
// it's "".
func assertNetwork(t *testing.T, line map[string]any, asn int, org string) {
	t.Helper()
	switch got, ok := line["asn"]; {
	case asn == 0 && ok:
		t.Errorf("asn = %v, want it absent; line: %v", got, line)
	case asn != 0 && got != float64(asn):
		t.Errorf("asn = %v, want %d; line: %v", got, asn, line)
	}
	switch got, ok := line["as_org"]; {
	case org == "" && ok:
		t.Errorf("as_org = %v, want it absent; line: %v", got, line)
	case org != "" && got != org:
		t.Errorf("as_org = %v, want %q; line: %v", got, org, line)
	}
}

func TestBotLinesSayTheBotsNetworkAndNeverItsIP(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.waitForASN(t)

	// MaxMind's ASN test database's networks.
	for _, tc := range []struct {
		ip  string
		asn int
		org string
	}{
		{"1.0.0.1", 15169, "Google Inc."},
		{"89.160.20.112", 29518, "Bredband2 AB"},
		{"12.81.92.1", 7018, "AT&T Services"},
		{"2600:6000::1", 237, "Merit Network Inc."},
		{"::ffff:1.0.0.1", 15169, "Google Inc."},
		{"216.160.83.56", 209, ""}, // a network without an organisation
		{"81.2.69.142", 0, ""},     // not in the database
	} {
		line := botFromIP(t, g, tc.ip)
		assertNetwork(t, line, tc.asn, tc.org)
		if _, ok := line["hosting"]; ok {
			t.Errorf("the Bot line from %s has hosting: %v", tc.ip, line)
		}
		raw := g.stats.lines()[len(g.stats.lines())-1]
		if strings.Contains(raw, strings.TrimPrefix(tc.ip, "::ffff:")) {
			t.Errorf("the Bot line from %s contains its IP:\n%s", tc.ip, raw)
		}
	}
}

// assertHosting fails the test unless line's hosting flag is want, or
// absent if want is nil.
func assertHosting(t *testing.T, line map[string]any, want *bool) {
	t.Helper()
	got, ok := line["hosting"]
	switch {
	case want == nil && ok:
		t.Errorf("hosting = %v, want it absent; line: %v", got, line)
	case want != nil && got != *want:
		t.Errorf("hosting = %v, want %v; line: %v", got, *want, line)
	}
}

func TestVisitorLinesSayWhetherTheVisitorIsOnAHostingNetworkButNotWhichNetwork(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.waitForASN(t)

	yes, no := true, false
	for _, tc := range []struct {
		ip      string
		hosting *bool
	}{
		{"1.0.0.1", &yes},      // AS15169, Google: in the hosting list
		{"89.160.20.112", &no}, // AS29518, Bredband2: an ISP
		{"12.81.92.1", &no},    // AS7018, AT&T: an ISP
		{"81.2.69.142", nil},   // not in the database
	} {
		for _, path := range []string{"/about/", "/posts/gone/"} {
			before := len(g.stats.lines())
			g.request(t, http.MethodGet, path, http.Header{"X-Forwarded-For": {tc.ip}, "User-Agent": {firefox}})
			line := g.statsLines(t, before+1)[before]
			if line["class"] != "page_view" && line["class"] != "not_found" {
				t.Fatalf("%s from %s: class = %v; line: %v", path, tc.ip, line["class"], line)
			}
			assertHosting(t, line, tc.hosting)
			assertNetwork(t, line, 0, "")
		}
	}
}

func TestWithoutTheASNDatabaseLinesOmitTheNetworkWithAWarning(t *testing.T) {
	t.Parallel()
	databases := newASNDatabaseServer(t)
	databases.releaseEveryMonth(t, geoLite2ASN)
	databases.setFailing(true)
	g := startGoblog(t, statsSite(t), databases.env())
	g.sync(t)
	g.waitForCity(t)
	g.waitForWarnings(t, 1)

	assertNetwork(t, botFromIP(t, g, "1.0.0.1"), 0, "")
	line := visitFromIP(t, g, "1.0.0.1")
	assertPageView(t, line, "/about/", "page")
	assertHosting(t, line, nil)
	if got := g.metric(t, asnLoaded); got != 0 {
		t.Errorf("%s = %v, want 0", asnLoaded, got)
	}
	if got := g.metric(t, cityLoaded); got != 1 {
		t.Errorf("%s = %v, want 1: the City database doesn't need the ASN one", cityLoaded, got)
	}
	warnings := g.statsWarnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "database=asn") {
		t.Errorf("got warnings %q, want one about the asn database", warnings)
	}
	if strings.Contains(g.metricsBody(t), asnAge) {
		t.Errorf("without an ASN database, there's an age metric:\n%s", g.metricsBody(t))
	}
}

func TestANewMonthsASNReleaseIsUsedWithoutARestart(t *testing.T) {
	t.Parallel()
	databases := newASNDatabaseServer(t)
	databases.release(t, "2026-03", geoLite2ASN)
	g := startGoblog(t, statsSite(t), databases.env())
	g.sync(t)
	g.waitForASN(t)
	// From 2026-03-01 to the clock's start, 2026-03-10 15:00.
	if got, want := g.metric(t, asnAge), (9*day + 15*time.Hour).Seconds(); got != want {
		t.Errorf("%s = %v, want %v", asnAge, got, want)
	}

	databases.release(t, "2026-04", geoLite2ASN)
	g.clock.set(time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC))
	visitFromIP(t, g, "1.0.0.1")
	g.waitForMetric(t, asnAge, (day + 9*time.Hour).Seconds())

	assertNetwork(t, botFromIP(t, g, "1.0.0.1"), 15169, "Google Inc.")
	if n := g.copiesOf(t, geoLite2ASN); n != 1 {
		t.Errorf("with April's release loaded, the work directory has %d ASN databases, want 1", n)
	}
	if warnings := g.statsWarnings(); len(warnings) != 0 {
		t.Errorf("got %d warnings, want none:\n%s", len(warnings), strings.Join(warnings, "\n"))
	}
}
