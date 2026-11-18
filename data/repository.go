package data

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// objectCacheSize bounds go-git's in-memory object cache. go-git's default
// (96 MiB) is more than goblog's whole memory limit; the clone is shallow
// and small, so a few MiB is plenty.
const objectCacheSize = 4 * cache.MiByte

// remoteName is the name the clone gives the Content Repository's remote.
const remoteName = "origin"

// RepositoryConfig says where the Content Repository is and where to keep the
// clone of it.
type RepositoryConfig struct {
	// URL is the Content Repository's URL: SSH (git@host:owner/repo.git or
	// ssh://), HTTPS, or file:// for tests.
	URL string
	// Branch is the branch to follow. Empty means the remote's default
	// branch.
	Branch string
	// Dir is where the clone goes. Whatever is there already is removed
	// before the first clone.
	Dir string
	// Auth authenticates to the remote, e.g. with an SSH deploy key. Nil
	// means none.
	Auth transport.AuthMethod
}

// Checkout is the Content Repository's files at one commit.
type Checkout struct {
	// Commit is the full hash of the checked-out commit.
	Commit string
	// Files are the commit's files. They stay valid until the next Update.
	Files fs.FS
}

// Repository is a shallow, single-branch clone of the Content Repository,
// kept on disk. It isn't safe for concurrent use.
type Repository struct {
	cfg  RepositoryConfig
	repo *git.Repository
	// branch is the branch being followed, once known.
	branch plumbing.ReferenceName
}

// NewRepository returns a Repository for cfg. It doesn't touch the network or
// the disk until Update.
func NewRepository(cfg RepositoryConfig) *Repository {
	return &Repository{cfg: cfg}
}

// Update brings the clone up to date with the remote's branch, cloning it
// first if needed, and returns the checked-out commit.
func (r *Repository) Update(ctx context.Context) (Checkout, error) {
	var err error
	if r.repo == nil {
		err = r.clone(ctx)
	} else {
		err = r.pull(ctx)
	}
	if err != nil {
		return Checkout{}, err
	}
	head, err := r.repo.Head()
	if err != nil {
		return Checkout{}, fmt.Errorf("reading the clone's HEAD: %w", err)
	}
	return Checkout{Commit: head.Hash().String(), Files: os.DirFS(r.cfg.Dir)}, nil
}

// clone makes a fresh shallow, single-branch clone into the configured
// directory.
func (r *Repository) clone(ctx context.Context) error {
	if err := os.RemoveAll(r.cfg.Dir); err != nil {
		return fmt.Errorf("clearing %s: %w", r.cfg.Dir, err)
	}
	opts := &git.CloneOptions{
		URL:          r.cfg.URL,
		Auth:         r.cfg.Auth,
		RemoteName:   remoteName,
		SingleBranch: true,
		Depth:        1,
		Tags:         git.NoTags,
	}
	if r.cfg.Branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(r.cfg.Branch)
	}
	repo, err := git.CloneContext(ctx, storage(r.cfg.Dir), osfs.New(r.cfg.Dir), opts)
	if err != nil {
		return fmt.Errorf("cloning %s: %w", r.cfg.URL, err)
	}
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("reading the clone's HEAD: %w", err)
	}
	r.repo, r.branch = repo, head.Name()
	return nil
}

// pull fetches the branch's latest commit and checks it out.
func (r *Repository) pull(ctx context.Context) error {
	remoteRef := plumbing.NewRemoteReferenceName(remoteName, r.branch.Short())
	err := r.repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{config.RefSpec("+" + r.branch.String() + ":" + remoteRef.String())},
		Depth:      1,
		Auth:       r.cfg.Auth,
		Tags:       git.NoTags,
		Force:      true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetching %s from %s: %w", r.branch.Short(), r.cfg.URL, err)
	}
	ref, err := r.repo.Reference(remoteRef, true)
	if err != nil {
		return fmt.Errorf("reading %s: %w", remoteRef, err)
	}
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("opening the clone's worktree: %w", err)
	}
	if err = wt.Reset(&git.ResetOptions{Commit: ref.Hash(), Mode: git.HardReset}); err != nil {
		return fmt.Errorf("checking out %s: %w", ref.Hash(), err)
	}
	return nil
}

// storage is go-git's on-disk object storage for a clone in dir, with a small
// object cache. Keeping objects on disk rather than in memory keeps goblog's
// memory use low.
func storage(dir string) *filesystem.Storage {
	return filesystem.NewStorage(osfs.New(filepath.Join(dir, git.GitDirName)), cache.NewObjectLRU(objectCacheSize))
}
