// Package app wires goblog's serve mode together from its layers: the
// Content Repository clone (data), sync and switch-over (service) and HTTP
// (api). cmd/goblog runs it; the seam-2 tests start it the same way.
package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dfryer1193/mjolnir/router"
	"github.com/dfryer1193/mjolnir/utils/log"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"

	"github.com/dfryer1193/goblog/api"
	"github.com/dfryer1193/goblog/data"
	"github.com/dfryer1193/goblog/service"
)

// The environment variables serve mode reads.
const (
	// EnvContentURL is the Content Repository's URL. Required.
	EnvContentURL = "GOBLOG_CONTENT_URL"
	// EnvContentBranch is the branch to publish. Default: the remote's
	// default branch.
	EnvContentBranch = "GOBLOG_CONTENT_BRANCH"
	// EnvDeployKey is a PEM-encoded SSH private key (a read-only deploy
	// key) for an SSH Content Repository URL. Optional.
	EnvDeployKey = "GOBLOG_DEPLOY_KEY"
	// EnvKnownHosts is known_hosts content for checking the SSH host's key.
	// Optional; without it, go-git's defaults apply ($SSH_KNOWN_HOSTS or
	// ~/.ssh/known_hosts).
	EnvKnownHosts = "GOBLOG_SSH_KNOWN_HOSTS"
	// EnvWorkDir is goblog's ephemeral working directory, for the clone and
	// the rendered output. Default: goblog under the system temp directory.
	EnvWorkDir = "GOBLOG_WORK_DIR"
	// EnvPort is the port to listen on. Default: 8080.
	EnvPort = "PORT"
)

// defaultPort is the port goblog listens on when PORT isn't set.
const defaultPort = "8080"

// sshUser is the user for SSH Content Repository URLs that don't name one.
const sshUser = "git"

// Working directory layout.
const (
	contentSubdir = "content"
	outputSubdir  = "output"
)

// Config is serve mode's runtime configuration.
type Config struct {
	// ContentURL is the Content Repository's URL.
	ContentURL string
	// ContentBranch is the branch to publish; empty means the remote's
	// default branch.
	ContentBranch string
	// DeployKey is a PEM-encoded SSH private key, or empty for none.
	DeployKey string
	// KnownHosts is known_hosts content, or empty for go-git's defaults.
	KnownHosts string
	// WorkDir is the working directory for the clone and the output.
	WorkDir string
	// Port is the port to listen on.
	Port string
	// Now is the clock. Nil means time.Now. It isn't read from the
	// environment; tests set it.
	Now func() time.Time
}

// ConfigFromEnv reads Config from the environment through lookup, which has
// [os.LookupEnv]'s signature.
func ConfigFromEnv(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && v != "" {
			return v
		}
		return def
	}
	cfg := Config{
		ContentURL:    get(EnvContentURL, ""),
		ContentBranch: get(EnvContentBranch, ""),
		DeployKey:     get(EnvDeployKey, ""),
		KnownHosts:    get(EnvKnownHosts, ""),
		WorkDir:       get(EnvWorkDir, filepath.Join(os.TempDir(), "goblog")),
		Port:          get(EnvPort, defaultPort),
	}
	if cfg.ContentURL == "" {
		return Config{}, fmt.Errorf("%s is required: set it to the Content Repository's URL", EnvContentURL)
	}
	return cfg, nil
}

// App is goblog's serve mode: an HTTP handler serving the site, and the
// Publisher that keeps it up to date.
type App struct {
	publisher *service.Publisher
	router    *router.Router
}

// New wires up serve mode from cfg. It doesn't sync: the site is served, and
// goblog is ready, after the first successful Sync.
func New(cfg Config) (*App, error) {
	auth, err := sshAuth(cfg)
	if err != nil {
		return nil, err
	}
	repo := data.NewRepository(data.RepositoryConfig{
		URL:    cfg.ContentURL,
		Branch: cfg.ContentBranch,
		Dir:    filepath.Join(cfg.WorkDir, contentSubdir),
		Auth:   auth,
	})
	publisher := service.NewPublisher(service.PublisherConfig{
		Repository: repo,
		OutputDir:  filepath.Join(cfg.WorkDir, outputSubdir),
		Now:        cfg.Now,
		Logger:     log.GetLogger("publish"),
	})
	r, err := api.NewRouter(publisher)
	if err != nil {
		return nil, fmt.Errorf("building the HTTP handler: %w", err)
	}
	return &App{publisher: publisher, router: r}, nil
}

// Router is serve mode's HTTP handler, as mjolnir's router so main can Run
// it.
func (a *App) Router() *router.Router {
	return a.router
}

// Handler is serve mode's HTTP handler.
func (a *App) Handler() http.Handler {
	return a.router
}

// Sync is the "sync now" operation: fetch the Content Repository's latest
// commit, render it and switch serving over. On failure the current site
// stays. Concurrent calls run one at a time.
func (a *App) Sync(ctx context.Context) error {
	if err := a.publisher.Sync(ctx); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}
	return nil
}

// sshAuth is the SSH auth for cfg's deploy key, or nil without one.
func sshAuth(cfg Config) (transport.AuthMethod, error) {
	if cfg.DeployKey == "" {
		return nil, nil //nolint:nilnil // no deploy key means no auth, which isn't an error
	}
	keys, err := gitssh.NewPublicKeys(sshUser, []byte(cfg.DeployKey), "")
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", EnvDeployKey, err)
	}
	if cfg.KnownHosts != "" {
		keys.HostKeyCallback, err = knownHostsCallback(cfg)
		if err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// knownHostsCallback checks host keys against cfg.KnownHosts, which it
// writes to a file in the working directory because go-git's known_hosts
// support reads files.
func knownHostsCallback(cfg Config) (ssh.HostKeyCallback, error) {
	if err := os.MkdirAll(cfg.WorkDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", cfg.WorkDir, err)
	}
	file := filepath.Join(cfg.WorkDir, "known_hosts")
	if err := os.WriteFile(file, []byte(cfg.KnownHosts), 0o600); err != nil {
		return nil, fmt.Errorf("writing %s: %w", EnvKnownHosts, err)
	}
	callback, err := gitssh.NewKnownHostsCallback(file)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", EnvKnownHosts, err)
	}
	return callback, nil
}
