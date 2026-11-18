package app_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestServesGzipVariantToClientsThatAcceptGzip(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	plain := g.get(t, "/posts/hello/")

	resp := g.request(t, http.MethodGet, "/posts/hello/", http.Header{"Accept-Encoding": {"gzip, deflate, br"}})

	assertStatus(t, "/posts/hello/", resp, http.StatusOK)
	if got := resp.header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := resp.header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	assertVariesOnEncoding(t, "gzip response", resp)
	if got := gunzip(t, resp.body); got != plain.body {
		t.Errorf("decompressed body differs from the plain body:\n%s", got)
	}
	if got := plain.header.Get("Content-Encoding"); got != "" {
		t.Errorf("plain response Content-Encoding = %q, want none", got)
	}
	assertVariesOnEncoding(t, "plain response", plain)
}

// assertVariesOnEncoding fails the test unless resp's Vary includes
// Accept-Encoding.
func assertVariesOnEncoding(t *testing.T, what string, resp response) {
	t.Helper()
	for _, v := range resp.header.Values("Vary") {
		for field := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(field), "Accept-Encoding") {
				return
			}
		}
	}
	t.Errorf("%s Vary = %q, want Accept-Encoding", what, resp.header.Values("Vary"))
}

// gunzip decompresses body, failing the test if it isn't gzip.
func gunzip(t *testing.T, body string) string {
	t.Helper()
	zr, err := gzip.NewReader(strings.NewReader(body))
	if err != nil {
		t.Fatalf("body isn't gzip: %v", err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompressing body: %v", err)
	}
	return string(data)
}
