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
	if strings.Contains(g.metricsBody(t), cityAge) {
		t.Errorf("without a City database, there's an age metric:\n%s", g.metricsBody(t))
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

// day is a day, for the City database's age.
const day = 24 * time.Hour

func TestANewMonthsReleaseIsUsedWithoutARestartAndTheOldCopyDeleted(t *testing.T) {
	t.Parallel()
	databases := newDatabaseServer(t)
	databases.release(t, "2026-03", geoLite2City)
	databases.release(t, "2026-04", geoIP2City)
	g := startGoblog(t, statsSite(t), databases.env())
	g.sync(t)
	g.waitForCity(t)
	// 214.78.0.1 is only in March's release, and 214.0.1.1 only in April's.
	assertLocation(t, visitFromIP(t, g, "214.78.0.1"), "US", "California")
	assertLocation(t, visitFromIP(t, g, "214.0.1.1"), "", "")
	// From 2026-03-01 to the clock's start, 2026-03-10 15:00.
	if got, want := g.metric(t, cityAge), (9*day + 15*time.Hour).Seconds(); got != want {
		t.Errorf("%s = %v, want %v", cityAge, got, want)
	}

	g.clock.set(time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC))
	visitFromIP(t, g, "214.0.1.1")
	g.waitForMetric(t, cityAge, (day + 9*time.Hour).Seconds())

	assertLocation(t, visitFromIP(t, g, "214.0.1.1"), "AU", "Victoria")
	assertLocation(t, visitFromIP(t, g, "214.78.0.1"), "", "")
	if n := g.copiesOf(t, geoLite2City); n != 0 {
		t.Errorf("with April's release loaded, the work directory has %d copies of March's, want 0", n)
	}
	if n := g.copiesOf(t, geoIP2City); n != 1 {
		t.Errorf("with April's release loaded, the work directory has %d copies of it, want 1", n)
	}
	if warnings := g.statsWarnings(); len(warnings) != 0 {
		t.Errorf("got %d warnings, want none:\n%s", len(warnings), strings.Join(warnings, "\n"))
	}
}

func TestUntilThisMonthsReleaseIsOutLastMonthsIsUsed(t *testing.T) {
	t.Parallel()
	databases := newDatabaseServer(t)
	databases.release(t, "2026-02", geoLite2City)
	g := startGoblog(t, statsSite(t), databases.env())
	g.sync(t)
	g.waitForCity(t)

	assertLocation(t, visitFromIP(t, g, "81.2.69.142"), "GB", "England")
	// From 2026-02-01 to the clock's start, 2026-03-10 15:00.
	if got, want := g.metric(t, cityAge), (37*day + 15*time.Hour).Seconds(); got != want {
		t.Errorf("%s = %v, want %v", cityAge, got, want)
	}

	// goblog tries again for March's an hour later.
	databases.release(t, "2026-03", geoIP2City)
	g.clock.advance(time.Hour)
	visitFromIP(t, g, "214.0.1.1")
	g.waitForMetric(t, cityAge, (9*day + 16*time.Hour).Seconds())
	assertLocation(t, visitFromIP(t, g, "214.0.1.1"), "AU", "Victoria")
	if n := g.copiesOf(t, geoLite2City); n != 0 {
		t.Errorf("with March's release loaded, the work directory has %d copies of February's, want 0", n)
	}
	// A release that isn't out yet isn't a problem.
	if warnings := g.statsWarnings(); len(warnings) != 0 {
		t.Errorf("got %d warnings, want none:\n%s", len(warnings), strings.Join(warnings, "\n"))
	}
}

func TestAFailedRefreshKeepsThePreviousCopyInUse(t *testing.T) {
	t.Parallel()
	databases := newDatabaseServer(t)
	databases.release(t, "2026-03", geoLite2City)
	databases.release(t, "2026-04", geoIP2City)
	g := startGoblog(t, statsSite(t), databases.env())
	g.sync(t)
	g.waitForCity(t)

	databases.setFailing(true)
	g.clock.set(time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC))
	visitFromIP(t, g, "214.78.0.1")
	g.waitForWarnings(t, 1)

	assertLocation(t, visitFromIP(t, g, "214.78.0.1"), "US", "California")
	if got := g.metric(t, cityLoaded); got != 1 {
		t.Errorf("%s = %v, want 1", cityLoaded, got)
	}
	// March's release, from 2026-03-01 to 2026-04-02 09:00.
	if got, want := g.metric(t, cityAge), (32*day + 9*time.Hour).Seconds(); got != want {
		t.Errorf("%s = %v, want %v", cityAge, got, want)
	}
	if n := g.copiesOf(t, geoLite2City); n != 1 {
		t.Errorf("the work directory has %d copies of March's release, want 1", n)
	}
}

func TestVisitor404sSayTheVisitorsCountryAndRegionAndBotsDont(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.waitForCity(t)

	for userAgent, class := range map[string]string{firefox: "not_found", "curl/8.5.0": "bot"} {
		before := len(g.stats.lines())
		resp := g.request(t, http.MethodGet, "/posts/gone/", http.Header{
			"User-Agent":      {userAgent},
			"X-Forwarded-For": {"81.2.69.142"},
		})
		assertStatus(t, "/posts/gone/", resp, http.StatusNotFound)
		line := g.statsLines(t, before+1)[before]
		if line["class"] != class {
			t.Fatalf("class = %v, want %s; line: %v", line["class"], class, line)
		}
		if class == "bot" {
			assertLocation(t, line, "", "")
		} else {
			assertLocation(t, line, "GB", "England")
		}
	}
}
