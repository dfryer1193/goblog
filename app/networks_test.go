package app_test

import (
	"net/http"
	"strings"
	"testing"
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
