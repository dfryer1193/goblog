package app_test

import (
	"net/http"
	"testing"
)

func TestNotReadyUntilFirstRenderThenReady(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)

	assertStatus(t, "/readyz", g.get(t, "/readyz"), http.StatusServiceUnavailable)

	g.sync(t)

	assertStatus(t, "/readyz", g.get(t, "/readyz"), http.StatusOK)
}

func TestServesPublishedPost(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{
		"posts/hello.md": post("Hello", "2026-03-01"),
		"posts/later.md": post("Later", "2026-04-01"),
	})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	index := g.get(t, "/")
	assertStatus(t, "/", index, http.StatusOK)
	assertContains(t, "/", index.body, `href="/posts/hello/"`)
	if got := index.header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("GET / Content-Type = %q, want text/html; charset=utf-8", got)
	}

	page := g.get(t, "/posts/hello/")
	assertStatus(t, "/posts/hello/", page, http.StatusOK)
	assertContains(t, "/posts/hello/", page.body, "Hello body.")

	assertStatus(t, "/posts/later/", g.get(t, "/posts/later/"), http.StatusNotFound)
}

func TestPostURLWithoutTrailingSlashRedirects(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	resp := g.get(t, "/posts/hello")

	assertStatus(t, "/posts/hello", resp, http.StatusMovedPermanently)
	if got := resp.header.Get("Location"); got != "/posts/hello/" {
		t.Errorf("GET /posts/hello Location = %q, want /posts/hello/", got)
	}
}

func TestUnknownPathsAreNotFound(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	for _, path := range []string{"/nope", "/nope/", "/posts/", "/posts/hello/missing.png", "/../etc/passwd"} {
		assertStatus(t, path, g.get(t, path), http.StatusNotFound)
	}
}

func TestPublishesTheConfiguredBranch(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	repo.commitOn(t, "live", map[string]string{"posts/live.md": post("Live", "2026-03-02")})
	g := startGoblog(t, repo, map[string]string{"GOBLOG_CONTENT_BRANCH": "live"})

	g.sync(t)

	assertStatus(t, "/posts/live/", g.get(t, "/posts/live/"), http.StatusOK)
}

func TestSyncPublishesTheLatestCommit(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	repo.commitOn(t, "other", map[string]string{"posts/other.md": post("Other", "2026-03-02")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	assertStatus(t, "/posts/other/", g.get(t, "/posts/other/"), http.StatusNotFound)

	repo.commit(t, map[string]string{"posts/second.md": post("Second", "2026-03-05")})
	g.sync(t)

	assertStatus(t, "/posts/second/", g.get(t, "/posts/second/"), http.StatusOK)
	assertContains(t, "/", g.get(t, "/").body, `href="/posts/second/"`)
	assertStatus(t, "/posts/other/", g.get(t, "/posts/other/"), http.StatusNotFound)
}

func TestFailedSyncKeepsTheCurrentSite(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	repo.remove(t)
	g.syncFails(t)

	assertStatus(t, "/readyz", g.get(t, "/readyz"), http.StatusOK)
	assertStatus(t, "/posts/hello/", g.get(t, "/posts/hello/"), http.StatusOK)
}

func TestFirstSyncFailureLeavesGoblogNotReady(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	repo.remove(t)
	g := startGoblog(t, repo, nil)

	g.syncFails(t)

	assertStatus(t, "/readyz", g.get(t, "/readyz"), http.StatusServiceUnavailable)
	assertStatus(t, "/", g.get(t, "/"), http.StatusServiceUnavailable)
}
