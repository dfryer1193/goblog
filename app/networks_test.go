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
