package app_test

// The seam-2 harness's stand-in for DB-IP's download site: monthly MMDB
// releases, served gzipped from httptest, from MaxMind's test databases in
// testdata/maxmind (see its ORIGIN.md).

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The test databases, in testdata/maxmind.
const (
	// geoLite2City is MaxMind's GeoLite2 City test database.
	geoLite2City = "GeoLite2-City-Test.mmdb"
	// geoIP2City is MaxMind's GeoIP2 City test database. Its data differs
	// from geoLite2City's, so it can stand in for the next month's release.
	geoIP2City = "GeoIP2-City-Test.mmdb"
)

// testDatabase is the test database file's contents.
func testDatabase(t *testing.T, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "maxmind", file))
	if err != nil {
		t.Fatalf("reading test database: %v", err)
	}
	return b
}

// databaseServer serves monthly City database releases at
// /city-YYYY-MM.mmdb.gz, gzipped, as DB-IP's download site does. A month
// without a release is 404.
type databaseServer struct {
	srv *httptest.Server

	mu sync.Mutex
	// releases are each month's database ("2026-03"), gzipped.
	releases map[string][]byte
	// everyMonth, if set, is released every month without its own release.
	everyMonth []byte
	// failing makes every download fail with 500.
	failing bool
	// held, while open, holds every download up until it's closed.
	held chan struct{}
}

// newDatabaseServer starts a databaseServer with no releases.
func newDatabaseServer(t *testing.T) *databaseServer {
	t.Helper()
	s := &databaseServer{releases: map[string][]byte{}}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.srv.Close)
	return s
}

// urlTemplate is GOBLOG_STATS_CITY_DB_URL for this server.
func (s *databaseServer) urlTemplate() string {
	return s.srv.URL + "/city-{YYYY}-{MM}.mmdb.gz"
}

// env is startGoblog's env for downloading from this server.
func (s *databaseServer) env() map[string]string {
	return map[string]string{"GOBLOG_STATS_CITY_DB_URL": s.urlTemplate()}
}

// release publishes the test database file as month's release ("2026-03").
func (s *databaseServer) release(t *testing.T, month, file string) {
	t.Helper()
	gz := gzipped(t, testDatabase(t, file))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releases[month] = gz
}

// releaseEveryMonth publishes the test database file as every month's
// release.
func (s *databaseServer) releaseEveryMonth(t *testing.T, file string) {
	t.Helper()
	gz := gzipped(t, testDatabase(t, file))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.everyMonth = gz
}

// setFailing makes every download fail with 500, or stops it.
func (s *databaseServer) setFailing(failing bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failing = failing
}

// hold holds every download up until the returned function is called.
func (s *databaseServer) hold(t *testing.T) func() {
	t.Helper()
	held := make(chan struct{})
	s.mu.Lock()
	s.held = held
	s.mu.Unlock()
	var once sync.Once
	release := func() { once.Do(func() { close(held) }) }
	t.Cleanup(release)
	return release
}

// serve answers a download.
func (s *databaseServer) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	held := s.held
	s.mu.Unlock()
	if held != nil {
		select {
		case <-held:
		case <-r.Context().Done():
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failing {
		http.Error(w, "failing", http.StatusInternalServerError)
		return
	}
	month, ok := strings.CutPrefix(r.URL.Path, "/city-")
	month, ok2 := strings.CutSuffix(month, ".mmdb.gz")
	db := s.releases[month]
	if db == nil {
		db = s.everyMonth
	}
	if !ok || !ok2 || db == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(db)
}

// gzipped is b, gzipped.
func gzipped(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		t.Fatalf("gzipping: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzipping: %v", err)
	}
	return buf.Bytes()
}

// waitForMetric waits until goblog's metric sample reads want, and fails the
// test if it doesn't within 5 seconds.
func (g *goblog) waitForMetric(t *testing.T, sample string, want float64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for g.metric(t, sample) != want {
		if time.Now().After(deadline) {
			t.Fatalf("%s = %v, want %v", sample, g.metric(t, sample), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// cityLoaded is the sample saying whether the City database is loaded.
const cityLoaded = `goblog_stats_database_loaded{database="city"}`

// cityAge is the sample saying how old the loaded City database is.
const cityAge = `goblog_stats_database_age_seconds{database="city"}`

// waitForCity waits until goblog has loaded a City database.
func (g *goblog) waitForCity(t *testing.T) {
	t.Helper()
	g.waitForMetric(t, cityLoaded, 1)
}

// waitForWarnings waits until Site Statistics have logged n warnings, and
// fails the test if they don't within 5 seconds.
func (g *goblog) waitForWarnings(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(g.statsWarnings()) < n {
		if time.Now().After(deadline) {
			t.Fatalf("got %d Site Statistics warnings, want %d:\n%s",
				len(g.statsWarnings()), n, strings.Join(g.statsWarnings(), "\n"))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// copiesOf counts the files in goblog's work directory that are copies of
// the test database file.
func (g *goblog) copiesOf(t *testing.T, file string) int {
	t.Helper()
	want := testDatabase(t, file)
	n := 0
	err := filepath.WalkDir(g.workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if b, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(b, want) {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the work directory: %v", err)
	}
	return n
}
