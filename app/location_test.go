package app_test

import (
	"net/http"
	"strings"
	"testing"
)

// visitFromIP is a Page View of /about/ from ip, as the gateway forwards it,
// and returns its line.
func visitFromIP(t *testing.T, g *goblog, ip string) map[string]any {
	t.Helper()
	return visit(t, g, http.Header{"X-Forwarded-For": {ip}, "User-Agent": {firefox}})
}

// assertLocation fails the test unless line says country and region, each
// absent if it's "".
func assertLocation(t *testing.T, line map[string]any, country, region string) {
	t.Helper()
	for field, want := range map[string]string{"country": country, "region": region} {
		got, ok := line[field]
		switch {
		case want == "" && ok:
			t.Errorf("%s = %v, want it absent; line: %v", field, got, line)
		case want != "" && got != want:
			t.Errorf("%s = %v, want %q; line: %v", field, got, want, line)
		}
	}
}

func TestPageViewsSayTheVisitorsCountryAndRegion(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.waitForCity(t)

	// MaxMind's test database also has subdivision codes, such as ENG and
	// WA, but DB-IP's City Lite doesn't: the region is the English name.
	for _, tc := range []struct {
		ip, country, region string
	}{
		{"81.2.69.142", "GB", "England"},
		{"2.125.160.216", "GB", "England"}, // England, then West Berkshire: the first
		{"216.160.83.56", "US", "Washington"},
		{"89.160.20.112", "SE", "Östergötland County"},
		{"2001:480::1", "US", "California"},
		{"::ffff:81.2.69.142", "GB", "England"},
		{"67.43.156.1", "BT", ""}, // a country without a subdivision
		{"10.0.0.1", "", ""},      // not in the database
	} {
		assertLocation(t, visitFromIP(t, g, tc.ip), tc.country, tc.region)
	}
}

func TestNoLineSaysACityOrCoordinates(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.waitForCity(t)

	// The test database's cities and coordinates for these IPs.
	for ip, finer := range map[string][]string{
		"81.2.69.142":   {"London", "51.5142", "-0.0931"},
		"2.125.160.216": {"Boxford", "Berkshire", "OX1", "51.75"},
		"216.160.83.56": {"Milton", "98354", "47.2513"},
		"2001:480::1":   {"San Diego", "92101", "32.7203"},
	} {
		line := visitFromIP(t, g, ip)
		for _, field := range []string{"city", "postal", "location", "latitude", "longitude"} {
			if v, ok := line[field]; ok {
				t.Errorf("the line from %s has %s %v", ip, field, v)
			}
		}
		raw := g.stats.lines()[len(g.stats.lines())-1]
		for _, s := range finer {
			if strings.Contains(raw, s) {
				t.Errorf("the line from %s contains %q:\n%s", ip, s, raw)
			}
		}
	}
}
