package app_test

// The seam-2 harness's stand-in for DB-IP's download site: monthly MMDB
// releases, served gzipped from httptest, from MaxMind's test databases in
// testdata/maxmind (see its ORIGIN.md).

import (
	"bytes"
	"compress/gzip"
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

// releaseEveryMonth publishes the test database file as every month's
// release.
func (s *databaseServer) releaseEveryMonth(t *testing.T, file string) {
	t.Helper()
	gz := gzipped(t, testDatabase(t, file))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.everyMonth = gz
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

// waitForCity waits until goblog has loaded a City database.
func (g *goblog) waitForCity(t *testing.T) {
	t.Helper()
	g.waitForMetric(t, cityLoaded, 1)
}
