package domain

import (
	"context"
	"time"

	"github.com/google/go-github/v75/github"
)

// SourceRepository defines the interface for accessing repository data (e.g., from GitHub).
// This allows the application to be decoupled from a specific implementation.
type SourceRepository interface {
	GetCommitsSince(ctx context.Context, branchName string, since time.Time) ([]*github.RepositoryCommit, error)
	GetCommitsInRange(ctx context.Context, baseCommit string, headCommit string) ([]*github.RepositoryCommit, error)
	GetCommit(ctx context.Context, sha string) (*github.RepositoryCommit, error)
	GetFileContents(ctx context.Context, path string, ref string) ([]byte, error)
	ListBranches(ctx context.Context) ([]*github.Branch, error)
	GetDefaultBranchName(ctx context.Context) (string, error)
	GetRepoFullName() string
}
