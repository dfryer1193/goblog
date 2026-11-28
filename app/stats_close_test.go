package app_test

import (
	"io"
	"net/http"
	"testing"
	"time"
)

// slowWriter is an output that takes a millisecond over each write.
type slowWriter struct {
	out io.Writer
}

// Write writes p to out, a millisecond late.
func (w slowWriter) Write(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	return w.out.Write(p)
}

func TestClosingWritesTheQueuedLinesFirst(t *testing.T) {
	t.Parallel()
	captured := &statsOutput{}
	g := startGoblogWritingStatsTo(t, statsSite(t), nil, slowWriter{out: captured})
	g.sync(t)
	const n = 300
	for range n {
		g.request(t, http.MethodGet, "/posts/hello/", nil)
	}

	g.app.Close()

	if got := len(captured.lines()); got != n {
		t.Fatalf("after Close, %d lines are written, want all %d", got, n)
	}
}

func TestClosingWithAStalledOutputWaitsOnlySoLong(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	g := startGoblogWritingStatsTo(t, statsSite(t), nil, stalledWriter{done: done})
	t.Cleanup(func() { close(done) })
	g.sync(t)
	for range 10 {
		g.request(t, http.MethodGet, "/posts/hello/", nil)
	}

	start := time.Now()
	g.app.Close()

	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("Close took %v with a stalled output, want it to give up within 5s", took)
	}
}
