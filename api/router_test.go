package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/api"
)

func alwaysReady(context.Context) bool { return true }

func get(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, http.NoBody)
	if err != nil {
		t.Fatalf("building request for %s: %v", path, err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body of %s: %v", path, err)
	}
	return resp.StatusCode, string(body)
}

func newServer(t *testing.T, ready func(context.Context) bool) *httptest.Server {
	t.Helper()
	r, err := api.NewRouter(ready)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestRootServesPlaceholderPage(t *testing.T) {
	t.Parallel()
	srv := newServer(t, alwaysReady)

	status, body := get(t, srv, "/")

	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, "goblog") {
		t.Errorf("GET / body = %q, want it to mention goblog", body)
	}
}

func TestProbesRespond(t *testing.T) {
	t.Parallel()
	srv := newServer(t, alwaysReady)

	for _, path := range []string{"/livez", "/readyz", "/startupz"} {
		if status, _ := get(t, srv, path); status != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, status, http.StatusOK)
		}
	}
}

func TestReadinessFollowsReadyCheck(t *testing.T) {
	t.Parallel()
	srv := newServer(t, func(context.Context) bool { return false })

	if status, _ := get(t, srv, "/readyz"); status != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz status = %d, want %d", status, http.StatusServiceUnavailable)
	}
}
