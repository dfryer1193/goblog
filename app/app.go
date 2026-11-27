// Package app wires goblog's serve mode together from its layers: the
// Content Repository clone (data), sync and switch-over (service) and HTTP
// (api). cmd/goblog runs it; the seam-2 tests start it the same way.
package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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
	"github.com/dfryer1193/goblog/stats"
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
	// EnvMetricsPort is the port serving the Site Statistics metrics at
	// /metrics, apart from the site. Default: 9090.
	EnvMetricsPort = "GOBLOG_METRICS_PORT"
	// EnvWebhookSecret is the GitHub webhook's secret. Optional; without
	// it, the webhook endpoint is disabled (404).
	EnvWebhookSecret = "GOBLOG_WEBHOOK_SECRET" //nolint:gosec // the variable's name, not a secret
	// EnvPollInterval is how often to check the Content Repository for a
	// new commit and the date for a rollover, as a Go duration such as
	// "1m" or "30s". Default: 1m.
	EnvPollInterval = "GOBLOG_POLL_INTERVAL"
	// EnvStatsSaltFile is the path of the daily salt for Site Statistics'
	// Visitor IDs (see the README for its contract). Optional; without it,
	// Page Views have no Visitor ID.
	EnvStatsSaltFile = "GOBLOG_STATS_SALT_FILE"
)

// syncTimeout bounds each background sync, so a hung fetch can't hold up
// every later one.
const syncTimeout = 5 * time.Minute

// defaultPort is the port goblog listens on when PORT isn't set.
const defaultPort = "8080"

// defaultMetricsPort is the metrics port when GOBLOG_METRICS_PORT isn't set.
const defaultMetricsPort = "9090"

// MetricsPath is the metrics handler's path.
const MetricsPath = "/metrics"

// defaultPollInterval is how often goblog polls when GOBLOG_POLL_INTERVAL
// isn't set.
const defaultPollInterval = time.Minute

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
	// MetricsPort is the port the metrics handler listens on.
	MetricsPort string
	// WebhookSecret is the GitHub webhook's secret, or empty to disable the
	// webhook endpoint.
	WebhookSecret string
	// PollInterval is how often Run polls.
	PollInterval time.Duration
	// StatsSaltFile is the path of the daily salt for Visitor IDs, or empty
	// for none.
	StatsSaltFile string
	// Now is the clock. Nil means time.Now. It isn't read from the
	// environment; tests set it.
	Now func() time.Time
	// StatsConfig is Site Statistics' settings. Nil means the defaults. It
	// isn't read from the environment: main sets it to the StatsConfig from
	// Mimir, and tests to their own.
	StatsConfig stats.Settings
	// StatsOutput receives the Site Statistics JSON lines. Nil means
	// stdout. It isn't read from the environment; tests set it.
	StatsOutput io.Writer
	// StatsLog receives what Site Statistics log, such as a warning that
	// there's no salt. Nil means goblog's ordinary logs. It isn't read from
	// the environment; tests set it.
	StatsLog io.Writer
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
		MetricsPort:   get(EnvMetricsPort, defaultMetricsPort),
		WebhookSecret: get(EnvWebhookSecret, ""),
		StatsSaltFile: get(EnvStatsSaltFile, ""),
	}
	if cfg.ContentURL == "" {
		return Config{}, fmt.Errorf("%s is required: set it to the Content Repository's URL", EnvContentURL)
	}
	interval, err := time.ParseDuration(get(EnvPollInterval, defaultPollInterval.String()))
	if err != nil || interval <= 0 {
		return Config{}, fmt.Errorf("%s must be a positive duration such as 1m or 30s", EnvPollInterval)
	}
	cfg.PollInterval = interval
	return cfg, nil
}

// App is goblog's serve mode: an HTTP handler serving the site, and the
// Publisher that keeps it up to date.
type App struct {
	publisher *service.Publisher
	router    *router.Router
	metrics   http.Handler
	logger    *slog.Logger
	// pollInterval is how often Run polls.
	pollInterval time.Duration
	// webhookSyncs runs the syncs webhooks ask for.
	webhookSyncs *trigger
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
	a := &App{publisher: publisher, logger: log.GetLogger("sync"), pollInterval: cfg.PollInterval}
	if cfg.WebhookSecret == "" {
		a.logger.Warn("The webhook is disabled because " + EnvWebhookSecret + " isn't set; only polling will sync")
	}
	a.webhookSyncs = &trigger{run: a.backgroundSync}
	statsLogger := log.GetLogger("stats")
	if cfg.StatsLog != nil {
		statsLogger = log.GetLoggerTo("stats", cfg.StatsLog)
	}
	siteStats, err := stats.New(stats.Config{
		Settings: cfg.StatsConfig,
		Now:      cfg.Now,
		Output:   cfg.StatsOutput,
		SiteURL:  publisher.SiteURL,
		SaltFile: cfg.StatsSaltFile,
		Logger:   statsLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("setting up Site Statistics: %w", err)
	}
	metrics := http.NewServeMux()
	metrics.Handle("GET "+MetricsPath, siteStats.MetricsHandler())
	a.metrics = metrics
	a.router, err = api.NewRouter(
		publisher,
		api.Webhook{Secret: cfg.WebhookSecret, Trigger: a.webhookSyncs.fire},
		siteStats.Wrap,
	)
	if err != nil {
		return nil, fmt.Errorf("building the HTTP handler: %w", err)
	}
	return a, nil
}

// Router is serve mode's HTTP handler: mjolnir's router, so main can Run it
// and tests can serve it with httptest.
func (a *App) Router() *router.Router {
	return a.router
}

// MetricsHandler serves the Site Statistics metrics at MetricsPath, for the
// metrics port. It's never on the Router.
func (a *App) MetricsHandler() http.Handler {
	return a.metrics
}

// Sync is the "sync now" operation: fetch the Content Repository's latest
// commit, render it and switch serving over. On failure the current site
// stays. Concurrent calls run one at a time. Sync and Poll are serve mode's
// operations as the seam-2 tests drive them; the Publisher behind them is
// an implementation detail.
func (a *App) Sync(ctx context.Context) error {
	if err := a.publisher.Sync(ctx); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}
	return nil
}

// Poll is the periodic check: it syncs if the Content Repository has a new
// commit, or if there's no site yet, and otherwise does nothing. It runs one
// at a time with Sync.
func (a *App) Poll(ctx context.Context) (service.PollResult, error) {
	result, err := a.publisher.Poll(ctx)
	if err != nil {
		return result, fmt.Errorf("polling: %w", err)
	}
	return result, nil
}

// Run keeps the site in sync until ctx is done: a first Sync straight away,
// then a Poll every poll interval, which also retries a failed first Sync.
// Failures are logged, and the previous site stays.
func (a *App) Run(ctx context.Context) {
	a.backgroundSync()
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()
	a.pollEvery(ctx, ticker.C)
}

// pollEvery polls on every tick until ctx is done.
func (a *App) pollEvery(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			a.backgroundPoll(ctx)
		}
	}
}

// backgroundPoll is one Poll on nobody's behalf: it logs what happened
// instead of returning it.
func (a *App) backgroundPoll(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	result, err := a.Poll(ctx)
	if err != nil {
		a.logger.ErrorContext(
			ctx,
			"Poll failed; still serving the previous site",
			"result",
			result.String(),
			"error",
			err,
		)
		return
	}
	if result != service.PollUnchanged {
		a.logger.InfoContext(ctx, "Poll updated the site", "result", result.String())
	}
}

// WaitForSyncs waits until the syncs that webhooks asked for have finished.
func (a *App) WaitForSyncs() {
	a.webhookSyncs.wait()
}

// backgroundSync is one Sync on nobody's behalf: it logs a failure instead of
// returning it.
func (a *App) backgroundSync() {
	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()
	if err := a.Sync(ctx); err != nil {
		a.logger.Error("Sync failed; still serving the previous site", "error", err)
	}
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
