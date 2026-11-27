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
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/dfryer1193/goblog/app"
	"github.com/dfryer1193/goblog/service"
)

// defaultBranch is the test Content Repository's default branch. It isn't
// "main" or "master", so a clone that guessed the branch would fail.
const defaultBranch = "trunk"

// webhookSecret is the webhook secret startGoblog configures by default.
const webhookSecret = "test-webhook-secret"

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

// set moves the clock to at.
func (c *testClock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = at
}

// advance moves the clock on by d.
func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// statsOutput captures the Site Statistics JSON lines goblog writes.
type statsOutput struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write captures p.
func (o *statsOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.Write(p)
}

// lines are the complete lines written so far.
func (o *statsOutput) lines() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	text := o.buf.String()
	if complete, _, ok := strings.CutLast(text, "\n"); ok {
		return strings.Split(complete, "\n")
	}
	return nil
}

// fakeStatsConfig is a StatsConfig a test can change while goblog runs.
type fakeStatsConfig struct {
	mu       sync.Mutex
	disabled bool
}

// Enabled reports whether Site Statistics are on. They are by default.
func (c *fakeStatsConfig) Enabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.disabled
}

// setEnabled turns Site Statistics on or off.
func (c *fakeStatsConfig) setEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disabled = !enabled
}

// goblog is a running goblog under test.
type goblog struct {
	app   *app.App
	srv   *httptest.Server
	clock *testClock
	// workDir is goblog's working directory (GOBLOG_WORK_DIR).
	workDir string
	// stats is where goblog writes its Site Statistics lines.
	stats *statsOutput
	// statsConfig is goblog's StatsConfig.
	statsConfig *fakeStatsConfig
	// metrics serves goblog's metrics handler, as its metrics port does.
	metrics *httptest.Server
	// saltFile is the salt file's path (GOBLOG_STATS_SALT_FILE), which
	// startGoblog writes testSalt to.
	saltFile string
}

// startGoblog starts goblog's serve mode against repo, configured from env
// the way main configures it. env adds to, or overrides, the defaults:
// GOBLOG_CONTENT_URL is repo's file:// URL, GOBLOG_WORK_DIR a temporary
// directory and GOBLOG_WEBHOOK_SECRET webhookSecret. The clock starts at startTime. goblog hasn't synced yet: call
// sync.
func startGoblog(t *testing.T, repo *contentRepo, env map[string]string) *goblog {
	t.Helper()
	return startGoblogWritingStatsTo(t, repo, env, nil)
}

// startGoblogWritingStatsTo is startGoblog, but goblog writes its Site
// Statistics lines to output instead of capturing them, unless it's nil.
func startGoblogWritingStatsTo(t *testing.T, repo *contentRepo, env map[string]string, output io.Writer) *goblog {
	t.Helper()
	workDir := t.TempDir()
	saltFile := filepath.Join(t.TempDir(), "salt")
	writeFile(t, saltFile, testSalt)
	vars := map[string]string{
		"GOBLOG_CONTENT_URL":     repo.url(),
		"GOBLOG_WORK_DIR":        workDir,
		"GOBLOG_WEBHOOK_SECRET":  webhookSecret,
		"GOBLOG_STATS_SALT_FILE": saltFile,
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
	captured := &statsOutput{}
	cfg.StatsOutput = captured
	if output != nil {
		cfg.StatsOutput = output
	}
	statsConfig := &fakeStatsConfig{}
	cfg.StatsConfig = statsConfig

	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	srv := httptest.NewServer(a.Router())
	t.Cleanup(srv.Close)
	metrics := httptest.NewServer(a.MetricsHandler())
	t.Cleanup(metrics.Close)
	return &goblog{
		app:         a,
		srv:         srv,
		clock:       clock,
		workDir:     cfg.WorkDir,
		stats:       captured,
		statsConfig: statsConfig,
		metrics:     metrics,
		saltFile:    cfg.StatsSaltFile,
	}
}

// writeSalt replaces the salt file's contents with salt, as the daily
// rotation does.
func (g *goblog) writeSalt(t *testing.T, salt string) {
	t.Helper()
	writeFile(t, g.saltFile, salt)
}

// testSalt is the salt startGoblog writes to the salt file: 32 bytes.
const testSalt = "test-salt-0123456789abcdefghijkl"

// writeFile writes content to path, replacing it.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// metric scrapes goblog's metrics handler at /metrics and returns the value
// of the sample name{labels}, such as goblog_page_views_total{kind="post"},
// or 0 if there's no such sample.
func (g *goblog) metric(t *testing.T, sample string) float64 {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, g.metrics.URL+"/metrics", http.NoBody)
	if err != nil {
		t.Fatalf("building metrics request: %v", err)
	}
	resp, err := g.metrics.Client().Do(req)
	if err != nil {
		t.Fatalf("scraping metrics: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading metrics: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status = %d; body:\n%s", resp.StatusCode, body)
	}
	for line := range strings.Lines(string(body)) {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), sample+" "); ok {
			f, parseErr := strconv.ParseFloat(value, 64)
			if parseErr != nil {
				t.Fatalf("metric %s has value %q: %v", sample, value, parseErr)
			}
			return f
		}
	}
	return 0
}

// statsLines waits for goblog to have written n Site Statistics lines, and
// returns them decoded. It fails the test if there are more, or if they
// don't arrive.
func (g *goblog) statsLines(t *testing.T, n int) []map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	lines := g.stats.lines()
	for len(lines) < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		lines = g.stats.lines()
	}
	if len(lines) != n {
		t.Fatalf("got %d Site Statistics lines, want %d:\n%s", len(lines), n, strings.Join(lines, "\n"))
	}
	decoded := make([]map[string]any, n)
	for i, line := range lines {
		if err := json.Unmarshal([]byte(line), &decoded[i]); err != nil {
			t.Fatalf("Site Statistics line %d isn't JSON: %v\n%s", i, err, line)
		}
	}
	return decoded
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

// poll runs one poll now and fails the test if it fails or does something
// other than want.
func (g *goblog) poll(t *testing.T, want service.PollResult) {
	t.Helper()
	got, err := g.app.Poll(t.Context())
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if got != want {
		t.Fatalf("poll = %v, want %v", got, want)
	}
}

// response is an HTTP response with its body read.
type response struct {
	status int
	header http.Header
	body   string
}

// get requests path from goblog, the way a client that doesn't accept gzip
// would.
func (g *goblog) get(t *testing.T, path string) response {
	t.Helper()
	return g.request(t, http.MethodGet, path, nil)
}

// request sends a method request for path to goblog with header added.
// Redirects aren't followed, and the body isn't decompressed: no
// Accept-Encoding is sent unless header has one.
func (g *goblog) request(t *testing.T, method, path string, header http.Header) response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, g.srv.URL+path, http.NoBody)
	if err != nil {
		t.Fatalf("building request for %s: %v", path, err)
	}
	maps.Copy(req.Header, header)
	transport, ok := g.srv.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatal("the test server's client has no *http.Transport")
	}
	transport = transport.Clone()
	transport.DisableCompression = true
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body of %s: %v", path, err)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: string(body)}
}

// sign is GitHub's X-Hub-Signature-256 value for body under secret.
func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// webhook delivers a GitHub webhook event with body, signed with signature
// (no X-Hub-Signature-256 header if it's empty), and returns the response.
func (g *goblog) webhook(t *testing.T, event, body, signature string) response {
	t.Helper()
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		g.srv.URL+"/webhook",
		bytes.NewBufferString(body),
	)
	if err != nil {
		t.Fatalf("building webhook request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Github-Event", event)
	if signature != "" {
		req.Header.Set("X-Hub-Signature-256", signature)
	}
	resp, err := g.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("POST /webhook: %v", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading webhook response: %v", err)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: string(respBody)}
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
