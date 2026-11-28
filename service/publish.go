package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dfryer1193/goblog/data"
	"github.com/dfryer1193/goblog/models"
)

// outputPrefix starts the name of every output directory under the
// Publisher's working directory.
const outputPrefix = "site-"

// PublisherConfig is what a Publisher needs.
type PublisherConfig struct {
	// Repository is the clone of the Content Repository.
	Repository *data.Repository
	// OutputDir is where rendered sites go, each in a fresh directory of its
	// own. Old output found there is removed.
	OutputDir string
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
	// Options are the build's options. The Publisher fills in Commit.
	Options BuildOptions
	// Logger logs each switch-over. Nil means slog.Default().
	Logger *slog.Logger
}

// Publisher keeps the served site up to date with the Content Repository: it
// fetches, renders into a fresh directory and switches serving over to it all
// at once. It is safe for concurrent use.
type Publisher struct {
	cfg PublisherConfig
	// mu serializes syncs, which share the clone.
	mu sync.Mutex
	// current is the output being served; nil until the first render.
	current atomic.Pointer[render]
	// cleaned records whether stale output from an earlier run is gone.
	cleaned bool
	// checkout is the clone's checkout as of the last successful update,
	// and fetched whether there has been one.
	checkout data.Checkout
	fetched  bool
	// retriedOn is the last day a Poll re-rendered for, so that a re-render
	// that fails isn't retried until the next day or commit.
	retriedOn models.Date
}

// PollResult says what a Poll did.
type PollResult int

const (
	// PollUnchanged means there was nothing to do: no fetch and no render.
	PollUnchanged PollResult = iota
	// PollSynced means the Poll fetched and rendered, as Sync does.
	PollSynced
	// PollRerendered means the date rolled over since the last render, so
	// the Poll rendered the same commit again, without a fetch, to publish
	// any Posts scheduled for the new day.
	PollRerendered
)

func (r PollResult) String() string {
	switch r {
	case PollUnchanged:
		return "unchanged"
	case PollSynced:
		return "synced"
	case PollRerendered:
		return "rerendered"
	}
	return fmt.Sprintf("PollResult(%d)", int(r))
}

// render is one rendered site on disk.
type render struct {
	dir    string
	files  fs.FS
	commit string
	// tz and day are the timezone the build decided "today" in, and that
	// day.
	tz  *time.Location
	day models.Date
	// url is the site's base URL, or "" if it has none.
	url string
}

// NewPublisher returns a Publisher for cfg that serves nothing until its
// first Sync.
func NewPublisher(cfg PublisherConfig) *Publisher {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Publisher{cfg: cfg}
}

// Sync fetches the Content Repository's latest commit, renders the site from
// it into a fresh directory and switches serving over to it. If anything
// fails, the current site stays in place. Concurrent calls run one at a time.
func (p *Publisher) Sync(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sync(ctx)
}

// Poll is the cheap periodic check. It syncs if the Content Repository's
// branch has a new commit, or if there's no site yet (so a failed first
// sync is retried). Otherwise, if the date in the site's timezone has
// changed since the last render, it renders the same commit again without
// fetching, so scheduled Posts publish; that happens even if the remote
// can't be reached. Otherwise it does nothing. If anything fails, the
// current site stays in place. It runs one at a time with Sync.
func (p *Publisher) Poll(ctx context.Context) (PollResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cur := p.current.Load()
	if !p.fetched || cur == nil {
		return PollSynced, p.sync(ctx)
	}
	head, headErr := p.cfg.Repository.RemoteHead(ctx)
	if headErr == nil && head != p.checkout.Commit {
		return PollSynced, p.sync(ctx)
	}
	if headErr != nil {
		headErr = fmt.Errorf("checking the Content Repository for a new commit: %w", headErr)
	}
	today := models.DateOf(p.cfg.Now().In(cur.tz))
	if today == cur.day || today == p.retriedOn {
		return PollUnchanged, headErr
	}
	p.retriedOn = today
	return PollRerendered, errors.Join(headErr, p.publish(p.checkout))
}

// sync is Sync with p.mu held.
func (p *Publisher) sync(ctx context.Context) error {
	if !p.cleaned {
		if err := p.removeStaleOutput(); err != nil {
			return err
		}
		p.cleaned = true
	}
	checkout, err := p.cfg.Repository.Update(ctx)
	if err != nil {
		return fmt.Errorf("updating the Content Repository: %w", err)
	}
	p.checkout, p.fetched = checkout, true
	return p.publish(checkout)
}

// publish builds the site from checkout into a fresh directory and switches
// over to it.
func (p *Publisher) publish(checkout data.Checkout) error {
	opts := p.cfg.Options
	opts.Commit = checkout.Commit
	site, err := Build(checkout.Files, p.cfg.Now(), opts)
	if err != nil {
		return fmt.Errorf("rendering commit %s: %w", checkout.Commit, err)
	}
	for _, problem := range site.Problems {
		p.cfg.Logger.Warn("Skipped a file", "commit", checkout.Commit, "problem", problem.String())
	}
	for _, warning := range site.Warnings {
		p.cfg.Logger.Warn("Worked around a problem in a file", "commit", checkout.Commit, "warning", warning.String())
	}

	if err = os.MkdirAll(p.cfg.OutputDir, outputDirMode); err != nil {
		return fmt.Errorf("creating %s: %w", p.cfg.OutputDir, err)
	}
	dir, err := os.MkdirTemp(p.cfg.OutputDir, outputPrefix)
	if err != nil {
		return fmt.Errorf("creating an output directory: %w", err)
	}
	if err = WriteSite(dir, site); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("writing commit %s: %w", checkout.Commit, err)
	}
	if err = os.Chmod(dir, outputDirMode); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("writing commit %s: %w", checkout.Commit, err)
	}

	prev := p.current.Swap(&render{
		dir: dir, files: os.DirFS(dir), commit: checkout.Commit, tz: site.Timezone, day: site.Today,
		url: site.URL,
	})
	p.cfg.Logger.Info(
		"Switched to a new render",
		"commit", checkout.Commit, "problems", len(site.Problems), "warnings", len(site.Warnings),
	)
	if prev != nil {
		if err = os.RemoveAll(prev.dir); err != nil {
			p.cfg.Logger.Warn("Failed to remove the previous render", "dir", prev.dir, "error", err)
		}
	}
	return nil
}

// removeStaleOutput removes output directories left by an earlier run, e.g.
// before a container restart.
func (p *Publisher) removeStaleOutput() error {
	stale, err := filepath.Glob(filepath.Join(p.cfg.OutputDir, outputPrefix+"*"))
	if err != nil {
		return fmt.Errorf("finding old output: %w", err)
	}
	for _, dir := range stale {
		if err = os.RemoveAll(dir); err != nil {
			return fmt.Errorf("removing old output: %w", err)
		}
	}
	return nil
}

// Ready reports whether a site has been rendered and switched in. It is
// cheap enough for every readiness probe.
func (p *Publisher) Ready(context.Context) bool {
	return p.current.Load() != nil
}

// Files are the files of the site being served, or nil before the first
// render.
func (p *Publisher) Files() fs.FS {
	if cur := p.current.Load(); cur != nil {
		return cur.files
	}
	return nil
}

// SiteURL is the base URL of the site being served, from its site.yaml's
// url, or "" before the first render or if it has none.
func (p *Publisher) SiteURL() string {
	if cur := p.current.Load(); cur != nil {
		return cur.url
	}
	return ""
}
