package service

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dfryer1193/goblog/data"
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
}

// render is one rendered site on disk.
type render struct {
	dir    string
	files  fs.FS
	commit string
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

	prev := p.current.Swap(&render{dir: dir, files: os.DirFS(dir), commit: checkout.Commit})
	p.cfg.Logger.Info("Switched to a new render", "commit", checkout.Commit, "problems", len(site.Problems))
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
