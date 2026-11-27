package app_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
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

func TestServingDoesntWaitForTheLocationDatabase(t *testing.T) {
	t.Parallel()
	databases := newDatabaseServer(t)
	databases.releaseEveryMonth(t, geoLite2City)
	finishDownload := databases.hold(t)
	g := startGoblog(t, statsSite(t), databases.env())
	g.sync(t)

	// While the download is held up, the site is served without location.
	assertLocation(t, visitFromIP(t, g, "81.2.69.142"), "", "")
	if got := g.metric(t, cityLoaded); got != 0 {
		t.Errorf("while downloading, %s = %v, want 0", cityLoaded, got)
	}

	finishDownload()
	g.waitForCity(t)
	assertLocation(t, visitFromIP(t, g, "81.2.69.142"), "GB", "England")
}

func TestWithTheDownloadFailingLinesOmitLocationWithAWarning(t *testing.T) {
	t.Parallel()
	databases := newDatabaseServer(t)
	databases.releaseEveryMonth(t, geoLite2City)
	databases.setFailing(true)
	g := startGoblog(t, statsSite(t), databases.env())
	g.sync(t)
	g.waitForWarnings(t, 1)

	line := visitFromIP(t, g, "81.2.69.142")

	assertPageView(t, line, "/about/", "page")
	assertLocation(t, line, "", "")
	if got := g.metric(t, cityLoaded); got != 0 {
		t.Errorf("%s = %v, want 0", cityLoaded, got)
	}
	if warnings := g.statsWarnings(); len(warnings) != 1 {
		t.Errorf("got %d warnings, want 1:\n%s", len(warnings), strings.Join(warnings, "\n"))
	}
	if strings.Contains(g.metricsText(t), "goblog_stats_database_age_seconds") {
		t.Errorf("without a City database, there's an age metric:\n%s", g.metricsText(t))
	}

	// goblog tries again an hour later.
	databases.setFailing(false)
	g.clock.advance(59 * time.Minute)
	visitFromIP(t, g, "81.2.69.142")
	if got := g.metric(t, cityLoaded); got != 0 {
		t.Errorf("59 minutes later, %s = %v, want 0, before trying again", cityLoaded, got)
	}
	g.clock.advance(time.Minute)
	visitFromIP(t, g, "81.2.69.142")
	g.waitForCity(t)
	assertLocation(t, visitFromIP(t, g, "81.2.69.142"), "GB", "England")
}
