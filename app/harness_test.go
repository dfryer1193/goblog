package app_test

// The seam-2 harness: goblog's serve mode, wired the way main wires it, run
// against a Content Repository created in the test (a file:// remote), with a
// controllable clock, through httptest.
//
//	repo := newContentRepo(t)
//	repo.commit(t, map[string]string{"posts/x.md": post("X", "2026-03-01")})
//	g := startGoblog(t, repo, nil)
//	g.sync(t)
//	resp := g.get(t, "/posts/x/")

import (
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/dfryer1193/goblog/app"
)

// defaultBranch is the test Content Repository's default branch. It isn't
// "main" or "master", so a clone that guessed the branch would fail.
const defaultBranch = "trunk"

// startTime is where the test clock starts: 2026-03-10 15:00 UTC.
func startTime() time.Time {
	return time.Date(2026, time.March, 10, 15, 0, 0, 0, time.UTC)
}

// contentRepo is a Content Repository in a temporary directory, reachable
// through its file:// URL.
type contentRepo struct {
	dir  string
	repo *git.Repository
}

// newContentRepo creates an empty Content Repository whose default branch is
// defaultBranch.
func newContentRepo(t *testing.T) *contentRepo {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("creating Content Repository: %v", err)
	}
	head := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(defaultBranch))
	if err = repo.Storer.SetReference(head); err != nil {
		t.Fatalf("setting default branch: %v", err)
	}
	return &contentRepo{dir: dir, repo: repo}
}

// remove deletes the Content Repository, so fetching from it fails.
func (r *contentRepo) remove(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll(r.dir); err != nil {
		t.Fatalf("removing Content Repository: %v", err)
	}
}

// url is the Content Repository's file:// URL.
func (r *contentRepo) url() string {
	return "file://" + filepath.ToSlash(r.dir)
}

// commit writes files (slash path → content) on the checked-out branch, the
// default branch unless inside commitOn, and commits them.
func (r *contentRepo) commit(t *testing.T, files map[string]string) {
	t.Helper()
	wt, err := r.repo.Worktree()
	if err != nil {
		t.Fatalf("opening worktree: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(r.dir, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("creating directory for %s: %v", name, err)
		}
		if err = os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
		if _, err = wt.Add(name); err != nil {
			t.Fatalf("adding %s: %v", name, err)
		}
	}
	sig := &object.Signature{Name: "Author", Email: "author@example.com", When: startTime()}
	if _, err = wt.Commit("Update content", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatalf("committing: %v", err)
	}
}

// commitOn is commit on branch instead, which it creates from the default
// branch if needed. The default branch stays checked out (and stays the
// remote's default branch).
func (r *contentRepo) commitOn(t *testing.T, branch string, files map[string]string) {
	t.Helper()
	r.switchTo(t, branch)
	r.commit(t, files)
	r.switchTo(t, defaultBranch)
}

// switchTo checks out branch, creating it from the current commit if needed.
func (r *contentRepo) switchTo(t *testing.T, branch string) {
	t.Helper()
	wt, err := r.repo.Worktree()
	if err != nil {
		t.Fatalf("opening worktree: %v", err)
	}
	name := plumbing.NewBranchReferenceName(branch)
	_, err = r.repo.Reference(name, false)
	if err = wt.Checkout(&git.CheckoutOptions{Branch: name, Create: err != nil}); err != nil {
		t.Fatalf("checking out %s: %v", branch, err)
	}
}

// post is a Post file with a title, and a Publish Date unless date is empty.
func post(title, date string) string {
	front := "title: " + title + "\n"
	if date != "" {
		front += "date: " + date + "\n"
	}
	return "---\n" + front + "---\n" + title + " body.\n"
}

// testClock is a controllable clock.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now is the current test time.
func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// goblog is a running goblog under test.
type goblog struct {
	app   *app.App
	srv   *httptest.Server
	clock *testClock
	// workDir is goblog's working directory (GOBLOG_WORK_DIR).
	workDir string
}

// startGoblog starts goblog's serve mode against repo, configured from env
// the way main configures it. env adds to, or overrides, the defaults:
// GOBLOG_CONTENT_URL is repo's file:// URL and GOBLOG_WORK_DIR a temporary
// directory. The clock starts at startTime. goblog hasn't synced yet: call
// sync.
func startGoblog(t *testing.T, repo *contentRepo, env map[string]string) *goblog {
	t.Helper()
	workDir := t.TempDir()
	vars := map[string]string{
		"GOBLOG_CONTENT_URL": repo.url(),
		"GOBLOG_WORK_DIR":    workDir,
	}
	maps.Copy(vars, env)
	cfg, err := app.ConfigFromEnv(func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	})
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	clock := &testClock{now: startTime()}
	cfg.Now = clock.Now

	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)
	return &goblog{app: a, srv: srv, clock: clock, workDir: cfg.WorkDir}
}

// sync runs one sync now (fetch, render and switch over) and fails the test
// if it fails.
func (g *goblog) sync(t *testing.T) {
	t.Helper()
	if err := g.app.Sync(t.Context()); err != nil {
		t.Fatalf("sync: %v", err)
	}
}

// syncFails runs one sync now and fails the test unless the sync fails.
func (g *goblog) syncFails(t *testing.T) {
	t.Helper()
	if err := g.app.Sync(t.Context()); err == nil {
		t.Fatal("sync succeeded, want it to fail")
	}
}

// response is an HTTP response with its body read.
type response struct {
	status int
	header http.Header
	body   string
}

// get requests path from goblog.
func (g *goblog) get(t *testing.T, path string) response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, g.srv.URL+path, http.NoBody)
	if err != nil {
		t.Fatalf("building request for %s: %v", path, err)
	}
	client := g.srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body of %s: %v", path, err)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: string(body)}
}

// assertStatus fails the test unless resp has status want.
func assertStatus(t *testing.T, path string, resp response, want int) {
	t.Helper()
	if resp.status != want {
		t.Fatalf("GET %s status = %d, want %d; body:\n%s", path, resp.status, want, resp.body)
	}
}

// assertContains fails the test unless body contains want.
func assertContains(t *testing.T, path, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Errorf("GET %s body doesn't contain %q; body:\n%s", path, want, body)
	}
}
