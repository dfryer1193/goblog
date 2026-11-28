//go:build unix

package app_test

import (
	"io"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

// stallSaltReads replaces the salt file with a named pipe, so rereading it
// blocks until the test ends, and moves the clock on so it's reread.
func (g *goblog) stallSaltReads(t *testing.T) {
	t.Helper()
	g.removeSalt(t)
	if err := syscall.Mkfifo(g.saltFile, 0o600); err != nil {
		t.Fatalf("making the salt file a named pipe: %v", err)
	}
	// Registered after startGoblog's cleanups, so it runs first: opening
	// the pipe for writing, then closing it, ends the stalled read.
	t.Cleanup(func() {
		if f, err := os.OpenFile(g.saltFile, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = f.Close()
		}
	})
	g.clock.advance(time.Minute)
}

// getMany sends n GETs for path, each with a 2-second timeout, and fails
// the test unless every one is answered 200 in time.
func (g *goblog) getMany(t *testing.T, path string, n int) {
	t.Helper()
	client := g.srv.Client()
	client.Timeout = 2 * time.Second
	for range n {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, g.srv.URL+path, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("User-Agent", browserUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, resp.StatusCode)
		}
	}
}

// assertMetricRises fails the test unless sample's value is above 0 within
// 5 seconds.
func (g *goblog) assertMetricRises(t *testing.T, sample string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for g.metric(t, sample) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%s = 0, want it above 0", sample)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStalledRecordingDoesntDelayRequestsAndCountsWhatItDrops(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	g.stallSaltReads(t)

	// Far more Page Views than any queue of requests would hold.
	g.getMany(t, "/posts/hello/", 3000)

	g.assertMetricRises(t, "goblog_stats_dropped_total")
}

func TestAStalledOutputCountsTheLinesItDrops(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	g := startGoblogWritingStatsTo(t, statsSite(t), nil, stalledWriter{done: done})
	t.Cleanup(func() { close(done) })
	g.sync(t)

	g.getMany(t, "/posts/hello/", 3000)

	g.assertMetricRises(t, "goblog_stats_dropped_total")
	// Recording still counts what it can't write.
	g.assertMetrics(t, map[string]float64{`goblog_page_views_total{kind="post"}`: 3000})
}
